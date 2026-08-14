#!/usr/bin/env bash
set -euo pipefail
echo "=== DB COUNTS ==="
docker exec freight_postgres psql -U bintrans_staging -d freight_platform -tAc "SELECT COUNT(*) FROM shipment.shipments;"
docker exec freight_postgres psql -U bintrans_staging -d freight_platform -tAc "SELECT COUNT(*) FROM control_tower.shipment_status_projection;"
echo "=== TENANT INDEXES ==="
docker exec freight_postgres psql -U bintrans_staging -d freight_platform -tAc "SELECT indexname FROM pg_indexes WHERE schemaname='shipment' AND tablename IN ('drivers','vehicles','shipments') AND indexname LIKE '%tenant_id%';"
echo "=== LOGIN EMPTY ==="
curl -sS -o /dev/null -w 'login_empty=%{http_code}\n' -X POST -H 'Content-Type: application/json' -d '{}' http://127.0.0.1:18080/api/v1/auth/login
echo "=== IMAGE TAGS ==="
docker inspect freight_api_gateway --format '{{.Config.Image}}'
docker inspect freight_control_tower_read_model_service --format '{{.Config.Image}}'
