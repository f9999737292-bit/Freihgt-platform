#!/usr/bin/env bash
# BINTRANS observability health verification (read-only; no secret echo).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

bintrans_require_env_file

PROM_PORT="$(bintrans_env_value PROMETHEUS_PORT)"
PROM_PORT="${PROM_PORT:-9090}"
GRAF_PORT="$(bintrans_env_value GRAFANA_PORT)"
GRAF_PORT="${GRAF_PORT:-3001}"
PROM_URL="http://127.0.0.1:${PROM_PORT}"
GRAF_URL="http://127.0.0.1:${GRAF_PORT}"

echo "=== BINTRANS observability health check ==="

gw_cid="$(bintrans_compose --profile messaging --profile read-model ps -q api-gateway 2>/dev/null | head -n1)"
[[ -n "${gw_cid}" ]] || bintrans_fail "api-gateway must be running before observability health check"

running="$(bintrans_compose --profile messaging --profile read-model --profile observability ps --format '{{.Service}}' 2>/dev/null | sort -u || true)"

for svc in prometheus grafana; do
  echo "${running}" | grep -qx "${svc}" \
    || bintrans_fail "expected observability service not running: ${svc}"
  echo "OK: service running: ${svc}"
done

for forbidden in migrate postgres redpanda; do
  echo "${running}" | grep -qx "${forbidden}" \
    && bintrans_fail "unexpected service in observability ps: ${forbidden}" || true
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

curl_check "prometheus" "${PROM_URL%/}/-/healthy"
curl_check "grafana" "${GRAF_URL%/}/api/health"

echo "bintrans-ct-staging-observability-health: PASS"
