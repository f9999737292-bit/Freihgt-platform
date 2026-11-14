# TRACKING-BINDING-OPTION-A-COMPATIBILITY-PROOF-0.1M

Isolated check of whether the reader order `EXECUTION → SHIPMENT → ACTIVE LINK → DRIVER → VEHICLE` deadlocks or loses its end state against the writer paths proved in 0.1L.

The order is a probe. It is not a production lock contract. `J-DML-001` stays open. This pack does not change product code, platform migrations, or database roles outside the disposable database.

## Isolation

| Fact | Observed value |
| --- | --- |
| Container | `bintrans-01m-writer-compat` |
| Container id | `92eca552645e7a801861cd2a9b64216af727851cd7f667325b8f19b2e66e6a6c` |
| Image | `postgres:16` `sha256:f1c3376c26f2609ab9f29f71f824103fe2fcd8ee0346485cb6122a4f93df6f94` |
| Server | `16.15 (Debian 16.15-1.pgdg13+2)` |
| `server_version_num` | `160015` |
| Database | `bintrans_01m_writer_compat` |
| Network | `none` |
| Published ports | none (`PortBindings` `{}`) |
| Authentication | `POSTGRES_HOST_AUTH_METHOD=trust` inside the container. No password is stored. |
| Roles | `proof_admin` from the image, plus `reader_probe` and `writer_probe` created by `setup.sql` in this database only |
| Cleanup | `docker rm -f bintrans-01m-writer-compat` after the runs were copied out. No other container was removed. |

The suite refuses any other database name and any server outside PostgreSQL 16. Platform migrations were not applied. No staging or production connection was used.

## How a result is judged

`run_all.sh` starts one `psql` per session. For every mandatory scenario it runs reader-first and writer-first. A passing process is not enough: each case records SQLSTATE, the committed snapshot, and, where a wait is required, `pg_stat_activity.wait_event_type = 'Lock'`.

Raw session output is under `runs/`. `assertions.tsv` is the verdict table. `commands.log` lists every `psql` invocation. The repository ignore of `*.log` does not apply to that file: `.gitignore` in this directory puts it back. `deadlock.txt` is the `40P01` scan of `*.stderr`.

## Attempts

| Attempt | passes | fails | Cause |
| --- | --- | --- | --- |
| `runs/attempt-1` | 64 | 67 | `reader_probe` lacked `UPDATE`, so `FOR SHARE` failed with `42501`. Writers then ran without a held reader. |
| `runs/attempt-2` | 125 | 6 | The privilege grant fixed the reader. Six reader-first writers committed the required rows, but `finish_bg` was called inside a command substitution and recorded `rc=-1`. |
| `runs/attempt-3` | 131 | 0 | `finish_bg` waits in the parent shell. This is the assertion run. |

Attempt 1 and attempt 2 are kept. The results below are attempt 3.

## Attempt 3

`NO_DEADLOCK_IN_TESTED_INTERLEAVINGS`

No `40P01` appears in attempt 3 stderr. That statement covers only the interleavings this harness ran.

`55P03` occurred once: `DL_T`, the direct link update with `lock_timeout = '2s'` while the reader held the link. The snapshot stayed on the original binding. After the reader committed, the same update committed.

Occupied `Project` sessions rolled back. Execution ids `1515…1515` and `1313…1313` were absent after `23505`. The following free-slot retry committed a different execution id. `DL_T` rolled back before the retry.

| Scenario | Reader first | Writer first | End state |
| --- | --- | --- | --- |
| Project, flag off, occupied slot | Reader held the link. Writer returned `23505` with no lock wait. Reader was still in `pg_sleep`. Snapshot unchanged. | `23505` in under 5 seconds. Snapshot unchanged. | New execution absent. Retry on the free shipment committed execution `1212…1212`. |
| Project, flag off, free slot | Covered by the retry above. | Reader of the old binding finished while the writer slept. Old line unchanged. | Free link `2223…2223` = `1212…1212`, execution count 3. |
| Project, flag on, occupied slot | Writer waited. Snapshot unchanged until the reader finished, then `23505`. | During the pre-insert sleep, locks on `transport_executions` = 0. Reader waited. Then `23505`. Reader still returned the original line. | Execution `1313…1313` absent. |
| Project, flag on, free slot | Writer waited on `drivers` and held no lock on `transport_executions` while waiting. | Same absence of an execution lock during the sleep. Reader waited. Both finished. | Free link = `1414…1414`, execution count 3. Old link unchanged. |
| AssignDriver | Writer waited. No commit while the reader slept. | Reader waited, then saw version 2 and the replacement driver. | Shipment version 2, status `ASSIGNED`, driver `…7778`. |
| AssignVehicle | Same shipment wait. | Reader saw the replacement vehicle. | Version 2, status `ASSIGNED`, vehicle `…3334`. |
| Driver-stop | Writer waited on the shared execution/shipment prefix. | Reader waited, then saw `LOADED` and `event_seq` 2. | That shipment and event sequence. Link relation mode during the plain `SELECT` was `AccessShareLock`; tuple locks on the link table were 0. |
| Successor | Writer waited. The old revision was not superseded while the reader held the execution. | Reader waited, then saw revision `…5556`. | Current revision `…5556`, revision `…5555` `SUPERSEDED`. Shipment version stayed 1. |
| Shipment status | Writer waited. Status stayed `PLANNED` while the reader slept. | Reader waited, then saw `IN_TRANSIT`. | Version 2, status `IN_TRANSIT`. |
| Direct link `UPDATE` | Writer waited on the link (`Lock` / `transactionid` / `active_shipments`) while the reader was in `pg_sleep`. Snapshot stayed original until the reader committed. | Reader waited, then `23514` for the old execution id. | Link moved to execution `aaaa…aaaa` with no execution lock. A read of `aaaa…aaaa` succeeds. `DL_T` timed out with `55P03`, left the original link, and the retry after release committed the move. |

Direct `UPDATE execution_id` still commits once the reader releases the link, and it never locks the execution. The bypass remains open without a shared protocol. This run does not close `J-DML-001`.

Flag-on `Project` waits on the shipment or the shared driver and then inserts a new execution id. It does not lock execution `1111…1111`. No tested pair produced a wait cycle.

AssignDriver, AssignVehicle, and shipment status wait only because this reader also takes the shipment `FOR SHARE`. Their product SQL does not lock the execution or the link.

## Checksums

`SHA256SUMS` is the SHA-256 of each committed file in this directory except `SHA256SUMS` itself. Paths are relative and use forward slashes. The hash is of the Git blob bytes.
