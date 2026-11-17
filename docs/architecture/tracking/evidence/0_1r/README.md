# 0.1R — External witness failure proof (local, test-only)

Stage: `TRACKING-BINDING-EXTERNAL-WITNESS-FAILURE-PROOF-0.1R` (BINTRANS, Agent C).
Reviewer: Agent J.

This pack tests what an external reservation witness for
`(issuer_namespace_id, request_id)` can and cannot provide in front of the
PostgreSQL ticket write described in the 0.1P architecture (PR #386,
`994bc8a24261a952d2b8b120427b21b4b1200e52`). It builds on the 0.1Q fencing
evidence (PR #387, `4c296a57a2a8366019aac693ae2f883c9d8d624d`) and does not modify it.

Nothing here is product code. No platform migration, real database role,
staging or production resource was touched. The witness is a **test-only**
implementation. It is not a production witness design and must not be read as one.

## Result in one paragraph

Three recorded attempts each ran 86 assertions: 86 PASS, 0 FAIL, 0 NOT_RUN.
Every attempt reproduced the same counterexample, **F-TAIL-LOSS**: if the
witness store loses records written after its last operator-accepted checkpoint,
the witness cannot detect the loss and accepts a second reservation for a key it
had already reserved. Everything else held in this model:
- Reservation always came before any PostgreSQL transaction.
- Concurrent reserves produced exactly one reservation.
- Unknown outcomes stayed `IN_DOUBT` and were never converted to `ABORT_CONFIRMED`.
- A stale PostgreSQL restore did not produce new authorizations.
- An unavailable or corrupted witness denied issuance without touching PostgreSQL.

The witness does **not** fence an old primary's PostgreSQL COMMIT, and it does
**not** prevent a PostgreSQL-only write; the gate only detects that write afterwards.
`WITNESS_FAILURE_DOMAIN=NOT_PROVEN`; see `FAILURE_DOMAIN.md`.

## Layout

| Path | Content |
| --- | --- |
| `common.py` | States, role map, allowed transitions, canonical JSON, HMAC signing, payload hash, witness client |
| `witness_server.py` | Test witness process: exclusive store lock, hash-chained log, verification on load, operations |
| `issuer_worker.py` | One issuer process per request, following 0.1P Transaction A, with fault points |
| `pgsession.py` | `psql` helpers over `docker exec` (one-shot and interactive session) |
| `sql/schema.sql` | Disposable ticket table (`UNIQUE(issuer_namespace_id, request_id)`) |
| `run_proof.py` | Orchestrator: container, witness processes, scenarios, assertions, cleanup |
| `verify_manifest.py` | MANIFEST / SHA256SUMS generation and verification against committed Git blobs |
| `attempts/attempt_{1,2,3}/` | Recorded attempts (raw evidence) |
| `attempts/debug_failed_1_harness_error/` | First debug attempt, failed on a harness bug (kept as evidence) |
| `attempts/summary.json`, `attempts/run_console.txt` | Aggregate result and console output of the recorded run |
| `FAILURE_DOMAIN.md` | What this local model cannot show |
| `FUTURE_WITNESS_REQUIREMENTS.md` | Requirements matrix for a future external store (no provider chosen) |
| `MANIFEST`, `SHA256SUMS` | Evidence manifest and checksums |

Each attempt directory contains:
- `assertions.json`: every assertion with its observed value.
- `RESULTS.md`: a summary of the assertions.
- `run_meta.json`: image ID, server version, network mode, cleanup flags, findings.
- `pg_server_log.txt`: full PostgreSQL server log with `log_connections`, which shows which application names actually opened PostgreSQL sessions.
- `setup/`: container and schema commands with stdout, stderr, and exit code.
- `traces/<scenario>.json`: ordered harness events and witness responses.
- `workers/<tag>.{config.json,stdout.txt,stderr.txt,exit}`: raw issuer process output. `EVENT` and `RESULT` lines are the fault-injection traces.
- `witness/<name>/`: the witness log `witness_log.jsonl`, the accepted checkpoint, the tamper description, and raw server stdout, stderr, and exit code for every server run.

## Test environment

- PostgreSQL: `postgres:16` image (PostgreSQL 16.15), container started with `--network none` and
  `--pull never`, no published ports, label `bintrans.proof=0.1r`, random
  container and database names per attempt, removed at the end of each attempt
  (`HYGIENE-CONTAINER-REMOVED`).
- PostgreSQL credentials:
  - A random per-attempt password is passed by environment variable name only and never written to disk.
  - Local socket access only.
  - Roles `wproof_admin` (bootstrap) and `wproof_issuer` (`SELECT, INSERT` on `tickets`) exist only inside the container.
- Witness:
  - A separate Python process on the host (Python 3.12, Windows).
  - Its store and checkpoint directories live in a per-attempt temp directory outside the repository, deleted after copying the evidence (`HYGIENE-TMP-REMOVED`).
  - IPC uses a local named pipe with a random authkey.
  - Every request is HMAC-signed with a random per-attempt key for each principal.
  - Keys are never written into evidence (`HYGIENE-SECRET-SCAN`).
- No real credentials, data, staging or production endpoints were used.

## Witness model (test-only)

### State machine

Unique key: `(issuer_namespace_id, request_id)`. A key gets exactly one
`RESERVE` record and then only `OUTCOME` records bound to that reservation's
`ticket_id`. History is never deleted. There is no update, delete, truncate,
rewrite or release operation (`W1-NO-*`, all `UNSUPPORTED_OP`). There is no
timeout-based release (`W1-NO-TIMEOUT-RELEASE`).

| Role | From | To | Condition |
| --- | --- | --- | --- |
| issuer (reservation owner) | `RESERVED` | `COMMIT_CONFIRMED` | evidence `pg_commit_ack=true` |
| issuer (reservation owner) | `RESERVED` | `ABORT_CONFIRMED` | evidence `pg_rollback_ack=true` |
| issuer (reservation owner) | `RESERVED` | `IN_DOUBT` | evidence `reason` |
| issuer (reservation owner) | `ABORT_CONFIRMED` | `COMMIT_CONFIRMED` | resume of the same reservation and ticket id |
| operator | `RESERVED` | `IN_DOUBT` | evidence `reason` |
| operator | `IN_DOUBT` | `COMMIT_CONFIRMED` | tombstone `ticket_id` and `payload_hash` equal the reservation |
| operator | `IN_DOUBT` | `PERMANENTLY_DENIED` | evidence `reason` |
| operator | `ABORT_CONFIRMED` | `PERMANENTLY_DENIED` | evidence `reason` |

Every other combination returns `INVALID_TRANSITION`. That includes
`IN_DOUBT → ABORT_CONFIRMED` for both roles, and every transition out of
`COMMIT_CONFIRMED` or `PERMANENTLY_DENIED`. An outcome equal to the current
state returns `IDEMPOTENT` and appends nothing. Outcomes with a different
`ticket_id` return `TICKET_MISMATCH`; outcomes from a non-owner issuer return
`NOT_RESERVATION_OWNER`.

Assertion `W1-TRANSITION-MATRIX` drives all 40 combinations of the 5 source
states, 4 targets and 2 roles against a live witness and compares them with the
table above. `W1-IN-DOUBT-NEVER-ABORT` and `W1-TERMINAL-STATES` restate the
critical rows separately.

### Authority boundary

| Principal | Role | May | May not |
| --- | --- | --- | --- |
| `issuer_a`, `issuer_b` | issuer | reserve (only as current epoch holder, with the current epoch); append outcomes for **its own** reservation | reserve as non-holder or with a stale epoch (`NOT_EPOCH_HOLDER` / `STALE_EPOCH`); touch another issuer's reservation; set epoch; accept checkpoint |
| `operator` | operator | set the fencing epoch (strictly increasing); append `IN_DOUBT` / `PERMANENTLY_DENIED`; append `COMMIT_CONFIRMED` only with a matching PostgreSQL tombstone; accept a checkpoint at the current head | reserve (`ROLE_FORBIDDEN`); lower the epoch (`EPOCH_NOT_MONOTONIC`); lower the checkpoint (`CHECKPOINT_LOWER_REJECTED`); accept a checkpoint with a wrong head hash |
| `auditor` | reader | export / health | any write |
| `harness` | fault | arm witness faults, shut down | any record write |
| everyone | — | — | change past records (no such operation; tampering on disk is detected on load, see below) |

An issuer process receives only its own HMAC key. The harness holds all keys,
because it plays the operator; see `FAILURE_DOMAIN.md`.

### Uniqueness and sequence protection

- **Single writer:** the server holds an exclusive OS lock on its store for its
  whole lifetime. A second server on the same store exits with code 3 and
  `store_locked` (`W2-SINGLE-WRITER`). Requests are serialized by one in-process
  lock, and the uniqueness check and the append happen under that lock.
- **Concurrency:** 3 rounds of 8 issuer processes reserved the same key with
  a start barrier. Each round produced exactly one `RESERVED`, seven
  `DUPLICATE`, and one `RESERVE` record in the log (`W2-RACE-1..3`, all three attempts).
- **Log integrity:**
  - Each record carries `seq`, `prev_hash`, `hash` (SHA-256 over canonical JSON) and the full signed request. The file is fsynced after each append.
  - On load the server verifies parseability, sequence, chain, hash, request MAC, record/request field agreement, state-machine replay, and the operator-signed checkpoint.
  - Any failure puts the witness in `WITNESS_FAILED` mode, where every request is answered with `WITNESS_FAILED`.

## Issuer flow under test (0.1P Transaction A)

1. Export witness history.
2. Restart recovery: the issuer's own `RESERVED` records from an older incarnation are moved to `IN_DOUBT`.
3. Look up the key:
   - `COMMIT_CONFIRMED` replays only if the PostgreSQL row matches; otherwise `RECONCILIATION_REQUIRED`.
   - `RESERVED` or `IN_DOUBT` is denied.
   - `PERMANENTLY_DENIED` is denied.
   - `ABORT_CONFIRMED` resumes the same reservation.
   - A payload mismatch is denied.
4. Gate:
   - The issuer must be the current epoch holder.
   - There must be no open (`RESERVED` or `IN_DOUBT`) reservations in the namespace.
   - Every `COMMIT_CONFIRMED` must have an identical PostgreSQL row.
   - There must be no PostgreSQL row without a committed reservation.
5. Reserve.
6. PostgreSQL `BEGIN; INSERT; COMMIT`.
7. Append outcome.
8. Respond.

The witness reservation and the PostgreSQL COMMIT are two independent commit
steps. They are not modeled as atomic, and every gap between them is a scenario below.

## Scenarios and answers

Each row lists the assertion IDs (identical in all three attempts) and answers
the four required questions:
1. Can a second ticket arise?
2. When must the gate stay CLOSED?
3. What data does reconciliation need?
4. When is only `PERMANENTLY_DENIED` allowed?

| Scenario | Assertions | Observed | Second ticket? | Gate CLOSED while | Reconciliation needs | Only `PERMANENTLY_DENIED` when |
| --- | --- | --- | --- | --- | --- | --- |
| Normal issue | `S0-ISSUE-OK`, `S0-RESERVE-BEFORE-PG` | Event order: `witness_reserve` before `pg_begin_sent`; row has the witness `seq` | no | — | — | — |
| A: reserved, PostgreSQL not started (crash) | `SA-*` | Exit 137; key `RESERVED`; no PostgreSQL row; **0** `-txn` connections in the server log. The restarted issuer moves the key to `IN_DOUBT` and is denied; other keys are denied (`OPEN_RESERVATIONS`) | no | the key is `RESERVED` or `IN_DOUBT` | witness reservation (ticket id, payload hash) + PostgreSQL lookup by key | no tombstone row in PostgreSQL |
| Crash before PostgreSQL COMMIT | `SPC-*` | INSERT done, process killed before COMMIT; PostgreSQL rolled back; key `RESERVED`, then `IN_DOUBT`, then `PERMANENTLY_DENIED`; retry denied | no | as A | as A | as A |
| B: PostgreSQL COMMIT OK, outcome missing (crash) | `SB-*` | Row exists, key `RESERVED`, then `IN_DOUBT`; retry and other keys denied; operator `COMMIT_CONFIRMED` with a wrong tombstone gives `EVIDENCE_MISMATCH`; with the matching tombstone gives `APPENDED` | no | until operator confirms against the tombstone | row `ticket_id` + `payload_hash` = reservation | never in this case (row matches) |
| C: PostgreSQL ROLLBACK, reservation remains | `SC-*` | `ABORT_CONFIRMED` with `pg_rollback_ack`; new reserve for the key gives `DUPLICATE`; retry with a different payload gives `PAYLOAD_MISMATCH`; retry with the same payload resumes the **same** reservation and ticket id | no | — | — | operator may close it with `PERMANENTLY_DENIED` |
| D1: COMMIT result unknown, actually committed | `SD1-*` | Issuer appends `IN_DOUBT`; row exists; reconciliation gives `COMMIT_CONFIRMED` | no | key is `IN_DOUBT` | as B | — |
| D2: COMMIT result unknown, not committed | `SD2-*` | Backend terminated before COMMIT (SQLSTATE 57P01); `IN_DOUBT`; the automatic `ABORT_CONFIRMED` attempt is rejected (`INVALID_TRANSITION`); resolution `PERMANENTLY_DENIED`; retry denied | no | key is `IN_DOUBT` | absence of a row is the only signal and cannot prove "never commits", so abort is not allowed | always, once no matching row is found |
| E: HTTP response lost after outcome | `SE-*` | Retry returns `ISSUE_REPLAY_ISSUED` with the same ticket id; no new witness records; different payload gives `PAYLOAD_MISMATCH` | no | — | — | — |
| Witness crash after append, before reply | `SL-*` | Witness exit 137; issuer sees `io:EOFError`, reports `ISSUE_WITNESS_OUTCOME_UNKNOWN`, opens **0** PostgreSQL transactions; after restart the reservation is durable (`RESERVED`), then `IN_DOUBT`, then `PERMANENTLY_DENIED` | no | key open | as A | no row |
| Witness unavailable | `SU-*` | Witness stopped; issuer reports `ISSUE_DENIED_WITNESS_UNAVAILABLE`, **0** PostgreSQL connections of any kind; no records added | no | always while unavailable | — | — |
| Stale PostgreSQL restore, witness history kept | `SR-*` | Dump after k1 issued; k2 issued; k3 crashed after COMMIT; restore of the old dump (k2, k3 rows gone). k2 retry gives `RECONCILIATION_REQUIRED` (not re-issued); k3 goes to `IN_DOUBT` then `PERMANENTLY_DENIED`; new key k4 is gated (`CONFIRMED_TICKET_MISSING_OR_DIFFERENT_IN_PG:k2`). PostgreSQL alone would have accepted a new k2 row (`SR-PG-ALONE-WOULD-ACCEPT`, rolled back) | no, while the witness history is intact | any confirmed ticket missing from PostgreSQL | witness history vs PostgreSQL rows | lost keys whose ticket cannot be restored |
| Split brain (two issuers, one history) | `SS-*` | Epoch moved A to B. A's stale issuance is gated and its direct reserve gives `NOT_EPOCH_HOLDER`; B issues. **A's PostgreSQL COMMIT for its earlier reservation still succeeds and its outcome is accepted** (`SS-OLD-COMMIT-NOT-FENCED`). A's PostgreSQL-only write (no reservation) succeeds; B's gate then reports `TICKET_IN_PG_WITHOUT_RESERVATION` (`SS-BYPASS-DETECTED-NOT-PREVENTED`). Races of 4 old-epoch A + 4 B processes: A always `NOT_EPOCH_HOLDER`, exactly one B `RESERVED` | possible via PostgreSQL-only writes; not prevented by the witness | any PostgreSQL row without a committed reservation | full row set vs history | — |
| Witness corruption | `SX-*` | Bit flip, rewritten record with recomputed hashes, rewritten signed request, reordered lines, truncation below the checkpoint, forged checkpoint, garbage line: all give `WITNESS_FAILED` with the exact reason; the issuer is denied with 0 PostgreSQL connections | no | while `WITNESS_FAILED` | out-of-band trusted copy of the history | — |
| Witness tail loss after checkpoint | `SX-TAIL-LOSS-UNDETECTED` | Truncated to the accepted checkpoint: witness reports `OK`; a lost reserved key (k2) is **reserved again** (`RESERVED`) | **yes, possible**: protection then rests only on the PostgreSQL unique constraint, which a stale PostgreSQL restore removes (combination not executed; see F-TAIL-LOSS) | cannot be detected locally | an external, independently stored checkpoint at or after the lost head | — |

## Findings

- **F-TAIL-LOSS** (reproduced in all three attempts):
  - What happened: the witness checks its log only against the last checkpoint the operator accepted. Records appended after that checkpoint can be lost (storage rollback, snapshot restore, truncated file) without detection, and the lost key accepted a second successful reservation.
  - Scope stop: per the stage rules, this is a two-successful-reservations finding, so scope expansion stopped here. The combination with a stale PostgreSQL restore, which would turn this into a second ticket, was **not** executed.
  - Requirement: a production witness needs an anchor for its head that is stored and verified independently of both the witness store and PostgreSQL, updated synchronously with every reservation.
- **Old-primary COMMIT is not fenced** (`SS-OLD-COMMIT-NOT-FENCED`): the witness only fences new reservations. `OLD_PRIMARY_FENCING=NOT_PROVEN`.
- **PostgreSQL-only writes are detected, not prevented** (`SS-BYPASS-DETECTED-NOT-PREVENTED`). Prevention would need database-side enforcement, which is out of scope here (see 0.1Q for role-level limits).
- **Nonce replay protection is in memory only** and resets on witness restart.
  Replayed requests are still bound by state (a replayed reserve gives
  `DUPLICATE`/`ALREADY_RESERVED_SAME`, a replayed outcome gives `IDEMPOTENT`), but no assertion targets
  cross-restart replay.

## Not proven here

See `FAILURE_DOMAIN.md` for the full list. In short:
- Independence of the witness from PostgreSQL, the host, its disk and its keys.
- Durability against power loss: directory entries are not fsynced on Windows, and no power-cut test was run.
- Tail-loss detection.
- Production-grade append-only storage. A file lock plus a hash-chained JSONL log is **not** one.
- Fencing of an old primary's COMMIT.
- Performance and availability.

## Attempts

| Attempt | Result | Notes |
| --- | --- | --- |
| `debug_failed_1_harness_error` | 26 PASS, 1 FAIL (`HARNESS-ERROR`) | Harness bug: the witness `fault` request used a body key `name` that collided with the orchestrator's witness-name parameter (`TypeError`). Renamed to `fault` in `run_proof.py` and `witness_server.py`. The run stopped after the state-machine scenario; container and temp dir were removed. |
| debug attempt 2 (not kept) | 86 PASS | Run outside the repository after the fix; identical assertions to the recorded attempts. |
| `attempt_1` | 86 PASS, 0 FAIL | finding F-TAIL-LOSS |
| `attempt_2` | 86 PASS, 0 FAIL | finding F-TAIL-LOSS |
| `attempt_3` | 86 PASS, 0 FAIL | finding F-TAIL-LOSS |

No assertion was weakened between the failed debug attempt and the recorded run.

## Reproduce

Requirements: Docker with a local `postgres:16` image (pulls are disabled),
Python 3.12+, no third-party Python packages.

```powershell
cd docs/architecture/tracking/evidence/0_1r
python run_proof.py --attempts 3 --out <empty output dir outside the repository>
docker ps -a --filter "label=bintrans.proof=0.1r"   # expect no rows
python verify_manifest.py --verify --rev HEAD        # expect SHA256_MANIFEST_VERIFIED=YES
```

`run_proof.py` exits non-zero if any assertion fails. It removes only
the container and temp directory it created.
