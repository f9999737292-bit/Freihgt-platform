# TRACKING-BINDING-AUTHORITY-0.1G — Binding Lifecycle

The names below are design labels. They are not new database values. The table still has `active`, `inactive`, and `revoked` (`infrastructure/migrations/000026_add_shipment_telemetry_foundation_v0.7.0.up.sql:18`). `status = 'active'` is the insert literal at `services/tracking-service/internal/repository/tracking_repository.go:475`. It is not evidence that the assignment is still authorized.

Current behavior is the inventory. The lifecycle here is the proposed architecture under option B, and it is not implemented.

## 1. Labels

| Label | Meaning |
| --- | --- |
| CREATED | Tracking has a row or a request, and no shipment decision is attached. |
| VERIFIED | The current shipment assignment matches the tenant, shipment, and driver. This does not by itself authorize an execution target. |
| ACTIVE | VERIFIED, and, for an execution target, the current execution operating tenant and driver match, the active shipment link matches, and a claimed vehicle matches the current vehicle rows. Computed at use. The vehicle, link, and revision parts are proposed. |
| REVOKED | The shipment assignment or the driver row no longer grants the scope, or a later explicit revoke says so. |
| EXPIRED | A validity interval on the decision has ended. |
| INVALIDATED_BY_REASSIGNMENT | The current shipment or execution driver no longer matches, or a claimed vehicle no longer matches the current vehicle column. The vehicle half is proposed. |

A row can be `status = 'active'` in the table and still be CREATED, EXPIRED, REVOKED, or INVALIDATED_BY_REASSIGNMENT under these labels. Target search uses ACTIVE only.

## 2. Who creates

Today, tracking-service creates the row from caller claims (`tracking_repository.go:472-477`).

Proposed: tracking stores a row only with a shipment decision that names the tenant, shipment, resolved driver, optional vehicle, decision time, and a version or row identity of the assignment. The driver id in that decision comes from `GetByUserIDAndTenant` (`driver_repository.go:82-86`), not from `X-Driver-ID`. A request that arrives with only the shared token stays CREATED and is not stored as a usable scope. `BINDING_CREATION_TRUST` stays open until that writer exists.

## 3. Who activates

There is no separate activator. ACTIVE is the use-time result of the shipment assignment check plus, for targets, the execution assignment check. A successful insert does not activate a target scope.

## 4. How assignment is checked

`EXISTING_DRIVER_CHECK=VERIFIED_BY_CODE`

`EXISTING_VEHICLE_CHECK=NOT_IMPLEMENTED_IN_THIS_PATH`

`VEHICLE_CHECK=PROPOSED`

Shipment half, existing: `CanDriverAccessShipment` (`driver_operations.go:271-278`) against the current shipment row, after the driver row is active (`driver_operations_service.go:72-76`). The predicate is tenant and `driver_id`. It does not confirm a vehicle.

Execution half, existing and only inside an execution command: `authorizeCommand` (`transport_execution_command_repository.go:159-187`) compares `cmd.OperatingTenantID` with the locked operating tenant (`:160-161`) and, for a driver actor, `cmd.ActorID` with the locked `driver_id` (`:181-187`). The lock reads those two columns by execution id (`:70-75`). It does not read `vehicle_id`. Tracking does not call this function. The proposed target read must not be described as repeating a vehicle comparison that this function does not perform.

`SHIPMENT_AUTHORITY_SOURCE=DESIGNED_NOT_IMPLEMENTED`

The future use-time decision adds the checks that these functions do not perform: a claimed vehicle against the current shipment and execution vehicle columns, the active shipment link (`000088:76-81`), the current revision, and a relationship pin when that execution's business contract requires one. Owner epoch stays the PR #375 write fence and is not this comparison. The proposed read does not take the command write lock.

Carrier company is on the driver row (`000003_create_transport_tables.up.sql:102`) and on the execution (`000088:13`). `AssignDriver` compares the driver's carrier with the shipment carrier (`shipment_service.go:266-267`). `AssignVehicle` does the same for a vehicle (`shipment_service.go:294-296`). Neither comparison runs during the tracking binding insert. This stage does not add a new carrier rule.

## 5. Who revokes

No production writer sets `revoked`. Proposed revocation is the assignment check failing: driver replaced, driver not active, or shipment not visible to that driver. An explicit `status = 'revoked'` can be written later by the same shipment-driven path. Until then, a row left at `active` is still excluded from target search when the re-check fails. `REVOCATION_CONTRACT=RECHECK_CURRENT_ASSIGNMENT`.

## 6. Driver change

`REASSIGNMENT_CONTRACT=OLD_SCOPE_FAILS_RECHECK`

When `transport.shipments.driver_id` or `transport.transport_executions.driver_id` no longer equals the decision's driver, the old scope is INVALIDATED_BY_REASSIGNMENT. Approach and ETA do not use it. The stored row is not a continuing grant.

## 7. Vehicle change

`DRIVER_ONLY_POLICY=ALLOWED_FOR_DRIVER_MATCH_ONLY`

`VEHICLE_REASSIGNMENT_POLICY=CLAIMED_VEHICLE_MUST_MATCH_CURRENT_ROW`

`EXECUTION_REVISION_POLICY=STALE_REVISION_IS_NOT_ACTIVE`

The gateway path stores no vehicle (`driver/handler.go:264` passes an empty vehicle id). A vehicle on the binding is a claim today. `EXISTING_VEHICLE_CHECK=NOT_IMPLEMENTED_IN_THIS_PATH`. An `X-Vehicle-ID` value is not authorized.

A shipment may have no vehicle (`000003:169` is nullable). A driver-only binding is allowed for the driver match in section 4. It permits a location for that shipment and an execution target whose current `driver_id` matches. It does not authorize a vehicle-id match. Approach and ETA do not treat the empty vehicle as every vehicle on that execution.

`VEHICLE_CHECK=PROPOSED`. When the binding names a vehicle, the future decision confirms that vehicle on the current `transport.shipments.vehicle_id` and, for an execution target, on the current `transport.transport_executions.vehicle_id`. A vehicle that belongs to another current assignment fails. After `AssignVehicle` (`shipment_repository.go:472-475`) the old claim fails the next check. The old row is not a grant. A changed `current_revision_id` (`transport_execution_successor_repository.go:172-176`) likewise requires a new decision even when the vehicle column is unchanged. Historical rows without a decision stay unconfirmed.

## 8. Operating tenant change

The binding's shipment tenant can remain while `transport.transport_executions.operating_tenant_id` changes. The execution half then fails. The shipment half can still be VERIFIED. VERIFIED is not enough for `ListActiveTargetsForActors`. The write fence in PR #375 still rechecks owner epoch on any later effect. The binding is not that fence.

## 9. Expiry

`active_to` exists (`000026:15`) and is not filtered by the current lookups (`tracking_repository.go:39`, `execution_tracking_repository.go:374`). No production writer of `active_to` was found. Proposed: a decision without a validity interval is not a target grant. A decision past its interval is EXPIRED. Historical rows written by the current insert have no decision interval, so they are not target grants.

## 10. Concurrent conflicting bindings

The unique index allows one active row per tenant, provider, and device (`000026:28-30`). It allows the same device in many tenants. Those are separate scopes, not a conflict.

Inside one tenant, the current function returns an error when it sees another shipment (`tracking_repository.go:464-467`), then inserts with `ON CONFLICT DO NOTHING` (`:476`). A race can return success while the other shipment remains the stored row. Proposed: success requires the stored row's shipment to equal the shipment in the decision, observed after the write. A conflict does not become the caller's binding.

## 11. Historical rows without a decision

Every row created by the current insert is CREATED under these labels: it has no decision id, no user, and no version. Target reads ignore them. They are not backfilled by this stage. `HISTORICAL_BINDING_POLICY=NOT_A_GRANT`.

## 12. Gates

- Creating the decision column, the revoke writer, or the re-check function is an implementation task.
- `IMPLEMENTATION_GATE=CLOSED`.
- `TENANT_ISOLATION_IMPLEMENTED=NO`.
