# Tracking DML 0.1E — writer matrix

Status: inventory of the tree at `3f306fc407aeda4bdc652d825954292f4a54fad6`. No privilege was changed.
`ALL_WRITERS_FENCED=NO`.
`LIVE_TABLE_OWNER=NOT_QUERIED`.
`OPERATOR_SQL=NOT_VERIFIED`.

PR #375 at `9f31b6cfd96a06ece04eb4b946787f9813bfeeb4` is the reference lock contract and is not edited. This matrix is current code. The "after B" column is a design, not a grant.

The running services use the `DATABASE_URL` login. `000103_tms_relationship_pins_security_foundation_v0_1.up.sql:6-7` says `freight` stays the shared login until a later role split. The tracking default login name is `freight` at `services/tracking-service/internal/config/config.go:87`. The credential in that default is not repeated here. A deployment that overrides the login was not read. No production Go path under `services/` executes `SET ROLE`. `shipment_runtime` and `company_runtime` are `NOLOGIN` (`000103:17-32`). They are not the current caller.

No `ALTER TABLE ... OWNER TO` exists for `transport.transport_executions`, `tracking.execution_tracking_state`, `tracking.execution_event_inbox`, or `tracking.execution_stop_eta_state`. The owner is the role that applied `000088` and `000091`. If that role was `freight`, `freight` owns the tables. A table owner is not limited by `GRANT` or `REVOKE`.

No `GRANT` on schema `tracking` was found through `000105`. `000103:50` grants schema `USAGE` on `company`, `transport`, and `core` to `freight_migrations`, `company_runtime`, and `shipment_runtime`.

## 1. Protected state and inbox

| ID | Current code | SQL object | Caller today | Transaction today | Fence bypass today | After option B, not installed |
|---|---|---|---|---|---|---|
| W1 | `UpsertExecutionTrackingState` at `execution_tracking_repository.go:102-137`, called from `execution_consumer.go:123` | `INSERT`/`UPDATE` `tracking.execution_tracking_state` | shared login | tracking transaction begun at `execution_consumer.go:89` | No execution-row lock. No epoch column exists (`000091:45-67`). `RowsAffected` is ignored. `WHERE` at lines 132-133 requires the stored tenant to equal the candidate and the sequence to be older, so this statement does not move tenant A to tenant B. A missing row inserts the candidate tenant. | Runtime has no `INSERT`/`UPDATE`. The protected function performs the write after the execution lock and the epoch check. |
| W2 | `ClearExecutionTarget` at `execution_tracking_repository.go:140-159`, called from `execution_consumer.go:130` | `UPDATE` `tracking.execution_tracking_state` | shared login | same tracking transaction | Same missing execution lock. Zero rows still return nil. The consumer commits at `execution_consumer.go:141`. | Same protected function. Zero rows raise and the caller transaction aborts. |
| W3 | `TouchExecutionSequence` at `execution_tracking_repository.go:162-168`, called from `execution_consumer.go:137` | `UPDATE` `tracking.execution_tracking_state` | shared login | same tracking transaction | Same as W2. | Same protected function. |
| W4 | `MarkApproachEmitted` at `execution_tracking_repository.go:310-323`, called from `approach.go:94` | `UPDATE` `tracking.execution_tracking_state` | shared login | tracking transaction begun at `approach.go:31` | Does not lock `transport.transport_executions`. Does not compare an epoch. `ListActiveTargetsForActors` at `approach.go:69` reads the pool before this write and is not the lock. `approach.go:95-96` returns a nil error when zero rows change, and `approach.go:52` can still commit. | Runtime has no `UPDATE`. A separate fixed function locks the execution row, checks tenant and epoch, then updates the approach columns. Zero rows abort the transaction. |
| W5 | `InsertExecutionInbox` at `execution_tracking_repository.go:73-83`, called from `execution_consumer.go:94` | `INSERT` `tracking.execution_event_inbox` | shared login | same transaction as W1, before the state lock at `execution_consumer.go:107` | The inbox row is inserted before any owner check. Duplicate `event_id` commits at `execution_consumer.go:104-106`. A different tenant commits at lines 111-113. A stale sequence commits at lines 114-116. Offset commit is `execution_event_consumer.go:60-61`, reached from line 102 after Apply returns nil. | Runtime has no `INSERT`. The protected function inserts the inbox only after the execution lock and the tenant and epoch checks, in the same uncommitted transaction as the state effect. A mismatch rolls both back. The Kafka offset is not committed. |

`control_tower.execution_event_inbox` (`000092:6`) is a different table and is not this fence.

## 2. ETA

| ID | Current code | SQL object | Caller today | Transaction today | Fence bypass today | After option B, not installed |
|---|---|---|---|---|---|---|
| W6 | `LockExecutionTrackingStateTx` at `execution_tracking_repository.go:186-193`, called from `eta_ingest_service.go:256` | `SELECT ... FOR UPDATE` on `tracking.execution_tracking_state` | shared login | tracking transaction begun at `eta_ingest_service.go:247` | Locks the state row before any execution row. That is the inverse of the lock order in PR #375. It does not read `transport.transport_executions`. | The ETA function locks the execution row first. Runtime no longer holds `UPDATE` on the state row, so this `FOR UPDATE` is no longer a runtime statement. |
| W7 | `UpsertExecutionStopETATx` at `eta_ingest_service.go:297`, SQL at `execution_tracking_repository.go:253-275` | `INSERT`/`UPDATE` `tracking.execution_stop_eta_state` | shared login | same ETA transaction | Writes ETA state without an execution-row lock and without an epoch check. The state-table lock above does not supply that check. | Runtime has no direct DML on this table. A fixed function writes it only after the execution lock and an equal epoch. |
| W8 | `InsertETAObservationTx` at `eta_ingest_service.go:265`, SQL at `eta_repository.go:53` | `INSERT` `tracking.eta_observation` | shared login | same ETA transaction | An observation is not an `execution_tracking_state` write. It is in the same transaction as W7. | The observation insert stays in the caller transaction that then calls the ETA function. A function failure aborts both. Direct observation insert without the function does not change the protected state table and is not treated as an owner-fence pass. |

## 3. Execution row

| ID | Current code | SQL object | Caller today | Transaction today | Fence bypass today | After option B, not installed |
|---|---|---|---|---|---|---|
| W9 | `insertExecution` at `transport_execution_repository.go:296-301` | `INSERT` `transport.transport_executions` | shared login | shipment projection transaction | Creates the row. No `owner_epoch` column exists. `000088:6-7` says shipment ownership is not moved. That sentence is not a trigger. | The insert sets `owner_epoch = 1` in the same statement. That column is not created here. |
| W10 | revision pointer update at `transport_execution_repository.go:93-96` | `UPDATE` `current_revision_id`, `updated_at` | shared login | same projection transaction | Does not assign `operating_tenant_id`. Does not lock tracking state. | Must not assign `owner_epoch`. The table-level `UPDATE` at `000103:1225` still allows a direct assignment until a later column revoke. |
| W11 | successor pointer update at `transport_execution_successor_repository.go:173-176` | `UPDATE` `current_revision_id`, `updated_at` | shared login | shipment transaction | Same as W10. The `WHERE` clause is only `id`. | Same constraint as W10. |
| W12 | superseded event counter at `transport_execution_successor_repository.go:458-461` | `UPDATE` `event_seq`, `updated_at` | shared login | shipment transaction | `event_seq` is not an owner epoch. | Must not change `owner_epoch`. |
| W13 | `emitExecutionEvent` at `transport_execution_command_repository.go:1225-1228` | `UPDATE` `event_seq`, `updated_at` | shared login | shipment command transaction | Same as W12. The payload is built from this transaction at lines 1232-1239 and has no epoch field today. | A later payload copies `owner_epoch` from the locked row. This stage does not change that Go file. |
| W14 | second event counter at `transport_execution_command_repository.go:1343-1346` | `UPDATE` `event_seq`, `updated_at` | shared login | shipment command transaction | Same as W12. | Same constraint as W12. |

`shipment_runtime` has table-level `SELECT, INSERT, UPDATE` on `transport.transport_executions` at `000103:1225`. `freight_migrations` has `SELECT` only at `000103:1223`. Neither role is a current production caller. `000103:399-414` rejects a change of `transport.transport_execution_revisions.operating_tenant_id` with `committed revision identity is immutable`. That trigger does not cover `transport.transport_executions.operating_tenant_id`.

## 4. Not production writers

These statements are tests. A test pool that connects as the table owner bypasses a future revoke.

- `services/tracking-service/internal/integration/driverlocation/execution_stop_tracking_integration_test.go:248`, `:265`, `:550`, and `:603`
- `services/shipment-service/internal/integration/transportexecution/projection_integration_test.go:191`
- `services/shipment-service/internal/integration/transportexecution/driver_stop_integration_test.go:239` and `:1089`
- `services/document-service/internal/integration/podaction/harness_test.go:227`
- company-service relationship tests under `services/company-service/internal/integration/relationship/`

`SET ROLE shipment_runtime` at `activation_runtime_integration_test.go:637` and `SET ROLE` in `postgres_security_foundation_helpers_test.go:293` are tests. They are not production callers.

No production `DELETE` of `tracking.execution_tracking_state`, `tracking.execution_event_inbox`, or `tracking.execution_stop_eta_state` was found under `services/tracking-service`.

`UpsertTrackingStateIfNewerTx` at `approach.go:46` writes `tracking.shipment_tracking_state` (`tracking_repository.go:162`). That is a different table. It is not an execution-owner writer. It remains outside this fence.

## 5. Roles that are not a fenced writer

| Actor | What the tree shows | Bypass that remains |
|---|---|---|
| Kafka consumer | W1, W2, W3, W5. Group default `tracking-execution-stop` at `config.go:136`. | Direct state and inbox DML. Apply nil leads to `CommitRecords`. |
| ETA ingest | W6, W7, W8. | ETA state write without an execution lock. Lock-order inversion. |
| `MarkApproachEmitted` | W4. | State update without an execution lock. Zero rows do not abort the transaction. |
| Operator SQL | No statement in this repository. | `NOT_VERIFIED`. Any session as the table owner can write. |
| `freight_migrations` | `NOLOGIN`. `SELECT` on `transport.transport_executions` at `000103:1223`. No tracking DML grant was found. | Not a current writer. A later grant would be a new path. |
| Shared login `freight` | Documented shared login at `000103:6-7`. Default tracking login name at `config.go:87`. | If it owns the tables, revoke does not stop it. `SHARED_FREIGHT_RISK=OPEN`. |
