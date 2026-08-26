#!/usr/bin/env bash
# Operator guide: publish v2.2 pilot images to BINTRANS staging registry (prepare + manual push).
# Does NOT login, push, or print credentials.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
SHA="${PILOT_GIT_SHA:-37c2eb62ccf9377359eb5c2fdf6f71eb9d187140}"
SHORT_SHA="${SHA:0:7}"
TAG="${FREIGHT_PILOT_IMAGE_TAG:-git-${SHORT_SHA}}"
REGISTRY="${BINTRANS_REGISTRY:-cr.selcloud.ru/bintrans-staging}"

SERVICES=(
  freight-cost-service
  api-gateway
  web-procurement
)

echo "=== v2.2 pilot registry publish guide ==="
echo "Registry: ${REGISTRY}"
echo "Tag: ${TAG}"
echo "Git SHA: ${SHA}"
echo

"${ROOT}/scripts/ops/freight_cost_pilot/build_v22_images.sh"

echo
echo "Operator steps (on trusted host with registry access):"
echo "  1. docker login ${REGISTRY%%/*}"
for svc in "${SERVICES[@]}"; do
  echo "  2. docker push ${REGISTRY}/${svc}:${TAG}"
  echo "  3. docker inspect --format='{{index .RepoDigests 0}}' ${REGISTRY}/${svc}:${TAG}"
done
echo
echo "Write digest-pinned refs to protected env:"
echo "  BINTRANS_FREIGHT_COST_IMAGE=${REGISTRY}/freight-cost-service@sha256:<digest>"
echo "  BINTRANS_API_GATEWAY_IMAGE=${REGISTRY}/api-gateway@sha256:<digest>"
echo "  BINTRANS_WEB_PROCUREMENT_IMAGE=${REGISTRY}/web-procurement@sha256:<digest>"
echo "  BINTRANS_IMAGE_TAG=${TAG}"
echo "  MIGRATION_TARGET=000064"
echo "  DEPLOYED_GIT_SHA=${SHORT_SHA}"
echo
echo "Rollback for freight-cost-service (new component):"
echo "  - Level 1: FREIGHT_COST_ANALYTICS_PROJECTION_ENABLED=false"
echo "  - Level 2: stop/remove freight-cost-service container"
echo "  - Gateway rollback: prior BINTRANS_API_GATEWAY_IMAGE digest (pre-v2.2)"
echo "  - web-procurement rollback: prior digest or remove service"
