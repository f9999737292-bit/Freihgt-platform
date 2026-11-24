# 0.1S: High-water protocol (test-only model)

`EXTERNAL_HIGH_WATER_MODEL`: a separate test-only authority process that keeps an append-only, hash-chained, signed history of marks for each witness journal.

`HIGH_WATER_INDEPENDENCE=LOGICAL_ONLY`: the authority has a separate process, store directory, OS lock, keys, and pipe authkey, but runs on the same machine as the witness journal.

## Two separately managed states

| | A. Witness journal (`witness_server.py`) | B. High-water authority (`hw_server.py`) |
| --- | --- | --- |
| Holds | GENESIS, EPOCH, RESERVE, OUTCOME records | Per journal: INIT, then one ADVANCE per journal record |
| Record fields | `seq`, `prev_hash`, `hash`, full signed request, type-specific fields | `seq`, `prev_hash`, `hash`, signed `witness_svc` request, `journal_id`, `journal_incarnation`, `mark_seq`, `mark_digest`, `prev_digest` |
| Writers | Issuers (reserve, own outcomes) and operator (epoch, operator outcomes, checkpoint) | Only `witness_svc`. Issuers, operator, and auditor are readers; `harness` may only shut it down |
| Store | `witness_log.jsonl` + `accepted_checkpoint.json` | `hw_log.jsonl` (different directory) |
| Keys | Witness key set + witness pipe authkey | Separate key set + separate pipe authkey |
| On load | Full local verification (as 0.1R), then comparison with the authority | Full verification: parse, sequence, chain, hash, request MAC, record/request agreement, INIT/ADVANCE replay. Any error puts it in `FAILED` mode |

### Mark contents

Each mark contains:
- `issuer_namespace_id`: not part of the mark. Every RESERVE record carries `ns` and `request_id`, and the head digest commits to all of them through the hash chain. One journal can serve several namespaces. A sharded design with one journal per namespace would need one mark per journal.
- `incarnation_id` (`journal_incarnation`): a random UUID written into the journal's GENESIS record and pinned by the authority's INIT. It never changes for a journal ID.
- `sequence` (`mark_seq`): the highest journal sequence number covered.
- `head_digest` (`mark_digest`): the hash of the journal record at `mark_seq`.
- Confirmation state: a journal record with `seq <= mark_seq` is **COVERED**. One record with `seq == mark_seq + 1` is **PREPARED** (journaled, never acknowledged). Any larger gap is invalid.
- History: every `(seq, digest)` ever accepted, not just the latest mark.

### Authority rules

- `init`: only once per journal ID, only at `seq 0` with the genesis digest. A repeat gives `ALREADY_INITIALIZED`.
- `advance`: allowed only as the exact successor (`seq = current + 1`, `prev_digest = current digest`, same incarnation). The other outcomes are:
  - A lower sequence number gives `NOT_MONOTONIC`.
  - The same sequence number with a different digest gives `CONFLICT`.
  - Skipping ahead gives `GAP`.
  - A wrong previous mark gives `PREV_MISMATCH`.
  - A different incarnation gives `INCARNATION_MISMATCH`.
  - Re-sending an already accepted pair gives `IDEMPOTENT`, with nothing appended.
- No `set`, `lower`, `reset`, `delete`, `truncate`, `rollback`, or `rewrite` operation exists (`UNSUPPORTED_OP`).
- Each operation is atomic within the authority, under a single lock with one fsynced append. This is **not** atomic with the journal append.

Evidence: `T9-LOWER-REJECTED` and `T10-SINGLE-WRITER`.

## Write protocol (selected: `append_first`)

```text
issuer --reserve--> witness
  1 validate (role, epoch holder, epoch, uniqueness)
  2 build record seq = mark+1, chained to the pinned head
  3 append + fsync to journal                -> record is PREPARED
  4 authority.advance(seq, digest, prev)     -> record is COVERED
  5 reply RESERVED (only now)
```

- A reservation counts as **durable for issuance only after step 4**. The issuer may start the PostgreSQL transaction only after receiving `RESERVED`, and `RESERVED` is sent only after step 4.
- If step 4 fails because the authority is unreachable or failed, the witness replies `HIGH_WATER_UNAVAILABLE` with `durability=PREPARED`. It then accepts no further write until the PREPARED record is covered, so the journal is never more than one record ahead.
- Every other rejection at step 4 puts the witness in `FAILED`.
- The issuer treats `HIGH_WATER_UNAVAILABLE`, a lost connection, or a timeout on reserve as an unknown outcome (`ISSUE_WITNESS_OUTCOME_UNKNOWN`). It opens no PostgreSQL transaction.
- Outcomes follow the same order. An outcome that is not covered is reported to the issuer as `ISSUE_OUTCOME_UNKNOWN`, never `ISSUE_OK`.

Comparison order `hw_first` (advance, then append) is implemented only for CW4. It is safe but not live; see `CRASH_WINDOW_MATRIX.md`.

## Reconciliation of journal and authority (start-up and every request while not CONSISTENT)

The comparison runs in this order:

1. The authority is unreachable or `FAILED`: `HIGH_WATER_UNAVAILABLE`. This is retryable, and no writes are served.
2. The authority has no mark and the journal is empty: initialize (INIT, GENESIS, advance to 1). The authority has no mark and the journal is non-empty: `FAILED HIGH_WATER_MISSING`. The witness never re-initializes.
3. The journal is shorter than the mark: `FAILED ROLLBACK_DETECTED`.
4. The journal incarnation differs from the mark: `FAILED INCARNATION_MISMATCH`.
5. Any historical pinned digest differs from the journal record at that sequence number: `FAILED FORK_DETECTED seq=<first mismatch>`.
6. The journal length equals the mark: `CONSISTENT`.
7. The journal is exactly one record longer than the mark: roll forward by advancing the mark to that record, which already chains from the pinned head. The witness becomes `CONSISTENT` (`roll_forward` event).
8. The journal is longer by more than one record: `FAILED UNCOVERED_SUFFIX`.

`FAILED` is permanent for the process. There is no automatic recovery; only an operator action outside this model (for example, putting back a complete journal copy, as in `SP-GATE-CLOSED-UNTIL-RECONCILED`) followed by a restart can clear it.

### Issuer's independent check

Before every issuance the issuer:
- Reads the mark itself, using its own reader key on the authority.
- Verifies the exported journal chain.
- Requires equal incarnation, length, head digest, and every pinned digest.

Otherwise it returns `ISSUE_DENIED_HISTORY_UNVERIFIED`, `ISSUE_DENIED_HIGH_WATER_UNAVAILABLE`, or `ISSUE_DENIED_WITNESS_FAILED` before any PostgreSQL access. This does not depend on the witness reporting its own state honestly; see `T7-ISSUER-OWN-HW-CHECK`.

## Witness state machine (unchanged from 0.1R)

States: `RESERVED`, `COMMIT_CONFIRMED`, `ABORT_CONFIRMED`, `IN_DOUBT`, `PERMANENTLY_DENIED`. Each key `(issuer_namespace_id, request_id)` has one RESERVE and then only outcomes bound to its ticket ID.

| Role | From | To | Condition |
| --- | --- | --- | --- |
| issuer (owner) | `RESERVED` | `COMMIT_CONFIRMED` | `pg_commit_ack=true` |
| issuer (owner) | `RESERVED` | `ABORT_CONFIRMED` | `pg_rollback_ack=true` |
| issuer (owner) | `RESERVED` | `IN_DOUBT` | `reason` |
| issuer (owner) | `ABORT_CONFIRMED` | `COMMIT_CONFIRMED` | resume of the same reservation |
| operator | `RESERVED` | `IN_DOUBT` | `reason` |
| operator | `IN_DOUBT` | `COMMIT_CONFIRMED` | tombstone ticket ID and payload hash match |
| operator | `IN_DOUBT` | `PERMANENTLY_DENIED` | `reason` |
| operator | `ABORT_CONFIRMED` | `PERMANENTLY_DENIED` | `reason` |

`IN_DOUBT → ABORT_CONFIRMED` is impossible for every role. Every reconciliation in this pack first attempts it and observes `INVALID_TRANSITION`.

In addition, the witness-level gate states (orthogonal to key states) are:

| Witness state | Health / export status | Serves writes | Leaves by |
| --- | --- | --- | --- |
| `CONSISTENT` | `OK` | yes | authority unreachable on write (to `PENDING`), any comparison failure (to `FAILED`) |
| `PENDING` (one PREPARED record) | `HIGH_WATER_UNAVAILABLE` | no | successful roll-forward (to `CONSISTENT`) |
| `HW_UNAVAILABLE` | `HIGH_WATER_UNAVAILABLE` | no | successful comparison (to `CONSISTENT`) or mismatch (to `FAILED`) |
| `FAILED` | `WITNESS_FAILED` with reason | no | none within the process |

Every write operation, `health`, and `export` re-runs the comparison while the witness is not `CONSISTENT`. The issuer accepts an export only when its status is `OK` **and** its own check of the mark passes.
