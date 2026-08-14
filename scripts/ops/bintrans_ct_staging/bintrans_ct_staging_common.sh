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
