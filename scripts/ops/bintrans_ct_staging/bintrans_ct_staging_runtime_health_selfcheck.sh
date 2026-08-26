#!/usr/bin/env bash
# Offline self-check for runtime health service-set contract (no live containers).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

target="${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_health.sh"
[[ -f "${target}" ]] || { echo "missing ${target}" >&2; exit 1; }

fail() { echo "runtime-health-selfcheck: $*" >&2; exit 1; }

runtime_only="$(printf '%s\n' \
  postgres redpanda \
  "${bintrans_runtime_service_names[@]}" | sort -u)"

full_stack="$(printf '%s\n' \
  "${bintrans_full_stack_service_names[@]}" | sort -u)"

missing_runtime="$(printf '%s\n' \
  postgres redpanda \
  identity-service company-service transport-order-service rfx-service \
  shipment-service document-service billing-register-service low-code-service \
  payment-service contract-rate-service freight-cost-service \
  control-tower-read-model-service | sort -u)"

with_migrate="$(printf '%s\n%s\n' "${runtime_only}" migrate | sort -u)"
with_unknown="$(printf '%s\n%s\n' "${runtime_only}" rogue-service | sort -u)"

expect_pass() {
  local label="$1" running="$2"
  if ! ( bintrans_validate_project_service_names "${running}" ) >/dev/null 2>&1; then
    fail "${label}: expected PASS"
  fi
}

expect_fail() {
  local label="$1" running="$2"
  if ( bintrans_validate_project_service_names "${running}" ) >/dev/null 2>&1; then
    fail "${label}: expected FAIL"
  fi
}

expect_pass "runtime-only approved set" "${runtime_only}"
expect_pass "full-stack approved set" "${full_stack}"
expect_fail "migrate present" "${with_migrate}"
expect_fail "unknown service present" "${with_unknown}"

if ! ( bintrans_assert_services_listed "${runtime_only}" \
  "${bintrans_foundation_service_names[@]}" "${bintrans_runtime_service_names[@]}" ) >/dev/null 2>&1; then
  fail "runtime-only required services: expected PASS"
fi

if ( bintrans_assert_services_listed "${missing_runtime}" \
  "${bintrans_foundation_service_names[@]}" "${bintrans_runtime_service_names[@]}" ) >/dev/null 2>&1; then
  fail "missing api-gateway: expected FAIL"
fi

grep -q 'bintrans_validate_project_service_names' "${target}" \
  || fail "runtime health must use approved project service validation"
grep -q 'bintrans_assert_services_listed' "${target}" \
  || fail "runtime health must assert required services explicitly"
if grep -q 'unexpected service running: prometheus' "${target}" \
  || grep -q 'forbidden_services=(migrate prometheus grafana)' "${target}"; then
  fail "runtime health must not treat prometheus/grafana as forbidden when present"
fi

[[ "${#bintrans_full_stack_service_names[@]}" -eq 17 ]] \
  || fail "full stack must contain 17 approved services"

echo "bintrans-ct-staging-runtime-health-selfcheck: PASS"
