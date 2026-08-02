# Control Tower Projection Rebuild

## Scope

Export/import v0.2 implements the working chain:

```text
shipment-service PostgreSQL (READ ONLY REPEATABLE READ)
→ NDJSON protocol v1
→ streaming importer
→ persistent rebuild job + staging rows
→ VALIDATED
```

Activation, rollback, live consumer advisory-lock integration, Kafka catch-up, and historical shadow acceptance remain out of scope.

> **A VALIDATED rebuild job contains a fully verified staging snapshot but does not modify the active Control Tower projection.**

## Authoritative DB sources

| Snapshot field | Authoritative source | Nullable | Rule |
| --- | --- | ---: | --- |
| `tenantId` | `transport.shipments.tenant_id` | No | Non-zero UUID |
| `shipmentId` | `transport.shipments.id` | No | Non-zero UUID |
| `currentStatus` | `transport.shipments.status` | No | Must match latest history `to_status` |
| `previousStatus` | Latest `transport.shipment_status_history.from_status` | Yes | NULL on initial create transition |
| `aggregateVersion` | `transport.shipments.version` | No | Must equal latest history `shipment_version`; >= 1 |
| `lastEventId` | `transport.shipment_event_outbox.id` (latest history row) | Yes | Allowed absent when outbox not published |
| `lastSourceEventId` | Latest `transport.shipment_status_history.id` | Yes | Required when canonical history exists |
| `sourceUpdatedAt` | Latest history `occurred_at` | No | Transition timestamp, not `shipments.updated_at` |

### Status/history consistency

Exporter rejects rows when:

- latest history is missing → `MISSING_CANONICAL_STATUS_HISTORY`
- `shipments.status` ≠ latest history `to_status` → `AUTHORITATIVE_STATUS_MISMATCH`
- `shipments.version` ≠ latest history `shipment_version` → `AUTHORITATIVE_STATUS_MISMATCH`
- `shipments.version` < 1 → `MISSING_AGGREGATE_VERSION`

Soft-deleted rows (`deleted_at IS NOT NULL`) are excluded.

### CREATED policy (variant B)

`CREATED` is a DB default pre-lifecycle state excluded from protocol v1 and read-model consumer allowlists. Domain create path writes `CARRIER_ASSIGNED` with canonical history. Legacy rows with `status='CREATED'` are **not silently excluded** — export fails with `UNSUPPORTED_SHIPMENT_STATUS`.

### Event metadata policy

Asymmetric metadata is allowed when history exists without a published outbox row:

```text
lastSourceEventId present, lastEventId absent → allowed
lastEventId present, lastSourceEventId absent → rejected (INCONSISTENT_METADATA)
both absent → allowed when no outbox/history IDs exported
both present → normal published path
```

### Kafka event ID source

| Purpose | Field |
| --- | --- |
| Outbox PK | `transport.shipment_event_outbox.id` |
| Kafka envelope `eventId` | Same UUID, embedded in outbox `payload.eventId` at outbox build time |
| `sourceEventId` | `transport.shipment_status_history.id` (FK `source_event_id`) |
| Aggregate ID | `transport.shipment_event_outbox.aggregate_id` (= shipment UUID) |
| Aggregate version | `transport.shipment_event_outbox.aggregate_version` |
| Event type | `transport.shipment_event_outbox.event_type` (`domain.MapOutboxEventType`) |

**Proof:** `BuildOutboxEventFromStatusHistory` assigns one UUID to both outbox row `id` and envelope `eventId`. Kafka publisher sends the pre-built payload unchanged. Read-model consumer parses `eventId` from envelope JSON → inbox/projection `last_event_id`.

Exporter `lastEventId` = `transport.shipment_event_outbox.id` joined on `source_event_id = latest history.id`.

### History→outbox uniqueness

Migration `000014` defines `UNIQUE (source_event_id)` on `transport.shipment_event_outbox`. At most one outbox row per canonical history row; snapshot join cannot multiply rows. DB constraint is asserted in integration tests.

### Outbox publish status semantics

`lastEventId` is the canonical event identity, not Kafka delivery confirmation:

| Outbox `status` | Snapshot `lastEventId` |
| --- | --- |
| row absent | `null` |
| `PENDING` | outbox `id` |
| `PUBLISHED` | outbox `id` |
| `FAILED` | outbox `id` |

Exporter validates outbox aggregate ID/version/event type and envelope `eventId`/`sourceEventId` consistency. Mismatch codes: `OUTBOX_AGGREGATE_ID_MISMATCH`, `OUTBOX_AGGREGATE_VERSION_MISMATCH`, `INCONSISTENT_OUTBOX_EVENT_ID`, `INCONSISTENT_OUTBOX_EVENT_TYPE`.

### CLI-to-CLI integration test

Windows-compatible E2E (no bash required):

```text
go test -tags=integration ./services/shipment-service/internal/integration/projectionrebuild/... -count=1
```

Builds real exporter/importer binaries, pipes stdout→stdin across two temporary PostgreSQL databases, asserts `VALIDATED`, stage/source row equality, and explicit projection/inbox/dead-letter unchanged counts.

### Tenant query predicate

Tenant-scoped export filters history inside the CTE (`WHERE h.tenant_id = $1`) and shipments (`WHERE s.tenant_id = $1 AND s.deleted_at IS NULL`).

### Large snapshot full pipeline

20k integration test covers exporter→importer CLI pipe with bounded batch inserts (default 500), stage row count validation, and active projection unchanged.

### Concurrent import

Duplicate concurrent importers for the same snapshot ID: one succeeds (`VALIDATED`), the second receives `SNAPSHOT_IMPORT_IN_PROGRESS` or `SNAPSHOT_ALREADY_IMPORTED`. Stage rows are not doubled.

### Late MarkFailed protection

`MarkFailed` updates only jobs in `IMPORTING` state; `VALIDATED` jobs cannot be downgraded.

### Shadow acceptance

Rebuild changes must not break shadow rollout `comparison=MATCH` with `primary` disabled. Run `make control-tower-shadow-rollout-acceptance` before merge.

## Snapshot protocol

NDJSON v1 (`manifest`, `shipment`, `complete`). Shared module: `packages/statussnapshot/`.

Manifest requires `ordering=TENANT_ID_SHIPMENT_ID`. TENANT scope requires manifest `tenantId` even for zero-row snapshots.

## Export transaction

Single PostgreSQL transaction per snapshot:

```text
BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY
→ authoritative row/tenant counts
→ streaming SELECT ORDER BY tenant_id, shipment_id
→ validate streamed counts
→ COMMIT
```

Manifest reports `transactionIsolation=REPEATABLE_READ`.

## Import confirmation

Persistent import requires:

```text
CONFIRM_PROJECTION_REBUILD_IMPORT=true
```

Separate from activation confirmation. Import may create jobs and stage rows only.

Activation still requires `CONFIRM_PROJECTION_REBUILD_ACTIVATION=true` and remains `NOT_IMPLEMENTED`.

## Job lifecycle

| State | Meaning |
| --- | --- |
| `IMPORTING` | Job created, stage batches may be partial |
| `VALIDATED` | DB-side stage validation passed; projection unchanged |
| `FAILED` | Safe error code stored; projection unchanged |

Snapshot ID reuse:

- `VALIDATED` → `SNAPSHOT_ALREADY_IMPORTED`
- `IMPORTING` → `SNAPSHOT_IMPORT_IN_PROGRESS`
- `FAILED` → `SNAPSHOT_IMPORT_REUSE_FORBIDDEN` (explicit cleanup required)

## Dry-run

Non-persistent stream validation. Job/stage/projection counts unchanged.

## Failure recovery

Broken stream (missing completion) → job `FAILED`, safe code, partial stage rows may remain, projection unchanged.

## Large snapshots

Exporter/importer stream row-by-row with bounded batch inserts (default 500, max 10000). No unbounded in-memory duplicate map.

## Not yet implemented

- Atomic activation / projection swap
- Rollback execution
- Live consumer shared advisory lock
- Kafka catch-up verification
- Historical shadow acceptance
- Primary mode (blocked)

Rebuild is **not** operationally ready for production cutover until activation and catch-up are implemented.

## Required statements

> The projection rebuild protocol represents an authoritative current-state snapshot and does not fabricate historical Kafka events.

> Kafka consumer offsets are never reset by the exporter or importer.

> Snapshot data is streamed through stdin/stdout by default and is not persisted to disk.
