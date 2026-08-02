# Control Tower Projection Rebuild — Core Infrastructure v0.1

## Scope

This document covers the core infrastructure phase only: protocol, migration schema, exporter/importer skeletons, streaming validation, and dry-run. Activation, rollback, live consumer advisory-lock integration, and Kafka catch-up acceptance are explicitly out of scope for this phase.

## Snapshot protocol

NDJSON v1 with record types `manifest`, `shipment`, `complete`. Shared module: `github.com/freight-platform/statussnapshot` (`packages/statussnapshot/`).

## Manifest

Fields: `recordType`, `schemaVersion`, `snapshotId`, `scope` (`ALL`|`TENANT`), optional `tenantId` (required for `TENANT`, absent for `ALL`), `ordering` (`TENANT_ID_SHIPMENT_ID`), `startedAt`, `transactionIsolation` (`REPEATABLE_READ`), `source` (`SHIPMENT_SERVICE`).

### Tenant identity

A TENANT-scoped snapshot identifies the tenant in the manifest, including when the snapshot contains zero shipment records.

## Ordering

Protocol v1 requires records to be ordered by tenant ID and shipment ID. This permits streaming duplicate and ordering validation with constant memory.

Exporter and importer validate ascending `(tenantId, shipmentId)` order. Violations emit `RECORD_ORDER_VIOLATION`; adjacent duplicates emit `DUPLICATE_SHIPMENT`.

## Shipment record

Authoritative fields from shipment-service (when export query is implemented): `currentStatus`, `aggregateVersion`, optional `previousStatus`, optional `lastEventId` / `lastSourceEventId`, `sourceUpdatedAt`. Fields are never fabricated.

`aggregateVersion` is required (>= 1). `previousStatus`, `lastEventId`, and `lastSourceEventId` are optional. `sourceUpdatedAt` is required. `lastEventId` without `lastSourceEventId` is rejected as inconsistent metadata; `lastSourceEventId` without `lastEventId` is allowed when history exists without a published outbox event.

## Duplicate detection

- **Stream validation:** adjacent-key comparison with O(1) memory (previous `(tenantId, shipmentId)` only).
- **Persistent import:** staging table primary key `(snapshot_id, tenant_id, shipment_id)`; unique violations are classified as `DUPLICATE_SHIPMENT` without exposing raw PostgreSQL errors.
- **No global unbounded map** of shipment keys during dry-run or import parsing.

## Completion / checksum

SHA-256 over canonical shipment JSON lines (fixed field order, trailing `\n` per line). Manifest and completion records are excluded from checksum. Empty shipment stream checksum: `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`.

## Exporter CLI

`/app/shipment-status-snapshot-export` with `--scope all`, `--tenant`, `--batch-size`, `--format ndjson`, `--output -`. Repository streaming returns `NOT_IMPLEMENTED_EXPORT_QUERY` until PostgreSQL export is implemented.

## Importer CLI

`/app/control-tower-status-snapshot-import` supports `--stdin`, `--dry-run`, `--status`. `--activate`, `--cleanup`, `--rollback` return `NOT_IMPLEMENTED` / `ACTIVATION_CONFIRMATION_REQUIRED` and never modify active projection.

## Dry-run semantics

Dry-run validates the full stream and emits an aggregated JSON report on stdout. No persistent validated job; active projection unchanged; rebuild job/stage tables unchanged.

## Persistent import semantics

Import creates a job, inserts stage rows in bounded batches (separate statements), then marks the job `VALIDATED` or `FAILED`. Batch failure leaves partial stage rows for diagnostics; active projection is unchanged until activation (not implemented). Full import is not a single atomic transaction across all batches.

## Migration objects

- `control_tower.shipment_status_projection_rebuild_job`
- `control_tower.shipment_status_projection_rebuild_stage`
- `control_tower.shipment_status_projection_rebuild_backup`
- Projection provenance columns: `projection_source`, `snapshot_id`, `authoritative_as_of`, `rebuilt_at`

Existing projection rows remain `projection_source=LIVE_EVENT`, `snapshot_id=NULL`. Down migration removes rebuild tables and provenance columns only; projection, inbox, and dead-letter tables are preserved.

## Advisory lock

`ProjectionRebuildAdvisoryLockKey = 0x4354505350524F4A` (`CTPSPROJ`). Helpers use `pg_advisory_xact_lock_shared` / `pg_advisory_xact_lock` within a transaction. Live consumer integration is not wired in v0.1.

## Security

Stream-only by default; no API Gateway routes; credentials from ENV only; stdout reserved for protocol/report.

## Required statements

> The projection rebuild protocol represents an authoritative current-state snapshot and does not fabricate historical Kafka events.

> Kafka consumer offsets are never reset by the exporter or importer.

> The core importer cannot modify the active projection until explicit activation support has been implemented and validated.

> Snapshot data is streamed through standard input and output by default and is not persisted to disk.

## Core limitations (not yet implemented)

- Real PostgreSQL exporter query
- Activation
- Rollback execution
- Live consumer advisory-lock integration
- Kafka catch-up acceptance
- Historical shadow acceptance
- Primary mode (remains blocked)

Rebuild is **not** operationally ready for production cutover in v0.1.
