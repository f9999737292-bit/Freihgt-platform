#!/usr/bin/env bash
# Read-only verification of staging demo seed (no writes).
set -euo pipefail

API="${API_GATEWAY_URL:-http://161.104.53.221}"
TENANT="${TENANT_ID:-74519f22-ff9b-4a8b-8fff-a958c689682f}"

check() {
  local id="$1" url="$2" expect="$3"
  local code body
  code="$(curl -sS -o /tmp/fp_verify_body.txt -w "%{http_code}" -H "X-Tenant-ID: ${TENANT}" "$url" || echo "000")"
  body="$(cat /tmp/fp_verify_body.txt 2>/dev/null || true)"
  if [[ "$code" == "$expect" ]] && echo "$body" | grep -q "DEMO-"; then
    echo "PASS ${id} code=${code}"
  elif [[ "$code" == "$expect" ]]; then
    echo "CHECK ${id} code=${code} (no DEMO- prefix in body)"
  else
    echo "FAIL ${id} code=${code} expected=${expect}"
  fi
}

echo "=== VERIFY ${API} tenant=${TENANT} ==="
curl -sf "${API}/health" >/dev/null && echo "PASS VFY-001 health=200" || echo "FAIL VFY-001 health"

check "VFY-002" "${API}/api/v1/transport-orders?tenant_id=${TENANT}&limit=10" "200"
check "VFY-003" "${API}/api/v1/shipments?tenant_id=${TENANT}&limit=10" "200"
check "VFY-004" "${API}/api/v1/billing-registers?tenant_id=${TENANT}&limit=10" "200"

code="$(curl -sS -o /tmp/fp_cf.txt -w "%{http_code}" -H "X-Tenant-ID: ${TENANT}" \
  "${API}/api/v1/low-code/form-templates/active?entity_type=TRANSPORT_ORDER" || echo "000")"
[[ "$code" == "200" ]] && echo "PASS VFY-005 runtime_template=${code}" || echo "FAIL VFY-005 runtime_template=${code}"

echo "=== done (read-only) ==="
