#!/usr/bin/env bash
# BINTRANS dedicated staging — start foundation ONLY (postgres + redpanda).
# Does NOT start runtime services, observability, or migration.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

bintrans_require_env_file

# Validate the actual compose invocation line — not arbitrary script source text.
self="$(readlink -f "${BASH_SOURCE[0]}" 2>/dev/null || realpath "${BASH_SOURCE[0]}" 2>/dev/null || echo "${BASH_SOURCE[0]}")"
compose_line="$(
  grep 'bintrans_compose.*up -d' "${self}" \
    | grep -v '^[[:space:]]*#' \
    | grep -v '^[[:space:]]*echo' \
    | tail -n1 \
    || true
)"
[[ -n "${compose_line}" ]] || bintrans_fail "foundation script missing bintrans_compose up -d invocation"

echo "${compose_line}" | grep -e '--no-deps' >/dev/null 2>&1 \
  || bintrans_fail "foundation compose command missing --no-deps"
echo "${compose_line}" | grep -e '--profile messaging' >/dev/null 2>&1 \
  || bintrans_fail "foundation compose command missing --profile messaging"
echo "${compose_line}" | grep -q 'postgres redpanda' \
  || bintrans_fail "foundation compose command must target postgres redpanda only"

forbidden_services=(
  migrate
  api-gateway
  identity-service
  company-service
  transport-order-service
  rfx-service
  shipment-service
  document-service
  billing-register-service
  low-code-service
  control-tower-read-model-service
  prometheus
  grafana
)
for svc in "${forbidden_services[@]}"; do
  if echo "${compose_line}" | grep -q "${svc}"; then
    bintrans_fail "foundation compose command must not include ${svc}"
  fi
done

echo "Starting foundation only: postgres, redpanda"
echo "Project: ${BINTRANS_COMPOSE_PROJECT}"
echo "Exact command:"
echo "  docker compose --env-file ${BINTRANS_STAGING_ENV} -p ${BINTRANS_COMPOSE_PROJECT} \\"
echo "    -f ${BINTRANS_COMPOSE_BASE} -f ${BINTRANS_COMPOSE_BINTRANS} \\"
echo "    --profile messaging up -d --no-deps postgres redpanda"

BINTRANS_INCLUDE_SHADOW=0 BINTRANS_INCLUDE_IMAGES=0 \
  bintrans_compose --profile messaging up -d --no-deps postgres redpanda

echo "Foundation services requested. Run bintrans_ct_staging_foundation_health.sh to verify."
