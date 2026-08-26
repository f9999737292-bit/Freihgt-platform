#!/usr/bin/env bash
# BINTRANS staging — canonical 13-service release image build (build-only; no push/start).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

DRY_RUN=0
if [[ "${1:-}" == "--dry-run" ]]; then
  DRY_RUN=1
fi
COMPOSE_BASE="${ROOT}/infrastructure/docker-compose/docker-compose.yml"
COMPOSE_STAGING="${ROOT}/infrastructure/docker-compose/docker-compose.bintrans-ct-staging.yml"

for f in "${COMPOSE_BASE}" "${COMPOSE_STAGING}"; do
  [[ -f "${f}" ]] || bintrans_fail "missing compose file for release build: ${f}"
done

if [[ "${DRY_RUN}" -ne 1 ]]; then
  if ! command -v docker >/dev/null 2>&1; then
    bintrans_fail "docker CLI not available"
  fi
fi

RELEASE_SHA="$(bintrans_resolve_release_build_sha)"
RELEASE_IMAGE_VERSION="${BINTRANS_IMAGE_VERSION:-$(bintrans_expected_image_tag_for_sha "${RELEASE_SHA}")}"
bintrans_validate_release_build_args "${RELEASE_SHA}" "${RELEASE_IMAGE_VERSION}"

mapfile -t RELEASE_SERVICES < <(bintrans_release_build_services)
[[ "${#RELEASE_SERVICES[@]}" -eq 13 ]] \
  || bintrans_fail "internal: release build service count must be 13 (found ${#RELEASE_SERVICES[@]})"

echo "=== BINTRANS staging release build ==="
echo "RELEASE_SHA=${RELEASE_SHA}"
echo "BINTRANS_IMAGE_VERSION=${RELEASE_IMAGE_VERSION}"
echo "RELEASE_BUILD_SERVICE_COUNT=${#RELEASE_SERVICES[@]}"
echo "COMPOSE_FILES=${COMPOSE_BASE} + ${COMPOSE_STAGING}"

bintrans_release_build_one() {
  local svc="$1"
  local -a cmd=(
    docker compose
    -f "${COMPOSE_BASE}"
    -f "${COMPOSE_STAGING}"
  )
  if [[ "${svc}" == "control-tower-read-model-service" ]]; then
    cmd+=(--profile read-model)
  fi
  cmd+=(
    build
    --build-arg "BINTRANS_GIT_SHA=${RELEASE_SHA}"
    --build-arg "BINTRANS_IMAGE_VERSION=${RELEASE_IMAGE_VERSION}"
    --progress=plain
    "${svc}"
  )
  if [[ "${DRY_RUN}" -eq 1 ]]; then
    printf 'DRY_RUN: '
    printf '%q ' "${cmd[@]}"
    printf '\n'
    return 0
  fi
  echo "Building ${svc} ..."
  "${cmd[@]}"
}

for svc in "${RELEASE_SERVICES[@]}"; do
  bintrans_release_build_one "${svc}"
done

echo "bintrans-ct-staging-release-build: PASS (${#RELEASE_SERVICES[@]} services, push/start not performed)"
