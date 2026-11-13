# TRACKING-BINDING-AUTHORITY-DECISION-0.1H — Binding Write

`BINDING_WRITE_ENFORCEMENT=REVOKE_INSERT_PLUS_DECISION_FUNCTION`

`DUPLICATE_BINDING_POLICY=CONFLICT_UNLESS_SAME_DECISION_SHIPMENT`

`REVOCATION_POLICY=RECHECK_AND_OMIT`

`BINDING_WRITE=BLOCKED_UNTIL_TRUSTED_PRESENTER`

`PRESENTER_IDENTITY_BOUND=NOT_IMPLEMENTED`

`DECISION_CONSUMPTION_GATE=CLOSED`

`PHYSICAL_DEVICE_AUTHENTICATION=NO`

`ATOMIC_CONSUMPTION_MODEL=SINGLE_FUNCTION_TRANSACTION_SPECIFIED_WRITE_BLOCKED`

`DECISION_REPLAY_POLICY=IDENTICAL_REPLAY_RETURNS_PRIOR_RESULT`

`ONE_WAY_CONSUMPTION=ISSUED_TO_CONSUMED_TRIGGER_SPECIFIED`

The enforcement is a privilege boundary. A documented call order is not the control. Nothing here is applied. `DIRECT_BINDING_INSERT_BYPASS=OPEN` on the current insert. The designed consume function is not granted, because the presenter cannot be passed in as a trusted value. `EXECUTE` stays revoked until a gateway-to-database presenter channel exists that is not a header, a body, or a SQL argument. `IMPLEMENTATION_GATE=CLOSED`.

## 1. One transaction, when the presenter exists

Shipment-service and tracking-service do not share a transaction. Two commits, one in each service, are not atomic. This contract does not call that pair atomic.

`transport` and `tracking` are schemas in one database. The designed consume function runs in one session and one transaction. It is `SECURITY DEFINER`, owned by `binding_authority_fence`, and it is not the table owner. `tracking_runtime` does not receive `EXECUTE`. The write stays blocked. `DECISION_CONSUMPTION_GATE=CLOSED`.

Inside that single transaction the order is fixed. The first read of the decision does not lock it and is not the authority. It only learns `shipment_id` and `execution_id`.

1. Lock `transport.shipments` for that `shipment_id` with `FOR UPDATE`.
2. When `execution_id` is not null, lock `transport.transport_executions` for that id with `FOR UPDATE`.
3. Lock the decision row with `FOR UPDATE`. If the locked row's `shipment_id` or `execution_id` is not the row already locked, the function rolls back. The unlocked read is not the authority.
4. Reject unless the trusted presenter equals `caller_user_id`. A header, a JSON field, or a function argument is not that presenter.
5. Reject unless the presented device equals `provider_device_id` stored on the decision. The presented value is compared. It is not the source of the device. The stored value is the resolved driver id as text. `PHYSICAL_DEVICE_AUTHENTICATION=NO`.
6. Reject when `expires_at` has passed.
7. If `use_status` is already `CONSUMED`, do not update the row. Compare the full identity in section 6. An identical tuple returns the stored binding. Any difference is a conflict.
8. Re-read the assignment through the function in section 4, while the locks from steps 1 and 2 are still held.
9. Insert the binding only when that locked re-read matches the decision. `ON CONFLICT DO NOTHING` is not used. A conflicting active device row whose shipment differs is a conflict, and the decision stays unconsumed.
10. Update the decision only with `use_status = 'ISSUED'` and `consumed_at IS NULL`. Set `consumed_at`, `use_status = CONSUMED`, and `binding_id` to the inserted binding. If that update changes zero rows, the function raises and the transaction rolls back. Zero rows is not authorization.
11. Commit.

The lock order is shipment, then execution, then decision. A second session that consumes the same decision waits on the shipment lock or the decision lock. It does not insert a second binding.

A crash before commit rolls the transaction back. The decision stays `ISSUED`, `consumed_at` stays null, and no binding remains. A crash after commit, with the response lost, leaves the decision `CONSUMED` and the binding stored. The retry is the replay in section 6. It does not consume the decision again and does not change `consumed_at`.

The function arguments do not include tenant, driver, vehicle, shipment, or presenter. Device is an input only for the equality check in step 5.

`owner_epoch=ABSENT` does not satisfy an execution target. The function may record a driver-only binding when the location checks pass. It does not record an execution-target grant while the epoch is absent. `EXECUTION_TARGET_AUTHORIZATION=BLOCKED_WHILE_OWNER_EPOCH_ABSENT`.

The current insert (`tracking_repository.go:472-477`) accepts the caller's tenant, shipment, driver, and vehicle and is the bypass. The HTTP handler at `tracking_handler.go:96-135` is that bypass. `REVOKE INSERT` on the binding table, from `tracking_runtime` and from `freight`, is what stops a SQL session from calling `INSERT` directly. Removing the Go call without the `REVOKE` leaves the bypass open. Neither revoke is applied. `REVOKE` also does not transfer ownership of the decision table. That transfer is in the authority contract.

## 2. One-way consumption

`ONE_WAY_CONSUMPTION=ISSUED_TO_CONSUMED_TRIGGER_SPECIFIED`

The only designed status transition is `ISSUED` to `CONSUMED`. The following are rejected: `CONSUMED` to `ISSUED`, `consumed_at` set back to null, a change of `consumed_at` after it was set, a change of any bound identifier, and a second binding for the same decision. Replay is a read. It is not a second transition.

The consume update is conditional:

```sql
UPDATE transport.binding_authority_decisions
   SET consumed_at = pg_catalog.clock_timestamp(),
       use_status = 'CONSUMED',
       binding_id = $binding_id
 WHERE decision_id = $decision_id
   AND use_status = 'ISSUED'
   AND consumed_at IS NULL
   AND binding_id IS NULL;
```

That statement is design SQL. It is not executed. The `WHERE` clause protects this update. It does not stop another writer that has `UPDATE` from setting the row back to `ISSUED`. `CONSUMED_AT_RESET_DENIED_BY_DESIGN=TRIGGER_NOT_ABSOLUTE_AGAINST_OWNER`.

The server restriction is a `BEFORE UPDATE` trigger on `transport.binding_authority_decisions`, for each row, created `ENABLE ALWAYS`. The trigger function is owned by `binding_authority_owner`. It compares `OLD` and `NEW` and raises on a forbidden transition. It allows `use_status` and `consumed_at` to change only when `OLD.use_status` is `ISSUED`, `OLD.consumed_at` is null, `NEW.use_status` is `CONSUMED`, and `NEW.consumed_at` is not null. It allows `binding_id` to change only from null to a value in that same transition. It rejects every change to `decision_id`, `caller_user_id`, `shipment_tenant_id`, `operating_tenant_id`, `shipment_id`, `driver_id`, `vehicle_id`, `vehicle_scope`, `execution_id`, `current_revision_id`, `shipment_version`, `revision_version`, `owner_epoch`, `relationship_required`, `provider_code`, `provider_device_id`, `issued_at`, and `expires_at`.

Install order in the future migration, after the ownership transfer and before any `EXECUTE` grant: create the trigger function, `ALTER FUNCTION ... OWNER TO binding_authority_owner`, create the trigger, then grant the column privileges in section 5. The trigger is not created in this change.

`binding_authority_fence` is not the table owner, so it cannot `DISABLE TRIGGER`. A runtime role with no `UPDATE` cannot start the reverse update. The table owner can drop or disable the trigger, and a superuser can do both. `ENABLE ALWAYS` keeps the trigger active when `session_replication_role` is `replica`. It does not stop the owner or a superuser. `TABLE_OWNER_DECISION_BYPASS=OPEN`.

If the conditional update reports zero rows, the function raises. The binding insert in the same transaction rolls back with it. The caller does not receive a new binding, and the function does not treat the old row as a newly authorized write.

## 3. Cases

| Case | Control |
| --- | --- |
| Direct internal insert | No `decision_id`, or a session without `EXECUTE`, writes nothing. Token equality (`tracking_handler.go:97-99`) does not become a decision. |
| Forged tenant | The function has no tenant argument. The decision's tenants come from the shipment and execution rows. |
| Forged driver or vehicle | The function has no driver or vehicle argument. A header value never reaches the insert. |
| Duplicate binding | One active row per tenant, provider, and device remains the unique index (`000026_add_shipment_telemetry_foundation_v0.7.0.up.sql:28-30`). |
| `ON CONFLICT DO NOTHING` race | That clause is not used. The current clause is `tracking_repository.go:476`. It can return success when another shipment won. The function locks the unique key, and on conflict reads the stored shipment. A different shipment is a conflict error. Success means the stored shipment is the decision's shipment. |
| Stale assignment | The re-read inside the same transaction fails the version, driver, vehicle, or revision check. No insert. Outcome `REAUTHORIZATION_REQUIRED`. The old decision is not consumed. |
| Revoked assignment | The driver no longer matches or the driver is inactive. `HARD_DENIAL`. No insert. A row already stored is not a grant on later reads. |
| Historical unverified rows | Rows written by the current insert have no `decision_id`. The function does not attach one. Target search skips them. `HISTORICAL_BINDING_POLICY=NOT_A_GRANT`. |
| Forged presenter | Presenter differs from `caller_user_id`. No insert. The channel that would supply the presenter is unavailable, so this function is not granted. |
| Forged device | Presented device differs from `provider_device_id` on the decision. No insert. |
| Replay | Identical presenter, device, and bound identifiers return the prior binding. Different identifiers conflict and leave the binding unchanged. |

Two tenants may each have an active row for one device. That is not a duplicate. The unique index is per tenant.

## 4. Assignment read

`ASSIGNMENT_SELECT_PATH=SHIPMENT_OWNED_READ_FUNCTION`

`DECISION_SELECT_PATH=COLUMN_SELECT_FOR_BINDING_AUTHORITY_FENCE`

The consume function does not receive `SELECT` on `transport.shipments`, `transport.drivers`, `transport.transport_executions`, `transport.transport_execution_revisions`, `transport.transport_execution_active_shipments`, or `transport.transport_execution_carrier_relationship_pins`. A table-wide `SELECT` for `binding_authority_fence` would let a session that can become that role read every tenant. That grant is not in this design. `TENANT_ISOLATION_IMPLEMENTED=NO`.

The assignment check is one proposed function, `transport.read_binding_assignment(uuid)`. The argument is the shipment id copied from the locked decision. It is not a tenant argument and it is not authorization. The function is `SECURITY DEFINER`, owned by a proposed `NOLOGIN` role `shipment_assignment_reader`. That role is not `shipment_runtime`, not `binding_authority_fence`, and not the owner of `transport.shipments`. `EXECUTE` is granted only to `binding_authority_fence`. It is not granted to `tracking_runtime`, `shipment_runtime`, `freight`, or `PUBLIC`. The function does not commit. Its locks are held by the calling transaction, which is the consume transaction. Calling it from another connection would be a second transaction, and this contract does not call that atomic.

The function owner is granted `SELECT` only on these existing columns:

- `transport.shipments`: `id`, `tenant_id`, `driver_id`, `vehicle_id`, `version`, `deleted_at` (`000003_create_transport_tables.up.sql:159-184`)
- `transport.drivers`: `id`, `tenant_id`, `status`, `deleted_at` (`000003:99-115`)
- `transport.transport_execution_active_shipments`: `shipment_tenant_id`, `shipment_id`, `execution_id` (`000088_tms_transport_execution_foundation_v0_1a.up.sql:76-81`)
- `transport.transport_executions`: `id`, `operating_tenant_id`, `driver_id`, `vehicle_id`, `current_revision_id` (`000088:10-16`)
- `transport.transport_execution_revisions`: `id`, `version` (`000088:22-34`)
- `transport.transport_execution_carrier_relationship_pins`: existence for (`execution_revision_id`, `shipment_tenant_id`, `shipment_id`) (`000103_tms_relationship_pins_security_foundation_v0_1.up.sql:128-142`)

There is no `owner_epoch` column. The function does not select one and does not return a numeric epoch. The consume function keeps `owner_epoch=ABSENT` and does not grant an execution target. The function locks the shipment row, then the execution row when the active link has one, in the order in section 1. It returns the row's own tenant, driver, vehicle, version, link, revision, revision `version`, and operating tenant. The consume function compares those values with the decision. A caller-supplied tenant that matches is not the check.

A missing `SELECT` or a missing `EXECUTE` fails the function closed. It does not fall back to the unscoped binding query.

## 5. Privileges

`FUNCTION_OWNER_PRIVILEGES=DECISION_COLUMN_SELECT_AND_CONSUME_UPDATE_ONLY`

`binding_authority_fence` needs `SELECT` on the decision columns the consume function reads. Those columns are the proposed row, not columns already stored in a table: `decision_id`, `caller_user_id`, `shipment_tenant_id`, `operating_tenant_id`, `shipment_id`, `driver_id`, `vehicle_id`, `vehicle_scope`, `execution_id`, `current_revision_id`, `shipment_version`, `revision_version`, `owner_epoch`, `relationship_required`, `provider_code`, `provider_device_id`, `issued_at`, `expires_at`, `consumed_at`, `use_status`, and `binding_id`. The grant is column `SELECT`, not `SELECT` to `tracking_runtime`. The fence also needs `UPDATE` on `consumed_at`, `use_status`, and `binding_id` only. Identity columns have no `UPDATE` grant. The trigger in section 2 still rejects a reverse transition and an identifier change even if a later grant adds `UPDATE`.

Binding `INSERT` is limited to the existing columns the function writes (`000026_add_shipment_telemetry_foundation_v0.7.0.up.sql:5-17`): `tenant_id`, `shipment_id`, `vehicle_id`, `driver_id`, `provider_code`, `provider_device_id`, `status`, `active_from`. `RETURNING id` needs `SELECT` on `id` for `binding_authority_fence`. The current table has no `decision_id` column, and this design does not add one there. The decision's `binding_id` points at `tracking.shipment_tracking_binding.id`.

| Role | Decision SELECT | Decision INSERT | Decision UPDATE | Decision DELETE | Binding INSERT |
| --- | --- | --- | --- | --- | --- |
| `binding_authority_owner` | Owner. Reads every column because it owns the table. | Only through the issue function it owns. Direct `INSERT` remains possible for the owner. | Owner. Can change `consumed_at` and identifiers, and can disable the trigger. | Owner. The design has no delete function. The owner can still delete. | Not required. The owner is not the binding writer. |
| `binding_authority_fence` | Column `SELECT` listed above, so the consume function can compare the row it locked. | None. It is not the issuer. | `consumed_at`, `use_status`, and `binding_id` only, so the one-way update can run. The trigger blocks the reverse. | None. | The binding columns listed above, so the same transaction can insert the row whose id is stored on the decision. |
| `shipment_assignment_reader` | None. | None. | None. | None. | None. Its `SELECT` is on the assignment columns in section 4, so the read function can return one shipment's current row. |
| `shipment_runtime` | None. A direct read would trust the runtime session. | None. Issue is a future `EXECUTE` on the issue function, not table `INSERT`. | None. | None. | None. |
| `tracking_runtime` | None. The runtime must not learn a decision by table read. | None. | None. This is what blocks a runtime `UPDATE` of `consumed_at`. | None. | None. The current direct insert stays open until this revoke exists. |
| `freight` | None in this design. | None in this design. | None in this design. | None in this design. | None in this design. Where `freight` owns an existing table, owner rights survive `REVOKE` (`000103:6-7`). |
| `freight_migrations` | None after the ownership transfer. | None after the transfer. The role is not a member of the owner. | None after the transfer. | None after the transfer. | None. |
| `PUBLIC` | Revoked. | Revoked. | Revoked. | Revoked. | Revoked. |
| Operator superuser | Can read. | Can insert. | Can update and can disable the trigger. | Can delete. | Can insert. Not a designed grant. |

The column `UPDATE` grant is what stops `binding_authority_fence` from assigning a new `shipment_id` in the normal statement. It does not stop `binding_authority_owner` or a superuser. The trigger stops the fence, and any other grantee, from reversing `CONSUMED` while the trigger is installed. It does not stop the owner who disables it. `TABLE_OWNER_DECISION_BYPASS=OPEN`. `SHARED_FREIGHT_RISK=OPEN`. `NOLOGIN` on the owner does not close either risk.

## 6. Replay

`DECISION_REPLAY_POLICY=IDENTICAL_REPLAY_RETURNS_PRIOR_RESULT`

After commit, a request with the same `decision_id` locks the decision and reads `use_status`, `consumed_at`, and `binding_id`. It does not run the conditional update. It loads `tracking.shipment_tracking_binding` by `binding_id` and compares the trusted presenter with `caller_user_id`, the presented device with `provider_device_id`, and the rest of the bound identity: `shipment_tenant_id`, `operating_tenant_id`, `shipment_id`, `driver_id`, `vehicle_id`, `vehicle_scope`, `execution_id`, `current_revision_id`, `shipment_version`, `revision_version`, `provider_code`. The binding row's `tenant_id`, `shipment_id`, `driver_id`, `vehicle_id`, `provider_code`, and `provider_device_id` must match that decision. A full match returns that binding and leaves `consumed_at` at the original timestamp.

A different presenter, a different device, or a different shipment is a conflict. The function updates nothing and inserts nothing. A replay does not set `use_status` back to `ISSUED` and does not null `consumed_at`. Returning the stored binding is not a new authorization and is not a new location effect.

Two concurrent consume calls take the locks in section 1. One commits. The other waits, then sees `CONSUMED`. If its identity matches, it returns the same binding. If its identity differs, it conflicts. Neither call resets `consumed_at`.

## 7. Assignment race

`ASSIGNMENT_ROW_LOCK=SPECIFIED_NOT_IMPLEMENTED`

A lock on the decision row does not lock `transport.shipments` or `transport.transport_executions`. Under `READ COMMITTED`, a re-read without `FOR UPDATE` on those rows sees the latest commit at that statement and does not keep the assignment unchanged until the binding commit. Another transaction can change `driver_id`, `vehicle_id`, or `version` after that read and before the insert. The use-time recheck is a later read. It does not undo a location effect that already committed.

The future condition is the lock order in section 1, held from the re-read through the binding insert and the consume update, in one transaction. That order matches shipment writers that lock the shipment before the execution: shipment, then execution, then decision. It is not installed. Two service transactions are still not one transaction. `DECISION_CONSUMPTION_GATE=CLOSED`.

## 8. What the revoke does not stop

The table owner and a superuser are not limited by `GRANT` and `REVOKE`. PR #376 left that bypass open. `SHARED_FREIGHT_RISK=OPEN`. `ALL_WRITERS_FENCED=NO`. This contract does not claim the owner is fenced. Ownership transfer removes the migration role as owner. It does not fence the new owner.

`ON_CONFLICT_DO_NOTHING_RACE=OPEN` until the function replaces the current insert. The design forbids that clause. The running code still has it.

## 9. Failure behavior

The caller sees conflict or not found, with the same privacy rule as the authority contract. A database error is unavailable. The function does not insert a row marked active and hope a later check corrects it. Zero rows from the consume update is a failure, not a successful replay. Replay is only the read in section 6 after a committed `CONSUMED` row.
