#!/usr/bin/env bash
# BINTRANS dedicated staging — start foundation ONLY (postgres + redpanda).
# Does NOT start runtime services, observability, or migration.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

bintrans_require_env_file

# Static self-check: this script must remain foundation-only.
self="$(readlink -f "${BASH_SOURCE[0]}" 2>/dev/null || realpath "${BASH_SOURCE[0]}" 2>/dev/null || echo "${BASH_SOURCE[0]}")"
if ! grep -e '--no-deps' "${self}" >/dev/null 2>&1; then
  bintrans_fail "foundation script missing required --no-deps guard"
fi
if grep -Eq 'up -d.*(api-gateway|identity-service|prometheus|grafana|control-tower-read-model-service|migrate)' "${self}"; then
  bintrans_fail "foundation script must not start runtime/observability/migrate services"
fi

echo "Starting foundation only: postgres, redpanda"
echo "Project: ${BINTRANS_COMPOSE_PROJECT}"
echo "Exact command:"
echo "  docker compose --env-file ${BINTRANS_STAGING_ENV} -p ${BINTRANS_COMPOSE_PROJECT} \\"
echo "    -f ${BINTRANS_COMPOSE_BASE} -f ${BINTRANS_COMPOSE_BINTRANS} \\"
echo "    --profile messaging up -d --no-deps postgres redpanda"

BINTRANS_INCLUDE_SHADOW=0 BINTRANS_INCLUDE_IMAGES=0 \
  bintrans_compose --profile messaging up -d --no-deps postgres redpanda

echo "Foundation services requested. Run bintrans_ct_staging_foundation_health.sh to verify."
