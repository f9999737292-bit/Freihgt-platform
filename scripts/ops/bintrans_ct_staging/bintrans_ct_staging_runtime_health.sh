#!/usr/bin/env bash
# BINTRANS dedicated staging — runtime health verification (read-only; no secret echo).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

bintrans_require_env_file

GATEWAY_URL="${GATEWAY_URL:-$(bintrans_env_value GATEWAY_URL)}"
GATEWAY_URL="${GATEWAY_URL:-http://127.0.0.1:18080}"
CTRM_PORT="$(bintrans_env_value CONTROL_TOWER_READ_MODEL_HOST_PORT)"
CTRM_PORT="${CTRM_PORT:-8089}"
CTRM_URL="http://127.0.0.1:${CTRM_PORT}"

expected_services=("${bintrans_runtime_service_names[@]}")
forbidden_services=(migrate prometheus grafana)

echo "=== BINTRANS runtime health check ==="

pg_cid="$(bintrans_postgres_container)"
rp_cid="$(bintrans_redpanda_container)"
[[ -n "${pg_cid}" ]] || bintrans_fail "postgres container not running"
[[ -n "${rp_cid}" ]] || bintrans_fail "redpanda container not running"

pg_user="$(bintrans_env_value POSTGRES_USER)"
pg_db="$(bintrans_env_value POSTGRES_DB)"
pg_user="${pg_user:-bintrans_staging}"
pg_db="${pg_db:-freight_platform}"

if docker exec "${pg_cid}" pg_isready -U "${pg_user}" -d "${pg_db}" >/dev/null 2>&1; then
  echo "OK: postgres healthy"
else
  bintrans_fail "postgres not ready"
fi

if docker exec "${rp_cid}" rpk cluster info --brokers localhost:9092 >/dev/null 2>&1; then
  echo "OK: redpanda healthy"
else
  bintrans_fail "redpanda not ready"
fi

running="$(bintrans_compose --profile messaging --profile read-model ps --format '{{.Service}}' 2>/dev/null | sort -u || true)"

for svc in "${expected_services[@]}"; do
  if echo "${running}" | grep -qx "${svc}"; then
    echo "OK: service running: ${svc}"
  else
    bintrans_fail "expected runtime service not running: ${svc}"
  fi
done

for forbidden in "${forbidden_services[@]}"; do
  if echo "${running}" | grep -qx "${forbidden}"; then
    bintrans_fail "unexpected service running: ${forbidden}"
  fi
done

curl_check() {
  local label="$1" url="$2"
  local code
  code="$(curl -fsS -o /dev/null -w '%{http_code}' --connect-timeout 5 --max-time 15 "${url}" 2>/dev/null || echo 000)"
  if [[ "${code}" =~ ^[23][0-9]{2}$ ]]; then
    echo "OK: ${label} HTTP ${code} (${url})"
  else
    bintrans_fail "${label} unhealthy (HTTP ${code}) at ${url}"
  fi
}

curl_check "api-gateway health" "${GATEWAY_URL%/}/health"
curl_check "control-tower-read-model health" "${CTRM_URL%/}/health"

echo "NOTE: authenticated endpoints require JWT_TOKEN (observation only); not required for container startup"
echo "bintrans-ct-staging-runtime-health: PASS"
