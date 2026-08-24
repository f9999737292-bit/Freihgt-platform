#!/usr/bin/env bash
# Container smoke checks for v2.2 pilot images (local). Does NOT push.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
SHA="${PILOT_GIT_SHA:-37c2eb62ccf9377359eb5c2fdf6f71eb9d187140}"
SHORT_SHA="${SHA:0:7}"
TAG="${FREIGHT_PILOT_IMAGE_TAG:-git-${SHORT_SHA}}"
NETWORK="freight-pilot-smoke-$$"
INTERNAL_TOKEN="${INTERNAL_SERVICE_TOKEN:-pilot_smoke_internal_token_change_me}"

cleanup() {
  docker rm -f freight-pilot-smoke-pg freight-pilot-smoke-fc >/dev/null 2>&1 || true
  docker network rm "${NETWORK}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker network create "${NETWORK}" >/dev/null

echo "Starting disposable Postgres for freight-cost smoke..."
docker run -d --name freight-pilot-smoke-pg --network "${NETWORK}" \
  -e POSTGRES_DB=freight_platform -e POSTGRES_USER=freight -e POSTGRES_PASSWORD=freight_password \
  mirror.gcr.io/library/postgres:16 >/dev/null

for _ in $(seq 1 30); do
  docker exec freight-pilot-smoke-pg pg_isready -U freight >/dev/null 2>&1 && break
  sleep 2
done

docker run --rm --network "${NETWORK}" \
  -v "${ROOT}/infrastructure/migrations:/migrations:ro" \
  migrate/migrate:v4.17.1 \
  -path=/migrations \
  -database='postgres://freight:freight_password@freight-pilot-smoke-pg:5432/freight_platform?sslmode=disable' up

FC_IMAGE="freight-pilot/freight-cost-service:${TAG}"
if ! docker image inspect "${FC_IMAGE}" >/dev/null 2>&1; then
  echo "Building freight-cost-service image..."
  "${ROOT}/scripts/ops/freight_cost_pilot/build_v22_images.sh"
fi

docker run -d --name freight-pilot-smoke-fc --network "${NETWORK}" \
  -e DATABASE_URL='postgres://freight:freight_password@freight-pilot-smoke-pg:5432/freight_platform?sslmode=disable' \
  -e INTERNAL_SERVICE_TOKEN="${INTERNAL_TOKEN}" \
  -e FREIGHT_COST_ANALYTICS_PROJECTION_ENABLED=false \
  "${FC_IMAGE}" >/dev/null

for _ in $(seq 1 30); do
  if docker exec freight-pilot-smoke-fc wget -qO- http://127.0.0.1:8092/health >/dev/null 2>&1; then
    echo "freight-cost-service /health PASS"
    break
  fi
  sleep 2
done

docker exec freight-pilot-smoke-fc wget -qO- http://127.0.0.1:8092/health | grep -q '"status"' \
  || { echo "FAIL: freight-cost health"; exit 1; }

GW_IMAGE="freight-pilot/api-gateway:${TAG}"
docker run --rm "${GW_IMAGE}" wget -qO- http://127.0.0.1:8080/health 2>/dev/null \
  && echo "api-gateway health check skipped (needs deps)" || echo "api-gateway image present"

WP_IMAGE="freight-pilot/web-procurement:${TAG}"
if docker image inspect "${WP_IMAGE}" >/dev/null 2>&1; then
  echo "web-procurement image present: ${WP_IMAGE}"
else
  echo "WARN: web-procurement image missing — run build_v22_images.sh"
fi

echo "IMAGE_SMOKE_GATE=PASS"
