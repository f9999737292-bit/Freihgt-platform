#!/usr/bin/env bash
# BINTRANS dedicated staging — read-only foundation health checks.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

bintrans_require_env_file

POSTGRES_USER="$(grep -E '^POSTGRES_USER=' "${BINTRANS_STAGING_ENV}" | tail -n1 | cut -d= -f2-)"
POSTGRES_DB="$(grep -E '^POSTGRES_DB=' "${BINTRANS_STAGING_ENV}" | tail -n1 | cut -d= -f2-)"
[[ -n "${POSTGRES_USER}" && -n "${POSTGRES_DB}" ]] || bintrans_fail "POSTGRES_USER and POSTGRES_DB required in protected env"

echo "=== docker compose ps (foundation) ==="
BINTRANS_INCLUDE_SHADOW=0 BINTRANS_INCLUDE_IMAGES=0 \
  bintrans_compose --profile messaging ps

echo
echo "=== docker ps (project ${BINTRANS_COMPOSE_PROJECT}) ==="
docker ps --filter "label=com.docker.compose.project=${BINTRANS_COMPOSE_PROJECT}" \
  --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'

pg_cid="$(bintrans_postgres_container)"
[[ -n "${pg_cid}" ]] || bintrans_fail "postgres container not running"

echo
echo "=== PostgreSQL pg_isready ==="
docker exec "${pg_cid}" pg_isready -U "${POSTGRES_USER}" -d "${POSTGRES_DB}"

rp_cid="$(bintrans_redpanda_container)"
[[ -n "${rp_cid}" ]] || bintrans_fail "redpanda container not running"

echo
echo "=== Redpanda cluster info ==="
docker exec "${rp_cid}" rpk cluster info --brokers localhost:9092

echo
echo "bintrans-ct-staging-foundation-health: PASS"
