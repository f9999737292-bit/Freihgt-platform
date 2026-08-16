#!/usr/bin/env bash
# BINTRANS dedicated staging — start runtime shadow services ONLY.
# Requires foundation (postgres + redpanda) already healthy. Does NOT start migrate/observability.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

bintrans_require_env_file

echo "=== BINTRANS runtime preflight (required before start) ==="
"${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_preflight.sh"

pg_cid="$(bintrans_postgres_container)"
rp_cid="$(bintrans_redpanda_container)"
[[ -n "${pg_cid}" && -n "${rp_cid}" ]] \
  || bintrans_fail "foundation must be running (postgres + redpanda) before runtime_up"

echo "Starting BINTRANS runtime shadow services (foundation unchanged)"
echo "Project: ${BINTRANS_COMPOSE_PROJECT}"

BINTRANS_INCLUDE_SHADOW=1 BINTRANS_INCLUDE_IMAGES=1 \
  bintrans_compose --profile messaging --profile read-model \
  up -d --no-build \
  "${bintrans_runtime_service_names[@]}"

echo "Runtime services requested. Run bintrans_ct_staging_runtime_health.sh to verify."
