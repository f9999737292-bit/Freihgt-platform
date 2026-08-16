#!/usr/bin/env bash
# BINTRANS CT staging — driver auth + delay/problem smoke (no secrets printed).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

bintrans_require_env_file

PASS_FILE="/protected/bintrans/control-tower-observation/pilot-driver.password"
[[ -f "${PASS_FILE}" ]] || bintrans_fail "pilot password file missing — run seed first"

TENANT="${PILOT_TENANT_ID:-873b3fbc-3cb4-413f-81cd-6fa2c94e785e}"
SHIP="${PILOT_SHIPMENT_ID:-c746460c-0649-4f0c-9233-f11d5da29aa7}"
EMAIL="${PILOT_DRIVER_EMAIL:-driver-pilot-p0@test.local}"
GATEWAY="${GATEWAY_URL:-http://127.0.0.1:18080}"

PASS="$(cat "${PASS_FILE}")"

curl -sS -X POST "${GATEWAY}/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"tenant_id\":\"${TENANT}\",\"email\":\"${EMAIL}\",\"password\":\"${PASS}\"}" > /tmp/pilot-login.json

python3 - <<'PY'
import json
d = json.load(open("/tmp/pilot-login.json"))
token = d.get("access_token")
if not token:
    print("LOGIN_HTTP_STATUS=FAIL")
    raise SystemExit(1)
open("/tmp/pilot.jwt", "w").write(token)
print("LOGIN_HTTP_STATUS=200")
print("JWT_ACQUIRED=YES")
PY

JWT="$(cat /tmp/pilot.jwt)"

check_route() {
  local name="$1"
  local route="$2"
  local code
  code="$(curl -sS -o "/tmp/${name}.json" -w "%{http_code}" -H "Authorization: Bearer ${JWT}" "${GATEWAY}${route}")"
  echo "${name}_HTTP=${code}"
}

check_route DRIVER_ME /api/v1/driver/me
check_route MY_SHIPMENTS /api/v1/driver/me/shipments
check_route SHIPMENT_DETAIL "/api/v1/driver/me/shipments/${SHIP}"

DELAY_KEY="driver-mobile-op:delay:e2e-v092-$(date +%s)"
DELAY_HTTP="$(curl -sS -o /tmp/delay.json -w "%{http_code}" -X POST \
  "${GATEWAY}/api/v1/driver/me/shipments/${SHIP}/delays" \
  -H "Authorization: Bearer ${JWT}" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: ${DELAY_KEY}" \
  -d "{\"reasonCode\":\"TRAFFIC\",\"reasonText\":\"E2E delay v0.9.2\",\"occurredAt\":\"2026-08-16T13:30:00Z\",\"idempotencyKey\":\"${DELAY_KEY}\"}")"
echo "DELAY_HTTP=${DELAY_HTTP}"
echo "IDEMPOTENCY_KEY_DELAY=${DELAY_KEY}"

PROBLEM_KEY="driver-mobile-op:problem:e2e-v092-$(date +%s)"
PROBLEM_HTTP="$(curl -sS -o /tmp/problem.json -w "%{http_code}" -X POST \
  "${GATEWAY}/api/v1/driver/me/shipments/${SHIP}/exceptions" \
  -H "Authorization: Bearer ${JWT}" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: ${PROBLEM_KEY}" \
  -d "{\"category\":\"TRAFFIC\",\"comment\":\"E2E problem v0.9.2\",\"occurredAt\":\"2026-08-16T13:31:00Z\",\"idempotencyKey\":\"${PROBLEM_KEY}\"}")"
echo "PROBLEM_HTTP=${PROBLEM_HTTP}"
echo "IDEMPOTENCY_KEY_PROBLEM=${PROBLEM_KEY}"

RETRY_HTTP="$(curl -sS -o /tmp/delay-retry.json -w "%{http_code}" -X POST \
  "${GATEWAY}/api/v1/driver/me/shipments/${SHIP}/delays" \
  -H "Authorization: Bearer ${JWT}" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: ${DELAY_KEY}" \
  -d "{\"reasonCode\":\"TRAFFIC\",\"reasonText\":\"E2E delay v0.9.2\",\"occurredAt\":\"2026-08-16T13:30:00Z\",\"idempotencyKey\":\"${DELAY_KEY}\"}")"
echo "RETRY_HTTP=${RETRY_HTTP}"

python3 - <<'PY'
import json
for path, label in [("/tmp/delay.json","DELAY"), ("/tmp/problem.json","PROBLEM"), ("/tmp/delay-retry.json","RETRY")]:
    d = json.load(open(path))
    print(f"{label}_OUTBOX_EVENT_ID=", d.get("outboxEventId"))
    print(f"{label}_REPLAYED=", d.get("replayed"))
PY

pg_cid="$(bintrans_postgres_container)"
docker exec "${pg_cid}" psql -U "${POSTGRES_USER:-bintrans_staging}" -d "${POSTGRES_DB:-freight_platform}" -tAc \
  "SELECT count(*) FROM control_tower.driver_event_inbox" | awk '{print "DRIVER_EVENT_INBOX_COUNT="$1}'

echo "bintrans-ct-staging-driver-e2e-smoke: DONE"
