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
| FT-ACK | Interruption during acknowledgement | Fault points before and after T1, T2, T3, T4, PostgreSQL COMMIT, outcome T1–T3, and T4' | Crash the witness, issuer, or anchor client at each point; drop responses; retry | Matches the transition table in `RECOVERY_ANCHOR_PROTOCOL.md` §2; never `ISSUE_OK` before the outcome is anchored; never auto `ABORT_CONFIRMED` | Every unknown outcome ends `IN_DOUBT` and is then reconciled |
| FT-TIMELINE | Old PostgreSQL timeline restored | Promote a replica or PITR to an earlier timeline; keep the old primary reachable | Issue on the new timeline; attempt a commit on the old primary | Anchored reservations block reissue on the new timeline; an old-primary commit is detected against the anchored history | `OLD_PRIMARY_FENCING` remains `NOT_PROVEN` unless a separate fencing mechanism passes its own test |
| FT-CONC | Concurrent issuers | Several issuer processes, and two issuers from different epochs | Same key reserved at once; epoch change during reservation | Exactly one anchored reservation per key; stale epoch rejected; anchor `CONFLICT` on any same-seq fork | Duplicate count = 0 over all rounds |
| FT-POWER | Power interruption | Hard power cut (not process kill) of the witness host, the anchor host, and each in turn, at every acknowledgement point | Restart and compare | No acknowledged record is lost from the anchor. Lost unacknowledged records are safe. An anchor ahead of the journal after a journal-host power loss gives `ROLLBACK_DETECTED` (fail closed) | Measures how often the fail-closed outcome blocks the namespace |
| FT-PRIV | Privileged rollback and credential compromise | One test per credential: issuer, writer, witness service, fencing controller, recovery operator, anchor admin, shared PostgreSQL login | Each principal attempts to lower or reset the anchor, rewrite history, append reservations, open the gate, restore a store | Single-credential compromise never controls all four states; anchor never lowered; forged tail detected by pins | The shared-login PostgreSQL bypass (0.1Q F3) is expected to remain detectable only, not prevented |
| FT-RESTORE-ANCHOR | Anchor restored alone | Restore the anchor's storage from its own backup to an older position | Restart the witness and the issuer | Witness sees `journal.len > anchor.seq + G`, giving `UNCOVERED_SUFFIX source=anchor` and FAILED; no re-anchoring of the gap without the verified records | Mirrors 0.1S P2A for the anchor |
| FT-INDEP | Failure-domain independence | Fault tests that take down PostgreSQL storage or host, the witness host, and the anchor host separately | Observe the other components | Each failure leaves the others' state unchanged; issuance fails closed | Required before `ANCHOR_FAILURE_DOMAIN` can change from `NOT_PROVEN` |

## Exit criteria for a future stage

Required:
- FT-P3B-A, FT-P3B-NOCOPY, FT-PART, FT-ACK, FT-CONC, FT-POWER, FT-PRIV, FT-RESTORE-ANCHOR, and FT-INDEP pass in three of three attempts on independent environments.
- FT-P3B-B is executed and its result is reported as the stated boundary.
- FT-TIMELINE is reported together with a separate fencing proof, or stays `OLD_PRIMARY_FENCING=NOT_PROVEN`.

Passing these tests does not open any gate by itself. `ISSUER_ISSUANCE_GATE`, `IMPLEMENTATION_GATE`, and `DECISION_CONSUMPTION_GATE` each need their own decision.
