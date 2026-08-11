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

running="$(bintrans_compose_running_service_names --profile messaging --profile read-model --profile observability)"

bintrans_validate_project_service_names "${running}"
bintrans_assert_services_listed "${running}" "${bintrans_observability_service_names[@]}"

while IFS= read -r line; do
  [[ -z "${line}" ]] && continue
  svc="${line%%|*}"
  state="${line#*|}"; state="${state%%|*}"
  health="${line##*|}"
  if [[ "${state}" != "running" ]]; then
    bintrans_fail "observability service not running: ${svc} (state=${state})"
  fi
  if [[ -n "${health}" && "${health}" == "unhealthy" ]]; then
    bintrans_fail "observability service unhealthy: ${svc}"
  fi
done < <(
  bintrans_compose --profile messaging --profile read-model --profile observability ps \
    --format '{{.Service}}|{{.State}}|{{.Health}}' 2>/dev/null \
    | grep -E '^(prometheus|grafana)\|'
)

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
