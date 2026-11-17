# 0.1S: Witness rollback detection with an independent high-water mark

Stage: `TRACKING-BINDING-EXTERNAL-WITNESS-ROLLBACK-DETECTION-0.1S` (BINTRANS, Agent C).
Reviewer: Agent J.
Addresses: finding **J-WIT-001 (HIGH)**. In 0.1R (PR #388, `12560d702bec29c1ed6b764c64e955ac52f937f7`), witness records lost after the last operator checkpoint went undetected, and a key was reserved twice.

This is an isolated local experiment with test-only components:
- No product code changed.
- No platform migrations were added.
- No real database roles, staging, or production resources were touched.
- The 0.1R evidence and PR #388 are not modified.

## Result

Each of the three recorded attempts ran 68 assertions: **68 PASS, 0 FAIL, 0 NOT_RUN**. The first debug attempt is kept as evidence; see the Attempts section.

The invariant under test was: *no previously confirmed `(issuer_namespace_id, request_id)` gets a second successful reservation after the witness history is restored.* In this model it held for every case where the witness journal and the high-water authority were not **both** rolled back.

| Check | Result (all three attempts) |
| --- | --- |
| J-WIT-001 regression: journal truncated exactly to the old operator checkpoint, k2 re-reserved | `TAIL_LOSS_DETECTED=YES`, `DUPLICATE_RESERVATION_ACCEPTED=NO`, issuer gate CLOSED, 0 PostgreSQL connections |
| T1–T10 | All PASS (table below) |
| Crash windows CW1–CW8 | No `ISSUE_OK` while durability is unknown; unknown outcomes block reissue; nothing is ever auto-converted to `ABORT_CONFIRMED` |
| Witness tail loss combined with a stale PostgreSQL restore | Tail loss detected, second `RESERVED` refused, no second ticket. After the full journal is restored, the gate stays closed until PostgreSQL is reconciled |
| Privileged bypass | Journal-only and authority-only tampering is detected or fails closed. **Rolling back both stores together is not detected** (counterexample reproduced). With PostgreSQL rolled back as well, a **second ticket is issued**. See `PRIVILEGED_BYPASS.md` |

These results do **not** prove production durability, power-loss safety, or failure-domain independence. The witness journal and the authority ran as separate processes with separate stores, locks, and keys **on one machine**: `HIGH_WATER_INDEPENDENCE=LOGICAL_ONLY` and `WITNESS_FAILURE_DOMAIN=NOT_PROVEN`. `ISSUER_ISSUANCE_GATE` stays `CLOSED` for the platform.

## Layout

| Path | Content |
| --- | --- |
| `common.py` | States, role maps, transitions, canonical JSON, HMAC signing, record layout, chain verification, client |
| `server_base.py` | Exclusive store lock, fsync append, serialized named-pipe request loop |
| `hw_server.py` | Test-only external high-water authority |
| `witness_server.py` | Test-only witness journal with high-water reconciliation and fault points |
| `issuer_worker.py` | One issuer process per request (0.1P Transaction A) with its own high-water history check |
| `pgsession.py`, `sql/schema.sql` | Disposable PostgreSQL helpers and ticket table (copied from 0.1R, renamed) |
| `run_proof.py` | Orchestrator for every scenario |
| `verify_manifest.py` | MANIFEST / SHA256SUMS against committed Git blobs |
| `HIGH_WATER_PROTOCOL.md` | Mark model, state machine, write protocol, reconciliation rules, authority boundary |
| `CRASH_WINDOW_MATRIX.md` | Every stop point, both write orders, and what is allowed afterwards |
| `PRIVILEGED_BYPASS.md` | Journal / authority / both / credential compromise |
| `REMAINING_UNPROVEN.md` | Properties this pack does not establish |
| `attempts/attempt_{1,2,3}/` | Recorded attempts |
| `attempts/debug_failed_1_harness_design/` | First debug attempt (2 FAIL, kept as evidence) |
| `attempts/summary.json`, `attempts/run_console.txt` | Aggregate result and console output |

Each attempt directory contains:
- `assertions.json`: id, scenario, kind, description, observed value, result.
- `RESULTS.md`: the assertions as a table.
- `run_meta.json`: image ID, server version, network mode, cleanup flags.
- `pg_server_log.txt`: the full server log with `log_connections`, prefix `sqlstate=%e app=%a`. It is the source for "0 PostgreSQL connections" checks.
- `setup/`: commands with stdout, stderr, and exit code.
- `traces/<scenario>.json`: ordered calls and responses.
- `traces/*_pg_dump|dropdb|createdb|pg_restore.*`: the restore commands with their exit codes.
- `workers/<tag>.{config.json,stdout.txt,stderr.txt,exit}`: per-step `EVENT` lines, the `RESULT` line, and the process exit code. Exit 137 marks an injected crash, and the pg event lines include SQLSTATEs.
- `servers/<name>/`: each witness or authority log (`witness_log.jsonl`, `hw_log.jsonl`), the accepted checkpoint, a `tamper_*.json` description of each injected tampering, and raw stdout, stderr, and exit code per server run.

## Environment

- **PostgreSQL:** `postgres:16` (PostgreSQL 16.15, image `sha256:f1c3376c26f2609ab9f29f71f824103fe2fcd8ee0346485cb6122a4f93df6f94`).
  - Started with `--network none` and `--pull never`, no published ports, label `bintrans.proof=0.1s`.
  - Three disposable databases per attempt.
  - Removed after every attempt (`HYGIENE-CONTAINER-REMOVED`).
- **PostgreSQL credentials:** a random per-attempt password passed by variable name only.
- **Witness and authority:**
  - Python 3.12 processes on the Windows host.
  - Stores live in a per-attempt temp directory outside the repository, removed afterwards (`HYGIENE-TMP-REMOVED`).
  - Each server has its own random pipe authkey and per-principal HMAC keys, held in memory only.
  - No key, authkey, or password appears in any evidence file (`HYGIENE-SECRET-SCAN`).

## Assertions per attempt

Kinds: 17 `POSITIVE_CONTROL`, 13 `DETECTION`, 21 `DENIAL`, 15 `FAIL_CLOSED`, 2 `COUNTEREXAMPLE_REPRODUCED`. All 68 PASS in each attempt. A PASS on a `COUNTEREXAMPLE_REPRODUCED` assertion means the predicted unsafe behaviour **was** observed.

### T1–T10 (required rollback-detection tests)

| Test | Assertions | Observed |
| --- | --- | --- |
| T1 journal complete, mark equal | `T1-CONSISTENT` | Witness `CONSISTENT` after restart, mark seq 6 = journal 6. Re-reserving k2 gives `DUPLICATE`. A new key in a fresh namespace gives `ISSUE_OK` after the issuer's own `history_check complete=true` |
| T2 last confirmed record deleted | `T2-*` | `ROLLBACK_DETECTED source=high_water journal_len=5 high_water_seq=6`. k2 reserve gives `WITNESS_FAILED` with no append and an unchanged mark. Issuer gets `ISSUE_DENIED_WITNESS_FAILED` with 0 PostgreSQL connections |
| T3 truncated exactly to the old operator checkpoint | `T3-*` | `ROLLBACK_DETECTED ... journal_len=4 high_water_seq=6`, otherwise as T2 |
| T4 byte copy restored, one seq below the mark | `T4-*` | `ROLLBACK_DETECTED ... journal_len=5 high_water_seq=6`, otherwise as T2 |
| T5 same seq, different digest (valid signed record, valid chain) | `T5-*` | `FORK_DETECTED source=high_water seq=6`, otherwise as T2 |
| T6 journal from another incarnation (valid chain) | `T6A-*`, `T6B-*` | With the local operator checkpoint present: `FORK_DETECTED source=operator_checkpoint seq=4`. With the checkpoint also deleted: `INCARNATION_MISMATCH source=high_water`. Both deny without append |
| T7 high-water unavailable | `T7-*` | Down at witness start: `HIGH_WATER_UNAVAILABLE` and reserve refused without append. Back: `CONSISTENT` without restart. Lost while the witness is consistent: the issuer's own mark read fails and it is denied before PostgreSQL. Lost mid-write: k4 journaled as `PREPARED` but answered `HIGH_WATER_UNAVAILABLE` (not `RESERVED`), and the next write is blocked. On return: one roll-forward, mark 7, k4 `RESERVED` and not re-reservable |
| T8 high-water corrupted | `T8A/B/C-*` | Authority loads `FAILED` with `HW_HASH_MISMATCH`, `HW_RECORD_REQUEST_MISMATCH`, or `HW_REQUEST_MAC_INVALID`. Witness `HIGH_WATER_UNAVAILABLE`, reserve refused, issuer denied before PostgreSQL |
| T9 attempt to lower the mark | `T9-*` | lower gives `NOT_MONOTONIC`; same seq with another digest gives `CONFLICT`; skip gives `GAP`; wrong previous gives `PREV_MISMATCH`; other incarnation gives `INCARNATION_MISMATCH`; re-init gives `ALREADY_INITIALIZED`; issuer, operator, or auditor write gives `ROLE_FORBIDDEN`; set/lower/reset/delete/truncate/rollback/rewrite give `UNSUPPORTED_OP`. Mark and authority log unchanged; witness still serves |
| T10 concurrent reserve of one request ID | `T10-*` | 3 rounds × 8 processes: exactly one `RESERVED` and seven `DUPLICATE`, one RESERVE record, mark = journal length. A second authority or witness process on the same store exits 3 (`store_locked`) |

### J-WIT-001 regression (`J_WIT_001`)

Steps 1–9 of the assignment ran in order.

- `JW-SETUP`: the mark at seq 6 equals the journal head.
- `JW-LOCAL-CHECKS-PASS-ON-TRUNCATED` (control): the truncated 4-record journal is a valid chain matching the operator checkpoint. The 0.1R checks alone would accept it.
- `JW-TAIL-LOSS-DETECTED`: the restarted witness reports `ROLLBACK_DETECTED source=high_water journal_len=4 high_water_seq=6`.
- `JW-DUPLICATE-NOT-ACCEPTED`: reserving k2 gives `WITNESS_FAILED`, and the journal contains 0 k2 reservations.
- `JW-GATE-CLOSED`: the issuer is denied for k2 and for a new key, with 0 PostgreSQL connections.

### Crash windows, combined stale PostgreSQL restore, privileged bypass

See `CRASH_WINDOW_MATRIX.md` and `PRIVILEGED_BYPASS.md`.

The combined test (`STALE_PG_COMBINED`, run only after T1–T10 and J-WIT-001 passed) produced:
- `SP-SETUP`: k1 and k2 issued; after the old dump is restored, PostgreSQL holds only k1.
- `SP-TAIL-LOSS-DETECTED` and `SP-NO-SECOND-RESERVED`: the journal truncated to the checkpoint is detected.
- `SP-NO-SECOND-TICKET`: reusing k2 and issuing a new key are denied before PostgreSQL, and only k1 exists.
- `SP-GATE-CLOSED-UNTIL-RECONCILED`: after the complete journal copy is put back (simulating authorized recovery), the witness is `CONSISTENT`. Issuance still stays closed: k2 gives `ISSUE_DENIED_RECONCILIATION_REQUIRED`, and new keys give `GATE_CLOSED` with `CONFIRMED_TICKET_MISSING_OR_DIFFERENT_IN_PG:k2`.

## Attempts

| Attempt | Result | Notes |
| --- | --- | --- |
| `debug_failed_1_harness_design` | 58 PASS, 2 FAIL, 1 NOT_RUN | `T1-CONSISTENT`: the positive-control issue used the seeded namespace, whose confirmed keys have no PostgreSQL rows, so the gate correctly refused. The test now uses a fresh namespace. `T6-DETECTED`: the local operator checkpoint caught the re-incarnated journal before the high-water check (`FORK_DETECTED source=operator_checkpoint`). T6 is now split into T6A (checkpoint present, that reason expected) and T6B (checkpoint deleted, `INCARNATION_MISMATCH source=high_water` expected). The combined test was `NOT_RUN` by its precondition. |
| debug attempt 2 | 68 PASS | Not kept in this pack; superseded by the recorded attempts |
| `attempt_1`, `attempt_2`, `attempt_3` | 68 PASS each | No findings raised by the harness; counterexamples are recorded as assertions |

## Reproduce

Requirements: Docker with a local `postgres:16` image (pulls are disabled), Python 3.12 or later, standard library only.

```powershell
cd docs/architecture/tracking/evidence/0_1s
python run_proof.py --attempts 3 --out <empty directory outside the repository>
docker ps -a --filter "label=bintrans.proof=0.1s"    # expect no rows
python verify_manifest.py --verify --rev HEAD         # expect SHA256_MANIFEST_VERIFIED=YES
```

`run_proof.py` exits non-zero if any assertion fails. It removes only the container and temp directory it created.
