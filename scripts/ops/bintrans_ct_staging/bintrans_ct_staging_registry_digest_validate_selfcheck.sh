#!/usr/bin/env bash
# Offline self-check for registry digest validation helper.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
VALIDATOR="${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_registry_digest_validate.sh"
FAKE='0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'
tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT

fail() { echo "registry-digest-validate-selfcheck: $*" >&2; exit 1; }

# Incomplete manifest must fail
incomplete="${tmpdir}/incomplete.env"
echo "BINTRANS_IDENTITY_IMAGE=cr.selcloud.ru/bintrans-staging/identity-service@sha256:${FAKE}" > "${incomplete}"
if BINTRANS_STAGING_ENV="${incomplete}" bash "${VALIDATOR}" >/dev/null 2>&1; then
  fail "incomplete manifest should FAIL"
fi
echo "OK: incomplete manifest rejected"

# Complete manifest must pass
complete="${tmpdir}/complete.env"
cat > "${complete}" <<EOF
BINTRANS_IDENTITY_IMAGE=cr.selcloud.ru/bintrans-staging/identity-service@sha256:${FAKE}
BINTRANS_COMPANY_IMAGE=cr.selcloud.ru/bintrans-staging/company-service@sha256:${FAKE}
BINTRANS_TRANSPORT_ORDER_IMAGE=cr.selcloud.ru/bintrans-staging/transport-order-service@sha256:${FAKE}
BINTRANS_RFX_IMAGE=cr.selcloud.ru/bintrans-staging/rfx-service@sha256:${FAKE}
BINTRANS_SHIPMENT_IMAGE=cr.selcloud.ru/bintrans-staging/shipment-service@sha256:${FAKE}
BINTRANS_DOCUMENT_IMAGE=cr.selcloud.ru/bintrans-staging/document-service@sha256:${FAKE}
BINTRANS_BILLING_REGISTER_IMAGE=cr.selcloud.ru/bintrans-staging/billing-register-service@sha256:${FAKE}
BINTRANS_LOW_CODE_IMAGE=cr.selcloud.ru/bintrans-staging/low-code-service@sha256:${FAKE}
BINTRANS_CONTROL_TOWER_READ_MODEL_IMAGE=cr.selcloud.ru/bintrans-staging/control-tower-read-model-service@sha256:${FAKE}
BINTRANS_API_GATEWAY_IMAGE=cr.selcloud.ru/bintrans-staging/api-gateway@sha256:${FAKE}
EOF
out="$(BINTRANS_STAGING_ENV="${complete}" bash "${VALIDATOR}" 2>&1)" || fail "complete manifest should PASS"
echo "${out}" | grep -q 'ALL_RUNTIME_IMAGES_PINNED=YES' || fail "expected ALL_RUNTIME_IMAGES_PINNED=YES"
echo "${out}" | grep -q 'RUNTIME_IMAGE_COUNT=10' || fail "expected RUNTIME_IMAGE_COUNT=10"
echo "OK: complete manifest accepted"

echo "bintrans-ct-staging-registry-digest-validate-selfcheck: PASS"
