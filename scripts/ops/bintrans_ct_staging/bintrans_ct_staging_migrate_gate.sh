#!/usr/bin/env bash
# BINTRANS dedicated staging — migration 000019 gate (read-only unless explicitly approved).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

MIGRATION_TARGET="${MIGRATION_TARGET:-000019}"
MIGRATION_VERSION=19

bintrans_require_env_file

set -a
# shellcheck disable=SC1090
source "${BINTRANS_STAGING_ENV}"
set +a

file_target="$(grep -E '^MIGRATION_TARGET=' "${BINTRANS_STAGING_ENV}" | tail -n1 | cut -d= -f2-)"
[[ "${file_target}" == "${MIGRATION_TARGET}" ]] \
  || bintrans_fail "MIGRATION_TARGET must be ${MIGRATION_TARGET} (found ${file_target:-<unset>})"

backup_verified="$(grep -E '^BACKUP_VERIFIED=' "${BINTRANS_STAGING_ENV}" | tail -n1 | cut -d= -f2- || echo NO)"
[[ "${backup_verified}" == "YES" ]] || bintrans_fail "BACKUP_VERIFIED=YES required before migration"

[[ -f "${ROOT}/infrastructure/migrations/000019_projection_rebuild_backup_last_event_type_nullable_v0.1.up.sql" ]] \
  || bintrans_fail "migration ${MIGRATION_TARGET} files missing at operator checkout"
[[ -f "${ROOT}/infrastructure/migrations/000019_projection_rebuild_backup_last_event_type_nullable_v0.1.down.sql" ]] \
  || bintrans_fail "migration ${MIGRATION_TARGET} down file missing at operator checkout"

[[ -n "${POSTGRES_USER:-}" && -n "${POSTGRES_DB:-}" && -n "${POSTGRES_PASSWORD:-}" ]] \
  || bintrans_fail "POSTGRES_* variables must be set in protected env"

migrate_db_url="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@postgres:5432/${POSTGRES_DB}?sslmode=disable"

pg_cid="$(bintrans_postgres_container)"
[[ -n "${pg_cid}" ]] || bintrans_fail "postgres container not running"

echo "=== PostgreSQL readiness ==="
docker exec "${pg_cid}" pg_isready -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" >/dev/null \
  || bintrans_fail "postgres not ready"

bintrans_migrate_version_raw() {
  BINTRANS_INCLUDE_SHADOW=0 BINTRANS_INCLUDE_IMAGES=0 \
    bintrans_compose --profile tools run --rm migrate \
    -path=/migrations \
    -database "${migrate_db_url}" \
    version 2>&1
}

echo "=== Current migration version ==="
version_output="$(bintrans_migrate_version_raw)" || version_rc=$?
version_rc="${version_rc:-0}"
echo "${version_output}"

if [[ "${version_rc}" -ne 0 ]]; then
  if echo "${version_output}" | grep -qi 'no migration'; then
    current_version=0
    dirty_state=no
  else
    bintrans_fail "unable to determine migration version (migrate version failed)"
  fi
elif echo "${version_output}" | grep -q '(dirty)'; then
  bintrans_fail "database migration state is dirty — manual intervention required before goto ${MIGRATION_VERSION}"
else
  current_version="$(echo "${version_output}" | tr -d '\r' | awk 'NF{print $1; exit}')"
  dirty_state=no
  if ! [[ "${current_version}" =~ ^[0-9]+$ ]]; then
    bintrans_fail "unable to parse migration version from: ${version_output}"
  fi
fi

echo "PARSED_CURRENT_VERSION=${current_version}"
echo "PARSED_DIRTY_STATE=${dirty_state}"

if [[ "${current_version}" -gt "${MIGRATION_VERSION}" ]]; then
  bintrans_fail "current migration version ${current_version} is greater than target ${MIGRATION_VERSION}"
fi

if [[ "${current_version}" -eq "${MIGRATION_VERSION}" ]]; then
  echo "ALREADY_AT_TARGET=YES"
  echo "Migration ${MIGRATION_TARGET} (version ${MIGRATION_VERSION}) already applied — no action taken."
  exit 0
fi

if [[ "${CONFIRM_MIGRATION_000019:-}" != "true" ]]; then
  echo
  echo "Migration NOT executed (gate-only mode)."
  echo "Current version: ${current_version}"
  echo "Target version: ${MIGRATION_VERSION} (${MIGRATION_TARGET})"
  echo "Operator approval required: CONFIRM_MIGRATION_000019=true"
  echo
  echo "Explicit target command (preferred over unbounded 'up'):"
  echo "  CONFIRM_MIGRATION_000019=true ${0}"
  exit 0
fi

echo "Applying migration goto ${MIGRATION_VERSION} ..."
BINTRANS_INCLUDE_SHADOW=0 BINTRANS_INCLUDE_IMAGES=0 \
  bintrans_compose --profile tools run --rm migrate \
  -path=/migrations \
  -database "${migrate_db_url}" \
  goto "${MIGRATION_VERSION}"

echo "=== Post-migration version (expect ${MIGRATION_VERSION}) ==="
post_output="$(bintrans_migrate_version_raw)"
echo "${post_output}"
if echo "${post_output}" | grep -q '(dirty)'; then
  bintrans_fail "post-migration state is dirty"
fi
post_version="$(echo "${post_output}" | tr -d '\r' | awk 'NF{print $1; exit}')"
[[ "${post_version}" == "${MIGRATION_VERSION}" ]] \
  || bintrans_fail "post-migration version ${post_version} != ${MIGRATION_VERSION}"

echo "bintrans-ct-staging-migrate-gate: migration applied to ${MIGRATION_VERSION}"
