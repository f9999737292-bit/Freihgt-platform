#!/usr/bin/env bash
# Verify OCI image revision labels match DEPLOYED_GIT_SHA for BINTRANS runtime services.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

bintrans_require_env_file
bintrans_validate_release_contract

REGISTRY="${BINTRANS_REGISTRY:-cr.selcloud.ru/bintrans-staging}"
TAG="$(bintrans_env_value BINTRANS_IMAGE_TAG)"
SOURCE_SHA="$(bintrans_env_value DEPLOYED_GIT_SHA)"

echo "=== BINTRANS image provenance check (OCI revision) ==="
echo "Expected publish tag: ${TAG}"
echo "Expected source SHA: ${SOURCE_SHA}"

if ! command -v docker >/dev/null 2>&1; then
  bintrans_fail "docker CLI not available"
fi

verified=0
missing=()
mismatch=()

for svc in "${bintrans_runtime_service_names[@]}"; do
  var="$(bintrans_runtime_image_var_for_service "${svc}")"
  ref="$(bintrans_env_value "${var}")"
  if [[ -z "${ref}" ]]; then
    ref="${REGISTRY}/${svc}:${TAG}"
  fi
  if ! docker image inspect "${ref}" >/dev/null 2>&1; then
    missing+=("${ref}")
    echo "MISSING: ${ref}"
    continue
  fi
  label="$(bintrans_oci_revision_label "${ref}")"
  if [[ -z "${label}" ]]; then
    mismatch+=("${ref}:missing-label")
    echo "NO_LABEL: ${ref}"
    continue
  fi
  if [[ "${label}" != "${SOURCE_SHA}" ]]; then
    mismatch+=("${ref}:${label}")
    echo "MISMATCH: ${ref} label=${label}"
    continue
  fi
  verified=$((verified + 1))
  echo "OK: ${ref} revision=${label}"
done

echo "OCI_REVISION_EXACT_COUNT=${verified}"
echo "EXPECTED_COUNT=${#bintrans_runtime_service_names[@]}"

if [[ ${#missing[@]} -gt 0 ]]; then
  echo "MISSING_REFS=${missing[*]}"
  bintrans_fail "not all runtime images present locally"
fi
if [[ ${#mismatch[@]} -gt 0 ]]; then
  echo "MISMATCH_REFS=${mismatch[*]}"
  bintrans_fail "OCI revision mismatch or missing on runtime images"
fi

echo "bintrans-ct-staging-image-provenance-check: PASS (OCI revision exact)"
