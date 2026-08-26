#!/usr/bin/env bash
# Offline self-check for runtime image digest validator.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
VALIDATOR="${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_images_validate.sh"
FAKE='0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'
SHORT63='0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcde'
LONG65='0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef00'
UPPER='0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF'
tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT

fail() { echo "runtime-images-validate-selfcheck: $*" >&2; exit 1; }

complete_env() {
  local d="$1"
  cat <<EOF
BINTRANS_IDENTITY_IMAGE=cr.selcloud.ru/bintrans-staging/identity-service@sha256:${d}
BINTRANS_COMPANY_IMAGE=cr.selcloud.ru/bintrans-staging/company-service@sha256:${d}
BINTRANS_TRANSPORT_ORDER_IMAGE=cr.selcloud.ru/bintrans-staging/transport-order-service@sha256:${d}
BINTRANS_RFX_IMAGE=cr.selcloud.ru/bintrans-staging/rfx-service@sha256:${d}
BINTRANS_SHIPMENT_IMAGE=cr.selcloud.ru/bintrans-staging/shipment-service@sha256:${d}
BINTRANS_DOCUMENT_IMAGE=cr.selcloud.ru/bintrans-staging/document-service@sha256:${d}
BINTRANS_BILLING_REGISTER_IMAGE=cr.selcloud.ru/bintrans-staging/billing-register-service@sha256:${d}
BINTRANS_LOW_CODE_IMAGE=cr.selcloud.ru/bintrans-staging/low-code-service@sha256:${d}
BINTRANS_PAYMENT_IMAGE=cr.selcloud.ru/bintrans-staging/payment-service@sha256:${d}
BINTRANS_CONTRACT_RATE_IMAGE=cr.selcloud.ru/bintrans-staging/contract-rate-service@sha256:${d}
BINTRANS_FREIGHT_COST_IMAGE=cr.selcloud.ru/bintrans-staging/freight-cost-service@sha256:${d}
BINTRANS_CONTROL_TOWER_READ_MODEL_IMAGE=cr.selcloud.ru/bintrans-staging/control-tower-read-model-service@sha256:${d}
BINTRANS_API_GATEWAY_IMAGE=cr.selcloud.ru/bintrans-staging/api-gateway@sha256:${d}
EOF
}

expect_fail() {
  local label="$1" env_file="$2"
  if BINTRANS_STAGING_ENV="${env_file}" bash "${VALIDATOR}" >/dev/null 2>&1; then
    fail "${label}: expected FAIL"
  fi
  echo "OK: ${label} rejected"
}

expect_pass() {
  local label="$1" env_file="$2"
  local out
  out="$(BINTRANS_STAGING_ENV="${env_file}" bash "${VALIDATOR}" 2>&1)" \
    || fail "${label}: expected PASS"
  echo "${out}" | grep -q 'ALL_RUNTIME_IMAGES_PINNED=YES' || fail "${label}: missing ALL_RUNTIME_IMAGES_PINNED=YES"
  echo "OK: ${label} accepted"
}

# 13 valid synthetic refs
env_ok="${tmpdir}/complete.env"
complete_env "${FAKE}" > "${env_ok}"
expect_pass "THIRTEEN_VALID_DIGESTS" "${env_ok}"

# 12/13
env_nine="${tmpdir}/nine.env"
complete_env "${FAKE}" > "${env_nine}"
grep -v 'BINTRANS_API_GATEWAY_IMAGE' "${env_nine}" > "${env_nine}.tmp" && mv "${env_nine}.tmp" "${env_nine}"
expect_fail "TWELVE_OF_THIRTEEN" "${env_nine}"

# tag-only
env_tag="${tmpdir}/tag.env"
echo "BINTRANS_IDENTITY_IMAGE=cr.selcloud.ru/bintrans-staging/identity-service:git-deadbeef" > "${env_tag}"
expect_fail "TAG_ONLY" "${env_tag}"

# wrong registry
env_reg="${tmpdir}/reg.env"
echo "BINTRANS_IDENTITY_IMAGE=registry.example.com/bintrans-staging/identity-service@sha256:${FAKE}" > "${env_reg}"
expect_fail "WRONG_REGISTRY" "${env_reg}"

# wrong service name
env_svc="${tmpdir}/svc.env"
echo "BINTRANS_IDENTITY_IMAGE=cr.selcloud.ru/bintrans-staging/wrong-service@sha256:${FAKE}" > "${env_svc}"
expect_fail "WRONG_SERVICE" "${env_svc}"

# 63-char digest
env_63="${tmpdir}/d63.env"
echo "BINTRANS_IDENTITY_IMAGE=cr.selcloud.ru/bintrans-staging/identity-service@sha256:${SHORT63}" > "${env_63}"
expect_fail "DIGEST_63_CHAR" "${env_63}"

# 65-char digest
env_65="${tmpdir}/d65.env"
echo "BINTRANS_IDENTITY_IMAGE=cr.selcloud.ru/bintrans-staging/identity-service@sha256:${LONG65}" > "${env_65}"
expect_fail "DIGEST_65_CHAR" "${env_65}"

# uppercase digest
env_up="${tmpdir}/upper.env"
echo "BINTRANS_IDENTITY_IMAGE=cr.selcloud.ru/bintrans-staging/identity-service@sha256:${UPPER}" > "${env_up}"
expect_fail "UPPERCASE_DIGEST" "${env_up}"

# placeholder
env_ph="${tmpdir}/ph.env"
echo "BINTRANS_IDENTITY_IMAGE=cr.selcloud.ru/bintrans-staging/identity-service@sha256:REPLACE_WITH_VERIFIED_DIGEST" > "${env_ph}"
expect_fail "PLACEHOLDER" "${env_ph}"

echo "bintrans-ct-staging-runtime-images-validate-selfcheck: PASS"
