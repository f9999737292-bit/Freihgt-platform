#!/usr/bin/env bash
# BINTRANS dedicated staging — start foundation ONLY (postgres + redpanda).
# Does NOT start runtime services, observability, or migration.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

bintrans_require_env_file

echo "Starting foundation only: postgres, redpanda"
echo "Project: ${BINTRANS_COMPOSE_PROJECT}"

BINTRANS_INCLUDE_SHADOW=0 BINTRANS_INCLUDE_IMAGES=0 \
  bintrans_compose --profile messaging up -d postgres redpanda

echo "Foundation services requested. Run bintrans_ct_staging_foundation_health.sh to verify."
