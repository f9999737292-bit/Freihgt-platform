#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

# Local defaults only; empty-string overrides from .env must not break localhost URLs.
GATEWAY_URL="${GATEWAY_URL:-http://127.0.0.1:8080}"
READ_MODEL_URL="${READ_MODEL_URL:-http://127.0.0.1:8089}"
SHIPMENT_URL="${SHIPMENT_URL:-http://127.0.0.1:8085}"
JWT_TOKEN="${JWT_TOKEN:-}"
TENANT_ID="${TENANT_ID:-}"
DEV_ADMIN_EMAIL="${DEV_ADMIN_EMAIL:-}"
DEV_ADMIN_PASSWORD="${DEV_ADMIN_PASSWORD:-}"
AUTH_ENABLED="${AUTH_ENABLED:-}"

[[ -n "${GATEWAY_URL}" ]] || GATEWAY_URL="http://127.0.0.1:8080"
[[ -n "${READ_MODEL_URL}" ]] || READ_MODEL_URL="http://127.0.0.1:8089"
[[ -n "${SHIPMENT_URL}" ]] || SHIPMENT_URL="http://127.0.0.1:8085"

SUMMARY_FILE="$(mktemp "${TMPDIR:-/tmp}/ct-shadow-summary.XXXXXX")"
cleanup() {
  rm -f "${SUMMARY_FILE}"
}
trap cleanup EXIT

fail() {
  echo "shadow-smoke: $*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "$1 is required"
}

curl_ok() {
  curl -fsS "$1" >/dev/null
}

metric_sum() {
  local url="$1"
  local pattern="$2"
  curl -fsS "${url}/metrics" | grep -E "^${pattern}" | awk '{sum+=$2} END {print sum+0}'
}

resolve_jwt_token() {
  if [[ -n "${JWT_TOKEN}" ]]; then
    return 0
  fi
  if [[ "${AUTH_ENABLED}" == "false" ]]; then
    return 0
  fi
  if [[ -z "${DEV_ADMIN_EMAIL}" || -z "${DEV_ADMIN_PASSWORD}" ]]; then
    fail "JWT_TOKEN or DEV_ADMIN_EMAIL+DEV_ADMIN_PASSWORD required for authenticated smoke"
  fi
  require_cmd jq
  local login_tmp
  login_tmp="$(mktemp "${TMPDIR:-/tmp}/ct-shadow-login.XXXXXX")"
  local code
  code="$(curl -sS -o "${login_tmp}" -w '%{http_code}' \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"${DEV_ADMIN_EMAIL}\",\"password\":\"${DEV_ADMIN_PASSWORD}\"}" \
    "${GATEWAY_URL}/api/v1/auth/login")"
  [[ "${code}" == "200" ]] || fail "dev login failed HTTP ${code}"
  JWT_TOKEN="$(jq -er '.access_token // empty' "${login_tmp}")"
  rm -f "${login_tmp}"
  [[ -n "${JWT_TOKEN}" ]] || fail "dev login response missing access_token"
}

echo "== health/readiness =="
curl_ok "${GATEWAY_URL}/health" || fail "gateway /health unavailable"
curl_ok "${GATEWAY_URL}/ready" || fail "gateway /ready unavailable"
curl_ok "${READ_MODEL_URL}/health" || fail "read-model /health unavailable"
curl_ok "${READ_MODEL_URL}/ready" || fail "read-model /ready unavailable"

echo "== shipment status-summary internal =="
curl -fsS "${SHIPMENT_URL}/health" >/dev/null || fail "shipment-service unavailable"
if [[ -n "${TENANT_ID}" ]]; then
  if ! curl -fsS -H "X-Tenant-ID: ${TENANT_ID}" "${SHIPMENT_URL}/internal/v1/shipments/status-summary" >/dev/null; then
    echo "shadow-smoke: warning: full legacy aggregate endpoint unavailable (status-summary 404/not ready)"
  fi
fi

echo "== consumer metadata =="
consumer_body="$(curl -fsS "${READ_MODEL_URL}/internal/v1/control-tower/status-summary" \
  -H "X-Tenant-ID: ${TENANT_ID:-00000000-0000-0000-0000-000000000001}")"
echo "${consumer_body}" | grep -Fq 'consumerRunning' \
  || fail "read-model summary missing consumerRunning metadata"

resolve_jwt_token

before_rm="$(metric_sum "${GATEWAY_URL}" 'control_tower_read_model_requests_total')"
before_cmp="$(metric_sum "${GATEWAY_URL}" 'control_tower_read_model_shadow_comparison_total')"
before_legacy="$(metric_sum "${GATEWAY_URL}" 'control_tower_legacy_status_aggregate_requests_total')"

echo "== control tower summary =="
auth_args=()
if [[ -n "${JWT_TOKEN}" ]]; then
  auth_args=(-H "Authorization: Bearer ${JWT_TOKEN}")
fi
summary_code="$(curl -sS -o "${SUMMARY_FILE}" -w '%{http_code}' "${auth_args[@]}" \
  "${GATEWAY_URL}/api/v1/control-tower/summary")"
[[ "${summary_code}" == "200" ]] || fail "control tower summary HTTP ${summary_code}"

require_cmd jq
jq -e '.statusSummary.source == "LEGACY"' "${SUMMARY_FILE}" >/dev/null \
  || fail "public statusSummary source must remain LEGACY in shadow mode"
if grep -qi 'control-tower-read-model-service\|8089\|redpanda\|postgres://' "${SUMMARY_FILE}"; then
  fail "public response leaked internal URL"
fi

after_rm="$(metric_sum "${GATEWAY_URL}" 'control_tower_read_model_requests_total')"
after_cmp="$(metric_sum "${GATEWAY_URL}" 'control_tower_read_model_shadow_comparison_total')"
after_legacy="$(metric_sum "${GATEWAY_URL}" 'control_tower_legacy_status_aggregate_requests_total')"

awk -v before="${before_rm}" -v after="${after_rm}" 'BEGIN { exit(after > before ? 0 : 1) }' \
  || fail "read-model request metric did not increase (before=${before_rm}, after=${after_rm})"
awk -v before="${before_cmp}" -v after="${after_cmp}" 'BEGIN { exit(after > before ? 0 : 1) }' \
  || fail "shadow comparison metric did not increase (before=${before_cmp}, after=${after_cmp})"
awk -v before="${before_legacy}" -v after="${after_legacy}" 'BEGIN { exit(after > before ? 0 : 1) }' \
  || fail "legacy aggregate metric did not increase (before=${before_legacy}, after=${after_legacy})"

echo "control-tower-shadow-rollout-smoke-test: OK"
