#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

if [[ -f "$SCRIPT_DIR/.env" ]]; then
  # shellcheck disable=SC1091
  source "$SCRIPT_DIR/.env"
fi

IDENTITY_SERVICE_URL="${IDENTITY_SERVICE_URL:-http://localhost:8081}"
COMPANY_SERVICE_URL="${COMPANY_SERVICE_URL:-http://localhost:8082}"
TRANSPORT_ORDER_SERVICE_URL="${TRANSPORT_ORDER_SERVICE_URL:-http://localhost:8083}"
RFX_SERVICE_URL="${RFX_SERVICE_URL:-http://localhost:8084}"
SHIPMENT_SERVICE_URL="${SHIPMENT_SERVICE_URL:-http://localhost:8085}"
DOCUMENT_SERVICE_URL="${DOCUMENT_SERVICE_URL:-http://localhost:8086}"
BILLING_REGISTER_SERVICE_URL="${BILLING_REGISTER_SERVICE_URL:-http://localhost:8087}"

POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-freight_postgres}"
POSTGRES_USER="${POSTGRES_USER:-freight}"
POSTGRES_DB="${POSTGRES_DB:-freight_platform}"
TENANT_CODE="${TENANT_CODE:-test-tenant}"
TENANT_NAME="${TENANT_NAME:-Test Tenant}"
SMOKE_RUN_ID="${SMOKE_RUN_ID:-$(date +%Y%m%d%H%M%S)}"

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

step() {
  echo -e "${BLUE}==>${NC} $1"
}

pass() {
  echo -e "${GREEN}OK:${NC} $1"
}

fail() {
  echo -e "${RED}ERROR:${NC} $1" >&2
  exit 1
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    fail "$1 is required but not installed"
  fi
}

# Git Bash on Windows: prefer docker.exe (MSYS docker wrapper may fail with "Resource temporarily unavailable").
docker_cmd() {
  local win_docker="/c/Program Files/Docker/Docker/resources/bin/docker.exe"
  if [[ -x "$win_docker" ]]; then
    "$win_docker" "$@"
  else
    docker "$@"
  fi
}

parse_http() {
  local raw="$1"
  HTTP_CODE="${raw##*__HTTP_CODE__:}"
  HTTP_BODY="${raw%__HTTP_CODE__:*}"
}

api_request() {
  local method="$1"
  local url="$2"
  local data="${3:-}"
  local raw

  if [[ -n "$data" ]]; then
    raw="$(printf '%s' "$data" | curl -sS -w "__HTTP_CODE__:%{http_code}" -X "$method" "$url" \
      -H "Content-Type: application/json; charset=utf-8" \
      --data-binary @-)"
  else
    raw="$(curl -sS -w "__HTTP_CODE__:%{http_code}" -X "$method" "$url")"
  fi

  parse_http "$raw"

  if [[ "$HTTP_CODE" -lt 200 || "$HTTP_CODE" -ge 300 ]]; then
    echo "$HTTP_BODY" | jq . 2>/dev/null || echo "$HTTP_BODY"
    fail "Request failed: $method $url (HTTP $HTTP_CODE)"
  fi

  echo "$HTTP_BODY"
}

assert_json_field() {
  local json="$1"
  local jq_expr="$2"
  local expected="$3"
  local label="$4"
  local actual

  actual="$(echo "$json" | jq -r "$jq_expr")"
  if [[ "$actual" != "$expected" ]]; then
    echo "$json" | jq . 2>/dev/null || echo "$json"
    fail "Expected $label=$expected, got $actual"
  fi
}

assert_json_number() {
  local json="$1"
  local jq_expr="$2"
  local expected="$3"
  local label="$4"

  echo "$json" | jq -e "$jq_expr | tonumber == $expected" >/dev/null || {
    echo "$json" | jq . 2>/dev/null || echo "$json"
    fail "Expected $label=$expected"
  }
}

check_health() {
  local service_name="$1"
  local url="$2"
  local port="$3"
  local raw http_code body

  raw="$(curl -sS -w "__HTTP_CODE__:%{http_code}" "$url/health" 2>/dev/null || true)"
  if [[ -z "$raw" ]]; then
    fail "Service $service_name is not running on port $port"
  fi

  parse_http "$raw"
  if [[ "$HTTP_CODE" -ne 200 ]]; then
    fail "Service $service_name is not running on port $port"
  fi

  body="$(echo "$HTTP_BODY" | jq -r '.status // empty')"
  if [[ "$body" != "ok" ]]; then
    fail "Service $service_name health check failed on port $port"
  fi

  pass "$service_name is healthy on port $port"
}

ensure_tenant() {
  step "Create or reuse test tenant ($TENANT_CODE)"

  if [[ -n "${TENANT_ID:-}" ]]; then
    pass "TENANT_ID=$TENANT_ID (from env)"
    return 0
  fi

  if ! docker_cmd inspect "$POSTGRES_CONTAINER" >/dev/null 2>&1; then
    fail "PostgreSQL container '$POSTGRES_CONTAINER' is not running. Run: make dev-up"
  fi

  TENANT_ID="$(docker_cmd exec -i "$POSTGRES_CONTAINER" psql -q -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tA -c "
    INSERT INTO core.tenants (code, name, country_code, default_locale, default_currency)
    VALUES ('${TENANT_CODE}', '${TENANT_NAME}', 'RU', 'ru-RU', 'RUB')
    ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name
    RETURNING id;
  " | grep -Eo '[0-9a-fA-F-]{36}' | head -n 1)"

  if [[ -z "$TENANT_ID" ]]; then
    TENANT_ID="$(docker_cmd exec -i "$POSTGRES_CONTAINER" psql -q -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tA -c \
      "SELECT id FROM core.tenants WHERE code = '${TENANT_CODE}' AND deleted_at IS NULL LIMIT 1;" | grep -Eo '[0-9a-fA-F-]{36}' | head -n 1)"
  fi

  if [[ -z "$TENANT_ID" ]]; then
    fail "Failed to create or load tenant $TENANT_CODE"
  fi

  pass "TENANT_ID=$TENANT_ID"
}

create_company() {
  local legal_name="$1"
  local company_type="$2"
  local body resp

  body="$(cat <<EOF
{
  "tenant_id": "$TENANT_ID",
  "legal_name": "$legal_name",
  "company_type": "$company_type",
  "country_code": "RU",
  "preferred_locale": "ru-RU"
}
EOF
)"

  resp="$(api_request POST "$COMPANY_SERVICE_URL/v1/companies" "$body")"
  echo "$resp" | jq -r '.id'
}

patch_shipment_status() {
  local status="$1"
  local body

  if [[ "$status" == "LOADED" || "$status" == "DELIVERED" ]]; then
    body="$(cat <<EOF
{
  "tenant_id": "$TENANT_ID",
  "status": "$status",
  "actual_time": "2026-07-01T12:00:00Z"
}
EOF
)"
  else
    body="$(cat <<EOF
{
  "tenant_id": "$TENANT_ID",
  "status": "$status"
}
EOF
)"
  fi

  SHIPMENT_JSON="$(api_request PATCH "$SHIPMENT_SERVICE_URL/v1/shipments/$SHIPMENT_ID/status" "$body")"
  assert_json_field "$SHIPMENT_JSON" '.status' "$status" "shipment status after $status"
  pass "Shipment status -> $status"
}

require_cmd jq
require_cmd curl
require_cmd docker

echo ""
echo "Freight Platform Integration Smoke Test"
echo "Run ID: TEST-$SMOKE_RUN_ID"
echo ""

step "Check health of all services"
check_health "identity-service" "$IDENTITY_SERVICE_URL" "8081"
check_health "company-service" "$COMPANY_SERVICE_URL" "8082"
check_health "transport-order-service" "$TRANSPORT_ORDER_SERVICE_URL" "8083"
check_health "rfx-service" "$RFX_SERVICE_URL" "8084"
check_health "shipment-service" "$SHIPMENT_SERVICE_URL" "8085"
check_health "document-service" "$DOCUMENT_SERVICE_URL" "8086"
check_health "billing-register-service" "$BILLING_REGISTER_SERVICE_URL" "8087"

ensure_tenant

step "Create shipper company"
SHIPPER_COMPANY_ID="$(create_company "ООО Грузоотправитель Тест" "SHIPPER")"
pass "SHIPPER_COMPANY_ID=$SHIPPER_COMPANY_ID"

step "Create consignee company"
CONSIGNEE_COMPANY_ID="$(create_company "ООО Грузополучатель Тест" "CONSIGNEE")"
pass "CONSIGNEE_COMPANY_ID=$CONSIGNEE_COMPANY_ID"

step "Create carrier company"
CARRIER_COMPANY_ID="$(create_company "ООО Перевозчик Тест" "CARRIER")"
pass "CARRIER_COMPANY_ID=$CARRIER_COMPANY_ID"

step "Create user via identity-service"
USER_JSON="$(api_request POST "$IDENTITY_SERVICE_URL/v1/users" "$(cat <<EOF
{
  "tenant_id": "$TENANT_ID",
  "email": "logist-${SMOKE_RUN_ID}@test.local",
  "password": "StrongPassword123!",
  "full_name": "Тестовый Логист",
  "preferred_locale": "ru-RU"
}
EOF
)")"
USER_ID="$(echo "$USER_JSON" | jq -r '.id')"
pass "USER_ID=$USER_ID"

step "Add user membership to shipper company"
MEMBERSHIP_JSON="$(api_request POST "$COMPANY_SERVICE_URL/v1/companies/$SHIPPER_COMPANY_ID/members" "$(cat <<EOF
{
  "tenant_id": "$TENANT_ID",
  "user_id": "$USER_ID",
  "position": "Логист"
}
EOF
)")"
pass "Membership created: $(echo "$MEMBERSHIP_JSON" | jq -r '.id')"

step "Create origin location (Склад Москва)"
ORIGIN_JSON="$(api_request POST "$TRANSPORT_ORDER_SERVICE_URL/v1/locations" "$(cat <<EOF
{
  "tenant_id": "$TENANT_ID",
  "company_id": "$SHIPPER_COMPANY_ID",
  "location_type": "WAREHOUSE",
  "name": "Склад Москва TEST",
  "country_code": "RU",
  "city": "Москва",
  "timezone": "Europe/Moscow"
}
EOF
)")"
ORIGIN_LOCATION_ID="$(echo "$ORIGIN_JSON" | jq -r '.id')"
pass "ORIGIN_LOCATION_ID=$ORIGIN_LOCATION_ID"

step "Create destination location (Склад Казань)"
DESTINATION_JSON="$(api_request POST "$TRANSPORT_ORDER_SERVICE_URL/v1/locations" "$(cat <<EOF
{
  "tenant_id": "$TENANT_ID",
  "company_id": "$CONSIGNEE_COMPANY_ID",
  "location_type": "WAREHOUSE",
  "name": "Склад Казань TEST",
  "country_code": "RU",
  "city": "Казань",
  "timezone": "Europe/Moscow"
}
EOF
)")"
DESTINATION_LOCATION_ID="$(echo "$DESTINATION_JSON" | jq -r '.id')"
pass "DESTINATION_LOCATION_ID=$DESTINATION_LOCATION_ID"

step "Create cargo"
CARGO_JSON="$(api_request POST "$TRANSPORT_ORDER_SERVICE_URL/v1/cargoes" "$(cat <<EOF
{
  "tenant_id": "$TENANT_ID",
  "cargo_type": "FMCG",
  "description": "Продукты питания",
  "gross_weight": 20000,
  "volume": 82,
  "items": [
    {
      "sku": "TEST-SKU-001",
      "name": "Продукты питания",
      "quantity": 100,
      "unit": "PALLET"
    }
  ]
}
EOF
)")"
CARGO_ID="$(echo "$CARGO_JSON" | jq -r '.id')"
pass "CARGO_ID=$CARGO_ID"

step "Create transport order TO-TEST-${SMOKE_RUN_ID}"
TRANSPORT_ORDER_JSON="$(api_request POST "$TRANSPORT_ORDER_SERVICE_URL/v1/transport-orders" "$(cat <<EOF
{
  "tenant_id": "$TENANT_ID",
  "order_number": "TO-TEST-${SMOKE_RUN_ID}",
  "shipper_company_id": "$SHIPPER_COMPANY_ID",
  "consignee_company_id": "$CONSIGNEE_COMPANY_ID",
  "origin_location_id": "$ORIGIN_LOCATION_ID",
  "destination_location_id": "$DESTINATION_LOCATION_ID",
  "cargo_id": "$CARGO_ID",
  "requested_pickup_date": "2026-07-01",
  "requested_delivery_date": "2026-07-03",
  "transport_mode": "ROAD",
  "equipment_type": "TENT_20T"
}
EOF
)")"
TRANSPORT_ORDER_ID="$(echo "$TRANSPORT_ORDER_JSON" | jq -r '.id')"
assert_json_field "$TRANSPORT_ORDER_JSON" '.status' "DRAFT" "transport order status"
pass "TRANSPORT_ORDER_ID=$TRANSPORT_ORDER_ID"

step "Submit transport order"
TRANSPORT_ORDER_JSON="$(api_request POST "$TRANSPORT_ORDER_SERVICE_URL/v1/transport-orders/$TRANSPORT_ORDER_ID/submit")"
assert_json_field "$TRANSPORT_ORDER_JSON" '.status' "READY_FOR_SOURCING" "transport order status"
pass "Transport order submitted"

step "Create freight request FR-TEST-${SMOKE_RUN_ID}"
FREIGHT_REQUEST_JSON="$(api_request POST "$RFX_SERVICE_URL/v1/freight-requests/from-transport-order" "$(cat <<EOF
{
  "tenant_id": "$TENANT_ID",
  "transport_order_id": "$TRANSPORT_ORDER_ID",
  "freight_request_number": "FR-TEST-${SMOKE_RUN_ID}",
  "request_type": "MINI_TENDER",
  "shipper_company_id": "$SHIPPER_COMPANY_ID",
  "response_deadline": "2026-12-31T18:00:00Z",
  "currency_code": "RUB"
}
EOF
)")"
FREIGHT_REQUEST_ID="$(echo "$FREIGHT_REQUEST_JSON" | jq -r '.id')"
assert_json_field "$FREIGHT_REQUEST_JSON" '.status' "DRAFT" "freight request status"
pass "FREIGHT_REQUEST_ID=$FREIGHT_REQUEST_ID"

step "Publish freight request"
FREIGHT_REQUEST_JSON="$(api_request POST "$RFX_SERVICE_URL/v1/freight-requests/$FREIGHT_REQUEST_ID/publish?tenant_id=$TENANT_ID")"
assert_json_field "$FREIGHT_REQUEST_JSON" '.status' "PUBLISHED" "freight request status"
pass "Freight request published"

step "Create bid BID-TEST-${SMOKE_RUN_ID}"
BID_JSON="$(api_request POST "$RFX_SERVICE_URL/v1/freight-requests/$FREIGHT_REQUEST_ID/bids" "$(cat <<EOF
{
  "tenant_id": "$TENANT_ID",
  "carrier_company_id": "$CARRIER_COMPANY_ID",
  "bid_number": "BID-TEST-${SMOKE_RUN_ID}",
  "currency_code": "RUB",
  "vat_rate": 20,
  "valid_until": "2026-12-31T18:00:00Z",
  "items": [
    {
      "description": "Москва — Казань TEST",
      "base_amount": 100000,
      "fuel_surcharge": 5000,
      "toll_amount": 3000,
      "extra_charges": 0,
      "vat_rate": 20
    }
  ]
}
EOF
)")"
BID_ID="$(echo "$BID_JSON" | jq -r '.id')"
assert_json_field "$BID_JSON" '.status' "DRAFT" "bid status"
pass "BID_ID=$BID_ID"

step "Submit bid"
BID_JSON="$(api_request POST "$RFX_SERVICE_URL/v1/bids/$BID_ID/submit?tenant_id=$TENANT_ID")"
assert_json_field "$BID_JSON" '.status' "SUBMITTED" "bid status"
pass "Bid submitted"

step "Accept bid"
BID_JSON="$(api_request POST "$RFX_SERVICE_URL/v1/bids/$BID_ID/accept?tenant_id=$TENANT_ID")"
assert_json_field "$BID_JSON" '.status' "ACCEPTED" "bid status"
pass "Bid accepted"

step "Create driver"
DRIVER_JSON="$(api_request POST "$SHIPMENT_SERVICE_URL/v1/drivers" "$(cat <<EOF
{
  "tenant_id": "$TENANT_ID",
  "carrier_company_id": "$CARRIER_COMPANY_ID",
  "full_name": "Иван Водитель TEST",
  "license_number": "LIC-${SMOKE_RUN_ID}",
  "license_country": "RU"
}
EOF
)")"
DRIVER_ID="$(echo "$DRIVER_JSON" | jq -r '.id')"
pass "DRIVER_ID=$DRIVER_ID"

step "Create vehicle"
VEHICLE_JSON="$(api_request POST "$SHIPMENT_SERVICE_URL/v1/vehicles" "$(cat <<EOF
{
  "tenant_id": "$TENANT_ID",
  "carrier_company_id": "$CARRIER_COMPANY_ID",
  "plate_number": "SMK-${SMOKE_RUN_ID}",
  "vehicle_type": "TRUCK",
  "equipment_type": "TENT_20T",
  "registration_country": "RU"
}
EOF
)")"
VEHICLE_ID="$(echo "$VEHICLE_JSON" | jq -r '.id')"
pass "VEHICLE_ID=$VEHICLE_ID"

step "Create shipment from accepted bid SH-TEST-${SMOKE_RUN_ID}"
SHIPMENT_JSON="$(api_request POST "$SHIPMENT_SERVICE_URL/v1/shipments/from-bid" "$(cat <<EOF
{
  "tenant_id": "$TENANT_ID",
  "shipment_number": "SH-TEST-${SMOKE_RUN_ID}",
  "bid_id": "$BID_ID",
  "transport_order_id": "$TRANSPORT_ORDER_ID",
  "planned_pickup_at": "2026-07-01T09:00:00Z",
  "planned_delivery_at": "2026-07-03T18:00:00Z"
}
EOF
)")"
SHIPMENT_ID="$(echo "$SHIPMENT_JSON" | jq -r '.id')"
assert_json_field "$SHIPMENT_JSON" '.status' "CARRIER_ASSIGNED" "shipment status"
pass "SHIPMENT_ID=$SHIPMENT_ID"

step "Assign driver to shipment"
SHIPMENT_JSON="$(api_request POST "$SHIPMENT_SERVICE_URL/v1/shipments/$SHIPMENT_ID/assign-driver" "$(cat <<EOF
{
  "tenant_id": "$TENANT_ID",
  "driver_id": "$DRIVER_ID"
}
EOF
)")"
assert_json_field "$SHIPMENT_JSON" '.status' "ACCEPTED_BY_CARRIER" "shipment status after assign-driver"
pass "Driver assigned"

step "Assign vehicle to shipment"
SHIPMENT_JSON="$(api_request POST "$SHIPMENT_SERVICE_URL/v1/shipments/$SHIPMENT_ID/assign-vehicle" "$(cat <<EOF
{
  "tenant_id": "$TENANT_ID",
  "vehicle_id": "$VEHICLE_ID"
}
EOF
)")"
assert_json_field "$SHIPMENT_JSON" '.status' "DRIVER_ASSIGNED" "shipment status after assign-vehicle"
pass "Vehicle assigned"

step "Advance shipment status to READY_FOR_BILLING"
for status in \
  PICKUP_SLOT_BOOKED \
  IN_PICKUP \
  LOADED \
  IN_TRANSIT \
  ARRIVED_AT_CONSIGNEE \
  UNLOADING \
  DELIVERED \
  DELIVERY_CONFIRMED \
  DOCUMENTS_COMPLETED \
  READY_FOR_BILLING
do
  patch_shipment_status "$status"
done

step "Create POD document DOC-TEST-${SMOKE_RUN_ID}"
DOCUMENT_JSON="$(api_request POST "$DOCUMENT_SERVICE_URL/v1/documents" "$(cat <<EOF
{
  "tenant_id": "$TENANT_ID",
  "document_number": "DOC-TEST-${SMOKE_RUN_ID}",
  "document_type": "POD",
  "owner_company_id": "$SHIPPER_COMPANY_ID",
  "related_entity_type": "SHIPMENT",
  "related_entity_id": "$SHIPMENT_ID",
  "legal_language": "ru-RU",
  "payload_json": {
    "shipment_number": "SH-TEST-${SMOKE_RUN_ID}",
    "delivery_confirmed": true
  }
}
EOF
)")"
DOCUMENT_ID="$(echo "$DOCUMENT_JSON" | jq -r '.id')"
assert_json_field "$DOCUMENT_JSON" '.document_status' "DRAFT" "document status"
pass "DOCUMENT_ID=$DOCUMENT_ID"

step "Move document to READY_FOR_SIGNING"
DOCUMENT_JSON="$(api_request POST "$DOCUMENT_SERVICE_URL/v1/documents/$DOCUMENT_ID/ready-for-signing" "$(cat <<EOF
{
  "tenant_id": "$TENANT_ID"
}
EOF
)")"
assert_json_field "$DOCUMENT_JSON" '.document_status' "READY_FOR_SIGNING" "document status"
pass "Document ready for signing"

step "Create signing session"
SIGNING_JSON="$(api_request POST "$DOCUMENT_SERVICE_URL/v1/documents/$DOCUMENT_ID/signing-sessions" "$(cat <<EOF
{
  "tenant_id": "$TENANT_ID",
  "required_signers_count": 1,
  "expires_at": "2026-12-31T18:00:00Z"
}
EOF
)")"
SIGNING_SESSION_ID="$(echo "$SIGNING_JSON" | jq -r '.id')"
pass "SIGNING_SESSION_ID=$SIGNING_SESSION_ID"

step "Add mock signature"
SIGNATURE_JSON="$(api_request POST "$DOCUMENT_SERVICE_URL/v1/signing-sessions/$SIGNING_SESSION_ID/signatures" "$(cat <<EOF
{
  "tenant_id": "$TENANT_ID",
  "signer_user_id": "$USER_ID",
  "signer_company_id": "$SHIPPER_COMPANY_ID",
  "signature_type": "SIMPLE_ELECTRONIC"
}
EOF
)")"
assert_json_field "$SIGNATURE_JSON" '.document.document_status' "SIGNED" "document status after signature"
pass "Document signed"

step "Create billing register BR-TEST-${SMOKE_RUN_ID}"
REGISTER_JSON="$(api_request POST "$BILLING_REGISTER_SERVICE_URL/v1/billing-registers" "$(cat <<EOF
{
  "tenant_id": "$TENANT_ID",
  "register_number": "BR-TEST-${SMOKE_RUN_ID}",
  "customer_company_id": "$SHIPPER_COMPANY_ID",
  "contractor_company_id": "$CARRIER_COMPANY_ID",
  "period_from": "2026-07-01",
  "period_to": "2026-07-15",
  "currency_code": "RUB",
  "vat_rate": 20
}
EOF
)")"
BILLING_REGISTER_ID="$(echo "$REGISTER_JSON" | jq -r '.id')"
assert_json_field "$REGISTER_JSON" '.status' "DRAFT" "billing register status"
pass "BILLING_REGISTER_ID=$BILLING_REGISTER_ID"

step "Add shipment to billing register"
ITEM_JSON="$(api_request POST "$BILLING_REGISTER_SERVICE_URL/v1/billing-registers/$BILLING_REGISTER_ID/items" "$(cat <<EOF
{
  "tenant_id": "$TENANT_ID",
  "shipment_id": "$SHIPMENT_ID",
  "route_description": "Москва — Казань TEST",
  "pickup_date": "2026-07-01",
  "delivery_date": "2026-07-03",
  "base_amount": 100000,
  "extra_charges": 5000,
  "penalties": 0,
  "vat_rate": 20
}
EOF
)")"
assert_json_number "$ITEM_JSON" '.amount_without_vat' 105000 "item amount_without_vat"
assert_json_number "$ITEM_JSON" '.vat_amount' 21000 "item vat_amount"
assert_json_number "$ITEM_JSON" '.amount_with_vat' 126000 "item amount_with_vat"
pass "Billing register item added"

step "Calculate billing register"
REGISTER_JSON="$(api_request POST "$BILLING_REGISTER_SERVICE_URL/v1/billing-registers/$BILLING_REGISTER_ID/calculate" "$(cat <<EOF
{
  "tenant_id": "$TENANT_ID"
}
EOF
)")"
assert_json_field "$REGISTER_JSON" '.status' "CALCULATED" "billing register status"
assert_json_number "$REGISTER_JSON" '.total_without_vat' 105000 "total_without_vat"
assert_json_number "$REGISTER_JSON" '.vat_amount' 21000 "vat_amount"
assert_json_number "$REGISTER_JSON" '.total_with_vat' 126000 "total_with_vat"
pass "Billing register calculated"

step "Approve billing register"
REGISTER_JSON="$(api_request POST "$BILLING_REGISTER_SERVICE_URL/v1/billing-registers/$BILLING_REGISTER_ID/approve" "$(cat <<EOF
{
  "tenant_id": "$TENANT_ID",
  "approved_by": "$USER_ID"
}
EOF
)")"
assert_json_field "$REGISTER_JSON" '.status' "APPROVED" "billing register status"
pass "Billing register approved"

step "Create UPD UPD-TEST-${SMOKE_RUN_ID}"
UPD_JSON="$(api_request POST "$BILLING_REGISTER_SERVICE_URL/v1/billing-registers/$BILLING_REGISTER_ID/upd" "$(jq -nc \
  --arg tenant_id "$TENANT_ID" \
  --arg upd_number "UPD-TEST-${SMOKE_RUN_ID}" \
  --arg seller "$CARRIER_COMPANY_ID" \
  --arg buyer "$SHIPPER_COMPANY_ID" \
  '{
    tenant_id: $tenant_id,
    upd_number: $upd_number,
    upd_date: "2026-07-16",
    seller_company_id: $seller,
    buyer_company_id: $buyer,
    function_code: "\u0421\u0427\u0424\u0414\u041e\u041f"
  }')")"
pass "UPD created: $(echo "$UPD_JSON" | jq -r '.id')"

REGISTER_JSON="$(api_request GET "$BILLING_REGISTER_SERVICE_URL/v1/billing-registers/$BILLING_REGISTER_ID")"
assert_json_field "$REGISTER_JSON" '.status' "CLOSING_DOCUMENTS_CREATED" "billing register status after UPD"
pass "Register moved to CLOSING_DOCUMENTS_CREATED"

step "Mark billing register as sent to EDO (mock)"
REGISTER_JSON="$(api_request POST "$BILLING_REGISTER_SERVICE_URL/v1/billing-registers/$BILLING_REGISTER_ID/mark-sent-to-edo" "$(cat <<EOF
{
  "tenant_id": "$TENANT_ID"
}
EOF
)")"
assert_json_field "$REGISTER_JSON" '.status' "SENT_TO_EDO" "billing register status"
pass "Register marked SENT_TO_EDO"

step "Mark billing register as signed by counterparty (mock)"
REGISTER_JSON="$(api_request POST "$BILLING_REGISTER_SERVICE_URL/v1/billing-registers/$BILLING_REGISTER_ID/mark-signed" "$(cat <<EOF
{
  "tenant_id": "$TENANT_ID"
}
EOF
)")"
assert_json_field "$REGISTER_JSON" '.status' "SIGNED_BY_COUNTERPARTY" "billing register status"
pass "Register marked SIGNED_BY_COUNTERPARTY"

step "Mark billing register as paid"
REGISTER_JSON="$(api_request POST "$BILLING_REGISTER_SERVICE_URL/v1/billing-registers/$BILLING_REGISTER_ID/mark-paid" "$(cat <<EOF
{
  "tenant_id": "$TENANT_ID"
}
EOF
)")"
assert_json_field "$REGISTER_JSON" '.status' "PAID" "billing register status"
pass "Register marked PAID"

step "Close billing register"
REGISTER_JSON="$(api_request POST "$BILLING_REGISTER_SERVICE_URL/v1/billing-registers/$BILLING_REGISTER_ID/close" "$(cat <<EOF
{
  "tenant_id": "$TENANT_ID"
}
EOF
)")"
assert_json_field "$REGISTER_JSON" '.status' "CLOSED" "billing register status"
pass "Register closed"

echo ""
echo -e "${GREEN}SMOKE TEST PASSED${NC}"
echo ""
echo "TENANT_ID=$TENANT_ID"
echo "SHIPPER_COMPANY_ID=$SHIPPER_COMPANY_ID"
echo "CONSIGNEE_COMPANY_ID=$CONSIGNEE_COMPANY_ID"
echo "CARRIER_COMPANY_ID=$CARRIER_COMPANY_ID"
echo "USER_ID=$USER_ID"
echo "TRANSPORT_ORDER_ID=$TRANSPORT_ORDER_ID"
echo "FREIGHT_REQUEST_ID=$FREIGHT_REQUEST_ID"
echo "BID_ID=$BID_ID"
echo "SHIPMENT_ID=$SHIPMENT_ID"
echo "DOCUMENT_ID=$DOCUMENT_ID"
echo "SIGNING_SESSION_ID=$SIGNING_SESSION_ID"
echo "BILLING_REGISTER_ID=$BILLING_REGISTER_ID"
echo ""
