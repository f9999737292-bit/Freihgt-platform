#!/usr/bin/env bash
# BINTRANS dedicated staging — static preflight (no deploy, no migration, no containers started).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

MIGRATION_TARGET="${MIGRATION_TARGET:-000019}"
MIGRATION_VERSION="${MIGRATION_TARGET#0}" # 000019 -> 19 for file/version checks

echo "=== BINTRANS dedicated staging preflight ==="

bintrans_require_env_file

# Required repository paths
required_files=(
  "${BINTRANS_COMPOSE_BASE}"
  "${BINTRANS_COMPOSE_BINTRANS}"
  "${BINTRANS_COMPOSE_SHADOW}"
  "${BINTRANS_COMPOSE_IMAGES}"
  "${ROOT}/scripts/ops/bintrans_ct_staging/staging.env.example"
  "${ROOT}/scripts/ops/bintrans_ct_staging/registry.images.template.env"
  "${ROOT}/infrastructure/migrations/000019_projection_rebuild_backup_last_event_type_nullable_v0.1.up.sql"
  "${ROOT}/infrastructure/migrations/000019_projection_rebuild_backup_last_event_type_nullable_v0.1.down.sql"
)

for f in "${required_files[@]}"; do
  [[ -f "${f}" ]] || bintrans_fail "missing required file: ${f}"
done
echo "OK: required repository files present (including migration ${MIGRATION_TARGET})"

# Protected env semantics (never print secret values)
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

env_not_empty() {
  local key="$1"
  local val
  val="$(grep -E "^${key}=" "${BINTRANS_STAGING_ENV}" | tail -n1 | cut -d= -f2- || true)"
  [[ -n "${val}" ]] || bintrans_fail "${key} must not be empty in protected env"
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
env_present MIGRATION_TARGET
env_check MIGRATION_TARGET "${MIGRATION_TARGET}"
env_not_placeholder_password
echo "OK: protected env shadow safety fields"

if grep -qE '^CONTROL_TOWER_READ_MODEL_MODE=primary' "${BINTRANS_STAGING_ENV}"; then
  bintrans_fail "PRIMARY mode detected in protected env"
fi
echo "OK: primary mode not enabled in protected env"

# Cohort gate — empty placeholder must not pass
cohort_path="$(grep -E '^COHORT_MANIFEST=' "${BINTRANS_STAGING_ENV}" | tail -n1 | cut -d= -f2- || true)"
[[ -n "${cohort_path}" ]] || bintrans_fail "COHORT_MANIFEST must be set"
if [[ -f "${cohort_path}" ]]; then
  if [[ ! -s "${cohort_path}" ]]; then
    echo "WARN: cohort file exists but is empty — COHORT_APPROVED=NO (loader will reject)"
  else
    if grep -q '"tenants"[[:space:]]*:[[:space:]]*\[\s*\]' "${cohort_path}" 2>/dev/null; then
      bintrans_fail "cohort manifest has empty tenants array — not valid for observation"
    fi
    approved="$(grep -E '^COHORT_APPROVED=' "${BINTRANS_STAGING_ENV}" | tail -n1 | cut -d= -f2- || echo NO)"
    [[ "${approved}" == "YES" ]] || echo "WARN: cohort file present but COHORT_APPROVED!=YES — Day 0 blocked"
  fi
else
  echo "WARN: cohort manifest not found at ${cohort_path} — COHORT_APPROVED=NO"
fi

# Registry template variables
grep -qE '^BINTRANS_REGISTRY=' "${BINTRANS_STAGING_ENV}" || bintrans_fail "BINTRANS_REGISTRY must be set"
grep -qE '^BINTRANS_IMAGE_TAG=' "${BINTRANS_STAGING_ENV}" || bintrans_fail "BINTRANS_IMAGE_TAG must be set"
echo "OK: registry variables present"

# Compose static render — foundation pack
render_foundation="$(mktemp)"
render_runtime="$(mktemp)"
trap 'rm -f "${render_foundation}" "${render_runtime}"' EXIT

BINTRANS_INCLUDE_SHADOW=0 BINTRANS_INCLUDE_IMAGES=0 \
  bintrans_compose --profile messaging config > "${render_foundation}"

BINTRANS_INCLUDE_SHADOW=1 BINTRANS_INCLUDE_IMAGES=1 \
  bintrans_compose --profile messaging --profile read-model config > "${render_runtime}"

echo "OK: docker compose config rendered (foundation + runtime packs)"

# Shadow semantics in runtime render
grep -q 'CONTROL_TOWER_READ_MODEL_MODE: shadow' "${render_runtime}" \
  || bintrans_fail "runtime compose must set CONTROL_TOWER_READ_MODEL_MODE=shadow"
grep -q 'CONTROL_TOWER_CONSUMER_ENABLED: "true"' "${render_runtime}" \
  || bintrans_fail "runtime compose must enable consumer"
grep -q 'SHIPMENT_OUTBOX_ENABLED: "true"' "${render_runtime}" \
  || bintrans_fail "runtime compose must enable shipment outbox"

if grep -q 'CONTROL_TOWER_READ_MODEL_MODE: primary' "${render_runtime}"; then
  bintrans_fail "PRIMARY mode detected in rendered runtime compose"
fi
echo "OK: shadow mode verified in rendered runtime compose"

# Dangerous host port detection in rendered compose config
check_no_wide_bind() {
  local cfg="$1"
  local label="$2"
  if grep -E 'published: "(5432|19092|9090|8080|8081|8082|8083|8084|8085|8086|8087|8088)"' "${cfg}" >/dev/null; then
    while IFS= read -r pub_line; do
      port="${pub_line#*published: \"}"
      port="${port%%\"*}"
      block="$(grep -B6 "published: \"${port}\"" "${cfg}" | tail -n7)"
      if ! echo "${block}" | grep -q 'host_ip: 127.0.0.1'; then
        bintrans_fail "dangerous host bind ${port} without 127.0.0.1 in ${label}"
      fi
    done < <(grep -E 'published: "(5432|19092|9090|8080|8081|8082|8083|8084|8085|8086|8087|8088)"' "${cfg}" || true)
  fi
}

check_no_wide_bind "${render_foundation}" "foundation"
check_no_wide_bind "${render_runtime}" "runtime"
echo "OK: no wide host binds on sensitive ports (127.0.0.1-only or unpublished)"

# Document remaining published ports
echo "--- Remaining host-published ports (runtime render) ---"
grep 'published:' "${render_runtime}" | sort -u || echo "(none)"
echo "--- End published ports ---"

# Backup gate reminder
backup_verified="$(grep -E '^BACKUP_VERIFIED=' "${BINTRANS_STAGING_ENV}" | tail -n1 | cut -d= -f2- || echo NO)"
if [[ "${backup_verified}" != "YES" ]]; then
  echo "NOTE: BACKUP_VERIFIED!=YES — migration must not run"
fi

echo "bintrans-ct-staging-preflight: PASS"
