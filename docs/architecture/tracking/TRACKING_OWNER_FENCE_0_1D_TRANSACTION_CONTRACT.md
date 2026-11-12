# Tracking owner fence 0.1D — transaction contract

Status: `SAME_DATABASE_OWNER_FENCE` is designed and not approved for implementation.
`SHIPMENT_OWNER_RACE=OPEN_UNTIL_IMPLEMENTED`.
`LOCK_ORDER_DEFINED=YES`.
`OWNER_EPOCH_LINEARIZATION_POINT=COMMIT_OF_THE_TRANSACTION_THAT_HOLDS_THE_EXECUTION_ROW_AND_STORES_OWNER_EPOCH`.

PR #374 at `89b509797ae61558582f4c198647ba13d21d9a47` records that the two schemas share database `freight_platform` and that two sessions do not make one transaction. This contract uses that shared database. It does not claim the current code follows the order below.

## 1. Required order

Every production writer of `tracking.execution_tracking_state`, and every ETA write that depends on that row, uses one tracking transaction:

1. `BEGIN`.
2. `SET TRANSACTION ISOLATION LEVEL READ COMMITTED`.
3. `SET LOCAL lock_timeout = '2s'`. The duration is a proposed bound, not a measured coordinator or database limit.
4. Call `transport.lock_execution_owner(execution_id)`. The function takes `SELECT ... FOR UPDATE` on `transport.transport_executions` and returns the stored `operating_tenant_id` and `owner_epoch`. The lock lasts until this transaction commits or rolls back. The function contains no `COMMIT`.
5. Read the trusted owner and epoch from that return value. A value observed before this call is not the decision value.
6. Validate event identity: `event_id`, execution id, sequence, tenant claim, hash, size, and event epoch. The event epoch must equal the locked epoch. The tenant claim must equal the locked tenant. The caller header is not the authority.
7. `SELECT ... FROM tracking.execution_tracking_state WHERE execution_id = $1 FOR UPDATE`.
8. Apply the state effect. `RowsAffected` must be greater than zero. The written `owner_epoch` equals the locked epoch. `last_event_id` equals this event. `last_execution_event_sequence` equals this event's sequence.
9. Persist the event proof in the same transaction: inbox `event_id`, tenant, execution, sequence, event type, SHA-256, size, and epoch.
10. `COMMIT`.
11. Only after that commit returns, the consumer may call `CommitRecords`.

Shipment's epoch change is an `UPDATE` of the same execution row in a shipment transaction. That statement locks the row. It uses no tracking lock unless it also writes tracking state, in which case it takes the execution lock first and the state lock second.

## 2. Linearization point

The linearization point is the commit of the transaction that holds `transport.transport_executions` `FOR UPDATE` and stores `owner_epoch`.

Under `READ COMMITTED`, the `FOR UPDATE` wait ends on the latest committed row. The function return is that row. A tracking transaction that acquired the lock while the epoch was 4 can commit a state effect for epoch 4. A shipment transaction waiting on that lock then commits epoch 5. The epoch-4 effect remains. This contract does not undo it.

A tracking transaction that acquires the lock after epoch 5 has committed reads 5. An event that carries 4 does not write. There is no committed state effect for epoch 4 after the epoch-5 commit.

## 3. Cases

| Case | Result |
|---|---|
| No execution row | The function returns no row. Roll back. `NON_TERMINAL`. No inbox row. No offset commit. |
| No tracking row | After the lock and the identity check succeed, the insert creates the row with the locked epoch. |
| Existing tracking row, same tenant, newer sequence, equal epoch | Update. `RowsAffected > 0`. Re-read before commit. |
| Tenant on the event differs from the locked tenant | Roll back. The `WHERE` tenant predicate also changes zero rows if a write is attempted. Zero rows are `NON_TERMINAL`. |
| Event epoch differs from the locked epoch | Roll back. A higher event epoch is not accepted as authority. |
| `RowsAffected = 0` | Roll back. A nil driver error is not a state effect. |
| Lock wait exceeds `lock_timeout` | The statement fails with `55P03`. Roll back. `NON_TERMINAL`. No offset commit. |
| Deadlock `40P01` | Roll back. `NON_TERMINAL`. The prescribed order is execution row, then tracking state. |
| Null epoch on the locked row or on the event | Roll back. `NON_TERMINAL`. Null is not stored as `1`. |
| Crash after the lock and before commit | PostgreSQL ends the session transaction. The lock is released. State and inbox stay as they were. The offset is not committed. |
| State update fails after the lock, including `RowsAffected = 0` | Roll back. The inbox insert does not remain. The offset is not committed. |
| Crash after commit and before `CommitRecords` | The proof remains. Replay uses that proof. The offset stays uncommitted until a later attempt sees the proof and then commits the offset once. |
| Isolation other than `READ COMMITTED` | Not this contract. A snapshot taken before the lock is not a second source of truth. |

A future foreign key from `tracking.execution_tracking_state.last_event_id` to `tracking.execution_event_inbox.event_id` requires the inbox insert to precede the state update inside the same uncommitted transaction. The inbox row is visible to that foreign key before commit. If the state update fails, both the inbox insert and the state change roll back together. The inbox insert is not committed on its own.

## 4. Counterexamples the order closes

- The current consumer reads shipment over HTTP at `execution_consumer.go:77-87` and writes state later in another transaction. That read is not step 4.
- A second call to the lock function in a new transaction releases the first lock before the state write. The state write must stay in the transaction that holds the lock.
- `pg_advisory_xact_lock` on the tracking session does not lock `transport.transport_executions`.
- An old consumer that calls `UpsertExecutionTrackingState` directly skips the function. The security document records that this binary stays behind the closed mixed-group gate, and that returning it is a separate controller action.

## 5. Deadlock

T1 locks the execution row and waits for the state row. ETA today locks the state row first at `eta_ingest_service.go:256`. If that path then waits for the execution row, the two transactions deadlock.

The contract puts the execution lock first on the consumer, on ETA ingest, and on `MarkApproachEmitted`. Shipment updates that only touch the execution row do not take the state lock, so they wait or are waited on without a second resource. A path that locks the state row first fails this contract even if it later calls the function.
