# Tracking owner fence 0.1D — writer inventory

Status: inventory of the tree at `3f306fc407aeda4bdc652d825954292f4a54fad6`.
Migration head in that tree is `infrastructure/migrations/000105_platform_postgres_role_foundation_v0_2_i1.up.sql`.
This document does not change SQL.

The session role of the production repository methods is the `DATABASE_URL` login. Migration `000103_tms_relationship_pins_security_foundation_v0_1.up.sql:6-7` states that `freight` stays the shared login until a later role split. `shipment_runtime` is `NOLOGIN` at `000103:31`. No production Go path in `services/` executes `SET ROLE`. A deployment that points `DATABASE_URL` at another database, or that sets a role outside this repository, is `NOT_VERIFIED`.

`transport.transport_executions` and `tracking.execution_tracking_state` are schemas in one migration chain. They are not one transaction. Each service calls `pool.Begin` on its own pool.

## 1. Production writers of `transport.transport_executions`

| ID | Service | Statement | Role | Transaction | Locks | Owner-fence bypass |
|---|---|---|---|---|---|---|
| S1 | shipment-service | `INSERT` at `services/shipment-service/internal/repository/transport_execution_repository.go:296-301` | shared login | caller transaction | new-row lock | Creates the row. No `owner_epoch` column exists. |
| S2 | shipment-service | `UPDATE` of `current_revision_id`, `updated_at` at `transport_execution_repository.go:92-96` | shared login | caller transaction | updated row | Does not assign `operating_tenant_id`. Does not lock tracking state. |
| S3 | shipment-service | `UPDATE` of `current_revision_id`, `updated_at` at `services/shipment-service/internal/repository/transport_execution_successor_repository.go:172-176` | shared login | caller transaction | updated row | Same as S2. |
| S4 | shipment-service | `UPDATE` of `event_seq` at `transport_execution_successor_repository.go:457-461` | shared login | caller transaction | updated row | Increments `event_seq`. That counter is not an owner epoch. |
| S5 | shipment-service | `UPDATE` of `event_seq` at `services/shipment-service/internal/repository/transport_execution_command_repository.go:1224-1228` | shared login | caller transaction | updated row | Same as S4. The payload is built after the update at `1232-1244`. |
| S6 | shipment-service | `UPDATE` of `event_seq` at `transport_execution_command_repository.go:1342-1346` | shared login | caller transaction | updated row | Same as S4. |

No production statement in `services/` assigns `transport.transport_executions.operating_tenant_id` after the insert. The column can still be named in a later migration. These updates lock the execution row only for their own transaction, and that transaction does not include `tracking.execution_tracking_state`.

## 2. Production writers of `tracking.execution_tracking_state`

| ID | Service | Statement | Role | Transaction | Locks | Owner-fence bypass |
|---|---|---|---|---|---|---|
| T1 | tracking-service | `INSERT`/`ON CONFLICT UPDATE` at `services/tracking-service/internal/repository/execution_tracking_repository.go:102-137`, called from `services/tracking-service/internal/service/execution_consumer.go:123` | shared login | `Begin` at `execution_tracking_repository.go:69-70`, entered at `execution_consumer.go:89` | `SELECT FOR UPDATE` of the state row at `execution_tracking_repository.go:86-93` | The HTTP read at `execution_consumer.go:77-87` happens before `Begin`. No execution-row lock. `RowsAffected` is ignored. A tenant mismatch changes zero rows and still returns nil. |
| T2 | tracking-service | `UPDATE` at `execution_tracking_repository.go:140-159`, called from `execution_consumer.go:130` | shared login | same consumer transaction | row lock from `UPDATE` | No execution-row lock. Zero rows still return nil. |
| T3 | tracking-service | `UPDATE` of `last_execution_event_sequence` at `execution_tracking_repository.go:162-168`, called from `execution_consumer.go:137` | shared login | same consumer transaction | row lock from `UPDATE` | Same as T2. |
| T4 | tracking-service | `UPDATE` at `execution_tracking_repository.go:310-323`, called from `services/tracking-service/internal/service/approach.go:94` | shared login | `Begin` at `approach.go:31` | row lock from `UPDATE` | `ListActiveTargetsForActors` at `approach.go:69` reads state on the pool, outside this transaction. No execution-row lock. |
| T5 | tracking-service | `INSERT` of `tracking.execution_event_inbox` at `execution_tracking_repository.go:73-83`, called from `execution_consumer.go:94` | shared login | same consumer transaction | new inbox row | Inbox identity can commit today without an epoch check. The 0.1D contract rolls that insert back unless the fence commits. |

`last_execution_event_sequence` is written by T1, T2, and T3. T4 does not assign it. The column is not an owner epoch.

The Kafka consumer commits the offset only after `Apply` returns nil. `commitRecord` calls `CommitRecords` at `services/tracking-service/internal/service/execution_event_consumer.go:60-61`, from `consumeBatch` at `execution_event_consumer.go:102`. That broker call is a second session after the database commit. It is not inside the database transaction.

## 3. ETA ingest

ETA ingest begins a tracking transaction at `services/tracking-service/internal/service/eta_ingest_service.go:247`. It locks the execution state row at `eta_ingest_service.go:256` through `LockExecutionTrackingStateTx` (`execution_tracking_repository.go:186-189`). It then inserts an ETA observation and upserts `tracking.execution_stop_eta_state` at `eta_ingest_service.go:297`. The commit is at `eta_ingest_service.go:302`.

This path does not assign `tracking.execution_tracking_state`. It still reads that row and writes execution ETA state without locking `transport.transport_executions`. It locks the tracking state row before any execution-row lock. Under the 0.1D order that is a lock-order inversion and an epoch bypass.

## 4. Writers that are not the execution owner row

`UpsertTrackingStateIfNewerTx` at `services/tracking-service/internal/repository/tracking_repository.go:153` writes `tracking.shipment_tracking_state`. `approach.go:46` calls it in the same transaction as T4. That table is not `tracking.execution_tracking_state`. It is not an owner-epoch writer for an execution.

`tracking.event_outbox` updates in `services/tracking-service/internal/outbox/kafka_publisher.go:70` publish tracking events. They do not change execution ownership.

## 5. Test-only SQL

Integration tests execute `INSERT` or `UPDATE` on these tables through a test pool. Examples:

- `services/tracking-service/internal/integration/driverlocation/execution_stop_tracking_integration_test.go:248`, `:265`, `:550`, and `:603`
- `services/shipment-service/internal/integration/transportexecution/projection_integration_test.go:191`
- `services/company-service/internal/integration/relationship/postgres_security_foundation_integration_test.go:179` and later updates in that file
- `services/document-service/internal/integration/podaction/harness_test.go:227`

Those statements are fixtures. They are not production writers. A test pool can bypass a future fence. The runtime contract does not treat them as authorized writers.

## 6. Not verified

`NOT_VERIFIED` covers SQL issued by an operator, a migration runner outside this inventory, or a process whose connection string is not the repository compose `DATABASE_URL`. The search of `services/` and `infrastructure/migrations` did not find another production `INSERT` or `UPDATE` of `tracking.execution_tracking_state`.
