# Control Tower Projection Rebuild

## Scope

Export/import v0.2 implements the working chain:

```text
shipment-service PostgreSQL (READ ONLY REPEATABLE READ)
→ NDJSON protocol v1
→ streaming importer
→ persistent rebuild job + staging rows
→ VALIDATED
→ atomic activation (v0.3)
→ Kafka catch-up (same consumer group)
```

Activation, rollback, advisory-lock coordination, and Kafka catch-up are implemented in v0.3. Primary mode remains blocked.

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

Activation, rollback, and cleanup require separate confirmations and do **not** auto-run after import.

## Activation (v0.3)

Preconditions:

- Job state `VALIDATED`
- Stage row count/checksum revalidated inside activation transaction
- Consumer pause recommended (not enforced) to minimize exclusive lock wait
- `CONFIRM_PROJECTION_REBUILD_ACTIVATION=true`
- CLI: `--activate --snapshot-id <uuid>`

Activation replaces the scoped active projection in one PostgreSQL transaction while holding the exclusive projection advisory lock (`ProjectionRebuildAdvisoryLockKey = 0x4354505350524F4A`).

Sequence:

```text
BEGIN → exclusive advisory lock → job FOR UPDATE → VALIDATED→ACTIVATING
→ revalidate stage → backup scoped active rows → DELETE scoped active
→ INSERT stage→active → post-validation → ACTIVE → COMMIT
```

Stage→active mapping:

| Stage | Active |
| --- | --- |
| `tenant_id` | `tenant_id` |
| `shipment_id` | `shipment_id` |
| `aggregate_version` | `shipment_version` |
| `current_status` | `current_status` |
| `previous_status` | `previous_status` |
| `last_event_id` (or nil UUID) | `last_event_id` |
| `last_source_event_id` (or nil UUID) | `last_source_event_id` |
| `last_event_type` from canonical outbox (`shipment.status.changed`, etc.) | `last_event_type` |
| `source_updated_at` | `last_occurred_at`, `authoritative_as_of` |
| activation time | `last_consumed_at`, `rebuilt_at`, `created_at`, `updated_at` |
| constant `true` | `complete` |
| cleared | gap fields |
| `AUTHORITATIVE_SNAPSHOT` | `projection_source` |
| job snapshot ID | `snapshot_id` |

`last_event_type` carries the **last real domain event type** from canonical outbox when present; it is `NULL` when the snapshot row has no published outbox event. Rebuild does **not** fabricate `projection.rebuild.snapshot` or other synthetic domain types.

After first live event on an activated row, consumer sets `projection_source=LIVE_EVENT`, `snapshot_id=NULL`, `authoritative_as_of=NULL`, and replaces event metadata (`last_event_id`, `last_source_event_id`, `last_event_type`, `last_occurred_at`, `last_consumed_at`) with the live Kafka envelope. `rebuilt_at` is preserved as the historical rebuild timestamp.

## Tenant activation and rollback

TENANT scope activation backs up, deletes, and inserts rows for the manifest tenant only. Other tenants remain field-by-field unchanged. Rollback restores backup rows for the scoped tenant; eligibility checks scope the live-write detector with `(projection_source <> AUTHORITATIVE_SNAPSHOT OR snapshot_id <> job_id) AND tenant_id = scoped_tenant`.

Empty-tenant scenarios:

- No prior active rows, non-empty stage → activation inserts rows, `backup_rows=0`; rollback removes tenant rows again.
- Prior active rows, empty validated stage → activation deletes tenant projection, `backup_rows>0`; rollback restores prior rows.

## Concurrent read visibility

During activation pause (mid-transaction after scoped DELETE), concurrent readers using ordinary READ COMMITTED queries see either the full pre-activation snapshot or, after COMMIT, the full post-activation snapshot. Partial/mixed tenant states are forbidden; integration tests block activation mid-transaction and poll readers.

## Summary query atomicity

Status summary HTTP handler reads totals/byStatus/incomplete inside a **single database transaction** so READ COMMITTED clients observe one consistent committed snapshot per request.

## Lock timeout

`SET LOCAL lock_timeout` uses bounded millisecond values derived from context deadline (safe numeric `set_config`, no string concatenation). Timeout or cancellation rolls back the DB transaction; Kafka offsets are not committed when projection lock acquisition fails.

## Rollback factual eligibility

Beyond provenance and row counts, eligibility runs bidirectional `EXCEPT` between active rows and stage-derived expected rows across version, status, event metadata, completeness, gap fields, provenance, and timestamps. Any drift → `ROLLBACK_WINDOW_CLOSED`. Live updates or new rows in scope also close the window.

## Kafka same-group catch-up

Consumer group remains `control-tower-shipment-status-v1`. Activation and rollback never read or modify Kafka offsets. Integration tests record `group_before=group_after`, `offset_after_activation=offset_before_pause`, and `offset_after_resume>offset_before_pause`.

Events published while consumer is paused: events already covered by snapshot version are STALE/DUPLICATE after resume; newer events apply as `LIVE_EVENT`.

## Historical and rollback acceptance

```text
make control-tower-projection-rebuild-historical-acceptance
make control-tower-projection-rebuild-rollback-acceptance
```

PostgreSQL fixture mode (`RUN_REBUILD_ACCEPTANCE_FIXTURE=1`) validates pre-activation mismatch, post-activation stage parity, inbox/dead-letter preservation, and post-rollback exact restore. Gateway mode requires `GATEWAY_URL`, `JWT`, and `TENANT_ID` for full shadow `comparison=MATCH` with `public source=LEGACY`.

## 20k query plans (diagnostic)

Integration test (`RUN_20K_REBUILD_TESTS=1`) exercises 20k stage/active/backup rows across multiple tenants. Representative plans use index scans on `idx_projection_rebuild_stage_snapshot_tenant` and `idx_shipment_status_projection_tenant_updated`; stage→active insert ~140ms and EXCEPT validation ~50ms on local fixture hardware (not production SLO).

## Observability

CLI activation/rollback emit structured operational summaries via `operation_summary.go` (slog fields: operation, scope, result, duration). These are not process-local Prometheus counters.

## Cleanup states

Cleanup allowed for `FAILED`, `CANCELLED`, `ROLLED_BACK`; forbidden for `ACTIVE` (`ACTIVE_CLEANUP_FORBIDDEN`). Removes stage and backup rows only; active projection, inbox, dead-letter, and Kafka offsets unchanged.

## Live consumer advisory lock

The live Kafka consumer acquires the shared projection advisory lock in every projection-write transaction before inbox dedupe and projection read/update. Static tests assert activation package does not import Kafka clients.

## Rollback window

Rollback allowed only while active scoped projection exactly matches activated snapshot (all rows `projection_source=AUTHORITATIVE_SNAPSHOT`, `snapshot_id=job id`, field-equivalent to stage). Any live write closes the window (`ROLLBACK_WINDOW_CLOSED`).

Requires `CONFIRM_PROJECTION_REBUILD_ROLLBACK=true` and `--rollback --snapshot-id <uuid>`.

Activation failure inside the activation transaction rolls back to `VALIDATED` with prior active projection intact; safe error codes may be recorded in a separate guarded update. Rollback failure leaves job `ACTIVE` with backup intact.

## Import confirmation

Persistent import requires:

```text
CONFIRM_PROJECTION_REBUILD_IMPORT=true
```

Separate from activation confirmation. Import may create jobs and stage rows only; it never activates.

Activation requires:

```text
CONFIRM_PROJECTION_REBUILD_ACTIVATION=true
```

## Job lifecycle

| State | Meaning |
| --- | --- |
| `IMPORTING` | Job created, stage batches may be partial |
| `VALIDATED` | DB-side stage validation passed; projection unchanged |
| `ACTIVATING` | Activation transaction in progress |
| `ACTIVE` | Scoped projection replaced; rollback window open until live writes |
| `ROLLING_BACK` | Rollback transaction in progress |
| `ROLLED_BACK` | Backup restored |
| `FAILED` | Safe error code stored; projection unchanged |
| `CLEANED` | Stage/backup rows removed; job audit retained |

Snapshot ID reuse:

- `VALIDATED` → `SNAPSHOT_ALREADY_IMPORTED`
- `IMPORTING` → `SNAPSHOT_IMPORT_IN_PROGRESS`
- `FAILED` → `SNAPSHOT_IMPORT_REUSE_FORBIDDEN` (explicit cleanup required)

## Kafka catch-up

After activation, resume the same consumer group (`control-tower-shipment-status-v1`). Offsets are never reset or modified by activation or rollback. Stale/duplicate events at or below snapshot version are acknowledged without reverting projection; newer events apply with `LIVE_EVENT` provenance.

## Not yet implemented

- Primary mode (blocked)
- Production SLO for activation lock duration

## Required statements

> Activation replaces the scoped active projection in one PostgreSQL transaction while holding the exclusive projection advisory lock.

> The live Kafka consumer acquires the shared projection advisory lock in every projection-write transaction.

> Kafka consumer offsets are never reset or modified by activation or rollback.

> Rollback is refused after any live projection write has changed the activated snapshot scope.

> Inbox and dead-letter records are preserved during activation and rollback.

> The projection rebuild protocol represents an authoritative current-state snapshot and does not fabricate historical Kafka events.

> Snapshot data is streamed through stdin/stdout by default and is not persisted to disk.
