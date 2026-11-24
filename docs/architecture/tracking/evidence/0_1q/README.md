# TRACKING-BINDING-DB-IDENTITY-FENCING-PROOF-0.1Q

This is an isolated, test-only proof of database identity separation and reconnect-first fencing for the BINTRANS tracking-binding issuer. It tests the PostgreSQL mechanics behind the 0.1P contract, draft PR #386 at `994bc8a24261a952d2b8b120427b21b4b1200e52`. The handover runs phases A CLOSE, B DENY RECONNECT, C QUIESCE, D TRANSITION, and E OPEN, in that order.

The proof ran in three fresh disposable PostgreSQL 16.15 containers. Every attempt had 119 PASS assertions, 0 FAIL, and 2 recorded observations. All witness scenarios are `NOT_RUN`. Nothing here proves old-primary fencing, witness atomicity, tenant authority, or product role separation.

The test configures roles inside throwaway containers only. It does not change product code, platform migrations, real database roles, staging, or production.

## Scope and isolation

| Item | Value |
| --- | --- |
| Base | `origin/main` `3f306fc407aeda4bdc652d825954292f4a54fad6` |
| Image | `postgres:16`, image id `sha256:f1c3376c26f2609ab9f29f71f824103fe2fcd8ee0346485cb6122a4f93df6f94`, server `PostgreSQL 16.15 (Debian 16.15-1.pgdg13+2)` |
| Engine | Local Docker Desktop, engine 29.5.3 |
| Network | `--network none` (`HostConfig.NetworkMode=none`), no published ports (`PortBindings={}`), `--pull never` |
| Containers | `bintrans-proof-0-1q-99e796c3`, `bintrans-proof-0-1q-2b0867f3`, `bintrans-proof-0-1q-a08c1aa0`, each labeled `bintrans.proof=0.1q` and removed after its attempt (`docker rm -f` of that name only) |
| Databases | `proof_0_1q_<hex>` (main, no PUBLIC CONNECT) and `proof_pub_<hex>` (keeps PUBLIC CONNECT for one counterexample) |
| Secrets | Random per-attempt passwords, passed to Docker through the process environment and never on the command line. Every saved file is redacted, and `T09-SECRET-SCAN` checks that no generated value remains. No real DSN, the real `freight` login, or any staging or production credential was used. |

Connections from the harness are made inside the container. They use loopback TCP (`127.0.0.1`, `scram-sha-256`) or the Unix socket (`scram-sha-256`, with `trust` only for `proof_admin`). The `pg_hba.conf` used is in each attempt's `setup/02_pg_hba.conf`.

## Role model

These roles are defined in `sql/00_roles.sql`, with grants in `sql/01_schema.sql`.

| Role | Attributes | Purpose |
| --- | --- | --- |
| `proof_issuer_login` | LOGIN NOINHERIT | Old issuer (epoch 1 holder) |
| `proof_issuer_b_login` | LOGIN NOINHERIT | New issuer (epoch 2 holder) |
| `proof_writer_login` | LOGIN NOINHERIT | Business writer: `consume_ticket`, `UPDATE (note)` |
| `proof_other_service_login` | LOGIN NOINHERIT | Unrelated service control. Its `application_name` is spoofed as `proof-issuer`. |
| `proof_fencing_login` | LOGIN NOINHERIT, no CREATEROLE, no memberships | Independent handover controller. Acts only through `proof_fence.*` and `proof.set_handover` / `transition_epoch` / `try_open_gate`. |
| `proof_authority_owner` | NOLOGIN | Owns `proof` schema, tables, and SECURITY DEFINER functions |
| `proof_fence_owner` | NOLOGIN CREATEROLE | Owns `proof_fence.*`. Holds ADMIN (INHERIT FALSE, SET FALSE) on the issuer roles and `pg_signal_backend` (INHERIT TRUE, SET FALSE). |
| `proof_fencing_direct_login` | LOGIN CREATEROLE, ADMIN on `proof_issuer_b_login` | Counterexample: a controller with direct role-management rights |
| `proof_shared_login` | LOGIN SUPERUSER | Counterexample: a shared privileged login (Test E) |
| `proof_admin` | bootstrap superuser | Disposable cluster admin and independent observer. Not a product role. |

Every function in `proof` and `proof_fence` is SECURITY DEFINER with `SET search_path = pg_catalog`. Each one has PUBLIC EXECUTE revoked in the same transaction that creates it. The one exception is `proof.unprotected_issue(text)`, which deliberately keeps the default PUBLIC EXECUTE as a counterexample.

## Required tests and where they are proven

Each attempt's `assertions.json` and `RESULTS.md` hold the full list. A `COUNTEREXAMPLE_REPRODUCED` row with PASS means the unsafe behavior predicted by 0.1P was actually observed.

| Requirement | Assertions | Result (3/3 attempts) |
| --- | --- | --- |
| Separate issuer and writer logins | `T01-LOGIN-*`, `T01-CAT-RUNTIME-ATTRS`, `T01-CAT-OWNERS-NOLOGIN`, `T01-NOLOGIN-*` | PASS |
| No membership, inheritance, or SET path to owner/issuer roles | `T01-CAT-NO-MEMBERSHIP`, `T01-CAT-NO-AUTH-MEMBERS` | PASS |
| `SET ROLE` / `SET SESSION AUTHORIZATION` denied (42501), identity unchanged | `T02-*` | PASS |
| Writer cannot issue, call issuer functions, or update sensitive columns; issuer has no business-table rights | `T03-W-*`, `T03-I-*`, `T03-O-*`, `T01-CAT-NO-PROTECTED-DML`, `T01-CAT-ISSUER-NO-BUSINESS`, `T01-CAT-WRITER-COLUMNS`, `T01-CAT-EXECUTE-MATRIX` | PASS |
| No PUBLIC EXECUTE; SECURITY DEFINER has fixed `search_path` | `T01-CAT-PUBLIC-EXECUTE`, `T01-CAT-SECDEF-SEARCH-PATH`; counterexample `T03-W-UNPROTECTED`, `T03-O-UNPROTECTED` | PASS |
| Controller cannot take over issuer directly | `T03-F-ALTER-PASSWORD`, `T03-F-ALTER-NOLOGIN-DIRECT`, `T03-F-SELF-GRANT`, `T03-F-TERMINATE-RAW`, `T03-F-TERMINATE-RAW-VICTIM-ALIVE` | PASS |
| A: reconnect denied, verified by a real connection (28000) | `T04-B-DENY`, `T04-B-RECONNECT-DENIED`, `T08-LOG-28000` | PASS |
| Reconnect ban before census | `T04-ORDER-DENY-BEFORE-CENSUS` (deny done < reconnect-denied < census sent) | PASS |
| B: open transaction survives the ban, then targeted termination, independent pid-gone check, rollback | `T04-B-EXISTING-SURVIVES-BAN`, `T04-C-CENSUS-EXACT`, `T04-C-SIGNALS-SENT`, `T04-C-BACKENDS-GONE`, `T04-C-CLIENTS-SEE-TERMINATION`, `T04-B-ISS1-57P01`, `T04-B-ROLLBACK`, `T04-C-CLIENT-EXIT-CODES`, `T08-LOG-57P01` | PASS |
| C: other service survives; census does not use `application_name` | `T04-C-CENSUS-EXCLUDES-OTHERS`, `T04-C-WRONG-PID-REFUSED`, `T04-C-TARGET-NOT-ALLOWLISTED`, `T04-C-OTHER-SERVICES-UNAFFECTED`; counterexample `T04-C-APPNAME-SPOOF` | PASS |
| D: pool reconnect denied (3 TCP + 2 socket) | `T04-D-POOL-RECONNECT` | PASS |
| E: shared login bypass | `T07-E-SHARED-LOGIN-BYPASS`, `T07-E-SUPERUSER-NOT-TERMINABLE` | `REPRODUCED_IN_TEST_MODEL` |
| Issuer COMMIT before quiesce | `T04-N4-INFLIGHT-COMMIT` (commit succeeds after CLOSE + DENY) | counterexample reproduced |
| `REVOKE CONNECT` with an active backend / insufficient ban / superuser | `T05-N1-REVOKE-ACTIVE-BACKEND`, `T05-N1-REVOKE-BLOCKS-NEW`, `T05-N2-INSUFFICIENT-REVOKE`, `T05-N3-SUPERUSER-IGNORES-REVOKE` | counterexamples reproduced |
| Epoch row `FOR UPDATE` is not a commit fence | `T06-E1-LOCK-NOT-FENCE` (changer waits with `wait_event_type=Lock`; holder commits under epoch 1) | counterexample reproduced |
| Epoch change during an unfinished transaction / stale commit | `T06-E2-STALE-SNAPSHOT-COMMIT` (reproduced), `T06-E2B-RR-FOR-UPDATE` (40001), `T06-E2C-RC-STALE-REJECTED` (P0001) | PASS |
| New epoch refused while old backends remain | `T04-C-NO-EPOCH-WITH-OLD-BACKENDS` (`OLD_BACKENDS_PRESENT`) | PASS |
| After confirmed fencing, old identity rejected | `T04-D-TRANSITION`, `T04-E-OLD-IDENTITY-REJECTED` | PASS |
| Unknown transition outcome keeps gate closed | `T06-E4-UNKNOWN-OUTCOME` | PASS |
| Gate stays closed without witness | `T04-E-GATE-STAYS-CLOSED`, `T04-E-NEW-HOLDER-BLOCKED` | PASS |
| Witness scenarios | none | `NOT_RUN` |

## Findings for review

- **F1 — A direct CREATEROLE controller is not separated from the issuer.** `proof_fencing_direct_login` holds CREATEROLE plus ADMIN on `proof_issuer_b_login`. It reset the issuer's password and logged in as the issuer (`T07-F-DIRECT-CONTROLLER-TAKEOVER`). It also granted itself the issuer role WITH SET TRUE and then ran `SET ROLE` to it (`T07-F-DIRECT-CONTROLLER-SELF-GRANT`). Both happened in 3 of 3 attempts. The future `binding_authority_fencing` must therefore not hold CREATEROLE or ADMIN on issuer roles. The model's narrow SECURITY DEFINER functions avoided this; whether that shape is acceptable for the product is a review decision.
- **F2 — `pg_signal_backend` is not role-scoped.** It allows terminating any non-superuser backend. In the model, the only thing that keeps the controller off other services is the allowlist and owner check in `proof_fence.terminate` (`T04-C-WRONG-PID-REFUSED`). Granting `pg_signal_backend` directly to a controller login would remove that protection.
- **F3 — A shared superuser login defeats every control.** It forged a ticket directly, reopened the gate, assumed the issuer identity, and reversed NOLOGIN so the old issuer could reconnect. Its backend also cannot be terminated by a `pg_signal_backend` holder (42501). Result: `SHARED_LOGIN_BYPASS=REPRODUCED_IN_TEST_MODEL`.
- **F4 — The PUBLIC EXECUTE default is dangerous.** A SECURITY DEFINER function that is never revoked from PUBLIC let the writer and the other service insert tickets.
- **F5 — The signal-versus-gone race was not observed.** Every pid was already gone at the first independent check after `pg_terminate_backend` returned true (`T04-C-SIGNAL-VS-GONE`, 3 of 3 attempts). This is timing luck, not evidence of safety. The model still confirms termination independently by polling a separate admin session, by client FATAL 57P01, by psql exit code 2, and by the server log.
- **F6 — The backend check is valid only after the reconnect ban.** `transition_epoch`'s `OLD_BACKENDS_PRESENT` check is a point-in-time read. It means something only because reconnect was denied first (phase B), and it does not hold against a superuser or a shared login (F3).

## Not proven

`OLD_PRIMARY_FENCING=NOT_PROVEN`, `WITNESS_FAILURE_DOMAIN=NOT_PROVEN`, `TRUSTED_TENANT_AUTHORITY=NOT_PROVEN`, `ISSUER_OPERATION_AUTHORIZATION=NOT_PROVEN`, `PRESENTER_IDENTITY_BOUND=NOT_IMPLEMENTED`, `J_DML_001=NOT_CLOSED`, `ALL_WRITERS_FENCED=NO`, `PRODUCTION_ROLE_SEPARATION` not implemented. Role checks here are not tenant authorization. See `PRODUCT_DIFFERENCES.md`.

## Layout

| Path | Content |
| --- | --- |
| `sql/00_roles.sql`, `sql/01_schema.sql`, `sql/02_catalog.sql` | Role/GRANT definitions, test schema and functions, catalog queries |
| `run_proof.py` | Orchestrator (stdlib Python). It runs concurrent psql sessions, adds per-step markers, and handles redaction and cleanup. |
| `verify_manifest.py` | Generates and verifies `MANIFEST` and `SHA256SUMS` against committed Git blobs |
| `attempts/run_console.txt`, `attempts/summary.json` | Console output and aggregate results |
| `attempts/attempt_N/RESULTS.md`, `assertions.json`, `results.json` | Per-attempt PASS/FAIL/OBSERVED assertions |
| `attempts/attempt_N/steps.jsonl` | Every step: SQL, raw stdout/stderr, parsed SQLSTATE, send/done timestamps, connection loss, exit code |
| `attempts/attempt_N/sessions/*` | Per-session psql input, raw stdout, raw stderr, and exit code. The `__M<n>__` lines are step markers. |
| `attempts/attempt_N/timeline.json` | Ordered events: phases, deny, reconnect, termination, confirmation |
| `attempts/attempt_N/server_log.txt` | Redacted PostgreSQL log with `sqlstate=%e user=%u app=%a` |
| `attempts/attempt_N/catalog/*`, `setup/*`, `final_state.json`, `run_meta.json` | Privilege catalog, setup output, final table state, attempt metadata |

For one-shot connection attempts, psql does not print the SQLSTATE. The step records `sqlstate_from_message`, derived from the server message text. The server log shows the authoritative `sqlstate=` value; see `T08-LOG-*`.

## Reproduce

These commands need a local Docker engine with `postgres:16` already pulled and Python 3.10 or newer. They create and remove only `bintrans-proof-0-1q-*` containers.

```text
python docs/architecture/tracking/evidence/0_1q/run_proof.py --attempts 3 --out <empty-dir>
python docs/architecture/tracking/evidence/0_1q/verify_manifest.py --verify --rev HEAD
```
