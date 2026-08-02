#!/usr/bin/env bash
# Live projection rebuild acceptance through real runtime boundaries.
# Does not print JWT, tenant UUIDs, or persist snapshots.
set -euo pipefail

# Git Bash on Windows converts /app/... docker paths unless disabled.
export MSYS_NO_PATHCONV=1
export MSYS2_ARG_CONV_EXCL="*"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

COMPOSE_FILE_ARGS="-f infrastructure/docker-compose/docker-compose.yml -f infrastructure/docker-compose/docker-compose.staging-shadow.yml -f infrastructure/docker-compose/docker-compose.rebuild-acceptance.yml"

dc() {
  docker compose ${COMPOSE_FILE_ARGS} --profile messaging --profile read-model --profile observability "$@"
}

GATEWAY_URL="${GATEWAY_URL:-http://127.0.0.1:8080}"
READ_MODEL_URL="${READ_MODEL_URL:-http://127.0.0.1:8089}"
SHIPMENT_URL="${SHIPMENT_URL:-http://127.0.0.1:8085}"
KAFKA_TOPIC="${KAFKA_TOPIC:-shipment.status.v1}"
KAFKA_GROUP="${KAFKA_GROUP:-control-tower-shipment-status-v1}"
POLL_TIMEOUT_SEC="${POLL_TIMEOUT_SEC:-180}"

JWT_TOKEN=""
TENANT_A=""
TENANT_B=""
SNAPSHOT_A=""
SNAPSHOT_B=""
ADMIN_EMAIL_A=""
ADMIN_PASSWORD_A=""
ADMIN_EMAIL_B=""
ADMIN_PASSWORD_B=""

fail() { echo "live-rebuild-acceptance: $*" >&2; exit 1; }
step() { echo "==> $*" >&2; }
require_cmd() { command -v "$1" >/dev/null 2>&1 || fail "$1 is required"; }

cleanup_secrets() {
  unset JWT_TOKEN ADMIN_PASSWORD_A ADMIN_PASSWORD_B CONFIRM_PROJECTION_REBUILD_IMPORT \
    CONFIRM_PROJECTION_REBUILD_ACTIVATION CONFIRM_PROJECTION_REBUILD_ROLLBACK \
    CONFIRM_PROJECTION_REBUILD_CLEANUP
}
trap cleanup_secrets EXIT

metric_sum() {
  local metrics
  metrics="$(curl -fsS "${GATEWAY_URL}/metrics")"
  echo "${metrics}" | grep -E "^${2}" | awk '{sum+=$2} END {print sum+0}' || true
}

metric_label() {
  local metrics
  metrics="$(curl -fsS "${GATEWAY_URL}/metrics")"
  echo "${metrics}" | grep -E "^${2}" | grep "${3}" | awk '{sum+=$2} END {print sum+0}' || true
}

set_consumer() {
  local enabled="$1"
  step "Set consumer enabled=${enabled}"
  CONTROL_TOWER_CONSUMER_ENABLED="${enabled}" dc up -d --no-deps --force-recreate control-tower-read-model-service >/dev/null
  local deadline=$((SECONDS + 60))
  while (( SECONDS < deadline )); do
    if curl -fsS "${READ_MODEL_URL}/health" >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  fail "read-model service did not become healthy"
}

parse_curl_http() {
  HTTP_CODE="${1##*__HTTP_CODE__:}"
  HTTP_BODY="${1%__HTTP_CODE__:*}"
}

login_jwt() {
  local tenant_id="$1" email="$2" password="$3"
  require_cmd jq
  local raw
  raw="$(curl -sS -w '__HTTP_CODE__:%{http_code}' \
    -H 'Content-Type: application/json' \
    -d "{\"tenant_id\":\"${tenant_id}\",\"email\":\"${email}\",\"password\":\"${password}\"}" \
    "${GATEWAY_URL}/api/v1/auth/login")"
  parse_curl_http "${raw}"
  [[ "${HTTP_CODE}" == "200" ]] || fail "login failed HTTP ${HTTP_CODE}"
  JWT_TOKEN="$(printf '%s' "${HTTP_BODY}" | jq -er '.access_token // empty')"
  [[ -n "${JWT_TOKEN}" ]] || fail "login missing access_token"
}

gateway_summary_assert() {
  local expect_match="$1"
  local before_match="${2:-0}"
  local raw
  raw="$(curl -sS -w '__HTTP_CODE__:%{http_code}' \
    -H "Authorization: Bearer ${JWT_TOKEN}" \
    "${GATEWAY_URL}/api/v1/control-tower/summary")"
  parse_curl_http "${raw}"
  [[ "${HTTP_CODE}" == "200" ]] || fail "summary HTTP ${HTTP_CODE}"
  printf '%s' "${HTTP_BODY}" | jq -e '.statusSummary.source == "LEGACY"' >/dev/null || fail "public source must be LEGACY"
  printf '%s' "${HTTP_BODY}" | jq -e '(.statusSummary.limitedDataset // false) == false' >/dev/null || fail "limitedDataset must be false"
  if [[ "${expect_match}" == "true" ]]; then
    printf '%s' "${HTTP_BODY}" | jq -e '(.statusSummaryFreshness.fallbackUsed // false) == false' >/dev/null \
      || fail "fallback must not be used post-rebuild"
  fi

  local after_match
  after_match="$(metric_label "${GATEWAY_URL}" 'control_tower_read_model_shadow_comparison_total' 'comparison="MATCH"')"
  if [[ "${expect_match}" == "true" ]]; then
    (( after_match > before_match )) || fail "MATCH metric did not increase"
    echo "live-rebuild-acceptance: post-rebuild comparison=MATCH" >&2
  else
    (( after_match == before_match )) || echo "live-rebuild-acceptance: pre-rebuild comparison != MATCH (match_total unchanged)" >&2
  fi
}

create_rebuild_tenant() {
  local label="$1"
  step "Create isolated tenant ${label} (consumer paused)"
  set_consumer false
  # Reuse shadow acceptance fixture with unique tenant; distribution 2/2/1 by default.
  eval "$("${ROOT}/scripts/dev/control_tower_shadow_rollout_acceptance_fixture.sh")"
  if [[ "${label}" == "A" ]]; then
    TENANT_A="${TENANT_ID}"
    ADMIN_EMAIL_A="${ADMIN_EMAIL}"
    ADMIN_PASSWORD_A="${ADMIN_PASSWORD}"
  else
    TENANT_B="${TENANT_ID}"
    ADMIN_EMAIL_B="${ADMIN_EMAIL}"
    ADMIN_PASSWORD_B="${ADMIN_PASSWORD}"
  fi
}

projection_count() {
  local tenant_id="$1"
  dc exec -T postgres psql -U freight -d freight_platform -tAc \
    "SELECT COUNT(*) FROM control_tower.shipment_status_projection WHERE tenant_id='${tenant_id}'" \
    | tr -d '[:space:]'
}

export_import_activate() {
  local tenant_id="$1"
  local var_snapshot="$2"
  step "Export → import → activate for tenant scope"
  local stream_file snapshot_id import_out activate_out export_err
  stream_file="$(mktemp /tmp/ct-rebuild-export.XXXXXX)"
  export_err="$(mktemp /tmp/ct-rebuild-export-err.XXXXXX)"
  dc exec -T shipment-service \
    /app/shipment-status-snapshot-export --tenant "${tenant_id}" --output - \
    >"${stream_file}" 2>"${export_err}" \
    || fail "exporter failed: $(tr -d '\r' < "${export_err}")"
  rm -f "${export_err}"
  [[ -s "${stream_file}" ]] || fail "exporter produced empty stream"
  snapshot_id="$(head -1 "${stream_file}" | jq -er '.snapshotId')" \
    || fail "could not parse snapshotId from export manifest"
  printf '%s\n' "live-rebuild-acceptance: import snapshot job created" >&2

  CONFIRM_PROJECTION_REBUILD_IMPORT=true
  import_out="$(cat "${stream_file}" | dc exec -T -i \
    -e CONFIRM_PROJECTION_REBUILD_IMPORT=true control-tower-read-model-service \
    /app/control-tower-status-snapshot-import --stdin 2>&1)" \
    || fail "import command failed: ${import_out}"
  rm -f "${stream_file}"
  unset CONFIRM_PROJECTION_REBUILD_IMPORT
  printf '%s\n' "${import_out}" | grep -qE '"state"[[:space:]]*:[[:space:]]*"VALIDATED"' \
    || fail "import not VALIDATED: ${import_out}"

  CONFIRM_PROJECTION_REBUILD_ACTIVATION=true
  activate_out="$(dc exec -T \
    -e CONFIRM_PROJECTION_REBUILD_ACTIVATION=true control-tower-read-model-service \
    /app/control-tower-status-snapshot-import --activate --snapshot-id "${snapshot_id}" 2>&1)" \
    || fail "activation command failed"
  unset CONFIRM_PROJECTION_REBUILD_ACTIVATION
  echo "${activate_out}" | grep -qE '"state"[[:space:]]*:[[:space:]]*"ACTIVE"' || fail "activation not ACTIVE: ${activate_out}"
  echo "${activate_out}" | grep -qE '"rollbackEligible"[[:space:]]*:[[:space:]]*true' || fail "rollbackEligible must be true"

  printf -v "${var_snapshot}" '%s' "${snapshot_id}"
}

kafka_group_offset() {
  dc exec -T redpanda rpk group describe "${KAFKA_GROUP}" -o json 2>/dev/null \
    | jq -r --arg topic "${KAFKA_TOPIC}" '
      [.members[]?.assignments[]? | select(.topic==$topic) | .offset] | add // 0'
}

wait_catchup() {
  local tenant_id="$1" expected="$2"
  local deadline=$((SECONDS + POLL_TIMEOUT_SEC))
  while (( SECONDS < deadline )); do
    local count
    count="$(projection_count "${tenant_id}")"
    if [[ "${count}" == "${expected}" ]]; then
      return 0
    fi
    sleep 2
  done
  fail "catch-up did not reach ${expected} projection rows"
}

patch_tenant_shipment_status() {
  local tenant_id="$1" email="$2" password="$3"
  login_jwt "${tenant_id}" "${email}" "${password}"
  local ship_id current_status next_status
  ship_id="$(dc exec -T postgres psql -U freight -d freight_platform -tAc \
    "SELECT shipment_id FROM control_tower.shipment_status_projection WHERE tenant_id='${tenant_id}' AND current_status='IN_TRANSIT' LIMIT 1" \
    | tr -d '[:space:]')"
  next_status="ARRIVED_AT_CONSIGNEE"
  if [[ -z "${ship_id}" ]]; then
    ship_id="$(dc exec -T postgres psql -U freight -d freight_platform -tAc \
      "SELECT shipment_id FROM control_tower.shipment_status_projection WHERE tenant_id='${tenant_id}' AND current_status='CARRIER_ASSIGNED' LIMIT 1" \
      | tr -d '[:space:]')"
    next_status="ACCEPTED_BY_CARRIER"
  fi
  [[ -n "${ship_id}" ]] || fail "no projection row eligible for live transition"
  current_status="$(dc exec -T postgres psql -U freight -d freight_platform -tAc \
    "SELECT current_status FROM control_tower.shipment_status_projection WHERE tenant_id='${tenant_id}' AND shipment_id='${ship_id}'" \
    | tr -d '[:space:]')"
  local user_id raw
  user_id="$(curl -fsS "${IDENTITY_SERVICE_URL:-http://127.0.0.1:8081}/v1/users?tenant_id=${tenant_id}&limit=100&offset=0" \
    | jq -r --arg email "${email}" '(.items // [])[] | select(.email==$email) | .id' | head -n1)"
  raw="$(curl -sS -w '__HTTP_CODE__:%{http_code}' -X PATCH \
    -H "Content-Type: application/json" \
    -H "X-Tenant-ID: ${tenant_id}" -H "X-User-ID: ${user_id}" \
    -d "{\"status\":\"${next_status}\"}" \
    "${SHIPMENT_URL}/v1/shipments/${ship_id}/status")"
  parse_curl_http "${raw}"
  [[ "${HTTP_CODE}" == "200" || "${HTTP_CODE}" == "204" ]] \
    || fail "live status patch failed HTTP ${HTTP_CODE} (${current_status}->${next_status})"
  echo "live-rebuild-acceptance: live event applied (${current_status}->${next_status})" >&2
  sleep 5
}

run_tenant_a_historical() {
  step "Tenant A historical rebuild acceptance"
  create_rebuild_tenant A
  step "Tenant A verify empty projection"
  local pre_count
  pre_count="$(projection_count "${TENANT_A}")"
  [[ "${pre_count}" == "0" ]] || fail "tenant A projection must be empty before rebuild (got ${pre_count})"

  step "Tenant A login for pre-rebuild summary"
  login_jwt "${TENANT_A}" "${ADMIN_EMAIL_A}" "${ADMIN_PASSWORD_A}"
  local before_match
  before_match="$(metric_label "${GATEWAY_URL}" 'control_tower_read_model_shadow_comparison_total' 'comparison="MATCH"')"
  step "Tenant A pre-rebuild gateway summary"
  gateway_summary_assert false "${before_match}"

  local offset_before
  offset_before="$(kafka_group_offset || echo 0)"
  echo "live-rebuild-acceptance: topic=${KAFKA_TOPIC} group=${KAFKA_GROUP} offset_before=${offset_before} consumerRunning=false" >&2

  export_import_activate "${TENANT_A}" SNAPSHOT_A

  local offset_after_activation
  offset_after_activation="$(kafka_group_offset || echo 0)"
  [[ "${offset_after_activation}" == "${offset_before}" ]] || fail "offsets changed during activation"

  set_consumer true
  wait_catchup "${TENANT_A}" 5

  local offset_after_catchup
  offset_after_catchup="$(kafka_group_offset || echo 0)"
  echo "live-rebuild-acceptance: offset_after_catchup=${offset_after_catchup} same_group=${KAFKA_GROUP}" >&2

  login_jwt "${TENANT_A}" "${ADMIN_EMAIL_A}" "${ADMIN_PASSWORD_A}"
  gateway_summary_assert true "${before_match}"

  step "Apply live event N+1 after catch-up"
  patch_tenant_shipment_status "${TENANT_A}" "${ADMIN_EMAIL_A}" "${ADMIN_PASSWORD_A}"
  sleep 5

  step "Rollback must be refused after live write"
  CONFIRM_PROJECTION_REBUILD_ROLLBACK=true
  local rb_out
  rb_out="$(dc exec -T \
    -e CONFIRM_PROJECTION_REBUILD_ROLLBACK=true control-tower-read-model-service \
    /app/control-tower-status-snapshot-import --rollback --snapshot-id "${SNAPSHOT_A}" 2>&1 || true)"
  unset CONFIRM_PROJECTION_REBUILD_ROLLBACK
  echo "${rb_out}" | grep -q 'ROLLBACK_WINDOW_CLOSED' || fail "expected ROLLBACK_WINDOW_CLOSED after live event"
}

run_tenant_b_rollback() {
  step "Tenant B operational rollback acceptance"
  create_rebuild_tenant B

  set_consumer true
  sleep 15
  local old_count
  old_count="$(projection_count "${TENANT_B}")"
  [[ "${old_count}" == "5" ]] || fail "tenant B baseline projection not populated"

  set_consumer false
  export_import_activate "${TENANT_B}" SNAPSHOT_B

  CONFIRM_PROJECTION_REBUILD_ROLLBACK=true
  local rb_out
  rb_out="$(dc exec -T \
    -e CONFIRM_PROJECTION_REBUILD_ROLLBACK=true control-tower-read-model-service \
    /app/control-tower-status-snapshot-import --rollback --snapshot-id "${SNAPSHOT_B}" 2>&1)"
  unset CONFIRM_PROJECTION_REBUILD_ROLLBACK
  echo "${rb_out}" | grep -qE '"state"[[:space:]]*:[[:space:]]*"ROLLED_BACK"' || fail "rollback not ROLLED_BACK"

  local restored
  restored="$(projection_count "${TENANT_B}")"
  [[ "${restored}" == "${old_count}" ]] || fail "rollback row count mismatch"

  rb_out="$(dc exec -T \
    -e CONFIRM_PROJECTION_REBUILD_ROLLBACK=true control-tower-read-model-service \
    /app/control-tower-status-snapshot-import --rollback --snapshot-id "${SNAPSHOT_B}" 2>&1 || true)"
  echo "${rb_out}" | grep -q 'SNAPSHOT_ALREADY_ROLLED_BACK' || fail "repeat rollback must be idempotent"

  CONFIRM_PROJECTION_REBUILD_CLEANUP=true
  local clean_out
  clean_out="$(dc exec -T \
    -e CONFIRM_PROJECTION_REBUILD_CLEANUP=true control-tower-read-model-service \
    /app/control-tower-status-snapshot-import --cleanup --snapshot-id "${SNAPSHOT_B}" 2>&1)"
  unset CONFIRM_PROJECTION_REBUILD_CLEANUP
  echo "${clean_out}" | grep -qE '"state"[[:space:]]*:[[:space:]]*"CLEANED"' || fail "cleanup not CLEANED"

  set_consumer true
  echo "live-rebuild-acceptance: tenant B rollback acceptance OK" >&2
}

main() {
  require_cmd curl
  require_cmd jq
  require_cmd docker

  step "Verify shadow stack health"
  curl -fsS "${GATEWAY_URL}/health" >/dev/null || fail "api-gateway unhealthy"
  curl -fsS "${READ_MODEL_URL}/health" >/dev/null || fail "read-model unhealthy (start shadow stack first)"
  local rebuild_tables
  rebuild_tables="$(dc exec -T postgres psql -U freight -d freight_platform -tAc \
    "SELECT COUNT(*) FROM pg_tables WHERE schemaname='control_tower' AND tablename='shipment_status_projection_rebuild_job'" \
    | tr -d '[:space:]')"
  [[ "${rebuild_tables}" == "1" ]] || fail "projection rebuild migrations missing (run: make migrate-up)"

  if [[ "${ROLLBACK_ONLY:-}" == "1" ]]; then
    run_tenant_b_rollback
    echo "control-tower-projection-rebuild-rollback-acceptance: OK"
    return 0
  fi

  run_tenant_a_historical
  run_tenant_b_rollback
  echo "control-tower-projection-rebuild-live-acceptance: OK"
}

main "$@"
