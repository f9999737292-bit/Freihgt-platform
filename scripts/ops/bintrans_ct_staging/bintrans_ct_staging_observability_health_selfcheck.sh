#!/usr/bin/env bash
# Offline self-check for observability health service-set contract (no live containers).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

target="${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_observability_health.sh"
[[ -f "${target}" ]] || { echo "missing ${target}" >&2; exit 1; }

fail() { echo "observability-health-selfcheck: $*" >&2; exit 1; }

full_stack="$(printf '%s\n' \
  postgres redpanda \
  identity-service company-service transport-order-service rfx-service \
  shipment-service document-service billing-register-service low-code-service \
  control-tower-read-model-service api-gateway \
  prometheus grafana | sort -u)"

missing_prometheus="$(printf '%s\n' \
  postgres redpanda \
  identity-service company-service transport-order-service rfx-service \
  shipment-service document-service billing-register-service low-code-service \
  control-tower-read-model-service api-gateway \
  grafana | sort -u)"

missing_grafana="$(printf '%s\n' \
  postgres redpanda \
  identity-service company-service transport-order-service rfx-service \
  shipment-service document-service billing-register-service low-code-service \
  control-tower-read-model-service api-gateway \
  prometheus | sort -u)"

with_migrate="$(printf '%s\n%s\n' "${full_stack}" migrate | sort -u)"
with_unknown="$(printf '%s\n%s\n' "${full_stack}" rogue-service | sort -u)"

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

expect_pass "full-stack with foundation and runtime" "${full_stack}"
expect_fail "migrate present" "${with_migrate}"
expect_fail "unknown service present" "${with_unknown}"

if ! ( bintrans_assert_services_listed "${full_stack}" \
  "${bintrans_observability_service_names[@]}" ) >/dev/null 2>&1; then
  fail "full-stack observability required: expected PASS"
fi

if ( bintrans_assert_services_listed "${missing_prometheus}" \
  "${bintrans_observability_service_names[@]}" ) >/dev/null 2>&1; then
  fail "missing prometheus: expected FAIL"
fi

if ( bintrans_assert_services_listed "${missing_grafana}" \
  "${bintrans_observability_service_names[@]}" ) >/dev/null 2>&1; then
  fail "missing grafana: expected FAIL"
fi

grep -q 'bintrans_validate_project_service_names' "${target}" \
  || fail "observability health must use approved project service validation"
grep -q 'bintrans_assert_services_listed' "${target}" \
  || fail "observability health must assert prometheus/grafana explicitly"
if grep -q 'unexpected service in observability ps: postgres' "${target}" \
  || grep -q 'for forbidden in migrate postgres redpanda' "${target}"; then
  fail "observability health must not treat foundation/runtime as forbidden in project ps"
fi

grep -q 'api-gateway must be running' "${target}" \
  || fail "observability health must gate on api-gateway"

echo "bintrans-ct-staging-observability-health-selfcheck: PASS"
