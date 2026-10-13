#!/usr/bin/env bash
# BINTRANS dedicated staging — start runtime shadow services ONLY.
# Requires foundation (postgres + redpanda) already running and healthy.
# Does not start, pull, repair, or recreate foundation. Does NOT start migrate/observability.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

# Inspect only. Does not start, pull, restart, or recreate the container.
bintrans_require_foundation_running_healthy() {
  local name="$1" cid="$2" status health
  if [[ -z "${cid}" ]]; then
    bintrans_fail "foundation ${name} container is missing"
  fi
  status="$(docker inspect -f '{{.State.Status}}' "${cid}")"
  if [[ "${status}" != "running" ]]; then
    bintrans_fail "foundation ${name} must be running and healthy (status=${status})"
  fi
  health="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "${cid}")"
  if [[ "${health}" != "healthy" ]]; then
    bintrans_fail "foundation ${name} must be running and healthy (health=${health})"
  fi
}

# Sourcing this file loads the foundation gate for local tests and does not roll out.
if [[ "${BASH_SOURCE[0]}" != "$0" ]]; then
  return 0
fi

bintrans_require_env_file

echo "=== BINTRANS runtime preflight (required before start) ==="
"${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_preflight.sh"

pg_cid="$(bintrans_postgres_container)"
rp_cid="$(bintrans_redpanda_container)"
bintrans_require_foundation_running_healthy postgres "${pg_cid}"
bintrans_require_foundation_running_healthy redpanda "${rp_cid}"

echo "Starting BINTRANS runtime shadow services (foundation unchanged)"
echo "Project: ${BINTRANS_COMPOSE_PROJECT}"

BINTRANS_INCLUDE_SHADOW=1 BINTRANS_INCLUDE_IMAGES=1 \
  bintrans_compose --profile messaging --profile read-model \
  up -d \
  --no-deps \
  --no-build \
  --pull always \
  --force-recreate \
  "${bintrans_runtime_service_names[@]}"

echo "Runtime services requested. Run bintrans_ct_staging_runtime_health.sh to verify."
