# Attempt 3 results

- PostgreSQL container: `bintrans-proof-0-1r-7b90a659` (network `none`, no published ports), removed: True
- Server: PostgreSQL 16.15 (Debian 16.15-1.pgdg13+2) on x86_64-pc-linux-gnu, compiled by gcc (Debian 14.2.0-19) 14.2.0, 64-bit
- Started 2026-11-17T17:39:50.339247+00:00, finished 2026-11-17T17:41:10.216899+00:00
- Harness error: None
- Counts: {"PASS": 86}
- Findings: ["F-TAIL-LOSS"]

`COUNTEREXAMPLE_REPRODUCED` with PASS means the predicted unsafe or unproven behavior was observed.

| ID | Scenario | Kind | Result | Description |
| --- | --- | --- | --- | --- |
| `W1-RESERVE` | W1_STATE_MACHINE | POSITIVE_CONTROL | PASS | First reserve of (ns, k1) is RESERVED |
| `W1-NO-SECOND-RESERVE` | W1_STATE_MACHINE | DENIAL | PASS | Second reserve of the same key is DUPLICATE and appends nothing |
| `W1-NON-HOLDER-RESERVE` | W1_STATE_MACHINE | DENIAL | PASS | A non-holder issuer cannot reserve |
| `W1-OPERATOR-CANNOT-RESERVE` | W1_STATE_MACHINE | DENIAL | PASS | operator cannot reserve |
| `W1-AUDITOR-CANNOT-RESERVE` | W1_STATE_MACHINE | DENIAL | PASS | auditor cannot reserve |
| `W1-BAD-SIGNATURE` | W1_STATE_MACHINE | DENIAL | PASS | A request signed with the wrong key is AUTH_FAILED |
| `W1-TICKET-MISMATCH` | W1_STATE_MACHINE | DENIAL | PASS | Outcome citing a different ticket_id is rejected |
| `W1-NOT-OWNER` | W1_STATE_MACHINE | DENIAL | PASS | Another issuer cannot append the outcome |
| `W1-EVIDENCE-REQUIRED` | W1_STATE_MACHINE | DENIAL | PASS | COMMIT_CONFIRMED without a commit acknowledgement is rejected |
| `W1-COMMIT` | W1_STATE_MACHINE | POSITIVE_CONTROL | PASS | Owner appends COMMIT_CONFIRMED for its reservation |
| `W1-OUTCOME-IDEMPOTENT` | W1_STATE_MACHINE | POSITIVE_CONTROL | PASS | Repeating the same outcome is IDEMPOTENT and appends nothing |
| `W1-NO-DELETE` | W1_STATE_MACHINE | DENIAL | PASS | There is no 'delete' operation (UNSUPPORTED_OP) |
| `W1-NO-UPDATE` | W1_STATE_MACHINE | DENIAL | PASS | There is no 'update' operation (UNSUPPORTED_OP) |
| `W1-NO-TRUNCATE` | W1_STATE_MACHINE | DENIAL | PASS | There is no 'truncate' operation (UNSUPPORTED_OP) |
| `W1-NO-REWRITE` | W1_STATE_MACHINE | DENIAL | PASS | There is no 'rewrite' operation (UNSUPPORTED_OP) |
| `W1-NO-RELEASE` | W1_STATE_MACHINE | DENIAL | PASS | There is no 'release' operation (UNSUPPORTED_OP) |
| `W1-NO-RESERVATION` | W1_STATE_MACHINE | DENIAL | PASS | Outcome without a reservation is rejected |
| `W1-EPOCH-MONOTONIC` | W1_STATE_MACHINE | DENIAL | PASS | Epoch cannot be set to a value not above the current one |
| `W1-EPOCH-ROLE` | W1_STATE_MACHINE | DENIAL | PASS | An issuer cannot set the fencing epoch |
| `W1-CHECKPOINT` | W1_STATE_MACHINE | POSITIVE_CONTROL | PASS | Operator records the accepted sequence |
| `W1-CHECKPOINT-NO-LOWER` | W1_STATE_MACHINE | DENIAL | PASS | The accepted sequence cannot be lowered |
| `W1-CHECKPOINT-HASH` | W1_STATE_MACHINE | DENIAL | PASS | A checkpoint with a wrong head hash is rejected |
| `W1-CHECKPOINT-ROLE` | W1_STATE_MACHINE | DENIAL | PASS | An issuer cannot move the checkpoint |
| `W1-FAULT-ROLE` | W1_STATE_MACHINE | DENIAL | PASS | Only the test harness principal can arm faults |
| `W1-NO-TIMEOUT-RELEASE` | W1_STATE_MACHINE | DENIAL | PASS | A RESERVED key is still taken after a 3 s wait (no expiry) |
| `W1-SURVIVES-RESTART` | W1_STATE_MACHINE | POSITIVE_CONTROL | PASS | After restart: k1 COMMIT_CONFIRMED with full history (reserve + outcome), k2 still RESERVED, same record count |
| `W1-TRANSITION-MATRIX` | W1_STATE_MACHINE | POSITIVE_CONTROL | PASS | All 40 (from_state, outcome, role) cases match the 0.1P table: allowed -> APPENDED, same state -> IDEMPOTENT, everything else -> INVALID_TRANSITION |
| `W1-IN-DOUBT-NEVER-ABORT` | W1_STATE_MACHINE | DENIAL | PASS | IN_DOUBT -> ABORT_CONFIRMED is rejected for issuer and operator |
| `W1-TERMINAL-STATES` | W1_STATE_MACHINE | DENIAL | PASS | COMMIT_CONFIRMED and PERMANENTLY_DENIED accept no other outcome |
| `W2-RACE-1` | W2_CONCURRENT_RESERVE | POSITIVE_CONTROL | PASS | Round 1: 8 concurrent processes reserve one key; exactly one RESERVED, 7 DUPLICATE, one RESERVE record |
| `W2-RACE-2` | W2_CONCURRENT_RESERVE | POSITIVE_CONTROL | PASS | Round 2: 8 concurrent processes reserve one key; exactly one RESERVED, 7 DUPLICATE, one RESERVE record |
| `W2-RACE-3` | W2_CONCURRENT_RESERVE | POSITIVE_CONTROL | PASS | Round 3: 8 concurrent processes reserve one key; exactly one RESERVED, 7 DUPLICATE, one RESERVE record |
| `W2-SINGLE-WRITER` | W2_CONCURRENT_RESERVE | DENIAL | PASS | A second witness process on the same store refuses to start (exit 3, store_locked) |
| `S0-ISSUE-OK` | S0_NORMAL | POSITIVE_CONTROL | PASS | Normal issuance returns ISSUE_OK with the ticket in PostgreSQL and COMMIT_CONFIRMED |
| `S0-RESERVE-BEFORE-PG` | S0_NORMAL | ORDERING | PASS | Witness RESERVED is received before the PostgreSQL ticket transaction begins |
| `SA-STATE-AFTER-CRASH` | S_A_CRASH_AFTER_RESERVATION | POSITIVE_CONTROL | PASS | Process died after RESERVED: exit 137, witness RESERVED, no PostgreSQL ticket transaction, no ticket |
| `SA-RESTART-IN-DOUBT` | S_A_CRASH_AFTER_RESERVATION | DENIAL | PASS | Restarted process marks the old reservation IN_DOUBT and issues nothing |
| `SA-GATE-CLOSED-WHILE-IN-DOUBT` | S_A_CRASH_AFTER_RESERVATION | DENIAL | PASS | A new request in the namespace is refused while a key is IN_DOUBT |
| `SA-RECONCILE` | S_A_CRASH_AFTER_RESERVATION | POSITIVE_CONTROL | PASS | Reconciliation: automatic ABORT_CONFIRMED rejected; tombstone absent -> PERMANENTLY_DENIED |
| `SA-RETRY-DENIED` | S_A_CRASH_AFTER_RESERVATION | DENIAL | PASS | Retry of the same request after reconciliation is ISSUE_DENIED_PERMANENTLY |
| `SA-NEW-KEY-AFTER-RECONCILE` | S_A_CRASH_AFTER_RESERVATION | POSITIVE_CONTROL | PASS | After reconciliation a new request_id issues normally |
| `SPC-STATE` | S_CRASH_BEFORE_PG_COMMIT | POSITIVE_CONTROL | PASS | Died after INSERT before COMMIT: exit 137, open transaction rolled back by the server, witness RESERVED |
| `SPC-NO-SECOND-TICKET` | S_CRASH_BEFORE_PG_COMMIT | DENIAL | PASS | Restart -> IN_DOUBT; reconciliation -> PERMANENTLY_DENIED (never ABORT); retry denied; zero tickets; one reservation |
| `SB-STATE` | S_B_COMMIT_WITHOUT_OUTCOME | POSITIVE_CONTROL | PASS | Died after COMMIT before outcome: ticket row present, witness still RESERVED, no response/ticket id returned |
| `SB-NO-GRANT-WHILE-IN-DOUBT` | S_B_COMMIT_WITHOUT_OUTCOME | DENIAL | PASS | Before reconciliation the retry gets no ticket (IN_DOUBT) and other requests are refused (gate closed) |
| `SB-RECONCILE-EVIDENCE-BOUND` | S_B_COMMIT_WITHOUT_OUTCOME | DENIAL | PASS | Operator COMMIT_CONFIRMED citing a different tombstone ticket is rejected |
| `SB-RECONCILE-COMMIT` | S_B_COMMIT_WITHOUT_OUTCOME | POSITIVE_CONTROL | PASS | Matching tombstone -> COMMIT_CONFIRMED; retry replays the same ticket; still one row and one reservation |
| `SC-ABORT-CONFIRMED` | S_C_ROLLBACK | POSITIVE_CONTROL | PASS | Observed ROLLBACK -> ABORT_CONFIRMED; reservation kept; no ticket |
| `SC-NO-NEW-RESERVATION` | S_C_ROLLBACK | DENIAL | PASS | ABORT_CONFIRMED does not free the key: a new reserve is DUPLICATE |
| `SC-PAYLOAD-MISMATCH` | S_C_ROLLBACK | DENIAL | PASS | Same request_id with a different payload is ISSUE_DENIED_PAYLOAD_MISMATCH |
| `SC-RESUME-SAME-RESERVATION` | S_C_ROLLBACK | POSITIVE_CONTROL | PASS | Retry resumes the same reservation: ISSUE_OK with the reserved ticket_id, one row, one RESERVE record |
| `SD1-IN-DOUBT` | S_D1_UNKNOWN_COMMIT_COMMITTED | POSITIVE_CONTROL | PASS | Unobserved COMMIT acknowledgement -> IN_DOUBT, no ticket id returned, row exists |
| `SD1-RECONCILE` | S_D1_UNKNOWN_COMMIT_COMMITTED | POSITIVE_CONTROL | PASS | Retry before reconciliation gets nothing; auto-abort rejected; matching row -> COMMIT_CONFIRMED; replay same ticket |
| `SD2-IN-DOUBT` | S_D2_UNKNOWN_COMMIT_NOT_COMMITTED | POSITIVE_CONTROL | PASS | Connection lost during COMMIT -> IN_DOUBT, no ticket id returned; the row does not exist |
| `SD2-NO-AUTO-ABORT` | S_D2_UNKNOWN_COMMIT_NOT_COMMITTED | DENIAL | PASS | Unknown outcome is never auto-converted to ABORT_CONFIRMED; it becomes PERMANENTLY_DENIED; retry gets no ticket |
| `SE-REPLAY` | S_E_RESPONSE_LOST | POSITIVE_CONTROL | PASS | Lost response: retry returns ISSUE_REPLAY_ISSUED with the same ticket; no new witness record; one row |
| `SE-PAYLOAD-MISMATCH` | S_E_RESPONSE_LOST | DENIAL | PASS | Retry with a different payload is ISSUE_DENIED_PAYLOAD_MISMATCH |
| `SL-NO-PG-WITHOUT-ACK` | S_WITNESS_CRASH_AFTER_APPEND | DENIAL | PASS | Witness died after the durable append but before replying: issuer gets no ack and does not start PostgreSQL |
| `SL-DURABLE-RESERVATION` | S_WITNESS_CRASH_AFTER_APPEND | POSITIVE_CONTROL | PASS | After witness restart the unacknowledged reservation is RESERVED; retry -> IN_DOUBT; reconciliation -> PERMANENTLY_DENIED |
| `SU-DENIED` | S_WITNESS_UNAVAILABLE | DENIAL | PASS | Witness down: ISSUE_DENIED_WITNESS_UNAVAILABLE and no PostgreSQL connection at all |
| `SU-NO-SIDE-EFFECT` | S_WITNESS_UNAVAILABLE | POSITIVE_CONTROL | PASS | After restart the witness has the same records and no reservation for k1 |
| `SR-SETUP` | S_STALE_PG_RESTORE | POSITIVE_CONTROL | PASS | Before restore: k1, k2 issued, k3 committed in PostgreSQL but witness RESERVED; after restoring the old dump only k1 remains |
| `SR-CONFIRMED-NOT-REISSUED` | S_STALE_PG_RESTORE | DENIAL | PASS | k2 is COMMIT_CONFIRMED in the witness but missing in the restored DB: no new ticket, reconciliation required |
| `SR-RESERVED-NOT-FREED` | S_STALE_PG_RESTORE | DENIAL | PASS | k3 (committed before the restore, row now gone) stays blocked: retry IN_DOUBT; auto-abort rejected; -> PERMANENTLY_DENIED, not a free request_id |
| `SR-GATE-CLOSED` | S_STALE_PG_RESTORE | DENIAL | PASS | A new request after the stale restore is refused: gate CLOSED because a confirmed ticket is missing in PostgreSQL |
| `SR-PG-ALONE-WOULD-ACCEPT` | S_STALE_PG_RESTORE | COUNTEREXAMPLE_REPRODUCED | PASS | Without the witness, the restored database accepts a second ticket for k2 (insert succeeds; rolled back by harness) |
| `SR-FINAL` | S_STALE_PG_RESTORE | POSITIVE_CONTROL | PASS | Final restored DB holds only k1; each key has exactly one witness reservation |
| `SS-HANDOVER` | S_SPLIT_BRAIN | POSITIVE_CONTROL | PASS | A issued k1 and holds an in-flight reservation k0; operator moves epoch 2 to B |
| `SS-OLD-COMMIT-NOT-FENCED` | S_SPLIT_BRAIN | COUNTEREXAMPLE_REPRODUCED | PASS | After handover, old issuer A's PostgreSQL COMMIT for its earlier reservation k0 still succeeds; the witness does not stop it and accepts A's outcome for that existing reservation |
| `SS-OLD-ISSUER-CANNOT-RESERVE` | S_SPLIT_BRAIN | DENIAL | PASS | Old issuer A cannot start new issuance (gate: not epoch holder) and the witness refuses its reserve even if it claims epoch 2 |
| `SS-NEW-ISSUER-WORKS` | S_SPLIT_BRAIN | POSITIVE_CONTROL | PASS | New holder B issues k5 under epoch 2 |
| `SS-BYPASS-DETECTED-NOT-PREVENTED` | S_SPLIT_BRAIN | COUNTEREXAMPLE_REPRODUCED | PASS | A writes ticket k9 straight into PostgreSQL with no reservation: PostgreSQL accepts it; B's next issuance sees TICKET_IN_PG_WITHOUT_RESERVATION and the gate stays CLOSED |
| `SS-RACE-1` | S_SPLIT_BRAIN | POSITIVE_CONTROL | PASS | Round 1: 4 old-epoch A + 4 new-epoch B processes race on one key: A all NOT_EPOCH_HOLDER, exactly one B RESERVED |
| `SS-RACE-2` | S_SPLIT_BRAIN | POSITIVE_CONTROL | PASS | Round 2: 4 old-epoch A + 4 new-epoch B processes race on one key: A all NOT_EPOCH_HOLDER, exactly one B RESERVED |
| `SS-RACE-3` | S_SPLIT_BRAIN | POSITIVE_CONTROL | PASS | Round 3: 4 old-epoch A + 4 new-epoch B processes race on one key: A all NOT_EPOCH_HOLDER, exactly one B RESERVED |
| `SX-FLIP` | S_WITNESS_CORRUPTION | DENIAL | PASS | Tamper 'flip' is detected at load (HASH_MISMATCH); every request refused; issuer denies without touching PostgreSQL |
| `SX-REWRITE-RECORD` | S_WITNESS_CORRUPTION | DENIAL | PASS | Tamper 'rewrite-record' is detected at load (RECORD_REQUEST_MISMATCH); every request refused; issuer denies without touching PostgreSQL |
| `SX-REWRITE-SIGNED` | S_WITNESS_CORRUPTION | DENIAL | PASS | Tamper 'rewrite-signed' is detected at load (REQUEST_MAC_INVALID); every request refused; issuer denies without touching PostgreSQL |
| `SX-REORDER` | S_WITNESS_CORRUPTION | DENIAL | PASS | Tamper 'reorder' is detected at load (SEQUENCE_VIOLATION); every request refused; issuer denies without touching PostgreSQL |
| `SX-TRUNCATE-BELOW-CHECKPOINT` | S_WITNESS_CORRUPTION | DENIAL | PASS | Tamper 'truncate-below-checkpoint' is detected at load (ROLLBACK_DETECTED); every request refused; issuer denies without touching PostgreSQL |
| `SX-FORGE-CHECKPOINT` | S_WITNESS_CORRUPTION | DENIAL | PASS | Tamper 'forge-checkpoint' is detected at load (CHECKPOINT_MAC_INVALID); every request refused; issuer denies without touching PostgreSQL |
| `SX-GARBAGE` | S_WITNESS_CORRUPTION | DENIAL | PASS | Tamper 'garbage' is detected at load (UNPARSEABLE_RECORD); every request refused; issuer denies without touching PostgreSQL |
| `SX-TAIL-LOSS-UNDETECTED` | S_WITNESS_CORRUPTION | COUNTEREXAMPLE_REPRODUCED | PASS | Removing records after the accepted checkpoint is NOT detected: witness loads OK and k2 (lost tail) can be reserved a second time; k1 (covered by the checkpoint) stays DUPLICATE |
| `HYGIENE-CONTAINER-REMOVED` | HYGIENE | POSITIVE_CONTROL | PASS | PostgreSQL container removed |
| `HYGIENE-TMP-REMOVED` | HYGIENE | POSITIVE_CONTROL | PASS | Witness temp directory removed |
| `HYGIENE-SECRET-SCAN` | HYGIENE | POSITIVE_CONTROL | PASS | No generated key, pipe authkey, or password in any evidence file |
