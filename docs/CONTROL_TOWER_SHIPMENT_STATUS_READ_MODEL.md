# Control Tower Shipment Status Read Model v0.1

## Purpose

The Control Tower read-model service (`control-tower-read-model-service`) consumes canonical shipment status events from Kafka and projects a tenant-scoped PostgreSQL read-model for internal Control Tower queries.

**The Control Tower projection is not the transactional source of truth. Shipment-service and shipment_status_history remain authoritative.**

## Data flow

```text
shipment-service
→ shipment_status_history
→ shipment_event_outbox
→ Kafka shipment.status.v1
→ Control Tower consumer (control-tower-shipment-status-v1)
→ control_tower.shipment_status_event_inbox
→ control_tower.shipment_status_projection
→ internal read API
```

## Kafka

| Setting | Value |
|---------|-------|
| Topic | `shipment.status.v1` |
| Consumer group | `control-tower-shipment-status-v1` |
| Client library | `github.com/twmb/franz-go v1.21.5` |
| Auto-commit | **Disabled** (`kgo.DisableAutoCommit`) |

**Kafka delivery is at least once. Projection idempotency is enforced through the event inbox before Kafka offsets are committed.**

**Kafka offsets are committed only after the corresponding PostgreSQL projection or dead-letter transaction has committed.**

### Offset commit sequence

Valid event:

```text
Poll → validate → BEGIN → inbox → projection → COMMIT → commit offset
```

Permanent invalid event:

```text
Poll → validate fails → BEGIN → dead-letter → COMMIT → commit offset
```

Transient DB failure: offset not committed; record redelivered; inbox deduplicates.

## PostgreSQL schema

Schema: `control_tower` (migration `000015`)

### Inbox — `control_tower.shipment_status_event_inbox`

Idempotency keys:

- `event_id` (PK)
- `source_event_id` (UNIQUE)
- `topic + partition_id + message_offset` (UNIQUE)

Outcomes: `APPLIED`, `GAP_APPLIED`, `STALE`, `DUPLICATE`

### Projection — `control_tower.shipment_status_projection`

PK: `(tenant_id, shipment_id)`

Indexes: `(tenant_id, current_status)`, `(tenant_id, updated_at DESC, shipment_id)`

Gap fields: `complete`, `gap_detected`, `gap_from_version`, `gap_to_version`

### Dead-letter — `control_tower.shipment_status_event_dead_letter`

Stores metadata only: topic/partition/offset, optional parsed IDs, `payload_sha256`, `error_code`.

**Raw invalid payload is never stored or logged.**

## Version handling

**Version gaps are surfaced explicitly and are not silently interpreted as a complete event history.**

| Scenario | Behavior |
|----------|----------|
| First event version 1 | Complete projection |
| First event version > 1 | Latest state with gap marker (`complete=false`) |
| Next version (N+1) | Apply update |
| Stale (≤ current) | Inbox `STALE`, projection unchanged |
| Gap (> N+1) | Apply latest state, set gap marker, `GAP_APPLIED` |
| Duplicate (event/source/position) | Inbox `DUPLICATE`, projection unchanged |

Existing gaps are not silently cleared when later sequential events arrive.

## Event contract

Schema: `packages/events/shipment-status-event.v1.json`

Supported `schemaVersion`: **1** only. Application-level validation (not Schema Registry in v0.1).

Allowed event types:

- `shipment.created`
- `shipment.status.changed`
- `shipment.cancelled`
- `shipment.ready_for_billing`
- `shipment.documents_completed`
- `shipment.financially_closed`

## Tenant isolation

- Projection PK: `tenant_id + shipment_id`
- Internal API tenant from trusted header `X-Tenant-ID` only
- Missing tenant → 401; malformed/zero → 400; foreign/unknown → 404

## Internal API

| Endpoint | Description |
|----------|-------------|
| `GET /internal/v1/control-tower/shipments/{shipmentId}/status` | Shipment status detail |
| `GET /internal/v1/control-tower/status-summary` | Tenant aggregates + freshness |
| `GET /internal/v1/control-tower/shipments/statuses` | Cursor-paginated list |

Not exposed via public `/api/v1/...`. API Gateway BFF integration is a separate rollout step.

## Health and readiness

- `/health` — process alive (200)
- `/ready` — PostgreSQL reachable (200). Kafka outage does not fail readiness; read API remains available with potentially stale projections.

## Failure behavior

| Failure | Behavior |
|---------|----------|
| PostgreSQL transient | No offset commit; redelivery; inbox dedupe |
| Offset commit after DB commit | Redelivery; duplicate detection; safe re-commit |
| Permanent invalid payload | Dead-letter + offset commit |
| Consumer restart | Resume from last committed offset; idempotent inbox |
| Kafka outage | Consumer polls fail; projection unchanged; metrics show errors |

## Metrics

Bounded labels only (`event_type`, `outcome`, `error_code`). No tenant/shipment/event IDs in labels.

Key metrics: `control_tower_shipment_consumer_records_total`, `control_tower_shipment_projection_applied_total`, `control_tower_shipment_dead_letter_total`, freshness gauges.

## Logging

Structured logs may include event/shipment IDs, topic/partition/offset, outcome, safe error codes.

Never logged: payload, headers map, tenant_id, credentials, raw DB URL.

## Docker Compose

Profile: `read-model` (optional). Consumer disabled by default in compose (`CONTROL_TOWER_CONSUMER_ENABLED=false`).

```bash
docker compose -f infrastructure/docker-compose/docker-compose.yml --profile read-model up -d control-tower-read-model-service
```

Requires `messaging` profile for Redpanda when enabling consumer.

## Consumer restart and offset commit failure

**A Kafka offset commit failure after a successful PostgreSQL commit may cause the same record to be delivered again. The event inbox makes this redelivery idempotent, after which the offset can be committed safely.**

1. PostgreSQL commit happens before Kafka offset commit.
2. Offset commit failure causes Kafka to redeliver the same record.
3. The inbox prevents a second projection mutation (duplicate detection by `event_id`, `source_event_id`, or Kafka position).
4. A new consumer instance with the **same stable production group ID** (`control-tower-shipment-status-v1`) can safely continue.
5. Duplicate records still require a successful offset commit before the partition advances.
6. The projection is **not** an exactly-once consumer; the **effect on projection is idempotent**.
7. Dead-letter inserts use the same ordering: PostgreSQL commit first, then offset commit.
8. Production uses the stable group ID; integration tests use unique group IDs and topics per suite.

When inbox PK prevents a second row, the original inbox row keeps the first outcome (`APPLIED` / `GAP_APPLIED`); redelivery is classified as `DUPLICATE` in processing/metrics without mutating projection.

## Integration tests

- PostgreSQL: `TEST_DATABASE_URL` — temporary database, migrations through `000015`
- Kafka: `TEST_KAFKA_BROKERS` — unique topics `shipment.status.v1.test.<suffix>`

## TODO (post v0.1)

- Gateway BFF routing to read-model internal API
- Projection rebuild/replay from Kafka
- Dead-letter replay tooling
- Consolidate Kafka TLS/SASL config into shared package (currently duplicated from shipment-service)

## v0.1 limitations

- Eventually consistent only (no exactly-once)
- Sequential consumer loop (no per-partition worker pool)
- Application-level schema validation only
- No frontend or public browser API
- Does not replace existing Control Tower summary in API Gateway
