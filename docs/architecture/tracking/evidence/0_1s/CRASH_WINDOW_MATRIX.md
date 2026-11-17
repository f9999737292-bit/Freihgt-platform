# 0.1S: Crash-window matrix

Each witness stop is a real process exit (`os._exit(137)`) at the named point, armed through the harness-only `fault` operation and limited to one record type. Issuer stops use the issuer's own fault points. Every row ran in its own witness/authority pair and namespace in all three attempts.

`ISSUE_OK` appears only where durability was established. Across all rows, an unknown outcome never became `ABORT_CONFIRMED`: every reconciliation first tried `ABORT_CONFIRMED` and observed `INVALID_TRANSITION`.

| # | Stop point | Order | Issuer sees | PostgreSQL txn | Journal vs mark after stop | After restart | Second ticket? | Assertion |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| CW1 | Issuer, before reservation | — | exit 137 | 0 | unchanged | key free; retry `ISSUE_OK` | no (nothing existed) | `CW1` |
| CW2 | Witness, after validating, before journal append | append_first | `ISSUE_WITNESS_OUTCOME_UNKNOWN` | 0 | unchanged, mark unchanged | `CONSISTENT`, key free; retry `ISSUE_OK` | no (nothing existed) | `CW2` |
| CW3 | Witness, **between journal append and high-water** | append_first | `ISSUE_WITNESS_OUTCOME_UNKNOWN` (never `RESERVED`) | 0 | journal = mark + 1 (PREPARED) | one `roll_forward`, k1 `RESERVED`; retry goes `IN_DOUBT` and is denied; other keys `GATE_CLOSED`; reconciliation `PERMANENTLY_DENIED` | no | `CW3-NO-ISSUE-OK`, `CW3-ROLL-FORWARD-NO-REISSUE` |
| CW4 | Witness, **between high-water and journal append** | hw_first (comparison) | `ISSUE_WITNESS_OUTCOME_UNKNOWN` | 0 | mark = journal + 1 | `FAILED ROLLBACK_DETECTED`, because this cannot be told apart from tail loss; every issuance denied | no, but **not live**: manual recovery required | `CW4-HW-FIRST-WORKS-NORMALLY`, `CW4-FAIL-CLOSED-NOT-LIVE` |
| CW5 | Witness, after high-water confirmed, before reply | append_first | `ISSUE_WITNESS_OUTCOME_UNKNOWN` | 0 | equal | `CONSISTENT`, no roll-forward; k1 `RESERVED`, then `IN_DOUBT`, then `PERMANENTLY_DENIED` | no | `CW5` |
| CW6 | Issuer, after PostgreSQL COMMIT | append_first | exit 137 | 1 (row exists) | equal | k1 `RESERVED`, then `IN_DOUBT`; reconciliation against the row gives `COMMIT_CONFIRMED`; retry replays the same ticket | no | `CW6` |
| CW7A | Witness, before appending `COMMIT_CONFIRMED` | append_first | `ISSUE_OUTCOME_UNKNOWN` (never `ISSUE_OK`) | 1 | equal (outcome absent) | k1 `RESERVED`, then `IN_DOUBT`, then `COMMIT_CONFIRMED`; replay of the same ticket | no | `CW7A` |
| CW7B | Witness, after appending `COMMIT_CONFIRMED`, before high-water | append_first | `ISSUE_OUTCOME_UNKNOWN` | 1 | journal = mark + 1 | outcome rolled forward; retry replays the same ticket | no | `CW7B` |
| CW8 | Reconciler stops after reading, before writing; then the witness stops after journaling the operator decision, before high-water | append_first | — | 0 | then journal = mark + 1 | key stays `IN_DOUBT` and gate closed while undecided; after restart the decision is rolled forward (`PERMANENTLY_DENIED`); re-running reconciliation gives `IDEMPOTENT`; retry denied | no | `CW8-RECONCILER-STOPS-BEFORE-APPEND`, `CW8-WITNESS-STOPS-DURING-DECISION` |

## The two write orders

`APPEND_HIGH_WATER_CRASH_WINDOW` (selected `append_first`, CW3 and CW7B):
- After a crash, the journal holds exactly one record that the mark does not cover and that was never acknowledged.
- The issuer cannot have started PostgreSQL work on an unacknowledged reservation.
- On restart the record is rolled forward only if it chains from the pinned head.
- If that record is later lost before roll-forward, nothing that was acknowledged is lost. A retry may then legitimately reserve the key, because no PostgreSQL transaction was started for it.
- Result: safe and live.

`HIGH_WATER_APPEND_CRASH_WINDOW` (comparison `hw_first`, CW4):
- After a crash, the mark covers a record that the journal does not contain. The witness cannot distinguish this from tail loss, so it must fail closed permanently.
- The order is safe, because no second reservation and no ticket arise, but it is **not live**: every such crash needs manual recovery.
- Making it live would need a prepared or confirmed state at the authority. In practice that means the authority keeps the record itself, which makes it a second journal.
- For that reason `append_first` is selected.

The journal append and the authority advance are two separate writes in both orders. Neither is claimed to be atomic with the other.

## Durability boundary

- "Durable" here means fsynced to a file on the host, followed by an acknowledged authority append that was also fsynced.
- No power-cut, OS-crash, or disk-cache test was run (`POWER_LOSS_DURABILITY=NOT_PROVEN`).
- Directory entries are not fsynced on Windows.
