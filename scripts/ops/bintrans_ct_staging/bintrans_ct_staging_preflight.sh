#!/usr/bin/env bash
# BINTRANS dedicated staging — static preflight (no deploy, no migration, no containers started).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

echo "=== BINTRANS dedicated staging preflight ==="

bintrans_require_env_file
bintrans_require_migration_target_contract

MIGRATION_TARGET="$(bintrans_read_protected_migration_target)"
MIGRATION_VERSION="$(bintrans_migration_version_from_target "${MIGRATION_TARGET}")"
mapfile -t MIGRATION_FILES < <(bintrans_resolve_migration_file_pair "${MIGRATION_TARGET}")
MIGRATION_UP="${MIGRATION_FILES[0]}"
MIGRATION_DOWN="${MIGRATION_FILES[1]}"
echo "OK: migration target=${MIGRATION_TARGET} version=${MIGRATION_VERSION}"
echo "OK: migration up file=${MIGRATION_UP##*/}"
echo "OK: migration down file=${MIGRATION_DOWN##*/}"

required_files=(
  "${BINTRANS_COMPOSE_BASE}"
  "${BINTRANS_COMPOSE_BINTRANS}"
  "${BINTRANS_COMPOSE_SHADOW}"
  "${BINTRANS_COMPOSE_IMAGES}"
  "${ROOT}/scripts/ops/bintrans_ct_staging/staging.env.example"
  "${ROOT}/scripts/ops/bintrans_ct_staging/registry.images.template.env"
  "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_foundation_up_selfcheck.sh"
  "${MIGRATION_UP}"
  "${MIGRATION_DOWN}"
)

for f in "${required_files[@]}"; do
  [[ -f "${f}" ]] || bintrans_fail "missing required file: ${f}"
done
echo "OK: required repository files present (migration ${MIGRATION_TARGET} from release SHA, not pack addition)"

"${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_foundation_up_selfcheck.sh"

env_check() {
  local key="$1" expected="$2"
  local actual
  actual="$(grep -E "^${key}=" "${BINTRANS_STAGING_ENV}" | tail -n1 | cut -d= -f2- || true)"
  [[ "${actual}" == "${expected}" ]] || bintrans_fail "${key} must be ${expected} (found: ${actual:-<unset>})"
}

env_present() {
  local key="$1"
  grep -qE "^${key}=" "${BINTRANS_STAGING_ENV}" || bintrans_fail "${key} must be set in protected env"
}

env_not_placeholder_password() {
  local val
  val="$(grep -E "^POSTGRES_PASSWORD=" "${BINTRANS_STAGING_ENV}" | tail -n1 | cut -d= -f2- || true)"
  [[ -n "${val}" ]] || bintrans_fail "POSTGRES_PASSWORD must be set in protected env"
  [[ "${val}" != "freight_password" ]] || bintrans_fail "POSTGRES_PASSWORD must not use dev default freight_password"
}

env_check CONTROL_TOWER_READ_MODEL_MODE shadow
env_check CONTROL_TOWER_CONSUMER_ENABLED true
env_check SHIPMENT_OUTBOX_ENABLED true
env_check AUTH_ENABLED true
env_present DEPLOYED_GIT_SHA
env_present BINTRANS_IMAGE_TAG
env_present MIGRATION_TARGET
env_present INTERNAL_SERVICE_TOKEN
BINTRANS_STAGING_ENV="${BINTRANS_STAGING_ENV}" bintrans_validate_release_contract
echo "OK: generic release contract (DEPLOYED_GIT_SHA + matching BINTRANS_IMAGE_TAG)"
env_not_placeholder_password
echo "OK: protected env migration target contract"
echo "OK: protected env shadow safety fields"

# Digest-pinned image references must be full registry paths, never bare @sha256:...
while IFS= read -r line; do
  [[ "${line}" =~ ^BINTRANS_.*_IMAGE= ]] || continue
  value="${line#*=}"
  [[ -n "${value}" ]] || continue
  if [[ "${value}" =~ ^@sha256: ]]; then
    bintrans_fail "digest image reference must include full registry path, not bare ${value}"
  fi
  if [[ "${value}" == *REPLACE_WITH_VERIFIED_DIGEST* ]]; then
    bintrans_fail "digest placeholder must be replaced before runtime deploy"
  fi
  if [[ "${value}" == *@sha256:* ]] && [[ "${value}" != cr.selcloud.ru/bintrans-staging/*@sha256:* ]]; then
    bintrans_fail "digest image reference must start with cr.selcloud.ru/bintrans-staging/"
  fi
done < "${BINTRANS_STAGING_ENV}"
echo "OK: registry digest references not using invalid bare/placeholder forms"

cohort_path="$(grep -E '^COHORT_MANIFEST=' "${BINTRANS_STAGING_ENV}" | tail -n1 | cut -d= -f2- || true)"
[[ -n "${cohort_path}" ]] || bintrans_fail "COHORT_MANIFEST must be set"
if [[ -f "${cohort_path}" ]]; then
  if [[ ! -s "${cohort_path}" ]]; then
    echo "WARN: cohort file exists but is empty — COHORT_APPROVED=NO (loader rejects empty cohort)"
  elif grep -q '"tenants"[[:space:]]*:[[:space:]]*\[\s*\]' "${cohort_path}" 2>/dev/null; then
    bintrans_fail "cohort manifest has empty tenants array"
  else
    approved="$(grep -E '^COHORT_APPROVED=' "${BINTRANS_STAGING_ENV}" | tail -n1 | cut -d= -f2- || echo NO)"
    [[ "${approved}" == "YES" ]] || echo "WARN: cohort file present but COHORT_APPROVED!=YES — Day 0 blocked"
  fi
else
  echo "WARN: cohort manifest not found at ${cohort_path} — COHORT_APPROVED=NO"
fi

grep -qE '^BINTRANS_REGISTRY=' "${BINTRANS_STAGING_ENV}" || bintrans_fail "BINTRANS_REGISTRY must be set"
grep -qE '^BINTRANS_IMAGE_TAG=' "${BINTRANS_STAGING_ENV}" || bintrans_fail "BINTRANS_IMAGE_TAG must be set"
echo "OK: registry variables present"

render_foundation="$(mktemp)"
render_runtime="$(mktemp)"
render_observability="$(mktemp)"
trap 'rm -f "${render_foundation}" "${render_runtime}" "${render_observability}"' EXIT

BINTRANS_INCLUDE_SHADOW=0 BINTRANS_INCLUDE_IMAGES=0 \
  bintrans_compose --profile messaging config > "${render_foundation}"

BINTRANS_INCLUDE_SHADOW=1 BINTRANS_INCLUDE_IMAGES=1 \
  bintrans_compose --profile messaging --profile read-model config > "${render_runtime}"

BINTRANS_INCLUDE_SHADOW=1 BINTRANS_INCLUDE_IMAGES=1 \
  bintrans_compose --profile messaging --profile read-model --profile observability config > "${render_observability}"

echo "OK: docker compose config rendered (foundation + runtime + observability)"

gateway_mode="$(bintrans_extract_gateway_mode "${render_runtime}")"
[[ "${gateway_mode}" == "shadow" ]] \
  || bintrans_fail "effective api-gateway CONTROL_TOWER_READ_MODEL_MODE must be shadow (found: ${gateway_mode:-<unset>})"

if [[ "$(bintrans_extract_gateway_mode "${render_observability}")" != "shadow" ]]; then
  bintrans_fail "observability render must keep api-gateway mode=shadow"
fi
echo "OK: effective api-gateway mode=shadow (not primary/disabled)"

grep -q 'CONTROL_TOWER_CONSUMER_ENABLED: "true"' "${render_runtime}" \
  || bintrans_fail "runtime compose must enable consumer"
grep -q 'SHIPMENT_OUTBOX_ENABLED: "true"' "${render_runtime}" \
  || bintrans_fail "runtime compose must enable shipment outbox"

check_no_wide_bind() {
  local cfg="$1"
  local label="$2"
  if grep -E 'published: "(5432|19092|9090|8080|8081|8082|8083|8084|8085|8086|8087|8088|8090|8091|8092|3000|3001)"' "${cfg}" >/dev/null; then
    while IFS= read -r pub_line; do
      port="${pub_line#*published: \"}"
      port="${port%%\"*}"
      block="$(grep -B6 "published: \"${port}\"" "${cfg}" | tail -n7)"
      if ! echo "${block}" | grep -q 'host_ip: 127.0.0.1'; then
        bintrans_fail "dangerous host bind ${port} without 127.0.0.1 in ${label}"
      fi
    done < <(grep -E 'published: "(5432|19092|9090|8080|8081|8082|8083|8084|8085|8086|8087|8088|8090|8091|8092|3000|3001)"' "${cfg}" || true)
  fi
}

check_no_wide_bind "${render_foundation}" "foundation"
check_no_wide_bind "${render_runtime}" "runtime"
check_no_wide_bind "${render_observability}" "observability"
echo "OK: no wide host binds on sensitive ports (127.0.0.1-only or unpublished)"

echo "--- Published ports: foundation ---"
grep -B3 'published:' "${render_foundation}" | grep -E '^(  [a-z0-9-]+:|        published:|        host_ip:)' || echo "(none)"
echo "--- Published ports: runtime shadow ---"
grep -B3 'published:' "${render_runtime}" | grep -E '^(  [a-z0-9-]+:|        published:|        host_ip:)' || echo "(none)"
echo "--- Published ports: observability ---"
grep -B3 'published:' "${render_observability}" | grep -E '^(  [a-z0-9-]+:|        published:|        host_ip:)' || echo "(none)"

backup_verified="$(grep -E '^BACKUP_VERIFIED=' "${BINTRANS_STAGING_ENV}" | tail -n1 | cut -d= -f2- || echo NO)"
if [[ "${backup_verified}" != "YES" ]]; then
  echo "NOTE: BACKUP_VERIFIED!=YES — migration must not run"
fi

echo "bintrans-ct-staging-preflight: PASS"
