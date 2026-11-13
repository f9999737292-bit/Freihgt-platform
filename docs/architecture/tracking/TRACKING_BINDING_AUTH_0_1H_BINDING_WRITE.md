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

`COMMAND_WRITER_LOCK_ORDER=EXECUTION_THEN_SHIPMENT`

`CONSUME_LOCK_ORDER=EXECUTION_THEN_SHIPMENT_THEN_DECISION_THEN_BINDING`

`DRIVER_ONLY_LOCK_ORDER=SHIPMENT_THEN_DECISION_THEN_BINDING`

`ASSIGNMENT_LOCK_MODE=FOR_SHARE`

`ASSIGNMENT_FUNCTION_OWNER=assignment_lock_owner`

`ASSIGNMENT_LOCK_PRIVILEGES=COLUMN_SELECT_PLUS_UPDATE_UPDATED_AT`

`FOR_SHARE_REQUIRES_UPDATE=YES`

`COLUMN_SELECT_ONLY_VALID=NO`

`MINIMUM_UPDATE_COLUMN=updated_at`

`LOCK_MODE_AND_PRIVILEGE_SET=SEPARATE_DECISIONS`

`DIRECT_UPDATED_AT_RISK=OPEN_FOR_LOCK_OWNER`

`ASSIGNMENT_LOCK_PRIVILEGE_PROVEN=ISOLATED_PASS_NOT_PRODUCTION`

A command transaction locks `transport.transport_executions` with `FOR UPDATE` (`transport_execution_command_repository.go:70-75`) and only later locks `transport.shipments` with `FOR UPDATE` in `loadShipment` (`:1145-1147`). `confirmAction` calls that shipment lock at `:527`, after the execution lock is already held. The same execution-then-shipment order is used by the other `loadShipment` calls in that transaction (`:620`, `:671`, `:762`, `:814`). The R3 order, shipment before execution, is the inverse of that writer. It is withdrawn.

Execution-scoped consume:

1. `SELECT ... FROM transport.transport_executions WHERE id = execution_id FOR SHARE`.
2. `SELECT ... FROM transport.shipments WHERE id = shipment_id AND deleted_at IS NULL FOR SHARE`.
3. When the locked shipment has a `driver_id`, `SELECT ... FROM transport.drivers WHERE id = that driver FOR SHARE`.
4. When the locked shipment has a `vehicle_id`, `SELECT ... FROM transport.vehicles WHERE id = that vehicle FOR SHARE`. This step stays after the driver lock. Activation locks the driver at `transport_execution_activation.go:308` and the vehicle at `:326`. A vehicle share does not turn a driver-only decision into a vehicle claim.
5. Lock the decision row with `FOR UPDATE`. If its `shipment_id` or `execution_id` is not the row already locked, the function rolls back.
6. Reject unless the trusted presenter equals `caller_user_id`. A header, a JSON field, or a function argument is not that presenter.
7. Reject unless the presented device equals `provider_device_id` stored on the decision. The presented value is compared. It is not the source of the device. The stored value is the resolved driver id as text. `PHYSICAL_DEVICE_AUTHENTICATION=NO`.
8. Reject when `expires_at` has passed.
9. If `use_status` is already `CONSUMED`, do not update the row. Compare the full identity in section 6. An identical tuple returns the stored binding. Any difference is a conflict.
10. Re-read the assignment through the function in section 4, while the locks above are still held. The re-read does not take a new lock in a different order.
11. Insert into `tracking.shipment_tracking_binding` only when that locked re-read matches the decision. `ON CONFLICT DO NOTHING` is not used. A conflicting active device row whose shipment differs is a conflict, and the decision stays unconsumed.
12. Update the decision only with `use_status = 'ISSUED'` and `consumed_at IS NULL`. Set `consumed_at`, `use_status = CONSUMED`, and `binding_id` to the inserted binding. If that update changes zero rows, the function raises and the transaction rolls back. Zero rows is not authorization.
13. Commit.

Driver-only consume, where the decision's `execution_id` is null, skips step 1. It does not look up an execution and lock it after the shipment. If the locked re-read finds an active execution link, the function rolls back with `REAUTHORIZATION_REQUIRED` and does not take the execution lock in that transaction. Taking it after the shipment lock would invert the command writer.

Every lock above is held until `COMMIT` or `ROLLBACK` of this transaction. The assignment function does not commit. A second session that uses the same order waits. It does not insert a second binding.

`SQLSTATE 40P01` is a deadlock. The consume transaction rolls back. The decision stays `ISSUED` when this transaction had not committed, no binding remains, and the caller receives unavailable. The deadlock is not a successful consume and not a replay. A lock timeout, `SQLSTATE 55P03`, is the same rollback. Locks from this transaction end with that rollback.

`FOR SHARE` is the assignment lock mode. PostgreSQL 16.15 does not accept that mode from `SELECT` alone. In an isolated database, column `SELECT` without `UPDATE` returned `42501` on `SELECT ... FOR SHARE`, and table `SELECT` without `UPDATE` returned the same `42501`. `GRANT UPDATE (updated_at)` on the locked table made `FOR SHARE` succeed. The same grant made `FOR UPDATE` succeed and made a direct `UPDATE` of `updated_at` succeed. `FOR SHARE` and `FOR UPDATE` do not differ in the `UPDATE` privilege they require. The lock mode and the privilege set are separate decisions. `FOR SHARE` stays the mode because it conflicts with an assignment `UPDATE` and with `FOR UPDATE`: 0.1I recorded a lock wait for both, and 0.1I-R1 recorded `55P03` for a shipment `UPDATE` while the share was held. Choosing `FOR UPDATE` would not remove the `UPDATE` grant. `MINIMUM_UPDATE_COLUMN=updated_at`. The value `NONE_ON_ASSIGNMENT_TABLES` is withdrawn. The decision row is the exception only in purpose: the consume function locks it with `FOR UPDATE` because it updates `consumed_at`, `use_status`, and `binding_id`. That decision update is not evidence that an assignment `FOR SHARE` can run without an `UPDATE` grant.

| Writer | Locks, in order | Compared with consume |
| --- | --- | --- |
| Driver command | Execution `FOR UPDATE` (`:70-75`), then shipment `FOR UPDATE` (`:1145-1147` from `:527`) | Same execution-then-shipment order. The second waiter blocks. No cycle. |
| AssignVehicle | `UPDATE transport.shipments` only (`shipment_repository.go:473-475`) | No execution lock. Consume's shipment `FOR SHARE` waits on that `UPDATE`, or the `UPDATE` waits on the share. |
| AssignDriver | `UPDATE transport.shipments` only (`shipment_repository.go:433-435`) | Same as AssignVehicle. |
| Successor revision | Execution `FOR UPDATE` (`transport_execution_successor_repository.go:85-89`), then `UPDATE current_revision_id` (`:172-176`). No shipment lock. | Same first lock as consume. No shipment inversion. |
| Disposition | Execution `FOR UPDATE` (`transport_execution_disposition_repository.go:487-494`). The shipment read at `:667` has no `FOR UPDATE`. | Execution first. No shipment-before-execution lock. |
| Activation, when the relationship flag is on | Shipment `FOR UPDATE` (`transport_execution_activation.go:259-263`), then driver (`:308`) and vehicle (`:326`), then insert of a new execution | It does not `FOR UPDATE` an existing execution. Consume may wait on the shipment. Activation does not then wait on the execution consume already holds. |
| Current binding insert | Unique index on `tracking.shipment_tracking_binding` (`tracking_repository.go:472-477`). No shipment or execution lock. | Consume inserts that table last. The current writer does not take a shipment or execution lock after the index. |
| Owner epoch | No column and no production lock. The 0.1D design locks the execution row first. | A future epoch writer locks the execution before any shipment. None is installed. |
| Tracking state | `tracking.execution_tracking_state` `FOR UPDATE` (`execution_tracking_repository.go:186-189`) | A different table. It does not lock `transport.shipments` or `transport.transport_executions`. |

`ALL_RELEVANT_WRITERS_CHECKED=YES`

`DEADLOCK_COUNTEREXAMPLE_RESOLVED=YES`

The resolved counterexample is consume locking the shipment and then the execution while a command holds the execution and waits for the shipment. Driver-only consume does not lock the execution, so it does not form that cycle either. This is not a claim that every product writer is fenced. `ALL_WRITERS_FENCED=NO`.

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

The server restriction is a `BEFORE UPDATE` trigger on `transport.binding_authority_decisions`, for each row. The trigger function is owned by `binding_authority_owner`. It compares `OLD` and `NEW` and raises on a forbidden transition. It allows `use_status` and `consumed_at` to change only when `OLD.use_status` is `ISSUED`, `OLD.consumed_at` is null, `NEW.use_status` is `CONSUMED`, and `NEW.consumed_at` is not null. It allows `binding_id` to change only from null to a value in that same transition. It rejects every change to `decision_id`, `caller_user_id`, `shipment_tenant_id`, `operating_tenant_id`, `shipment_id`, `driver_id`, `vehicle_id`, `vehicle_scope`, `execution_id`, `current_revision_id`, `shipment_version`, `revision_version`, `owner_epoch`, `relationship_required`, `provider_code`, `provider_device_id`, `issued_at`, and `expires_at`.

`TRIGGER_INSTALL_ORDER=SET_ROLE_OWNER_THEN_FUNCTION_THEN_TRIGGER_THEN_REVOKE`

`FUNCTION_OWNER_TRANSFER=CREATE_WHILE_SET_ROLE_BEFORE_REVOKE`

`TEMPORARY_MEMBERSHIP_REVOKED=AFTER_TRIGGER_SAME_TRANSACTION`

PostgreSQL 16 lets the table owner create a trigger on that table. After `OWNER TO`, `freight_migrations` is not the owner. It can `CREATE TRIGGER` only while it can still `SET ROLE` to `binding_authority_owner`. Revoking that membership first, and then creating the trigger as `freight_migrations`, is not a valid sequence. A superuser can create the trigger without the membership. This design does not rely on that shortcut for the non-superuser path.

The statements below are one future migration transaction. They are not executed. Nothing is committed between the ownership transfer and the trigger.

```sql
SET ROLE binding_authority_owner;
CREATE FUNCTION transport.reject_binding_decision_reversal()
    RETURNS trigger
    LANGUAGE plpgsql
    SECURITY INVOKER
    SET search_path = pg_catalog
AS $fn$
BEGIN
    IF NEW.decision_id IS DISTINCT FROM OLD.decision_id
       OR NEW.caller_user_id IS DISTINCT FROM OLD.caller_user_id
       OR NEW.shipment_tenant_id IS DISTINCT FROM OLD.shipment_tenant_id
       OR NEW.operating_tenant_id IS DISTINCT FROM OLD.operating_tenant_id
       OR NEW.shipment_id IS DISTINCT FROM OLD.shipment_id
       OR NEW.driver_id IS DISTINCT FROM OLD.driver_id
       OR NEW.vehicle_id IS DISTINCT FROM OLD.vehicle_id
       OR NEW.vehicle_scope IS DISTINCT FROM OLD.vehicle_scope
       OR NEW.execution_id IS DISTINCT FROM OLD.execution_id
       OR NEW.current_revision_id IS DISTINCT FROM OLD.current_revision_id
       OR NEW.shipment_version IS DISTINCT FROM OLD.shipment_version
       OR NEW.revision_version IS DISTINCT FROM OLD.revision_version
       OR NEW.owner_epoch IS DISTINCT FROM OLD.owner_epoch
       OR NEW.relationship_required IS DISTINCT FROM OLD.relationship_required
       OR NEW.provider_code IS DISTINCT FROM OLD.provider_code
       OR NEW.provider_device_id IS DISTINCT FROM OLD.provider_device_id
       OR NEW.issued_at IS DISTINCT FROM OLD.issued_at
       OR NEW.expires_at IS DISTINCT FROM OLD.expires_at
    THEN
        RAISE EXCEPTION 'binding decision identity is immutable' USING ERRCODE = '23514';
    END IF;
    IF OLD.use_status = 'CONSUMED' OR OLD.consumed_at IS NOT NULL THEN
        IF NEW.use_status IS DISTINCT FROM 'CONSUMED'
           OR NEW.consumed_at IS DISTINCT FROM OLD.consumed_at
           OR NEW.binding_id IS DISTINCT FROM OLD.binding_id
        THEN
            RAISE EXCEPTION 'binding decision consumption is one way' USING ERRCODE = '23514';
        END IF;
        RETURN NEW;
    END IF;
    IF OLD.use_status = 'ISSUED' AND OLD.consumed_at IS NULL AND OLD.binding_id IS NULL
       AND NEW.use_status = 'CONSUMED' AND NEW.consumed_at IS NOT NULL AND NEW.binding_id IS NOT NULL
    THEN
        RETURN NEW;
    END IF;
    IF NEW.use_status IS NOT DISTINCT FROM OLD.use_status
       AND NEW.consumed_at IS NOT DISTINCT FROM OLD.consumed_at
       AND NEW.binding_id IS NOT DISTINCT FROM OLD.binding_id
    THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'binding decision consumption is one way' USING ERRCODE = '23514';
END;
$fn$;
REVOKE ALL ON FUNCTION transport.reject_binding_decision_reversal() FROM PUBLIC;
CREATE TRIGGER trg_binding_decision_one_way
    BEFORE UPDATE ON transport.binding_authority_decisions
    FOR EACH ROW
    EXECUTE FUNCTION transport.reject_binding_decision_reversal();
GRANT EXECUTE ON FUNCTION transport.reject_binding_decision_reversal() TO binding_authority_fence;
RESET ROLE;
REVOKE binding_authority_owner FROM freight_migrations;
```

The trigger is created enabled for a normal session. `ENABLE ALWAYS`, which also fires when `session_replication_role` is `replica`, is not part of this sequence. On PostgreSQL 16 that form is a superuser operation. The migration role is not a superuser, and this design does not pretend it can set that mode. A replica session can skip an ordinarily enabled trigger. The table owner can also disable the trigger. `TABLE_OWNER_DECISION_BYPASS=OPEN`.

The function owner is `binding_authority_owner` because the function is created while that role is current. A later `ALTER FUNCTION ... OWNER TO` is not required on this path. Runtime roles receive no membership, so they cannot `SET ROLE` to the owner and cannot replace the function body. `freight_migrations` cannot either, once the `REVOKE` has run. Column grants in section 5 are issued in this same `SET ROLE` block, before `RESET ROLE`. Other sessions do not see the new table until this transaction commits, so there is no committed interval in which a runtime role can update the decision without the trigger.

`MIGRATION_PRIVILEGES_VALID=SPECIFIED_FOR_EACH_DDL_NOT_EXECUTED`

`binding_authority_fence` is not the table owner, so it cannot `DISABLE TRIGGER`. A runtime role with no `UPDATE` cannot start the reverse update. The table owner can drop or disable the trigger, and a superuser can do both. A replica session can skip the ordinarily enabled trigger. `TABLE_OWNER_DECISION_BYPASS=OPEN`.

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

The assignment check is one proposed function, `transport.read_binding_assignment(uuid, uuid)`. The first argument is the decision's `execution_id`, and it is null for a driver-only decision. The second is the decision's `shipment_id`. Neither argument is a tenant, and neither is authorization. The function is `SECURITY DEFINER`, owned by a proposed `NOLOGIN` role `assignment_lock_owner`. That role is not `shipment_runtime`, not `binding_authority_fence`, not `tracking_runtime`, and not the owner of `transport.shipments`. `shipment_assignment_reader` is not the function owner. `EXECUTE` is granted only to `binding_authority_fence`. It is not granted to `tracking_runtime`, `shipment_runtime`, `freight`, or `PUBLIC`. `REVOKE ALL ON FUNCTION transport.read_binding_assignment(uuid, uuid) FROM PUBLIC` is part of the same future migration. The function does not commit and contains no dynamic SQL. Its locks are held by the calling transaction. Calling it from another connection would be a second transaction, and this contract does not call that atomic.

`SECURITY_DEFINER_MODEL=FIXED_SEARCH_PATH_QUALIFIED_NAMES_NO_DYNAMIC_SQL`

The function is created with `SET search_path = pg_catalog`. Every table reference is schema-qualified. The body locks in the section 1 order and then reads. When `execution_id` is null it does not lock `transport.transport_executions`. When `execution_id` is not null it locks that execution before the shipment, even if the active link has already gone. The driver share and the vehicle share come after the shipment share and before the function returns. The consume transaction then locks the decision. It does not take a lock that comes before a lock already taken.

```sql
CREATE FUNCTION transport.read_binding_assignment(p_execution_id uuid, p_shipment_id uuid)
    RETURNS TABLE (
        shipment_id uuid,
        shipment_tenant_id uuid,
        driver_id uuid,
        vehicle_id uuid,
        shipment_version integer,
        shipment_deleted boolean,
        driver_status text,
        execution_id uuid,
        operating_tenant_id uuid,
        execution_driver_id uuid,
        execution_vehicle_id uuid,
        current_revision_id uuid,
        revision_version integer
    )
    LANGUAGE plpgsql
    SECURITY DEFINER
    SET search_path = pg_catalog
AS $fn$
DECLARE
    v_driver uuid;
    v_vehicle uuid;
BEGIN
    IF p_execution_id IS NOT NULL THEN
        PERFORM 1
          FROM transport.transport_executions
         WHERE id = p_execution_id
           FOR SHARE;
    END IF;
    PERFORM 1
      FROM transport.shipments
     WHERE id = p_shipment_id
       AND deleted_at IS NULL
       FOR SHARE;
    SELECT s.driver_id, s.vehicle_id
      INTO v_driver, v_vehicle
      FROM transport.shipments AS s
     WHERE s.id = p_shipment_id
       AND s.deleted_at IS NULL;
    IF v_driver IS NOT NULL THEN
        PERFORM 1 FROM transport.drivers WHERE id = v_driver FOR SHARE;
    END IF;
    IF v_vehicle IS NOT NULL THEN
        PERFORM 1 FROM transport.vehicles WHERE id = v_vehicle FOR SHARE;
    END IF;
    RETURN QUERY
    SELECT s.id, s.tenant_id, s.driver_id, s.vehicle_id, s.version, false,
           d.status::text,
           e.id, e.operating_tenant_id, e.driver_id, e.vehicle_id, e.current_revision_id,
           r.version
      FROM transport.shipments AS s
      LEFT JOIN transport.drivers AS d
        ON d.id = s.driver_id
      LEFT JOIN transport.transport_execution_active_shipments AS link
        ON link.shipment_tenant_id = s.tenant_id
       AND link.shipment_id = s.id
      LEFT JOIN transport.transport_executions AS e
        ON e.id = link.execution_id
      LEFT JOIN transport.transport_execution_revisions AS r
        ON r.id = e.current_revision_id
     WHERE s.id = p_shipment_id
       AND s.deleted_at IS NULL;
END;
$fn$;
```

That body is a design. It is not created in the product database. The driver and vehicle `FOR SHARE` steps are inside `transport.read_binding_assignment`, after the shipment share and before the decision lock. They are not placed before the execution share. No migration enables row-level security on these tables, so RLS is not this control. `TENANT_ISOLATION_IMPLEMENTED=NO`. The function owner needs `USAGE` on schema `transport`. `000103:50` grants that usage to `freight_migrations`, `company_runtime`, and `shipment_runtime`. It does not grant it to `assignment_lock_owner`. The future migration adds that usage. Without it the function cannot resolve the table names.

The function owner is granted `SELECT` on the columns the body reads:

- `transport.shipments`: `id`, `tenant_id`, `driver_id`, `vehicle_id`, `version`, `deleted_at` (`000003_create_transport_tables.up.sql:159-184`)
- `transport.drivers`: `id`, `tenant_id`, `status`, `deleted_at` (`000003:99-115`)
- `transport.vehicles`: `id`, `tenant_id`, `status` (`000003:75-93`)
- `transport.transport_execution_active_shipments`: `shipment_tenant_id`, `shipment_id`, `execution_id` (`000088_tms_transport_execution_foundation_v0_1a.up.sql:76-81`)
- `transport.transport_executions`: `id`, `operating_tenant_id`, `driver_id`, `vehicle_id`, `current_revision_id` (`000088:10-16`)
- `transport.transport_execution_revisions`: `id`, `version` (`000088:22-34`)
- `transport.transport_execution_carrier_relationship_pins`: existence for (`execution_revision_id`, `shipment_tenant_id`, `shipment_id`) (`000103_tms_relationship_pins_security_foundation_v0_1.up.sql:128-142`)

Each table that the body locks with `FOR SHARE` also needs `UPDATE` on `updated_at`, and on no other column:

- `transport.transport_executions.updated_at` (`000088:18`)
- `transport.shipments.updated_at` (`000003:181`)
- `transport.drivers.updated_at` (`000003:112`)
- `transport.vehicles.updated_at` (`000003:88`)

The joined revision, active-shipment, and pin rows are read. This lock order does not take `FOR SHARE` on those three tables, so they do not receive `UPDATE (updated_at)` for this function. `tenant_id`, `driver_id`, `vehicle_id`, `version`, and `current_revision_id` stay without `UPDATE`. There is no `owner_epoch` column to grant. `ASSIGNMENT_LOCK_PRIVILEGES=COLUMN_SELECT_PLUS_UPDATE_UPDATED_AT`. `freight_migrations` is not the owner of `transport.shipments`. The table owner or a superuser issues both the `SELECT` grants and the `UPDATE (updated_at)` grants.

`ASSIGNMENT_LOCK_PRIVILEGE_PROVEN=ISOLATED_PASS_NOT_PRODUCTION`. PostgreSQL 16.15, database `binding_privilege_proof_r1`, network `none`, no production credentials: column `SELECT` and table `SELECT` each returned `42501` for `FOR SHARE`; `assignment_lock_owner` with column `SELECT` plus `UPDATE (updated_at)` completed `FOR SHARE` on the shipment, driver, vehicle, and execution rows; the same role completed `FOR UPDATE` on the shipment; that role's `UPDATE` of `driver_id` returned `42501`; its `UPDATE` of `updated_at` changed the stored timestamp. The full `RETURN QUERY` join in the design body was not executed. The production schema was not used. Implementation stays closed until a later isolated run executes that full body and until this gate is still true for that body.

The future migration creates the function as `assignment_lock_owner`, not by a later `ALTER FUNCTION OWNER` from `freight_migrations`. `freight_migrations` receives `assignment_lock_owner` with `INHERIT FALSE, SET TRUE` only inside the uncommitted migration. `SET ROLE assignment_lock_owner` runs `CREATE FUNCTION`, so the function owner is that role. The lock owner needs `CREATE` on schema `transport` for that statement. `CREATE` is revoked before commit. `USAGE` remains. `EXECUTE` is granted to `binding_authority_fence`, and `PUBLIC` is revoked, before `RESET ROLE`. The membership is then revoked. After that revoke, `freight_migrations` cannot `SET ROLE` to `assignment_lock_owner`.

Only `assignment_lock_owner` or a superuser can replace the function body. `tracking_runtime`, `shipment_runtime`, `binding_authority_fence`, and `freight` have no membership in that role. `NOLOGIN` is not the control. A membership with `INHERIT TRUE` lets the member `UPDATE updated_at` without `SET ROLE`. A membership with `SET TRUE` lets `SET ROLE` do the same. The isolated session showed both, then revoked both. The design grants neither. After the revoke, `SET ROLE` returned `42501` and the member count for the fence and the runtime was `0`.

### Privilege model

Three models were compared against the 16.15 result. The lock order does not change under the selected model.

Option A grants `UPDATE (updated_at)` to `shipment_assignment_reader` and keeps that role as the function owner. The required privileges are column `SELECT` plus `UPDATE (updated_at)` on each locked table. The risk is the one the session measured: that role can change `updated_at` directly. `updated_at` is not `version`. A change does not by itself rewrite `driver_id`, `vehicle_id`, or `version`, and a later assignment `UPDATE` can overwrite the timestamp. It is still a direct write, so a role that exists to read would hold a write grant. Any later `INHERIT TRUE` or `SET TRUE` membership would hand that write to the member.

Option B uses a separate owner, `assignment_lock_owner`. The privileges are the same column `SELECT` list and `UPDATE (updated_at)` on the four locked tables. `binding_authority_fence` receives `EXECUTE` only. `tracking_runtime`, `shipment_runtime`, `freight`, and `PUBLIC` receive no membership and no `EXECUTE`. The function body takes `FOR SHARE` and does not assign `updated_at`. The measured reduction is that the write grant is not attached to a read role and is not held by the fence or the runtime. The measured limit is that the owner itself can still change `updated_at`, and so can a table owner, a superuser, or any future member. `INHERIT TRUE` and `SET TRUE` were enough to perform that `UPDATE` in the isolated session.

Option C would take the share inside shipment-service, in the same PostgreSQL transaction that consumes the decision. `shipment_runtime` already updates shipments, drivers, and vehicles, so this does not create a smaller privilege. An HTTP call to another service is a second transaction and does not hold the share. Putting the decision update in shipment-service moves the binding-authority write across the service boundary. That option is not selected.

Option B is the contract. Option A leaves the required write on the read role. Option C does not reduce the privilege and does not keep one database transaction without a boundary change. `RECOMMENDED_PRIVILEGE_MODEL=B_SEPARATE_LOCK_FUNCTION_OWNER`.

`DIRECT_UPDATED_AT_RISK=OPEN_FOR_LOCK_OWNER`. The open writers are `assignment_lock_owner`, the table owner, and a superuser. The fence and the runtime are outside that set only while they have no membership in `assignment_lock_owner`. A compromised function body owned by the lock role can use the same `UPDATE` grant. The body specified here does not.

There is no `owner_epoch` column. The function does not select one and does not return a numeric epoch. The consume function keeps `owner_epoch=ABSENT` and does not grant an execution target. It returns the row's own tenant, driver, vehicle, version, link, revision, revision `version`, and operating tenant. The consume function compares those values with the decision. A caller-supplied tenant that matches is not the check.

A missing `SELECT` or a missing `EXECUTE` fails the function closed. It does not fall back to the unscoped binding query.

## 5. Privileges

`FUNCTION_OWNER_PRIVILEGES=DECISION_COLUMN_SELECT_AND_CONSUME_UPDATE_ONLY`

`ASSIGNMENT_FUNCTION_OWNER_PRIVILEGES=COLUMN_SELECT_PLUS_UPDATE_UPDATED_AT`

The decision token above is the fence's grant on the decision row. It is not the assignment lock. `assignment_lock_owner` has no decision grant. Its grant is the column `SELECT` list and `UPDATE (updated_at)` in section 4.

`binding_authority_fence` needs `SELECT` on the decision columns the consume function reads. Those columns are the proposed row, not columns already stored in a table: `decision_id`, `caller_user_id`, `shipment_tenant_id`, `operating_tenant_id`, `shipment_id`, `driver_id`, `vehicle_id`, `vehicle_scope`, `execution_id`, `current_revision_id`, `shipment_version`, `revision_version`, `owner_epoch`, `relationship_required`, `provider_code`, `provider_device_id`, `issued_at`, `expires_at`, `consumed_at`, `use_status`, and `binding_id`. The grant is column `SELECT`, not `SELECT` to `tracking_runtime`. The fence also needs `UPDATE` on `consumed_at`, `use_status`, and `binding_id` only. Identity columns have no `UPDATE` grant. The trigger in section 2 still rejects a reverse transition and an identifier change even if a later grant adds `UPDATE`.

Binding `INSERT` is limited to the existing columns the function writes (`000026_add_shipment_telemetry_foundation_v0.7.0.up.sql:5-17`): `tenant_id`, `shipment_id`, `vehicle_id`, `driver_id`, `provider_code`, `provider_device_id`, `status`, `active_from`. `RETURNING id` needs `SELECT` on `id` for `binding_authority_fence`. The current table has no `decision_id` column, and this design does not add one there. The decision's `binding_id` points at `tracking.shipment_tracking_binding.id`.

| Role | Decision SELECT | Decision INSERT | Decision UPDATE | Decision DELETE | Binding INSERT |
| --- | --- | --- | --- | --- | --- |
| `binding_authority_owner` | Owner. Reads every column because it owns the table. | Only through the issue function it owns. Direct `INSERT` remains possible for the owner. | Owner. Can change `consumed_at` and identifiers, and can disable the trigger. | Owner. The design has no delete function. The owner can still delete. | Not required. The owner is not the binding writer. |
| `binding_authority_fence` | Column `SELECT` listed above, so the consume function can compare the row it locked. | None. It is not the issuer. | `consumed_at`, `use_status`, and `binding_id` only, so the one-way update can run. The trigger blocks the reverse. | None. | The binding columns listed above, so the same transaction can insert the row whose id is stored on the decision. |
| `assignment_lock_owner` | None. | None. | None. | None. | None. It owns `transport.read_binding_assignment`. Its table rights are column `SELECT` plus `UPDATE (updated_at)` on the four locked assignment tables in section 4. It can change `updated_at` on those tables. It has no `UPDATE` on `driver_id` or `version`. |
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

The future condition is the lock order in section 1, held from the share through the binding insert and the consume update, in one transaction. That order follows the command writer: execution, then shipment, then decision, then the binding insert. The R3 sentence that put the shipment lock before the execution lock is withdrawn. It is not installed. A shipment assignment that commits in another session is visible to this transaction only at the statement that acquires the share. After that share is held, the other session's `UPDATE` waits. Two service commits are still not one transaction. `DECISION_CONSUMPTION_GATE=CLOSED`.

## 8. What the revoke does not stop

The table owner and a superuser are not limited by `GRANT` and `REVOKE`. PR #376 left that bypass open. `SHARED_FREIGHT_RISK=OPEN`. `ALL_WRITERS_FENCED=NO`. This contract does not claim the owner is fenced. Ownership transfer removes the migration role as owner. It does not fence the new owner.

`ON_CONFLICT_DO_NOTHING_RACE=OPEN` until the function replaces the current insert. The design forbids that clause. The running code still has it.

## 9. Failure behavior

The caller sees conflict or not found, with the same privacy rule as the authority contract. A database error is unavailable. The function does not insert a row marked active and hope a later check corrects it. Zero rows from the consume update is a failure, not a successful replay. Replay is only the read in section 6 after a committed `CONSUMED` row.
