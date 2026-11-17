# 0.1T: Recovery anchor protocol (proposed)

Status: `PROPOSED_NOT_IMPLEMENTED`. Nothing here was executed. The protocol extends the 0.1S high-water protocol (PR #389, `docs/architecture/tracking/evidence/0_1s/HIGH_WATER_PROTOCOL.md`) with one more, independent store.

## 1. Minimal anchor contract

The anchor keeps a separate, append-only position log for each `namespace_id`.

| Field | Meaning | Rule |
| --- | --- | --- |
| `namespace_id` | The `issuer_namespace_id` whose witness history is anchored (0.1P) | Fixed per log. A journal serving several namespaces is anchored under each of them, or the namespaces are given separate journals |
| `generation` / `incarnation` | The journal incarnation pinned at the first entry | Never changes for the log. A different incarnation is a mismatch, not a new start |
| `monotonic_sequence` | The highest journal sequence number the anchor covers | Accepted only as the exact successor (`seq = current_sequence + 1`) or as an idempotent repeat of an already pinned `(seq, digest)` pair. Any other value is rejected (`GAP`, `NOT_MONOTONIC`, `CONFLICT`) |
| `head_digest` | The hash of the journal record at `monotonic_sequence` | Must chain to `prev_digest`, the digest pinned at `seq - 1`, and must be recomputable from that record |
| `checkpoint_certificate` | Signed by the anchor over `(namespace_id, incarnation, seq, head_digest, anchor_log_position, challenge)` | Issued on every append and on every read |
| Authentication | The witness signs each append with its own asymmetric key; the anchor signs certificates with its own key | The two keys are held in different administrative domains (`RECOVERY_ANCHOR_THREAT_MODEL.md`, CI2) |
| No rollback of an accepted position | No lower, set, reset, delete, truncate, or rewrite operation | A request below the current position gives `NOT_MONOTONIC`; the same sequence with another digest gives `CONFLICT` |
| Provable continuity | The anchor keeps **every** accepted `(seq, digest)` pair, not only the latest | A verifier can check a whole journal against the pins, so a valid-looking forged tail is detected (as in 0.1S P1/P4) |
| Denial on mismatch | Any disagreement between journal and anchor closes the gate | There is no "best effort" mode |

### Sequence rule (contract version 1)

```text
GROUP_COMMIT=DISABLED
MAX_SEQUENCE_INCREMENT=1
NEXT_SEQUENCE=current_sequence+1
REQUIRED_PINS=EVERY_ACKNOWLEDGED_RECORD
```

- One anchor append covers exactly one journal record.
- Every anchor append names its immediate predecessor `(seq - 1, prev_digest)`. That predecessor must equal the anchor's current pinned head.
- Every append carries the record's own digest, which a verifier can recompute from the journal record.
- Every acknowledged record therefore has its own pin. There is no acknowledged record whose digest is known to the anchor only through a later head.
- A request that would advance the sequence by more than one is rejected (`GAP`).

Group commit, meaning one anchor append covering several records, is **not** part of this contract. It is neither implemented nor tested. It is listed as a future research item in `FUTURE_ISOLATED_TEST_PLAN.md`, and it would need its own contract version and review.

### The anchor must confirm the history in use, not just a checkpoint

A certificate that only says "seq N exists" is not enough. Before the gate opens, or before any `RESERVED` is acted on, the issuer must:

1. Request a fresh certificate with its own random challenge. A cached or replayed certificate does not count.
2. Obtain the journal export it will rely on.
3. Verify the export is a valid hash chain starting from the incarnation's GENESIS.
4. Verify that the export's length equals the certificate's `seq` (or exceeds it by exactly the uncovered PREPARED record, which is not acted on), and that the record at `seq` hashes to the certificate's `head_digest`.
5. Verify every pinned `(seq, digest)` in the anchor's history against the export.

If any step fails, cannot be performed, or times out, the gate stays `CLOSED`. A certificate for a checkpoint the issuer cannot connect to its own history counts as a failure.

## 2. Acknowledgement boundary

```text
issuer --reserve--> witness
  T1  JOURNAL APPEND        (fsync)                       record PREPARED
  T2  HIGH-WATER ADVANCE    (exact successor)             record HW_COVERED
  T3  ANCHOR CONFIRMATION   (exact successor, cert)       record ANCHORED
  T4  ISSUANCE ACK          RESERVED + certificate        issuer may start PostgreSQL work
issuer: PostgreSQL ticket transaction, COMMIT
issuer --outcome--> witness: T1..T3 again for the outcome record
  T4' ISSUE_OK              only after the outcome record is ANCHORED
```

These are four separate writes to three stores (T1 journal, T2 high-water, T3 anchor) followed by a reply. **No two of them are one atomic transaction**, and no claim of atomicity is made.

Safety comes from three rules:
- **Ordering:** each write happens only after the previous one is durable.
- **Bounded lag:** at most one record sits between the journal and the anchor. The witness accepts no new write while a record is not yet anchored.
- **Acknowledgement only at the end:** no acknowledgement is sent before the last store has confirmed.

Steady-state invariant: `anchor.seq <= hw.seq <= journal.len <= anchor.seq + 1`. On recovery the witness rolls the lagging stores forward from the journal, in order, one record at a time, after verifying the chain.

Is the order achievable without false atomicity claims? **Yes, in design.** The 0.1S local model already ran the same pattern for journal and high-water (`append_first`, CW3, CW7B). This design adds one more forward-only step. Whether a concrete anchor provides the durability and rollback properties assumed here is not proven (`ANCHOR_ROLLBACK_PROTECTION=REQUIRED_NOT_PROVEN`).

### Classifying a reservation without an observed acknowledgement

When the issuer sends `reserve` and does not observe `RESERVED`, it does **not** know whether the journal append happened. Each of these leaves the question open:
- no response;
- a timeout;
- a dropped connection;
- an issuer crash.

An unobserved acknowledgement is never treated as proof of absence. The request is classified only by a later **authoritative read**, which needs all of the following:
- The witness is `CONSISTENT` against a fresh, challenge-bound anchor certificate obtained **after** the failure (§3 rule 9).
- `journal.len == anchor.seq`: no PREPARED or uncovered record exists.
- The issuer's own history check (§1 steps 1–5) passes on that export.
- The witness that might have been processing the original request has restarted, or has completed every request it accepted before the read. Writes are serialized, so after this no earlier request can still append.

| Case | Condition | Result |
| --- | --- | --- |
| **A. Before the first write** | The authoritative read shows no `RESERVE` record for `(issuer_namespace_id, request_id)`, and no PREPARED tail exists | `RESERVATION_STATE=ABSENT`, `REQUEST_ID=AVAILABLE_SUBJECT_TO_AUTHORITY_CHECK`. The issuer may send a **new** `reserve`. Only a fresh `RESERVED` from the witness lets PostgreSQL work begin, and the witness re-applies uniqueness, epoch, and gate checks to it. There is no `IN_DOUBT` and no reconciliation in this case |
| **B. Record exists, acknowledgement not observed** | The read shows a `RESERVE` record for the key, whether PREPARED, HW-covered, or anchored | `RESERVATION_STATE=PENDING_OR_IN_DOUBT`, `SECOND_RESERVATION=DENIED`. While unanchored the state is `PENDING`: the witness blocks writes and rolls forward. Once anchored, the key is `RESERVED`; the issuer did not see the acknowledgement, so it moves the key to `IN_DOUBT` and it is reconciled. A new `reserve` gets `DUPLICATE` |
| **C. Outcome unknown, absence not proven** | No authoritative read is possible. Causes include: witness or anchor unreachable, witness `FAILED` or `PENDING`, history check failing, `HANDOVER_PENDING`, or a known restore not yet reconciled | `ISSUANCE_GATE=CLOSED`, `REQUEST_ID=NOT_RELEASED`. No new `reserve` is sent as if the key were free, and no PostgreSQL work starts. The request stays in C until a later authoritative read puts it in A or B |

Case A also covers a record that was appended but lost before it was anchored, because the authoritative read cannot distinguish it from no append. This is safe only because of the boundary above: such a record was never acknowledged, so no PostgreSQL transaction can have started for it. If an acknowledgement for that record is later found, the key falls under §4: the journal must be recovered from a copy that chains to the anchor, and is never regenerated.

### Transition analysis

Notation: "stale snapshot" means the local stores (journal, high-water, PostgreSQL) are restored to a point before the transition while the anchor survives. If the anchor is restored too, see Scenario B in `P3B_RECOVERY_DECISION.md`.

| Transition | Crash before | Crash after | Lost response | Retry | Unknown result | Issuer restart | Stale snapshot restored |
| --- | --- | --- | --- | --- | --- | --- | --- |
| T1 journal append | Nothing written; no reply. The issuer is in **case C** until an authoritative read; that read then gives **case A** | Record PREPARED in journal only; never acknowledged. The issuer is in case C until a read; the read then gives **case B** (`PENDING`, then `RESERVED` after roll-forward) | (internal) | Restarted witness rolls it forward through T2 and T3, or fails closed if the chain does not verify | Case C: gate CLOSED for this request, `REQUEST_ID=NOT_RELEASED`, no PostgreSQL work. Only an authoritative read moves it to A or B | Issuer treats every request it had in flight as case C and classifies each by authoritative read: A means send a new `reserve`; B means `DUPLICATE`, then `IN_DOUBT` and reconciliation; an unreadable store keeps it in C | Record not anchored, so never acknowledged, so no ticket exists. If the restore is known, the 0.1P restore procedure closes the gate (case C). If it is undetected, the read gives case A, which is safe for the reason stated above |
| T2 high-water advance | Same as "T1 after" | HW covers it; anchor does not | Witness re-sends the same `(seq, digest)` and gets `IDEMPOTENT` | Idempotent by design (0.1S T9 `idempotent_current`) | Witness state `PENDING`; no further writes; no acknowledgement. Issuer: case C, then case B after a read | As T1 | Record not anchored: same as T1. If the HW store is restored below the journal by more than one record, the witness gets `UNCOVERED_SUFFIX` and fails closed (0.1S P2A) |
| T3 anchor confirmation | Same as "T2 after" | Anchor covers it; witness has not replied. Issuer: case C, then case B | Witness re-sends and gets `IDEMPOTENT` plus a fresh certificate | Idempotent. The same seq with another digest gives `CONFLICT`, and the witness is FAILED | Witness state `PENDING` until it holds a certificate covering the record; gate CLOSED. Issuer: case C, then case B | As T1 | **The anchored record is now protected.** Restoring journal, HW, and PostgreSQL below it gives anchor.seq > journal length, so `ROLLBACK_DETECTED` and the gate is CLOSED (Scenario A) |
| T4 issuance ACK (`RESERVED`) | Record anchored, no reply. Issuer: case C, then case B | Issuer has the reply and certificate | Issuer: case C, then case B; no PostgreSQL work | Re-reserve gives `DUPLICATE`, then `IN_DOUBT` and reconciliation; never a second reservation | Treated as existing (0.1R R14); never released | As T1 (case B after the read) | As T3 |
| PostgreSQL COMMIT | No ticket | Ticket committed; outcome not yet journaled | Issuer does not know whether COMMIT succeeded | Reconcile against the PostgreSQL row; `IN_DOUBT → ABORT_CONFIRMED` is forbidden | `IN_DOUBT`; gate CLOSED until reconciled | Issuer finds `RESERVED` or `IN_DOUBT` and reconciles | Reservation anchored: detected. If PostgreSQL alone is stale, the anchored reservation still blocks reissue (0.1R `SR-*`) |
| Outcome T1–T3 | As for the reservation | As for the reservation | As for the reservation | As for the reservation | `ISSUE_OUTCOME_UNKNOWN`; never `ISSUE_OK` | Reconcile | If the outcome is lost but the reservation is anchored, the key returns to `RESERVED`, then `IN_DOUBT`, then reconciliation against PostgreSQL (`COMMIT_CONFIRMED`, or `PERMANENTLY_DENIED` when no row exists). No second ticket |
| T4' `ISSUE_OK` | Outcome anchored, no reply | Done | Issuer retry replays the same ticket ID | Replay | Replay only | Replay only | As T3 |

**Result:** no reply that lets the issuer start PostgreSQL work (`RESERVED`), and no `ISSUE_OK`, is ever sent before T3 has confirmed the record. The order `JOURNAL APPEND → HIGH-WATER ADVANCE → ANCHOR CONFIRMATION → ACK → POSTGRESQL BEGIN` is unchanged.

Every crash, lost response, or retry ends in one of three cases:
- **Case A** (proven absent): a new reservation is subject to the full authority check.
- **Case B** (record exists): a second reservation is denied.
- **Case C** (not yet classified): the gate stays closed and the request ID is not released.

An unknown outcome never releases a request ID by itself.

`BLOCKED_PROTOCOL_DESIGN` does **not** apply to the acknowledgement boundary. It applies to the recovery limits described next.

## 3. Detection and gate rules

Start-up and every gate decision compare the stores in this order. The first failing rule wins.

1. The anchor is unreachable, the certificate is invalid, or the challenge does not match: `ANCHOR_UNAVAILABLE`, gate CLOSED, retryable.
2. The anchor has no log for this namespace but the journal is non-empty: `ANCHOR_MISSING`, FAILED. The witness never re-initializes the anchor.
3. `journal.len < anchor.seq`: `ROLLBACK_DETECTED source=anchor`, FAILED, `HANDOVER_PENDING`.
4. The incarnation differs: `INCARNATION_MISMATCH source=anchor`, FAILED.
5. Any anchored pin differs from the journal: `FORK_DETECTED source=anchor`, FAILED.
6. The high-water comparison from 0.1S (rollback, fork, `UNCOVERED_SUFFIX`) applies unchanged.
7. `journal.len > anchor.seq + 1`: `UNCOVERED_SUFFIX source=anchor`, FAILED.
8. `journal.len == anchor.seq + 1`: roll forward T2 and then T3 for that single record. Until both succeed, the state is `PENDING` and the gate is CLOSED.
9. Everything equal: the anchor half of the gate is satisfied. The other 0.1P conditions still apply: no `RESERVED` or `IN_DOUBT` keys, the PostgreSQL tombstones match, handover is not pending, and the epoch is current.

`FAILED` and `HANDOVER_PENDING` are left only by the recovery procedure below. They are never left by a restart, a timeout, or a person's approval alone.

## 4. Recovery after detection

The anchor stores digests, not records. After `ROLLBACK_DETECTED source=anchor`, it proves that history beyond the restored journal existed. It does **not** say which `request_id` values that history contained.

1. Close: the fencing controller sets `HANDOVER_PENDING`. The gate is CLOSED for every `request_id` in the namespace, because the missing records may contain any of them.
2. Locate a copy: search any journal copy (off-site backup, replica, export, auditor's copy) for the missing segment. The copy itself does not need to be trusted.
3. Verify the copy:
   - its records chain from the restored journal head;
   - every anchored pin matches;
   - the final record hashes to the anchor's current `head_digest` under the same incarnation;
   - every record's signature verifies.

   A copy that ends below the anchor's head is incomplete and is rejected.
4. Re-append: the verified segment is restored into the journal byte for byte, and the high-water mark is rolled forward to it. Nothing is re-signed, renumbered, or regenerated.
5. Reconcile PostgreSQL: every reservation in the recovered history is compared with the restored tickets.
   - A reserved key without a ticket is reconciled through `IN_DOUBT` to `PERMANENTLY_DENIED` or to a denial tombstone.
   - It is **not** re-issued.
6. Open: only through 0.1P phase E. A fresh anchor certificate must match the recovered journal.

If no complete, verifiable copy exists, **history completeness cannot be established**. The namespace stays CLOSED indefinitely. No automatic recovery exists, and a person's approval does not replace the proof.

Moving to a new namespace (0.1P "separate administrative workflow") does not make old `request_id` values safe. They are safe only if the new namespace can show they are disjoint from the old history, and digest-only anchoring cannot show that. This is `BLOCKED_PROTOCOL_DESIGN` **for recovery without the full history**. It is resolved only by an anchor that also stores the records or their keys (variant classes 1 and 2 in `P3B_RECOVERY_DECISION.md`), or by a request-ID scheme that is provably disjoint across namespaces. Neither is selected here.

## 5. What this protocol does not do

- It does not detect a consistent rollback that **includes the anchor** (Scenario B).
- It does not fence an old primary's PostgreSQL COMMIT (`OLD_PRIMARY_FENCING=NOT_PROVEN`).
- It does not prevent direct PostgreSQL writes (`DIRECT_PG_BYPASS=OPEN`).
- It does not establish durability, failure-domain independence, or availability of any concrete anchor.
- It adds one synchronous, cross-domain round trip to every reservation and every outcome. If the anchor is unavailable, issuance is unavailable, by design.
