#!/usr/bin/env bash
# Compares public Control Tower JSON between disabled and shadow gateway modes.
set -euo pipefail

GATEWAY_URL="${GATEWAY_URL:-http://127.0.0.1:8080}"
JWT_TOKEN="${JWT_TOKEN:-}"
TENANT_ID="${TENANT_ID:-}"
ADMIN_EMAIL="${ADMIN_EMAIL:-}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-}"
AUTH_ENABLED="${AUTH_ENABLED:-true}"
COMPOSE_FILE="${COMPOSE_FILE:-infrastructure/docker-compose/docker-compose.yml}"
COMPOSE_SHADOW="${COMPOSE_SHADOW:-infrastructure/docker-compose/docker-compose.staging-shadow.yml}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

fail() { echo "equivalence-check: $*" >&2; exit 1; }

login_jwt() {
  [[ -n "${JWT_TOKEN}" ]] && return 0
  [[ -n "${TENANT_ID}" && -n "${ADMIN_EMAIL}" && -n "${ADMIN_PASSWORD}" ]] \
    || fail "JWT or tenant admin credentials required"
  local tmp code
  tmp="$(mktemp "${TMPDIR:-/tmp}/ct-eq-login.XXXXXX")"
  code="$(curl -sS -o "${tmp}" -w '%{http_code}' -H 'Content-Type: application/json' \
    -d "{\"tenant_id\":\"${TENANT_ID}\",\"email\":\"${ADMIN_EMAIL}\",\"password\":\"${ADMIN_PASSWORD}\"}" \
    "${GATEWAY_URL}/api/v1/auth/login")"
  [[ "${code}" == "200" ]] || fail "login HTTP ${code}"
  JWT_TOKEN="$(jq -er '.access_token // empty' "${tmp}")"
  rm -f "${tmp}"
}

fetch_summary() {
  local out="$1"
  curl -fsS -H "Authorization: Bearer ${JWT_TOKEN}" \
    "${GATEWAY_URL}/api/v1/control-tower/summary" > "${out}"
}

normalize() {
  jq 'del(.generatedAt, .requestId)
     | walk(if type == "object" then del(.lastUpdatedAt, .occurredAt) else . end)' "$1"
}

redeploy_gateway() {
  local mode="$1"
  if [[ "${mode}" == "shadow" ]]; then
    AUTH_ENABLED=true docker compose -f "${ROOT}/${COMPOSE_FILE}" -f "${ROOT}/${COMPOSE_SHADOW}" \
      --profile messaging --profile read-model --profile observability \
      up -d --force-recreate api-gateway >/dev/null
  else
    AUTH_ENABLED=true docker compose -f "${ROOT}/${COMPOSE_FILE}" \
      --profile messaging --profile read-model --profile observability \
      up -d --force-recreate api-gateway >/dev/null
  fi
  sleep 20
}

login_jwt
disabled_tmp="$(mktemp)"
shadow_tmp="$(mktemp)"
norm_disabled="$(mktemp)"
norm_shadow="$(mktemp)"
trap 'rm -f "${disabled_tmp}" "${shadow_tmp}" "${norm_disabled}" "${norm_shadow}"; unset JWT_TOKEN ADMIN_PASSWORD' EXIT

redeploy_gateway disabled
fetch_summary "${disabled_tmp}"
redeploy_gateway shadow
fetch_summary "${shadow_tmp}"

normalize "${disabled_tmp}" > "${norm_disabled}"
normalize "${shadow_tmp}" > "${norm_shadow}"

if diff -u "${norm_disabled}" "${norm_shadow}" >/dev/null; then
  echo "control-tower-shadow-equivalence-check: OK"
else
  fail "disabled vs shadow semantic JSON mismatch"
fi
