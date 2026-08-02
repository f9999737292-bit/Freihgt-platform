#!/usr/bin/env bash
# Consumer restart drill: pause → backlog → resume same group → lag=0.
set -euo pipefail
export MSYS_NO_PATHCONV=1
export MSYS2_ARG_CONV_EXCL="*"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

COMPOSE_FILE_ARGS="-f infrastructure/docker-compose/docker-compose.yml -f infrastructure/docker-compose/docker-compose.staging-shadow.yml -f infrastructure/docker-compose/docker-compose.rebuild-acceptance.yml"
KAFKA_TOPIC="${CONTROL_TOWER_KAFKA_TOPIC:-shipment.status.v1}"
KAFKA_GROUP="${CONTROL_TOWER_KAFKA_GROUP_ID:-control-tower-shipment-status-v1}"

dc() {
  docker compose ${COMPOSE_FILE_ARGS} --profile messaging --profile read-model --profile observability "$@"
}

fail() { echo "consumer-restart-drill: $*" >&2; exit 1; }

kafka_lines() {
  dc exec -T redpanda rpk group describe "${KAFKA_GROUP}" 2>/dev/null \
    | awk -v topic="${KAFKA_TOPIC}" '$1 == topic && $2 ~ /^[0-9]+$/ {print $2, $3, $5, $6}'
}

echo "==> Capture offsets before pause"
kafka_lines

echo "==> Pause consumer"
CONTROL_TOWER_CONSUMER_ENABLED=false dc up -d --no-deps --force-recreate control-tower-read-model-service >/dev/null
sleep 10

echo "==> During pause (allow bounded backlog)"
kafka_lines

echo "==> Resume same consumer group"
CONTROL_TOWER_CONSUMER_ENABLED=true dc up -d --no-deps --force-recreate control-tower-read-model-service >/dev/null

deadline=$((SECONDS + 300))
while (( SECONDS < deadline )); do
  lag_sum="$(kafka_lines | awk '{s+=$4} END {print s+0}')"
  if [[ "${lag_sum}" == "0" ]]; then
    echo "==> After catch-up"
    kafka_lines
    echo "control-tower-shadow-observation-consumer-restart-drill: OK"
    exit 0
  fi
  sleep 5
done
fail "lag did not return to zero within timeout"
