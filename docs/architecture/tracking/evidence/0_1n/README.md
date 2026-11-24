# 0.1N foreign-key key-share evidence

Isolated PostgreSQL 16 proof for `TRACKING-BINDING-FK-KEYSHARE-COMPATIBILITY-PROOF-0.1N`. Product code, platform migrations, database roles, staging, and production were not changed.

## Environment

| Fact | Value |
| --- | --- |
| Container | `0cc913cf4c97bbe9734900211e42c763a04d26adf06365b308c1b3eeb4a7bcf2` |
| Name | `bintrans-01n-fk-keyshare` |
| Image | `postgres:16` `sha256:f1c3376c26f2609ab9f29f71f824103fe2fcd8ee0346485cb6122a4f93df6f94` |
| Server | `16.15 (Debian 16.15-1.pgdg13+2)`, `server_version_num=160015` |
| NetworkMode | `none` |
| PortBindings | `{}` |
| Database | `bintrans_01n_fk_keyshare` |
| Auth | `POSTGRES_HOST_AUTH_METHOD=trust` for user `proof_admin`. No password is set. |

The container was removed with `docker rm -f bintrans-01n-fk-keyshare` after the logs were copied out.

## Reproduce

From this directory, on a machine that already has that image:

```bash
docker run -d --name bintrans-01n-fk-keyshare --network none \
  -e POSTGRES_HOST_AUTH_METHOD=trust \
  -e POSTGRES_USER=proof_admin \
  -e POSTGRES_DB=bintrans_01n_fk_keyshare \
  postgres:16
docker cp . bintrans-01n-fk-keyshare:/tmp/evidence-0-1n
docker exec -w /tmp/evidence-0-1n bintrans-01n-fk-keyshare bash run_all.sh
```

`run_all.sh` refuses any other database name and any server outside PostgreSQL 16. The default output directory is `runs/attempt-1`. Use `OUT=runs/attempt-N` for another attempt. Copy `runs` back before `docker rm -f bintrans-01n-fk-keyshare`.

## Result

`runs/attempt-3` is the accepted run: 135 pass, 0 fail, exit code 0. `deadlock.txt` contains `NO_DEADLOCK_IN_TESTED_FK_INTERLEAVINGS`.

| Attempt | Pass | Fail | Why it is kept |
| --- | --- | --- | --- |
| 1 | 117 | 5 | The key-share and return checks counted `pg_locks` tuple rows. Granted row locks were not listed there. SQLSTATE, wait, and snapshot checks in that attempt passed. |
| 2 | 134 | 1 | `pgrowlocks` returned `NONE` for the new execution because that row was still uncommitted. |
| 3 | 135 | 0 | The new-execution check reads `HEAP_XMAX_KEYSHR_LOCK` through `pageinspect`. |

Do not treat attempt 1 or attempt 2 as a passing suite.

## Files

- `FK_INVENTORY.md` lists every foreign key found for this path, including those not installed.
- `setup.sql` and `fixture.sql` build the disposable schema.
- `writers/` holds one statement sequence per test.
- `run_all.sh` is the orchestration.
- `MODEL.md` maps each test to the Go transaction and the observed lock mode.
- `SHA256SUMS` lists the Git blob ids of the committed files, excluding itself.
