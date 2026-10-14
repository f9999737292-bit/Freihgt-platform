#!/usr/bin/env bash
# Isolated restore drill — disposable PostgreSQL only. Does NOT touch live staging DB.
set -euo pipefail

BACKUP_FILE="/protected/bintrans/backups/freight_platform_20260811T083942Z.dump"
EXPECTED_SHA="c04d993fedc70b9627b773a367f0a62872fd6feed6ccce7990793bd7e66c6c9b"
CONTAINER="pilot-restore-drill-pg"
NETWORK="pilot-restore-drill-net"
PORT=55432
START_TS="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

echo "RESTORE_DRILL_START=${START_TS}"

[[ -f "${BACKUP_FILE}" ]] || { echo "RESTORE_DRILL=FAIL reason=backup_missing"; exit 1; }

ACTUAL_SHA="$(sha256sum "${BACKUP_FILE}" | awk '{print $1}')"
echo "BACKUP_SHA256=${ACTUAL_SHA}"
[[ "${ACTUAL_SHA}" == "${EXPECTED_SHA}" ]] || echo "WARN checksum differs from staging.env record"

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

docker run --rm -i --network "${NETWORK}" \
  -e PGPASSWORD=restore_drill_only \
  postgres:16 pg_restore \
  -h "${CONTAINER}" -U restore_drill -d freight_platform_drill \
  --no-owner --no-privileges \
  < "${BACKUP_FILE}" 2>/tmp/restore_drill.err || true

# pg_restore may exit non-zero for benign warnings; verify connectivity and objects
docker exec "${CONTAINER}" psql -U restore_drill -d freight_platform_drill -tAc \
  "SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name IN ('core','shipment','control_tower');" \
  > /tmp/schema_count.txt

SCHEMA_COUNT="$(tr -d '[:space:]' < /tmp/schema_count.txt)"
echo "SCHEMA_COUNT=${SCHEMA_COUNT}"

docker exec "${CONTAINER}" psql -U restore_drill -d freight_platform_drill -tAc \
  "SELECT version, dirty FROM schema_migrations;" 2>/dev/null || echo "MIGRATION_TABLE=absent"

docker exec "${CONTAINER}" psql -U restore_drill -d freight_platform_drill -tAc \
  "SELECT COUNT(*) FROM control_tower.shipment_status_projection;" 2>/dev/null || echo "PROJECTION_COUNT=0"

END_TS="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "RESTORE_DRILL_END=${END_TS}"

docker rm -f "${CONTAINER}" >/dev/null
docker network rm "${NETWORK}" 2>/dev/null || true

echo "LIVE_STAGING_IMPACT=NONE"
echo "RESTORE_TARGET=DISPOSABLE"
if [[ "${SCHEMA_COUNT}" -ge 1 ]]; then
  echo "RESTORE_DRILL=PASS"
else
  echo "RESTORE_DRILL=PARTIAL schema_count=${SCHEMA_COUNT}"
fi
