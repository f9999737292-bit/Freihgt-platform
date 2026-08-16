#!/usr/bin/env bash
# Static self-check for observability_up.sh contract.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
target="${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_observability_up.sh"
[[ -f "${target}" ]] || { echo "missing ${target}" >&2; exit 1; }

fail() { echo "observability-up-selfcheck: $*" >&2; exit 1; }

grep -q 'bintrans_compose' "${target}" || fail "must invoke bintrans_compose"
grep -q '\-\-profile observability' "${target}" || fail "must include observability profile"
grep -q 'up -d' "${target}" || fail "must include up -d"
grep -q 'prometheus' "${target}" || fail "must include prometheus"
grep -q 'grafana' "${target}" || fail "must include grafana"
grep -q '\-\-no-build' "${target}" || fail "must include --no-build"

for forbidden in migrate postgres redpanda; do
  grep -qE "up -d.*${forbidden}|${forbidden}.*up -d" "${target}" && fail "must not start ${forbidden}"
done

grep -q 'api-gateway must be running' "${target}" || fail "must gate on api-gateway running"

echo "bintrans-ct-staging-observability-up-selfcheck: PASS"
