#!/usr/bin/env bash
# Check local publish-tag image presence for BINTRANS runtime services.
# Does NOT prove source SHA cryptographically — see docs/BINTRANS_STAGING_IMAGE_PROVENANCE.md
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

REGISTRY="${BINTRANS_REGISTRY:-cr.selcloud.ru/bintrans-staging}"
TAG="${BINTRANS_IMAGE_TAG:-git-b75eb3d}"
SOURCE_SHA="${DEPLOYED_GIT_SHA:-b75eb3d}"

echo "=== BINTRANS local image provenance check ==="
echo "Expected publish tag: ${TAG}"
echo "Expected source SHA (operator metadata): ${SOURCE_SHA}"
echo "LIMITATION: service Dockerfiles contain no GIT_SHA/SOURCE labels; tag name alone is not cryptographic proof."

if ! command -v docker >/dev/null 2>&1; then
  bintrans_fail "docker CLI not available"
fi

found=0
missing=()

for svc in "${bintrans_runtime_service_names[@]}"; do
  ref="${REGISTRY}/${svc}:${TAG}"
  if docker image inspect "${ref}" >/dev/null 2>&1; then
    found=$((found + 1))
    created="$(docker image inspect --format='{{.Created}}' "${ref}" 2>/dev/null || echo unknown)"
    echo "PRESENT: ${ref} (created=${created})"
  else
    missing+=("${ref}")
    echo "MISSING: ${ref}"
  fi
done

echo "LOCAL_PUBLISH_TAG_COUNT=${found}"
echo "EXPECTED_COUNT=${#bintrans_runtime_service_names[@]}"

if [[ ${#missing[@]} -gt 0 ]]; then
  echo "MISSING_REFS=${missing[*]}"
  bintrans_fail "not all publish-tag images present locally"
fi

echo "bintrans-ct-staging-image-provenance-check: PASS (tag presence only)"
