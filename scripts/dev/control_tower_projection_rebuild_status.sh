#!/usr/bin/env bash
set -euo pipefail

: "${SNAPSHOT_ID:?SNAPSHOT_ID is required}"
: "${COMPOSE:=docker compose -f infrastructure/docker-compose/docker-compose.yml -f infrastructure/docker-compose/docker-compose.staging-shadow.yml}"

${COMPOSE} exec -T control-tower-read-model-service \
  /app/control-tower-status-snapshot-import --status --snapshot-id "${SNAPSHOT_ID}"
