#!/usr/bin/env bash
set -euo pipefail

GATEWAY_URL="${GATEWAY_URL:-http://127.0.0.1:8080}"
READ_MODEL_URL="${READ_MODEL_URL:-http://127.0.0.1:8089}"
SHIPMENT_URL="${SHIPMENT_URL:-http://127.0.0.1:8085}"
AUTH_ENABLED="${AUTH_ENABLED:-true}"
JWT_TOKEN="${JWT_TOKEN:-}"
TENANT_ID="${TENANT_ID:-}"
ADMIN_EMAIL="${ADMIN_EMAIL:-}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-}"
POLL_TIMEOUT_SEC="${POLL_TIMEOUT_SEC:-120}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

fail() { echo "acceptance: $*" >&2; exit 1; }

require_cmd() { command -v "$1" >/dev/null 2>&1 || fail "$1 is required"; }

metric_sum() {
  curl -fsS "${1}/metrics" | grep -E "^${2}" | awk '{sum+=$2} END {print sum+0}'
}

metric_label() {
  local url="$1" name="$2" label="$3"
  curl -fsS "${url}/metrics" | grep -E "^${name}" | grep "${label}" | awk '{sum+=$2} END {print sum+0}'
}

login_jwt() {
  if [[ -n "${JWT_TOKEN}" ]]; then
    return 0
  fi
  [[ -n "${TENANT_ID}" && -n "${ADMIN_EMAIL}" && -n "${ADMIN_PASSWORD}" ]] \
    || fail "JWT_TOKEN or TENANT_ID+ADMIN_EMAIL+ADMIN_PASSWORD required"
  require_cmd jq
  local tmp code
  tmp="$(mktemp "${TMPDIR:-/tmp}/ct-acc-login.XXXXXX")"
  code="$(curl -sS -o "${tmp}" -w '%{http_code}' \
    -H 'Content-Type: application/json' \
    -d "{\"tenant_id\":\"${TENANT_ID}\",\"email\":\"${ADMIN_EMAIL}\",\"password\":\"${ADMIN_PASSWORD}\"}" \
    "${GATEWAY_URL}/api/v1/auth/login")"
  [[ "${code}" == "200" ]] || fail "login failed HTTP ${code}"
  JWT_TOKEN="$(jq -er '.access_token // empty' "${tmp}")"
  rm -f "${tmp}"
  [[ -n "${JWT_TOKEN}" ]] || fail "login missing access_token"
}

wait_for_projection() {
  local expected="$1" deadline=$((SECONDS + POLL_TIMEOUT_SEC))
  local legacy_tmp rm_tmp
  legacy_tmp="$(mktemp "${TMPDIR:-/tmp}/ct-acc-legacy.XXXXXX")"
  rm_tmp="$(mktemp "${TMPDIR:-/tmp}/ct-acc-rm.XXXXXX")"
  while (( SECONDS < deadline )); do
    curl -fsS -H "X-Tenant-ID: ${TENANT_ID}" \
      "${SHIPMENT_URL}/internal/v1/shipments/status-summary" >"${legacy_tmp}"
    curl -fsS -H "X-Tenant-ID: ${TENANT_ID}" \
      "${READ_MODEL_URL}/internal/v1/control-tower/status-summary" >"${rm_tmp}"
    if jq -e --argjson expected "${expected}" '
      (.totalShipments // 0) == $expected
      and (.complete == true)
      and (.countedShipments // 0) == (.totalShipments // 0)
    ' "${legacy_tmp}" >/dev/null \
      && jq -e --argjson expected "${expected}" '
      (.totalShipments // 0) == $expected
      and ((.incompleteProjections // 0) == 0)
    ' "${rm_tmp}" >/dev/null \
      && jq -e -s '
      ((.[0].byStatus // {}) | to_entries | sort_by(.key))
      == ((.[1].byStatus // {}) | to_entries | sort_by(.key))
    ' "${legacy_tmp}" "${rm_tmp}" >/dev/null; then
      rm -f "${legacy_tmp}" "${rm_tmp}"
      echo "acceptance: projection converged (total=${expected}, byStatus aligned)" >&2
      return 0
    fi
    sleep 2
  done
  rm -f "${legacy_tmp}" "${rm_tmp}"
  fail "projection did not converge to ${expected} with aligned byStatus within ${POLL_TIMEOUT_SEC}s"
}

verify_legacy_endpoint() {
  local body code
  body="$(curl -fsS -H "X-Tenant-ID: ${TENANT_ID}" \
    "${SHIPMENT_URL}/internal/v1/shipments/status-summary")"
  echo "${body}" | jq -e '.complete == true' >/dev/null \
    || fail "legacy status-summary complete!=true"
  echo "${body}" | jq -e '.countedShipments == .totalShipments' >/dev/null \
    || fail "legacy counted/total mismatch"
  local sum counted
  sum="$(echo "${body}" | jq '[.byStatus[]] | add // 0')"
  counted="$(echo "${body}" | jq '.countedShipments')"
  [[ "${sum}" == "${counted}" ]] || fail "legacy byStatus sum mismatch"
}

cleanup_jwt() { unset JWT_TOKEN ADMIN_PASSWORD; }

trap cleanup_jwt EXIT

echo "== acceptance fixture =="
eval "$("${ROOT}/scripts/dev/control_tower_shadow_rollout_acceptance_fixture.sh")"

echo "== wait projection convergence =="
wait_for_projection 5

echo "== verify full legacy endpoint =="
verify_legacy_endpoint

echo "== authenticated shadow summary =="
login_jwt

local_before_legacy="$(metric_sum "${GATEWAY_URL}" 'control_tower_legacy_status_aggregate_requests_total')"
local_before_rm="$(metric_sum "${GATEWAY_URL}" 'control_tower_read_model_requests_total')"
local_before_match="$(metric_label "${GATEWAY_URL}" 'control_tower_read_model_shadow_comparison_total' 'comparison="MATCH"')"

summary_tmp="$(mktemp "${TMPDIR:-/tmp}/ct-acc-summary.XXXXXX")"
code="$(curl -sS -o "${summary_tmp}" -w '%{http_code}' \
  -H "Authorization: Bearer ${JWT_TOKEN}" \
  "${GATEWAY_URL}/api/v1/control-tower/summary")"
[[ "${code}" == "200" ]] || fail "summary HTTP ${code}"

require_cmd jq
jq -e '.statusSummary.source == "LEGACY"' "${summary_tmp}" >/dev/null \
  || fail "statusSummary.source must be LEGACY"
jq -e '(.statusSummary.limitedDataset // false) == false' "${summary_tmp}" >/dev/null \
  || fail "statusSummary must not be limited for acceptance tenant"
jq -e '(.statusSummaryFreshness.fallbackUsed // false) == false' "${summary_tmp}" >/dev/null \
  || fail "fallback must not be used for acceptance tenant"
rm -f "${summary_tmp}"

after_legacy="$(metric_sum "${GATEWAY_URL}" 'control_tower_legacy_status_aggregate_requests_total')"
after_rm="$(metric_sum "${GATEWAY_URL}" 'control_tower_read_model_requests_total')"
after_match="$(metric_label "${GATEWAY_URL}" 'control_tower_read_model_shadow_comparison_total' 'comparison="MATCH"')"

(( after_legacy > local_before_legacy )) || fail "legacy metric did not increase"
(( after_rm > local_before_rm )) || fail "read-model metric did not increase"
(( after_match > local_before_match )) || fail "MATCH comparison metric did not increase (before=${local_before_match}, after=${after_match})"

local_match_now="$(metric_label "${GATEWAY_URL}" 'control_tower_read_model_shadow_comparison_total' 'comparison="MATCH"')"
local_mismatch_now="$(metric_sum "${GATEWAY_URL}" 'control_tower_read_model_shadow_comparison_total')"
local_mismatch_now=$((local_mismatch_now - local_match_now))
echo "acceptance: legacy_delta=$((after_legacy-local_before_legacy)) rm_delta=$((after_rm-local_before_rm)) match_delta=$((after_match-local_before_match)) match_total=${local_match_now}" >&2
echo "control-tower-shadow-rollout-acceptance: OK"
