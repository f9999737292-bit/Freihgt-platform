#!/usr/bin/env bash
# Local mocked proof that the runtime_up foundation gate fails closed.
# Does not start containers, pull images, or contact staging.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
runtime="${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_up.sh"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_up.sh
source "${runtime}"

fail() { echo "foundation-guard-selfcheck: $*" >&2; exit 1; }

expect_fail() {
  local label="$1" name="$2" cid="$3" mock_status="$4" mock_health="$5" needle="$6"
  local err rc
  set +e
  err="$(
    {
      docker() {
        local fmt=""
        while [[ $# -gt 0 ]]; do
          case "$1" in
            -f)
              fmt="$2"
              shift 2
              ;;
            *)
              shift
              ;;
          esac
        done
        if [[ "${fmt}" == *Health* ]]; then
          printf '%s\n' "${mock_health}"
        else
          printf '%s\n' "${mock_status}"
        fi
      }
      bintrans_compose() { echo "foundation mutation invoked" >&2; return 97; }
      bintrans_require_foundation_running_healthy "${name}" "${cid}"
    } 2>&1
  )"
  rc=$?
  set -e
  [[ "${rc}" -ne 0 ]] || fail "${label} expected non-zero exit"
  [[ "${err}" == *"${needle}"* ]] || fail "${label} missing '${needle}' in: ${err}"
  [[ "${err}" != *"foundation mutation invoked"* ]] || fail "${label} mutated foundation"
  echo "${label}=PASS"
}

expect_fail TEST_FOUNDATION_PRECHECK_POSTGRES_MISSING postgres "" running healthy \
  "foundation postgres container is missing"
expect_fail TEST_FOUNDATION_PRECHECK_POSTGRES_RUNNING postgres pg-cid exited healthy \
  "foundation postgres must be running and healthy (status="
expect_fail TEST_FOUNDATION_PRECHECK_POSTGRES_HEALTHY postgres pg-cid running unhealthy \
  "foundation postgres must be running and healthy (health="
expect_fail TEST_FOUNDATION_PRECHECK_REDPANDA_MISSING redpanda "" running healthy \
  "foundation redpanda container is missing"
expect_fail TEST_FOUNDATION_PRECHECK_REDPANDA_RUNNING redpanda rp-cid exited healthy \
  "foundation redpanda must be running and healthy (status="
expect_fail TEST_FOUNDATION_PRECHECK_REDPANDA_HEALTHY redpanda rp-cid running unhealthy \
  "foundation redpanda must be running and healthy (health="

set +e
ready_err="$(
  {
    docker() {
      local fmt=""
      while [[ $# -gt 0 ]]; do
        case "$1" in
          -f)
            fmt="$2"
            shift 2
            ;;
          *)
            shift
            ;;
        esac
      done
      if [[ "${fmt}" == *Health* ]]; then
        printf '%s\n' healthy
      else
        printf '%s\n' running
      fi
    }
    bintrans_compose() { echo "foundation mutation invoked" >&2; return 97; }
    bintrans_require_foundation_running_healthy postgres pg-cid
    bintrans_require_foundation_running_healthy redpanda rp-cid
  } 2>&1
)"
ready_rc=$?
set -e
[[ "${ready_rc}" -eq 0 ]] || fail "running+healthy foundation must pass: ${ready_err}"
[[ "${ready_err}" != *"foundation mutation invoked"* ]] || fail "ready gate mutated foundation"

set +e
exec_err="$(BINTRANS_STAGING_ENV="${ROOT}/scripts/ops/bintrans_ct_staging/fixtures/missing-foundation-guard.env" bash "${runtime}" 2>&1)"
exec_rc=$?
set -e
[[ "${exec_rc}" -ne 0 ]] || fail "executed runtime_up must fail before rollout when env is absent"
[[ "${exec_err}" == *"protected env missing"* ]] || fail "executed runtime_up did not fail at env validation"
[[ "${exec_err}" != *"Starting BINTRANS runtime"* ]] || fail "executed runtime_up reached compose up"

echo "FOUNDATION_FAIL_CLOSED_TESTS=PASS"
echo "bintrans-ct-staging-runtime-up-foundation-guard-selfcheck: PASS"
