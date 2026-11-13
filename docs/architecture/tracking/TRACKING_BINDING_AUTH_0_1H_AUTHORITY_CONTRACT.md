# TRACKING-BINDING-AUTHORITY-DECISION-0.1H — Authority Contract

This is the contract offered to the controller. It is not implemented. `IMPLEMENTATION_GATE=CLOSED`. `J_DML_001=NOT_CLOSED`.

Baseline: `origin/main` is `3f306fc407aeda4bdc652d825954292f4a54fad6`. PR #378 remains Draft at `03d0d22937a847abd7eb164aebf5962ec651c67d`. A caller-supplied tenant is not a grant.

`SHIPMENT_AUTHORITY_CONTRACT=DESIGNED_FOR_CONTROLLER_DECISION`

## 1. Who may ask

Two caller classes exist in the tree. They are not interchangeable.

User caller. The gateway copies `UserID` and `TenantID` from the verified auth context (`services/api-gateway/internal/http/middleware/auth_context.go:13-16`) on the driver route (`services/api-gateway/internal/driver/handler.go:360-364`). Shipment resolves that user to a driver with `GetByUserIDAndTenant` (`services/shipment-service/internal/repository/driver_repository.go:82-86`). Tracking's `X-Tenant-ID` parser (`services/tracking-service/internal/http/handlers/tracking_handler.go:249-254`) is not this identity.

Service caller. `X-Internal-Service-Token` proves possession of a shared secret (`packages/shared-go/internalauth/auth.go:9` and `:16-25`). It does not name a service, a tenant, or an operation. No service is authorized to receive a binding decision. Named service identity is a separate contract and is unresolved.

## 2. Check, in order

The checker runs inside shipment-service against current rows. Tracking does not perform it. A failure at any step stops the decision. Later steps are not used to explain the failure.

1. Caller identity. A user caller must be the gateway user. A service caller is rejected until a named identity with an allowlist exists.
2. Driver row. `transport.drivers` for that user and the gateway tenant is present, not deleted, and active (`driver_operations_service.go:64-76`).
3. Shipment assignment. `CanDriverAccessShipment` (`driver_operations.go:271-278`) requires the shipment tenant and `shipment.driver_id`. This is the existing driver check. It does not read `vehicle_id`.
4. Claimed vehicle. If the caller names a vehicle, `transport.shipments.vehicle_id` for that shipment must equal it. `transport.shipments.vehicle_id` is nullable (`000003_create_transport_tables.up.sql:169`). A different shipment's vehicle is not a match. `AssignVehicle` (`shipment_service.go:274-299`) is the shipment writer and is not consulted by tracking today. If the caller names no vehicle, the decision is driver-only. `EXISTING_VEHICLE_CHECK=NOT_IMPLEMENTED_IN_THIS_PATH`. `VEHICLE_CHECK=PROPOSED`.
5. Execution link. `transport.transport_execution_active_shipments` maps `(shipment_tenant_id, shipment_id)` to one `execution_id` (`000088_tms_transport_execution_foundation_v0_1a.up.sql:76-81`). The execution row's `operating_tenant_id` is the operating tenant of the decision. It is read from the row. It is not taken from the caller. `000088:6-7` says shipment ownership is not moved, so the shipment tenant and the operating tenant may differ. Both are recorded. Absence of an active link means the decision has no execution scope.
6. Current revision. `current_revision_id` must be the revision recorded on the decision. A successor changes that column without rewriting `driver_id` or `vehicle_id` (`transport_execution_successor_repository.go:172-176`). A stale revision yields no execution scope.
7. Execution driver. The execution `driver_id` must equal the resolved driver. `authorizeCommand` already compares the operating tenant and the driver actor for a command (`transport_execution_command_repository.go:159-161` and `:181-187`). The lock select is only those two columns (`:70-75`). The binding contract does not call that command and does not take its write lock. It repeats the comparison as a read.
8. Execution vehicle, only when a vehicle was claimed. The execution `vehicle_id` (`000088:14`, nullable) must equal the claimed vehicle. A driver-only decision does not match on `vehicle_id`.
9. Company relationship. Required only when that execution has a relationship pin (`000103_tms_relationship_pins_security_foundation_v0_1.up.sql:128`) or the business contract for the execution says a pin is required. `CanDriverAccessShipment` does not read the pin. A missing required pin yields no execution scope. A decision with no pin requirement does not fail for lack of a pin.
10. Assignment version. The decision records `transport.shipments.version` and the active revision `version`. Either value changing before use invalidates the decision.
11. Owner epoch. PR #375 requires an epoch for a new committed execution effect. The column is not in the current migration head. The decision field is `owner_epoch=ABSENT` until that column exists. Any right that depends on the epoch is denied while it is absent. The field is never filled from the caller.

`EXISTING_DRIVER_CHECK=VERIFIED_BY_CODE`

## 3. Result

A successful check writes one decision row that tracking's login cannot insert. The row contains:

| Field | Source |
| --- | --- |
| `decision_id` | Generated by the checker. The caller does not choose it. |
| `caller_user_id` | Gateway auth context. Empty for a rejected service caller. |
| `shipment_tenant_id` | Shipment row. |
| `shipment_id` | Shipment row. |
| `driver_id` | Driver row. |
| `vehicle_id` | Current shipment vehicle, or null when the decision is driver-only. |
| `vehicle_scope` | `DRIVER_ONLY` or `VEHICLE_CLAIMED`. |
| `execution_id` | Active shipment link, or null when there is no link. |
| `operating_tenant_id` | Execution row, or null when there is no execution scope. |
| `current_revision_id` | Execution row, or null. |
| `shipment_version` | Shipment row. |
| `revision_version` | Active revision, or null. |
| `owner_epoch` | `ABSENT` until the column exists. |
| `relationship_required` | Whether this execution required a pin. |
| `issued_at` | Checker clock. |
| `expires_at` | `issued_at` plus 15 minutes. The same duration exists on the integration token (`packages/shared-go/integrationauth/claims.go:13`). This contract does not reuse that token. |
| `consumed_at` | Null until the binding write consumes the decision. |

Rights on the result are separate. A driver-only shipment match does not set execution or coordinate rights. See the driver-only contract.

## 4. Life of a decision

The decision is single-use for a binding write. A target read does not honor `decision_id` as permission. The read re-evaluates the current rows.

Unused decisions expire at `expires_at`. An expired, consumed, or version-mismatched decision is denied even when the assignment would still pass. The caller requests a new check.

## 5. Denial

The caller receives one of: unauthorized, not found, conflict, unavailable, expired. Forged tenant, forged driver, forged vehicle, unknown shipment, and unknown execution use the same not-found body. The body contains no coordinates, driver id, vehicle id, tenant id, or decision contents.

If the shipment or execution rows cannot be read, the result is unavailable and no decision row is written. The checker does not fall back to the caller's tenant, driver, or vehicle.

## 6. What this does not approve

No API, table, role, or token is created. The controller decides whether to accept this contract. Until that decision, `CURRENT_BINDING_TRUST=CALLER_CLAIMS` and `BINDING_IS_AUTHORIZATION_PROOF=NO`.
