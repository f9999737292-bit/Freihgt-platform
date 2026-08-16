#!/usr/bin/env bash
# Static self-check for foundation startup script contract.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
target="${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_foundation_up.sh"

[[ -f "${target}" ]] || { echo "missing ${target}" >&2; exit 1; }

fail() { echo "foundation-up-selfcheck: $*" >&2; exit 1; }

compose_line="$(
  grep 'bintrans_compose.*up -d' "${target}" \
    | grep -v '^[[:space:]]*#' \
    | grep -v '^[[:space:]]*echo' \
    | tail -n1 \
    || true
)"
[[ -n "${compose_line}" ]] || fail "must contain bintrans_compose up -d invocation"

echo "${compose_line}" | grep -e '--no-deps' >/dev/null 2>&1 || fail "must include --no-deps"
echo "${compose_line}" | grep -e '--profile messaging' >/dev/null 2>&1 || fail "must use messaging profile"
echo "${compose_line}" | grep -q 'postgres' || fail "must include postgres"
echo "${compose_line}" | grep -q 'redpanda' || fail "must include redpanda"
echo "${compose_line}" | grep -q 'postgres redpanda' || fail "must target postgres redpanda only"

forbidden_services=(
  migrate
  api-gateway
  identity-service
  company-service
  transport-order-service
  rfx-service
  shipment-service
  document-service
  billing-register-service
  low-code-service
  control-tower-read-model-service
  prometheus
  grafana
)
for svc in "${forbidden_services[@]}"; do
  if echo "${compose_line}" | grep -q "${svc}"; then
    fail "compose up line must not include ${svc}"
  fi
done

# Guard must validate compose_line, not whole-script grep (avoids false positives).
if grep -q 'grep -Eq.*up -d.*migrate' "${target}"; then
  fail "must not use whole-script up -d.*migrate grep guard (false-positive risk)"
fi
if ! grep -q 'compose_line=' "${target}"; then
  fail "must extract compose_line for structured validation"
fi

echo "bintrans-ct-staging-foundation-up-selfcheck: PASS"
