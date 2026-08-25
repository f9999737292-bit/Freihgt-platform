#!/usr/bin/env bash
# BINTRANS dedicated staging — target-driven migration gate (read-only unless explicitly approved).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

bintrans_require_env_file
bintrans_require_migration_target_contract

MIGRATION_TARGET="$(bintrans_read_protected_migration_target)"
MIGRATION_VERSION="$(bintrans_migration_version_from_target "${MIGRATION_TARGET}")"
mapfile -t MIGRATION_FILES < <(bintrans_resolve_migration_file_pair "${MIGRATION_TARGET}")
MIGRATION_UP="${MIGRATION_FILES[0]}"
MIGRATION_DOWN="${MIGRATION_FILES[1]}"

set -a
# shellcheck disable=SC1090
source "${BINTRANS_STAGING_ENV}"
set +a

[[ -n "${POSTGRES_USER:-}" && -n "${POSTGRES_DB:-}" && -n "${POSTGRES_PASSWORD:-}" ]] \
  || bintrans_fail "POSTGRES_* variables must be set in protected env"

migrate_db_url="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@postgres:5432/${POSTGRES_DB}?sslmode=disable"

pg_cid="$(bintrans_postgres_container)"
[[ -n "${pg_cid}" ]] || bintrans_fail "postgres container not running"

echo "=== migration gate target=${MIGRATION_TARGET} version=${MIGRATION_VERSION} ==="
echo "migration up file=${MIGRATION_UP##*/}"
echo "migration down file=${MIGRATION_DOWN##*/}"

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

bintrans_read_migration_version() {
  local output="$1"
  local parsed version dirty parse_rc

  if ! parsed="$(bintrans_parse_migrate_version "${output}")"; then
    parse_rc=$?
    if [[ "${parse_rc}" -eq 2 ]]; then
      bintrans_fail "conflicting migration version lines in migrate version output"
    fi
    bintrans_fail "unable to parse migration version from migrate output"
  fi

  version="${parsed%% *}"
  dirty="${parsed#* }"
  if [[ "${dirty}" == yes ]]; then
    bintrans_fail "database migration state is dirty — manual intervention required before goto ${MIGRATION_VERSION}"
  fi

  if ! [[ "${version}" =~ ^[0-9]+$ ]]; then
    bintrans_fail "parsed migration version is not numeric: ${version}"
  fi

  printf '%s\n' "${version}"
}

echo "=== Current migration version ==="
version_output="$(bintrans_migrate_version_raw)" || version_rc=$?
version_rc="${version_rc:-0}"
echo "${version_output}"

if [[ "${version_rc}" -ne 0 ]] && ! echo "${version_output}" | grep -qi 'no migration'; then
  bintrans_fail "unable to determine migration version (migrate version failed)"
fi

current_version="$(bintrans_read_migration_version "${version_output}")"
dirty_state=no

echo "PARSED_CURRENT_VERSION=${current_version}"
echo "PARSED_DIRTY_STATE=${dirty_state}"
echo "CURRENT_VERSION=${current_version}"
echo "TARGET_VERSION=${MIGRATION_VERSION}"

if [[ "${current_version}" -gt "${MIGRATION_VERSION}" ]]; then
  bintrans_fail "current migration version ${current_version} is greater than target ${MIGRATION_VERSION} (${MIGRATION_TARGET})"
fi

if [[ "${current_version}" -eq "${MIGRATION_VERSION}" ]]; then
  echo "ALREADY_AT_TARGET=YES"
  echo "NO ACTION TAKEN"
  echo "Migration ${MIGRATION_TARGET} (version ${MIGRATION_VERSION}) already applied — no action taken."
  exit 0
fi

backup_verified="$(grep -E '^BACKUP_VERIFIED=' "${BINTRANS_STAGING_ENV}" | tail -n1 | cut -d= -f2- || echo NO)"
echo "BACKUP_VERIFIED=${backup_verified:-<unset>}"

if [[ "${CONFIRM_MIGRATION_TARGET:-}" != "true" ]]; then
  echo
  echo "Migration NOT executed (gate-only mode)."
  echo "Current version: ${current_version}"
  echo "Target version: ${MIGRATION_VERSION} (${MIGRATION_TARGET})"
  echo "Operator approval required: CONFIRM_MIGRATION_TARGET=true (invocation-local only)"
  echo
  echo "Explicit target command (preferred over unbounded 'up'):"
  echo "  CONFIRM_MIGRATION_TARGET=true ${0}"
  exit 0
fi

[[ "${backup_verified}" == "YES" ]] || bintrans_fail "BACKUP_VERIFIED=YES required before migration execution"

echo "Applying migration goto ${MIGRATION_VERSION} ..."
BINTRANS_INCLUDE_SHADOW=0 BINTRANS_INCLUDE_IMAGES=0 \
  bintrans_compose --profile tools run --rm migrate \
  -path=/migrations \
  -database "${migrate_db_url}" \
  goto "${MIGRATION_VERSION}"

echo "=== Post-migration version (expect ${MIGRATION_VERSION}) ==="
post_output="$(bintrans_migrate_version_raw)"
echo "${post_output}"

post_version="$(bintrans_read_migration_version "${post_output}")"
[[ "${post_version}" == "${MIGRATION_VERSION}" ]] \
  || bintrans_fail "post-migration version ${post_version} != ${MIGRATION_VERSION}"

echo "bintrans-ct-staging-migrate-gate: migration applied to ${MIGRATION_VERSION}"
