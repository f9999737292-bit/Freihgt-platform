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

env_migration_target="${MIGRATION_TARGET}"
file_target="$(grep -E '^MIGRATION_TARGET=' "${BINTRANS_STAGING_ENV}" | tail -n1 | cut -d= -f2-)"
[[ "${file_target}" == "${MIGRATION_TARGET}" ]] \
  || bintrans_fail "MIGRATION_TARGET must be ${MIGRATION_TARGET} (found ${file_target})"

backup_verified="$(grep -E '^BACKUP_VERIFIED=' "${BINTRANS_STAGING_ENV}" | tail -n1 | cut -d= -f2- || echo NO)"
[[ "${backup_verified}" == "YES" ]] || bintrans_fail "BACKUP_VERIFIED=YES required before migration"

[[ -f "${ROOT}/infrastructure/migrations/000019_projection_rebuild_backup_last_event_type_nullable_v0.1.up.sql" ]] \
  || bintrans_fail "migration ${MIGRATION_TARGET} files missing at operator checkout"

[[ -n "${POSTGRES_USER:-}" && -n "${POSTGRES_DB:-}" && -n "${POSTGRES_PASSWORD:-}" ]] \
  || bintrans_fail "POSTGRES_* variables must be set in protected env"

migrate_db_url="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@postgres:5432/${POSTGRES_DB}?sslmode=disable"

pg_cid="$(bintrans_postgres_container)"
[[ -n "${pg_cid}" ]] || bintrans_fail "postgres container not running"

echo "=== Current migration version ==="
BINTRANS_INCLUDE_SHADOW=0 BINTRANS_INCLUDE_IMAGES=0 \
  bintrans_compose --profile tools run --rm migrate \
  -path=/migrations \
  -database "${migrate_db_url}" \
  version || true

if [[ "${CONFIRM_MIGRATION_000019:-}" != "true" ]]; then
  echo
  echo "Migration NOT executed (gate-only mode)."
  echo "Operator approval required: CONFIRM_MIGRATION_000019=true"
  echo
  echo "Explicit target command (preferred over unbounded 'up'):"
  echo "  CONFIRM_MIGRATION_000019=true ${0}"
  echo
  echo "Equivalent compose invocation:"
  echo "  bintrans_compose --profile tools run --rm migrate \\"
  echo "    -path=/migrations -database '<DATABASE_URL>' goto ${MIGRATION_VERSION}"
  exit 0
fi

echo "Applying migration goto ${MIGRATION_VERSION} ..."
BINTRANS_INCLUDE_SHADOW=0 BINTRANS_INCLUDE_IMAGES=0 \
  bintrans_compose --profile tools run --rm migrate \
  -path=/migrations \
  -database "${migrate_db_url}" \
  goto "${MIGRATION_VERSION}"

echo "=== Post-migration version (expect ${MIGRATION_VERSION}) ==="
BINTRANS_INCLUDE_SHADOW=0 BINTRANS_INCLUDE_IMAGES=0 \
  bintrans_compose --profile tools run --rm migrate \
  -path=/migrations \
  -database "${migrate_db_url}" \
  version

echo "bintrans-ct-staging-migrate-gate: migration applied to ${MIGRATION_VERSION}"
