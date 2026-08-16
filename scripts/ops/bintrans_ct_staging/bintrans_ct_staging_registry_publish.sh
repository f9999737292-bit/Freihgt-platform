#!/usr/bin/env bash
# BINTRANS registry publish helper — prepares operator steps; does NOT login/push/pull.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

REGISTRY="${BINTRANS_REGISTRY:-cr.selcloud.ru/bintrans-staging}"
TAG="${BINTRANS_IMAGE_TAG:-git-b75eb3d}"
SOURCE_SHA="${DEPLOYED_GIT_SHA:-b75eb3d}"

echo "=== BINTRANS registry publish (prepare-only) ==="
echo "Registry: ${REGISTRY}"
echo "Publish tag: ${TAG}"
echo "Expected runtime source SHA: ${SOURCE_SHA}"

if ! command -v docker >/dev/null 2>&1; then
  bintrans_fail "docker CLI not available"
fi

# Fail safely if registry auth appears absent (no credentials printed).
if ! docker info >/dev/null 2>&1; then
  bintrans_fail "docker daemon not reachable"
fi

if ! grep -q "${REGISTRY%%/*}" "${HOME}/.docker/config.json" 2>/dev/null; then
  echo "NOTE: no docker config entry for ${REGISTRY%%/*} — operator must docker login before push"
fi

missing_local=()
ready_local=()

for svc in "${bintrans_runtime_service_names[@]}"; do
  remote_ref="${REGISTRY}/${svc}:${TAG}"
  if docker image inspect "${remote_ref}" >/dev/null 2>&1; then
    ready_local+=("${svc}")
    echo "OK: local image present ${remote_ref}"
  else
    missing_local+=("${svc}")
    echo "MISSING: local image ${remote_ref}"
  fi
done

if [[ ${#missing_local[@]} -gt 0 ]]; then
  echo
  echo "Build/publish tag images locally before push, e.g.:"
  echo "  make platform-build-service SERVICE=<service>"
  echo "  docker tag <local-compose-image> ${REGISTRY}/<service>:${TAG}"
  bintrans_fail "missing local publish-tag images: ${missing_local[*]}"
fi

echo
echo "Operator push sequence (manual — this script does NOT push):"
for svc in "${bintrans_runtime_service_names[@]}"; do
  echo "  docker tag ${REGISTRY}/${svc}:${TAG} ${REGISTRY}/${svc}:${TAG}"
  echo "  docker push ${REGISTRY}/${svc}:${TAG}"
  echo "  docker inspect --format='{{index .RepoDigests 0}}' ${REGISTRY}/${svc}:${TAG}"
  echo "  # write BINTRANS_*_IMAGE=${REGISTRY}/${svc}@sha256:<digest> to protected env"
done

if [[ "${BINTRANS_REGISTRY_PUBLISH_CONFIRM:-}" == "YES" ]]; then
  bintrans_fail "push is intentionally disabled in repository tooling — operator must run docker push manually after login"
fi

echo
echo "bintrans-ct-staging-registry-publish: PREPARE_OK (no push executed)"
