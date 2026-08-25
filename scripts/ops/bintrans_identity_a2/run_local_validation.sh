#!/usr/bin/env bash
# Isolated local validation for BINTRANS identity Wave A2.1
# Does NOT touch freight_postgres / freight_postgres_data.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$ROOT"

COMPOSE_BASE="infrastructure/docker-compose/docker-compose.yml"
COMPOSE_OVERLAY="infrastructure/docker-compose/docker-compose.a21-validate.yml"
export COMPOSE_PROJECT_NAME="bintrans_a21_validate"

compose() {
  docker compose -f "$COMPOSE_BASE" -f "$COMPOSE_OVERLAY" "$@"
}

PG_CONTAINER="bintrans_a21_validate_postgres"
TENANT_ID="74519f22-ff9b-4a8b-8fff-a958c689682f"
API_GATEWAY_URL="http://localhost:18080"
OPS_DIR="$ROOT/scripts/ops/bintrans_identity_a2"

step() { echo "==> $1" >&2; }
pass() { echo "OK: $1" >&2; }
fail() { echo "FAIL: $1" >&2; exit 1; }

start_stack() {
  step "Start isolated postgres + migrations + core services"
  compose up -d postgres
  for i in $(seq 1 30); do
    if docker exec "$PG_CONTAINER" pg_isready -U freight -d freight_platform >/dev/null 2>&1; then
      break
    fi
    sleep 2
    [[ "$i" -eq 30 ]] && fail "postgres not ready"
  done
  compose --profile tools run --rm migrate \
    -path=/migrations \
    -database "postgres://freight:freight_password@postgres:5432/freight_platform?sslmode=disable" up
  compose up -d --build identity-service company-service api-gateway
  for i in $(seq 1 90); do
    if curl -sf "${API_GATEWAY_URL}/health" >/dev/null 2>&1; then
      break
    fi
    sleep 3
    [[ "$i" -eq 90 ]] && fail "api gateway not healthy"
  done
}

stop_stack() {
  step "Stop isolated validation stack"
  compose down -v >/dev/null 2>&1 || true
}

trap stop_stack EXIT

run_migration_tests() {
  step "Phase 1 — migration toolkit tests (disposable DB)"
  start_stack

  docker exec -i "$PG_CONTAINER" psql -U freight -d freight_platform -v ON_ERROR_STOP=1 \
    -f - < "$OPS_DIR/fixtures/legacy_dev_identity.sql"
  docker exec -i "$PG_CONTAINER" psql -U freight -d freight_platform -v ON_ERROR_STOP=1 \
    -f - < "$OPS_DIR/preflight.sql" >/tmp/a21_preflight_before.txt

  ADMIN_UUID_BEFORE="$(docker exec -i "$PG_CONTAINER" psql -U freight -d freight_platform -t -A -c \
    "SELECT id FROM core.users WHERE tenant_id='${TENANT_ID}'::uuid AND lower(email)='admin@7rights.local';")"
  RBAC_BEFORE="$(docker exec -i "$PG_CONTAINER" psql -U freight -d freight_platform -t -A -c \
    "SELECT count(*) FROM core.user_roles WHERE user_id='8541a3a3-bde7-4fed-9501-37b9953bf904'::uuid;")"
  MEM_BEFORE="$(docker exec -i "$PG_CONTAINER" psql -U freight -d freight_platform -t -A -c \
    "SELECT count(*) FROM core.company_memberships WHERE user_id='8541a3a3-bde7-4fed-9501-37b9953bf904'::uuid;")"

  docker exec -i "$PG_CONTAINER" psql -U freight -d freight_platform -v ON_ERROR_STOP=1 \
    -f - < "$OPS_DIR/migrate.sql"

  ADMIN_UUID_AFTER="$(docker exec -i "$PG_CONTAINER" psql -U freight -d freight_platform -t -A -c \
    "SELECT id FROM core.users WHERE tenant_id='${TENANT_ID}'::uuid AND lower(email)='admin@bintrans.local';")"
  RBAC_AFTER="$(docker exec -i "$PG_CONTAINER" psql -U freight -d freight_platform -t -A -c \
    "SELECT count(*) FROM core.user_roles WHERE user_id='8541a3a3-bde7-4fed-9501-37b9953bf904'::uuid;")"
  MEM_AFTER="$(docker exec -i "$PG_CONTAINER" psql -U freight -d freight_platform -t -A -c \
    "SELECT count(*) FROM core.company_memberships WHERE user_id='8541a3a3-bde7-4fed-9501-37b9953bf904'::uuid;")"

  [[ "$ADMIN_UUID_BEFORE" == "$ADMIN_UUID_AFTER" ]] || fail "admin UUID changed during migration"
  [[ "$RBAC_BEFORE" == "$RBAC_AFTER" ]] || fail "RBAC count changed"
  [[ "$MEM_BEFORE" == "$MEM_AFTER" ]] || fail "membership count changed"

  docker exec -i "$PG_CONTAINER" psql -U freight -d freight_platform -v ON_ERROR_STOP=1 \
    -f - < "$OPS_DIR/rollback.sql"
  pass "migration forward + rollback"

  stop_stack
  start_stack

  docker exec -i "$PG_CONTAINER" psql -U freight -d freight_platform -v ON_ERROR_STOP=1 \
    -f - < "$OPS_DIR/fixtures/legacy_dev_identity.sql"
  docker exec -i "$PG_CONTAINER" psql -U freight -d freight_platform -v ON_ERROR_STOP=1 \
    -f - < "$OPS_DIR/fixtures/collision_extra_user.sql"

  if docker exec -i "$PG_CONTAINER" psql -U freight -d freight_platform -v ON_ERROR_STOP=1 \
    -f - < "$OPS_DIR/migrate.sql" >/tmp/a21_collision_migrate.txt 2>/tmp/a21_collision_migrate.err; then
    fail "collision migration should abort"
  fi
  grep -q "DUPLICATE_TARGET_EMAIL_POLICY=FAIL_CLOSED" /tmp/a21_collision_migrate.err \
    || fail "expected fail-closed abort message"

  COLLISION_ADMIN_COUNT="$(docker exec -i "$PG_CONTAINER" psql -U freight -d freight_platform -t -A -c \
    "SELECT count(*) FROM core.users WHERE tenant_id='${TENANT_ID}'::uuid AND deleted_at IS NULL AND lower(email)='admin@7rights.local';")"
  [[ "$COLLISION_ADMIN_COUNT" == "1" ]] || fail "partial mutation after collision abort"
  pass "collision abort verified"

  stop_stack
  trap - EXIT
}

assert_count() {
  local email="$1"
  local expected="$2"
  local actual
  actual="$(docker exec -i "$PG_CONTAINER" psql -U freight -d freight_platform -t -A -c \
    "SELECT count(*) FROM core.users WHERE tenant_id='${TENANT_ID}'::uuid AND deleted_at IS NULL AND lower(email)=lower('${email}');")"
  [[ "$actual" == "$expected" ]] || fail "expected ${expected} user(s) for ${email}, got ${actual}"
}

login_check() {
  local email="$1"
  local password="$2"
  local body http_code
  body="$(jq -n --arg tenant_id "$TENANT_ID" --arg email "$email" --arg password "$password" \
    '{tenant_id:$tenant_id,email:$email,password:$password}')"
  http_code="$(curl -sS -o /tmp/a21_login.json -w "%{http_code}" \
    -X POST "${API_GATEWAY_URL}/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    --data-binary "$body")"
  [[ "$http_code" == "200" ]] || fail "login failed for ${email} (HTTP ${http_code})"
}

run_seed_tests() {
  step "Phase 2 — canonical seed validation (fresh disposable DB)"
  trap stop_stack EXIT
  start_stack

  API_GATEWAY_URL="$API_GATEWAY_URL" \
  IDENTITY_SERVICE_URL="http://localhost:18081" \
  COMPANY_SERVICE_URL="http://localhost:18082" \
  POSTGRES_CONTAINER="$PG_CONTAINER" \
  bash scripts/dev/seed_dev_admin.sh

  API_GATEWAY_URL="$API_GATEWAY_URL" \
  IDENTITY_SERVICE_URL="http://localhost:18081" \
  COMPANY_SERVICE_URL="http://localhost:18082" \
  bash scripts/dev/seed_demo_data.sh || true

  API_GATEWAY_URL="$API_GATEWAY_URL" \
  IDENTITY_SERVICE_URL="http://localhost:18081" \
  COMPANY_SERVICE_URL="http://localhost:18082" \
  POSTGRES_CONTAINER="$PG_CONTAINER" \
  bash scripts/dev/seed_dev_admin.sh

  API_GATEWAY_URL="$API_GATEWAY_URL" \
  IDENTITY_SERVICE_URL="http://localhost:18081" \
  COMPANY_SERVICE_URL="http://localhost:18082" \
  bash scripts/dev/seed_demo_data.sh || true

  assert_count "admin@bintrans.local" 1
  assert_count "shipper@bintrans.local" 1
  assert_count "carrier@bintrans.local" 1
  assert_count "forwarder@bintrans.local" 1
  assert_count "consignee@bintrans.local" 1

  OLD_COUNT="$(docker exec -i "$PG_CONTAINER" psql -U freight -d freight_platform -t -A -c \
    "SELECT count(*) FROM core.users WHERE tenant_id='${TENANT_ID}'::uuid AND deleted_at IS NULL AND lower(email) LIKE '%@7rights.local';")"
  [[ "$OLD_COUNT" == "0" ]] || fail "expected 0 @7rights.local users, got ${OLD_COUNT}"

  TENANT_CODE="$(docker exec -i "$PG_CONTAINER" psql -U freight -d freight_platform -t -A -c \
    "SELECT code FROM core.tenants WHERE id='${TENANT_ID}'::uuid;")"
  [[ "$TENANT_CODE" == "dev-bintrans" ]] || fail "expected tenant code dev-bintrans, got ${TENANT_CODE}"

  login_check "admin@bintrans.local" "Admin123456!"
  login_check "shipper@bintrans.local" "Demo123456!"
  login_check "carrier@bintrans.local" "Demo123456!"
  login_check "forwarder@bintrans.local" "Demo123456!"
  login_check "consignee@bintrans.local" "Demo123456!"
  pass "seed + login validation"
}

run_migration_tests
run_seed_tests
stop_stack

echo ""
echo "A21_LOCAL_VALIDATION=PASS"
