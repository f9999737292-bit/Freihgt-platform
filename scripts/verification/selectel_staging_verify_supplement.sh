#!/usr/bin/env bash
set -euo pipefail
source /protected/bintrans/control-tower-observation/staging.env
GATEWAY_URL="${GATEWAY_URL:-http://127.0.0.1:18080}"
email="${BINTRANS_STAGING_AUTH_TEST_EMAIL:-}"
pass="${BINTRANS_STAGING_AUTH_TEST_PASSWORD:-}"
tenant_id=$(python3 - "${COHORT_MANIFEST}" <<'PY'
import json, sys
data=json.load(open(sys.argv[1]))
t=(data.get("tenants") or [{}])[0]
print(t.get("tenantId") or t.get("tenant_id") or "")
PY
)
login_tmp=$(mktemp)
body=$(python3 - <<PY
import json
print(json.dumps({"tenant_id":"${tenant_id}","email":"${email}","password":"${pass}"}))
PY
)
curl -sS -o "${login_tmp}" -H 'Content-Type: application/json' -d "${body}" "${GATEWAY_URL}/api/v1/auth/login" >/dev/null
JWT_TOKEN=$(python3 - "${login_tmp}" <<'PY'
import json,sys; print(json.load(open(sys.argv[1])).get("access_token",""))
PY
)
rm -f "${login_tmp}"
a=$(mktemp); b=$(mktemp)
curl -sS -o "${a}" -H "Authorization: Bearer ${JWT_TOKEN}" "${GATEWAY_URL}/api/v1/control-tower/summary" >/dev/null
curl -sS -o "${b}" -H "Authorization: Bearer ${JWT_TOKEN}" -H 'X-Tenant-ID: 00000000-0000-0000-0000-000000000099' "${GATEWAY_URL}/api/v1/control-tower/summary" >/dev/null
python3 - "${a}" "${b}" <<'PY'
import json, sys, hashlib
def digest(path):
    data=json.load(open(path))
    # Compare tenant-scoped summary fields only
    key={
        "source": (data.get("statusSummary") or {}).get("source"),
        "criticalEvents": len(data.get("criticalEvents") or []),
        "activeShipments": len(data.get("activeShipments") or []),
        "kpis": len(data.get("kpis") or []),
    }
    return hashlib.sha256(json.dumps(key, sort_keys=True).encode()).hexdigest()[:16]
d1=digest(sys.argv[1]); d2=digest(sys.argv[2])
print("UNTRUSTED_HEADER_CHANGES_SUMMARY=" + ("NO" if d1==d2 else "YES"))
PY
ship_tmp=$(mktemp)
ship_code=$(curl -sS -o "${ship_tmp}" -w '%{http_code}' -H "Authorization: Bearer ${JWT_TOKEN}" "${GATEWAY_URL}/api/v1/shipments?limit=5")
echo "SHIPMENTS_LIST_HTTP=${ship_code}"
python3 - "${ship_tmp}" <<'PY'
import json, sys
try:
    data=json.load(open(sys.argv[1]))
    items=data.get("items") or data.get("shipments") or data.get("data") or []
    print("shipments_count=" + str(len(items)))
    if items and isinstance(items[0], dict):
        sid=items[0].get("id","")
        print("sample_shipment_present=" + ("YES" if sid else "NO"))
except Exception:
    print("shipments_parse=FAIL")
PY
if python3 - "${ship_tmp}" <<'PY' | grep -q sample_shipment_present=YES
import json,sys
items=(json.load(open(sys.argv[1])).get("items") or [])
print("sample_shipment_present=" + ("YES" if items else "NO"))
PY
then
  sid=$(python3 - "${ship_tmp}" <<'PY'
import json,sys
items=(json.load(open(sys.argv[1])).get("items") or [])
print(items[0].get("id","") if items else "")
PY
)
  ev_tmp=$(mktemp)
  ev_code=$(curl -sS -o "${ev_tmp}" -w '%{http_code}' -H "Authorization: Bearer ${JWT_TOKEN}" "${GATEWAY_URL}/api/v1/shipments/${sid}/events")
  echo "LIST_OWN_EVENTS_HTTP=${ev_code}"
  python3 - "${ev_tmp}" <<'PY'
import json,sys
try:
    events=(json.load(open(sys.argv[1])).get("events") or [])
    print("list_eventCount="+str(len(events)))
    print("list_hasDerived="+str(any(bool(e.get("derived")) for e in events if isinstance(e,dict))).lower())
except Exception:
    print("list_events_parse=FAIL")
PY
  rm -f "${ev_tmp}"
fi
rm -f "${a}" "${b}" "${ship_tmp}"
unset JWT_TOKEN BINTRANS_STAGING_AUTH_TEST_PASSWORD
