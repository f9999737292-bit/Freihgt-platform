# Control Tower Read Model Service

Internal microservice that consumes canonical `shipment.status.v1` Kafka events and maintains a PostgreSQL read-model for Control Tower.

## Responsibilities

- Consume shipment status events from Kafka (`control-tower-shipment-status-v1` consumer group)
- Idempotent projection via `control_tower.shipment_status_event_inbox`
- Expose tenant-scoped internal read API (`X-Tenant-ID` only)
- Surface version gaps and incomplete projections explicitly

## Non-goals (v0.1)

- Does not mutate `transport.shipments` or `shipment_status_history`
- Does not publish Kafka events
- Does not replace shipment-service or API Gateway Control Tower BFF

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `CONTROL_TOWER_READ_MODEL_SERVICE_PORT` / `CONTROL_TOWER_HTTP_PORT` | `8089` | HTTP port |
| `CONTROL_TOWER_DATABASE_URL` | — | PostgreSQL URL (required) |
| `CONTROL_TOWER_CONSUMER_ENABLED` | `false` | Enable Kafka consumer |
| `CONTROL_TOWER_KAFKA_BROKERS` | — | Required when consumer enabled |
| `CONTROL_TOWER_KAFKA_TOPIC` | `shipment.status.v1` | Source topic |
| `CONTROL_TOWER_KAFKA_GROUP_ID` | `control-tower-shipment-status-v1` | Stable consumer group |
| `CONTROL_TOWER_KAFKA_CLIENT_ID` | `control-tower-read-model-service` | Kafka client ID |

See `internal/config/config.go` for TLS/SASL and timeout settings.

## Internal API

- `GET /internal/v1/control-tower/shipments/{shipmentId}/status`
- `GET /internal/v1/control-tower/status-summary`
- `GET /internal/v1/control-tower/shipments/statuses`

Health: `GET /health`, `GET /ready`, `GET /metrics`

## Local development

```bash
# Unit tests
make control-tower-read-model-test

# Build
make control-tower-read-model-build

# Docker (profile read-model; consumer disabled by default)
make control-tower-read-model-up
```

Enable consumer locally:

```bash
export CONTROL_TOWER_CONSUMER_ENABLED=true
export CONTROL_TOWER_KAFKA_BROKERS=localhost:19092
export CONTROL_TOWER_DATABASE_URL=postgres://freight:freight_password@localhost:5432/freight_platform?sslmode=disable
make run-control-tower-read-model-service
```

## Documentation

See [docs/CONTROL_TOWER_SHIPMENT_STATUS_READ_MODEL.md](../../docs/CONTROL_TOWER_SHIPMENT_STATUS_READ_MODEL.md).

## Consumer restart

Integration test `TestConsumerRestartOffsetCommitFailureE2E` proves: DB commit → forced offset commit failure → consumer shutdown → second consumer same group → redelivery → inbox duplicate → safe offset commit → next event applied.

```bash
make control-tower-read-model-restart-e2e-test
```

Requires `TEST_KAFKA_BROKERS` and `TEST_DATABASE_URL`.
