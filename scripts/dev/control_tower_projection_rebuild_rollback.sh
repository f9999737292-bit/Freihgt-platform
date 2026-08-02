#!/usr/bin/env bash
set -euo pipefail

if [[ "${CONFIRM_PROJECTION_REBUILD_ROLLBACK:-}" != "true" ]]; then
  echo "ROLLBACK_CONFIRMATION_REQUIRED" >&2
  exit 2
fi

: "${SNAPSHOT_ID:?SNAPSHOT_ID is required}"
: "${COMPOSE:=docker compose -f infrastructure/docker-compose/docker-compose.yml -f infrastructure/docker-compose/docker-compose.staging-shadow.yml}"

${COMPOSE} exec -T control-tower-read-model-service \
  env CONFIRM_PROJECTION_REBUILD_ROLLBACK=true \
  /app/control-tower-status-snapshot-import --rollback --snapshot-id "${SNAPSHOT_ID}"
