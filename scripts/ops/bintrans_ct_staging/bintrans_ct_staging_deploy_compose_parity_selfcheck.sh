#!/usr/bin/env bash
# Verify deploy/runtime helpers use canonical bintrans_compose stack (R4-OPS-003).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

fail() { echo "deploy-compose-parity-selfcheck: $*" >&2; exit 1; }

ops_dir="${ROOT}/scripts/ops/bintrans_ct_staging"
required=(
  bintrans_ct_staging_runtime_up.sh
  bintrans_ct_staging_runtime_health.sh
  bintrans_ct_staging_runtime_preflight.sh
  bintrans_ct_staging_observability_up.sh
  bintrans_ct_staging_observability_health.sh
  bintrans_ct_staging_foundation_up.sh
  bintrans_ct_staging_foundation_health.sh
  bintrans_ct_staging_migrate_gate.sh
  bintrans_ct_staging_preflight.sh
)

for script in "${required[@]}"; do
  target="${ops_dir}/${script}"
  [[ -f "${target}" ]] || fail "missing ${script}"
  grep -q 'bintrans_compose' "${target}" || fail "${script} must invoke bintrans_compose"
done

# bintrans_compose must include pool overlay by default.
grep -q 'BINTRANS_COMPOSE_POOL' "${ops_dir}/bintrans_ct_staging_common.sh" \
  || fail "common.sh must reference pool overlay"
grep -q 'BINTRANS_INCLUDE_POOL:-1' "${ops_dir}/bintrans_ct_staging_common.sh" \
  || fail "pool overlay must be included by default"

fixture_env="${ops_dir}/fixtures/compose-static.env"
[[ -f "${fixture_env}" ]] || fail "compose fixture missing"
export BINTRANS_STAGING_ENV="${fixture_env}"
render="$(mktemp)"
trap 'rm -f "${render}"' EXIT
BINTRANS_INCLUDE_SHADOW=1 BINTRANS_INCLUDE_IMAGES=1 \
  bintrans_compose --profile messaging --profile read-model config > "${render}"

budget="$(bintrans_calculate_rendered_aggregate_pool_budget "${render}")"
expected="$(bintrans_staging_expected_aggregate_pool_budget)"
[[ "${budget}" -eq "${expected}" ]] \
  || fail "rendered pool budget ${budget} != ${expected}"

echo "RENDERED_POOL_BUDGET=${budget}"
echo "bintrans-ct-staging-deploy-compose-parity-selfcheck: PASS"
