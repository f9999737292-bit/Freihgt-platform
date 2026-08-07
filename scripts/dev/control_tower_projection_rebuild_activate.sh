#!/usr/bin/env bash
set -euo pipefail

if [[ "${CONFIRM_PROJECTION_REBUILD_ACTIVATION:-}" != "true" ]]; then
  echo "ACTIVATION_CONFIRMATION_REQUIRED" >&2
  exit 2
fi

: "${SNAPSHOT_ID:?SNAPSHOT_ID is required}"
: "${COMPOSE:=docker compose -f infrastructure/docker-compose/docker-compose.yml -f infrastructure/docker-compose/docker-compose.staging-shadow.yml}"

${COMPOSE} exec -T control-tower-read-model-service \
  env CONFIRM_PROJECTION_REBUILD_ACTIVATION=true \
  /app/control-tower-status-snapshot-import --activate --snapshot-id "${SNAPSHOT_ID}"
