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
6. Current revision. `current_revision_id` must be the revision recorded on the decision. A successor changes that column without rewriting `driver_id` or `vehicle_id` (`transport_execution_successor_repository.go:172-176`). That writer does not prove a revision change can never affect the driver assignment. The old decision is not kept. Outcome: `REAUTHORIZATION_REQUIRED` for the location right and `EXECUTION_SCOPE_DENIED` for an execution target. This is not a `HARD_DENIAL` of the driver row. A new check is required before any new effect. `DRIVER_ONLY_REVISION_POLICY=REAUTHORIZATION_REQUIRED`.
7. Execution driver. The execution `driver_id` must equal the resolved driver. `authorizeCommand` already compares the operating tenant and the driver actor for a command (`transport_execution_command_repository.go:159-161` and `:181-187`). The lock select is only those two columns (`:70-75`). The binding contract does not call that command and does not take its write lock. It repeats the comparison as a read.
8. Execution vehicle, only when a vehicle was claimed. The execution `vehicle_id` (`000088:14`, nullable) must equal the claimed vehicle. A driver-only decision does not match on `vehicle_id`.
9. Company relationship. Required only when that execution has a relationship pin (`000103_tms_relationship_pins_security_foundation_v0_1.up.sql:128`) or the business contract for the execution says a pin is required. `CanDriverAccessShipment` does not read the pin. A missing required pin yields no execution scope. A decision with no pin requirement does not fail for lack of a pin.
10. Assignment version. The decision records `transport.shipments.version` and the active revision `version`. A change of `shipment.version` before use is `REAUTHORIZATION_REQUIRED` for the location right and `EXECUTION_SCOPE_DENIED` for an execution target. The old decision grants no new effect. `SHIPMENT_VERSION_POLICY=REAUTHORIZATION_REQUIRED`.
11. Owner epoch. PR #375 requires an epoch for a new committed execution effect. The column is not in the current migration head. The decision field is `owner_epoch=ABSENT` until that column exists. `EXECUTION_TARGET_AUTHORIZATION=BLOCKED_WHILE_OWNER_EPOCH_ABSENT`. An execution target is denied unless every item in section 3.1 holds, including a confirmed epoch. The field is never filled from the caller. A driver-only location right does not require the epoch and still requires a current decision.

`EXISTING_DRIVER_CHECK=VERIFIED_BY_CODE`

## 3. Result

A successful check writes one decision row that tracking's login cannot insert. The row contains:

| Field | Source |
| --- | --- |
| `decision_id` | Generated by the checker. The caller does not choose it. The id alone is not a grant. |
| `caller_user_id` | Gateway `AuthContext.UserID` (`auth_context.go:14`). Empty for a rejected service caller. |
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
| `provider_code` | Set by the checker. For this driver path the value is `driver_mobile`. |
| `provider_device_id` | Set by the checker from the resolved driver id. Not from the caller. |
| `consumed_at` | Null until the same transaction that inserts the binding sets it. |
| `use_status` | `ISSUED` until that transaction sets `CONSUMED`. |

`decision_id` does not authorize a presenter, a device, a tenant, or a shipment. Consumption matches the whole row.

### 3.1 Execution target

`EXECUTION_TARGET_AUTHORIZATION=BLOCKED_WHILE_OWNER_EPOCH_ABSENT`

An execution target exists only when all of the following are true together:

- a current authorized driver scope
- the active shipment-to-execution link
- the current revision
- the execution driver assignment matches
- a claimed vehicle matches, when a vehicle was claimed
- the operating tenant was read from the execution row
- `owner_epoch` is a confirmed current value, not `ABSENT`
- the authority decision is unconsumed and unexpired
- use-time recheck of the binding still matches those rows

A missing epoch denies the target. A driver match does not fill that gap. Location submission is a separate right and is specified in the driver-only contract.

## 3.2 Decision table

`DECISION_TABLE_SPECIFIED=transport.binding_authority_decisions`

The name is proposed. The table is not in the migration head. No migration is created here.

| Item | Contract |
| --- | --- |
| Schema and name | `transport.binding_authority_decisions` |
| Table owner | `binding_authority_owner`, a proposed `NOLOGIN` role. It is not `freight`, `shipment_runtime`, or `tracking_runtime`. |
| Who may insert a decision | Only the issue function, `SECURITY DEFINER`, owned by `binding_authority_owner`. `shipment_runtime` may receive `EXECUTE` on that function later. It does not receive table `INSERT`. |
| Who may update | Only the consume function, owned by `binding_authority_fence`. That role is granted `UPDATE` on `consumed_at` and `use_status` only. It is not the table owner. Identity columns have no `UPDATE` grant. |
| Who may delete | Nobody in the runtime design. Expired and consumed rows stay. There is no purge function. |
| `tracking_runtime` | No `INSERT`, `UPDATE`, or `DELETE`. No `EXECUTE` on the consume function while the presenter channel in section 3.3 is unavailable. |
| `shipment_runtime` | No table DML. Future `EXECUTE` on the issue function only. |
| `freight_migrations` | May create the table in a later migration, then loses DML and `EXECUTE`. `000103:6-7` leaves `freight` as the shared login. That sentence is not a grant on this table. |
| `PUBLIC` | `REVOKE ALL`. |
| Operator and superuser sessions | Not granted. A superuser can still update the row. That bypass stays open. |

`NOLOGIN` is not the control. `000103:17`, `000103:24`, and `000103:31` already create `NOLOGIN NOINHERIT` roles, and a membership with `INHERIT` or `SET` would still confer the owner's rights. This design grants `tracking_runtime`, `shipment_runtime`, and `freight` no membership in `binding_authority_owner` or `binding_authority_fence`. `INHERIT` and `SET` are both false because there is no membership to inherit or assume.

Bound identifiers (`shipment_tenant_id`, `operating_tenant_id`, `shipment_id`, `driver_id`, `vehicle_id`, `execution_id`, `current_revision_id`, `provider_device_id`, `caller_user_id`) are written once by the issue function. The consume function does not assign them. A later `UPDATE` of those columns is a violation. The table owner can still perform that `UPDATE`. `TABLE_OWNER_DECISION_BYPASS=OPEN`.

`expires_at` is `issued_at` plus 15 minutes. `shipment_version` and `revision_version` are copied from the rows at issue time. Audit is the decision row itself, including `consumed_at`. No second audit table is created. The row is not deleted.

## 3.3 Presenter

`PRESENTER_IDENTITY_SOURCE=GATEWAY_AUTH_CONTEXT_USER_ID`

`PRESENTER_BOUND_TO_DECISION=REQUIRED_AND_UNAVAILABLE`

The presenter is `AuthContext.UserID` after gateway JWT validation (`auth_context.go:14`), the same value the driver route copies in `driver/handler.go:366-368`. The check is `authenticated_presenter == decision.caller_user_id`.

The presenter is not `X-User-ID` as read by `resolveVerifiedUser` (`services/shipment-service/internal/http/handlers/transition_context.go:15-18`), not a body field, and not a SQL argument chosen by `tracking_runtime`. Shipment-service trusts that header only because the comment at `tenant.go:13-14` says the gateway set it. Tracking's consume path does not have that proof.

PostgreSQL does not see the JWT. A function parameter that carries the user id is the caller choosing the presenter. Named service identity is unresolved, so no service can present a user it has authenticated. `BINDING_WRITE=BLOCKED_UNTIL_TRUSTED_PRESENTER`. The consume function is specified and is not granted.

## 3.4 Device

`DEVICE_IDENTITY_SOURCE=FIXED_IN_DECISION_FROM_RESOLVED_DRIVER`

`DEVICE_BOUND_TO_DECISION=YES`

Option B is the contract. At issue time the checker writes `provider_code` and `provider_device_id`. For `driver_mobile`, `provider_device_id` is the resolved driver id rendered as text. The current code derives that text from the caller-supplied driver (`tracking_repository.go:461`). This contract derives it from the driver row returned by `GetByUserIDAndTenant`. The caller does not supply the device id.

Option A, a lookup of `transport.driver_device` at write time, is not used. That table stores a push token and `revoked_at` (`000034_add_driver_task_platform_v0.2.up.sql:74-88`). The location path does not read it, and one driver can have more than one device row. A lookup at write time could choose the wrong row.

The consume function rejects a presented device that differs from `decision.provider_device_id`. A caller-supplied device UUID is not proof of possession.

Rights on the result are separate. A driver-only shipment match does not set execution or coordinate rights. See the driver-only contract. An execution target additionally requires section 3.1. `owner_epoch=ABSENT` fails that section.

## 4. Life of a decision

`DECISION_REPLAY_POLICY=IDENTICAL_REPLAY_RETURNS_PRIOR_RESULT`

A target read does not honor `decision_id` as permission. The read re-evaluates the current rows.

The first successful consume inserts the binding and sets `consumed_at` in the same transaction. A later request with the same `decision_id`, the same presenter, the same device, and the same bound identifiers returns that stored binding and does not insert another. A later request that changes any bound identifier is a conflict and does not change the binding. `ON CONFLICT DO NOTHING` is not that result.

Unused decisions expire at `expires_at`. An expired decision is denied even when the assignment would still pass. A version or revision mismatch is `REAUTHORIZATION_REQUIRED`, not a reuse of the old id. The caller requests a new check.

## 5. Denial

The caller receives one of: unauthorized, not found, conflict, unavailable, expired. Forged tenant, forged driver, forged vehicle, unknown shipment, unknown execution, and a presenter who is not `caller_user_id` use the same not-found or unauthorized body. The body contains no coordinates, driver id, vehicle id, tenant id, or decision contents.

Outcome words used across this PR:

| Word | Meaning |
| --- | --- |
| `HARD_DENIAL` | The request is refused. No new decision and no new effect. Tenant mismatch, a reassigned driver, a failed presenter check, and an unavailable authority source are in this class. |
| `DRIVER_ONLY_ALLOWED` | A new check confirmed the current driver assignment and no vehicle was claimed. This is not an execution target. |
| `EXECUTION_SCOPE_DENIED` | No execution target. Used when the epoch is absent, the link or revision does not match, or the vehicle claim fails. |
| `REAUTHORIZATION_REQUIRED` | The old decision grants nothing. A new check is required before a location effect or an execution effect. |

If the shipment or execution rows cannot be read, the result is unavailable and no decision row is written. The checker does not fall back to the caller's tenant, driver, or vehicle. That outcome is `HARD_DENIAL` for both the location right and the execution target.

## 6. What this does not approve

No API, table, role, or token is created. `transport.binding_authority_decisions` is a name in this document. The controller decides whether to accept this contract. Until that decision, `CURRENT_BINDING_TRUST=CALLER_CLAIMS` and `BINDING_IS_AUTHORIZATION_PROOF=NO`. The consume function stays ungranted. `BINDING_WRITE=BLOCKED_UNTIL_TRUSTED_PRESENTER`.
