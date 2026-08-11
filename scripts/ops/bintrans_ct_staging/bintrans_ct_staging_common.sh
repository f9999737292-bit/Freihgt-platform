#!/usr/bin/env bash
# Shared helpers for BINTRANS dedicated Control Tower staging operator scripts.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"

export BINTRANS_COMPOSE_PROJECT="${BINTRANS_COMPOSE_PROJECT:-bintrans-ct-staging}"
export BINTRANS_STAGING_ENV="${BINTRANS_STAGING_ENV:-/protected/bintrans/control-tower-observation/staging.env}"
export BINTRANS_COMPOSE_BASE="${ROOT}/infrastructure/docker-compose/docker-compose.yml"
export BINTRANS_COMPOSE_BINTRANS="${ROOT}/infrastructure/docker-compose/docker-compose.bintrans-ct-staging.yml"
export BINTRANS_COMPOSE_SHADOW="${ROOT}/infrastructure/docker-compose/docker-compose.staging-shadow.yml"
export BINTRANS_COMPOSE_IMAGES="${ROOT}/infrastructure/docker-compose/docker-compose.bintrans-ct-staging-images.yml"

bintrans_compose() {
  local -a files=(
    -f "${BINTRANS_COMPOSE_BASE}"
    -f "${BINTRANS_COMPOSE_BINTRANS}"
  )
  if [[ "${BINTRANS_INCLUDE_SHADOW:-0}" == "1" ]]; then
    files+=(-f "${BINTRANS_COMPOSE_SHADOW}")
  fi
  if [[ "${BINTRANS_INCLUDE_IMAGES:-0}" == "1" ]]; then
    files+=(-f "${BINTRANS_COMPOSE_IMAGES}")
  fi
  docker compose \
    --env-file "${BINTRANS_STAGING_ENV}" \
    -p "${BINTRANS_COMPOSE_PROJECT}" \
    "${files[@]}" \
    "$@"
}

bintrans_fail() {
  echo "bintrans-ct-staging: $*" >&2
  exit 1
}

bintrans_require_env_file() {
  [[ -f "${BINTRANS_STAGING_ENV}" ]] || bintrans_fail "protected env missing: ${BINTRANS_STAGING_ENV}"
}

bintrans_postgres_container() {
  bintrans_compose --profile messaging ps -q postgres 2>/dev/null | head -n1
}

bintrans_redpanda_container() {
  bintrans_compose --profile messaging ps -q redpanda 2>/dev/null | head -n1
}

bintrans_env_value() {
  grep -E "^${1}=" "${BINTRANS_STAGING_ENV}" | tail -n1 | cut -d= -f2- || true
}

# Parse golang-migrate "version" output that may include Docker Compose lifecycle noise.
# Prints "<version> <dirty>" (dirty: yes|no). Exit 0 ok, 1 unparseable, 2 conflicting.
bintrans_parse_migrate_version() {
  local output="$1"
  local -a versions=()
  local dirty=no line

  while IFS= read -r line; do
    line="$(printf '%s' "$line" | tr -d '\r' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    [[ -z "$line" ]] && continue
    if [[ "$line" =~ ^([0-9]+)[[:space:]]*\(dirty\)[[:space:]]*$ ]]; then
      versions+=("${BASH_REMATCH[1]}")
      dirty=yes
    elif [[ "$line" =~ ^([0-9]+)[[:space:]]*$ ]]; then
      versions+=("${BASH_REMATCH[1]}")
    fi
  done <<< "$(printf '%s\n' "$output")"

  if [[ ${#versions[@]} -eq 0 ]]; then
    if echo "$output" | grep -qi 'no migration'; then
      printf '0 no\n'
      return 0
    fi
    return 1
  fi

  local first="${versions[0]}"
  for v in "${versions[@]}"; do
    [[ "$v" == "$first" ]] || return 2
  done

  printf '%s %s\n' "$first" "$dirty"
  return 0
}

bintrans_jwt_secret_placeholder() {
  case "$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')" in
    dev_secret_change_me|changeme|change_me|replace_me|example|example_secret|secret|your_secret|your_password) return 0 ;;
  esac
  return 1
}

bintrans_require_nonplaceholder_jwt_secret() {
  local count val
  count="$(grep -cE '^[[:space:]]*JWT_SECRET=' "${BINTRANS_STAGING_ENV}" || true)"
  [[ "${count}" -eq 1 ]] || bintrans_fail "JWT_SECRET must appear exactly once in protected env"
  val="$(bintrans_env_value JWT_SECRET)"
  [[ -n "${val}" ]] || bintrans_fail "JWT_SECRET must be non-empty in protected env"
  if bintrans_jwt_secret_placeholder "${val}"; then
    bintrans_fail "JWT_SECRET must not use an obvious placeholder value"
  fi
}

bintrans_digest_image_pattern='^cr\.selcloud\.ru/bintrans-staging/[a-z0-9-]+@sha256:[0-9a-f]{64}$'

bintrans_runtime_image_vars=(
  BINTRANS_IDENTITY_IMAGE
  BINTRANS_COMPANY_IMAGE
  BINTRANS_TRANSPORT_ORDER_IMAGE
  BINTRANS_RFX_IMAGE
  BINTRANS_SHIPMENT_IMAGE
  BINTRANS_DOCUMENT_IMAGE
  BINTRANS_BILLING_REGISTER_IMAGE
  BINTRANS_LOW_CODE_IMAGE
  BINTRANS_CONTROL_TOWER_READ_MODEL_IMAGE
  BINTRANS_API_GATEWAY_IMAGE
)

bintrans_extract_gateway_mode() {
  awk '
    /^  api-gateway:/ { in_gw=1; next }
    in_gw && /^  [a-zA-Z0-9_-]+:/ { exit }
    in_gw && $1 == "CONTROL_TOWER_READ_MODEL_MODE:" { print $2; exit }
  ' "$1"
}
