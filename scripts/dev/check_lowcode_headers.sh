#!/usr/bin/env bash
# Dev check for low-code runtime headers contract.
set -euo pipefail

API_GATEWAY_URL="${API_GATEWAY_URL:-http://localhost:8080}"
TENANT_ID="${TENANT_ID:-74519f22-ff9b-4a8b-8fff-a958c689682f}"
DEMO_ORDER_ID="${DEMO_ORDER_ID:-2db04b49-665c-469f-bcb1-ffeb1274fedb}"
ACTIVE_TEMPLATE_ID="${ACTIVE_TEMPLATE_ID:-b1111111-1111-4111-8111-111111111102}"
REQUEST_ID="lowcode-header-check-$(date +%s)"

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

step() { echo -e "${BLUE}==>${NC} $1"; }
pass() { echo -e "${GREEN}OK:${NC} $1"; }
fail() { echo -e "${RED}ERROR:${NC} $1" >&2; exit 1; }

parse_http() {
  local raw="$1"
  HTTP_CODE="${raw##*__HTTP_CODE__:}"
  HTTP_BODY="${raw%__HTTP_CODE__:*}"
}

step "1. Without X-Tenant-ID → TENANT_REQUIRED"
parse_http "$(curl -sS -w "__HTTP_CODE__:%{http_code}" \
  "${API_GATEWAY_URL}/api/v1/low-code/form-templates/active?entity_type=TRANSPORT_ORDER")"
if [[ "$HTTP_CODE" != "400" ]]; then
  fail "expected 400, got ${HTTP_CODE}: ${HTTP_BODY}"
fi
if ! grep -q 'TENANT_REQUIRED' <<<"$HTTP_BODY"; then
  fail "expected TENANT_REQUIRED, got: ${HTTP_BODY}"
fi
pass "TENANT_REQUIRED without tenant header"

step "2. With X-Tenant-ID → OK"
parse_http "$(curl -sS -w "__HTTP_CODE__:%{http_code}" \
  -H "X-Tenant-ID: ${TENANT_ID}" \
  "${API_GATEWAY_URL}/api/v1/low-code/form-templates/active?entity_type=TRANSPORT_ORDER")"
if [[ "$HTTP_CODE" != "200" ]]; then
  fail "expected 200, got ${HTTP_CODE}: ${HTTP_BODY}"
fi
pass "active templates returned"

step "3. PUT custom-field-values with X-Request-ID → audit contains request_id"
PAYLOAD="$(cat <<EOF
{
  "entity_type": "TRANSPORT_ORDER",
  "entity_id": "${DEMO_ORDER_ID}",
  "form_template_id": "${ACTIVE_TEMPLATE_ID}",
  "values": [
    {
      "field_code": "internal_cost_center",
      "value_json": "HEADER-CHECK"
    }
  ]
}
EOF
)"
parse_http "$(printf '%s' "$PAYLOAD" | curl -sS -w "__HTTP_CODE__:%{http_code}" -X PUT \
  "${API_GATEWAY_URL}/api/v1/low-code/custom-field-values" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: ${TENANT_ID}" \
  -H "X-Request-ID: ${REQUEST_ID}" \
  --data-binary @-)"
if [[ "$HTTP_CODE" != "200" ]]; then
  fail "custom field PUT failed: ${HTTP_CODE} ${HTTP_BODY}"
fi
pass "custom field values saved"

parse_http "$(curl -sS -w "__HTTP_CODE__:%{http_code}" \
  -H "X-Tenant-ID: ${TENANT_ID}" \
  "${API_GATEWAY_URL}/api/v1/low-code/audit-events?entity_type=TRANSPORT_ORDER&entity_id=${DEMO_ORDER_ID}&limit=5")"
if [[ "$HTTP_CODE" != "200" ]]; then
  fail "audit list failed: ${HTTP_CODE} ${HTTP_BODY}"
fi
if ! grep -q "${REQUEST_ID}" <<<"$HTTP_BODY"; then
  fail "expected request_id ${REQUEST_ID} in audit response: ${HTTP_BODY}"
fi
pass "audit event includes request_id=${REQUEST_ID}"

echo
echo -e "${GREEN}LOWCODE HEADERS CHECK PASSED${NC}"
