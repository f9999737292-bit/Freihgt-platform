# Attempt 2 results

- PostgreSQL container: `bintrans-proof-0-1s-d76b2f46` (network `none`, no published ports), removed: True
- Server: PostgreSQL 16.15 (Debian 16.15-1.pgdg13+2) on x86_64-pc-linux-gnu, compiled by gcc (Debian 14.2.0-19) 14.2.0, 64-bit
- Started 2026-11-17T18:50:06.203220+00:00, finished 2026-11-17T18:51:41.616242+00:00
- Harness error: None
- Counts: {"PASS": 68}
- Findings: []

`COUNTEREXAMPLE_REPRODUCED` with PASS means the predicted unsafe or unproven behavior was observed.

| ID | Scenario | Kind | Result | Description |
| --- | --- | --- | --- | --- |
| `T1-CONSISTENT` | T1_CONSISTENT | POSITIVE_CONTROL | PASS | Journal complete and equal to the high-water mark: witness CONSISTENT after restart, k2 cannot be reserved again (DUPLICATE), a new key in a fresh namespace is issued after the issuer's own history check |
| `T2-DETECTED` | T2_LAST_RECORD_DELETED | DETECTION | PASS | Last confirmed record (seq 6) deleted: the witness refuses to serve with reason ROLLBACK_DETECTED source=high_water |
| `T2-NO-DUPLICATE` | T2_LAST_RECORD_DELETED | DENIAL | PASS | Re-reserving k2 is refused (WITNESS_FAILED), nothing is appended, the mark is unchanged |
| `T2-GATE-CLOSED` | T2_LAST_RECORD_DELETED | DENIAL | PASS | The issuer is denied before any PostgreSQL connection |
| `T3-DETECTED` | T3_TRUNCATED_TO_CHECKPOINT | DETECTION | PASS | Journal truncated exactly to the old operator checkpoint (seq 4): the witness refuses to serve with reason ROLLBACK_DETECTED source=high_water |
| `T3-NO-DUPLICATE` | T3_TRUNCATED_TO_CHECKPOINT | DENIAL | PASS | Re-reserving k2 is refused (WITNESS_FAILED), nothing is appended, the mark is unchanged |
| `T3-GATE-CLOSED` | T3_TRUNCATED_TO_CHECKPOINT | DENIAL | PASS | The issuer is denied before any PostgreSQL connection |
| `T4-DETECTED` | T4_RESTORED_ONE_BELOW | DETECTION | PASS | Journal replaced by a byte copy taken at seq 5 (one below the mark): the witness refuses to serve with reason ROLLBACK_DETECTED source=high_water |
| `T4-NO-DUPLICATE` | T4_RESTORED_ONE_BELOW | DENIAL | PASS | Re-reserving k2 is refused (WITNESS_FAILED), nothing is appended, the mark is unchanged |
| `T4-GATE-CLOSED` | T4_RESTORED_ONE_BELOW | DENIAL | PASS | The issuer is denied before any PostgreSQL connection |
| `T5-DETECTED` | T5_SAME_SEQ_OTHER_DIGEST | DETECTION | PASS | Journal with the same length but a validly signed, validly chained different record at seq 6: the witness refuses to serve with reason FORK_DETECTED source=high_water seq=6 |
| `T5-NO-DUPLICATE` | T5_SAME_SEQ_OTHER_DIGEST | DENIAL | PASS | Re-reserving k2 is refused (WITNESS_FAILED), nothing is appended, the mark is unchanged |
| `T5-GATE-CLOSED` | T5_SAME_SEQ_OTHER_DIGEST | DENIAL | PASS | The issuer is denied before any PostgreSQL connection |
| `T6A-DETECTED` | T6_OTHER_INCARNATION | DETECTION | PASS | Journal rebuilt with the same records under a different GENESIS incarnation (valid chain); operator checkpoint left in place: the witness refuses to serve with reason FORK_DETECTED source=operator_checkpoint |
| `T6A-NO-DUPLICATE` | T6_OTHER_INCARNATION | DENIAL | PASS | Re-reserving k2 is refused (WITNESS_FAILED), nothing is appended, the mark is unchanged |
| `T6A-GATE-CLOSED` | T6_OTHER_INCARNATION | DENIAL | PASS | The issuer is denied before any PostgreSQL connection |
| `T6B-DETECTED` | T6_OTHER_INCARNATION | DETECTION | PASS | Journal rebuilt under a different GENESIS incarnation and the local operator checkpoint deleted too: the witness refuses to serve with reason INCARNATION_MISMATCH source=high_water |
| `T6B-NO-DUPLICATE` | T6_OTHER_INCARNATION | DENIAL | PASS | Re-reserving k2 is refused (WITNESS_FAILED), nothing is appended, the mark is unchanged |
| `T6B-GATE-CLOSED` | T6_OTHER_INCARNATION | DENIAL | PASS | The issuer is denied before any PostgreSQL connection |
| `T7-HW-DOWN-AT-START` | T7_HW_UNAVAILABLE | FAIL_CLOSED | PASS | High-water authority down when the witness starts: witness HIGH_WATER_UNAVAILABLE, reserve refused without append, issuer denied before PostgreSQL |
| `T7-RECOVERS-WHEN-HW-RETURNS` | T7_HW_UNAVAILABLE | POSITIVE_CONTROL | PASS | When the authority returns, the witness re-verifies and becomes CONSISTENT without restart |
| `T7-ISSUER-OWN-HW-CHECK` | T7_HW_UNAVAILABLE | FAIL_CLOSED | PASS | Witness still CONSISTENT in memory, authority down: the issuer's own high-water read fails and it is denied before PostgreSQL |
| `T7-PREPARED-NOT-ACKNOWLEDGED` | T7_HW_UNAVAILABLE | FAIL_CLOSED | PASS | Authority lost mid-operation: the k4 record is journaled (PREPARED) but the reply is HIGH_WATER_UNAVAILABLE, not RESERVED; the next write (k5) is refused without append |
| `T7-ROLL-FORWARD` | T7_HW_UNAVAILABLE | POSITIVE_CONTROL | PASS | Authority back: the single PREPARED record chains from the pinned head and is rolled forward; k4 is RESERVED and cannot be reserved again |
| `T8A-DETECTED` | T8_HW_CORRUPTED | DETECTION | PASS | High-water store tampered (mark_digest changed, hash not recomputed): the authority loads in FAILED mode with HW_HASH_MISMATCH |
| `T8A-GATE-CLOSED` | T8_HW_CORRUPTED | FAIL_CLOSED | PASS | The witness reports HIGH_WATER_UNAVAILABLE, refuses k2 and the issuer is denied before PostgreSQL |
| `T8B-DETECTED` | T8_HW_CORRUPTED | DETECTION | PASS | High-water store tampered (mark_digest changed and record hash recomputed (signed request unchanged)): the authority loads in FAILED mode with HW_RECORD_REQUEST_MISMATCH |
| `T8B-GATE-CLOSED` | T8_HW_CORRUPTED | FAIL_CLOSED | PASS | The witness reports HIGH_WATER_UNAVAILABLE, refuses k2 and the issuer is denied before PostgreSQL |
| `T8C-DETECTED` | T8_HW_CORRUPTED | DETECTION | PASS | High-water store tampered (mark_digest and signed request body changed, record hash recomputed, MAC not recomputable): the authority loads in FAILED mode with HW_REQUEST_MAC_INVALID |
| `T8C-GATE-CLOSED` | T8_HW_CORRUPTED | FAIL_CLOSED | PASS | The witness reports HIGH_WATER_UNAVAILABLE, refuses k2 and the issuer is denied before PostgreSQL |
| `T9-LOWER-REJECTED` | T9_HW_LOWER_REJECTED | DENIAL | PASS | Every attempt to lower, overwrite, skip, fork, re-initialise or delete the mark is rejected; non-writer principals cannot write; the mark and the authority log are unchanged |
| `T9-WITNESS-STILL-CONSISTENT` | T9_HW_LOWER_REJECTED | POSITIVE_CONTROL | PASS | After the rejected attempts the witness restarts CONSISTENT and serves a new reservation |
| `T10-RACE-1` | T10_CONCURRENT_RESERVE | POSITIVE_CONTROL | PASS | Round 1: 8 processes reserve one key at once: exactly one RESERVED, seven DUPLICATE, one RESERVE record, mark equal to journal length |
| `T10-RACE-2` | T10_CONCURRENT_RESERVE | POSITIVE_CONTROL | PASS | Round 2: 8 processes reserve one key at once: exactly one RESERVED, seven DUPLICATE, one RESERVE record, mark equal to journal length |
| `T10-RACE-3` | T10_CONCURRENT_RESERVE | POSITIVE_CONTROL | PASS | Round 3: 8 processes reserve one key at once: exactly one RESERVED, seven DUPLICATE, one RESERVE record, mark equal to journal length |
| `T10-SINGLE-WRITER` | T10_CONCURRENT_RESERVE | DENIAL | PASS | A second authority or witness process on the same store exits with store_locked (code 3) |
| `JW-SETUP` | J_WIT_001 | POSITIVE_CONTROL | PASS | Steps 1-6: epoch, k1 reserved+confirmed, operator checkpoint, k2 reserved+confirmed; the independent mark equals the journal head |
| `JW-LOCAL-CHECKS-PASS-ON-TRUNCATED` | J_WIT_001 | POSITIVE_CONTROL | PASS | Control: the truncated journal is a valid chain that still satisfies the operator checkpoint, so the 0.1R local checks alone would accept it |
| `JW-TAIL-LOSS-DETECTED` | J_WIT_001 | DETECTION | PASS | TAIL_LOSS_DETECTED=YES after restart |
| `JW-DUPLICATE-NOT-ACCEPTED` | J_WIT_001 | DENIAL | PASS | DUPLICATE_RESERVATION_ACCEPTED=NO: reserve k2 is refused and the journal holds no k2 reservation |
| `JW-GATE-CLOSED` | J_WIT_001 | DENIAL | PASS | ISSUER_ISSUANCE_GATE=CLOSED: the issuer is denied for k2 and for a new key, with no PostgreSQL connection |
| `CW1` | CW1_BEFORE_RESERVATION | POSITIVE_CONTROL | PASS | Issuer stops before reserve: no record, no PostgreSQL transaction; the key is still free and the retry issues |
| `CW2` | CW2_AFTER_PREPARE_BEFORE_APPEND | POSITIVE_CONTROL | PASS | Witness stops after validating, before append: issuer sees an unknown outcome and opens no transaction; nothing was journaled or marked, so after restart the key is free and the retry issues |
| `CW3-NO-ISSUE-OK` | CW3_BETWEEN_APPEND_AND_HW | FAIL_CLOSED | PASS | Crash between journal append and high-water update: the issuer never receives RESERVED, reports an unknown outcome and opens no PostgreSQL transaction; the journal is one record ahead of the mark |
| `CW3-ROLL-FORWARD-NO-REISSUE` | CW3_BETWEEN_APPEND_AND_HW | DENIAL | PASS | After restart the PREPARED record is rolled forward (mark = journal); k1 is RESERVED, the retry goes IN_DOUBT and is denied, other keys are gated, reconciliation cannot abort and ends PERMANENTLY_DENIED |
| `CW4-HW-FIRST-WORKS-NORMALLY` | CW4_BETWEEN_HW_AND_APPEND | POSITIVE_CONTROL | PASS | Control: the hw_first order issues normally |
| `CW4-FAIL-CLOSED-NOT-LIVE` | CW4_BETWEEN_HW_AND_APPEND | FAIL_CLOSED | PASS | Crash between high-water update and journal append: no transaction; after restart the mark is ahead of the journal, which is indistinguishable from tail loss, so the witness stays FAILED and every issuance is denied (safe, but needs manual recovery; this is why append_first is selected) |
| `CW5` | CW5_AFTER_HW_BEFORE_REPLY | FAIL_CLOSED | PASS | Crash after the mark covers the reservation, before the reply: issuer unknown outcome, no transaction; after restart the journal equals the mark (no roll-forward), k1 RESERVED -> IN_DOUBT -> PERMANENTLY_DENIED |
| `CW6` | CW6_AFTER_PG_COMMIT | POSITIVE_CONTROL | PASS | Issuer stops after COMMIT, before outcome: row exists, key RESERVED; retry IN_DOUBT; reconciliation against the tombstone gives COMMIT_CONFIRMED; then the retry replays the same ticket |
| `CW7A` | CW7A_OUTCOME_BEFORE_APPEND | FAIL_CLOSED | PASS | Witness stops before appending COMMIT_CONFIRMED: issuer reports ISSUE_OUTCOME_UNKNOWN (never ISSUE_OK); row exists, key RESERVED -> IN_DOUBT; reconciliation COMMIT_CONFIRMED; replay of the same ticket |
| `CW7B` | CW7B_OUTCOME_AFTER_APPEND_BEFORE_HW | FAIL_CLOSED | PASS | Witness stops after appending COMMIT_CONFIRMED, before the mark: issuer reports ISSUE_OUTCOME_UNKNOWN; after restart the outcome is rolled forward and the retry replays the same ticket |
| `CW8-RECONCILER-STOPS-BEFORE-APPEND` | CW8_DURING_RECONCILIATION | FAIL_CLOSED | PASS | Reconciler reads state and PostgreSQL, then stops before writing: the key stays IN_DOUBT and the gate stays closed |
| `CW8-WITNESS-STOPS-DURING-DECISION` | CW8_DURING_RECONCILIATION | FAIL_CLOSED | PASS | Witness stops after journaling the operator's PERMANENTLY_DENIED, before the mark: the reconciler sees no reply; after restart the decision is rolled forward; re-running reconciliation is idempotent and never aborts; k1 stays denied |
| `SP-SETUP` | STALE_PG_COMBINED | POSITIVE_CONTROL | PASS | k1 issued, dump taken, checkpoint, k2 issued; after the stale restore PostgreSQL holds only k1 |
| `SP-TAIL-LOSS-DETECTED` | STALE_PG_COMBINED | DETECTION | PASS | Witness tail loss detected against the high-water mark |
| `SP-NO-SECOND-RESERVED` | STALE_PG_COMBINED | DENIAL | PASS | A second reservation of k2 is refused |
| `SP-NO-SECOND-TICKET` | STALE_PG_COMBINED | DENIAL | PASS | Reusing k2 and issuing a new key are denied before PostgreSQL; no second ticket exists |
| `SP-GATE-CLOSED-UNTIL-RECONCILED` | STALE_PG_COMBINED | DENIAL | PASS | After the complete journal is restored the witness is CONSISTENT again, but issuance stays closed: k2 needs reconciliation (confirmed ticket missing in the stale database) and new keys are gated |
| `P1-DETECTED` | P1_JOURNAL_ONLY | DETECTION | PASS | Attacker with journal write access and every witness key replaces the tail with a longer valid chain: the pinned digest at seq 5 no longer matches -> FORK_DETECTED; k2 cannot be reserved |
| `P2A-HW-ROLLBACK-FAILS-CLOSED` | P2_HIGH_WATER_ONLY | FAIL_CLOSED | PASS | Authority store rolled back by more than one record: the journal is ahead by an unacknowledged-looking suffix that cannot be rolled forward -> UNCOVERED_SUFFIX, witness refuses |
| `P2B-HW-WIPED-NO-REINIT` | P2_HIGH_WATER_ONLY | FAIL_CLOSED | PASS | Authority store deleted: the witness with a non-empty journal refuses (HIGH_WATER_MISSING) and does not re-initialise the mark |
| `P3A-UNDETECTED-BY-WITNESS` | P3_BOTH_ROLLED_BACK | COUNTEREXAMPLE_REPRODUCED | PASS | Both stores rolled back consistently: the witness is CONSISTENT and accepts a second successful reservation of k2 (outside the model: the authority itself must not be rollback-able) |
| `P3A-PG-ROW-STILL-GATES` | P3_BOTH_ROLLED_BACK | DENIAL | PASS | PostgreSQL was not rolled back: the issuer's gate sees k2's row without a reservation and refuses |
| `P3B-SECOND-TICKET` | P3_BOTH_ROLLED_BACK | COUNTEREXAMPLE_REPRODUCED | PASS | Journal, authority and PostgreSQL all rolled back to the same point: k2 is issued again with a different ticket id. No local mechanism detects a consistent rollback of every store |
| `P4-HISTORY-PIN-DETECTS` | P4_WITNESS_CREDENTIAL | DETECTION | PASS | With the witness credential the attacker can advance the mark to a forged head (the authority cannot check content) but cannot lower it; the earlier pinned digests still expose the rewrite (FORK_DETECTED seq=5) |
| `HYGIENE-CONTAINER-REMOVED` | HYGIENE | POSITIVE_CONTROL | PASS | PostgreSQL container removed |
| `HYGIENE-TMP-REMOVED` | HYGIENE | POSITIVE_CONTROL | PASS | Server temp directory removed |
| `HYGIENE-SECRET-SCAN` | HYGIENE | POSITIVE_CONTROL | PASS | No generated key, pipe authkey, or password in any evidence file |
