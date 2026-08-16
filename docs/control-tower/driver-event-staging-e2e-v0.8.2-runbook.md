# Control Tower Driver Event — Staging E2E Runbook (v0.8.2)

Use this runbook to verify the driver delay/problem → Control Tower path in **staging only**. Do not enable globally in production without pilot approval.

## Enable / disable

| Action | Setting |
|---|---|
| **Enable** | `CONTROL_TOWER_DRIVER_EVENTS_ENABLED=true` |
| **Disable** | `CONTROL_TOWER_DRIVER_EVENTS_ENABLED=false` (default) |

Feature flag default is **false** (`control-tower-read-model-service/internal/config/config.go`).

## Required configuration (no secrets in Git)

### Control Tower consumer (`control-tower-read-model-service`)

| Variable | Purpose | Default |
|---|---|---|
| `CONTROL_TOWER_DRIVER_EVENTS_ENABLED` | Start driver Kafka consumer | `false` |
| `CONTROL_TOWER_DRIVER_KAFKA_BROKERS` | Broker list (required when enabled) | — |
| `CONTROL_TOWER_DRIVER_KAFKA_TOPIC` | Driver events topic | `driver.events.v1` |
| `CONTROL_TOWER_DRIVER_KAFKA_GROUP_ID` | Consumer group | `control-tower-driver-events-v1` |
| `CONTROL_TOWER_DRIVER_KAFKA_CLIENT_ID` | Client id | `control-tower-driver-events-consumer` |
| `CONTROL_TOWER_DRIVER_KAFKA_POLL_TIMEOUT` | Poll timeout | `1s` |
| `CONTROL_TOWER_DRIVER_KAFKA_PROCESS_TIMEOUT` | Handler timeout | `10s` |
| `CONTROL_TOWER_DRIVER_KAFKA_COMMIT_TIMEOUT` | Commit timeout | `5s` |

### Shipment producer / outbox (`shipment-service`)

| Variable | Purpose | Default |
|---|---|---|
| `SHIPMENT_KAFKA_DRIVER_TOPIC` | Outbox publish topic | `driver.events.v1` |

### Tracking loss detector (optional path)

| Variable | Purpose |
|---|---|
| `TRACKING_LOSS_DETECTOR_ENABLED` | Enable server-side tracking loss |
| `CONTROL_TOWER_DRIVER_TRACKING_LOST_AFTER_MINUTES` | Lost threshold |

## Verification flow

```
Driver client (API gateway)
  → POST /api/v1/driver/me/shipments/{id}/delays
  → POST /api/v1/driver/me/shipments/{id}/exceptions
  → shipment-service domain + transport.shipment_event_outbox
  → Kafka topic driver.events.v1
  → control-tower-read-model-service driverconsumer
  → domain/driver_event_ingestion.go (normalize + tenant/shipment guard)
  → control_tower.driver_event_inbox + automation + critical workflow
  → gateway /api/v1/control-tower/* 
  → web-admin Control Tower UI
```

## Staging E2E scenarios

Use dedicated test tenant, shipment, and driver. Never use production customer data.

### E2E-01 — Driver delay

1. Authenticate as test driver (gateway → shipment-service).
2. `POST /api/v1/driver/me/shipments/{shipmentId}/delays` with reason code and idempotency key.
3. Confirm outbox row: `event_type=driver.delay.reported`.
4. Confirm Kafka message on `driver.events.v1`.
5. Confirm CT consumer metrics/logs (`driverconsumer` consumed counter).
6. Confirm `control_tower.driver_event_inbox` row and automation recommendation if rules seeded.
7. Query Control Tower summary / shipment timeline via gateway.

Record: event id, correlation id, Kafka offset.

### E2E-02 — Driver problem

1. `POST /api/v1/driver/me/shipments/{shipmentId}/exceptions` (category e.g. `VEHICLE_BREAKDOWN`).
2. Confirm outbox `event_type=driver.problem.reported`.
3. Confirm CT critical workflow row for tenant/shipment.
4. ACK via `POST /api/v1/control-tower/critical-events/{eventId}/acknowledge`.

### E2E-03 — Duplicate

Replay same Kafka payload or repeat API call with same idempotency key. Expect single inbox/automation side effect.

### E2E-04 — Tenant isolation

Publish or inject event with tenant A shipment id but tenant B header/context. Expect reject/ignore; no tenant B mutation of tenant A data.

### E2E-05 — Feature flag off

Set `CONTROL_TOWER_DRIVER_EVENTS_ENABLED=false`, restart CT service. Driver events must not be consumed; unrelated CT processing continues.

## Observability

Prometheus (`driverconsumer/metrics.go`):

- `control_tower_driver_events_consumed_total`
- `control_tower_driver_events_failed_total`
- `control_tower_driver_events_duplicate_total`

Structured logs via `slog` on consume failures and permanent rejections.

## Local verification (repository)

```powershell
cd services/control-tower-read-model-service
go test ./internal/config -v
go test -tags=integration ./internal/integration/automation/... -run DriverEvent
go build ./internal/driverconsumer/...

cd ../shipment-service
go test -tags=integration ./internal/integration/driverplatform/... -run "Delay|Exception|CrossTenant"

cd ../tracking-service
go test -tags=integration ./internal/integration/trackingloss/...
```

Kafka integration test (requires `TEST_DATABASE_URL` or embedded PG harness):

```powershell
go test -tags=integration ./internal/integration/kafka/... -run DriverEvent
```

## Known gaps (v0.8.2)

- No native mobile driver app in this repository; verification uses HTTP driver API via gateway.
- Staging runtime verification requires Selectel/staging credentials (not available in CI sandbox).
