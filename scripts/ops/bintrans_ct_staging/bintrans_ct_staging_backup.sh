#!/usr/bin/env bash
# BINTRANS dedicated staging — PostgreSQL backup (pg_dump custom format).
# Never prints POSTGRES_PASSWORD. Does not upload or auto-delete backups.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

bintrans_require_env_file

POSTGRES_USER="$(grep -E '^POSTGRES_USER=' "${BINTRANS_STAGING_ENV}" | tail -n1 | cut -d= -f2-)"
POSTGRES_DB="$(grep -E '^POSTGRES_DB=' "${BINTRANS_STAGING_ENV}" | tail -n1 | cut -d= -f2-)"
[[ -n "${POSTGRES_USER}" && -n "${POSTGRES_DB}" ]] || bintrans_fail "POSTGRES_USER and POSTGRES_DB required"

BACKUP_DIR="${BINTRANS_BACKUP_DIR:-/protected/bintrans/backups}"
mkdir -p "${BACKUP_DIR}"
chmod 700 "${BACKUP_DIR}"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
backup_file="${BACKUP_DIR}/freight_platform_${timestamp}.dump"

pg_cid="$(bintrans_postgres_container)"
[[ -n "${pg_cid}" ]] || bintrans_fail "postgres container not running — start foundation first"

echo "Creating backup: ${backup_file}"

docker exec "${pg_cid}" pg_dump \
  -U "${POSTGRES_USER}" \
  -d "${POSTGRES_DB}" \
  -Fc \
  --no-owner \
  --no-privileges \
  > "${backup_file}"

[[ -s "${backup_file}" ]] || bintrans_fail "backup file is empty"

# Structural verification for pg_dump custom format (magic PGDMP)
if ! head -c 5 "${backup_file}" | grep -q 'PGDMP'; then
  bintrans_fail "backup does not look like pg_dump custom format"
fi

docker run --rm -i postgres:16 pg_restore -l < "${backup_file}" >/dev/null \
  || bintrans_fail "pg_restore -l verification failed"

VALIDATOR="${ROOT}/scripts/ops/validate_postgres_backup.sh"
[[ -x "${VALIDATOR}" ]] || bintrans_fail "backup validator missing: ${VALIDATOR}"
"${VALIDATOR}" "${backup_file}" || bintrans_fail "backup content validation failed (empty or missing application objects)"

checksum="$(sha256sum "${backup_file}" | awk '{print $1}')"
chmod 600 "${backup_file}"

echo "BACKUP_PATH=${backup_file}"
echo "BACKUP_SHA256=${checksum}"
echo "BACKUP_SIZE_BYTES=$(stat -c '%s' "${backup_file}" 2>/dev/null || stat -f '%z' "${backup_file}")"
echo "BACKUP_VALIDATION=PASS"
echo
echo "Operator may set in protected env after review:"
echo "  BACKUP_VERIFIED=YES"
echo "  BACKUP_PATH=${backup_file}"
echo "  BACKUP_SHA256=${checksum}"
echo
echo "bintrans-ct-staging-backup: PASS"
