#!/usr/bin/env bash
# Static self-check for foundation startup script contract.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
target="${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_foundation_up.sh"

[[ -f "${target}" ]] || { echo "missing ${target}" >&2; exit 1; }

fail() { echo "foundation-up-selfcheck: $*" >&2; exit 1; }

compose_line="$(grep 'bintrans_compose.*up -d' "${target}" || true)"
[[ -n "${compose_line}" ]] || fail "must contain bintrans_compose up -d invocation"

echo "${compose_line}" | grep -e '--no-deps' >/dev/null 2>&1 || fail "must include --no-deps"
echo "${compose_line}" | grep -q 'postgres redpanda' || fail "must target postgres redpanda only"
echo "${compose_line}" | grep -e '--profile messaging' >/dev/null 2>&1 || fail "must use messaging profile"

for forbidden in api-gateway identity-service prometheus grafana control-tower-read-model-service migrate; do
  if echo "${compose_line}" | grep -q "${forbidden}"; then
    fail "compose up line must not include ${forbidden}"
  fi
done

echo "bintrans-ct-staging-foundation-up-selfcheck: PASS"
