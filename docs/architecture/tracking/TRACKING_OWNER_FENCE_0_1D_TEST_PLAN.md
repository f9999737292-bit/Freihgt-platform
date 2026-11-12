# Tracking owner fence 0.1D — test plan

Status: designed, not executed.
`CROSS_TENANT_TESTS_DEFINED=YES`.
`CRASH_RECOVERY_TESTS_DEFINED=YES`.
`BROKER_SAFETY_PROVEN=NO`.

These tests need one PostgreSQL database that contains schema `transport` and schema `tracking`, plus the future `owner_epoch` column and `transport.lock_execution_owner`. They are not authorized by this stage. They do not use staging or production. A broker assertion is limited to `OffsetFetch` after the database result. Coordinator apply time stays unproven.

Each test records the SQL state before the stimulus and after the sessions end. `PASS` and `FAIL` use those rows.

## 1. Epoch 4, then epoch 5

Setup: execution `owner_epoch = 4`. Tracking state `owner_epoch = 4` for an earlier event.
Stimulus: session A calls `lock_execution_owner` and sees 4. Session B updates the epoch to 5 and commits only after A commits a state write for epoch 4.
Expected SQL: execution `owner_epoch = 5`. The state row for the epoch-4 event remains `owner_epoch = 4`. No later update stores `owner_epoch = 4`.
`PASS`: B's commit is after A's commit, and the epoch-4 row was not written again after B committed.
`FAIL`: a state row shows `owner_epoch = 4` from a transaction that acquired the execution lock after B committed.

## 2. Tracking transaction before the owner transition

Setup: execution epoch 4. No epoch-5 update has started.
Stimulus: the tracking transaction locks the execution, writes the matching event, and commits. The shipment transaction then sets epoch 5 and commits.
Expected SQL: state `owner_epoch = 4`, `last_event_id` equals the event, inbox contains that `event_id`. Execution epoch is then 5.
`PASS`: the epoch-4 effect is still present after epoch 5. It is not deleted.
`FAIL`: the epoch-5 commit removes or rewrites the epoch-4 proof.

## 3. Tracking transaction after the owner transition

Setup: execution epoch is already 5. The event carries epoch 4.
Stimulus: the tracking transaction calls the lock function and then attempts the state write.
Expected SQL: execution epoch stays 5. State `owner_epoch` stays at its previous value. Inbox count for the new `event_id` is 0.
`PASS`: the transaction rolls back.
`FAIL`: the inbox gains a row or the state epoch becomes 4.

## 4. Parallel writers

Setup: execution epoch 4. Two tracking sessions carry the same epoch-4 event identity, and one shipment session sets epoch 5.
Stimulus: all three start together. Tracking sessions use execution-then-state order.
Expected SQL: one inbox row for that `event_id` or zero if both lose the race to epoch 5. State `owner_epoch` is 4 only when a tracking commit happened before epoch 5. Otherwise it is unchanged. Execution epoch ends at 5.
`PASS`: no second inbox row, and no epoch-4 write is ordered after the epoch-5 commit.
`FAIL`: two inbox rows, or a state write of epoch 4 after epoch 5 is visible.

## 5. No tracking state row

Setup: execution epoch 1 and no `tracking.execution_tracking_state` row.
Stimulus: a matching event runs the full order.
Expected SQL: one state row with `owner_epoch = 1`, `last_event_id` equal to the event, and one inbox row with the same id, hash, and size.
`PASS`: the insert follows a lock return for that execution.
`FAIL`: the insert runs when the function returns no execution row.

## 6. Tenant A against tenant B

Setup: execution `operating_tenant_id` is tenant A and epoch is 4. The event claims tenant B and epoch 4.
Stimulus: the tracking transaction locks the execution and compares the claim.
Expected SQL: state `operating_tenant_id` stays A. Inbox has no row for the event. `RowsAffected` of any state update is 0 before rollback.
`PASS`: both sessions end with the pre-test state row unchanged.
`FAIL`: the stored tenant becomes B, or the offset advances.

## 7. Deadlock and lock timeout

Setup: session A holds the execution lock and waits for the state row. Session B holds the state row and then calls `lock_execution_owner`.
Stimulus: the inverted order, which this contract forbids, and a second run in the prescribed order with `lock_timeout = '2s'`.
Expected SQL: on the inverted run, PostgreSQL reports `40P01` or a lock timeout, and both transactions roll back. On the prescribed run, one session waits and then reads the committed epoch. No `40P01`.
`PASS`: the forbidden order leaves no partial state row. The prescribed order completes without `40P01`.
`FAIL`: the prescribed order deadlocks, or either run commits a state row whose epoch differs from the execution epoch at that commit.

## 8. Crash before commit

Setup: the tracking transaction has locked the execution and has not committed.
Stimulus: the session is terminated.
Expected SQL: state and inbox match the pre-test counts. Execution epoch is unchanged by the dead session.
`PASS`: the next lock sees the pre-test epoch and no new inbox row.
`FAIL`: a new inbox row or a new `last_event_id` is present.

## 9. Crash after commit and before the Kafka offset

Setup: the tracking transaction has committed the proof. `CommitRecords` has not been called.
Stimulus: the process stops. `OffsetFetch` is read after restart.
Expected SQL: inbox and state contain the proof. `OffsetFetch` remains on the pre-test offset.
`PASS`: restart does not insert a second inbox row. The offset advances once only after the proof is read.
`FAIL`: the offset advances without a matching `last_event_id`, hash, size, and epoch.

## 10. Restart and replay

Setup: the proof from test 9 is committed and the offset is still old.
Stimulus: a new process loads the same record.
Expected SQL: inbox count stays 1. State `last_event_id` stays equal to the event. After the proof check, `OffsetFetch` shows the next offset.
`PASS`: one offset advance and no second state update.
`FAIL`: the offset advances while `last_event_id` is null, or the same record advances the offset twice.

## 11. ETA ingest bypass

Setup: state `owner_epoch = 4` and execution `owner_epoch = 5`. ETA ingest is required to call `lock_execution_owner` before `LockExecutionTrackingStateTx`.
Stimulus: an ETA observation for that execution.
Expected SQL: `tracking.execution_stop_eta_state` is unchanged. The tracking state tenant and epoch are unchanged.
`PASS`: the ETA transaction rolls back because the locked execution epoch differs from the state epoch.
`FAIL`: `execution_stop_eta_state` changes while those epochs differ. The current code at `eta_ingest_service.go:256` is this failure mode until it takes the execution lock first.

## 12. Old consumer bypass

Setup: the group has one member that writes through `UpsertExecutionTrackingState` and does not call `lock_execution_owner`.
Stimulus: the deployment gate evaluates the group, and the old session attempts a state update after execution epoch 5.
Expected SQL: the new mode writes nothing. If the old session is still connected, its update is a bypass and the gate result is closed.
`PASS`: the new consumer does not join, and no new-mode transaction commits.
`FAIL`: the new mode runs beside the old session, or the gate treats that session as compatible.

`MarkApproachEmitted` is covered by the same rule as test 11. A call at `approach.go:94` that updates state after the execution epoch has moved, without holding `lock_execution_owner` in that transaction, is a `FAIL`.
