#!/usr/bin/env bash
# Build v2.2 pilot release images locally. Does NOT push to registry.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
SHA="${PILOT_GIT_SHA:-37c2eb62ccf9377359eb5c2fdf6f71eb9d187140}"
SHORT_SHA="${SHA:0:7}"
TAG="${FREIGHT_PILOT_IMAGE_TAG:-git-${SHORT_SHA}}"
REGISTRY="${BINTRANS_REGISTRY:-cr.selcloud.ru/bintrans-staging}"

echo "=== Build Freight Cost Intelligence v2.2 pilot images ==="
echo "Source SHA: ${SHA}"
echo "Tag: ${TAG}"

build_one() {
  local name="$1"
  local dockerfile="$2"
  local context="${3:-${ROOT}}"
  local -a build_args=()
  shift 3 || true
  while [[ $# -gt 0 ]]; do
    build_args+=(--build-arg "$1")
    shift
  done
  local local_ref="freight-pilot/${name}:${TAG}"
  local remote_ref="${REGISTRY}/${name}:${TAG}"
  echo "--- Building ${name} ---"
  docker build -f "${dockerfile}" "${build_args[@]}" -t "${local_ref}" -t "${remote_ref}" "${context}"
  docker inspect --format='{{.Id}}' "${local_ref}"
  echo "OK: ${local_ref}"
  echo "OK: ${remote_ref}"
}

build_one freight-cost-service "${ROOT}/services/freight-cost-service/Dockerfile"
build_one api-gateway "${ROOT}/services/api-gateway/Dockerfile"
build_one web-procurement "${ROOT}/apps/web-procurement/Dockerfile" "${ROOT}" \
  "NUXT_PUBLIC_API_BASE_URL=${NUXT_PUBLIC_API_BASE_URL:-http://127.0.0.1:8080}" \
  "NUXT_PUBLIC_FREIGHT_COST_WORKSPACE_ENABLED=false"

echo
echo "IMAGE_SMOKE_GATE: run scripts/ops/freight_cost_pilot/image_smoke.sh"
echo "Publish: operator docker login + push ${REGISTRY}/<service>:${TAG}"
