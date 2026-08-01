# Shipment Status Kafka Publisher v0.1

## Purpose

v0.1 connects the existing PostgreSQL transactional outbox worker to a **Kafka-compatible producer** so shipment status events can be consumed by downstream services.

Kafka publication does not replace the PostgreSQL transactional outbox. PostgreSQL remains the durable source of pending events until broker acknowledgement is received and the outbox row is marked PUBLISHED.

Delivery is at least once. Consumers must deduplicate by eventId or sourceEventId.

## Why Kafka-compatible transport

- Industry-standard event streaming protocol for microservice integration
- Compatible with managed Kafka offerings and on-prem clusters
- Preserves the existing outbox boundary: handlers never publish directly

## Why Redpanda in dev/test only

Redpanda provides a lightweight Kafka-protocol broker for local Docker and CI. Production may use any Kafka-compatible cluster; shipment-service does not depend on Redpanda-specific APIs.

## Flow

```text
shipment mutation
→ shipment_status_history
→ shipment_event_outbox (same PostgreSQL tx)
→ outbox worker
→ KafkaPublisher (franz-go)
→ topic shipment.status.v1
```

## Topic

| Setting | Default |
|---------|---------|
| `SHIPMENT_KAFKA_TOPIC` | `shipment.status.v1` |

Production topics are owned by platform operations. The application **does not** auto-create production topics.

Dev topic initialization:

```bash
make messaging-up
make shipment-kafka-topic-create
```

Recommended dev settings: 3 partitions, replication factor 1, cleanup policy delete.

## Message key

Kafka record key = shipment aggregate UUID (`aggregate_id`).

- Same shipment events share one key
- Key does not include tenant ID (tenant is in envelope)
- Ordering is guaranteed **only within a partition for messages sharing the same shipment key**

## Event envelope

Value is the full JSON envelope stored in outbox `payload`, validated by `packages/events/shipment-status-event.v1.json`:

- `eventId`, `eventType`, `schemaVersion`, `occurredAt`, `tenantId`
- `aggregate`, `sourceEventId`, `data`
- optional `correlationId`

The publisher sends the full envelope, not `data` alone.

## Kafka headers (allowlist)

| Header | Source |
|--------|--------|
| `event_type` | outbox event type |
| `schema_version` | schema version |
| `source_event_id` | history row ID |
| `correlation_id` | envelope (if present) |
| `content_type` | `application/json` |

Not forwarded: Authorization, JWT, Cookie, X-User-*, X-Tenant-ID, SASL credentials, raw `outbox.headers`.

## At-least-once semantics

1. Worker claims outbox row
2. Publisher waits for broker acknowledgement
3. Worker calls `MarkPublished`
4. If step 2 succeeds but step 3 fails, retry may produce duplicate Kafka messages with the same `eventId`

Consumers must deduplicate by `eventId` or `sourceEventId`.

## Producer acknowledgement

`KafkaPublisher.Publish` uses synchronous produce with `RequiredAcks=all`. Success is returned only after broker acknowledgement. Enqueue-without-ack is not treated as published.

## Client retry limits

Franz-go uses bounded transport retries (`RecordRetries=3`). The worker `SHIPMENT_OUTBOX_PUBLISH_TIMEOUT` bounds the full publish operation. Durable retry/backoff remains in the outbox worker (`ReleaseWithRetry`).

Invariant: `SHIPMENT_OUTBOX_LEASE_TIMEOUT > SHIPMENT_OUTBOX_PUBLISH_TIMEOUT`.

## Worker durable retry

Transient broker/network/timeout errors schedule retry with exponential backoff. Permanent payload/config errors mark the outbox row `FAILED`.

## Broker outage behavior

- Invalid Kafka config → shipment-service startup failure
- Broker temporarily unavailable → service starts; HTTP readiness unchanged; outbox rows stay `PENDING`; metrics/logs show failures

## Readiness

Kafka broker availability is **not** an HTTP readiness dependency.

## Configuration

| Variable | Default | Notes |
|----------|---------|-------|
| `SHIPMENT_OUTBOX_TRANSPORT` | empty | `kafka` when outbox enabled |
| `SHIPMENT_KAFKA_BROKERS` | empty | Required when transport=kafka |
| `SHIPMENT_KAFKA_TOPIC` | `shipment.status.v1` | |
| `SHIPMENT_KAFKA_CLIENT_ID` | `shipment-service` | |
| `SHIPMENT_KAFKA_DIAL_TIMEOUT` | `10s` | |
| `SHIPMENT_KAFKA_WRITE_TIMEOUT` | `10s` | |
| `SHIPMENT_KAFKA_TLS_ENABLED` | `false` | |
| `SHIPMENT_KAFKA_TLS_CA_FILE` | empty | |
| `SHIPMENT_KAFKA_TLS_CERT_FILE` | empty | Requires key |
| `SHIPMENT_KAFKA_TLS_KEY_FILE` | empty | Requires cert |
| `SHIPMENT_KAFKA_TLS_SERVER_NAME` | empty | |
| `SHIPMENT_KAFKA_SASL_MECHANISM` | empty | `plain`, `scram-sha-256`, `scram-sha-512` |
| `SHIPMENT_KAFKA_SASL_USERNAME` | empty | Required with SASL |
| `SHIPMENT_KAFKA_SASL_PASSWORD` | empty | Never logged |

When `SHIPMENT_OUTBOX_ENABLED=false`, Kafka settings are not required.

## TLS / SASL

TLS and SASL are validated at startup when outbox transport is `kafka`. Passwords are never included in error strings or logs.

## Local Redpanda

```bash
make messaging-up
make messaging-status
make shipment-kafka-topic-create
```

Compose profile: `messaging`

| Listener | Address |
|----------|---------|
| Docker network | `redpanda:9092` |
| Host (dev) | `localhost:19092` |

Stop without removing unrelated volumes:

```bash
make messaging-down
```

PowerShell (Git Bash recommended for Makefile):

```powershell
& "C:\Program Files\Git\bin\bash.exe" -lc "make messaging-up"
```

## Integration test isolation

Each Kafka integration test creates its own topic:

```text
shipment.status.v1.test.<random-suffix>
```

Requirements:

* Topic is created via `kadm` test helper before publish/consume
* Topic has 3 partitions and replication factor 1 (local Redpanda)
* Topic is deleted in `t.Cleanup` after the test
* The shared dev topic `shipment.status.v1` is **not** used for test isolation
* Consumer offset starts at `AtStart` on the empty unique topic
* Consumer reads until the expected `eventId` is found (not "first message wins")
* `TEST_KAFKA_BROKERS` is required; `TEST_KAFKA_TOPIC` is optional (manual smoke only)
* Optional prefix override: `TEST_KAFKA_TOPIC_PREFIX=shipment.status.v1.test`
* Kafka admin permissions exist only in integration tests and dev scripts, not in shipment-service runtime
* CI tests can run sequentially or in parallel because topic names are unique

Production topic `shipment.status.v1` is owned by platform operations and created via `make shipment-kafka-topic-create`. The application does not auto-create production topics and must not rely on broker auto-topic creation in production.

Redpanda is used only as a local/dev/test Kafka-compatible runtime.

## Integration tests

Kafka-only (unique topic per test; only broker required):

```powershell
$env:TEST_KAFKA_BROKERS = "localhost:19092"
make outbox-kafka-integration-test
```

PostgreSQL + Kafka end-to-end:

```powershell
$env:TEST_DATABASE_URL = "postgres://freight:freight_password@localhost:5432/postgres?sslmode=disable"
$env:TEST_KAFKA_BROKERS = "localhost:19092"
make outbox-end-to-end-test
```

## Logging / data minimization

Publisher/worker logs may include: `event_id`, `source_event_id`, `aggregate_id`, `aggregate_version`, `event_type`, `topic`, `attempt`, `duration`, `result`, `error_code`.

Never logged: message value/payload, JWT, SASL password, tenant ID in transport logs, full headers map.

## Metrics

| Metric | Labels |
|--------|--------|
| `shipment_outbox_kafka_publish_total` | `event_type`, `result` |
| `shipment_outbox_kafka_publish_duration_seconds` | `event_type`, `result` |
| `shipment_outbox_kafka_publish_errors_total` | `event_type`, `error_code` |

No topic/broker/tenant/shipment/event labels.

## Schema Registry

Not used in v0.1. Event contract is the versioned JSON schema in `packages/events/shipment-status-event.v1.json`.

## Future consumer rollout

Downstream services will consume `shipment.status.v1`, deduplicate by `eventId`/`sourceEventId`, and treat the envelope as the contract. Consumer deployment, offset management, and replay tooling are out of scope for v0.1.

## Go client

Library: [`github.com/twmb/franz-go`](https://github.com/twmb/franz-go) (pure Go, lazy connection, sync produce with acks).

Producer closes after outbox worker shutdown via `CloseablePublisher.Close` with a bounded context.
