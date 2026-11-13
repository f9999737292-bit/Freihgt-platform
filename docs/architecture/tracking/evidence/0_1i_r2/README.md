# 0.1I-R2 isolated PostgreSQL evidence

This directory is the reproducible evidence pack for J-PGP-001. It is not a platform migration and it is not on a production deployment path.

## What this proves

On a local PostgreSQL 16 server, in a database created for this run:

- `FOR SHARE` with column `SELECT` and no `UPDATE` returns `42501`.
- `FOR SHARE` with table `SELECT` and no `UPDATE` returns `42501`.
- `UPDATE (updated_at)` lets `assignment_lock_owner` take `FOR SHARE` and change `updated_at`.
- The same role receives `42501` for `UPDATE` of `driver_id`.
- A second session's `UPDATE` waits while `FOR SHARE` is held, then completes after commit.
- `transport.read_binding_assignment(uuid)` is `SECURITY DEFINER`, owned by `assignment_lock_owner`, with `search_path = pg_catalog`. Fence execution reports that owner, the fence session user, `pg_catalog`, and the fixed driver id.
- Runtime cannot `SET ROLE` to the owner, cannot `UPDATE` the shipment, and cannot `EXECUTE` the function.
- `INHERIT TRUE` and `SET TRUE` expose `updated_at` while the membership exists. After `REVOKE`, they do not.
- A rolled-back consume leaves `use_status = ISSUED` and `consumed_at IS NULL`.

## What this does not prove

The function under test locks one shipment row and returns four values. It is not the full design `RETURN QUERY` over executions, drivers, vehicles, revisions, and pins. `FULL_FUNCTION_TEST=NOT_RUN`.

This run does not prove tenant isolation, freight credentials, the production schema, every writer, Kafka, or end-to-end binding authority. No BINTRANS migration is applied. The database is not `freight_platform`.

## Isolation

Use a local Docker engine. Do not publish a port. Do not attach a network. Do not point the command at staging or production.

```text
docker run -d --name bintrans-priv-proof-0-1i-r2 --network none \
  -e POSTGRES_HOST_AUTH_METHOD=trust \
  -e POSTGRES_USER=proof_admin \
  -e POSTGRES_DB=binding_privilege_proof_0_1i_r2 \
  postgres:16
```

Trust authentication is the container's local socket. No password is set and none is stored in this directory. Copy this directory into the container, strip CR from the shell script, and run:

```text
docker exec bintrans-priv-proof-0-1i-r2 bash /tmp/evidence-0-1i-r2/run_all.sh
```

The runner's exit status is non-zero when any assertion fails. An expected `42501` is checked inside that test's own `psql` process. It does not stop later tests.

After copying `runs/` out, remove only a container whose name is `bintrans-priv-proof-0-1i-r2`.

## Artifact origin

| Path | Origin |
| --- | --- |
| `prior_scripts/0_1i/` and `prior_scripts/0_1i_r1/` | Script files found on disk before this run. Copied unchanged. |
| `prior_scripts/README.md` | Written for 0.1I-R2. `ORIGINAL_TRANSCRIPT=NOT_AVAILABLE`. |
| `setup.sql`, `privilege_tests.sql`, `security_definer_tests.sql`, `rollback_tests.sql`, `concurrency_session_a.sql`, `concurrency_session_b.sql`, `run_all.sh` | New 0.1I-R2 harness. |
| `runs/attempt-1/` | First 0.1I-R2 execution. Kept. Six assertions failed because of harness defects, not because those PostgreSQL facts were the opposite. |
| `runs/attempt-2/` | Second execution in the same container after `DROP DATABASE`. Kept. `setup.sql` stopped because roles are cluster-wide and still existed. Later `42501` lines in that attempt are `permission denied for schema transport`, because `GRANT USAGE` never ran. |
| `runs/attempt-3/` | Fresh container and a new database. Primary `NEW_RUN_EVIDENCE`. |

Attempt 1 harness defects, recorded in that attempt's stderr:

- `E04_update_updated_at` read `updated_at` in the lock-owner statement. That role has no `SELECT` on `updated_at`, so PostgreSQL returned `42501`. The corrected test updates the column and lets `proof_admin` read it afterwards.
- `E06_create_function` returned `42501` `permission denied for schema transport`. The lock owner had no `CREATE` on the schema. The corrected statement grants `CREATE` for that statement and revokes it afterwards.
- `E05_waiter_blocked` looked for an ungranted relation lock. `RowExclusiveLock` does not conflict with `RowShareLock` at table level, so the query returned `NOT_WAITING` while the session was still running. The corrected poll uses `wait_event_type = 'Lock'`.

`E09` codes `42883` in attempt 1 follow from the function not existing. They are not a separate privilege result.

`assertions.tsv` is a summary. It does not replace the stdout and stderr files.

## Harness, separate from PostgreSQL

`psql -tAc` on a script that contains `BEGIN`, `UPDATE`, `ROLLBACK`, and `SELECT` prints the command tags and the data row. The 0.1I recorder compared that whole stdout with `ISSUED|null` and stored `FAIL` even though the data row was `ISSUED|null`.

`HARNESS_multistatement_tags.stdout` keeps that combined output. The row state is judged by `E07_use_status.stdout`, `E07_consumed_at_is_null.stdout`, and `E07_state_predicate.stdout`. The predicate returns `PASS` or `FAIL`. It does not compare the server's boolean spelling.

## Fixed identifiers

- Database: `binding_privilege_proof_0_1i_r2`
- Shipment: `22222222-2222-2222-2222-222222222222`
- Driver: `33333333-3333-3333-3333-333333333333`
- Decision: `66666666-6666-6666-6666-666666666666`

`SHA256SUMS` lists every other file in this directory.
