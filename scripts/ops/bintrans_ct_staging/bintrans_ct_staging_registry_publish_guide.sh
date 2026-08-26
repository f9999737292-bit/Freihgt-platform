#!/usr/bin/env bash
# Operator guide for registry publish + digest capture (DO NOT login/push from this script).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

REGISTRY="${BINTRANS_REGISTRY:-cr.selcloud.ru/bintrans-staging}"
TAG="${BINTRANS_IMAGE_TAG:-<set-from-DEPLOYED_GIT_SHA>}"

cat <<EOF
=== BINTRANS staging registry publish workflow (operator manual) ===

This script prints steps only. It does NOT docker login, push, or pull.

Canonical release build command:
  git checkout <DEPLOYED_GIT_SHA>
  make bintrans-staging-release-build

Registry: ${REGISTRY}
Mutable publish tag: ${TAG} (must match DEPLOYED_GIT_SHA)

For EACH of the 13 canonical runtime services:

$(printf '  %s\n' "${bintrans_runtime_service_names[@]}")

1. Checkout exact release SHA
2. Build all 13 via: make bintrans-staging-release-build
3. Validate OCI revision labels match DEPLOYED_GIT_SHA
4. Tag for registry:
     docker tag <local-image> ${REGISTRY}/<service>:${TAG}
5. Operator login (out of band):
     docker login ${REGISTRY}
6. Push tag (manual):
     docker push ${REGISTRY}/<service>:${TAG}
7. Obtain canonical digest:
     docker inspect --format='{{index .RepoDigests 0}}' ${REGISTRY}/<service>:${TAG}
8. Add to protected staging.env:
     BINTRANS_<SERVICE>_IMAGE=${REGISTRY}/<service>@sha256:<digest>
9. Validate protected env:
     bintrans_ct_staging_runtime_images_validate.sh

Repository tooling remains prepare-only — no automatic push.
EOF
