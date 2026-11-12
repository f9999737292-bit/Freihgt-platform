# Tracking DML 0.1E — test plan

Status: designed, not executed.
`TESTS_EXECUTED=NO`.
`BROKER_SAFETY_PROVEN=NO`.

These tests need a disposable PostgreSQL 16 database. They do not use staging or production. They do not run in this stage. Each test records the SQL rows before the stimulus and after the sessions end. `PASS` and `FAIL` use those rows. A broker check is `OffsetFetch` after the database result. Coordinator apply time stays unproven.

The protected functions and the epoch trigger are part of the fixture. They are not part of this repository yet.

## 1. Authorized state write

Setup: execution tenant A, `owner_epoch = 5`. No state row. A new `event_id`.
Stimulus: `tracking_service_login` calls `tracking.apply_route_stop_current` in one transaction and commits.
Expected: one inbox row and one state row. Tenant A, epoch 5, `last_event_id` equal to the event, sequence equal to the event. `OffsetFetch` advances only after that commit.
`PASS`: both rows match, and the offset was still the old value until the commit returned.
`FAIL`: the state row exists without the inbox row, the epoch differs, or the offset advanced before the commit.

## 2. Unauthorized direct update

Setup: the same login as test 1, after the DML revoke.
Stimulus: `UPDATE tracking.execution_tracking_state SET updated_at = now()`.
Expected: `42501`. The row is unchanged.
`PASS`: the error is `42501` and the row is unchanged.
`FAIL`: the statement succeeds.

## 3. Cross-tenant claim

Setup: locked execution tenant A, epoch 5. The event claims tenant B and epoch 5.
Stimulus: the protected call.
Expected: `OWNER_STATE_MISMATCH`. Rollback. No new inbox row. No state change. No offset commit.
`PASS`: the three stored values match the setup.
`FAIL`: an inbox row remains, the state tenant changes, or the offset advances.

## 4. Stale epoch

Setup: execution epoch 5. The event carries epoch 4. State, if present, stays at its previous epoch.
Stimulus: the protected call.
Expected: `STALE_EPOCH`. Rollback. No new inbox row. No state write. No offset commit. A second event that carries epoch 6 is the same failure. A higher event epoch is not authority.
`PASS`: execution epoch remains 5 and no new proof row exists.
`FAIL`: state epoch becomes 4 or 6, or an inbox row remains.

## 5. Concurrent owner change

Setup: session A has locked the execution at epoch 4 and has not committed. Session B is the epoch function, waiting to set epoch 5.
Stimulus: A commits an epoch-4 effect. B then commits epoch 5. A second tracking transaction then tries the epoch-4 event again.
Expected: the epoch-4 effect remains after B commits. The second attempt raises `STALE_EPOCH` and does not write.
`PASS`: one epoch-4 proof, execution epoch 5, and no later epoch-4 write.
`FAIL`: the epoch-5 commit deletes the epoch-4 proof, or the second attempt writes.

## 6. Concurrent tracking writers

Setup: two sessions, one new event, execution epoch 4.
Stimulus: both call the same protected function.
Expected: one inbox row. One state update. The loser hits the inbox primary key or the state predicate and rolls back.
`PASS`: one inbox row and one state row.
`FAIL`: two inbox rows, or a committed state row whose epoch is not 4.

## 7. Shared privileged login

Setup: a session whose `session_user` is the owner of `tracking.execution_tracking_state`. The fixture records that owner. This stage does not know the live owner.
Stimulus: direct `UPDATE` of the state row, and a call to the protected function.
Expected observation: the direct `UPDATE` succeeds because the owner is not limited by the revoke. The function call is specified to fail `USAGE` when that login is not a member of `tracking_runtime`.
`PASS` is not defined as proof of least privilege. A rejected function call does not prove the owner lacks DML.
`FAIL`: the report treats this session as evidence that the revoke fences every writer.

## 8. SET ROLE

Setup: a login that is a member of `tracking_runtime` `WITH INHERIT FALSE, SET TRUE`.
Stimulus: read `session_user`, `current_user`, and `pg_has_role` for `MEMBER` and `USAGE` before and after `SET ROLE tracking_runtime`, then call the function.
Expected from the PostgreSQL 16 wording cited by PR #375: `SET ROLE` does not change `session_user`, and it does not make `USAGE` true for that login. The body is specified to reject the call. Whether the server does so was not executed.
`PASS`: the booleans match that wording and no state row is committed by this login.
`FAIL`: `USAGE` becomes true because of `SET ROLE`, or a state row commits.
`SET_ROLE_BYPASS=NOT_CLAIMED_SAFE_WITHOUT_TEST`.

## 9. Inherited privileges

Setup: `tracking_service_login` with `INHERIT TRUE` and `SET FALSE`.
Stimulus: call the function without `SET ROLE`. Then attempt direct table DML.
Expected: the function call is accepted and the direct DML returns `42501`.
`PASS`: both results.
`FAIL`: the call requires `SET ROLE`, or the direct DML succeeds.

## 10. Public execute

Setup: an unrelated login that has `USAGE` on schema `transport` and schema `tracking` and is not a member of `tracking_runtime`.
Stimulus: call every function in both schemas, and read `proacl`.
Expected: `42501` on every non-trigger function except none for this login. Trigger functions are recorded separately because a direct call fails as a trigger function even when `EXECUTE` remains granted to `PUBLIC`.
`PASS`: the unrelated login executes nothing, and `PUBLIC` cannot execute the new functions.
`FAIL`: any non-trigger function executes, or `PUBLIC` can execute a protected function.
This test is the catalog audit. `PUBLIC_EXECUTE_AUDIT=REQUIRED` until it has a result.

## 11. Crash before commit

Setup: the function has locked the execution and has not committed.
Stimulus: terminate the session.
Expected: no new inbox row, no new `last_event_id`, execution epoch unchanged, offset unchanged.
`PASS`: the next lock sees the pre-test rows.
`FAIL`: a new inbox row or a new state proof is present.

## 12. Crash after commit

Setup: the function's transaction has committed. `CommitRecords` has not been called.
Stimulus: stop the process. Read `OffsetFetch`.
Expected: the proof remains. The offset stays at the pre-test value until a later attempt sees the proof and commits the offset once.
`PASS`: one later offset advance and no second inbox row.
`FAIL`: the offset advances without the proof, or the same record advances it twice.

## 13. Immediate foreign key

Setup: an immediate foreign key from `last_event_id` to `tracking.execution_event_inbox.event_id`.
Stimulus: the function inserts the inbox row and then writes `last_event_id` in the same transaction.
Expected: commit succeeds only when tenant, execution, and event identity match. A state row that points at another tenant's inbox UUID does not commit.
`PASS`: the committed pair matches on those fields.
`FAIL`: commit succeeds for a UUID-only match, or the state statement runs before the inbox insert.

## 14. Duplicate event_id

Setup: the proof from test 1 is committed. The same raw record is offered again. A second record has a different `event_id`, the same hash, and a different sequence.
Stimulus: one replay call, then one hash-only call.
Expected: the replay inserts nothing and does not change the state row. After the proof check it may commit the offset once. The hash-only call raises and does not commit an offset. Hash equality alone is not terminal.
`PASS`: inbox count stays 1 for the original event, and the hash-only call leaves no new row.
`FAIL`: a second inbox row, an overwritten inbox row, or an offset advance for the hash-only record.

## 15. ETA writer

Setup: execution epoch 5. State epoch 4.
Stimulus: the current order at `eta_ingest_service.go:256`, then the protected ETA function.
Expected: the current order is the failure mode until it is replaced. The protected function raises `STALE_EPOCH` and leaves `tracking.execution_stop_eta_state` unchanged.
`PASS`: no ETA row changes while the epochs differ.
`FAIL`: `execution_stop_eta_state` changes, or the state row is locked before the execution row.

## 16. MarkApproachEmitted

Setup: execution tenant A, epoch 5. State tenant A, epoch 4.
Stimulus: `BEGIN`, the fenced approach function. A second case uses state tenant B and epoch 5.
Expected: both roll back. `approach_emitted_at` is unchanged. No new inbox row. No offset commit. The pre-lock read at `approach.go:69` is not the epoch used by the function.
`PASS`: the state row and the offset match the setup.
`FAIL`: `approach_emitted_at` changes, or a zero-row update still commits, which is the current result at `approach.go:95-96`.

## 17. Old consumer restart

Setup: the group has one member that calls `UpsertExecutionTrackingState` and does not call the protected function.
Stimulus: start that member after the revoke, as `tracking_service_login`. Repeat as the table owner.
Expected: the non-owner login receives `42501` and commits nothing. The owner login can write. The new process has already left the group because the old member is present.
`PASS`: the new mode writes nothing, and the report does not call the owner session fenced.
`FAIL`: the new mode runs beside the old member, or the owner session is reported as blocked by the revoke.

## 18. Operator SQL

Setup: a session that is not an application login. The statement text is not in this repository.
Stimulus: `UPDATE tracking.execution_tracking_state` as that session.
Expected: record the owner and the result. Success means the session is a bypass.
`PASS` is not available in this stage. `OPERATOR_SQL=NOT_VERIFIED`.
`FAIL`: the plan claims this session was blocked without a server result.

## 19. Migration rollback

Setup: the new grants are installed, the new binary is stopped, and no non-null epoch column is present.
Stimulus: run the down migration. Then repeat with one non-null `owner_epoch`.
Expected: the first down restores the previous owner and grants. The second down stops and leaves the column in place.
`PASS`: the empty case restores grants, and the non-null case changes nothing.
`FAIL`: the down runs while a consumer is connected, or it rewrites historical epochs.

`SHIPMENT_OWNER_RACE=OPEN_UNTIL_IMPLEMENTED`. None of these tests has been run, so none of them closes that race.
