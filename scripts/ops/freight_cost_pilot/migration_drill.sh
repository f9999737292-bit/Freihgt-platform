#!/usr/bin/env bash
# Disposable Postgres migration drill for Freight Cost Intelligence v2.2 (000001–000064).
# NON-PRODUCTION only. Does not touch CT VM or production databases.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
COMPOSE_DIR="${ROOT}/infrastructure/docker-compose"
PROJECT="${FREIGHT_PILOT_DRILL_PROJECT:-freight-pilot-migration-drill}"
POSTGRES_PORT="${FREIGHT_PILOT_DRILL_PG_PORT:-55432}"
DB_URL="postgres://freight:freight_password@127.0.0.1:${POSTGRES_PORT}/freight_platform?sslmode=disable"

cleanup() {
  (cd "${COMPOSE_DIR}" && docker compose -p "${PROJECT}" down -v --remove-orphans) >/dev/null 2>&1 || true
}

trap cleanup EXIT

echo "=== Freight Cost v2.2 migration drill (disposable Postgres :${POSTGRES_PORT}) ==="

cd "${COMPOSE_DIR}"
export POSTGRES_PORT
docker compose -p "${PROJECT}" up -d postgres
echo "Waiting for Postgres..."
for _ in $(seq 1 30); do
  if docker compose -p "${PROJECT}" exec -T postgres pg_isready -U freight -d freight_platform >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

echo "Applying migrations (up)..."
docker compose -p "${PROJECT}" run --rm migrate up

echo "Verifying schema version..."
VERSION_OUT="$(docker compose -p "${PROJECT}" run --rm migrate version 2>&1 || true)"
echo "${VERSION_OUT}"

if ! echo "${VERSION_OUT}" | grep -qE '(^|[^0-9])64([^0-9]|$)'; then
  echo "FAIL: expected migration version 64" >&2
  exit 1
fi

if echo "${VERSION_OUT}" | grep -qi 'dirty'; then
  echo "FAIL: schema dirty flag detected" >&2
  exit 1
fi

echo "Smoke: freight_cost analytics tables exist..."
docker compose -p "${PROJECT}" exec -T postgres psql -U freight -d freight_platform -c \
  "SELECT to_regclass('freight_cost.analytics_order_fact') IS NOT NULL AS order_fact,
          to_regclass('freight_cost.analytics_lane_period') IS NOT NULL AS lane_period,
          to_regclass('freight_cost.analytics_benchmark_lane') IS NOT NULL AS benchmark,
          to_regclass('freight_cost.analytics_opportunity') IS NOT NULL AS opportunity;"

echo "DISPOSABLE_MIGRATION_DRILL=PASS"
echo "SCHEMA_AFTER=64"
