# Tracking DML 0.1E — protected write contract

Status: design. No function is created. No test of this contract was executed.
`PROTECTED_WRITE_SAFE_WITHOUT_TESTS=NO`.
`TENANT_ISOLATION_IMPLEMENTED=NO`.
`ALL_WRITERS_FENCED=NO`.

This contract extends the lock in PR #375 from a read to the state write. It uses the same caller rules as that PR at `9f31b6cfd96a06ece04eb4b946787f9813bfeeb4`: `session_user` is the login, `current_user` inside `SECURITY DEFINER` is the function owner, and `pg_has_role(session_user, 'tracking_runtime', 'USAGE')` means the privileges are available without `SET ROLE`. `SET ROLE` is not claimed safe without a test.

## 1. Roles

Three roles, none created here.

`tracking_service_login` is the only accepted login. Attributes: `LOGIN`, `NOSUPERUSER`, `NOBYPASSRLS`, `INHERIT`, `NOCREATEDB`, `NOCREATEROLE`. It is a member of `tracking_runtime` `WITH INHERIT TRUE, SET FALSE` and of no other role. It is not `freight`. It receives no direct table privilege and no direct `EXECUTE`.

`tracking_runtime` is `NOLOGIN`, `NOSUPERUSER`, `NOBYPASSRLS`, `NOINHERIT`. It receives:

- `USAGE` on schema `transport` and schema `tracking`.
- `EXECUTE` on the write functions in section 2 and the read functions in section 4, and on no other function.
- no `INSERT`, `UPDATE`, or `DELETE` on `transport.transport_executions`, `tracking.execution_tracking_state`, `tracking.execution_event_inbox`, or `tracking.execution_stop_eta_state`.
- no table `SELECT` on those four tables. The existing Go reads do not get a table grant. They call the functions in section 4.
- no `CREATE` on either schema and no `GRANT ALL`.

`tracking_state_fence` owns the write functions. Attributes: `NOLOGIN`, `NOSUPERUSER`, `NOBYPASSRLS`, `NOINHERIT`, `NOCREATEDB`, `NOCREATEROLE`. It is not the owner of `transport.transport_executions` or of the tracking tables. It is not `shipment_runtime`. No login is a member of it, and it has no `SET` grant. PostgreSQL 16 documents that a role whose privileges are inherited without `SET ROLE` should not own SQL objects. `tracking_runtime` owns none. `tracking_state_fence` owns the write functions and is not an inheritable membership.

The function owner holds the rights the body uses:

- `USAGE` on schema `transport` and schema `tracking`.
- `SELECT (id, operating_tenant_id, owner_epoch)` and `UPDATE (updated_at)` on `transport.transport_executions`. The `UPDATE` column is `updated_at` because `SELECT ... FOR UPDATE` requires `UPDATE` on at least one column. The body never assigns that column. This grant was not executed. It is not widened to `operating_tenant_id` or `owner_epoch` if a server rejects it.
- `SELECT`, `INSERT`, and `UPDATE` on `tracking.execution_tracking_state` and `tracking.execution_event_inbox`.
- for the ETA function only, `SELECT`, `INSERT`, and `UPDATE` on `tracking.execution_stop_eta_state`.
- no `DELETE`.
- no `UPDATE` of `transport.transport_executions.operating_tenant_id` or `owner_epoch`.

That last list is why the function owner can bypass the body. A session that becomes `tracking_state_fence` can `UPDATE` the state table without calling the function. `NOLOGIN` and the absence of membership are the designed barrier. They were not tested. `FUNCTION_OWNER_BYPASS=OPEN_UNTIL_TESTED`. The table owner and a superuser are separate bypasses.

## 2. Function shape

Each function is `SECURITY DEFINER`, `LANGUAGE plpgsql`, with `search_path = pg_catalog`. Every name in the body is schema-qualified. There is no dynamic SQL and no `EXECUTE` of a constructed string. There is no `COMMIT`. The lock lasts until the caller commits or rolls back.

`REVOKE ALL ON FUNCTION ... FROM PUBLIC` is required. PostgreSQL grants `EXECUTE` to `PUBLIC` on a new function unless that revoke is present. `GRANT EXECUTE` goes to `tracking_runtime` only.

The body accepts the call only when `pg_has_role(session_user, 'tracking_runtime', 'USAGE')` is true. It does not read `current_user` as the caller.

The functions are fixed statements, one per effect, so a new effect cannot be selected by a string:

- `tracking.apply_route_stop_current`
- `tracking.apply_route_stop_clear`
- `tracking.apply_execution_sequence_touch`
- `tracking.mark_approach_emitted_fenced`
- `tracking.apply_execution_stop_eta`

An unknown call is not given a default branch.

## 3. Body order

Every function uses the caller transaction.

1. `SELECT operating_tenant_id, owner_epoch FROM transport.transport_executions WHERE id = p_execution_id FOR UPDATE`. No row raises `EXECUTION_NOT_FOUND`. A null epoch raises `EPOCH_ABSENT`. Null is not read as `1`.
2. Compare the event's tenant claim and epoch claim with that locked row. The payload tenant, `X-Tenant-ID`, and `ListActiveTargetsForActors` are claims. A tenant mismatch raises `OWNER_STATE_MISMATCH`. An epoch that is not equal, including a higher event epoch, raises `STALE_EPOCH`.
3. `SELECT ... FROM tracking.execution_tracking_state WHERE execution_id = p_execution_id FOR UPDATE`. This is the second lock. The execution row is already held.
4. Read inbox rows for this `event_id` and for this execution and sequence. A different `event_id` with the same execution and sequence raises `INBOX_SEQUENCE_CONFLICT`. The function does not overwrite either row.
5. When `event_id` is new, insert the inbox row: `event_id`, tenant, execution, sequence, event type, and the future hash, size, and epoch columns when they exist. The insert is not a terminal confirmation.
6. Apply the one fixed state statement for that function. `last_event_id`, when the column exists, is written in that same statement, after the inbox insert, so an immediate foreign key sees the inbox row in this transaction. The referenced inbox row must have the same tenant, execution, and event identity. A UUID alone is not proof.
7. Require `RowsAffected > 0` for a new effect. Zero rows raise `STATE_NOT_APPLIED`. The function does not return success for zero rows. A nil driver error is not a state effect.
8. Re-read the state row. Tenant, execution, sequence, epoch, and `last_event_id` must match the locked execution and the inbox row.

The caller then commits. Only after that commit returns may the consumer call `CommitRecords` (`execution_event_consumer.go:60-61`). Any raised exception aborts the PostgreSQL transaction. The new inbox row does not remain. The offset is not committed.

When `event_id` already exists, the function does not insert again and does not overwrite the inbox row. It treats the call as replay. Hash equality alone is not terminal. A prior effect is verified only when the existing inbox row and `last_event_id` agree on event id, tenant, execution, sequence, hash, and size, and the locked epoch equals the stored epoch. A verified replay writes nothing and returns. A partial match raises. The current inbox table (`000091:35-43`) has no hash, size, or epoch column, so a historical row cannot satisfy that proof. Replay of those rows stays `NON_TERMINAL`.

`mark_approach_emitted_fenced` uses the same lock and checks, then the approach-column update now at `execution_tracking_repository.go:312-318`. It does not use the pre-lock list at `approach.go:69` as the epoch. Zero rows raise, which current `approach.go:95-96` does not do.

`apply_execution_stop_eta` locks the execution row before the state row, requires the stored state epoch to equal the locked epoch, and then runs the ETA upsert now at `execution_tracking_repository.go:253-275`. It does not change `owner_epoch`.

## 4. Reads

`RECOMMENDED_READ_MODEL=HYBRID_PROTECTED_READ`.
`MINIMAL_TABLE_SELECT=NOT_SELECTED`.
`TENANT_ISOLATION_IMPLEMENTED=NO`.
`TENANT_SCOPED_READ_APPROVED=NO`.

Two read shapes were compared.

Table `SELECT`, including a column list, lets `tracking_runtime` run any `WHERE` it can send. `GetState` filters only by `execution_id` (`execution_tracking_repository.go:178-179`). `ListActiveTargetsForActors` filters by actor ids and has no tenant predicate (`execution_tracking_repository.go:211-215`). `GetExecutionStopETA` filters by the supplied tenant (`execution_tracking_repository.go:291`), and that value is `X-Tenant-ID` (`tracking_handler.go:249-254`, `headers.go:11`). A predicate in Go is not a privilege. No row level security policy exists for these tables through `000105`. Table `SELECT` is not granted. Row level security is not selected; a single runtime role that can set the tenant value would recreate a caller-supplied tenant.

Protected read functions are `SECURITY DEFINER`, `search_path = pg_catalog`, with schema-qualified names, no dynamic SQL, and no `COMMIT`. `REVOKE ALL ... FROM PUBLIC` applies. `EXECUTE` goes to `tracking_runtime` only. The owner is `tracking_read_fence`: `NOLOGIN`, `NOSUPERUSER`, `NOBYPASSRLS`, `NOINHERIT`, and not a member of any login. It has `SELECT` on the columns those statements read and no `INSERT`, `UPDATE`, or `DELETE`. It is not `tracking_state_fence`. The write owner is therefore not required for these reads, and the read owner cannot change a row. A session that becomes `tracking_read_fence` can still read every tenant. That bypass is open until no session can become the role. `FUNCTION_OWNER_BYPASS=OPEN_UNTIL_TESTED`.

The functions are:

- `tracking.read_execution_state(p_execution_id uuid, p_claimed_tenant uuid)` returns the row only when `operating_tenant_id` equals `p_claimed_tenant`. A different claim returns no row. The claim is not proof that the caller is that tenant.
- `tracking.list_active_targets(p_driver_ids uuid[], p_vehicle_ids uuid[], p_claimed_tenant uuid)` uses the actor predicate at lines 211-215 and adds `operating_tenant_id = p_claimed_tenant`. It does not preserve the current cross-tenant result. `approach.go:69` and `eta_ingest_service.go:192` pass no tenant. `LIST_ACTIVE_TARGETS_TENANT_SOURCE=MISSING`. The function is not a license to ship the current statement.
- `tracking.read_execution_stop_eta(p_claimed_tenant uuid, p_execution_stop_id uuid)` keeps both predicates now at line 291 inside the function. The HTTP header remains a claim.

A function that accepts the tenant from its caller does not isolate tenants. `tracking_runtime` is one role for every tenant. Naming tenant A still returns tenant A's row. `TENANT_ISOLATION_IMPLEMENTED=NO`. The pre-implementation gate is `TENANT_SCOPED_READ_APPROVED=NO`.

These reads do not take the execution lock and do not change state. They are not a substitute for the write order in section 3. `ListActiveTargetsForActors` before that lock is still not the authoritative epoch.

## 5. What this contract does not prove

The functions were not created and were not executed. Column-level `UPDATE` sufficiency for `SELECT ... FOR UPDATE` was not executed. `PUBLIC` execute on existing functions was not read from `pg_proc.proacl`. Tenant isolation is a specified comparison, not a running check. Direct DML by the table owner remains possible. `PROTECTED_WRITE_SAFE_WITHOUT_TESTS=NO`.
