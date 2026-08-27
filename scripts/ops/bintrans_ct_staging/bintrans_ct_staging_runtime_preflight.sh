#!/usr/bin/env bash
# BINTRANS dedicated staging — runtime deploy preflight (no containers started).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

echo "=== BINTRANS dedicated staging runtime preflight ==="

bintrans_require_env_file

required_files=(
  "${BINTRANS_COMPOSE_BASE}"
  "${BINTRANS_COMPOSE_BINTRANS}"
  "${BINTRANS_COMPOSE_POOL}"
  "${BINTRANS_COMPOSE_SHADOW}"
  "${BINTRANS_COMPOSE_IMAGES}"
)
for f in "${required_files[@]}"; do
  [[ -f "${f}" ]] || bintrans_fail "missing required file: ${f}"
done

bintrans_require_runtime_env_contract
echo "OK: runtime safety env fields + JWT_SECRET + POSTGRES_PASSWORD"

bintrans_validate_pool_env_contract
echo "OK: staging pool env contract (DB_MAX_OPEN_CONNS/DB_MAX_IDLE_CONNS/DB_MAX_OPEN_LIGHT)"

bintrans_validate_all_runtime_digest_images
echo "OK: all runtime services use digest-pinned registry references"

cohort_approved="$(bintrans_env_value COHORT_APPROVED || echo NO)"
if [[ "${cohort_approved}" != "YES" ]]; then
  echo "NOTE: COHORT_APPROVED!=YES — runtime smoke may proceed; Day 0 observation remains blocked"
fi

render_runtime="$(mktemp)"
render_observability="$(mktemp)"
trap 'rm -f "${render_runtime}" "${render_observability}"' EXIT

BINTRANS_INCLUDE_SHADOW=1 BINTRANS_INCLUDE_IMAGES=1 \
  bintrans_compose --profile messaging --profile read-model config > "${render_runtime}"

BINTRANS_INCLUDE_SHADOW=1 BINTRANS_INCLUDE_IMAGES=1 \
  bintrans_compose --profile messaging --profile read-model --profile observability config > "${render_observability}"

gateway_mode="$(bintrans_extract_gateway_mode "${render_runtime}")"
case "${gateway_mode}" in
  shadow) ;;
  primary) bintrans_fail "PRIMARY mode is forbidden on BINTRANS staging" ;;
  disabled|"") bintrans_fail "effective api-gateway CONTROL_TOWER_READ_MODEL_MODE must be shadow (found: ${gateway_mode:-<unset>})" ;;
  *) bintrans_fail "unexpected api-gateway CONTROL_TOWER_READ_MODEL_MODE: ${gateway_mode}" ;;
esac
echo "OK: effective api-gateway mode=shadow"

if grep -q 'JWT_SECRET: dev_secret_change_me' "${render_runtime}"; then
  bintrans_fail "effective runtime config must not use dev JWT_SECRET default"
fi
if grep -qE 'JWT_SECRET: ""' "${render_runtime}"; then
  bintrans_fail "effective runtime JWT_SECRET must not be empty"
fi
if ! grep -q 'JWT_SECRET:' "${render_runtime}"; then
  bintrans_fail "effective runtime config must include JWT_SECRET from protected env"
fi
echo "OK: JWT_SECRET externalized in effective runtime config"

bintrans_check_no_wide_bind "${render_runtime}" "runtime"
bintrans_check_no_wide_bind "${render_observability}" "observability"
echo "OK: runtime port isolation verified"

bintrans_validate_rendered_pool_budget "${render_runtime}"
echo "OK: rendered aggregate PostgreSQL pool budget=$(bintrans_calculate_rendered_aggregate_pool_budget "${render_runtime}")"

echo "--- Published ports: runtime shadow ---"
grep -B3 'published:' "${render_runtime}" | grep -E '^(  [a-z0-9-]+:|        published:|        host_ip:)' || echo "(none)"
echo "--- Published ports: observability ---"
grep -B3 'published:' "${render_observability}" | grep -E '^(  [a-z0-9-]+:|        published:|        host_ip:)' || echo "(none)"

echo "bintrans-ct-staging-runtime-preflight: PASS"
