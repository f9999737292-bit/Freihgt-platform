# Isolated assignment-integrity proof 0.1K

This directory proves four gaps in the 0.1J read function against a disposable PostgreSQL 16 database. It does not install the function in BINTRANS. Draft PR #380 at `0fb0c61348be121eb3e2d1941f83f865acd0faf9` is unchanged. The 0.1I and 0.1J evidence trees are not in this branch.

Primary run: `runs/attempt-2`. `suite_counts.txt` records `passes=66 fails=0`. Attempt 1 is kept. Its four failures were command tags (`SET`, `UPDATE 1`, `DO`) left in the single-line reader. The PostgreSQL rows and SQLSTATEs in that attempt already matched.

## Observed database

- Image: `postgres:16`
- Server: PostgreSQL 16.15 (Debian 16.15-1.pgdg13+2), `server_version_num` 160015
- Container: `bintrans-integrity-proof-0-1k`
- Container id: `b87e4f9c4b1e208750d7f3b85ef5e24a530f29dca825a2ff2e05967a8fcd7ec6`
- Network: `none`
- Published ports: none
- Database: `binding_assignment_integrity_0_1k`
- Login: `proof_admin`, trust authentication, no password stored
- Database list: `binding_assignment_integrity_0_1k,postgres,template0,template1`
- The container was removed after `runs/attempt-2` was copied out

`proof_admin` creates the schema and restores fixtures. The execution-identity and cross-tenant calls run as `fence_probe`. A superuser query is not the tenant-isolation result. `TENANT_ISOLATION_IMPLEMENTED` stays `NO`.

## Reproduce

Use a new output directory so the saved attempts stay intact. Roles are cluster-wide, so each run needs a new container.

```bash
docker run -d --name bintrans-integrity-proof-0-1k --network none \
  -e POSTGRES_HOST_AUTH_METHOD=trust \
  -e POSTGRES_USER=proof_admin \
  -e POSTGRES_DB=binding_assignment_integrity_0_1k \
  postgres:16
docker cp docs/architecture/tracking/evidence/0_1k bintrans-integrity-proof-0-1k:/tmp/evidence-0-1k
docker exec bintrans-integrity-proof-0-1k \
  bash -lc 'OUT=runs/repro bash /tmp/evidence-0-1k/run_all.sh'
docker rm -f bintrans-integrity-proof-0-1k
```

`run_all.sh` continues only when the current database is `binding_assignment_integrity_0_1k` and `server_version_num` is in the PostgreSQL 16 range.

## What attempt 2 showed

`transport.read_binding_assignment(uuid, uuid)` is `SECURITY DEFINER`, owned by `assignment_lock_owner`, with `search_path=pg_catalog`. `EXECUTE` is false for `PUBLIC` and `tracking_runtime`, and true for `binding_authority_fence`. The body has `RETURN QUERY`, no dynamic SQL, and no reference to `transport_execution_participants`.

The matching call returns:

```text
11111111-1111-1111-1111-111111111111|4|55555555-5555-5555-5555-555555555555|7|cccccccc-cccc-cccc-cccc-cccccccccccc
```

That is the input execution, bigint epoch 4, and the ACTIVE revision. The historical participant execution `99999999-9999-9999-9999-999999999999` is not in the row.

`transport.read_binding_assignment_0_1j` is the previous body. Called as `fence_probe` with the historical execution and the main shipment, it returns `11111111-1111-1111-1111-111111111111`. The input execution is not the execution in the row. The 0.1K body raises `23514` (`input execution is not the active link`) for that call, for another shipment's execution, and for an execution whose operating tenant is `dddddddd-dddd-dddd-dddd-dddddddddddd`. Those stdout files contain neither the substituted id nor the linked execution. A missing execution is `P0002`.

The active-slot primary key rejects a second link with `23505`. The one-ACTIVE revision index rejects a second `ACTIVE` revision with `23505`. A current pointer at a `SUPERSEDED` revision is `23514`. A null `current_revision_id` on an execution target is `P0002`. The 000088-style trigger still allows both of those pointer states; the read function is what rejects them.

`owner_epoch` is `bigint`. A null epoch on an execution target is `42501`. Epoch 4 matches the decision whose epoch is 4. A decision stored as 3 raises `40001` inside the same transaction as a consume update, and that decision stays `ISSUED` with epoch 4 unchanged. `owner_epoch - 1` is `23514`, and the stored value stays 4. Changing `operating_tenant_id` without a greater epoch is `23514`, and the row stays on `cccccccc-cccc-cccc-cccc-cccccccccccc` at epoch 4. The same change with `owner_epoch + 1` commits; the next read returns epoch 5 and operating tenant `dddddddd-dddd-dddd-dddd-dddddddddddd`.

Column `SELECT` without `UPDATE` cannot `FOR SHARE` (`42501`). The lock owner can update `updated_at` and cannot update `driver_id` or `owner_epoch`. Runtime cannot execute the function, `SET ROLE` to the owner, or update those columns. After the inherit and set memberships are revoked, the fence and runtime membership count is 0.

Concurrency, each in its own session:

- A revision writer holds the execution, and the read waits on `Lock` / `transactionid`. After commit the read returns revision `12121212-1212-1212-1212-121212121212` version 8.
- A link writer that locks the execution first makes the old execution id raise `23514` after commit. The reader stdout does not contain the replacement execution.
- A link writer that updates only `transport_execution_active_shipments` commits while the read session is still in `pg_sleep`. The execution `FOR SHARE` does not fence that writer. The later read of the old execution id is `23514`. That pair is not one atomic assignment.
- An epoch writer holds the execution. After commit the read returns epoch 5, and the decision stored as 3 raises `40001`.
- A command locks the execution and only later the shipment. The read waits, has no shipment `RowShareLock` during that wait, returns the original row, and does not see `40P01`. `FOR UPDATE` and `FOR SHARE` are both stored as `RowShareLock`.
- A shipment update against the held share returns `55P03`. The version stays 3.
- A driver-only call sees the active execution and takes no execution `RowShareLock`. A concurrent `FOR UPDATE` of that execution succeeds.

`FUTURE_MIGRATION.md` lists the column, trigger, function checks, and grant a later BINTRANS migration would need. This run does not apply it.

## Test schema differences

- Tables come from `setup.sql`. Platform migrations `000001`–`000105` are not applied.
- There are no foreign keys, enums, row-level security policies, cargos, stops, or pins.
- `owner_epoch bigint` and `transport.reject_owner_epoch_regression` exist only in this database.
- The revision trigger checks execution and operating tenant. It does not require status `ACTIVE`.
- `transport_execution_participants` stores the historical slot. The proof function does not read it.
- `reset_fixture.sql` uses `session_replication_role = replica` so the admin restore can put the epoch back. `command_probe` is not given that setting.
- The stale-epoch script updates the decision and then raises. The fence's consume grants exist so that rollback can be observed. They are not a product consume function.
- The extra return column is `owner_epoch bigint`.
- Authentication is trust, with no password. A postmaster crash was not induced.

## Digest

`SHA256SUMS` lists the SHA-256 of each committed Git blob except itself. Paths are relative to this directory. `.gitattributes` sets `-text` here, so Git does not normalize line endings. `git check-attr text` returns `unset` for a file inside this directory. The canonical digest is the committed blob.
