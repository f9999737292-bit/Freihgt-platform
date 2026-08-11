#!/usr/bin/env bash
# Static self-check for runtime_up.sh service contract.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

target="${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_up.sh"
[[ -f "${target}" ]] || { echo "missing ${target}" >&2; exit 1; }

fail() { echo "runtime-up-selfcheck: $*" >&2; exit 1; }

grep -q 'bintrans_compose' "${target}" || fail "must invoke bintrans_compose"
grep -q 'up -d' "${target}" || fail "must include up -d"
grep -q '\-\-no-build' "${target}" || fail "must include --no-build"
grep -q '\-\-profile read-model' "${target}" || fail "must include read-model profile"
grep -q 'bintrans_runtime_service_names' "${target}" \
  || fail "must start canonical bintrans_runtime_service_names set"

for forbidden in migrate prometheus grafana postgres redpanda; do
  if grep -qE "up -d[^\"]*${forbidden}" "${target}"; then
    fail "must not include ${forbidden} in compose up invocation"
  fi
done

grep -q 'bintrans_ct_staging_runtime_preflight.sh' "${target}" \
  || fail "must invoke runtime preflight before compose up"

# Verify common.sh service list matches expected 10-service runtime pack.
[[ "${#bintrans_runtime_service_names[@]}" -eq 10 ]] \
  || fail "bintrans_runtime_service_names must contain 10 services"

echo "bintrans-ct-staging-runtime-up-selfcheck: PASS"
