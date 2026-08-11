#!/usr/bin/env bash
# Operator guide for registry publish + digest capture (DO NOT login/push from this script).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
REGISTRY="${BINTRANS_REGISTRY:-cr.selcloud.ru/bintrans-staging}"
TAG="${BINTRANS_IMAGE_TAG:-git-b75eb3d}"

cat <<EOF
=== BINTRANS staging registry publish workflow (operator manual) ===

This script prints steps only. It does NOT docker login, push, or pull.

Runtime source SHA: b75eb3d
Mutable publish tag: ${TAG}
Registry: ${REGISTRY}

For EACH of the 10 runtime services:

  identity-service
  company-service
  transport-order-service
  rfx-service
  shipment-service
  document-service
  billing-register-service
  low-code-service
  control-tower-read-model-service
  api-gateway

1. Build or locate local image for runtime source b75eb3d
2. Tag:
     docker tag <local-image> ${REGISTRY}/<service>:${TAG}
3. Operator login (out of band):
     docker login ${REGISTRY}
4. Push tag:
     docker push ${REGISTRY}/<service>:${TAG}
5. Obtain canonical digest:
     docker inspect --format='{{index .RepoDigests 0}}' ${REGISTRY}/<service>:${TAG}
6. Verify digest form:
     ${REGISTRY}/<service>@sha256:<64-lowercase-hex>
7. Add to protected staging.env:
     BINTRANS_<SERVICE>_IMAGE=${REGISTRY}/<service>@sha256:<digest>
8. Optional verify pull by digest:
     docker pull ${REGISTRY}/<service>@sha256:<digest>

After all 10 services pinned, run on VM:

  ./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_registry_digest_validate.sh
  ./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_preflight.sh

Template: scripts/ops/bintrans_ct_staging/runtime.images.digest.env.example

EOF

echo "bintrans-ct-staging-registry-publish-guide: printed"
