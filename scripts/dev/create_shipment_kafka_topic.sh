#!/usr/bin/env bash
set -euo pipefail

TOPIC="${SHIPMENT_KAFKA_TOPIC:-shipment.status.v1}"
PARTITIONS="${SHIPMENT_KAFKA_TOPIC_PARTITIONS:-3}"
REPLICATION="${SHIPMENT_KAFKA_TOPIC_REPLICATION:-1}"
CONTAINER="${REDPANDA_CONTAINER:-freight_redpanda}"
MAX_ATTEMPTS="${SHIPMENT_KAFKA_TOPIC_CREATE_ATTEMPTS:-30}"
SLEEP_SECONDS="${SHIPMENT_KAFKA_TOPIC_CREATE_SLEEP:-2}"

if ! docker ps --format '{{.Names}}' | grep -qx "$CONTAINER"; then
  echo "Redpanda container $CONTAINER is not running. Start it with: make messaging-up" >&2
  exit 1
fi

attempt=1
while [ "$attempt" -le "$MAX_ATTEMPTS" ]; do
  output="$(docker exec "$CONTAINER" rpk topic create "$TOPIC" \
    --brokers localhost:9092 \
    --partitions "$PARTITIONS" \
    --replicas "$REPLICATION" \
    --topic-config cleanup.policy=delete 2>&1 || true)"
  echo "$output"
  if echo "$output" | grep -qi "TOPIC_ALREADY_EXISTS"; then
    echo "Topic $TOPIC already exists"
    exit 0
  fi
  if echo "$output" | grep -qi "created topic"; then
    echo "Topic $TOPIC created"
    exit 0
  fi
  echo "Broker not ready (attempt $attempt/$MAX_ATTEMPTS); retrying in ${SLEEP_SECONDS}s..."
  attempt=$((attempt + 1))
  sleep "$SLEEP_SECONDS"
done

echo "Failed to create topic $TOPIC" >&2
exit 1
