#!/usr/bin/env bash
# BINTRANS CT staging — idempotent pilot driver seed for Driver Mobile E2E.
# Requires: protected staging.env, postgres running, migrations through driver platform.
# Password MUST be supplied via PILOT_DRIVER_PASSWORD (never printed or committed).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

bintrans_require_env_file

set -a
# shellcheck disable=SC1090
source "${BINTRANS_STAGING_ENV}"
set +a

: "${PILOT_DRIVER_PASSWORD:?PILOT_DRIVER_PASSWORD must be set in environment (not committed)}"

PILOT_DRIVER_EMAIL="${PILOT_DRIVER_EMAIL:-driver-pilot-p0@test.local}"
PILOT_DRIVER_FULL_NAME="${PILOT_DRIVER_FULL_NAME:-BINTRANS CT Pilot Driver}"
PILOT_SHIPMENT_NUMBER="${PILOT_SHIPMENT_NUMBER:-BINTRANS-CT-STAGING-20260811T173628Z-SH-03}"

pg_cid="$(bintrans_postgres_container)"
[[ -n "${pg_cid}" ]] || bintrans_fail "postgres container not running"

if ! command -v python3 >/dev/null 2>&1; then
  bintrans_fail "python3 required to hash PILOT_DRIVER_PASSWORD without logging it"
fi

password_hash="$(
  PILOT_DRIVER_PASSWORD="${PILOT_DRIVER_PASSWORD}" docker run --rm -i \
    -e PILOT_DRIVER_PASSWORD \
    python:3.12-slim \
    python - <<'PY'
import bcrypt, os
print(bcrypt.hashpw(os.environ["PILOT_DRIVER_PASSWORD"].encode(), bcrypt.gensalt(rounds=12)).decode())
PY
)"

echo "=== BINTRANS pilot driver seed (idempotent) ==="
echo "Email alias: ${PILOT_DRIVER_EMAIL}"
echo "Target shipment number: ${PILOT_SHIPMENT_NUMBER}"

docker exec -i "${pg_cid}" psql -v ON_ERROR_STOP=1 -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" \
  -v pilot_email="${PILOT_DRIVER_EMAIL}" \
  -v pilot_name="${PILOT_DRIVER_FULL_NAME}" \
  -v pilot_hash="${password_hash}" \
  -v pilot_shipment="${PILOT_SHIPMENT_NUMBER}" <<'SQL'
WITH tenant AS (
  SELECT id AS tenant_id FROM core.tenants ORDER BY created_at LIMIT 1
),
carrier AS (
  SELECT c.id AS carrier_id, c.tenant_id
  FROM core.companies c
  JOIN tenant t ON c.tenant_id = t.tenant_id
  WHERE c.company_type = 'CARRIER'
  ORDER BY c.created_at
  LIMIT 1
),
driver_role AS (
  SELECT id AS role_id FROM core.roles WHERE name = 'Driver' LIMIT 1
),
upsert_user AS (
  INSERT INTO core.users (id, tenant_id, email, password_hash, full_name, status)
  SELECT gen_random_uuid(), carrier.tenant_id, :'pilot_email', :'pilot_hash', :'pilot_name', 'ACTIVE'
  FROM carrier
  ON CONFLICT (tenant_id, email) DO UPDATE
    SET password_hash = EXCLUDED.password_hash,
        full_name = EXCLUDED.full_name,
        status = 'ACTIVE',
        updated_at = now()
  RETURNING id, tenant_id
),
user_row AS (
  SELECT id, tenant_id FROM upsert_user
  UNION ALL
  SELECT u.id, u.tenant_id FROM core.users u
  JOIN carrier ON u.tenant_id = carrier.tenant_id
  WHERE u.email = :'pilot_email'
  LIMIT 1
),
grant_role AS (
  INSERT INTO core.user_roles (id, tenant_id, user_id, role_id)
  SELECT gen_random_uuid(), user_row.tenant_id, user_row.id, driver_role.role_id
  FROM user_row, driver_role
  ON CONFLICT DO NOTHING
  RETURNING user_id
),
upsert_driver AS (
  INSERT INTO transport.drivers (id, tenant_id, carrier_company_id, user_id, full_name, status)
  SELECT gen_random_uuid(), user_row.tenant_id, carrier.carrier_id, user_row.id, :'pilot_name', 'ACTIVE'
  FROM user_row, carrier
  ON CONFLICT ON CONSTRAINT uq_drivers_tenant_user_active DO UPDATE
    SET full_name = EXCLUDED.full_name,
        status = 'ACTIVE',
        updated_at = now()
  RETURNING id, tenant_id, user_id
),
driver_row AS (
  SELECT id, tenant_id, user_id FROM upsert_driver
  UNION ALL
  SELECT d.id, d.tenant_id, d.user_id
  FROM transport.drivers d
  JOIN user_row u ON d.user_id = u.id AND d.tenant_id = u.tenant_id
  WHERE d.deleted_at IS NULL
  LIMIT 1
),
target_shipment AS (
  SELECT s.id, s.tenant_id
  FROM transport.shipments s
  JOIN driver_row dr ON s.tenant_id = dr.tenant_id
  WHERE s.shipment_number = :'pilot_shipment'
  LIMIT 1
),
assign AS (
  UPDATE transport.shipments s
  SET driver_id = driver_row.id,
      status = CASE WHEN s.status IN ('ACCEPTED_BY_CARRIER', 'DRIVER_ASSIGNED') THEN 'PICKUP_SLOT_BOOKED' ELSE s.status END,
      updated_at = now(),
      version = s.version + 1
  FROM driver_row, target_shipment ts
  WHERE s.id = ts.id
  RETURNING s.id AS shipment_id
)
SELECT
  (SELECT tenant_id FROM tenant) AS tenant_id,
  (SELECT id FROM user_row) AS user_id,
  (SELECT id FROM driver_row) AS driver_id,
  (SELECT shipment_id FROM assign) AS shipment_id;
SQL

echo "bintrans-ct-staging-seed-pilot-driver: OK"
