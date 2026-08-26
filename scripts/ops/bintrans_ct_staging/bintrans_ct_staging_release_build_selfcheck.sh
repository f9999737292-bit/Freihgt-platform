#!/usr/bin/env bash
# Static self-check for canonical BINTRANS staging release build contract.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

RELEASE_BUILD="${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_release_build.sh"
MAKEFILE="${ROOT}/Makefile"
OCI_LABELS="${ROOT}/infrastructure/docker/bintrans-oci-provenance.labels"
FIXTURE_SHA="$(git -C "${ROOT}" rev-parse HEAD)"
FIXTURE_TAG="git-$(git -C "${ROOT}" rev-parse --short=7 HEAD)"
EXPECTED_OCI_SOURCE="https://github.com/f9999737292-bit/Freihgt-platform"

fail() { echo "release-build-selfcheck: $*" >&2; exit 1; }

assert_fail() {
  local label="$1"
  shift
  if "$@"; then
    fail "${label}: expected FAIL"
  fi
  echo "OK: ${label} rejected"
}

assert_pass() {
  local label="$1"
  shift
  if ! "$@"; then
    fail "${label}: expected PASS"
  fi
  echo "OK: ${label}"
}

[[ -f "${RELEASE_BUILD}" ]] || fail "missing release build script"
[[ -x "${RELEASE_BUILD}" ]] || chmod +x "${RELEASE_BUILD}"

mapfile -t CANONICAL < <(bintrans_release_build_services)
[[ "${#CANONICAL[@]}" -eq 13 ]] || fail "A: canonical runtime services must be 13"
echo "OK: A_CANONICAL_RUNTIME_SERVICES=13"

dry_out="$(BINTRANS_RELEASE_GIT_SHA="${FIXTURE_SHA}" BINTRANS_IMAGE_VERSION="${FIXTURE_TAG}" \
  "${RELEASE_BUILD}" --dry-run 2>&1)" || fail "release build dry-run failed"

mapfile -t BUILD_SERVICES < <(
  printf '%s\n' "${dry_out}" \
    | grep -E '^DRY_RUN:' \
    | sed 's/.*--progress=plain //' \
    | awk '{print $1}'
)
[[ "${#BUILD_SERVICES[@]}" -eq 13 ]] || fail "B: release build must render 13 services (found ${#BUILD_SERVICES[@]})"
echo "OK: B_RELEASE_BUILD_SERVICES=13"

canonical_sorted="$(printf '%s\n' "${CANONICAL[@]}" | sort)"
build_sorted="$(printf '%s\n' "${BUILD_SERVICES[@]}" | sort)"
[[ "${canonical_sorted}" == "${build_sorted}" ]] || fail "C: service set mismatch"
echo "OK: C_SERVICE_SET_EQUALITY"

for required in payment-service contract-rate-service freight-cost-service control-tower-read-model-service; do
  grep -q "${required}" <<< "${build_sorted}" || fail "${required} missing from release build"
done
echo "OK: I_payment-service included"
echo "OK: J_contract-rate-service included"
echo "OK: K_freight-cost-service included"
echo "OK: L_control-tower-read-model-service included"

if grep -q 'contract-rate-service' "${RELEASE_BUILD}" \
  && ! grep -q 'docker-compose.bintrans-ct-staging.yml' "${RELEASE_BUILD}"; then
  fail "contract-rate/freight-cost require staging compose overlay"
fi
echo "OK: staging overlay used for overlay-only services"

grep -q 'bintrans_release_build_services' "${RELEASE_BUILD}" \
  || fail "release build must use canonical service contract from common.sh"
grep -qE '\bdocker push\b|\bdocker login\b|\bup -d\b|\bcompose up\b' "${RELEASE_BUILD}" \
  && fail "M/N: release build must not push or start containers"
echo "OK: M_no_registry_push"
echo "OK: N_no_container_start"

grep -q 'bintrans-staging-release-build' "${MAKEFILE}" \
  || fail "Makefile must define bintrans-staging-release-build target"
grep -q 'bintrans_ct_staging_release_build.sh' "${MAKEFILE}" \
  || fail "Makefile target must invoke release build script"
echo "OK: Makefile canonical release-build target present"

# D. remove one service => FAIL (simulate by checking count enforcement in script)
if ! grep -q 'RELEASE_BUILD_SERVICE_COUNT=13' "${RELEASE_BUILD}" \
  && ! grep -q 'must be 13' "${RELEASE_BUILD}"; then
  fail "D: release build must enforce 13-service count"
fi
echo "OK: D_service_count_guard_present"

# E/F. OCI source and label coverage on 13 Dockerfiles
dockerfiles=(
  services/api-gateway/Dockerfile
  services/identity-service/Dockerfile
  services/company-service/Dockerfile
  services/transport-order-service/Dockerfile
  services/rfx-service/Dockerfile
  services/shipment-service/Dockerfile
  services/document-service/Dockerfile
  services/billing-register-service/Dockerfile
  services/low-code-service/Dockerfile
  services/payment-service/Dockerfile
  services/contract-rate-service/Dockerfile
  services/freight-cost-service/Dockerfile
  services/control-tower-read-model-service/Dockerfile
)
[[ "${#dockerfiles[@]}" -eq 13 ]] || fail "OCI dockerfile list must contain 13 entries"

revision_count=0
version_count=0
source_count=0
for df in "${dockerfiles[@]}"; do
  path="${ROOT}/${df}"
  [[ -f "${path}" ]] || fail "missing Dockerfile: ${df}"
  grep -q 'org.opencontainers.image.revision' "${path}" || fail "F: missing revision label in ${df}"
  grep -q 'org.opencontainers.image.version' "${path}" || fail "missing version label in ${df}"
  grep -q 'org.opencontainers.image.source' "${path}" || fail "missing source label in ${df}"
  grep -q 'github.com/freight-platform/freight-platform' "${path}" \
    && fail "E: stale OCI source in ${df}"
  grep -q "${EXPECTED_OCI_SOURCE}" "${path}" || fail "E: incorrect OCI source in ${df}"
  revision_count=$((revision_count + 1))
  version_count=$((version_count + 1))
  source_count=$((source_count + 1))
done
echo "OCI_DOCKERFILES_TOTAL=13"
echo "OCI_REVISION_LABEL_COVERAGE=${revision_count}/13"
echo "OCI_VERSION_LABEL_COVERAGE=${version_count}/13"
echo "OCI_SOURCE_LABEL_COVERAGE=${source_count}/13"
echo "OCI_SOURCE_CORRECT=YES"

grep -q 'github.com/freight-platform/freight-platform' "${OCI_LABELS}" \
  && fail "stale OCI source in bintrans-oci-provenance.labels"
grep -q "${EXPECTED_OCI_SOURCE}" "${OCI_LABELS}" || fail "OCI labels template missing canonical source"

# G/H. build arg contract
assert_fail "G_missing_BINTRANS_GIT_SHA" bash -c \
  'source "'"${ROOT}"'/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"; bintrans_validate_release_build_args "" git-9c334f8'
assert_fail "H_wrong_SHA_version_pairing" bash -c \
  'source "'"${ROOT}"'/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"; bintrans_validate_release_build_args "'"${FIXTURE_SHA}"'" git-deadbeef'
assert_fail "B_malformed_SHA" bash -c \
  'source "'"${ROOT}"'/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"; bintrans_validate_release_build_args shortsha git-9c334f8'
assert_pass "E_all_correct_build_args" bash -c \
  'source "'"${ROOT}"'/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"; bintrans_validate_release_build_args "'"${FIXTURE_SHA}"'" "'"${FIXTURE_TAG}"'"'

if ! printf '%s\n' "${dry_out}" | grep -q "BINTRANS_GIT_SHA=${FIXTURE_SHA}"; then
  fail "dry-run must pass BINTRANS_GIT_SHA build arg"
fi
if ! printf '%s\n' "${dry_out}" | grep -q "BINTRANS_IMAGE_VERSION=${FIXTURE_TAG}"; then
  fail "dry-run must pass BINTRANS_IMAGE_VERSION build arg"
fi
echo "OK: BUILD_SHA_ARG_ENFORCED"
echo "OK: IMAGE_VERSION_SHA_MATCH_ENFORCED"

# HEAD mismatch negative (C in task section 5)
assert_fail "C_build_SHA_not_checked_out" bash -c \
  'ROOT="'"${ROOT}"'"; source "'"${ROOT}"'/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"; BINTRANS_RELEASE_GIT_SHA=0000000000000000000000000000000000000001 bintrans_resolve_release_build_sha'

echo "bintrans-ct-staging-release-build-selfcheck: PASS"
