#!/usr/bin/env bash
set -euo pipefail

API_GATEWAY_URL="${API_GATEWAY_URL:-http://localhost:8080}"
IDENTITY_SERVICE_URL="${IDENTITY_SERVICE_URL:-http://localhost:8081}"
COMPANY_SERVICE_URL="${COMPANY_SERVICE_URL:-http://localhost:8082}"

TENANT_ID="${TENANT_ID:-74519f22-ff9b-4a8b-8fff-a958c689682f}"
TENANT_CODE="${TENANT_CODE:-dev-7rights}"
TENANT_NAME="${TENANT_NAME:-7Rights Dev Tenant}"

ADMIN_EMAIL="${ADMIN_EMAIL:-admin@7rights.local}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-Admin123456!}"
ADMIN_FULL_NAME="${ADMIN_FULL_NAME:-7Rights Dev Admin}"

COMPANY_LEGAL_NAME="${COMPANY_LEGAL_NAME:-ООО 7Rights Dev}"
COMPANY_TYPE="${COMPANY_TYPE:-PLATFORM_OPERATOR}"

POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-freight_postgres}"
POSTGRES_USER="${POSTGRES_USER:-freight}"
POSTGRES_DB="${POSTGRES_DB:-freight_platform}"

step() { echo "==> $1"; }
pass() { echo "OK: $1"; }
warn() { echo "TODO: $1"; }
fail() { echo "ERROR: $1" >&2; exit 1; }

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

api_request_optional() {
  local method="$1"
  local url="$2"
  local data="${3:-}"
  local raw

  if [[ -n "$data" ]]; then
    raw="$(curl -sS -w "__HTTP_CODE__:%{http_code}" -X "$method" "$url" \
      -H "Content-Type: application/json" \
      -d "$data" 2>/dev/null || true)"
  else
    raw="$(curl -sS -w "__HTTP_CODE__:%{http_code}" -X "$method" "$url" 2>/dev/null || true)"
  fi

  if [[ -z "$raw" ]]; then
    HTTP_CODE="0"
    HTTP_BODY=""
    return 1
  fi

  parse_http "$raw"
  [[ "$HTTP_CODE" -ge 200 && "$HTTP_CODE" -lt 300 ]]
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

ensure_tenant() {
  if ! command -v docker >/dev/null 2>&1; then
    warn "docker not available; assuming tenant $TENANT_ID already exists"
    return 0
  fi
  if ! docker inspect "$POSTGRES_CONTAINER" >/dev/null 2>&1; then
    warn "postgres container not running; assuming tenant $TENANT_ID already exists"
    return 0
  fi

  step "Ensure dev tenant exists in PostgreSQL"
  docker exec -i "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -c "
    INSERT INTO core.tenants (id, code, name, country_code, default_locale, default_currency)
    VALUES ('${TENANT_ID}', '${TENANT_CODE}', '${TENANT_NAME}', 'RU', 'ru-RU', 'RUB')
    ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name;
  " >/dev/null
  pass "tenant ready: $TENANT_ID"
}

find_company_id() {
  local raw
  if ! raw="$(curl -sS "$COMPANY_SERVICE_URL/v1/companies?tenant_id=${TENANT_ID}&limit=200&offset=0" 2>/dev/null)"; then
    echo ""
    return 0
  fi
  echo "$raw" | jq -r --arg name "$COMPANY_LEGAL_NAME" '
    (.items // [])[] | select(.legal_name == $name) | .id' | head -n 1
}

create_company() {
  local body resp
  body="$(jq -n \
    --arg tenant_id "$TENANT_ID" \
    --arg legal_name "$COMPANY_LEGAL_NAME" \
    --arg company_type "$COMPANY_TYPE" \
    '{
      tenant_id: $tenant_id,
      legal_name: $legal_name,
      short_name: "7Rights Dev",
      company_type: $company_type,
      country_code: "RU",
      preferred_locale: "ru-RU"
    }')"

  if api_request_optional POST "$COMPANY_SERVICE_URL/v1/companies" "$body"; then
    echo "$HTTP_BODY" | jq -r '.id'
    return 0
  fi
  echo ""
  return 1
}

find_user_id() {
  local raw
  if ! raw="$(curl -sS "$IDENTITY_SERVICE_URL/v1/users?tenant_id=${TENANT_ID}&limit=200&offset=0" 2>/dev/null)"; then
    echo ""
    return 0
  fi
  echo "$raw" | jq -r --arg email "$ADMIN_EMAIL" '
    (.items // [])[] | select(.email == $email) | .id' | head -n 1
}

create_user() {
  local body
  body="$(jq -n \
    --arg tenant_id "$TENANT_ID" \
    --arg email "$ADMIN_EMAIL" \
    --arg password "$ADMIN_PASSWORD" \
    --arg full_name "$ADMIN_FULL_NAME" \
    '{
      tenant_id: $tenant_id,
      email: $email,
      password: $password,
      full_name: $full_name,
      preferred_locale: "ru-RU"
    }')"

  if api_request_optional POST "$IDENTITY_SERVICE_URL/v1/users" "$body"; then
    echo "$HTTP_BODY" | jq -r '.id'
    return 0
  fi

  if [[ "$HTTP_CODE" -eq 409 || "$HTTP_CODE" -eq 422 ]]; then
    find_user_id
    return 0
  fi

  echo ""
  return 1
}

ensure_membership() {
  local company_id="$1"
  local user_id="$2"
  local body members_raw

  members_raw="$(curl -sS "$COMPANY_SERVICE_URL/v1/companies/${company_id}/members?tenant_id=${TENANT_ID}&limit=200&offset=0" 2>/dev/null || true)"
  if echo "$members_raw" | jq -e --arg uid "$user_id" '(.items // [])[] | select(.user_id == $uid)' >/dev/null 2>&1; then
    pass "membership already exists for user $user_id"
    return 0
  fi

  body="$(jq -n --arg tenant_id "$TENANT_ID" --arg user_id "$user_id" '{
    tenant_id: $tenant_id,
    user_id: $user_id,
    position: "Platform Administrator"
  }')"

  if api_request_optional POST "$COMPANY_SERVICE_URL/v1/companies/${company_id}/members" "$body"; then
    pass "membership created"
    return 0
  fi

  warn "could not create company membership (HTTP $HTTP_CODE); endpoint may require auth"
}

assign_platform_admin_role() {
  local company_id="$1"
  local user_id="$2"
  local roles_raw role_id body

  roles_raw="$(curl -sS "$IDENTITY_SERVICE_URL/v1/roles?tenant_id=${TENANT_ID}&limit=200&offset=0" 2>/dev/null || true)"
  role_id="$(echo "$roles_raw" | jq -r '(.items // [])[] | select(.code == "PLATFORM_ADMIN") | .id' | head -n 1)"

  if [[ -z "$role_id" || "$role_id" == "null" ]]; then
    warn "PLATFORM_ADMIN role not found in tenant; run migrations (make migrate-up)"
    return 0
  fi

  body="$(jq -n --arg tenant_id "$TENANT_ID" --arg role_id "$role_id" '{
    tenant_id: $tenant_id,
    role_id: $role_id
  }')"

  if api_request_optional POST "$IDENTITY_SERVICE_URL/v1/users/${user_id}/companies/${company_id}/roles" "$body"; then
    pass "PLATFORM_ADMIN role assigned"
    return 0
  fi

  if [[ "$HTTP_CODE" -eq 409 ]]; then
    pass "PLATFORM_ADMIN role already assigned"
    return 0
  fi

  warn "could not assign PLATFORM_ADMIN role (HTTP $HTTP_CODE)"
}

main() {
  require_cmd curl
  require_cmd jq

  echo ""
  echo "Freight Platform — seed dev admin"
  echo "Tenant: $TENANT_ID"
  echo "Email:  $ADMIN_EMAIL"
  echo ""

  step "Check API Gateway health"
  if ! check_gateway_health; then
    fail "API Gateway unavailable. Start platform first: make platform-up"
  fi
  pass "API Gateway is healthy"

  ensure_tenant

  step "Find or create dev company"
  COMPANY_ID="$(find_company_id)"
  if [[ -z "$COMPANY_ID" || "$COMPANY_ID" == "null" ]]; then
    COMPANY_ID="$(create_company)"
  fi
  if [[ -z "$COMPANY_ID" || "$COMPANY_ID" == "null" ]]; then
    fail "Failed to create or find company '$COMPANY_LEGAL_NAME'"
  fi
  pass "COMPANY_ID=$COMPANY_ID"

  step "Find or create dev admin user"
  USER_ID="$(find_user_id)"
  if [[ -z "$USER_ID" || "$USER_ID" == "null" ]]; then
    USER_ID="$(create_user)"
  fi
  if [[ -z "$USER_ID" || "$USER_ID" == "null" ]]; then
    fail "Failed to create or find user '$ADMIN_EMAIL'"
  fi
  pass "USER_ID=$USER_ID"

  step "Ensure company membership"
  ensure_membership "$COMPANY_ID" "$USER_ID"

  step "Assign PLATFORM_ADMIN role (if available)"
  assign_platform_admin_role "$COMPANY_ID" "$USER_ID"

  echo ""
  pass "Dev admin seed completed"
  echo "Login in web-admin with tenant $TENANT_ID and $ADMIN_EMAIL"
  echo "Mock auth (NUXT_PUBLIC_MOCK_AUTH=true) remains available for local UI development."
}

main "$@"
