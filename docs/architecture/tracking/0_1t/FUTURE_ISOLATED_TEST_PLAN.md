# 0.1T: Future isolated test plan

Status: `DEFINED_NOT_RUN`. No test in this file was executed in 0.1T. The tests require a selected anchor (see `P3B_RECOVERY_DECISION.md`, feasibility gate) and **independent environments**:
- PostgreSQL;
- the witness (journal and high-water);
- the anchor.

Each must be on separate hosts, storage, and administrative accounts. Running them on one host would only repeat the logical results of 0.1S.

Every test needs separate authorization before it runs. None of them may touch staging or production without that authorization.

## Common rules

- At least three independent attempts per test. Failed attempts and counterexamples are kept.
- Raw evidence per attempt: commands, stdout/stderr, exit codes, SQLSTATEs, store logs, anchor certificates, and a SHA256 manifest over committed blobs.
- Results are `PASS`, `FAIL`, `NOT_RUN`, or `BLOCKED`; a counterexample that reproduces is recorded as such, not as a pass of the safety property.
- "No second ticket" is checked by an observer with read access to every store. That observer is not the issuer.
- No secret value appears in evidence.

## Tests

| ID | Name | Setup | Action | Expected (safety) | Expected (liveness / notes) |
| --- | --- | --- | --- | --- | --- |
| FT-P3B-A | P3B with the anchor intact | Issue k1; snapshot PostgreSQL, journal, and HW; issue k2 (anchored) | Restore PostgreSQL, journal, and HW to the snapshot; leave the anchor; restart; request k2 and a new key k3 | `ROLLBACK_DETECTED source=anchor`; `HANDOVER_PENDING`; gate CLOSED; 0 PostgreSQL transactions; k2 has one ticket ID ever; k3 denied | Recovery from a verifiable copy restores k2's records byte for byte; k2 reconciled to denial, not reissued |
| FT-P3B-B | P3B with the anchor also rolled back | As FT-P3B-A, plus snapshot of the anchor | Restore all four to the snapshot; request k2 | **Expected to reproduce the counterexample** (second ticket) unless a separately chosen external mechanism exists | Records the exact boundary of the claim; must not be reported as fixed |
| FT-P3B-NOCOPY | Recovery without the full history | FT-P3B-A, with every copy of the missing segment destroyed | Attempt recovery with an incomplete copy and with none | Every attempt rejected; namespace stays CLOSED; no reinitialization of the anchor | Documents permanent unavailability of the namespace |
| FT-PART | Network partition | Partition witness from anchor; separately issuer from anchor | Reserve, outcome, gate check | No `RESERVED` and no `ISSUE_OK` during the partition; record stays PREPARED or HW_COVERED | After healing: one roll-forward, one reservation, no duplicate |
| FT-ACK-A | Interruption before the first write (proven absence) | Fault point before T1: witness crashes after receiving `reserve`, before appending | Issuer sees no response. Before any authoritative read, try a retry and an issuance. Then do the authoritative read (`RECOVERY_ANCHOR_PROTOCOL.md` §2) and retry | Before the read: case C (gate CLOSED, `REQUEST_ID=NOT_RELEASED`, 0 PostgreSQL transactions). After the read: `RESERVATION_STATE=ABSENT`, `REQUEST_ID=AVAILABLE_SUBJECT_TO_AUTHORITY_CHECK`. The new `reserve` gets one fresh `RESERVED` after uniqueness, epoch, and gate checks. **No `IN_DOUBT` and no reconciliation** | Exactly one reservation and one ticket for the key |
| FT-ACK-B | Record exists, acknowledgement not observed | Fault points after T1, after T2, after T3, before and after T4; also dropped `RESERVED` responses | Issuer retries before and after the authoritative read; issuer restarts | Before the read: case C. After the read: `RESERVATION_STATE=PENDING_OR_IN_DOUBT`, `SECOND_RESERVATION=DENIED`. Every retry gets `DUPLICATE`. An unanchored record is rolled forward one record at a time. The anchored key goes `IN_DOUBT`, then is reconciled (`COMMIT_CONFIRMED` against an existing row, otherwise `PERMANENTLY_DENIED`). Never auto `ABORT_CONFIRMED` | Duplicate count = 0 |
| FT-ACK-C | Outcome unknown, absence not proven | Same fault points as A and B, combined with: witness unreachable, anchor unreachable, witness `FAILED`, history check failing, `HANDOVER_PENDING`, known restore not reconciled | Issuer retries and requests issuance for the key and for unrelated keys | `ISSUANCE_GATE=CLOSED`, `REQUEST_ID=NOT_RELEASED`, 0 PostgreSQL transactions. No `reserve` is sent as if the key were free. The state does not change on timeout, restart, or a person's approval | Leaves C only via an authoritative read, to A or B as appropriate |
| FT-ACK-PG | PostgreSQL commit and outcome | Fault points before and after PostgreSQL COMMIT, outcome T1–T3, and T4' | Crash the issuer or witness; drop responses; retry | Matches `RECOVERY_ANCHOR_PROTOCOL.md` §2. Never `ISSUE_OK` before the outcome is anchored. Unknown COMMIT goes `IN_DOUBT` and is reconciled; never auto `ABORT_CONFIRMED`. Retry after `ISSUE_OK` replays the same ticket ID | No second ticket |
| FT-SEQ | Sequence rule of contract v1 | Witness service credential, anchor at sequence `n` | Submit `n+2` (gap), `n` with another digest, `n-1`, a multi-record batch, and `n+1` with a wrong predecessor | `GAP`, `CONFLICT`, `NOT_MONOTONIC`, rejection of the batch, `PREV_MISMATCH`; the anchor position and pins are unchanged | Confirms `GROUP_COMMIT=DISABLED`, `MAX_SEQUENCE_INCREMENT=1` |
| FT-TIMELINE | Old PostgreSQL timeline restored | Promote a replica or PITR to an earlier timeline; keep the old primary reachable | Issue on the new timeline; attempt a commit on the old primary | Anchored reservations block reissue on the new timeline; an old-primary commit is detected against the anchored history | `OLD_PRIMARY_FENCING` remains `NOT_PROVEN` unless a separate fencing mechanism passes its own test |
| FT-CONC | Concurrent issuers | Several issuer processes, and two issuers from different epochs | Same key reserved at once; epoch change during reservation | Exactly one anchored reservation per key; stale epoch rejected; anchor `CONFLICT` on any same-seq fork | Duplicate count = 0 over all rounds |
| FT-POWER | Power interruption | Hard power cut (not process kill) of the witness host, the anchor host, and each in turn, at every acknowledgement point | Restart and compare | No acknowledged record is lost from the anchor. Lost unacknowledged records are safe. An anchor ahead of the journal after a journal-host power loss gives `ROLLBACK_DETECTED` (fail closed) | Measures how often the fail-closed outcome blocks the namespace |
| FT-PRIV | Privileged rollback and credential compromise | One test per credential: issuer, writer, witness service, fencing controller, recovery operator, anchor admin, shared PostgreSQL login | Each principal attempts to lower or reset the anchor, rewrite history, append reservations, open the gate, restore a store | Single-credential compromise never controls all four states; anchor never lowered; forged tail detected by pins | The shared-login PostgreSQL bypass (0.1Q F3) is expected to remain detectable only, not prevented |
| FT-RESTORE-ANCHOR-PLUS-ONE | Anchor restored alone, exactly one record behind | Restore the anchor's storage from its own backup so that `journal.len = anchor.seq + 1`, `anchor.seq <= high_water.seq <= journal.len`, the hash chain is valid, the pending record's immediate predecessor equals the anchor's pinned head, exactly one pending record exists, and no conflicting anchor entry exists at `anchor.seq + 1` | Restart the witness and the issuer; request a new reservation and an issuance before the anchor confirms the record; then let roll-forward complete | `ISSUER_ISSUANCE_GATE=CLOSED`, `RECOVERY_STATE=PENDING`, `ROLL_FORWARD=ONE_RECORD_ONLY`, `NEW_RESERVATION_BEFORE_CONFIRMATION=DENIED`. Roll-forward of that single record succeeds only after rules 3–6 of `RECOVERY_ANCHOR_PROTOCOL.md` §3 pass. Those rules check rollback, incarnation, every pin, the high-water comparison, and the record's hash, signature, and predecessor. The anchor must also accept the exact successor and return a fresh certificate covering it. The length equality alone never opens the gate. After confirmation the 0.1P conditions in rule 9 still apply | Mirrors 0.1S T7 roll-forward for the anchor |
| FT-RESTORE-ANCHOR-UNCOVERED-SUFFIX | Anchor restored alone, more than one record behind | Restore the anchor's storage so that `journal.len > anchor.seq + 1` | Restart the witness and the issuer; request reservations and issuance | `ISSUER_ISSUANCE_GATE=CLOSED`, `RECOVERY_STATE=UNCOVERED_SUFFIX` (`UNCOVERED_SUFFIX source=anchor`, FAILED), `AUTOMATIC_ROLL_FORWARD=DENIED`. No record of the gap is re-anchored automatically | Recovery only after a separate history-completeness verification under an authorized recovery protocol (§4 of the protocol, 0.1P phase E). No automatic return to service is promised. Mirrors 0.1S P2A for the anchor |
| FT-RESTORE-ANCHOR-NEG | Negative cases for the `+1` state | Start from the FT-RESTORE-ANCHOR-PLUS-ONE preconditions, then break exactly one of them per run | (a) pending record with a wrong predecessor; (b) pending record whose digest does not match its content or the submitted digest; (c) corrupted hash chain anywhere in the journal; (d) journal or pending record from a conflicting incarnation; (e) anchor unavailable during roll-forward; (f) unknown result of the previous anchor confirmation (response lost) | (a) `PREV_MISMATCH`, FAILED; (b) `FORK_DETECTED` or a rejected digest, FAILED; (c) load-time chain failure, FAILED; (d) `INCARNATION_MISMATCH`, FAILED; (e) `ANCHOR_UNAVAILABLE`, `PENDING`, retryable. For (f) the witness re-sends the same `(seq, digest)` and accepts only `IDEMPOTENT` or `ADVANCED` with a fresh certificate covering it; any other answer gives FAILED, and until the certificate is obtained the state is `PENDING`. In every case gate CLOSED, no new reservation, and **no automatic gate opening** | Expected results only; not run in 0.1T |
| FT-INDEP | Failure-domain independence | Fault tests that take down PostgreSQL storage or host, the witness host, and the anchor host separately | Observe the other components | Each failure leaves the others' state unchanged; issuance fails closed | Required before `ANCHOR_FAILURE_DOMAIN` can change from `NOT_PROVEN` |

## Exit criteria for a future stage

Required:
- FT-P3B-A, FT-P3B-NOCOPY, FT-PART, FT-ACK-A, FT-ACK-B, FT-ACK-C, FT-ACK-PG, FT-SEQ, FT-CONC, FT-POWER, FT-PRIV, FT-RESTORE-ANCHOR-PLUS-ONE, FT-RESTORE-ANCHOR-UNCOVERED-SUFFIX, FT-RESTORE-ANCHOR-NEG, and FT-INDEP pass in three of three attempts on independent environments.
- FT-P3B-B is executed and its result is reported as the stated boundary.
- FT-TIMELINE is reported together with a separate fencing proof, or stays `OLD_PRIMARY_FENCING=NOT_PROVEN`.

Passing these tests does not open any gate by itself. `ISSUER_ISSUANCE_GATE`, `IMPLEMENTATION_GATE`, and `DECISION_CONSUMPTION_GATE` each need their own decision.

## Future research (not part of contract v1)

- **Group commit:** one anchor append covering several records. It is not implemented, not tested, and not allowed by contract v1. A later study would need:
  - a new contract version;
  - a per-record pin and predecessor rule for every record in the group;
  - its own crash-window analysis and tests;
  - review before any use.
