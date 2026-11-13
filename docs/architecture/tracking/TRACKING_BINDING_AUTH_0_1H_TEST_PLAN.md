# TRACKING-BINDING-AUTHORITY-DECISION-0.1H — Test Plan

These tests are specified. None are executed. Every row is `NOT_RUN`. The oracle is this contract. The current insert and the current target query fail any oracle that requires a decision. That failure is expected and is not a pass.

`CROSS_TENANT_TEST_PLAN=SPECIFIED_NOT_RUN`

Errors contain no coordinates, driver id, vehicle id, or tenant id.

## 1. Valid shipment-owned decision

- Setup: gateway user U resolves to active driver D. Shipment S in tenant A has `driver_id` D and a version. An active link points at execution E on revision R. E's driver is D. No vehicle is claimed.
- Input: the checker is asked for U, S, and no vehicle.
- Oracle: PASS when one decision row records A, S, D, `DRIVER_ONLY`, E, E's operating tenant, and R, and `owner_epoch=ABSENT` denies execution-target rights while the location right is present. FAIL when the decision copies a header tenant that was not on the shipment row, or when `ABSENT` epoch still grants a target read.

## 2. Forged tenant

- Setup: the gateway tenant is A. The caller also sends tenant B.
- Input: a check and a binding write for S.
- Oracle: PASS when the decision tenant is the shipment row's tenant and a header B creates no decision and no binding. FAIL when `tracking_handler.go:101-104` stores B as authority.

## 3. Forged vehicle

- Setup: D is assigned to shipment A. Vehicle V is current on shipment B.
- Input: a check for A that claims V.
- Oracle: PASS when the decision is denied and no binding records V for A. FAIL when the driver match authorizes V, or when `X-Vehicle-ID` is stored as a grant. `EXISTING_VEHICLE_CHECK=NOT_IMPLEMENTED_IN_THIS_PATH`.

## 4. Driver-only authorization

- Setup: S has driver D and a null `vehicle_id`. Execution E has driver D and some other vehicle.
- Input: a decision with no claimed vehicle, then a target read.
- Oracle: PASS when the location right exists and the target is kept only by `driver_id`, with E's vehicle ignored as a key. FAIL when the null vehicle matches every vehicle, or when the missing vehicle denies the driver assignment.

## 5. Vehicle reassignment

- Setup: a decision claimed V1. `AssignVehicle` then sets the shipment to V2 (`shipment_repository.go:472-475`).
- Input: reuse of that decision for a write, and a target read.
- Oracle: PASS when the write inserts nothing and the target set omits V1. FAIL when the stored binding still authorizes V1. `VEHICLE_REASSIGNMENT_POLICY=CLAIMED_VEHICLE_MUST_MATCH_CURRENT_ROW`.

## 6. Execution revision change

- Setup: the decision records revision R1. A successor sets `current_revision_id` to R2 (`transport_execution_successor_repository.go:172-176`). Driver and vehicle columns stay as they were.
- Input: a target read and a binding write that still cites R1.
- Oracle: PASS when execution scope is denied until a new decision names R2. FAIL when unchanged driver and vehicle columns keep R1 active. `EXECUTION_REVISION_POLICY=STALE_REVISION_IS_NOT_ACTIVE`.

## 7. Direct binding insert bypass

- Setup: a shared-token caller sends a tenant, driver, and vehicle with no `decision_id`.
- Input: the internal location route and a direct `INSERT`.
- Oracle: PASS when the future function and the `REVOKE` write nothing, and the report still marks the current path `DIRECT_BINDING_INSERT_BYPASS=OPEN`. FAIL when the current insert is described as already closed, or when the token is accepted as a decision.

## 8. Duplicate binding race

- Setup: two writes for the same tenant and device. One decision is for shipment S1. The other is for S2. Only S1's assignment is current.
- Input: both calls, overlapping.
- Oracle: PASS when S2 observes conflict and the stored shipment is S1. FAIL when `ON CONFLICT DO NOTHING` (`tracking_repository.go:476`) lets S2 observe success. `DUPLICATE_BINDING_POLICY=CONFLICT_UNLESS_SAME_DECISION_SHIPMENT`.

## 9. Revoked assignment

- Setup: a decision was valid. The shipment `driver_id` then changes, or the driver row is no longer active.
- Input: the unused decision is presented for a write, and a target read runs.
- Oracle: PASS when both the write and the target omit the scope. FAIL when `status = 'active'` on an older binding is enough. `REVOCATION_POLICY=RECHECK_AND_OMIT`.

## 10. Shared driver across tenants

- Setup: tenant A and tenant B each have a current driver assignment and an active device binding. The driver UUID may be the same or different.
- Input: a target read for that device.
- Oracle: PASS when the result contains two labeled scopes, or only the scopes whose current rows match, and `LIMIT 1` is not applied. FAIL when `tracking_repository.go:51-53` keeps one tenant.

## 11. Shared vehicle across tenants

- Setup: vehicle V is current on an execution in A and is also stored on an execution in B. Only A's decision claimed V. B is driver-only or claims a different vehicle.
- Input: a target read.
- Oracle: PASS when B is not returned because of V. FAIL when `actorMatchesTarget` (`eta_ingest_service.go:311-327`) grants B from the vehicle UUID.

## 12. Untrusted service identity

- Setup: a caller presents the shared tracking token, or integration-token headers without that token.
- Input: a request for a binding decision and for `GetExecutionStop`.
- Oracle: PASS when no decision is issued and no ETA body is returned. FAIL when the shared token or a client service-name header is a named identity. `NAMED_SERVICE_IDENTITY=UNRESOLVED_NO_EXISTING_MECHANISM`.

## 13. Expired authority decision

- Setup: a decision whose `expires_at` has passed. The shipment assignment is unchanged.
- Input: a binding write using that `decision_id`.
- Oracle: PASS when nothing is inserted and a new check is required. FAIL when the unexpired assignment bypasses `expires_at`, or when the same `decision_id` can be consumed twice.

## 14. Concurrent reassignment

- Setup: session 1 has evaluated scope A. Session 2 commits a new shipment driver or a new revision before session 1 writes or finishes the read.
- Input: session 1's write and target result.
- Oracle: PASS when the version mismatch drops the scope and no effect is committed from the stale view. FAIL when the earlier decision or the pre-lock target list remains the authority. The execution command lock is not this read.

## 15. Authorization source unavailable

- Setup: shipment or execution rows cannot be read.
- Input: a check, a binding write, and a target read.
- Oracle: PASS when no decision is stored, no binding is inserted, and the target set is empty, with no call to the unscoped actor query. FAIL when headers or `status = 'active'` are the fallback. `USE_TIME_RECHECK=CURRENT_ROWS_OR_EMPTY`.

## 16. T16 — forged presenter

- Setup: a decision records `caller_user_id` U1 from the gateway auth context. User U2, or a SQL argument equal to U1, is presented.
- Authority: `AuthContext.UserID` (`auth_context.go:14`). Not a header and not a function parameter.
- Expected database state: no binding. `consumed_at` stays null.
- Authorization result: `HARD_DENIAL`. `BINDING_WRITE=BLOCKED_UNTIL_TRUSTED_PRESENTER` because that presenter channel is not available to the consume function.
- Oracle: PASS when U2 is refused and a function argument of U1 is not treated as U1. FAIL when `decision_id` alone consumes the row.

## 17. T17 — forged device

- Setup: the decision stores `provider_device_id` taken from the resolved driver. The caller presents a different device id.
- Authority: `DEVICE_IDENTITY_SOURCE=FIXED_IN_DECISION_FROM_RESOLVED_DRIVER`.
- Expected database state: no binding. The decision device is unchanged.
- Authorization result: mismatch. No consume.
- Oracle: PASS when the other device is refused. FAIL when the caller's device UUID replaces the decision value or is accepted as ownership.

## 18. T18 — decision replay

- Setup: one consume committed a binding for decision D, presenter U, device X, and shipment S.
- Authority: the stored binding and the consumed decision row.
- Expected database state: still one binding. `consumed_at` stays the first timestamp.
- Authorization result: the identical tuple returns that binding. A second call that changes S, the vehicle, or the device is a conflict.
- Oracle: PASS on the prior result for the identical tuple and on no change for a different tuple. FAIL when replay inserts a second binding, or when `ON CONFLICT DO NOTHING` is reported as authorization.

## 19. T19 — concurrent decision consumption

- Setup: two sessions present the same decision. One has the matching device. The other presents a different shipment id.
- Authority: the row lock `FOR UPDATE` inside the one consume transaction.
- Expected database state: one binding, for the shipment on the decision, or no binding if the first session also fails the re-read. The second session does not overwrite it.
- Authorization result: the loser waits, then conflicts if its identifiers differ.
- Oracle: PASS when only one commit exists and the different-identifier session changes nothing. FAIL when both insert, or when two service transactions are described as one atomic commit.

## 20. T20 — unauthorized update of consumed_at

- Setup: `tracking_runtime` and `shipment_runtime` attempt `UPDATE` of `consumed_at` and of `shipment_id` on `transport.binding_authority_decisions`.
- Authority: column grants. Only `binding_authority_fence` may update `consumed_at` and `use_status`. Identity columns have no such grant. The table owner remains able to update them.
- Expected database state: runtime updates do not commit. An owner update can commit.
- Authorization result: runtime denied. Owner bypass open.
- Oracle: PASS when the report marks runtime denied and `TABLE_OWNER_DECISION_BYPASS=OPEN`. FAIL when `NOLOGIN` is called a sufficient ban, or when the runtime update is described as already rejected by installed grants. The table does not exist yet.

## 21. T21 — decision identifiers modified

- Setup: a session that can act as `binding_authority_owner` changes `shipment_id` on an issued decision. A runtime role attempts the same change.
- Authority: identity columns are written only by the issue function.
- Expected database state: the runtime change does not commit. The owner change can commit and is a bypass, not a valid decision.
- Authorization result: the altered decision no longer matches the shipment row, so consume refuses it. The owner path stays open.
- Oracle: PASS when consume refuses the altered identifiers and the owner risk stays open. FAIL when a changed `shipment_id` still consumes, or when the runtime is said to be unable to change it because the role is `NOLOGIN`.

## 22. T22 — revision change while driver unchanged

- Setup: a decision records revision R1 and driver D. A successor sets `current_revision_id` to R2 (`transport_execution_successor_repository.go:172-176`). `driver_id` on the execution is still D.
- Authority: the new revision. The unchanged driver column is not a version-independent proof.
- Expected database state: no new location effect and no execution target from the R1 decision.
- Authorization result: `REAUTHORIZATION_REQUIRED` for the location right. `EXECUTION_SCOPE_DENIED` for the target.
- Oracle: PASS when R1 grants neither effect. FAIL when the location right survives because `driver_id` was not rewritten. `DRIVER_ONLY_REVISION_POLICY=REAUTHORIZATION_REQUIRED`.

## 23. T23 — shipment version changed

- Setup: the decision records `shipment.version` 3. A later shipment update commits version 4. The driver is unchanged.
- Authority: `shipment.version` on the decision.
- Expected database state: the old decision does not insert a binding and does not return a target.
- Authorization result: `REAUTHORIZATION_REQUIRED` for location. `EXECUTION_SCOPE_DENIED` for the target.
- Oracle: PASS when version 3 grants no new effect. FAIL when the driver match skips the version. `SHIPMENT_VERSION_POLICY=REAUTHORIZATION_REQUIRED`.

## 24. T24 — owner epoch absent

- Setup: a fresh decision has a current driver, an active shipment link, the current revision, and `owner_epoch=ABSENT`.
- Authority: PR #375 epoch rule. The column is not in the migration head.
- Expected database state: a driver-only location binding may be specified. No execution-target grant is stored or returned.
- Authorization result: `DRIVER_ONLY_ALLOWED` for location after the fresh check. `EXECUTION_SCOPE_DENIED` for the target.
- Oracle: PASS when the target is denied despite the link, revision, and driver. FAIL when those three alone return a target. `EXECUTION_TARGET_AUTHORIZATION=BLOCKED_WHILE_OWNER_EPOCH_ABSENT`.

## 25. T25 — reassignment between issue and consume

- Setup: a decision is issued for driver D1. Before consume, the shipment `driver_id` becomes D2.
- Authority: the re-read inside the consume transaction.
- Expected database state: no binding. The decision stays unconsumed.
- Authorization result: `HARD_DENIAL` for D1. D2 has no decision yet.
- Oracle: PASS when the issued decision does not bind D1 after the change. FAIL when consume uses the decision's copied driver and skips the current row.

## 26. T26 — ownership transfer after CREATE TABLE

- Setup: a future migration creates `transport.binding_authority_decisions`. The creator is the owner. The next statement is `ALTER TABLE ... OWNER TO binding_authority_owner`.
- Roles: the creating session, then `binding_authority_owner`. `binding_authority_fence` is not the target.
- Expected SQL result: `pg_class.relowner` names `binding_authority_owner`. `REVOKE` without that statement leaves the creator as owner.
- Binding state: no binding. The table is still proposed.
- Oracle: PASS when the report requires the `OWNER TO` statement and records that `REVOKE` is not the transfer. FAIL when the design says `REVOKE` removes owner rights. Not executed.

## 27. T27 — migration role without table ownership after transfer

- Setup: the transfer migration grants `binding_authority_owner` to `freight_migrations` with `INHERIT FALSE, SET TRUE`, transfers ownership, then revokes the membership before commit.
- Roles: `freight_migrations` after commit. It is `NOSUPERUSER` (`000103:17`, `:46`).
- Expected SQL result: `freight_migrations` is not `relowner` and is not a member. `SET ROLE binding_authority_owner` fails. `INSERT`, `UPDATE`, and `DELETE` fail.
- Binding state: unchanged.
- Oracle: PASS when membership does not survive the migration. FAIL when `SET TRUE` is left in place, or when `NOLOGIN` is treated as the ban. Not executed.

## 28. T28 — decision SELECT privilege

- Setup: `binding_authority_fence` selects the decision columns listed in the binding-write contract. `tracking_runtime` selects the same row.
- Roles: `binding_authority_fence`, `tracking_runtime`.
- Expected SQL result: the fence select is the designed grant. The runtime select is denied. Neither grant exists today.
- Binding state: unchanged.
- Oracle: PASS when runtime `SELECT` is absent from the design and the fence list matches the proposed columns. FAIL when `tracking_runtime` is granted table `SELECT`. Not executed.

## 29. T29 — assignment SELECT privilege

- Setup: `binding_authority_fence` selects `transport.shipments` directly. The same session calls `transport.read_binding_assignment` for one shipment id.
- Roles: `binding_authority_fence`, `assignment_lock_owner`.
- Expected SQL result: direct table `SELECT` is denied. The function returns the row's own `tenant_id`, `driver_id`, `vehicle_id`, `version`, link, revision, and operating tenant. It does not return an owner epoch.
- Binding state: unchanged. The function does not insert.
- Oracle: PASS when the direct grant is absent and the function is the only assignment path. FAIL when the fence has table `SELECT` on shipments or executions. Not executed.

## 30. T30 — consumed_at reset attempt

- Setup: a decision is `CONSUMED` with a non-null `consumed_at`. `binding_authority_fence` updates `consumed_at` to null. The trigger is installed.
- Roles: `binding_authority_fence`.
- Expected SQL result: the trigger raises. The row stays `CONSUMED` with the original timestamp. The `WHERE consumed_at IS NULL` clause would also change zero rows, and zero rows is a failure.
- Binding state: the stored binding is unchanged.
- Oracle: PASS when the reset does not commit. FAIL when a null `consumed_at` on a `CONSUMED` row is accepted. Not executed.

## 31. T31 — CONSUMED to ISSUED attempt

- Setup: the same consumed row. A session with `UPDATE` on `use_status` sets `use_status` back to `ISSUED`.
- Roles: `binding_authority_fence`.
- Expected SQL result: the trigger raises. Status stays `CONSUMED`.
- Binding state: unchanged.
- Oracle: PASS when the reverse transition does not commit. FAIL when the `WHERE use_status = 'ISSUED'` clause is described as enough to stop every writer. Not executed.

## 32. T32 — bound identifier modification

- Setup: an issued decision. The fence updates `shipment_id`. The owner updates `shipment_id` with the trigger still enabled.
- Roles: `binding_authority_fence`, `binding_authority_owner`.
- Expected SQL result: the fence has no `UPDATE` on `shipment_id`. The trigger also raises if the statement runs. The owner can disable the trigger and then change `shipment_id`.
- Binding state: no binding from the fence statement.
- Oracle: PASS when the fence change does not commit and the owner path stays `TABLE_OWNER_DECISION_BYPASS=OPEN`. FAIL when the fence update is reported as already rejected by an installed grant. Not executed.

## 33. T33 — simultaneous consumption

- Setup: two sessions present the same decision and the same identity. Both use execution, then shipment, then decision when the decision has an execution. A driver-only decision skips the execution lock.
- Roles: the consume function owner. `EXECUTE` is not granted to a runtime role.
- Expected SQL result: one transaction commits. The other waits, then reads `CONSUMED` and returns the same binding. `consumed_at` is the first timestamp.
- Binding state: one binding.
- Oracle: PASS when the second session does not insert and does not reset `consumed_at`. FAIL when the sessions are two service commits described as one atomic result. Not executed.

## 34. T34 — crash after commit, response lost

- Setup: the consume transaction has committed. The caller did not receive the response and retries the same presenter, device, and bound identifiers.
- Roles: the consume function, still ungranted.
- Expected SQL result: the retry reads `binding_id` and returns that binding. It does not run the conditional update.
- Binding state: still one binding. `consumed_at` unchanged.
- Oracle: PASS when the retry is a read of the stored result. FAIL when the retry inserts a second binding or treats the returned row as a new grant. Not executed.

## 35. T35 — replay with a different presenter or device

- Setup: the decision is `CONSUMED` for presenter U1 and device X. The retry uses U2 or device Y.
- Roles: the consume function.
- Expected SQL result: conflict. No `UPDATE` and no `INSERT`.
- Binding state: the original binding remains.
- Oracle: PASS when U2 and Y change nothing. FAIL when a different presenter receives the binding or resets `use_status`. Not executed.

## 36. T36 — privileged owner bypass documented

- Setup: `binding_authority_owner` disables the transition trigger and sets `use_status` to `ISSUED` and `consumed_at` to null. A superuser does the same.
- Roles: table owner and superuser.
- Expected SQL result: both can commit. The design records the bypass.
- Binding state: the binding row may still exist. The decision no longer matches the one-way rule.
- Oracle: PASS when the report keeps `TABLE_OWNER_DECISION_BYPASS=OPEN` and does not call the trigger absolute. FAIL when the owner is described as stopped by `REVOKE` or by `NOLOGIN`. Not executed.

## 37. T37 — missing SELECT privilege fails closed

- Setup: `assignment_lock_owner` lacks `SELECT` on `transport.shipments.driver_id`, or `binding_authority_fence` lacks `EXECUTE` on `transport.read_binding_assignment`.
- Roles: those two roles. `tracking_runtime` has neither.
- Expected SQL result: the consume function raises. It does not insert a binding and does not mark the decision `CONSUMED`. It does not call `FindActiveBindingByDeviceAnyTenant`.
- Binding state: no new binding.
- Oracle: PASS when the failure is closed. FAIL when a missing grant falls back to the caller's headers or to `status = 'active'`. Not executed.

## 38. T38 — consume versus execution command

- Setup: session A runs a driver command. It has locked `transport.transport_executions` (`transport_execution_command_repository.go:70-75`) and is waiting to lock the shipment. Session B is consume for that execution and shipment.
- Roles: the command session, and the ungranted consume function.
- Statement order: A locks the execution, then waits for the shipment. B locks the execution first, then the shipment.
- Expected locks: B waits on A's execution `FOR UPDATE`. A does not wait on a shipment lock held by B.
- Expected SQL result: no `40P01`. One session proceeds when the other commits or rolls back.
- Decision state: unchanged while B waits. Consumed only if B later commits a match.
- Binding state: no binding from the wait itself.
- Oracle: PASS when the two sessions do not form a cycle. FAIL when B locks the shipment before the execution. Not executed.

## 39. T39 — execution-first lock order

- Setup: one execution-scoped decision. The consume function is traced, not granted.
- Roles: `assignment_lock_owner` for the share, `binding_authority_fence` for the later decision lock.
- Statement order: execution `FOR SHARE`, shipment `FOR SHARE`, driver `FOR SHARE`, vehicle `FOR SHARE` when a vehicle id is present, decision `FOR UPDATE`, binding insert.
- Expected locks: that order, held to commit or rollback.
- Expected SQL result: success only when the locked rows match the decision.
- Decision state: `CONSUMED` only on that success.
- Binding state: one row only on that success.
- Oracle: PASS when the trace shows execution before shipment. FAIL when the shipment share is first. Not executed.

## 40. T40 — driver-only binding without execution

- Setup: the decision has a null `execution_id`. An active link to an execution exists by the time of the re-read.
- Roles: `assignment_lock_owner`.
- Statement order: shipment `FOR SHARE`, then driver share. No execution lock.
- Expected locks: no `FOR SHARE` on `transport.transport_executions` in this transaction.
- Expected SQL result: rollback with `REAUTHORIZATION_REQUIRED`.
- Decision state: stays `ISSUED`.
- Binding state: no insert.
- Oracle: PASS when the function does not lock the execution after the shipment. FAIL when it locks the execution to complete the check. Not executed.

## 41. T41 — assignment change while consume waits

- Setup: session A has updated `transport.shipments.driver_id` and not committed. Session B is consume for the old driver.
- Roles: the shipment updater and the assignment reader.
- Statement order: A holds the shipment row. B reaches `FOR SHARE` on that shipment and waits. A commits.
- Expected locks: B's share starts after A's commit.
- Expected SQL result: B reads the new driver and does not consume.
- Decision state: stays `ISSUED`.
- Binding state: no insert.
- Oracle: PASS when the pre-wait driver is not written into a binding. FAIL when B consumes the driver it saw before the wait. Not executed.

## 42. T42 — no UPDATE grant rejects FOR UPDATE

- Setup: a role has the column `SELECT` list and no `UPDATE`. It runs `SELECT ... FOR UPDATE` on `transport.shipments`. This role is not the designed `assignment_lock_owner` grant.
- Roles: a column-select role with no `UPDATE`.
- Statement order: the `FOR UPDATE` statement only.
- Expected locks: none.
- Expected SQL result: `42501`.
- Decision state: unchanged.
- Binding state: unchanged.
- Oracle: PASS on `42501`. FAIL when `FOR UPDATE` succeeds without an `UPDATE` grant. Isolated PostgreSQL 16.15 returned `42501` for `FOR UPDATE` with column `SELECT` and no `UPDATE` (0.1I `P9`). The designed lock owner has `UPDATE (updated_at)`, so this oracle does not apply to that role. Under that grant, `FOR UPDATE` returned OK (0.1I-R1 `B4`).

## 43. T43 — column SELECT is not enough for FOR SHARE

- Setup: a role has column `SELECT` on `transport.shipments` and no `UPDATE`. A second role has table `SELECT` and no `UPDATE`.
- Roles: those two roles. Not the designed lock-owner grant.
- Statement order: `SELECT id ... FOR SHARE` in each session.
- Expected locks: none.
- Expected SQL result: `42501`.
- Decision state: unchanged.
- Binding state: unchanged.
- Oracle: PASS on `42501`. FAIL when `FOR SHARE` succeeds with `SELECT` and no `UPDATE`. The R4 oracle, which required the share to succeed without an `UPDATE` grant, is withdrawn. Isolated PostgreSQL 16.15 returned `42501` for column `SELECT` (0.1I `P1`, `P1c`; 0.1I-R1 `B1`) and for table `SELECT` (0.1I-R1 `B2`).

## 50. T50 — lock owner FOR SHARE on the four tables

- Setup: `assignment_lock_owner` has column `SELECT` and `UPDATE (updated_at)` on the shipment, driver, vehicle, and execution tables in an isolated database.
- Roles: `assignment_lock_owner`.
- Statement order: `FOR SHARE` on each table.
- Expected locks: the share is acquired.
- Expected SQL result: success on each table.
- Decision state: unchanged.
- Binding state: unchanged.
- Oracle: PASS when all four shares succeed and column `SELECT` alone still returns `42501`. Isolated PostgreSQL 16.15 returned OK for shipment, driver, vehicle, and execution (`B3`, `B5`, `B6`, `B7`). The production schema was not used.

## 51. T51 — updated_at write and identity column denial

- Setup: the same lock owner updates `updated_at`, then updates `driver_id`.
- Roles: `assignment_lock_owner`.
- Statement order: `UPDATE updated_at`, then `UPDATE driver_id`.
- Expected locks: the row lock of each update statement.
- Expected SQL result: `updated_at` changes. `driver_id` returns `42501`.
- Decision state: unchanged.
- Binding state: unchanged.
- Oracle: PASS when the timestamp changes and the identity column is denied. Isolated PostgreSQL 16.15 changed `updated_at` (`B9`) and returned `42501` for `driver_id` (`B8`). `DIRECT_UPDATED_AT_RISK=OPEN_FOR_LOCK_OWNER`.

## 52. T52 — fence and runtime cannot assume the lock owner

- Setup: `binding_authority_fence` has `EXECUTE` only. `tracking_runtime` has no membership and no `EXECUTE`.
- Roles: fence and runtime logins.
- Statement order: fence `UPDATE updated_at`; runtime `SET ROLE assignment_lock_owner`; fence `SET ROLE assignment_lock_owner`; `PUBLIC` and runtime `EXECUTE`.
- Expected locks: none acquired.
- Expected SQL result: `42501` on each statement. Fence `EXECUTE` of the definer function succeeds, with `current_user` equal to `assignment_lock_owner`.
- Decision state: unchanged.
- Binding state: unchanged.
- Oracle: PASS on those `42501` results and on the definer identity. Isolated PostgreSQL 16.15 matched them (`B10` through `B15`). `current_user` was `assignment_lock_owner` and `driver_id` was `33333333-3333-3333-3333-333333333333`.

## 53. T53 — membership is the updated_at bypass

- Setup: a login receives `assignment_lock_owner` with `INHERIT TRUE`, then the grant is revoked. Another login receives it with `INHERIT FALSE, SET TRUE`, then that grant is revoked.
- Roles: the two probe logins and `assignment_lock_owner`.
- Statement order: inherit and update; revoke; update again; update without `SET ROLE`; update after `SET ROLE`; revoke; `SET ROLE`.
- Expected locks: the row lock only while the membership allows the update.
- Expected SQL result: `INHERIT TRUE` updates `updated_at`. After revoke, `42501`. `SET TRUE` without `SET ROLE` is `42501`. With `SET ROLE`, the update succeeds. After revoke, `SET ROLE` is `42501`.
- Decision state: unchanged.
- Binding state: unchanged.
- Oracle: PASS when each of those results occurs and the fence and runtime membership count is then `0`. Isolated PostgreSQL 16.15 matched them (`B16` through `B21`). The design does not grant either membership.

## 54. T54 — share still conflicts with UPDATE

- Setup: `assignment_lock_owner` holds shipment `FOR SHARE`. Another session updates `version` with `lock_timeout` of 2 seconds.
- Roles: the lock owner and the updater.
- Statement order: share, then the blocked update, then the holder's commit.
- Expected locks: the update waits on the share.
- Expected SQL result: `55P03`. `version` stays `1`.
- Decision state: unchanged.
- Binding state: unchanged.
- Oracle: PASS on `55P03` and an unchanged `version`. Isolated PostgreSQL 16.15 returned `55P03` and left `version` at `1` (`L1`, `L2`).

## 55. T55 — rollback fields, not the psql script text

- Setup: one transaction sets `use_status` to `CONSUMED` and sets `consumed_at`, then rolls back.
- Roles: the session that can update the decision row.
- Statement order: `BEGIN`, `UPDATE`, `ROLLBACK`, then two separate queries.
- Expected locks: none after the rollback.
- Expected SQL result: `SELECT use_status` returns `ISSUED`. `SELECT consumed_at IS NULL` returns true. No error SQLSTATE is raised.
- Decision state: `ISSUED`, `consumed_at` null.
- Binding state: unchanged.
- Oracle: PASS on those two fields. FAIL when the assertion compares the whole `psql -tAc` stdout of the multi-statement script with `ISSUED|null`. That stdout also contains the command tags `BEGIN`, `UPDATE 1`, and `ROLLBACK`. Isolated PostgreSQL 16.15 reproduced that stdout, and the separate fields were `ISSUED` and `t`. The predicate `use_status = 'ISSUED' AND consumed_at IS NULL` returned PASS (`H1`, `H5`).

## 44. T44 — share against the command writer

- Setup: the command session is inside `loadShipment` (`transport_execution_command_repository.go:1145-1147`), waiting or holding `FOR UPDATE` on the shipment. Consume holds `FOR SHARE` on the same shipment, after its execution share.
- Roles: the command transaction and `assignment_lock_owner`.
- Statement order: both have already locked the same execution, execution first.
- Expected locks: the shipment lock modes conflict. One waits. Neither then locks the execution in the other direction.
- Expected SQL result: no `40P01` from this pair.
- Decision state: unchanged by the wait.
- Binding state: unchanged by the wait.
- Oracle: PASS when the wait is on the shipment and the execution order matches. FAIL when the sessions deadlock. Not executed.

## 45. T45 — trigger creation before membership revoke

- Setup: the future migration has run `ALTER TABLE ... OWNER TO binding_authority_owner`.
- Roles: `freight_migrations` and `binding_authority_owner`.
- Statement order: `SET ROLE`, `CREATE FUNCTION`, `CREATE TRIGGER`, `RESET ROLE`, then `REVOKE`.
- Expected locks: none. This is DDL.
- Expected SQL result: `CREATE TRIGGER` as `freight_migrations` after the revoke fails. The same statement during `SET ROLE binding_authority_owner` is the designed success.
- Decision state: the table is not visible to other sessions until the migration commits.
- Binding state: none.
- Oracle: PASS when the documented order creates the trigger before the revoke. FAIL when the contract revokes membership and then creates the trigger as the migration role. Not executed.

## 46. T46 — function owner transfer

- Setup: `CREATE FUNCTION transport.reject_binding_decision_reversal()` runs during `SET ROLE binding_authority_owner`.
- Roles: `binding_authority_owner`, then `tracking_runtime`.
- Statement order: create while set to the owner, revoke membership, then `tracking_runtime` attempts `CREATE OR REPLACE FUNCTION`.
- Expected locks: none.
- Expected SQL result: the function owner is `binding_authority_owner`. The runtime replace fails.
- Decision state: unchanged.
- Binding state: unchanged.
- Oracle: PASS when the owner is the role that created the function and the runtime cannot replace the body. FAIL when `ALTER FUNCTION OWNER` is left until after the membership revoke on the non-superuser path. Not executed.

## 47. T47 — migration role after revoke

- Setup: the migration transaction has committed.
- Roles: `freight_migrations`.
- Statement order: `SET ROLE binding_authority_owner`, then `UPDATE` of `consumed_at`.
- Expected locks: none acquired.
- Expected SQL result: `SET ROLE` fails. The role is not the table owner.
- Decision state: unchanged.
- Binding state: unchanged.
- Oracle: PASS when membership does not survive the commit. FAIL when `SET TRUE` remains. Not executed.

## 48. T48 — SQLSTATE 40P01 rollback

- Setup: a deadlock detector returns `40P01` to the consume session.
- Roles: the consume function's session.
- Statement order: the error aborts the transaction.
- Expected locks: the shares, the decision lock, and any binding index lock from this transaction end with the rollback.
- Expected SQL result: `40P01`. The caller receives unavailable.
- Decision state: `ISSUED` if this transaction had not committed.
- Binding state: no row from this transaction.
- Oracle: PASS when the error is a rollback and not a consumed decision. FAIL when the handler commits the binding after `40P01`. Not executed.

## 49. T49 — concurrent consumption versus assignment update

- Setup: session A consumes an execution-scoped decision. Session B updates `shipment.driver_id` in another transaction.
- Roles: consume function and the shipment updater.
- Statement order: both use their own transactions. A locks execution, then shipment `FOR SHARE`. B's `UPDATE` needs the shipment row.
- Expected locks: one waits for the other. They do not share a second connection's commit.
- Expected SQL result: the session that acquires the shipment lock second sees the winner's commit. If B commits first, A does not consume. If A commits first, B's update waits until A's commit, and the old decision is already consumed.
- Decision state: consumed only if A read the matching driver under its share.
- Binding state: A's binding exists only if A committed. B does not insert a binding.
- Oracle: PASS when the two commits stay independent and A does not consume a driver that changed before its share. FAIL when the documents call the two commits one atomic operation. Not executed.

## Evidence classification

The product tests 1-49 were not run against the production schema. The rows below classify the isolated PostgreSQL 16.15 sessions. The two original `FAIL` rows stay visible.

| Class | Evidence | SQLSTATE or result |
| --- | --- | --- |
| Confirmed PostgreSQL failure | 0.1I `P1` and `P1c`: `FOR SHARE` with column `SELECT` and no `UPDATE`. Expectation in that run was success. | `42501` |
| Confirmed PostgreSQL failure | 0.1I-R1 `B1` column `SELECT`, `B2` table `SELECT`, both without `UPDATE`. | `42501` |
| Confirmed PostgreSQL failure | 0.1I `P9`: `FOR UPDATE` with column `SELECT` and no `UPDATE`. | `42501` |
| Confirmed PostgreSQL success | 0.1I-R1 `B3`, `B5`, `B6`, `B7`: `FOR SHARE` after `UPDATE (updated_at)` on shipment, driver, vehicle, and execution. | OK |
| Confirmed PostgreSQL success | 0.1I-R1 `B4`: `FOR UPDATE` with the same `UPDATE (updated_at)` grant. The modes do not differ in that privilege. | OK |
| Confirmed PostgreSQL success | 0.1I-R1 `B9`: lock owner `UPDATE` of `updated_at` changed the timestamp. `B8`: `UPDATE` of `driver_id`. | OK and `42501` |
| Confirmed PostgreSQL success | 0.1I-R1 `B10`-`B14`, `B17`, `B18`, `B20`: fence, runtime, `PUBLIC`, and revoked membership. | `42501` |
| Confirmed PostgreSQL success | 0.1I-R1 `B15`: fence `EXECUTE`. `current_user` was `assignment_lock_owner`. | OK |
| Confirmed PostgreSQL success | 0.1I-R1 `B16` and `B19`: `INHERIT TRUE`, and `SET TRUE` plus `SET ROLE`, can update `updated_at`. Grants were then revoked. | OK |
| Confirmed PostgreSQL success | 0.1I lock waits and `40P01`; 0.1I-R1 `L1` share against `UPDATE`. | `55P03`, version stayed `1` |
| Harness false negative | 0.1I `R1`. `psql -tAc` stdout was `BEGIN`, `UPDATE 1`, `ROLLBACK`, `ISSUED\|null`. The comparison required the whole stdout to equal `ISSUED\|null`. The stored row was `ISSUED` and `consumed_at` null. | recorded `FAIL`; database row correct |
| Harness false negative | 0.1I-R1 `H1` reproduced that stdout. `H5` compared `use_status` and `consumed_at IS NULL` in separate queries. | fields `ISSUED` and `t`; predicate PASS; no error SQLSTATE |
| Invalid initial test | 0.1I `P4`. `pg_catalog.current_user` under `search_path = pg_catalog` was parsed as a missing table. | `42P01` |
| Invalid initial test | 0.1I `P6c` returned OK while the function owner was the superuser. It did not prove the reader role. | OK, not evidence |
| Invalid initial test | 0.1I-R1 `H3` and `H4` cast the boolean to text, compared `true` with `t`, and recorded `FAIL`. The uncast field was `t`. | recorder `FAIL`; field true |
| Test not run | 0.1I table-owner `DISABLE TRIGGER`. | `NOT_RUN` |
| Test not run | Full design `RETURN QUERY` join, production schema, tests 1-49 as product tests. | `NOT_RUN` |

## Execution record

| Check | Result |
| --- | --- |
| Tests 1-41 and 44-49 as product tests | `NOT_RUN` |
| T42 negative `FOR UPDATE` without `UPDATE` | isolated `42501` |
| T43 column or table `SELECT` without `UPDATE` | isolated `42501` |
| T50-T55 isolated privilege and rollback checks | passed on PostgreSQL 16.15 in `binding_privilege_proof_r1` |
| Full design function body | `NOT_RUN` |
| Product code | unchanged |
| Migrations | none |
| Secrets | unchanged |
| Decision table | not created in the product database |
| Ownership transfer | not executed on the product database |
| `ASSIGNMENT_LOCK_PRIVILEGE_PROVEN` | `ISOLATED_PASS_NOT_PRODUCTION` |
