# E05 concurrency

Session A and session B are separate `psql` processes. They do not share a transaction.

Session A (`concurrency_session_a.sql`, login `lock_probe`):

1. `SET ROLE assignment_lock_owner`.
2. `BEGIN`.
3. `SELECT ... FOR SHARE` on shipment `22222222-2222-2222-2222-222222222222`.
4. `pg_sleep(8)`.
5. `COMMIT`.

Session B (`concurrency_session_b.sql`, login `writer_probe`) starts only after `pg_locks` shows a granted `RowShareLock` on `shipments` for `lock_probe`. Session B runs:

```sql
UPDATE transport.shipments
   SET driver_id = driver_id
 WHERE id = '22222222-2222-2222-2222-222222222222';
```

`writer_probe` has `UPDATE (driver_id)`. It has no membership in `assignment_lock_owner`.

While session A is still inside `pg_sleep`, the runner reads `pg_stat_activity.wait_event_type` for `writer_probe`. The expected marker is `WAITING`, which is a `Lock` wait. A table-level `RowExclusiveLock` can be granted at the same time, because it does not conflict with `RowShareLock`. After session A commits, session B's statement is expected to finish as `UPDATE 1`.

`run_all.sh` is the isolated runner for this sequence. The captured primary output is under `runs/attempt-3/`: `E05_session_a.stdout`, `E05_session_a.stderr`, `E05_session_b.stdout`, `E05_session_b.stderr`, `E05_waiter_poll.stdout`, and `E05_waiter_detail.stdout`. `runs/attempt-1/` keeps the first poll, which did not read `wait_event_type`.
