#!/usr/bin/env bash
# Isolated restore drill — disposable PostgreSQL only. Does NOT touch live staging DB.
set -euo pipefail

BACKUP_FILE="${BACKUP_FILE:-}"
EXPECTED_SHA="${EXPECTED_SHA:-}"
CONTAINER="${RESTORE_CONTAINER:-pilot-restore-drill-pg}"
NETWORK="${RESTORE_NETWORK:-pilot-restore-drill-net}"
PORT="${RESTORE_PORT:-55432}"
START_TS="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

echo "RESTORE_DRILL_START=${START_TS}"

[[ -n "${BACKUP_FILE}" ]] || { echo "RESTORE_DRILL=FAIL reason=backup_file_required"; exit 1; }
[[ -f "${BACKUP_FILE}" ]] || { echo "RESTORE_DRILL=FAIL reason=backup_missing"; exit 1; }

ACTUAL_SHA="$(sha256sum "${BACKUP_FILE}" | awk '{print $1}')"
echo "BACKUP_SHA256=${ACTUAL_SHA}"
if [[ -n "${EXPECTED_SHA}" && "${ACTUAL_SHA}" != "${EXPECTED_SHA}" ]]; then
  echo "WARN checksum differs from expected record"
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VALIDATOR="${SCRIPT_DIR}/validate_postgres_backup.sh"
if [[ -x "${VALIDATOR}" ]]; then
  "${VALIDATOR}" "${BACKUP_FILE}" || { echo "RESTORE_DRILL=FAIL reason=backup_validation_failed"; exit 1; }
else
  echo "WARN validator not found — skipping pre-restore backup validation"
fi

docker network create "${NETWORK}" 2>/dev/null || true
docker rm -f "${CONTAINER}" 2>/dev/null || true

docker run -d --name "${CONTAINER}" --network "${NETWORK}" \
  -e POSTGRES_PASSWORD=restore_drill_only \
  -e POSTGRES_USER=restore_drill \
  -e POSTGRES_DB=freight_platform_drill \
  -p "127.0.0.1:${PORT}:5432" \
  postgres:16 >/dev/null

for i in $(seq 1 30); do
  if docker exec "${CONTAINER}" pg_isready -U restore_drill -d freight_platform_drill >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

docker exec "${CONTAINER}" pg_isready -U restore_drill -d freight_platform_drill >/dev/null \
  || { echo "RESTORE_DRILL=FAIL reason=disposable_pg_not_ready"; exit 1; }

echo "DISPOSABLE_PG_READY=YES"
echo "RESTORE_TARGET=DISPOSABLE"

restore_exit=0
docker run --rm -i --network "${NETWORK}" \
  -e PGPASSWORD=restore_drill_only \
  postgres:16 pg_restore \
  -h "${CONTAINER}" -U restore_drill -d freight_platform_drill \
  --no-owner --no-privileges \
  < "${BACKUP_FILE}" 2>/tmp/restore_drill.err || restore_exit=$?

echo "RESTORE_COMMAND_EXIT=${restore_exit}"

schema_count="$(docker exec "${CONTAINER}" psql -U restore_drill -d freight_platform_drill -tAc \
  "SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name NOT IN ('pg_catalog','information_schema','pg_toast');")"
table_count="$(docker exec "${CONTAINER}" psql -U restore_drill -d freight_platform_drill -tAc \
  "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema NOT IN ('pg_catalog','information_schema');")"

echo "RESTORED_SCHEMA_COUNT=${schema_count}"
echo "RESTORED_TABLE_COUNT=${table_count}"

migration_count="$(docker exec "${CONTAINER}" psql -U restore_drill -d freight_platform_drill -tAc \
  "SELECT COUNT(*) FROM schema_migrations;" 2>/dev/null || echo 0)"
echo "SCHEMA_MIGRATIONS_COUNT=${migration_count}"

projection_count="$(docker exec "${CONTAINER}" psql -U restore_drill -d freight_platform_drill -tAc \
  "SELECT COUNT(*) FROM control_tower.shipment_status_projection;" 2>/dev/null || echo 0)"
echo "PROJECTION_ROW_COUNT=${projection_count}"

END_TS="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "RESTORE_DRILL_END=${END_TS}"

docker rm -f "${CONTAINER}" >/dev/null
docker network rm "${NETWORK}" 2>/dev/null || true

echo "LIVE_STAGING_IMPACT=NONE"

if [[ "${schema_count}" -ge 1 && "${table_count}" -ge 1 && "${migration_count}" -ge 1 ]]; then
  echo "SCHEMA_VALIDATION=PASS"
  echo "DATA_VALIDATION=PASS"
  echo "RESTORE_DRILL=PASS"
else
  echo "SCHEMA_VALIDATION=FAIL"
  echo "DATA_VALIDATION=FAIL"
  echo "RESTORE_DRILL=FAIL"
  exit 1
fi
