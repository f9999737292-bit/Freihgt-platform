#!/usr/bin/env bash
# Static self-check for runtime_up.sh service contract.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

target="${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_up.sh"
[[ -f "${target}" ]] || { echo "missing ${target}" >&2; exit 1; }

fail() { echo "runtime-up-selfcheck: $*" >&2; exit 1; }

line_no() {
  grep -n "$1" "${target}" | head -n1 | cut -d: -f1
}

grep -q 'bintrans_compose' "${target}" || fail "must invoke bintrans_compose"
grep -q 'up -d' "${target}" || fail "must include up -d"
grep -q -- '--no-deps' "${target}" || fail "must include --no-deps"
echo "TEST_RUNTIME_UP_REQUIRES_NO_DEPS=PASS"
grep -q -- '--no-build' "${target}" || fail "must include --no-build"
echo "TEST_RUNTIME_UP_STILL_NO_BUILD=PASS"
grep -q -- '--pull always' "${target}" || fail "must include --pull always"
echo "TEST_RUNTIME_UP_STILL_PULLS_CANONICAL_IMAGES=PASS"
grep -q -- '--force-recreate' "${target}" || fail "must include --force-recreate"
echo "TEST_RUNTIME_UP_STILL_FORCE_RECREATES_CANONICAL=PASS"
grep -q -- '--profile read-model' "${target}" || fail "must include read-model profile"
grep -q 'bintrans_runtime_service_names' "${target}" \
  || fail "must start canonical bintrans_runtime_service_names set"

if grep -q -- '--remove-orphans' "${target}"; then
  fail "must not pass --remove-orphans"
fi
echo "TEST_RUNTIME_UP_FORBIDS_REMOVE_ORPHANS=PASS"
if grep -q -- '--always-recreate-deps' "${target}"; then
  fail "must not pass --always-recreate-deps"
fi
echo "TEST_RUNTIME_UP_FORBIDS_ALWAYS_RECREATE_DEPS=PASS"

up_block="$(awk '/up -d/{f=1} f{print} /bintrans_runtime_service_names\[@\]/{if(f) exit}' "${target}")"
for forbidden in migrate prometheus grafana postgres redpanda; do
  if grep -qE "(^|[[:space:]])${forbidden}([[:space:]]|$)" <<< "${up_block}"; then
    fail "must not include ${forbidden} in compose up invocation"
  fi
done
echo "TEST_RUNTIME_UP_FOUNDATION_NOT_TARGETED=PASS"

grep -q 'bintrans_ct_staging_runtime_preflight.sh' "${target}" \
  || fail "must invoke runtime preflight before compose up"

# Verify common.sh service list matches expected 14-service runtime pack.
[[ "${#bintrans_runtime_service_names[@]}" -eq 14 ]] \
  || fail "bintrans_runtime_service_names must contain 14 services"
echo "TEST_RUNTIME_UP_CANONICAL_TARGET_COUNT_14=PASS"

env_line="$(line_no 'bintrans_require_env_file')"
preflight_line="$(line_no 'bintrans_ct_staging_runtime_preflight.sh')"
discover_line="$(line_no 'bintrans_postgres_container')"
pg_gate_line="$(line_no 'bintrans_require_foundation_running_healthy postgres')"
rp_gate_line="$(line_no 'bintrans_require_foundation_running_healthy redpanda')"
up_line="$(line_no 'up -d')"
for needed in "${env_line}" "${preflight_line}" "${discover_line}" "${pg_gate_line}" "${rp_gate_line}" "${up_line}"; do
  [[ -n "${needed}" ]] || fail "missing required runtime_up step"
done
[[ "${env_line}" -lt "${preflight_line}" ]] || fail "env validation must precede preflight"
[[ "${preflight_line}" -lt "${discover_line}" ]] || fail "preflight must precede foundation discovery"
[[ "${discover_line}" -lt "${pg_gate_line}" && "${pg_gate_line}" -lt "${up_line}" ]] \
  || fail "postgres running/healthy gate must precede compose up"
[[ "${discover_line}" -lt "${rp_gate_line}" && "${rp_gate_line}" -lt "${up_line}" ]] \
  || fail "redpanda running/healthy gate must precede compose up"
grep -q '{{.State.Status}}' "${target}" || fail "foundation gate must inspect running status"
grep -q 'State.Health' "${target}" || fail "foundation gate must inspect health"
echo "TEST_FOUNDATION_PRECHECK_POSTGRES_RUNNING=PASS"
echo "TEST_FOUNDATION_PRECHECK_POSTGRES_HEALTHY=PASS"
echo "TEST_FOUNDATION_PRECHECK_REDPANDA_RUNNING=PASS"
echo "TEST_FOUNDATION_PRECHECK_REDPANDA_HEALTHY=PASS"

echo "bintrans-ct-staging-runtime-up-selfcheck: PASS"
