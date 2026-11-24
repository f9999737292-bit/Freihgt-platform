# Tracking owner fence 0.1D — transaction contract

Status: `SAME_DATABASE_OWNER_FENCE` is designed and not approved for implementation.
`SHIPMENT_OWNER_RACE=OPEN_UNTIL_IMPLEMENTED`.
`LOCK_ORDER_DEFINED=YES`.
`ORDER=EXECUTION_LOCK → OWNER_EPOCH_CHECK → INBOX_INSERT → STATE_EFFECT_AND_PROOF → COMMIT`.
`INBOX_BEFORE_STATE_PROOF=YES`.
`SAME_TRANSACTION=REQUIRED`.
`FAILED_STATE_UPDATE_ROLLBACKS_INBOX=YES`.
`OFFSET_COMMIT_AFTER_DURABLE_EFFECT=YES`.
`OWNER_EPOCH_LINEARIZATION_POINT=COMMIT_OF_THE_TRANSACTION_THAT_HOLDS_THE_EXECUTION_ROW_AND_STORES_OWNER_EPOCH`.

PR #374 at `89b509797ae61558582f4c198647ba13d21d9a47` records that the two schemas share database `freight_platform` and that two sessions do not make one transaction. This contract uses that shared database. It does not claim the current code follows the order below.

## 1. Required order

Every production writer of `tracking.execution_tracking_state`, and every ETA write that depends on that row, uses one tracking transaction. The state effect follows the inbox insert. Both are in the same uncommitted transaction.

1. `BEGIN`.
2. `SET TRANSACTION ISOLATION LEVEL READ COMMITTED`.
3. `SET LOCAL lock_timeout = '2s'`. The duration is a proposed bound, not a measured coordinator or database limit.
4. Call `transport.lock_execution_owner(execution_id)`. This is the authoritative execution lock. The function takes `SELECT ... FOR UPDATE` on `transport.transport_executions` and returns the stored `operating_tenant_id` and `owner_epoch`. The lock lasts until this transaction commits or rolls back. The function contains no `COMMIT`.
5. Check tenant and owner epoch against that return. The event epoch must equal the locked epoch. The tenant claim must equal the locked tenant. A value observed before this call, including `ListActiveTargetsForActors`, is not the decision value. The payload tenant and `X-Tenant-ID` are claims. A mismatch is `OWNER_STATE_MISMATCH`: roll back, with no state effect, no new inbox row, and no offset commit.
6. Check the existing tracking state and the event identity. `SELECT ... FROM tracking.execution_tracking_state WHERE execution_id = $1 FOR UPDATE`. Compare `event_id`, execution id, sequence, tenant, hash, size, and event epoch with the locked execution and, when a state row exists, with that locked state row. This statement locks the state row. It does not write it. The execution row is already locked, so the lock order stays execution, then tracking state.
7. Check conflicting inbox rows. An existing row with this `event_id` uses the replay path in step 8. A different `event_id` with the same execution and sequence conflicts. Roll that transaction back. Do not overwrite either row.
8. When `event_id` is new, insert the inbox row in this same uncommitted transaction. The insert records `event_id`, tenant, execution, sequence, event type, SHA-256, size, and epoch. The insert is not a terminal confirmation, and it is not committed by itself. When `event_id` already exists, do not insert again and do not overwrite the stored row. Replay is a separate read of that committed inbox row and of the state row. Hash equality alone is not `TERMINAL`. A prior effect is verified only when the existing inbox row and `last_event_id` agree on event id, tenant, execution, sequence, hash, and size, and the locked epoch equals the stored epoch. A verified prior effect skips the state write in steps 9 and 10. Anything short of that proof is `NON_TERMINAL` and rolls back at this step, with no offset commit.
9. For a new `event_id` only, apply the conditional state effect after the inbox insert. The same statement writes `last_event_id`. An immediate foreign key from `tracking.execution_tracking_state.last_event_id` to `tracking.execution_event_inbox.event_id` then sees the inbox row inside this transaction. The reference is valid only when that inbox row has the same tenant, the same execution, and the same event identity. A UUID that exists for another tenant or another execution is not proof. A verified prior effect does not write the state row again.
10. For that new effect, require `RowsAffected > 0`. The written sequence equals this event's sequence. The written `owner_epoch` equals the locked epoch. `last_event_id` equals this event. Zero rows are not a state effect. A nil driver error is not a state effect.
11. Re-read the state row in this transaction before commit. `last_event_id`, tenant, execution, sequence, and epoch must match the locked execution and the inbox row.
12. `COMMIT`. For a new effect, the inbox insert and the state effect become durable together. Neither remains without the other. For a verified prior effect, commit adds no inbox row and changes no state row.
13. Only after that commit returns may the consumer call `CommitRecords`.

If the state update fails, or changes zero rows, the transaction rolls back. The new inbox insert does not remain. The offset is not committed.

Shipment's epoch change is an `UPDATE` of the same execution row in a shipment transaction. That statement locks the row. It uses no tracking lock unless it also writes tracking state, in which case it takes the execution lock first and the state lock second.

## 2. Linearization point

The linearization point is the commit of the transaction that holds `transport.transport_executions` `FOR UPDATE` and stores `owner_epoch`.

Under `READ COMMITTED`, the `FOR UPDATE` wait ends on the latest committed row. The function return is that row. A tracking transaction that acquired the lock while the epoch was 4 can commit a state effect for epoch 4. A shipment transaction waiting on that lock then commits epoch 5. The epoch-4 effect remains. This contract does not undo it.

A tracking transaction that acquires the lock after epoch 5 has committed reads 5. An event that carries 4 does not write. There is no committed state effect for epoch 4 after the epoch-5 commit.

## 3. Cases

| Case | Result |
|---|---|
| No execution row | The function returns no row. Roll back. `NON_TERMINAL`. No inbox row. No offset commit. |
| No tracking row | After the lock, the epoch check, and the inbox insert, the state insert writes the locked epoch and `last_event_id`. If that insert fails, rollback removes the inbox row. |
| Existing tracking row, same tenant, newer sequence, equal epoch | Inbox insert first, then the conditional update. `RowsAffected > 0`. Re-read before commit. Tenant, execution, and event identity match. A UUID match alone is not enough. |
| Tenant on the event differs from the locked tenant | Rejected after the lock and before the inbox insert. Roll back. No new inbox row. No state update. No offset commit. |
| Event epoch differs from the locked epoch | Roll back before the inbox insert. A higher event epoch is not accepted as authority. |
| Existing `event_id` | No second insert and no overwrite. Hash equality alone is not terminal. A complete prior proof may commit the read and then allow one offset confirmation. A partial match rolls back with no offset commit. |
| `RowsAffected = 0` | Roll back the whole transaction, including a new inbox insert. A nil driver error is not a state effect. |
| Lock wait exceeds `lock_timeout` | The statement fails with `55P03`. Roll back. `NON_TERMINAL`. No offset commit. |
| Deadlock `40P01` | Roll back. `NON_TERMINAL`. The prescribed order is execution row, then tracking state. |
| Null epoch on the locked row or on the event | Roll back. `NON_TERMINAL`. Null is not stored as `1`. |
| Crash after the lock and before commit | PostgreSQL ends the session transaction. The lock is released. State and inbox stay as they were. The offset is not committed. |
| State update fails after the lock, including `RowsAffected = 0` | Roll back the whole transaction. The inbox insert does not remain. The offset is not committed. |
| Crash after commit and before `CommitRecords` | The proof remains. Replay uses that proof. The offset stays uncommitted until a later attempt sees the proof and then commits the offset once. |
| Isolation other than `READ COMMITTED` | Not this contract. A snapshot taken before the lock is not a second source of truth. |

## 4. Counterexamples the order closes

- The current consumer reads shipment over HTTP at `execution_consumer.go:77-87` and writes state later in another transaction. That read is not the execution lock.
- A second call to the lock function in a new transaction releases the first lock before the state write. The state write must stay in the transaction that holds the lock.
- `pg_advisory_xact_lock` on the tracking session does not lock `transport.transport_executions`.
- An old consumer that calls `UpsertExecutionTrackingState` directly skips the function. The security document records that this binary stays behind the closed mixed-group gate, and that returning it is a separate controller action.

## 5. Deadlock

The consumer locks the execution row and then waits for the state row. ETA today locks the state row first at `eta_ingest_service.go:256`. If that path then waits for the execution row, the two transactions deadlock.

The contract puts the execution lock first on the consumer, on ETA ingest, and on `MarkApproachEmitted`. Shipment updates that only touch the execution row do not take the state lock, so they wait or are waited on without a second resource. A path that locks the state row first fails this contract even if it later calls the function.
