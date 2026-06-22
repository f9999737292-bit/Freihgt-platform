#!/usr/bin/env bash
set -euo pipefail

API_GATEWAY_URL="${API_GATEWAY_URL:-http://localhost:8080}"
IDENTITY_SERVICE_URL="${IDENTITY_SERVICE_URL:-http://localhost:8081}"
COMPANY_SERVICE_URL="${COMPANY_SERVICE_URL:-http://localhost:8082}"
TRANSPORT_ORDER_SERVICE_URL="${TRANSPORT_ORDER_SERVICE_URL:-http://localhost:8083}"
RFX_SERVICE_URL="${RFX_SERVICE_URL:-http://localhost:8084}"
SHIPMENT_SERVICE_URL="${SHIPMENT_SERVICE_URL:-http://localhost:8085}"
DOCUMENT_SERVICE_URL="${DOCUMENT_SERVICE_URL:-http://localhost:8086}"
BILLING_REGISTER_SERVICE_URL="${BILLING_REGISTER_SERVICE_URL:-http://localhost:8087}"

TENANT_ID="${TENANT_ID:-74519f22-ff9b-4a8b-8fff-a958c689682f}"
DEMO_PASSWORD="${DEMO_PASSWORD:-Demo123456!}"
LIST_LIMIT="${LIST_LIMIT:-100}"

step() { echo "==> $1" >&2; }
pass() { echo "OK: $1" >&2; }
warn() { echo "WARN: $1" >&2; }
skip() { echo "SKIP: $1" >&2; }

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "ERROR: $1 is required but not installed" >&2
    exit 1
  fi
}

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

curl_json() {
  local method="$1"
  local url="$2"
  local data="${3:-}"

  if [[ -n "$data" ]]; then
    printf '%s' "$data" | curl -sS -w "__HTTP_CODE__:%{http_code}" -X "$method" "$url" \
      -H "Content-Type: application/json; charset=utf-8" \
      --data-binary @-
  else
    curl -sS -w "__HTTP_CODE__:%{http_code}" -X "$method" "$url" \
      -H "Accept: application/json"
  fi
}

api_request_optional() {
  local method="$1"
  local url="$2"
  local data="${3:-}"
  local raw

  raw="$(curl_json "$method" "$url" "$data" 2>/dev/null || true)"
  if [[ -z "$raw" ]]; then
    HTTP_CODE="0"
    HTTP_BODY=""
    return 1
  fi

  parse_http "$raw"
  [[ "$HTTP_CODE" -ge 200 && "$HTTP_CODE" -lt 300 ]]
}

api_get_json() {
  local url="$1"
  local raw

  raw="$(curl -sS "$url" 2>/dev/null || true)"
  if [[ -z "$raw" ]]; then
    echo ""
    return 1
  fi
  if echo "$raw" | jq -e '.error' >/dev/null 2>&1; then
    echo ""
    return 1
  fi
  echo "$raw"
}

check_gateway_health() {
  local raw http_code
  raw="$(curl -sS -w "__HTTP_CODE__:%{http_code}" "$API_GATEWAY_URL/health" 2>/dev/null || true)"
  if [[ -z "$raw" ]]; then
    return 1
  fi
  parse_http "$raw"
  [[ "$HTTP_CODE" -eq 200 ]]
}

find_company_id() {
  local legal_name="$1"
  local raw
  raw="$(api_get_json "$COMPANY_SERVICE_URL/v1/companies?tenant_id=${TENANT_ID}&limit=${LIST_LIMIT}&offset=0")"
  if [[ -z "$raw" ]]; then
    echo ""
    return 0
  fi
  echo "$raw" | jq -r --arg name "$legal_name" '
    (.items // [])[] | select(.legal_name == $name) | .id' | head -n 1
}

ensure_company() {
  local legal_name="$1"
  local company_type="$2"
  local short_name="$3"
  local existing_id body

  existing_id="$(find_company_id "$legal_name")"
  if [[ -n "$existing_id" && "$existing_id" != "null" ]]; then
    skip "company exists: $legal_name ($existing_id)"
    echo "$existing_id"
    return 0
  fi

  body="$(jq -n \
    --arg tenant_id "$TENANT_ID" \
    --arg legal_name "$legal_name" \
    --arg short_name "$short_name" \
    --arg company_type "$company_type" \
    '{
      tenant_id: $tenant_id,
      legal_name: $legal_name,
      short_name: $short_name,
      company_type: $company_type,
      country_code: "RU",
      preferred_locale: "ru-RU"
    }')"

  if api_request_optional POST "$COMPANY_SERVICE_URL/v1/companies" "$body"; then
    pass "company created: $legal_name"
    echo "$HTTP_BODY" | jq -r '.id'
    return 0
  fi

  if [[ "$HTTP_CODE" -eq 409 ]]; then
    find_company_id "$legal_name"
    return 0
  fi

  warn "could not create company $legal_name (HTTP $HTTP_CODE)"
  echo ""
}

find_user_id() {
  local email="$1"
  local raw
  raw="$(api_get_json "$IDENTITY_SERVICE_URL/v1/users?tenant_id=${TENANT_ID}&limit=${LIST_LIMIT}&offset=0")"
  if [[ -z "$raw" ]]; then
    echo ""
    return 0
  fi
  echo "$raw" | jq -r --arg email "$email" '
    (.items // [])[] | select(.email == $email) | .id' | head -n 1
}

ensure_user() {
  local email="$1"
  local full_name="$2"
  local existing_id body

  existing_id="$(find_user_id "$email")"
  if [[ -n "$existing_id" && "$existing_id" != "null" ]]; then
    skip "user exists: $email ($existing_id)"
    echo "$existing_id"
    return 0
  fi

  body="$(jq -n \
    --arg tenant_id "$TENANT_ID" \
    --arg email "$email" \
    --arg password "$DEMO_PASSWORD" \
    --arg full_name "$full_name" \
    '{
      tenant_id: $tenant_id,
      email: $email,
      password: $password,
      full_name: $full_name,
      preferred_locale: "ru-RU"
    }')"

  if api_request_optional POST "$IDENTITY_SERVICE_URL/v1/users" "$body"; then
    pass "user created: $email"
    echo "$HTTP_BODY" | jq -r '.id'
    return 0
  fi

  if [[ "$HTTP_CODE" -eq 409 || "$HTTP_CODE" -eq 422 ]]; then
    find_user_id "$email"
    return 0
  fi

  warn "could not create user $email (HTTP $HTTP_CODE)"
  echo ""
}

ensure_membership() {
  local company_id="$1"
  local user_id="$2"
  local position="$3"
  local members_raw body

  if [[ -z "$user_id" || "$user_id" == "null" ]]; then
    warn "skip membership: empty user_id for company $company_id"
    return 0
  fi

  members_raw="$(api_get_json "$COMPANY_SERVICE_URL/v1/companies/${company_id}/members?tenant_id=${TENANT_ID}&limit=${LIST_LIMIT}&offset=0")"
  if [[ -n "$members_raw" ]] && echo "$members_raw" | jq -e --arg uid "$user_id" '(.items // [])[] | select(.user_id == $uid)' >/dev/null 2>&1; then
    skip "membership exists: user $user_id in company $company_id"
    return 0
  fi

  body="$(jq -n --arg tenant_id "$TENANT_ID" --arg user_id "$user_id" --arg position "$position" '{
    tenant_id: $tenant_id,
    user_id: $user_id,
    position: $position
  }')"

  if api_request_optional POST "$COMPANY_SERVICE_URL/v1/companies/${company_id}/members" "$body"; then
    pass "membership created for user $user_id"
    return 0
  fi

  if [[ "$HTTP_CODE" -eq 409 ]]; then
    skip "membership already exists for user $user_id"
    return 0
  fi

  warn "could not create membership (HTTP $HTTP_CODE)"
}

ensure_role() {
  local company_id="$1"
  local user_id="$2"
  local role_code="$3"
  local roles_raw role_id body

  if [[ -z "$user_id" || "$user_id" == "null" ]]; then
    warn "skip role $role_code: empty user_id"
    return 0
  fi

  roles_raw="$(api_get_json "$IDENTITY_SERVICE_URL/v1/users/${user_id}/roles?tenant_id=${TENANT_ID}")"
  if [[ -n "$roles_raw" ]] && echo "$roles_raw" | jq -e --arg company_id "$company_id" --arg role "$role_code" '
    (.items // [])[] | select(.code == $role and (.company_id // "") == $company_id)
  ' >/dev/null 2>&1; then
    skip "role $role_code already assigned"
    return 0
  fi

  roles_raw="$(api_get_json "$IDENTITY_SERVICE_URL/v1/roles?tenant_id=${TENANT_ID}&limit=${LIST_LIMIT}&offset=0")"
  role_id="$(echo "$roles_raw" | jq -r --arg code "$role_code" '(.items // [])[] | select(.code == $code) | .id' | head -n 1)"
  if [[ -z "$role_id" || "$role_id" == "null" ]]; then
    warn "role $role_code not found"
    return 0
  fi

  body="$(jq -n --arg tenant_id "$TENANT_ID" --arg role_id "$role_id" '{
    tenant_id: $tenant_id,
    role_id: $role_id
  }')"

  if api_request_optional POST "$IDENTITY_SERVICE_URL/v1/users/${user_id}/companies/${company_id}/roles" "$body"; then
    pass "role $role_code assigned"
    return 0
  fi

  if [[ "$HTTP_CODE" -eq 409 ]]; then
    skip "role $role_code already assigned"
    return 0
  fi

  warn "could not assign role $role_code (HTTP $HTTP_CODE)"
}

find_location_id() {
  local name="$1"
  local raw
  raw="$(api_get_json "$TRANSPORT_ORDER_SERVICE_URL/v1/locations?tenant_id=${TENANT_ID}&limit=${LIST_LIMIT}&offset=0&search=$(printf '%s' "$name" | jq -sRr @uri)")"
  if [[ -z "$raw" ]]; then
    echo ""
    return 0
  fi
  echo "$raw" | jq -r --arg name "$name" '
    (.items // [])[] | select(.name == $name) | .id' | head -n 1
}

ensure_location() {
  local company_id="$1"
  local name="$2"
  local city="$3"
  local existing_id body

  existing_id="$(find_location_id "$name")"
  if [[ -n "$existing_id" && "$existing_id" != "null" ]]; then
    echo "$existing_id"
    return 0
  fi

  body="$(jq -n \
    --arg tenant_id "$TENANT_ID" \
    --arg company_id "$company_id" \
    --arg name "$name" \
    --arg city "$city" \
    '{
      tenant_id: $tenant_id,
      company_id: $company_id,
      location_type: "WAREHOUSE",
      name: $name,
      country_code: "RU",
      city: $city,
      timezone: "Europe/Moscow"
    }')"

  if api_request_optional POST "$TRANSPORT_ORDER_SERVICE_URL/v1/locations" "$body"; then
    echo "$HTTP_BODY" | jq -r '.id'
    return 0
  fi

  find_location_id "$name"
}

find_cargo_id() {
  echo ""
}

ensure_demo_cargo() {
  local description="DEMO FMCG груз"
  local existing_id body

  existing_id="$(find_cargo_id "$description")"
  if [[ -n "$existing_id" && "$existing_id" != "null" ]]; then
    echo "$existing_id"
    return 0
  fi

  body="$(jq -n \
    --arg tenant_id "$TENANT_ID" \
    --arg description "$description" \
    '{
      tenant_id: $tenant_id,
      cargo_type: "FMCG",
      description: $description,
      gross_weight: 18000,
      volume: 76,
      items: [
        {
          sku: "DEMO-SKU-001",
          name: "Демо товар",
          quantity: 80,
          unit: "PALLET"
        }
      ]
    }')"

  if api_request_optional POST "$TRANSPORT_ORDER_SERVICE_URL/v1/cargoes" "$body"; then
    echo "$HTTP_BODY" | jq -r '.id'
    return 0
  fi

  find_cargo_id "$description"
}

find_transport_order_id() {
  local order_number="$1"
  local raw
  raw="$(api_get_json "$TRANSPORT_ORDER_SERVICE_URL/v1/transport-orders?tenant_id=${TENANT_ID}&limit=${LIST_LIMIT}&offset=0")"
  if [[ -z "$raw" ]]; then
    echo ""
    return 0
  fi
  echo "$raw" | jq -r --arg num "$order_number" '
    (.items // [])[] | select(.order_number == $num) | .id' | head -n 1
}

get_transport_order_status() {
  local order_id="$1"
  local raw
  raw="$(api_get_json "$TRANSPORT_ORDER_SERVICE_URL/v1/transport-orders/${order_id}?tenant_id=${TENANT_ID}")"
  echo "$raw" | jq -r '.status // empty'
}

ensure_transport_order() {
  local order_number="$1"
  local shipper_id="$2"
  local consignee_id="$3"
  local origin_name="$4"
  local origin_city="$5"
  local dest_name="$6"
  local dest_city="$7"
  local pickup_date="$8"
  local delivery_date="$9"
  local existing_id origin_id dest_id cargo_id body status

  existing_id="$(find_transport_order_id "$order_number")"
  if [[ -n "$existing_id" && "$existing_id" != "null" ]]; then
    status="$(get_transport_order_status "$existing_id")"
    if [[ "$status" == "DRAFT" ]]; then
      api_request_optional POST "$TRANSPORT_ORDER_SERVICE_URL/v1/transport-orders/${existing_id}/submit" "" || true
    fi
    skip "transport order exists: $order_number ($existing_id)"
    echo "$existing_id"
    return 0
  fi

  origin_id="$(ensure_location "$shipper_id" "$origin_name" "$origin_city")"
  dest_id="$(ensure_location "$consignee_id" "$dest_name" "$dest_city")"
  cargo_id="$(ensure_demo_cargo)"

  body="$(jq -n \
    --arg tenant_id "$TENANT_ID" \
    --arg order_number "$order_number" \
    --arg shipper_id "$shipper_id" \
    --arg consignee_id "$consignee_id" \
    --arg origin_id "$origin_id" \
    --arg dest_id "$dest_id" \
    --arg cargo_id "$cargo_id" \
    --arg pickup_date "$pickup_date" \
    --arg delivery_date "$delivery_date" \
    '{
      tenant_id: $tenant_id,
      order_number: $order_number,
      shipper_company_id: $shipper_id,
      consignee_company_id: $consignee_id,
      origin_location_id: $origin_id,
      destination_location_id: $dest_id,
      cargo_id: $cargo_id,
      requested_pickup_date: $pickup_date,
      requested_delivery_date: $delivery_date,
      transport_mode: "ROAD",
      equipment_type: "TENT_20T"
    }')"

  if ! api_request_optional POST "$TRANSPORT_ORDER_SERVICE_URL/v1/transport-orders" "$body"; then
    if [[ "$HTTP_CODE" -eq 409 ]]; then
      find_transport_order_id "$order_number"
      return 0
    fi
    warn "could not create transport order $order_number (HTTP $HTTP_CODE)"
    echo ""
    return 0
  fi

  existing_id="$(echo "$HTTP_BODY" | jq -r '.id')"
  api_request_optional POST "$TRANSPORT_ORDER_SERVICE_URL/v1/transport-orders/${existing_id}/submit" "" || true
  pass "transport order created: $order_number"
  echo "$existing_id"
}

find_freight_request_id() {
  local fr_number="$1"
  local raw
  raw="$(api_get_json "$RFX_SERVICE_URL/v1/freight-requests?tenant_id=${TENANT_ID}&limit=${LIST_LIMIT}&offset=0")"
  if [[ -z "$raw" ]]; then
    echo ""
    return 0
  fi
  echo "$raw" | jq -r --arg num "$fr_number" '
    (.items // [])[] | select(.freight_request_number == $num) | .id' | head -n 1
}

ensure_freight_request() {
  local fr_number="$1"
  local transport_order_id="$2"
  local shipper_id="$3"
  local existing_id body status

  existing_id="$(find_freight_request_id "$fr_number")"
  if [[ -n "$existing_id" && "$existing_id" != "null" ]]; then
    skip "freight request exists: $fr_number ($existing_id)"
    echo "$existing_id"
    return 0
  fi

  body="$(jq -n \
    --arg tenant_id "$TENANT_ID" \
    --arg transport_order_id "$transport_order_id" \
    --arg fr_number "$fr_number" \
    --arg shipper_id "$shipper_id" \
    '{
      tenant_id: $tenant_id,
      transport_order_id: $transport_order_id,
      freight_request_number: $fr_number,
      request_type: "MINI_TENDER",
      shipper_company_id: $shipper_id,
      response_deadline: "2026-12-31T18:00:00Z",
      currency_code: "RUB"
    }')"

  if ! api_request_optional POST "$RFX_SERVICE_URL/v1/freight-requests/from-transport-order" "$body"; then
    if [[ "$HTTP_CODE" -eq 409 ]]; then
      find_freight_request_id "$fr_number"
      return 0
    fi
    warn "could not create freight request $fr_number (HTTP $HTTP_CODE)"
    echo ""
    return 0
  fi

  existing_id="$(echo "$HTTP_BODY" | jq -r '.id')"
  status="$(echo "$HTTP_BODY" | jq -r '.status // empty')"
  if [[ "$status" == "DRAFT" ]]; then
    api_request_optional POST "$RFX_SERVICE_URL/v1/freight-requests/${existing_id}/publish?tenant_id=${TENANT_ID}" "" || true
  fi
  pass "freight request created: $fr_number"
  echo "$existing_id"
}

publish_freight_request_if_needed() {
  local fr_id="$1"
  local raw status
  raw="$(api_get_json "$RFX_SERVICE_URL/v1/freight-requests/${fr_id}?tenant_id=${TENANT_ID}")"
  status="$(echo "$raw" | jq -r '.status // empty')"
  if [[ "$status" == "DRAFT" ]]; then
    api_request_optional POST "$RFX_SERVICE_URL/v1/freight-requests/${fr_id}/publish?tenant_id=${TENANT_ID}" "" || true
  fi
}

find_bid_id() {
  local fr_id="$1"
  local bid_number="$2"
  local raw
  raw="$(api_get_json "$RFX_SERVICE_URL/v1/freight-requests/${fr_id}/bids?tenant_id=${TENANT_ID}")"
  if [[ -z "$raw" ]]; then
    echo ""
    return 0
  fi
  echo "$raw" | jq -r --arg num "$bid_number" '
    (.items // [])[] | select(.bid_number == $num) | .id' | head -n 1
}

ensure_bid() {
  local fr_id="$1"
  local carrier_id="$2"
  local bid_number="$3"
  local route_desc="$4"
  local existing_id body status

  publish_freight_request_if_needed "$fr_id"

  existing_id="$(find_bid_id "$fr_id" "$bid_number")"
  if [[ -n "$existing_id" && "$existing_id" != "null" ]]; then
    skip "bid exists: $bid_number ($existing_id)"
    echo "$existing_id"
    return 0
  fi

  body="$(jq -n \
    --arg tenant_id "$TENANT_ID" \
    --arg carrier_id "$carrier_id" \
    --arg bid_number "$bid_number" \
    --arg route_desc "$route_desc" \
    '{
      tenant_id: $tenant_id,
      carrier_company_id: $carrier_id,
      bid_number: $bid_number,
      currency_code: "RUB",
      vat_rate: 20,
      valid_until: "2026-12-31T18:00:00Z",
      items: [
        {
          description: $route_desc,
          base_amount: 95000,
          fuel_surcharge: 4500,
          toll_amount: 2500,
          extra_charges: 0,
          vat_rate: 20
        }
      ]
    }')"

  if ! api_request_optional POST "$RFX_SERVICE_URL/v1/freight-requests/${fr_id}/bids" "$body"; then
    if [[ "$HTTP_CODE" -eq 409 ]]; then
      find_bid_id "$fr_id" "$bid_number"
      return 0
    fi
    warn "could not create bid $bid_number (HTTP $HTTP_CODE)"
    echo ""
    return 0
  fi

  existing_id="$(echo "$HTTP_BODY" | jq -r '.id')"
  status="$(echo "$HTTP_BODY" | jq -r '.status // empty')"
  if [[ "$status" == "DRAFT" ]]; then
    api_request_optional POST "$RFX_SERVICE_URL/v1/bids/${existing_id}/submit?tenant_id=${TENANT_ID}" "" || true
  fi
  status="$(api_get_json "$RFX_SERVICE_URL/v1/freight-requests/${fr_id}/bids?tenant_id=${TENANT_ID}" | jq -r --arg id "$existing_id" '(.items // [])[] | select(.id == $id) | .status' | head -n 1)"
  if [[ "$status" == "SUBMITTED" ]]; then
    api_request_optional POST "$RFX_SERVICE_URL/v1/bids/${existing_id}/accept?tenant_id=${TENANT_ID}" "" || true
  fi
  pass "bid ready: $bid_number"
  echo "$existing_id"
}

accept_bid_if_needed() {
  local fr_id="$1"
  local bid_id="$2"
  local raw status
  raw="$(api_get_json "$RFX_SERVICE_URL/v1/freight-requests/${fr_id}/bids?tenant_id=${TENANT_ID}")"
  status="$(echo "$raw" | jq -r --arg id "$bid_id" '(.items // [])[] | select(.id == $id) | .status' | head -n 1)"
  if [[ "$status" == "DRAFT" ]]; then
    api_request_optional POST "$RFX_SERVICE_URL/v1/bids/${bid_id}/submit?tenant_id=${TENANT_ID}" "" || true
    status="SUBMITTED"
  fi
  if [[ "$status" == "SUBMITTED" ]]; then
    api_request_optional POST "$RFX_SERVICE_URL/v1/bids/${bid_id}/accept?tenant_id=${TENANT_ID}" "" || true
  fi
}

find_rfx_event_id() {
  local rfx_number="$1"
  local raw
  raw="$(api_get_json "$RFX_SERVICE_URL/v1/rfx-events?tenant_id=${TENANT_ID}&limit=${LIST_LIMIT}&offset=0")"
  if [[ -z "$raw" ]]; then
    echo ""
    return 0
  fi
  echo "$raw" | jq -r --arg num "$rfx_number" '
    (.items // [])[] | select(.rfx_number == $num) | .id' | head -n 1
}

ensure_rfx_event() {
  local rfx_number="$1"
  local owner_id="$2"
  local title="$3"
  local existing_id body status

  existing_id="$(find_rfx_event_id "$rfx_number")"
  if [[ -n "$existing_id" && "$existing_id" != "null" ]]; then
    skip "RFX event exists: $rfx_number ($existing_id)"
    echo "$existing_id"
    return 0
  fi

  body="$(jq -n \
    --arg tenant_id "$TENANT_ID" \
    --arg rfx_number "$rfx_number" \
    --arg owner_id "$owner_id" \
    --arg title "$title" \
    '{
      tenant_id: $tenant_id,
      rfx_number: $rfx_number,
      rfx_type: "LANE_TENDER",
      category: "FREIGHT",
      title: $title,
      description: "Демо-тендер на перевозки для dev UI",
      owner_company_id: $owner_id,
      currency_code: "RUB",
      response_deadline: "2026-12-31T18:00:00Z"
    }')"

  if ! api_request_optional POST "$RFX_SERVICE_URL/v1/rfx-events" "$body"; then
    if [[ "$HTTP_CODE" -eq 409 ]]; then
      find_rfx_event_id "$rfx_number"
      return 0
    fi
    warn "could not create RFX event $rfx_number (HTTP $HTTP_CODE)"
    echo ""
    return 0
  fi

  existing_id="$(echo "$HTTP_BODY" | jq -r '.id')"
  status="$(echo "$HTTP_BODY" | jq -r '.status // empty')"
  if [[ "$status" == "DRAFT" ]]; then
    api_request_optional POST "$RFX_SERVICE_URL/v1/rfx-events/${existing_id}/publish?tenant_id=${TENANT_ID}" "" || true
  fi
  pass "RFX event created: $rfx_number"
  echo "$existing_id"
}

find_driver_id() {
  local license_number="$1"
  local raw
  raw="$(api_get_json "$SHIPMENT_SERVICE_URL/v1/drivers?tenant_id=${TENANT_ID}&limit=${LIST_LIMIT}&offset=0")"
  if [[ -z "$raw" ]]; then
    echo ""
    return 0
  fi
  echo "$raw" | jq -r --arg lic "$license_number" '
    (.items // [])[] | select(.license_number == $lic) | .id' | head -n 1
}

ensure_driver() {
  local carrier_id="$1"
  local license_number="$2"
  local full_name="$3"
  local existing_id body

  existing_id="$(find_driver_id "$license_number")"
  if [[ -n "$existing_id" && "$existing_id" != "null" ]]; then
    echo "$existing_id"
    return 0
  fi

  body="$(jq -n \
    --arg tenant_id "$TENANT_ID" \
    --arg carrier_id "$carrier_id" \
    --arg full_name "$full_name" \
    --arg license_number "$license_number" \
    '{
      tenant_id: $tenant_id,
      carrier_company_id: $carrier_id,
      full_name: $full_name,
      license_number: $license_number,
      license_country: "RU"
    }')"

  if api_request_optional POST "$SHIPMENT_SERVICE_URL/v1/drivers" "$body"; then
    echo "$HTTP_BODY" | jq -r '.id'
    return 0
  fi

  find_driver_id "$license_number"
}

find_vehicle_id() {
  local plate_number="$1"
  local raw
  raw="$(api_get_json "$SHIPMENT_SERVICE_URL/v1/vehicles?tenant_id=${TENANT_ID}&limit=${LIST_LIMIT}&offset=0")"
  if [[ -z "$raw" ]]; then
    echo ""
    return 0
  fi
  echo "$raw" | jq -r --arg plate "$plate_number" '
    (.items // [])[] | select(.plate_number == $plate) | .id' | head -n 1
}

ensure_vehicle() {
  local carrier_id="$1"
  local plate_number="$2"
  local existing_id body

  existing_id="$(find_vehicle_id "$plate_number")"
  if [[ -n "$existing_id" && "$existing_id" != "null" ]]; then
    echo "$existing_id"
    return 0
  fi

  body="$(jq -n \
    --arg tenant_id "$TENANT_ID" \
    --arg carrier_id "$carrier_id" \
    --arg plate_number "$plate_number" \
    '{
      tenant_id: $tenant_id,
      carrier_company_id: $carrier_id,
      plate_number: $plate_number,
      vehicle_type: "TRUCK",
      equipment_type: "TENT_20T",
      registration_country: "RU"
    }')"

  if api_request_optional POST "$SHIPMENT_SERVICE_URL/v1/vehicles" "$body"; then
    echo "$HTTP_BODY" | jq -r '.id'
    return 0
  fi

  find_vehicle_id "$plate_number"
}

find_shipment_id() {
  local shipment_number="$1"
  local raw
  raw="$(api_get_json "$SHIPMENT_SERVICE_URL/v1/shipments?tenant_id=${TENANT_ID}&limit=${LIST_LIMIT}&offset=0")"
  if [[ -z "$raw" ]]; then
    echo ""
    return 0
  fi
  echo "$raw" | jq -r --arg num "$shipment_number" '
    (.items // [])[] | select(.shipment_number == $num) | .id' | head -n 1
}

get_shipment_status() {
  local shipment_id="$1"
  local raw
  raw="$(api_get_json "$SHIPMENT_SERVICE_URL/v1/shipments/${shipment_id}")"
  echo "$raw" | jq -r '.status // empty'
}

ensure_shipment_from_bid() {
  local shipment_number="$1"
  local bid_id="$2"
  local transport_order_id="$3"
  local existing_id body

  existing_id="$(find_shipment_id "$shipment_number")"
  if [[ -n "$existing_id" && "$existing_id" != "null" ]]; then
    skip "shipment exists: $shipment_number ($existing_id)"
    echo "$existing_id"
    return 0
  fi

  body="$(jq -n \
    --arg tenant_id "$TENANT_ID" \
    --arg shipment_number "$shipment_number" \
    --arg bid_id "$bid_id" \
    --arg transport_order_id "$transport_order_id" \
    '{
      tenant_id: $tenant_id,
      shipment_number: $shipment_number,
      bid_id: $bid_id,
      transport_order_id: $transport_order_id,
      planned_pickup_at: "2026-08-01T09:00:00Z",
      planned_delivery_at: "2026-08-03T18:00:00Z"
    }')"

  if ! api_request_optional POST "$SHIPMENT_SERVICE_URL/v1/shipments/from-bid" "$body"; then
    if [[ "$HTTP_CODE" -eq 409 ]]; then
      find_shipment_id "$shipment_number"
      return 0
    fi
    warn "could not create shipment $shipment_number (HTTP $HTTP_CODE)"
    echo ""
    return 0
  fi

  pass "shipment created: $shipment_number"
  echo "$HTTP_BODY" | jq -r '.id'
}

patch_shipment_status() {
  local shipment_id="$1"
  local status="$2"
  local body

  if [[ "$status" == "LOADED" || "$status" == "DELIVERED" ]]; then
    body="$(jq -n --arg tenant_id "$TENANT_ID" --arg status "$status" '{
      tenant_id: $tenant_id,
      status: $status,
      actual_time: "2026-08-02T12:00:00Z"
    }')"
  else
    body="$(jq -n --arg tenant_id "$TENANT_ID" --arg status "$status" '{
      tenant_id: $tenant_id,
      status: $status
    }')"
  fi

  api_request_optional PATCH "$SHIPMENT_SERVICE_URL/v1/shipments/${shipment_id}/status" "$body" || true
}

status_rank() {
  case "$1" in
    CARRIER_ASSIGNED) echo 1 ;;
    ACCEPTED_BY_CARRIER) echo 2 ;;
    VEHICLE_ASSIGNED) echo 3 ;;
    DRIVER_ASSIGNED) echo 4 ;;
    PICKUP_SLOT_BOOKED) echo 5 ;;
    DELIVERY_SLOT_BOOKED) echo 6 ;;
    IN_PICKUP) echo 7 ;;
    LOADED) echo 8 ;;
    IN_TRANSIT) echo 9 ;;
    ARRIVED_AT_CONSIGNEE) echo 10 ;;
    UNLOADING) echo 11 ;;
    DELIVERED) echo 12 ;;
    DELIVERY_CONFIRMED) echo 13 ;;
    DOCUMENTS_COMPLETED) echo 14 ;;
    READY_FOR_BILLING) echo 15 ;;
    *) echo 0 ;;
  esac
}

advance_shipment_to() {
  local shipment_id="$1"
  local target_status="$2"
  local current target_rank current_rank status

  target_rank="$(status_rank "$target_status")"
  current="$(get_shipment_status "$shipment_id")"
  current_rank="$(status_rank "$current")"

  if [[ "$current_rank" -ge "$target_rank" && "$target_rank" -gt 0 ]]; then
    skip "shipment $shipment_id already at or beyond $target_status (current: $current)"
    return 0
  fi

  local -a chain=(
    CARRIER_ASSIGNED
    ACCEPTED_BY_CARRIER
    DRIVER_ASSIGNED
    PICKUP_SLOT_BOOKED
    IN_PICKUP
    LOADED
    IN_TRANSIT
    ARRIVED_AT_CONSIGNEE
    UNLOADING
    DELIVERED
    DELIVERY_CONFIRMED
    DOCUMENTS_COMPLETED
    READY_FOR_BILLING
  )

  for status in "${chain[@]}"; do
    current="$(get_shipment_status "$shipment_id")"
    current_rank="$(status_rank "$current")"
    local step_rank
    step_rank="$(status_rank "$status")"
    if [[ "$current_rank" -lt "$step_rank" && "$step_rank" -le "$target_rank" ]]; then
      patch_shipment_status "$shipment_id" "$status"
    fi
    current="$(get_shipment_status "$shipment_id")"
    if [[ "$(status_rank "$current")" -ge "$target_rank" ]]; then
      break
    fi
  done

  pass "shipment $shipment_id advanced toward $target_status (now: $(get_shipment_status "$shipment_id"))"
}

assign_driver_and_vehicle() {
  local shipment_id="$1"
  local driver_id="$2"
  local vehicle_id="$3"
  local current body

  current="$(get_shipment_status "$shipment_id")"
  if [[ "$(status_rank "$current")" -lt "$(status_rank "ACCEPTED_BY_CARRIER")" ]]; then
    body="$(jq -n --arg tenant_id "$TENANT_ID" --arg driver_id "$driver_id" '{
      tenant_id: $tenant_id,
      driver_id: $driver_id
    }')"
    api_request_optional POST "$SHIPMENT_SERVICE_URL/v1/shipments/${shipment_id}/assign-driver" "$body" || true
  fi

  current="$(get_shipment_status "$shipment_id")"
  if [[ "$(status_rank "$current")" -lt "$(status_rank "DRIVER_ASSIGNED")" ]]; then
    body="$(jq -n --arg tenant_id "$TENANT_ID" --arg vehicle_id "$vehicle_id" '{
      tenant_id: $tenant_id,
      vehicle_id: $vehicle_id
    }')"
    api_request_optional POST "$SHIPMENT_SERVICE_URL/v1/shipments/${shipment_id}/assign-vehicle" "$body" || true
  fi
}

find_document_id() {
  local document_number="$1"
  local raw
  raw="$(api_get_json "$DOCUMENT_SERVICE_URL/v1/documents?tenant_id=${TENANT_ID}&limit=${LIST_LIMIT}&offset=0")"
  if [[ -z "$raw" ]]; then
    echo ""
    return 0
  fi
  echo "$raw" | jq -r --arg num "$document_number" '
    (.items // [])[] | select(.document_number == $num) | .id' | head -n 1
}

ensure_document() {
  local document_number="$1"
  local owner_id="$2"
  local shipment_id="$3"
  local shipment_number="$4"
  local existing_id body

  existing_id="$(find_document_id "$document_number")"
  if [[ -n "$existing_id" && "$existing_id" != "null" ]]; then
    skip "document exists: $document_number ($existing_id)"
    echo "$existing_id"
    return 0
  fi

  body="$(jq -n \
    --arg tenant_id "$TENANT_ID" \
    --arg document_number "$document_number" \
    --arg owner_id "$owner_id" \
    --arg shipment_id "$shipment_id" \
    --arg shipment_number "$shipment_number" \
    '{
      tenant_id: $tenant_id,
      document_number: $document_number,
      document_type: "POD",
      owner_company_id: $owner_id,
      related_entity_type: "SHIPMENT",
      related_entity_id: $shipment_id,
      legal_language: "ru-RU",
      payload_json: {
        shipment_number: $shipment_number,
        delivery_confirmed: true
      }
    }')"

  if ! api_request_optional POST "$DOCUMENT_SERVICE_URL/v1/documents" "$body"; then
    if [[ "$HTTP_CODE" -eq 409 ]]; then
      find_document_id "$document_number"
      return 0
    fi
    warn "could not create document $document_number (HTTP $HTTP_CODE)"
    echo ""
    return 0
  fi

  existing_id="$(echo "$HTTP_BODY" | jq -r '.id')"
  pass "document created: $document_number"
  echo "$existing_id"
}

find_billing_register_id() {
  local register_number="$1"
  local raw
  raw="$(api_get_json "$BILLING_REGISTER_SERVICE_URL/v1/billing-registers?tenant_id=${TENANT_ID}&limit=${LIST_LIMIT}&offset=0")"
  if [[ -z "$raw" ]]; then
    echo ""
    return 0
  fi
  echo "$raw" | jq -r --arg num "$register_number" '
    (.items // [])[] | select(.register_number == $num) | .id' | head -n 1
}

ensure_billing_register() {
  local register_number="$1"
  local customer_id="$2"
  local contractor_id="$3"
  local shipment_id="$4"
  local route_desc="$5"
  local admin_user_id="$6"
  local existing_id body status items_count

  existing_id="$(find_billing_register_id "$register_number")"
  if [[ -n "$existing_id" && "$existing_id" != "null" ]]; then
    skip "billing register exists: $register_number ($existing_id)"
    echo "$existing_id"
    return 0
  fi

  body="$(jq -n \
    --arg tenant_id "$TENANT_ID" \
    --arg register_number "$register_number" \
    --arg customer_id "$customer_id" \
    --arg contractor_id "$contractor_id" \
    '{
      tenant_id: $tenant_id,
      register_number: $register_number,
      customer_company_id: $customer_id,
      contractor_company_id: $contractor_id,
      period_from: "2026-08-01",
      period_to: "2026-08-15",
      currency_code: "RUB",
      vat_rate: 20
    }')"

  if ! api_request_optional POST "$BILLING_REGISTER_SERVICE_URL/v1/billing-registers" "$body"; then
    if [[ "$HTTP_CODE" -eq 409 ]]; then
      find_billing_register_id "$register_number"
      return 0
    fi
    warn "could not create billing register $register_number (HTTP $HTTP_CODE)"
    echo ""
    return 0
  fi

  existing_id="$(echo "$HTTP_BODY" | jq -r '.id')"

  body="$(jq -n \
    --arg tenant_id "$TENANT_ID" \
    --arg shipment_id "$shipment_id" \
    --arg route_desc "$route_desc" \
    '{
      tenant_id: $tenant_id,
      shipment_id: $shipment_id,
      route_description: $route_desc,
      pickup_date: "2026-08-01",
      delivery_date: "2026-08-03",
      base_amount: 95000,
      extra_charges: 4500,
      penalties: 0,
      vat_rate: 20
    }')"

  api_request_optional POST "$BILLING_REGISTER_SERVICE_URL/v1/billing-registers/${existing_id}/items" "$body" || true

  body="$(jq -n --arg tenant_id "$TENANT_ID" '{ tenant_id: $tenant_id }')"
  api_request_optional POST "$BILLING_REGISTER_SERVICE_URL/v1/billing-registers/${existing_id}/calculate" "$body" || true

  body="$(jq -n --arg tenant_id "$TENANT_ID" --arg approved_by "$admin_user_id" '{
    tenant_id: $tenant_id,
    approved_by: $approved_by
  }')"
  api_request_optional POST "$BILLING_REGISTER_SERVICE_URL/v1/billing-registers/${existing_id}/approve" "$body" || true

  UPD_PAYLOAD="$(jq -nc \
    --arg tenant_id "$TENANT_ID" \
    --arg upd_number "DEMO-UPD-001" \
    --arg seller "$contractor_id" \
    --arg buyer "$customer_id" \
    '{
      tenant_id: $tenant_id,
      upd_number: $upd_number,
      upd_date: "2026-08-16",
      seller_company_id: $seller,
      buyer_company_id: $buyer,
      function_code: "\u0421\u0427\u0424\u0414\u041e\u041f"
    }')"
  api_request_optional POST "$BILLING_REGISTER_SERVICE_URL/v1/billing-registers/${existing_id}/upd" "$UPD_PAYLOAD" || true

  pass "billing register created: $register_number"
  echo "$existing_id"
}

count_demo_entities() {
  local companies tos frs shipments docs registers rfx
  companies="$(api_get_json "$COMPANY_SERVICE_URL/v1/companies?tenant_id=${TENANT_ID}&limit=${LIST_LIMIT}&offset=0" | jq '[.items // [] | length] | add // 0')"
  tos="$(api_get_json "$TRANSPORT_ORDER_SERVICE_URL/v1/transport-orders?tenant_id=${TENANT_ID}&limit=${LIST_LIMIT}&offset=0" | jq '[.items // [] | map(select(.order_number | startswith("DEMO-TO-"))) | length] | add // 0')"
  frs="$(api_get_json "$RFX_SERVICE_URL/v1/freight-requests?tenant_id=${TENANT_ID}&limit=${LIST_LIMIT}&offset=0" | jq '[.items // [] | map(select(.freight_request_number | startswith("DEMO-FR-"))) | length] | add // 0')"
  shipments="$(api_get_json "$SHIPMENT_SERVICE_URL/v1/shipments?tenant_id=${TENANT_ID}&limit=${LIST_LIMIT}&offset=0" | jq '[.items // [] | map(select(.shipment_number | startswith("DEMO-SH-"))) | length] | add // 0')"
  docs="$(api_get_json "$DOCUMENT_SERVICE_URL/v1/documents?tenant_id=${TENANT_ID}&limit=${LIST_LIMIT}&offset=0" | jq '[.items // [] | map(select(.document_number | startswith("DEMO-DOC-"))) | length] | add // 0')"
  registers="$(api_get_json "$BILLING_REGISTER_SERVICE_URL/v1/billing-registers?tenant_id=${TENANT_ID}&limit=${LIST_LIMIT}&offset=0" | jq '[.items // [] | map(select(.register_number | startswith("DEMO-BR-"))) | length] | add // 0')"
  rfx="$(api_get_json "$RFX_SERVICE_URL/v1/rfx-events?tenant_id=${TENANT_ID}&limit=${LIST_LIMIT}&offset=0" | jq '[.items // [] | map(select(.rfx_number | startswith("DEMO-RFX-"))) | length] | add // 0')"
  echo "counts companies=${companies} demo_transport_orders=${tos} demo_freight_requests=${frs} demo_shipments=${shipments} demo_documents=${docs} demo_billing_registers=${registers} demo_rfx_events=${rfx}"
}

main() {
  local platform_id shipper_id carrier_id forwarder_id consignee_id
  local shipper_user carrier_user forwarder_user consignee_user admin_user
  local to1 to2 to3 to4 to5
  local fr1 fr2 fr3 bid1 bid2 bid3
  local sh_planned sh_progress sh_billing driver_id vehicle_id
  local doc_id br_id rfx_id

  require_cmd curl
  require_cmd jq

  echo ""
  echo "Freight Platform — seed demo data"
  echo "Tenant: $TENANT_ID"
  echo ""

  step "Check API Gateway health"
  if ! check_gateway_health; then
    echo "ERROR: API Gateway unavailable. Run: make platform-up-no-build && make seed-dev-admin" >&2
    exit 1
  fi
  pass "API Gateway is healthy"

  step "Ensure demo companies"
  platform_id="$(ensure_company "ООО 7Rights Dev" "PLATFORM_OPERATOR" "7Rights Dev")"
  shipper_id="$(ensure_company "ООО Грузовладелец Север" "SHIPPER" "Грузовладелец Север")"
  carrier_id="$(ensure_company "ООО Перевозчик Волга" "CARRIER" "Перевозчик Волга")"
  forwarder_id="$(ensure_company "ООО Экспедитор Логистик" "FORWARDER" "Экспедитор Логистик")"
  consignee_id="$(ensure_company "ООО Грузополучатель Центр" "CONSIGNEE" "Грузополучатель Центр")"

  for cid in "$platform_id" "$shipper_id" "$carrier_id" "$forwarder_id" "$consignee_id"; do
    if [[ -z "$cid" || "$cid" == "null" ]]; then
      echo "ERROR: failed to resolve required company id" >&2
      exit 1
    fi
  done

  step "Ensure demo users and memberships"
  admin_user="$(find_user_id "admin@7rights.local")"
  shipper_user="$(ensure_user "shipper@7rights.local" "Демо Грузовладелец")"
  carrier_user="$(ensure_user "carrier@7rights.local" "Демо Перевозчик")"
  forwarder_user="$(ensure_user "forwarder@7rights.local" "Демо Экспедитор")"
  consignee_user="$(ensure_user "consignee@7rights.local" "Демо Грузополучатель")"

  ensure_membership "$shipper_id" "$shipper_user" "Логист грузовладельца"
  ensure_membership "$carrier_id" "$carrier_user" "Диспетчер перевозчика"
  ensure_membership "$forwarder_id" "$forwarder_user" "Менеджер закупок"
  ensure_membership "$consignee_id" "$consignee_user" "Оператор получателя"

  ensure_role "$shipper_id" "$shipper_user" "SHIPPER_LOGIST"
  ensure_role "$carrier_id" "$carrier_user" "CARRIER_DISPATCHER"
  ensure_role "$forwarder_id" "$forwarder_user" "PROCUREMENT_MANAGER"
  ensure_role "$consignee_id" "$consignee_user" "CONSIGNEE_OPERATOR"

  step "Create demo transport orders"
  to1="$(ensure_transport_order "DEMO-TO-001" "$shipper_id" "$consignee_id" \
    "DEMO Склад Москва" "Москва" "DEMO Склад Санкт-Петербург" "Санкт-Петербург" "2026-08-01" "2026-08-03")"
  to2="$(ensure_transport_order "DEMO-TO-002" "$shipper_id" "$consignee_id" \
    "DEMO Склад Казань" "Казань" "DEMO Склад Екатеринбург" "Екатеринбург" "2026-08-05" "2026-08-07")"
  to3="$(ensure_transport_order "DEMO-TO-003" "$shipper_id" "$consignee_id" \
    "DEMO Склад Краснодар" "Краснодар" "DEMO Склад Ростов" "Ростов-на-Дону" "2026-08-08" "2026-08-09")"
  to4="$(ensure_transport_order "DEMO-TO-004" "$shipper_id" "$consignee_id" \
    "DEMO Склад Новосибирск" "Новосибирск" "DEMO Склад Омск" "Омск" "2026-08-10" "2026-08-11")"
  to5="$(ensure_transport_order "DEMO-TO-005" "$shipper_id" "$consignee_id" \
    "DEMO Склад Нижний Новгород" "Нижний Новгород" "DEMO Склад Самара" "Самара" "2026-08-12" "2026-08-13")"

  step "Create demo freight requests and bids"
  fr1="$(ensure_freight_request "DEMO-FR-001" "$to1" "$shipper_id")"
  fr2="$(ensure_freight_request "DEMO-FR-002" "$to2" "$shipper_id")"
  fr3="$(ensure_freight_request "DEMO-FR-003" "$to3" "$shipper_id")"

  bid1="$(ensure_bid "$fr1" "$carrier_id" "DEMO-BID-001" "Москва — Санкт-Петербург")"
  bid2="$(ensure_bid "$fr2" "$carrier_id" "DEMO-BID-002" "Казань — Екатеринбург")"
  bid3="$(ensure_bid "$fr3" "$forwarder_id" "DEMO-BID-003" "Краснодар — Ростов-на-Дону")"

  accept_bid_if_needed "$fr1" "$bid1"
  accept_bid_if_needed "$fr2" "$bid2"
  accept_bid_if_needed "$fr3" "$bid3"

  step "Create demo RFX event"
  rfx_id="$(ensure_rfx_event "DEMO-RFX-001" "$shipper_id" "Демо тендер: магистральные перевозки")"

  step "Create demo shipments (planned / in progress / billing-ready)"
  sh_planned="$(ensure_shipment_from_bid "DEMO-SH-PLANNED" "$bid1" "$to1")"
  sh_progress="$(ensure_shipment_from_bid "DEMO-SH-IN-PROGRESS" "$bid2" "$to2")"
  sh_billing="$(ensure_shipment_from_bid "DEMO-SH-BILLING" "$bid3" "$to3")"

  driver_id="$(ensure_driver "$carrier_id" "DEMO-LIC-001" "Иван Демо-Водитель")"
  vehicle_id="$(ensure_vehicle "$carrier_id" "DEMO-A123BC77")"

  if [[ -n "$sh_progress" && "$sh_progress" != "null" ]]; then
    assign_driver_and_vehicle "$sh_progress" "$driver_id" "$vehicle_id"
    advance_shipment_to "$sh_progress" "IN_TRANSIT"
  fi

  if [[ -n "$sh_billing" && "$sh_billing" != "null" ]]; then
    assign_driver_and_vehicle "$sh_billing" "$driver_id" "$vehicle_id"
    advance_shipment_to "$sh_billing" "READY_FOR_BILLING"
  fi

  step "Create demo document"
  doc_id="$(ensure_document "DEMO-DOC-001" "$shipper_id" "$sh_billing" "DEMO-SH-BILLING")"

  step "Create demo billing register with UPD"
  if [[ -n "$admin_user" && "$admin_user" != "null" ]]; then
    br_id="$(ensure_billing_register "DEMO-BR-001" "$shipper_id" "$forwarder_id" "$sh_billing" \
      "Краснодар — Ростов-на-Дону (demo)" "$admin_user")"
  else
    warn "admin user not found; skipping billing register (run make seed-dev-admin first)"
    br_id=""
  fi

  step "Demo entity counts"
  count_demo_entities

  echo ""
  echo "Demo seed summary"
  echo "-----------------"
  echo "tenant_id:       $TENANT_ID"
  echo "platform_id:     $platform_id"
  echo "shipper_id:      $shipper_id"
  echo "carrier_id:      $carrier_id"
  echo "forwarder_id:    $forwarder_id"
  echo "consignee_id:    $consignee_id"
  echo "transport_orders: DEMO-TO-001..005"
  echo "freight_requests: DEMO-FR-001..003"
  echo "rfx_event:       DEMO-RFX-001 ($rfx_id)"
  echo "shipments:       DEMO-SH-PLANNED, DEMO-SH-IN-PROGRESS, DEMO-SH-BILLING"
  echo "document:        DEMO-DOC-001 ($doc_id)"
  echo "billing_register: DEMO-BR-001 ($br_id)"
  echo ""
  pass "Demo seed completed"
}

main "$@"
