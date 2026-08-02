#!/usr/bin/env bash
# Creates an isolated post-rollout tenant with five shipments for shadow acceptance.
# Does not print or persist tenant UUIDs to git-tracked files.
set -euo pipefail

API_GATEWAY_URL="${API_GATEWAY_URL:-http://127.0.0.1:8080}"
IDENTITY_SERVICE_URL="${IDENTITY_SERVICE_URL:-http://127.0.0.1:8081}"
COMPANY_SERVICE_URL="${COMPANY_SERVICE_URL:-http://127.0.0.1:8082}"
TRANSPORT_ORDER_SERVICE_URL="${TRANSPORT_ORDER_SERVICE_URL:-http://127.0.0.1:8083}"
RFX_SERVICE_URL="${RFX_SERVICE_URL:-http://127.0.0.1:8084}"
SHIPMENT_SERVICE_URL="${SHIPMENT_SERVICE_URL:-http://127.0.0.1:8085}"
LIST_LIMIT="${LIST_LIMIT:-100}"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

fail() { echo "acceptance-fixture: $*" >&2; exit 1; }
step() { echo "==> $*" >&2; }

require_cmd() { command -v "$1" >/dev/null 2>&1 || fail "$1 is required"; }

new_tenant_id() {
  if [[ -n "${ACCEPTANCE_TENANT_ID:-}" ]]; then
    echo "${ACCEPTANCE_TENANT_ID}"
    return 0
  fi
  if command -v uuidgen >/dev/null 2>&1; then
    uuidgen | tr '[:upper:]' '[:lower:]'
    return 0
  fi
  python - <<'PY'
import uuid
print(uuid.uuid4())
PY
}

parse_http() {
  HTTP_CODE="${1##*__HTTP_CODE__:}"
  HTTP_BODY="${1%__HTTP_CODE__:*}"
}

curl_json() {
  local method="$1" url="$2" data="${3:-}" tenant_header="${4:-}" user_header="${5:-}"
  local args=(-sS -w "__HTTP_CODE__:%{http_code}" -X "$method" "$url" -H "Content-Type: application/json; charset=utf-8")
  [[ -n "$tenant_header" ]] && args+=(-H "$tenant_header")
  [[ -n "$user_header" ]] && args+=(-H "$user_header")
  if [[ -n "$data" ]]; then
    printf '%s' "$data" | curl "${args[@]}" --data-binary @-
  else
    curl "${args[@]}" -H "Accept: application/json"
  fi
}

api_request() {
  local method="$1" url="$2" data="${3:-}" tenant_header="${4:-}" user_header="${5:-}" raw
  raw="$(curl_json "$method" "$url" "$data" "$tenant_header" "$user_header")"
  parse_http "$raw"
  [[ "$HTTP_CODE" -ge 200 && "$HTTP_CODE" -lt 300 ]]
}

shipment_headers() {
  SHIPMENT_TENANT_HDR="X-Tenant-ID: ${TENANT_ID}"
  SHIPMENT_USER_HDR="X-User-ID: ${USER_ID}"
}

api_get() {
  curl -fsS "$1"
}

ensure_company() {
  local legal_name="$1" company_type="$2" short_name="$3"
  local raw existing body
  raw="$(api_get "${COMPANY_SERVICE_URL}/v1/companies?tenant_id=${TENANT_ID}&limit=${LIST_LIMIT}&offset=0" 2>/dev/null || true)"
  existing="$(echo "$raw" | jq -r --arg n "$legal_name" '(.items // [])[] | select(.legal_name == $n) | .id' | head -n1)"
  if [[ -n "$existing" && "$existing" != "null" ]]; then
    echo "$existing"
    return 0
  fi
  body="$(jq -n --arg tenant_id "$TENANT_ID" --arg legal_name "$legal_name" \
    --arg short_name "$short_name" --arg company_type "$company_type" \
    '{tenant_id:$tenant_id,legal_name:$legal_name,short_name:$short_name,company_type:$company_type,country_code:"RU",preferred_locale:"ru-RU"}')"
  api_request POST "${COMPANY_SERVICE_URL}/v1/companies" "$body"
  echo "$HTTP_BODY" | jq -r '.id'
}

ensure_location() {
  local company_id="$1" name="$2" city="$3"
  local body
  body="$(jq -n --arg tenant_id "$TENANT_ID" --arg company_id "$company_id" \
    --arg name "$name" --arg city "$city" \
    '{tenant_id:$tenant_id,company_id:$company_id,location_type:"WAREHOUSE",name:$name,country_code:"RU",city:$city,timezone:"Europe/Moscow"}')"
  api_request POST "${TRANSPORT_ORDER_SERVICE_URL}/v1/locations" "$body"
  echo "$HTTP_BODY" | jq -r '.id'
}

ensure_cargo() {
  local body
  body="$(jq -n --arg tenant_id "$TENANT_ID" \
    '{tenant_id:$tenant_id,cargo_type:"FMCG",description:"ACC cargo",gross_weight:18000,volume:76,items:[{sku:"ACC-001",name:"item",quantity:1,unit:"PALLET"}]}')"
  api_request POST "${TRANSPORT_ORDER_SERVICE_URL}/v1/cargoes" "$body"
  echo "$HTTP_BODY" | jq -r '.id'
}

ensure_transport_order() {
  local order_number="$1" shipper_id="$2" consignee_id="$3"
  local origin_id dest_id cargo_id body id
  origin_id="$(ensure_location "$shipper_id" "ACC origin ${order_number}" "CityA")"
  dest_id="$(ensure_location "$consignee_id" "ACC dest ${order_number}" "CityB")"
  cargo_id="$(ensure_cargo)"
  body="$(jq -n --arg tenant_id "$TENANT_ID" --arg order_number "$order_number" \
    --arg shipper_id "$shipper_id" --arg consignee_id "$consignee_id" \
    --arg origin_id "$origin_id" --arg dest_id "$dest_id" --arg cargo_id "$cargo_id" \
    '{tenant_id:$tenant_id,order_number:$order_number,shipper_company_id:$shipper_id,consignee_company_id:$consignee_id,origin_location_id:$origin_id,destination_location_id:$dest_id,cargo_id:$cargo_id,requested_pickup_date:"2026-08-01",requested_delivery_date:"2026-08-03",transport_mode:"ROAD",equipment_type:"TENT_20T"}')"
  api_request POST "${TRANSPORT_ORDER_SERVICE_URL}/v1/transport-orders" "$body"
  id="$(echo "$HTTP_BODY" | jq -r '.id')"
  api_request POST "${TRANSPORT_ORDER_SERVICE_URL}/v1/transport-orders/${id}/submit" "" || true
  echo "$id"
}

ensure_freight_request() {
  local fr_number="$1" to_id="$2" shipper_id="$3"
  local body id status
  body="$(jq -n --arg tenant_id "$TENANT_ID" --arg transport_order_id "$to_id" \
    --arg fr_number "$fr_number" --arg shipper_id "$shipper_id" \
    '{tenant_id:$tenant_id,transport_order_id:$transport_order_id,freight_request_number:$fr_number,request_type:"MINI_TENDER",shipper_company_id:$shipper_id,response_deadline:"2026-12-31T18:00:00Z",currency_code:"RUB"}')"
  api_request POST "${RFX_SERVICE_URL}/v1/freight-requests/from-transport-order" "$body"
  id="$(echo "$HTTP_BODY" | jq -r '.id')"
  status="$(echo "$HTTP_BODY" | jq -r '.status // empty')"
  if [[ "$status" == "DRAFT" ]]; then
    api_request POST "${RFX_SERVICE_URL}/v1/freight-requests/${id}/publish?tenant_id=${TENANT_ID}" "" || true
  fi
  echo "$id"
}

ensure_bid() {
  local fr_id="$1" carrier_id="$2" bid_number="$3"
  local body id
  body="$(jq -n --arg tenant_id "$TENANT_ID" --arg carrier_id "$carrier_id" --arg bid_number "$bid_number" \
    '{tenant_id:$tenant_id,carrier_company_id:$carrier_id,bid_number:$bid_number,currency_code:"RUB",vat_rate:20,valid_until:"2026-12-31T18:00:00Z",items:[{description:"route",base_amount:90000,fuel_surcharge:0,toll_amount:0,extra_charges:0,vat_rate:20}]}')"
  api_request POST "${RFX_SERVICE_URL}/v1/freight-requests/${fr_id}/bids" "$body"
  id="$(echo "$HTTP_BODY" | jq -r '.id')"
  api_request POST "${RFX_SERVICE_URL}/v1/bids/${id}/submit?tenant_id=${TENANT_ID}" "" || true
  api_request POST "${RFX_SERVICE_URL}/v1/bids/${id}/accept?tenant_id=${TENANT_ID}" "" || true
  echo "$id"
}

create_shipment() {
  local shipment_number="$1" bid_id="$2" to_id="$3"
  local body
  body="$(jq -n --arg shipment_number "$shipment_number" \
    --arg bid_id "$bid_id" --arg transport_order_id "$to_id" \
    '{shipment_number:$shipment_number,bid_id:$bid_id,transport_order_id:$transport_order_id,planned_pickup_at:"2026-08-01T09:00:00Z",planned_delivery_at:"2026-08-03T18:00:00Z"}')"
  api_request POST "${SHIPMENT_SERVICE_URL}/v1/shipments/from-bid" "$body" "${SHIPMENT_TENANT_HDR}" "${SHIPMENT_USER_HDR}" \
    || fail "create shipment ${shipment_number} failed HTTP ${HTTP_CODE}"
  echo "$HTTP_BODY" | jq -r '.id'
}

patch_status() {
  local shipment_id="$1" status="$2"
  local body
  if [[ "$status" == "LOADED" ]]; then
    body="$(jq -n --arg status "$status" \
      '{status:$status,actual_time:"2026-08-02T12:00:00Z"}')"
  else
    body="$(jq -n --arg status "$status" '{status:$status}')"
  fi
  api_request PATCH "${SHIPMENT_SERVICE_URL}/v1/shipments/${shipment_id}/status" "$body" "${SHIPMENT_TENANT_HDR}" "${SHIPMENT_USER_HDR}" \
    || fail "patch status ${status} for ${shipment_id} failed HTTP ${HTTP_CODE}"
}

advance_to_in_transit() {
  local shipment_id="$1"
  local seq=(ACCEPTED_BY_CARRIER DRIVER_ASSIGNED PICKUP_SLOT_BOOKED IN_PICKUP LOADED IN_TRANSIT)
  local s
  for s in "${seq[@]}"; do
    patch_status "$shipment_id" "$s"
  done
}

cancel_shipment() {
  local shipment_id="$1"
  local body
  body='{"reason":"acceptance cancel"}'
  api_request POST "${SHIPMENT_SERVICE_URL}/v1/shipments/${shipment_id}/cancel" "$body" "${SHIPMENT_TENANT_HDR}" "${SHIPMENT_USER_HDR}" \
    || fail "cancel shipment ${shipment_id} failed HTTP ${HTTP_CODE}"
}

main() {
  require_cmd curl
  require_cmd jq

  TENANT_ID="$(new_tenant_id)"
  TENANT_CODE="${TENANT_CODE:-ct-acc-${TENANT_ID:0:8}}"
  ADMIN_EMAIL="${ADMIN_EMAIL:-acc-admin@${TENANT_CODE}.local}"
  ADMIN_PASSWORD="${ADMIN_PASSWORD:-Acceptance123456!}"

  step "Seed acceptance tenant admin (tenant id kept in process env only)"
  TENANT_ID="$TENANT_ID" TENANT_CODE="$TENANT_CODE" TENANT_NAME="Control Tower Acceptance" \
    ADMIN_EMAIL="$ADMIN_EMAIL" ADMIN_PASSWORD="$ADMIN_PASSWORD" \
    API_GATEWAY_URL="$API_GATEWAY_URL" IDENTITY_SERVICE_URL="$IDENTITY_SERVICE_URL" \
    COMPANY_SERVICE_URL="$COMPANY_SERVICE_URL" \
    "${ROOT}/scripts/dev/seed_dev_admin.sh" >/dev/null

  USER_ID="$(api_get "${IDENTITY_SERVICE_URL}/v1/users?tenant_id=${TENANT_ID}&limit=${LIST_LIMIT}&offset=0" \
    | jq -r --arg email "$ADMIN_EMAIL" '(.items // [])[] | select(.email == $email) | .id' | head -n1)"
  [[ -n "${USER_ID}" && "${USER_ID}" != "null" ]] || fail "acceptance admin user not found"
  shipment_headers

  step "Create companies and five acceptance shipments"
  local shipper carrier consignee i to fr bid sh
  shipper="$(ensure_company "ACC Shipper" "SHIPPER" "ACC Shipper")"
  carrier="$(ensure_company "ACC Carrier" "CARRIER" "ACC Carrier")"
  consignee="$(ensure_company "ACC Consignee" "CONSIGNEE" "ACC Consignee")"

  local ids=()
  for i in 1 2 3 4 5; do
    to="$(ensure_transport_order "ACC-TO-00${i}" "$shipper" "$consignee")"
    fr="$(ensure_freight_request "ACC-FR-00${i}" "$to" "$shipper")"
    bid="$(ensure_bid "$fr" "$carrier" "ACC-BID-00${i}")"
    sh="$(create_shipment "ACC-SH-00${i}" "$bid" "$to")"
    ids+=("$sh")
  done

  advance_to_in_transit "${ids[2]}"
  advance_to_in_transit "${ids[3]}"
  cancel_shipment "${ids[4]}"

  # Export for parent acceptance script (stdout is machine-readable env exports).
  printf 'export TENANT_ID=%q\n' "$TENANT_ID"
  printf 'export ADMIN_EMAIL=%q\n' "$ADMIN_EMAIL"
  printf 'export ADMIN_PASSWORD=%q\n' "$ADMIN_PASSWORD"
  echo "acceptance-fixture: OK (5 shipments: CARRIER_ASSIGNED×2 IN_TRANSIT×2 CANCELLED×1)" >&2
}

main "$@"
