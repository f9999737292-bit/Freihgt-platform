#!/usr/bin/env bash
# Static self-check for runtime preflight pass/fail cases (no DB/containers).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
RUNTIME_PREFLIGHT="${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_preflight.sh"
FAKE_DIGEST='0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'

fail() { echo "runtime-preflight-selfcheck: $*" >&2; exit 1; }

FIXTURE_SHA="b75eb3de751002da94a3c271fda30d09be1db450"
FIXTURE_TAG="git-b75eb3d"

base_env() {
  cat <<EOF
STAGING_ENVIRONMENT=selectel-staging
DEPLOYED_GIT_SHA=${FIXTURE_SHA}
MIGRATION_TARGET=000036
COHORT_MANIFEST=/protected/bintrans/control-tower-cohort.json
OBSERVATION_OUTPUT_DIR=/protected/bintrans/control-tower-observation
POSTGRES_DB=freight_platform
POSTGRES_USER=bintrans_staging
POSTGRES_PASSWORD=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
BINTRANS_REGISTRY=cr.selcloud.ru/bintrans-staging
BINTRANS_IMAGE_TAG=${FIXTURE_TAG}
INTERNAL_SERVICE_TOKEN=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
API_GATEWAY_HOST_PORT=18080
CONTROL_TOWER_READ_MODEL_HOST_PORT=8089
PROMETHEUS_PORT=9090
GRAFANA_PORT=3001
GATEWAY_URL=http://127.0.0.1:18080
PROMETHEUS_URL=http://127.0.0.1:9090
CONTROL_TOWER_READ_MODEL_MODE=shadow
CONTROL_TOWER_CONSUMER_ENABLED=true
SHIPMENT_OUTBOX_ENABLED=true
AUTH_ENABLED=true
BACKUP_VERIFIED=YES
BACKUP_PATH=/protected/bintrans/backups/test.dump
BACKUP_SHA256=deadbeef
COHORT_APPROVED=NO
JWT_TOKEN=
EOF
}

digest_images() {
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

run_expect_fail() {
  local label="$1"
  local env_file="$2"
  if BINTRANS_STAGING_ENV="${env_file}" bash "${RUNTIME_PREFLIGHT}" >/dev/null 2>&1; then
    fail "${label}: expected FAIL, got PASS"
  fi
  echo "OK: ${label} rejected"
}

run_expect_pass() {
  local label="$1"
  local env_file="$2"
  local out rc
  set +e
  out="$(BINTRANS_STAGING_ENV="${env_file}" bash "${RUNTIME_PREFLIGHT}" 2>&1)"
  rc=$?
  set -e
  if [[ "${rc}" -ne 0 ]]; then
    echo "${out}" | tail -8 >&2
    fail "${label}: expected PASS, got FAIL"
  fi
  echo "OK: ${label} accepted"
}

tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT

# A: missing JWT_SECRET
env_a="${tmpdir}/missing_jwt.env"
base_env > "${env_a}"
digest_images "${FAKE_DIGEST}" >> "${env_a}"
run_expect_fail "MISSING_JWT_SECRET" "${env_a}"

# B: empty JWT_SECRET
env_b_empty="${tmpdir}/empty_jwt.env"
base_env > "${env_b_empty}"
echo 'JWT_SECRET=' >> "${env_b_empty}"
digest_images "${FAKE_DIGEST}" >> "${env_b_empty}"
run_expect_fail "EMPTY_JWT_SECRET" "${env_b_empty}"

# C: placeholder JWT_SECRET
env_b="${tmpdir}/placeholder_jwt.env"
base_env > "${env_b}"
echo 'JWT_SECRET=dev_secret_change_me' >> "${env_b}"
digest_images "${FAKE_DIGEST}" >> "${env_b}"
run_expect_fail "PLACEHOLDER_JWT_SECRET" "${env_b}"

# D: mutable tag-only images (no digest vars)
env_c="${tmpdir}/tag_only.env"
base_env > "${env_c}"
echo 'JWT_SECRET=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef' >> "${env_c}"
run_expect_fail "MUTABLE_TAG_ONLY" "${env_c}"

# E: malformed digest
env_e_bad="${tmpdir}/bad_digest.env"
base_env > "${env_e_bad}"
echo 'JWT_SECRET=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef' >> "${env_e_bad}"
digest_images "NOT_A_VALID_DIGEST" >> "${env_e_bad}"
run_expect_fail "MALFORMED_DIGEST" "${env_e_bad}"

# F: missing one service image
env_f="${tmpdir}/missing_one.env"
base_env > "${env_f}"
echo 'JWT_SECRET=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef' >> "${env_f}"
digest_images "${FAKE_DIGEST}" >> "${env_f}"
grep -v 'BINTRANS_API_GATEWAY_IMAGE' "${env_f}" > "${env_f}.tmp"
mv "${env_f}.tmp" "${env_f}"
run_expect_fail "MISSING_ONE_IMAGE" "${env_f}"

# G: primary mode
env_d="${tmpdir}/primary.env"
base_env | sed 's/CONTROL_TOWER_READ_MODEL_MODE=shadow/CONTROL_TOWER_READ_MODEL_MODE=primary/' > "${env_d}"
echo 'JWT_SECRET=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef' >> "${env_d}"
digest_images "${FAKE_DIGEST}" >> "${env_d}"
run_expect_fail "PRIMARY_MODE" "${env_d}"

# H: wrong registry
env_h="${tmpdir}/wrong_registry.env"
base_env > "${env_h}"
echo 'JWT_SECRET=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef' >> "${env_h}"
digest_images "${FAKE_DIGEST}" >> "${env_h}"
grep -v 'BINTRANS_' "${env_h}" > /dev/null 2>&1 || true
sed 's|cr.selcloud.ru/bintrans-staging|registry.example.com/bintrans-staging|g' "${env_h}" > "${env_h}.tmp"
mv "${env_h}.tmp" "${env_h}"
run_expect_fail "WRONG_REGISTRY" "${env_h}"

# I: valid shadow + digest + strong JWT
env_e="${tmpdir}/valid.env"
base_env > "${env_e}"
echo 'JWT_SECRET=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef' >> "${env_e}"
digest_images "${FAKE_DIGEST}" >> "${env_e}"
run_expect_pass "VALID_SHADOW_DIGEST_CONFIG" "${env_e}"

# J: distinct digest refs across all 13 services (must not false-positive mixed tags)
env_j="${tmpdir}/distinct_digest.env"
base_env > "${env_j}"
echo 'JWT_SECRET=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef' >> "${env_j}"
idx=1
while IFS= read -r svc; do
  var="$(BINTRANS_STAGING_ENV="${env_j}" bash -c 'source "'"${ROOT}"'/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"; bintrans_runtime_image_var_for_service "'"${svc}"'"')"
  d="$(printf '%064x' "${idx}")"
  echo "${var}=cr.selcloud.ru/bintrans-staging/${svc}@sha256:${d}" >> "${env_j}"
  idx=$((idx + 1))
done < <(bash -c 'source "'"${ROOT}"'/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"; printf "%s\n" "${bintrans_runtime_service_names[@]}"')
run_expect_pass "DISTINCT_DIGEST_REFS_ALL_SERVICES" "${env_j}"

echo "bintrans-ct-staging-runtime-preflight-selfcheck: PASS"
