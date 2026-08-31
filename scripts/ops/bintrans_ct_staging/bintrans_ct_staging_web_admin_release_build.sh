#!/usr/bin/env bash
# BINTRANS CT staging — web-admin release image build (build-only; no push/start).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

DRY_RUN=0
if [[ "${1:-}" == "--dry-run" ]]; then
  DRY_RUN=1
fi

RELEASE_SHA="$(bintrans_resolve_release_build_sha)"
RELEASE_IMAGE_VERSION="${BINTRANS_IMAGE_VERSION:-$(bintrans_expected_image_tag_for_sha "${RELEASE_SHA}")}"
bintrans_validate_release_build_args "${RELEASE_SHA}" "${RELEASE_IMAGE_VERSION}"

REGISTRY="${BINTRANS_REGISTRY:-cr.selcloud.ru/bintrans-staging}"
IMAGE_NAME="${REGISTRY}/web-admin"
LOCAL_TAG="${IMAGE_NAME}:${RELEASE_IMAGE_VERSION}"
LOCAL_SHA_TAG="${IMAGE_NAME}:${RELEASE_SHA}"

NUXT_PUBLIC_API_BASE_URL="${NUXT_PUBLIC_API_BASE_URL:-http://127.0.0.1:18080}"
NUXT_PUBLIC_MOCK_AUTH="${NUXT_PUBLIC_MOCK_AUTH:-false}"

echo "=== BINTRANS CT staging web-admin release build ==="
echo "RELEASE_SHA=${RELEASE_SHA}"
echo "BINTRANS_IMAGE_VERSION=${RELEASE_IMAGE_VERSION}"
echo "LOCAL_TAG=${LOCAL_TAG}"
echo "NUXT_PUBLIC_API_BASE_URL=${NUXT_PUBLIC_API_BASE_URL}"
echo "NUXT_PUBLIC_MOCK_AUTH=${NUXT_PUBLIC_MOCK_AUTH}"

build_cmd=(
  docker build
  -f "${ROOT}/apps/web-admin/Dockerfile"
  --build-arg "BINTRANS_GIT_SHA=${RELEASE_SHA}"
  --build-arg "BINTRANS_IMAGE_VERSION=${RELEASE_IMAGE_VERSION}"
  --build-arg "NUXT_PUBLIC_API_BASE_URL=${NUXT_PUBLIC_API_BASE_URL}"
  --build-arg "NUXT_PUBLIC_MOCK_AUTH=${NUXT_PUBLIC_MOCK_AUTH}"
  -t "${LOCAL_TAG}"
  -t "${LOCAL_SHA_TAG}"
  "${ROOT}/apps/web-admin"
)

if [[ "${DRY_RUN}" -eq 1 ]]; then
  printf 'DRY_RUN: '
  printf '%q ' "${build_cmd[@]}"
  printf '\n'
  exit 0
fi

"${build_cmd[@]}"

DIGEST="$(docker image inspect "${LOCAL_TAG}" --format '{{index .RepoDigests 0}}' 2>/dev/null || true)"
if [[ -z "${DIGEST}" ]]; then
  DIGEST="$(docker image inspect "${LOCAL_TAG}" --format '{{.Id}}')"
fi

echo "WEB_ADMIN_LOCAL_IMAGE_ID=${LOCAL_TAG}"
echo "WEB_ADMIN_IMAGE_DIGEST=${DIGEST}"
echo "bintrans-ct-staging-web-admin-release-build: PASS (push/start not performed)"
