# TRACKING-BINDING-AUTHORITY-0.1G — Binding Lifecycle

The names below are design labels. They are not new database values. The table still has `active`, `inactive`, and `revoked` (`infrastructure/migrations/000026_add_shipment_telemetry_foundation_v0.7.0.up.sql:18`). `status = 'active'` is the insert literal at `services/tracking-service/internal/repository/tracking_repository.go:475`. It is not evidence that the assignment is still authorized.

Current behavior is the inventory. The lifecycle here is the proposed architecture under option B, and it is not implemented.

## 1. Labels

| Label | Meaning |
| --- | --- |
| CREATED | Tracking has a row or a request, and no shipment decision is attached. |
| VERIFIED | The current shipment assignment matches the tenant, shipment, and driver. This does not by itself authorize an execution target. |
| ACTIVE | VERIFIED, and, for an execution target, the current execution owner and actor match that scope. Computed at use. |
| REVOKED | The shipment assignment or the driver row no longer grants the scope, or a later explicit revoke says so. |
| EXPIRED | A validity interval on the decision has ended. |
| INVALIDATED_BY_REASSIGNMENT | The shipment or execution driver or vehicle changed after the row was stored. |

A row can be `status = 'active'` in the table and still be CREATED, EXPIRED, REVOKED, or INVALIDATED_BY_REASSIGNMENT under these labels. Target search uses ACTIVE only.

## 2. Who creates

Today, tracking-service creates the row from caller claims (`tracking_repository.go:472-477`).

Proposed: tracking stores a row only with a shipment decision that names the tenant, shipment, resolved driver, optional vehicle, decision time, and a version or row identity of the assignment. The driver id in that decision comes from `GetByUserIDAndTenant` (`driver_repository.go:82-86`), not from `X-Driver-ID`. A request that arrives with only the shared token stays CREATED and is not stored as a usable scope. `BINDING_CREATION_TRUST` stays open until that writer exists.

## 3. Who activates

There is no separate activator. ACTIVE is the use-time result of the shipment assignment check plus, for targets, the execution assignment check. A successful insert does not activate a target scope.

## 4. How assignment is checked

Shipment half: `CanDriverAccessShipment` (`driver_operations.go:271-278`) against the current shipment row, after the driver row is active (`driver_operations_service.go:72-76`).

Execution half, only for target reads: current `operating_tenant_id` and driver or vehicle on `transport.transport_executions`. The command function that compares them today is `authorizeCommand` (`transport_execution_command_repository.go:159-187`). The proposed read repeats that comparison. It does not call the command and does not take the command's write lock.

Carrier company is on the driver row (`000003_create_transport_tables.up.sql:102`) and on the execution (`000088:13`). Neither is compared during the current insert. The proposed shipment decision includes the carrier only when shipment already treats it as part of the assignment. This stage does not add a new carrier rule.

## 5. Who revokes

No production writer sets `revoked`. Proposed revocation is the assignment check failing: driver replaced, driver not active, or shipment not visible to that driver. An explicit `status = 'revoked'` can be written later by the same shipment-driven path. Until then, a row left at `active` is still excluded from target search when the re-check fails. `REVOCATION_CONTRACT=RECHECK_CURRENT_ASSIGNMENT`.

## 6. Driver change

`REASSIGNMENT_CONTRACT=OLD_SCOPE_FAILS_RECHECK`

When `transport.shipments.driver_id` or `transport.transport_executions.driver_id` no longer equals the decision's driver, the old scope is INVALIDATED_BY_REASSIGNMENT. Approach and ETA do not use it. The stored row is not a continuing grant.

## 7. Vehicle change

The gateway path stores no vehicle (`driver/handler.go:264` passes an empty vehicle id). A vehicle on the binding is a claim today. Proposed: a vehicle is part of the scope only when the shipment decision includes the shipment's current `vehicle_id`. A later change of that column invalidates a scope that was bound to the old vehicle. A scope that was driver-only does not gain a vehicle from the caller header.

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
