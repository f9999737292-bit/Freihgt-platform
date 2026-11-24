# 0.1S: Privileged bypass

This file covers attackers or faults with direct write access to the stores, or with stolen credentials. The harness plays the attacker: it holds every key, edits files while the servers are stopped, and calls the authority with stolen credentials. All results below were observed in all three attempts.

| Case | Capability | What was done | Observed | Classification | Assertion |
| --- | --- | --- | --- | --- | --- |
| P1 | Journal storage + **every witness key** | Truncated to seq 4, then appended 3 forged, validly signed and chained RESERVE records, so the journal (7) is longer than the mark (6) | `FORK_DETECTED source=high_water seq=5`; k2 re-reserve refused | Detected | `P1-DETECTED` |
| T5, T6B | Journal storage + every witness key (+ the local checkpoint) | Same-length fork; whole journal re-incarnated with the checkpoint deleted | `FORK_DETECTED seq=6`, `INCARNATION_MISMATCH` | Detected | `T5-*`, `T6B-*` |
| P2A | Authority storage only | Authority log rolled back to its first 4 records (mark seq 3) while the journal holds 6 | `UNCOVERED_SUFFIX journal_len=6 high_water_seq=3`; witness refuses | Fail closed (availability loss) | `P2A-HW-ROLLBACK-FAILS-CLOSED` |
| P2B | Authority storage only | Authority log deleted | `HIGH_WATER_MISSING`; the witness does **not** re-initialize the mark | Fail closed | `P2B-HW-WIPED-NO-REINIT` |
| T8A–C | Authority storage only | Mark edited (with and without a recomputed hash, with a modified signed request) | Authority loads `FAILED`; witness `HIGH_WATER_UNAVAILABLE`; issuer denied | Detected / fail closed | `T8*-*` |
| P4 | Journal storage + every witness key + **stolen `witness_svc` authority credential** | Journal tail replaced by forged records, then the mark advanced to the forged head (accepted: the authority cannot check content); a lower mark attempted | Advance `ADVANCED`, lower `NOT_MONOTONIC`. The restarted witness reports `FORK_DETECTED seq=5` because the earlier pinned digests still disagree | Detected (history pins) | `P4-HISTORY-PIN-DETECTS` |
| P3A | **Both** journal and authority storage | Both restored to the same earlier byte snapshot (before k2 was issued); PostgreSQL untouched | Witness `CONSISTENT`; **a second successful reservation of k2 is accepted**. The issuer's gate still refuses k2 because the PostgreSQL row exists without a reservation (`TICKET_IN_PG_WITHOUT_RESERVATION:k2`) | **Counterexample reproduced**: undetected by the witness | `P3A-UNDETECTED-BY-WITNESS`, `P3A-PG-ROW-STILL-GATES` |
| P3B | Both stores **and** PostgreSQL | All three restored to the same earlier point | k2 issued again (`ISSUE_OK`) with a **different ticket ID** | **Counterexample reproduced**: second ticket | `P3B-SECOND-TICKET` |

## Conclusions

- The mark detects rollback or rewriting of **one** side.
  - Journal-side tampering is detected, even with every witness key and a stolen authority write credential, because the authority's history of `(seq, digest)` is append-only and the attacker cannot remove earlier pins.
  - Authority-side tampering is detected or fails closed.
- A **consistent rollback of both stores is not detected** by anything in this model. Resistance to compromise of both sides is not claimed.
  - With PostgreSQL intact, the issuer gate still refuses, so the bypass is detected afterwards rather than prevented.
  - With PostgreSQL rolled back as well, a second ticket is issued.
- **Credential limits.** HMAC keys are symmetric, and the witness process holds every principal's verification key. Whoever controls the witness process can forge issuer and operator requests, so the journal alone cannot attribute records to principals. The authority credential of `witness_svc` lets an attacker advance marks, but not lower or rewrite them.
- **Prevention would need properties this model does not have:**
  - An authority store that cannot be rolled back, or that is anchored outside every store an attacker can restore.
  - Asymmetric signatures held by the issuers and the operator.
  - Credentials and storage for the authority that are separated from the witness and PostgreSQL operators.

  These are requirements R1–R4 and R12 of the 0.1R `FUTURE_WITNESS_REQUIREMENTS.md` (PR #388, `docs/architecture/tracking/evidence/0_1r/`), and they remain unproven.

`DIRECT_PG_BYPASS=OPEN`: writes made directly to PostgreSQL (0.1R `SS-BYPASS-DETECTED-NOT-PREVENTED`) are not addressed here.
