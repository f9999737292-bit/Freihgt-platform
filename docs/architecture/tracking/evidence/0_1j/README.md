# Isolated full-function proof 0.1J

This directory is the evidence for `transport.read_binding_assignment` in a disposable PostgreSQL 16 database. It does not install the function in BINTRANS. The contract text is draft PR #379 at `965504203dd83c8c1bdba531ff645a787a0ebe8b`. That pull request is unchanged.

Primary run: `runs/attempt-5`. `suite_counts.txt` records `passes=60 fails=0`. Attempts 1–4 are kept with their harness notes.

## Observed database

- Image: `postgres:16`
- Server: PostgreSQL 16.15 (Debian 16.15-1.pgdg13+2), `server_version_num` 160015
- Container: `bintrans-full-fn-proof-0-1j`
- Container id: `df351c44211eec065604ea32fa9da5c07dbe843c96a07b2ffb7ddac35201dd2d`
- Network: `none`
- Published ports: none
- Database: `binding_full_function_proof_0_1j`
- Login: `proof_admin`, trust authentication, no password stored
- Database list: `binding_full_function_proof_0_1j,postgres,template0,template1`
- The container was removed after `runs/attempt-5` was copied out

## Reproduce

Use a new output directory so the saved attempts stay intact.

```bash
docker run -d --name bintrans-full-fn-proof-0-1j --network none \
  -e POSTGRES_HOST_AUTH_METHOD=trust \
  -e POSTGRES_USER=proof_admin \
  -e POSTGRES_DB=binding_full_function_proof_0_1j \
  postgres:16
docker cp docs/architecture/tracking/evidence/0_1j bintrans-full-fn-proof-0-1j:/tmp/evidence-0-1j
docker exec bintrans-full-fn-proof-0-1j \
  bash -lc 'OUT=runs/repro bash /tmp/evidence-0-1j/run_all.sh'
docker rm -f bintrans-full-fn-proof-0-1j
```

`run_all.sh` refuses to continue unless the current database is `binding_full_function_proof_0_1j` and `server_version_num` is in the PostgreSQL 16 range. Roles are cluster-wide. A second run in the same container fails in `setup.sql` because `CREATE ROLE` has no `IF NOT EXISTS`. Start a new container for each run.

## What attempt 5 executed

`function.sql` creates `transport.read_binding_assignment(uuid, uuid)` as `SECURITY DEFINER` with `search_path = pg_catalog`, owned by `assignment_lock_owner`. The body uses `RETURN QUERY`. The definition contains no `EXECUTE` and no `format(`.

The returned fixture row is:

```text
22222222-2222-2222-2222-222222222222|aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa|33333333-3333-3333-3333-333333333333|44444444-4444-4444-4444-444444444444|3|false|ACTIVE|11111111-1111-1111-1111-111111111111|cccccccc-cccc-cccc-cccc-cccccccccccc|33333333-3333-3333-3333-333333333333|44444444-4444-4444-4444-444444444444|55555555-5555-5555-5555-555555555555|7
```

Privilege results in attempt 5:

- Column `SELECT` and table `SELECT` without `UPDATE` both return `42501` for `FOR SHARE`.
- `UPDATE (updated_at)` is what lets the owner take the share lock. The same grant lets `assignment_lock_owner` update `updated_at`. `UPDATE` of `driver_id` stays `42501`.
- `tracking_runtime` cannot execute the function, `SET ROLE` to the owner, or update `updated_at`, `driver_id`, or `version` (`42501`).
- `PUBLIC` execute is false. `binding_authority_fence` execute is true.
- Membership of the fence and runtime roles in `assignment_lock_owner` is 0 after the inherit and set probes are revoked.
- `INHERIT TRUE` can update `updated_at` until that membership is revoked.

Assignment and concurrency results in attempt 5:

- Missing shipment, deleted shipment, missing execution, and missing driver return `P0002`.
- Two active execution links return `21000`.
- Tenant, driver, and vehicle mismatches return `23514`.
- An execution target with owner epoch text `ABSENT` returns `42501`. Epoch text other than `CURRENT` returns `40001`.
- A driver-only read of a shipment with no execution link takes no `RowShareLock` on `transport_executions`. A concurrent `FOR UPDATE` of that execution succeeds. The same transaction inserts the test binding and marks decision `6676` consumed.
- A driver-only read that finds an execution link returns `40001` and leaves decision `6679` issued.
- Command-then-consume on execution, then shipment, returns the original row. `C01_locks.stdout` shows granted `RowShareLock` on `transport_executions` and no shipment `RowShareLock` for `fence_probe` during the wait. `C02_activity.stdout` shows `fence_probe` waiting on `Lock` / `transactionid` while the writer is in `pg_sleep`. PostgreSQL stores both `FOR SHARE` and `FOR UPDATE` as relation mode `RowShareLock`.
- AssignDriver, AssignVehicle, and a successor revision are visible to the function after the writer commits. The original decision then mismatches (`23514` or `40001`).
- A link inserted during a driver-only hold is visible to the second read. That read still has no execution `RowShareLock`.
- An update against a held share returns `55P03`. After that transaction commits, another connection's update is not blocked.
- The function call and the decision update in `concurrency_same_transaction.sql` see four granted `RowShareLock` rows. Rollback restores decision `6666...6666` to `ISSUED` and leaves no binding row.
- Shipment-first versus the function's execution-first order deadlocks. The consume session receives `40P01` inside `transport.read_binding_assignment` while taking `FOR SHARE` on `transport.shipments`. Decision `6670` stays `ISSUED` and its binding count stays 0.
- `pg_terminate_backend` of the open consume session rolls the decision back to `ISSUED`. A postmaster crash was not induced.

The function's returned row and the harness consume scripts are not a protected binding write. Two commits in different sessions are not one atomic operation.

## Attempt history

- Attempt 1 stopped before `setup.sql`. `string_agg` ordered by a constant, so the database-list assertion failed. `freight_platform` was absent.
- Attempt 2 recorded 43 passes and 17 failures. The full `RETURN QUERY` row matched. The failures came from a SQLSTATE parser that required `ERROR:` at the start of the line, and from polls that looked for lock mode `ForUpdate` or `ForShare`.
- Attempt 3 recorded 56 passes and 4 failures. `SET deadlock_timeout` returned `42501` for the non-superuser roles. The driver and vehicle follow-up comparisons used decisions `6672` and `6673`, which already named the new identifiers, and raised `P0001`.
- Attempt 4 recorded 59 passes and 1 failure. The deadlock was real: the shipment-first session returned `40P01`. Its `deadlock_timeout` check runs once per wait, and the consume session's 200ms check had already run while the other session was still in `pg_sleep`, so the consume session committed decision `6670`.
- Attempt 5 is the primary run. The consume `deadlock_timeout` is 5 seconds and the shipment-first sleep is 2 seconds, so the cycle exists when that one check runs. The runner applies those values with `ALTER ROLE` because `SET deadlock_timeout` is superuser-only, then resets them.

## Test schema differences

- The tables are created by `setup.sql`. Platform migrations `000001`–`000105` are not applied.
- There are no enums, foreign keys, or row-level security policies. Status and `owner_epoch` are text.
- There is no `owner_epoch` column on the assignment tables. The epoch checks are in `assignment_compare.sql`, against the decision fixture. `ABSENT`, `STALE`, and `CURRENT` are fixture tokens.
- The pin table is inserted and is not read or locked. Revision and active-shipment rows are read and are not locked.
- The published 0.1H body does not raise on a missing row or on more than one row. This test body raises `P0002` and `21000`.
- `binding_authority_fence` can update the decision's `use_status`, `consumed_at`, and `binding_id`, and can insert the test binding table. That is the harness consume, not a product consume function.
- The deadlock consume updates the decision before calling the function so the aborted transaction has a write to roll back. That statement order is the harness order.
- Crash recovery is `pg_terminate_backend`.
- Authentication is trust with no password.

## Digest

`SHA256SUMS` lists the SHA-256 of each committed Git blob except itself. Paths are relative to this directory. `.gitattributes` sets `-text` for this directory, so Git does not normalize line endings. `git check-attr text` returns `unset` for a file inside this directory. The canonical digest is the committed blob, not a Windows worktree copy that a checkout may rewrite.
