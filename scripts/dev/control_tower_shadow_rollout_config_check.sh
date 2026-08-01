#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
COMPOSE_BASE="${ROOT}/infrastructure/docker-compose/docker-compose.yml"
COMPOSE_SHADOW="${ROOT}/infrastructure/docker-compose/docker-compose.staging-shadow.yml"

fail() {
  echo "config-check: $*" >&2
  exit 1
}

[[ -f "${COMPOSE_BASE}" ]] || fail "missing base compose file"
[[ -f "${COMPOSE_SHADOW}" ]] || fail "missing staging shadow compose file"

merged="$(docker compose -f "${COMPOSE_BASE}" -f "${COMPOSE_SHADOW}" --profile read-model --profile messaging config 2>/dev/null)" \
  || fail "docker compose config failed"

echo "${merged}" | grep -q 'CONTROL_TOWER_READ_MODEL_MODE: shadow' \
  || fail "staging override must set gateway mode=shadow"

base_only="$(docker compose -f "${COMPOSE_BASE}" config 2>/dev/null)"
echo "${base_only}" | grep -q 'CONTROL_TOWER_READ_MODEL_MODE: disabled' \
  || fail "base compose default mode must remain disabled"

echo "${merged}" | grep -q 'CONTROL_TOWER_CONSUMER_ENABLED: "true"' \
  || fail "staging override must enable read-model consumer"
echo "${merged}" | grep -q 'CONTROL_TOWER_KAFKA_GROUP_ID: control-tower-shipment-status-v1' \
  || fail "consumer group must remain stable"
echo "${merged}" | grep -q 'CONTROL_TOWER_KAFKA_TOPIC: shipment.status.v1' \
  || fail "kafka topic must remain shipment.status.v1"
echo "${merged}" | grep -q 'CONTROL_TOWER_READ_MODEL_BASE_URL: http://control-tower-read-model-service:8089' \
  || fail "internal read-model URL missing"

if echo "${merged}" | grep -qiE 'password=|sasl.*password|BEGIN PRIVATE KEY'; then
  fail "compose config must not embed credentials"
fi

if echo "${merged}" | grep -q 'CONTROL_TOWER_READ_MODEL_MODE: primary'; then
  fail "primary mode must not appear in shadow staging config"
fi

if echo "${merged}" | grep -A2 'api-gateway:' | grep -q 'control-tower-read-model-service'; then
  fail "gateway must not hard-depend on read-model in depends_on"
fi

echo "control-tower-shadow-rollout-config-check: OK"
