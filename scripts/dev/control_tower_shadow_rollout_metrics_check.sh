#!/usr/bin/env bash
set -euo pipefail

GATEWAY_URL="${GATEWAY_URL:-http://127.0.0.1:8080}"
READ_MODEL_URL="${READ_MODEL_URL:-http://127.0.0.1:8089}"
PROMETHEUS_URL="${PROMETHEUS_URL:-http://127.0.0.1:9090}"
JWT_TOKEN="${JWT_TOKEN:-}"
TENANT_ID="${TENANT_ID:-}"
DEV_ADMIN_EMAIL="${DEV_ADMIN_EMAIL:-}"
DEV_ADMIN_PASSWORD="${DEV_ADMIN_PASSWORD:-}"
AUTH_ENABLED="${AUTH_ENABLED:-true}"
REQUIRE_METRIC_DELTA="${REQUIRE_METRIC_DELTA:-true}"
REQUIRE_MATCH="${REQUIRE_MATCH:-true}"

[[ -n "${GATEWAY_URL}" ]] || GATEWAY_URL="http://127.0.0.1:8080"
[[ -n "${READ_MODEL_URL}" ]] || READ_MODEL_URL="http://127.0.0.1:8089"

fail() {
  echo "metrics-check: $*" >&2
  exit 1
}

check_metric_present() {
  local url="$1"
  local name="$2"
  curl -fsS "${url}/metrics" | grep -qE "^# HELP ${name} |^${name}" || fail "missing metric ${name} on ${url}"
}

metric_sum() {
  local url="$1"
  local pattern="$2"
  curl -fsS "${url}/metrics" | grep -E "^${pattern}" | awk '{sum+=$2} END {print sum+0}'
}

metric_match() {
  curl -fsS "${GATEWAY_URL}/metrics" | grep 'control_tower_read_model_shadow_comparison_total{comparison="MATCH"' | awk '{sum+=$2} END {print sum+0}'
}

resolve_jwt_token() {
  if [[ -n "${JWT_TOKEN}" ]]; then
    return 0
  fi
  if [[ "${AUTH_ENABLED}" == "false" ]]; then
    fail "AUTH_ENABLED=false is not valid for metrics acceptance"
  fi
  if [[ -z "${TENANT_ID}" || -z "${DEV_ADMIN_EMAIL}" || -z "${DEV_ADMIN_PASSWORD}" ]]; then
    fail "JWT_TOKEN or TENANT_ID+DEV_ADMIN_EMAIL+DEV_ADMIN_PASSWORD required"
  fi
  command -v jq >/dev/null 2>&1 || fail "jq is required"
  local login_tmp
  login_tmp="$(mktemp "${TMPDIR:-/tmp}/ct-shadow-login.XXXXXX")"
  local code
  code="$(curl -sS -o "${login_tmp}" -w '%{http_code}' \
    -H 'Content-Type: application/json' \
    -d "{\"tenant_id\":\"${TENANT_ID}\",\"email\":\"${DEV_ADMIN_EMAIL}\",\"password\":\"${DEV_ADMIN_PASSWORD}\"}" \
    "${GATEWAY_URL}/api/v1/auth/login")"
  [[ "${code}" == "200" ]] || fail "dev login failed HTTP ${code}"
  JWT_TOKEN="$(jq -er '.access_token // empty' "${login_tmp}")"
  rm -f "${login_tmp}"
  [[ -n "${JWT_TOKEN}" ]] || fail "dev login response missing access_token"
}

echo "== gateway control tower metrics =="
check_metric_present "${GATEWAY_URL}" 'control_tower_read_model_shadow_comparison_total'
check_metric_present "${GATEWAY_URL}" 'control_tower_read_model_requests_total'
check_metric_present "${GATEWAY_URL}" 'control_tower_read_model_request_duration_seconds'
check_metric_present "${GATEWAY_URL}" 'control_tower_legacy_status_aggregate_requests_total'
check_metric_present "${GATEWAY_URL}" 'control_tower_legacy_status_fallback_total'

echo "== read-model consumer metrics =="
check_metric_present "${READ_MODEL_URL}" 'control_tower_shipment_consumer_records_total'
check_metric_present "${READ_MODEL_URL}" 'control_tower_shipment_consumer_errors_total'
check_metric_present "${READ_MODEL_URL}" 'control_tower_shipment_projection_applied_total'
check_metric_present "${READ_MODEL_URL}" 'control_tower_shipment_dead_letter_total'

if [[ "${REQUIRE_METRIC_DELTA}" == "true" ]]; then
  resolve_jwt_token
  before_rm="$(metric_sum "${GATEWAY_URL}" 'control_tower_read_model_requests_total')"
  before_legacy="$(metric_sum "${GATEWAY_URL}" 'control_tower_legacy_status_aggregate_requests_total')"
  before_match="$(metric_match)"
  auth_args=(-H "Authorization: Bearer ${JWT_TOKEN}")
  curl -fsS "${auth_args[@]}" "${GATEWAY_URL}/api/v1/control-tower/summary" >/dev/null \
    || fail "authenticated control tower summary failed"
  after_rm="$(metric_sum "${GATEWAY_URL}" 'control_tower_read_model_requests_total')"
  after_legacy="$(metric_sum "${GATEWAY_URL}" 'control_tower_legacy_status_aggregate_requests_total')"
  after_match="$(metric_match)"
  awk -v before="${before_rm}" -v after="${after_rm}" 'BEGIN { exit(after > before ? 0 : 1) }' \
    || fail "read-model request metric did not increase after summary request"
  awk -v before="${before_legacy}" -v after="${after_legacy}" 'BEGIN { exit(after > before ? 0 : 1) }' \
    || fail "legacy aggregate metric did not increase after summary request"
  if [[ "${REQUIRE_MATCH}" == "true" ]]; then
    awk -v before="${before_match}" -v after="${after_match}" 'BEGIN { exit(after > before ? 0 : 1) }' \
      || fail "MATCH comparison metric did not increase (before=${before_match}, after=${after_match})"
  fi
fi

if curl -fsS "${PROMETHEUS_URL}/api/v1/status/config" >/dev/null 2>&1; then
  echo "== prometheus recording rules =="
  curl -fsS "${PROMETHEUS_URL}/api/v1/rules" | grep -q 'control_tower:shadow_comparison:rate5m' \
    || echo "metrics-check: warning: recording rule not loaded (prometheus may need observability profile)"
fi

trap 'unset JWT_TOKEN DEV_ADMIN_PASSWORD' EXIT

echo "control-tower-shadow-rollout-metrics-check: OK"
