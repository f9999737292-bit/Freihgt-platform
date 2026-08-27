#!/usr/bin/env bash
# Static self-check for staging PostgreSQL pool budget contract (no DB/containers).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

RUNTIME_PREFLIGHT="${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_preflight.sh"
FAKE_DIGEST='0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'

fail() { echo "pool-budget-selfcheck: $*" >&2; exit 1; }

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
DB_MAX_OPEN_CONNS=7
DB_MAX_IDLE_CONNS=3
DB_MAX_OPEN_LIGHT=5
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
  shift 2
  if env "$@" BINTRANS_STAGING_ENV="${env_file}" bash "${RUNTIME_PREFLIGHT}" >/dev/null 2>&1; then
    fail "${label}: expected FAIL, got PASS"
  fi
  echo "OK: ${label} rejected"
}

run_expect_pass() {
  local label="$1"
  local env_file="$2"
  shift 2
  local out rc
  set +e
  out="$(env "$@" BINTRANS_STAGING_ENV="${env_file}" bash "${RUNTIME_PREFLIGHT}" 2>&1)"
  rc=$?
  set -e
  if [[ "${rc}" -ne 0 ]]; then
    echo "${out}" | tail -8 >&2
    fail "${label}: expected PASS, got FAIL"
  fi
  echo "OK: ${label} accepted"
}

assert_aggregate_from_render() {
  local env_file="$1" expected="$2"
  local render aggregate
  render="$(mktemp)"
  trap 'rm -f "${render}"' RETURN
  BINTRANS_STAGING_ENV="${env_file}" BINTRANS_INCLUDE_SHADOW=1 BINTRANS_INCLUDE_IMAGES=1 \
    bintrans_compose --profile messaging --profile read-model config > "${render}"
  aggregate="$(bintrans_calculate_rendered_aggregate_pool_budget "${render}")"
  [[ "${aggregate}" -eq "${expected}" ]] \
    || fail "render aggregate ${aggregate} != ${expected}"
  echo "OK: rendered aggregate pool budget=${aggregate}"
}

tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT

valid_env="${tmpdir}/valid.env"
base_env > "${valid_env}"
echo 'JWT_SECRET=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef' >> "${valid_env}"
digest_images "${FAKE_DIGEST}" >> "${valid_env}"

# PASS: canonical 80-budget render
run_expect_pass "VALID_POOL_BUDGET_80" "${valid_env}"
assert_aggregate_from_render "${valid_env}" 80

# Verify per-service rendered values
render="$(mktemp)"
BINTRANS_STAGING_ENV="${valid_env}" BINTRANS_INCLUDE_SHADOW=1 BINTRANS_INCLUDE_IMAGES=1 \
  bintrans_compose --profile messaging --profile read-model config > "${render}"
for svc in identity-service company-service payment-service; do
  mo="$(bintrans_rendered_service_env_value "${render}" "${svc}" "DB_MAX_OPEN_CONNS")"
  mi="$(bintrans_rendered_service_env_value "${render}" "${svc}" "DB_MAX_IDLE_CONNS")"
  [[ "${mo}" == "7" && "${mi}" == "3" ]] || fail "${svc} render ${mo}/${mi} != 7/3"
done
for svc in shipment-service control-tower-read-model-service; do
  mo="$(bintrans_rendered_service_env_value "${render}" "${svc}" "DB_MAX_OPEN_CONNS")"
  [[ "${mo}" == "5" ]] || fail "${svc} light render ${mo} != 5"
done
gw="$(bintrans_rendered_service_env_value "${render}" "api-gateway" "DB_MAX_OPEN_CONNS")"
[[ -z "${gw}" ]] || fail "api-gateway must not receive DB_MAX_OPEN_CONNS (got ${gw})"
echo "OK: per-service rendered pool table matches contract"
rm -f "${render}"

# FAIL: pool overlay omitted
run_expect_fail "POOL_OVERLAY_OMITTED" "${valid_env}" BINTRANS_INCLUDE_POOL=0

# FAIL: unsafe aggregate (25 default would be 300)
unsafe_env="${tmpdir}/unsafe.env"
base_env | sed 's/DB_MAX_OPEN_CONNS=7/DB_MAX_OPEN_CONNS=25/' > "${unsafe_env}"
echo 'JWT_SECRET=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef' >> "${unsafe_env}"
digest_images "${FAKE_DIGEST}" >> "${unsafe_env}"
run_expect_fail "UNSAFE_AGGREGATE_300" "${unsafe_env}"

# FAIL: idle > open
bad_idle_env="${tmpdir}/bad_idle.env"
base_env | sed 's/DB_MAX_IDLE_CONNS=3/DB_MAX_IDLE_CONNS=9/' > "${bad_idle_env}"
echo 'JWT_SECRET=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef' >> "${bad_idle_env}"
digest_images "${FAKE_DIGEST}" >> "${bad_idle_env}"
run_expect_fail "IDLE_GT_OPEN" "${bad_idle_env}"

# FAIL: zero max open
zero_env="${tmpdir}/zero.env"
base_env | sed 's/DB_MAX_OPEN_CONNS=7/DB_MAX_OPEN_CONNS=0/' > "${zero_env}"
echo 'JWT_SECRET=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef' >> "${zero_env}"
digest_images "${FAKE_DIGEST}" >> "${zero_env}"
run_expect_fail "ZERO_MAX_OPEN" "${zero_env}"

# FAIL: non-numeric
nan_env="${tmpdir}/nan.env"
base_env | sed 's/DB_MAX_OPEN_CONNS=7/DB_MAX_OPEN_CONNS=abc/' > "${nan_env}"
echo 'JWT_SECRET=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef' >> "${nan_env}"
digest_images "${FAKE_DIGEST}" >> "${nan_env}"
run_expect_fail "NON_NUMERIC_MAX_OPEN" "${nan_env}"

# FAIL: light > general max
light_env="${tmpdir}/light.env"
base_env | sed 's/DB_MAX_OPEN_LIGHT=5/DB_MAX_OPEN_LIGHT=9/' > "${light_env}"
echo 'JWT_SECRET=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef' >> "${light_env}"
digest_images "${FAKE_DIGEST}" >> "${light_env}"
run_expect_fail "LIGHT_GT_GENERAL" "${light_env}"

# Pool compose file must exist in repository
[[ -f "${BINTRANS_COMPOSE_POOL}" ]] || fail "missing ${BINTRANS_COMPOSE_POOL}"
grep -q 'DB_MAX_OPEN_CONNS' "${BINTRANS_COMPOSE_POOL}" \
  || fail "pool compose must reference DB_MAX_OPEN_CONNS"

# runtime_up path includes pool via bintrans_compose
grep -q 'BINTRANS_INCLUDE_POOL' "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh" \
  || fail "bintrans_compose must support pool overlay inclusion"

echo "bintrans-ct-staging-pool-budget-selfcheck: PASS"
