#!/usr/bin/env bash
# Read-only Selectel staging verification (runs on staging VM).
# Credentials from protected env only; stdout is redacted metadata.
set -euo pipefail

cd /opt/bintrans/control-tower-staging
set -a
# shellcheck disable=SC1091
source /protected/bintrans/control-tower-observation/staging.env
set +a

GATEWAY_URL="${GATEWAY_URL:-http://127.0.0.1:18080}"
login_tmp=$(mktemp)
sum_tmp=$(mktemp)
JWT_TOKEN="${JWT_TOKEN:-}"
login_code=000

json_field() {
  python3 - "$1" "$2" <<'PY'
import json, sys
path = sys.argv[2].split(".")
try:
    data = json.load(open(sys.argv[1], encoding="utf-8"))
except Exception:
    print("")
    raise SystemExit(0)
cur = data
for part in path:
    if part == "":
        continue
    if isinstance(cur, dict):
        cur = cur.get(part)
    else:
        cur = None
        break
if cur is None:
    print("")
elif isinstance(cur, bool):
    print("true" if cur else "false")
else:
    print(cur)
PY
}

if [[ -z "${JWT_TOKEN}" ]]; then
  email="${DEV_ADMIN_EMAIL:-${ADMIN_EMAIL:-${BINTRANS_STAGING_AUTH_TEST_EMAIL:-}}}"
  pass="${DEV_ADMIN_PASSWORD:-${ADMIN_PASSWORD:-${BINTRANS_STAGING_AUTH_TEST_PASSWORD:-}}}"
  tenant_id="${TENANT_ID:-}"
  if [[ -z "${tenant_id}" && -n "${COHORT_MANIFEST:-}" && -f "${COHORT_MANIFEST}" ]]; then
    tenant_id=$(python3 - "${COHORT_MANIFEST}" <<'PY'
import json, sys
try:
    data = json.load(open(sys.argv[1], encoding="utf-8"))
    tenants = data.get("tenants") or []
    if tenants:
        t = tenants[0]
        print(t.get("tenantId") or t.get("tenant_id") or "")
except Exception:
    print("")
PY
)
  fi
  if [[ -n "${email}" && -n "${pass}" && -n "${tenant_id}" ]]; then
    login_body=$(python3 - <<PY
import json
print(json.dumps({"tenant_id": "${tenant_id}", "email": "${email}", "password": "${pass}"}))
PY
)
    login_code=$(curl -sS -o "${login_tmp}" -w '%{http_code}' \
      -H 'Content-Type: application/json' -d "${login_body}" \
      "${GATEWAY_URL}/api/v1/auth/login")
    JWT_TOKEN=$(python3 - "${login_tmp}" <<'PY'
import json, sys
try:
    data = json.load(open(sys.argv[1], encoding="utf-8"))
    print(data.get("access_token", "") or "")
except Exception:
    print("")
PY
)
  fi
fi
rm -f "${login_tmp}"

if [[ -z "${JWT_TOKEN}" ]]; then
  echo "AUTH_LOGIN=FAIL code=${login_code}"
  exit 0
fi

echo "AUTH_LOGIN=PASS code=${login_code}"

min=999999
max=0
sum=0
code=000
for i in 1 2 3; do
  t1=$(date +%s%3N)
  code=$(curl -sS -o "${sum_tmp}" -w '%{http_code}' \
    -H "Authorization: Bearer ${JWT_TOKEN}" \
    "${GATEWAY_URL}/api/v1/control-tower/summary")
  t2=$(date +%s%3N)
  ms=$((t2 - t1))
  sum=$((sum + ms))
  (( ms < min )) && min=$ms
  (( ms > max )) && max=$ms
  echo "CT_SUMMARY_RUN_${i}=${code} latency_ms=${ms}"
done
avg=$((sum / 3))
echo "CT_LATENCY_MIN=${min} CT_LATENCY_AVG=${avg} CT_LATENCY_MAX=${max}"

if [[ "${code}" == "200" ]]; then
  python3 - "${sum_tmp}" <<'PY'
import json, sys
data = json.load(open(sys.argv[1], encoding="utf-8"))
ss = data.get("statusSummary") or {}
sf = data.get("statusSummaryFreshness") or {}
shipments = data.get("activeShipments") or []
print("source=" + str(ss.get("source", "")))
print("limitedDataset=" + str(bool(ss.get("limitedDataset", False))).lower())
print("fallbackUsed=" + str(bool(sf.get("fallbackUsed", False))).lower())
print("kpiCount=" + str(len(data.get("kpis") or [])))
print("activeShipments=" + str(len(shipments)))
print("criticalEvents=" + str(len(data.get("criticalEvents") or [])))
print("hasDemoIds=" + str(any(str(s.get("id", "")).upper().startswith("DEMO-") for s in shipments if isinstance(s, dict))).lower())
PY
fi

foreign_id="00000000-0000-0000-0000-000000000099"
echo "FOREIGN_SHIPMENT_HTTP=$(curl -sS -o /dev/null -w '%{http_code}' \
  -H "Authorization: Bearer ${JWT_TOKEN}" \
  -H 'X-Tenant-ID: 00000000-0000-0000-0000-000000000099' \
  "${GATEWAY_URL}/api/v1/shipments/${foreign_id}")"
echo "FOREIGN_EVENTS_HTTP=$(curl -sS -o /dev/null -w '%{http_code}' \
  -H "Authorization: Bearer ${JWT_TOKEN}" \
  -H 'X-Tenant-ID: 00000000-0000-0000-0000-000000000099' \
  "${GATEWAY_URL}/api/v1/shipments/${foreign_id}/events")"
echo "QUERY_TENANT_BYPASS_HTTP=$(curl -sS -o /dev/null -w '%{http_code}' \
  -H "Authorization: Bearer ${JWT_TOKEN}" \
  "${GATEWAY_URL}/api/v1/shipments/${foreign_id}?tenant_id=00000000-0000-0000-0000-000000000099")"
echo "UNTRUSTED_TENANT_SUMMARY_HTTP=$(curl -sS -o /dev/null -w '%{http_code}' \
  -H "Authorization: Bearer ${JWT_TOKEN}" \
  -H 'X-Tenant-ID: 00000000-0000-0000-0000-000000000099' \
  "${GATEWAY_URL}/api/v1/control-tower/summary")"
echo "FOREIGN_DRIVER_HTTP=$(curl -sS -o /dev/null -w '%{http_code}' \
  -H "Authorization: Bearer ${JWT_TOKEN}" \
  -H 'X-Tenant-ID: 00000000-0000-0000-0000-000000000099' \
  "${GATEWAY_URL}/api/v1/drivers/00000000-0000-0000-0000-000000000099")"
echo "FOREIGN_VEHICLE_HTTP=$(curl -sS -o /dev/null -w '%{http_code}' \
  -H "Authorization: Bearer ${JWT_TOKEN}" \
  -H 'X-Tenant-ID: 00000000-0000-0000-0000-000000000099' \
  "${GATEWAY_URL}/api/v1/vehicles/00000000-0000-0000-0000-000000000099")"
echo "CT_FILTER_HTTP=$(curl -sS -o /dev/null -w '%{http_code}' \
  -H "Authorization: Bearer ${JWT_TOKEN}" \
  "${GATEWAY_URL}/api/v1/control-tower/summary?status=IN_TRANSIT")"

own_id=$(python3 - "${sum_tmp}" <<'PY'
import json, sys
try:
    data = json.load(open(sys.argv[1], encoding="utf-8"))
    shipments = data.get("activeShipments") or []
    print(shipments[0].get("id", "") if shipments else "")
except Exception:
    print("")
PY
)
if [[ -n "${own_id}" ]]; then
  ev_tmp=$(mktemp)
  ev_code=$(curl -sS -o "${ev_tmp}" -w '%{http_code}' \
    -H "Authorization: Bearer ${JWT_TOKEN}" \
    "${GATEWAY_URL}/api/v1/shipments/${own_id}/events")
  echo "OWN_EVENTS_HTTP=${ev_code}"
  python3 - "${ev_tmp}" <<'PY'
import json, sys
try:
    data = json.load(open(sys.argv[1], encoding="utf-8"))
    events = data.get("events") or []
    print("eventCount=" + str(len(events)))
    print("hasDerived=" + str(any(bool(e.get("derived")) for e in events if isinstance(e, dict))).lower())
    print("hasProvenance=" + str(any("provenance" in e for e in events if isinstance(e, dict))).lower())
except Exception:
    print("OWN_EVENTS_PARSE=FAIL")
PY
  rm -f "${ev_tmp}"
else
  echo "OWN_EVENTS_HTTP=SKIP_NO_SHIPMENTS"
fi

unset JWT_TOKEN ADMIN_PASSWORD DEV_ADMIN_PASSWORD BINTRANS_STAGING_AUTH_TEST_PASSWORD
rm -f "${sum_tmp}"
