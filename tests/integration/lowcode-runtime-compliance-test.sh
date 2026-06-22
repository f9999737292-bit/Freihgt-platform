#!/usr/bin/env bash
# Verifies low-code runtime integration guardrails:
# - custom field PUT does not change core transport order status
# - entity detail panel source does not call core entity write APIs
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

API_GATEWAY_URL="${API_GATEWAY_URL:-http://localhost:8080}"
TENANT_ID="${TENANT_ID:-74519f22-ff9b-4a8b-8fff-a958c689682f}"
DEMO_ORDER_REF="${DEMO_ORDER_REF:-DEMO-TO-001}"

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

step() { echo -e "${BLUE}==>${NC} $1"; }
pass() { echo -e "${GREEN}OK:${NC} $1"; }
fail() { echo -e "${RED}ERROR:${NC} $1" >&2; exit 1; }

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    fail "$1 is required but not installed"
  fi
}

parse_http() {
  local raw="$1"
  HTTP_CODE="${raw##*__HTTP_CODE__:}"
  HTTP_BODY="${raw%__HTTP_CODE__:*}"
}

json_field() {
  python - "$1" "$2" <<'PY'
import json, sys
data = json.loads(sys.argv[1])
path = sys.argv[2].split(".")
cur = data
for key in path:
    if key == "":
        continue
    if isinstance(cur, list):
        cur = cur[int(key)]
    elif isinstance(cur, dict):
        cur = cur.get(key)
    else:
        cur = None
        break
if cur is None:
    raise SystemExit(1)
print(cur)
PY
}

step "Static check: LowCodeCustomFieldsPanel does not call core entity write APIs"
PANEL_FILE="${ROOT_DIR}/apps/web-admin/components/low-code/LowCodeCustomFieldsPanel.vue"
if [[ ! -f "$PANEL_FILE" ]]; then
  fail "panel file not found: $PANEL_FILE"
fi
if grep -E 'api(Put|Post|Patch)\([^)]*/(transport-orders|shipments|billing-registers|documents|rfx|freight-requests)' "$PANEL_FILE" >/dev/null 2>&1; then
  fail "LowCodeCustomFieldsPanel must not call core entity write APIs"
fi
if ! grep -q 'saveCustomFieldValues' "$PANEL_FILE"; then
  fail "LowCodeCustomFieldsPanel must save via saveCustomFieldValues"
fi
pass "LowCodeCustomFieldsPanel uses low-code save API only"

step "Resolve demo transport order ${DEMO_ORDER_REF}"
parse_http "$(curl -sS -w "__HTTP_CODE__:%{http_code}" \
  -H "X-Tenant-ID: ${TENANT_ID}" \
  "${API_GATEWAY_URL}/api/v1/transport-orders?tenant_id=${TENANT_ID}&limit=100")"
if [[ "$HTTP_CODE" != "200" ]]; then
  fail "transport orders list failed: HTTP ${HTTP_CODE} ${HTTP_BODY}"
fi
ORDER_ID="$(python - "$HTTP_BODY" "$DEMO_ORDER_REF" <<'PY'
import json, sys
items = json.loads(sys.argv[1]).get("items") or []
ref = sys.argv[2]
for item in items:
    if item.get("order_number") == ref:
        print(item["id"])
        break
PY
)" || true
if [[ -z "${ORDER_ID:-}" ]]; then
  fail "demo transport order ${DEMO_ORDER_REF} not found; run make seed-demo-data"
fi
pass "ORDER_ID=${ORDER_ID}"

step "Load core transport order status (before)"
parse_http "$(curl -sS -w "__HTTP_CODE__:%{http_code}" \
  -H "X-Tenant-ID: ${TENANT_ID}" \
  "${API_GATEWAY_URL}/api/v1/transport-orders/${ORDER_ID}?tenant_id=${TENANT_ID}")"
if [[ "$HTTP_CODE" != "200" ]]; then
  fail "get transport order failed: HTTP ${HTTP_CODE}"
fi
STATUS_BEFORE="$(json_field "$HTTP_BODY" "status")"
pass "status before=${STATUS_BEFORE}"

step "Resolve active form template"
parse_http "$(curl -sS -w "__HTTP_CODE__:%{http_code}" \
  -H "X-Tenant-ID: ${TENANT_ID}" \
  "${API_GATEWAY_URL}/api/v1/low-code/form-templates/active?entity_type=TRANSPORT_ORDER")"
if [[ "$HTTP_CODE" != "200" ]]; then
  fail "active template failed: HTTP ${HTTP_CODE} ${HTTP_BODY}"
fi
ACTIVE_TEMPLATE_ID="$(json_field "$HTTP_BODY" "items.0.id")"
pass "active template=${ACTIVE_TEMPLATE_ID}"

step "PUT custom field value via low-code API"
PAYLOAD="$(cat <<EOF
{
  "entity_type": "TRANSPORT_ORDER",
  "entity_id": "${ORDER_ID}",
  "form_template_id": "${ACTIVE_TEMPLATE_ID}",
  "validation_context": {
    "entity_status": "${STATUS_BEFORE}",
    "role": "PLATFORM_ADMIN"
  },
  "values": [
    {
      "field_code": "internal_cost_center",
      "value_json": "COMPLIANCE-TEST"
    }
  ]
}
EOF
)"
parse_http "$(printf '%s' "$PAYLOAD" | curl -sS -w "__HTTP_CODE__:%{http_code}" -X PUT \
  "${API_GATEWAY_URL}/api/v1/low-code/custom-field-values" \
  -H "Content-Type: application/json; charset=utf-8" \
  -H "X-Tenant-ID: ${TENANT_ID}" \
  -H "X-Low-Code-Entity-Status: ${STATUS_BEFORE}" \
  -H "X-Low-Code-Role: PLATFORM_ADMIN" \
  --data-binary @-)"
if [[ "$HTTP_CODE" != "200" ]]; then
  fail "custom field upsert failed: HTTP ${HTTP_CODE} ${HTTP_BODY}"
fi
pass "custom field values saved"

step "Verify core transport order status unchanged"
parse_http "$(curl -sS -w "__HTTP_CODE__:%{http_code}" \
  -H "X-Tenant-ID: ${TENANT_ID}" \
  "${API_GATEWAY_URL}/api/v1/transport-orders/${ORDER_ID}?tenant_id=${TENANT_ID}")"
if [[ "$HTTP_CODE" != "200" ]]; then
  fail "get transport order after save failed: HTTP ${HTTP_CODE}"
fi
STATUS_AFTER="$(json_field "$HTTP_BODY" "status")"
if [[ "$STATUS_BEFORE" != "$STATUS_AFTER" ]]; then
  fail "core status changed: ${STATUS_BEFORE} -> ${STATUS_AFTER}"
fi
pass "core status unchanged (${STATUS_AFTER})"

step "Verify custom field value persisted"
parse_http "$(curl -sS -w "__HTTP_CODE__:%{http_code}" \
  -H "X-Tenant-ID: ${TENANT_ID}" \
  "${API_GATEWAY_URL}/api/v1/low-code/custom-field-values?entity_type=TRANSPORT_ORDER&entity_id=${ORDER_ID}")"
if [[ "$HTTP_CODE" != "200" ]]; then
  fail "get custom field values failed: HTTP ${HTTP_CODE}"
fi
if ! grep -q 'COMPLIANCE-TEST' <<<"$HTTP_BODY"; then
  fail "expected custom field value in response"
fi
pass "custom field value persisted"

echo
echo -e "${GREEN}LOWCODE RUNTIME COMPLIANCE TEST PASSED${NC}"
