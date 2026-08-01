# Shipment Status Outbox v0.1

## Purpose

Shipment status changes already persist canonical rows in `transport.shipment_status_history`. v0.1 adds a **transactional outbox** so each history row is accompanied by a durable publish intent in the **same PostgreSQL transaction**.

This avoids dual write:

```text
COMMIT shipment/history
→ INSERT outbox   ❌
```

The target invariant:

```text
shipment status update
+ shipment_status_history
+ shipment_event_outbox
```

either commit together or roll back together.

**Outbox does not replace status history as the UI timeline source.** Shipment Event History continues to read `shipment_status_history`, not the outbox table.

## Dual-write problem

Publishing to an external broker after commit creates inconsistency when the broker call fails. Publishing before commit creates inconsistency when the DB transaction rolls back. Transactional outbox stores the event durably inside the business transaction; a separate worker publishes asynchronously.

## Table schema

`transport.shipment_event_outbox` (migration `000014_create_shipment_event_outbox_v0.1`):

| Column | Purpose |
|--------|---------|
| `id` | Outbox row ID; also `eventId` in published envelope |
| `tenant_id` | Tenant scope |
| `aggregate_type` | `SHIPMENT` |
| `aggregate_id` | Shipment ID |
| `aggregate_version` | Shipment optimistic-lock version |
| `event_type` | Versioned domain event type |
| `schema_version` | Envelope schema version (`1`) |
| `source_event_id` | FK → `shipment_status_history.id` (unique) |
| `payload` | Serialized event envelope (JSONB) |
| `headers` | Transport metadata JSONB |
| `status` | `PENDING` / `PUBLISHED` / `FAILED` |
| `attempts` | Publish attempt counter |
| `available_at` | Next eligible publish time |
| `locked_at` / `locked_by` | Claim lease |
| `published_at` | Successful publish timestamp |
| `last_error_code` | Safe classified error code only |
| `created_at` | Insert timestamp |

Indexes:

- partial pending index on `(status, available_at, created_at)` where `status = 'PENDING'`
- tenant/aggregate diagnostic index on `(tenant_id, aggregate_id, aggregate_version)`

## Transactional invariant

```text
Изменение shipment status, запись shipment_status_history и создание shipment_event_outbox выполняются в одной PostgreSQL-транзакции. Успешный transition без соответствующей history/outbox записи недопустим.
```

Create:

```text
BEGIN → INSERT shipment → INSERT history → INSERT outbox → COMMIT
```

Transition:

```text
BEGIN → UPDATE shipment → INSERT history → INSERT outbox → COMMIT
```

Assign driver/vehicle without status change writes neither history nor outbox.

## Link to status history

- One history row → at most one outbox row (`UNIQUE(source_event_id)`)
- `source_event_id` is the persisted history UUID returned by `INSERT ... RETURNING id`
- Outbox payload `sourceEventId` matches that history ID

## Event envelope

Contract: `packages/events/shipment-status-event.v1.json`

Example:

```json
{
  "eventId": "outbox-row-uuid",
  "eventType": "shipment.status.changed",
  "schemaVersion": 1,
  "occurredAt": "2026-08-01T12:00:00Z",
  "tenantId": "tenant-uuid",
  "aggregate": {
    "type": "SHIPMENT",
    "id": "shipment-uuid",
    "version": 4
  },
  "sourceEventId": "shipment-status-history-uuid",
  "correlationId": "request-id",
  "data": {
    "fromStatus": "IN_TRANSIT",
    "toStatus": "DELIVERED",
    "reasonCode": null,
    "actorType": "USER"
  }
}
```

Excluded by default: JWT, email, phone, full shipment body, HTTP request body, SQL errors, internal URLs, actor ID.

## Event type mapping

| Status history | Outbox `event_type` |
|----------------|---------------------|
| `from_status = NULL` | `shipment.created` |
| `to_status = CANCELLED` | `shipment.cancelled` |
| `to_status = READY_FOR_BILLING` | `shipment.ready_for_billing` |
| `to_status = DOCUMENTS_COMPLETED` | `shipment.documents_completed` |
| `to_status = FINANCIALLY_CLOSED` | `shipment.financially_closed` |
| other status change | `shipment.status.changed` |

## Delivery semantics

```text
Outbox обеспечивает at-least-once delivery. Потребители обязаны быть идемпотентными по eventId или sourceEventId.
```

- Successful publish followed by failed `MarkPublished` may cause duplicate delivery
- Crash after claim but before publish/mark allows another worker to reclaim after lease timeout
- Not exactly-once

## Claim / lease model

Worker claims pending rows with:

```sql
SELECT ...
FROM transport.shipment_event_outbox
WHERE status = 'PENDING'
  AND available_at <= $now
  AND (locked_at IS NULL OR locked_at < $stale_lock_cutoff)
ORDER BY created_at ASC, id ASC
FOR UPDATE SKIP LOCKED
LIMIT $batch_size
```

Same transaction sets `locked_at`, `locked_by`, increments `attempts`.

## Retry / backoff

| Attempt after claim | Delay |
|---------------------|-------|
| 1 | +5s |
| 2 | +15s |
| 3 | +60s |
| 4+ | +5m |

After `SHIPMENT_OUTBOX_MAX_ATTEMPTS` (default 5): `status = FAILED`.

Safe error codes only in DB: `TRANSIENT_NETWORK`, `TRANSIENT_TIMEOUT`, `BROKER_UNAVAILABLE`, `PAYLOAD_REJECTED`, `CONFIGURATION_ERROR`, `UNKNOWN_PUBLISH_ERROR`.

## Publisher transport

v0.1 **does not publish to an external broker in production by default**:

- `SHIPMENT_OUTBOX_ENABLED=false` (default) — worker does not start; outbox rows accumulate for future publishing
- `SHIPMENT_OUTBOX_ENABLED=true` requires `SHIPMENT_OUTBOX_TRANSPORT`; startup fails because no broker transport is implemented yet
- No production noop publisher that marks events published without transport

Future Kafka/NATS integration should reuse the existing `EventPublisher` interface and project config conventions.

## Worker lifecycle

Integrated in `services/shipment-service/cmd/server/main.go`:

```text
load config → connect DB → create publisher → start HTTP + worker → shutdown signal
→ stop HTTP → wait for worker → exit
```

Readiness:

- worker disabled → HTTP readiness unchanged
- worker enabled, broker temporarily unavailable → service stays ready; events retry in outbox
- invalid publisher config with worker enabled → process exits at startup
- DB unavailable → existing readiness semantics

## Metrics

Prometheus metrics (low-cardinality labels only):

- `shipment_outbox_claimed_total{event_type}`
- `shipment_outbox_published_total{event_type,result}`
- `shipment_outbox_publish_failed_total{event_type,error_code}`
- `shipment_outbox_marked_failed_total{event_type,error_code}`
- `shipment_outbox_publish_duration_seconds{event_type,result}`
- `shipment_outbox_pending_count`
- `shipment_outbox_failed_count`
- `shipment_outbox_oldest_pending_age_seconds`

Gauges refresh once per worker poll cycle.

## Logging

Structured logs include: event ID, source event ID, aggregate ID/version, event type, attempt, worker ID, result, duration, safe error code.

Payload, JWT, credentials, email, phone, and raw broker responses are not logged.

## Retention

Published rows are **not deleted** in v0.1. Without a future retention/purge job the outbox table will grow.

## TODO / future work

- Broker transport implementation (Kafka/NATS) behind `EventPublisher`
- Published-row retention/purge
- Explicit FAILED replay tooling (internal only)
- Optional NOTIFY-based wakeups instead of pure polling

## v0.1 scope limits

- No public outbox CRUD API
- No backfill of historical transitions
- No shipment state machine changes
- No frontend/admin outbox UI
- No exactly-once guarantees

## PostgreSQL integration verification

Live PostgreSQL tests validate atomic writes, rollback, claim/lease semantics, and worker state transitions against a **disposable database**. They never run against the main dev dataset.

### Prerequisites

1. Start local PostgreSQL (Docker Compose):

```bash
make dev-up
```

2. Set admin connection URL (connects to server; tests create/drop their own database):

```bash
# PowerShell
$env:TEST_DATABASE_URL = "postgres://freight:freight_password@localhost:5432/postgres?sslmode=disable"

# bash
export TEST_DATABASE_URL="postgres://freight:freight_password@localhost:5432/postgres?sslmode=disable"
```

Use credentials from your local `.env` / Compose config. **Do not commit real passwords.**

### What the tests do

1. Create `freight_platform_outbox_test_<random>` database.
2. Apply all migrations through `000014_create_shipment_event_outbox_v0.1`.
3. Run integration scenarios (atomic create/transition, rollback, optimistic lock, unique source event, concurrent claim, lease recovery, ownership checks, worker + PostgreSQL).
4. Drop the temporary database (`DROP DATABASE ... WITH (FORCE)`).

### Run

```bash
make outbox-integration-test
```

Or directly:

```bash
cd services/shipment-service
go test -tags=integration ./internal/integration/outbox/... -count=1 -v
```

Without `TEST_DATABASE_URL`, integration tests **skip** with a clear message (unit tests still run via `go test ./...`).

Production broker transport is **not** required or implemented in v0.1; integration tests use an in-memory test publisher with the real PostgreSQL repository.
