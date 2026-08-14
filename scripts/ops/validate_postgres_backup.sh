#!/usr/bin/env bash
# Validate PostgreSQL custom-format backup contains recoverable application content.
# Fail-closed: non-zero if dump is structurally empty or missing critical metadata.
# Never prints secrets. Safe to run read-only against backup file only.
set -euo pipefail

MIN_SIZE_BYTES="${MIN_BACKUP_SIZE_BYTES:-10240}"
BACKUP_FILE="${1:-}"

if [[ -z "${BACKUP_FILE}" ]]; then
  echo "usage: validate_postgres_backup.sh <backup.dump>" >&2
  exit 2
fi

[[ -f "${BACKUP_FILE}" ]] || { echo "BACKUP_VALIDATION=FAIL reason=file_missing"; exit 1; }

size_bytes="$(stat -c '%s' "${BACKUP_FILE}" 2>/dev/null || stat -f '%z' "${BACKUP_FILE}")"
echo "BACKUP_SIZE_BYTES=${size_bytes}"

if [[ "${size_bytes}" -lt "${MIN_SIZE_BYTES}" ]]; then
  echo "BACKUP_VALIDATION=FAIL reason=size_below_minimum min=${MIN_SIZE_BYTES}"
  exit 1
fi

if ! head -c 5 "${BACKUP_FILE}" | grep -q 'PGDMP'; then
  echo "BACKUP_VALIDATION=FAIL reason=invalid_format"
  exit 1
fi

toc_file="$(mktemp)"
trap 'rm -f "${toc_file}"' EXIT

docker run --rm -i postgres:16 pg_restore -l < "${BACKUP_FILE}" > "${toc_file}" 2>/dev/null \
  || { echo "BACKUP_VALIDATION=FAIL reason=pg_restore_list_failed"; exit 1; }

schema_count="$(grep -c ' SCHEMA ' "${toc_file}" || true)"
table_count="$(grep -c ' TABLE ' "${toc_file}" || true)"
echo "BACKUP_SCHEMA_COUNT=${schema_count}"
echo "BACKUP_TABLE_COUNT=${table_count}"

if [[ "${schema_count}" -lt 1 ]]; then
  echo "BACKUP_VALIDATION=FAIL reason=no_application_schemas"
  exit 1
fi

if [[ "${table_count}" -lt 1 ]]; then
  echo "BACKUP_VALIDATION=FAIL reason=no_application_tables"
  exit 1
fi

if ! grep -q 'schema_migrations' "${toc_file}"; then
  echo "BACKUP_VALIDATION=FAIL reason=schema_migrations_missing"
  exit 1
fi

echo "CRITICAL_TABLES_PRESENT=schema_migrations"
echo "BACKUP_VALIDATION=PASS"
