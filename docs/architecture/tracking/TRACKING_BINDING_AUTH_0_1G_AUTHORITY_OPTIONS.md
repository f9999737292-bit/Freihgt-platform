# TRACKING-BINDING-AUTHORITY-0.1G — Authority Options

The controller has not chosen a model. This document compares three shapes and recommends one for a later decision. `IMPLEMENTATION_GATE=CLOSED`. No service is changed.

## 1. What must be true before a binding can authorize a target read

A binding used by `ListActiveTargetsForActors` needs server-owned facts, checked at use time. The facts that exist in code today are narrower than the full decision.

Shipment tenant and execution operating tenant are different columns. `000088_tms_transport_execution_foundation_v0_1a.up.sql:6-7` says shipment ownership is not moved. A shipment-tenant match is not an execution-owner match.

`X-Tenant-ID`, `X-Driver-ID`, the shared token, a driver UUID, a vehicle UUID, and a stored binding are not substitutes for those rows.

`CURRENT_BINDING_TRUST=CALLER_CLAIMS`

`BINDING_IS_AUTHORIZATION_PROOF=NO`

`DIRECT_BINDING_INSERT_BYPASS=OPEN`

`ON_CONFLICT_DO_NOTHING_RACE=OPEN`

`SHIPMENT_AUTHORITY_SOURCE=DESIGNED_NOT_IMPLEMENTED`

`J_DML_001=NOT_CLOSED`

The insert at `services/tracking-service/internal/repository/tracking_repository.go:472-477` stores the caller values. It does not call `CanDriverAccessShipment` or `authorizeCommand`.

## 1.1 J-TBA-001 — checks that exist, and checks that do not

`EXISTING_DRIVER_CHECK=VERIFIED_BY_CODE`

`EXISTING_VEHICLE_CHECK=NOT_IMPLEMENTED_IN_THIS_PATH`

`VEHICLE_CHECK=PROPOSED`

| Check | Status | Evidence |
| --- | --- | --- |
| Driver identity for a shipment user | Existing on the shipment driver route | `GetByUserIDAndTenant` selects `transport.drivers` by `user_id` and `tenant_id` (`services/shipment-service/internal/repository/driver_repository.go:82-86`). |
| Driver assignment on a shipment | Existing on that same route | `CanDriverAccessShipment` (`services/shipment-service/internal/domain/driver_operations.go:271-278`) requires `shipment.TenantID == tenantID` and `shipment.DriverID == driverID`. It does not read `vehicle_id`. |
| Operating tenant on an execution command | Existing for that command | `authorizeCommand` (`services/shipment-service/internal/repository/transport_execution_command_repository.go:159-161`) returns tenant denied when `cmd.OperatingTenantID` differs from the locked execution tenant. |
| Driver actor on that command | Existing when the actor kind is driver | The same function (`:181-187`) requires the locked `driver_id` to equal `cmd.ActorID`. The locked columns are `operating_tenant_id, driver_id` (`:70-75`). `vehicle_id` is not selected and is not a parameter. |
| Vehicle assignment | Not implemented on the binding path or in those two functions | `VEHICLE_CHECK=PROPOSED`. See section 1.2. |
| Shipment-to-execution linkage | Not implemented on this path | `transport.transport_executions` has no `shipment_id` (`000088:4-7`). The active link is `transport.transport_execution_active_shipments` (`000088:76-81`). Neither cited function reads it. |
| Owner epoch and current assignment at reuse | Specified, not enforced | PR #375. `current_revision_id` is a different column and is not an owner epoch. |

One function does not check driver and vehicle together. `authorizeCommand` checks the operating tenant and, for a driver actor, the driver. `CanDriverAccessShipment` checks the shipment tenant and the shipment driver.

## 1.2 Vehicle authority — proposed

`VEHICLE_CHECK_SOURCE=CURRENT_SHIPMENT_AND_EXECUTION_ROWS_WHEN_CLAIMED`

`transport.shipments.vehicle_id` is nullable (`infrastructure/migrations/000003_create_transport_tables.up.sql:169`). `transport.transport_executions.vehicle_id` is nullable (`000088:14`). A shipment is not required to have a vehicle.

Shipment assignment of a vehicle is a separate operation from driver assignment. `AssignVehicle` (`services/shipment-service/internal/service/shipment_service.go:274-299`) loads the vehicle in the shipment tenant and requires the vehicle's `carrier_company_id` to match the shipment's carrier. The update writes `vehicle_id` (`services/shipment-service/internal/repository/shipment_repository.go:472-475`). It does not compare the vehicle with the shipment driver, and it does not read the execution.

Execution `vehicle_id` is written by the projection insert (`services/shipment-service/internal/repository/transport_execution_repository.go:294-300`). Production updates of `transport.transport_executions` found in shipment-service change `current_revision_id` (`transport_execution_repository.go:92-96`, `transport_execution_successor_repository.go:172-176`) or `event_seq` (`transport_execution_command_repository.go:1225-1227`). They do not assign a new `vehicle_id`.

`X-Vehicle-ID` on the tracking route (`services/tracking-service/internal/http/handlers/tracking_handler.go:116-123`) is not an authorized vehicle. The gateway driver location call passes an empty vehicle (`services/api-gateway/internal/driver/handler.go:264`).

`DRIVER_ONLY_POLICY=ALLOWED_FOR_DRIVER_MATCH_ONLY`

When the binding names no vehicle, a future decision may still authorize the driver. That scope allows a location for the authorized shipment and an execution target whose current `driver_id` equals that driver. It does not add the execution's `vehicle_id` as a match key. Approach and ETA must not accept the scope through a vehicle-only comparison. A shipment that also has a vehicle does not, by that fact, reject the driver-only scope.

When the binding names a vehicle, the future decision must show that same vehicle on the current `transport.shipments.vehicle_id` for that tenant and shipment. For an execution target, the same vehicle must also be the current `transport.transport_executions.vehicle_id` of the linked execution. A vehicle that is current on a different shipment or execution is not a match. A null shipment vehicle denies a claimed vehicle.

`VEHICLE_REASSIGNMENT_POLICY=CLAIMED_VEHICLE_MUST_MATCH_CURRENT_ROW`

A later `AssignVehicle` changes the shipment column. The old claimed vehicle fails the next check. The stored binding is not reused as proof. An execution `vehicle_id` that still holds the old value is still the execution column that must match a claimed vehicle. This stage does not assert that the two columns move together.

`EXECUTION_REVISION_POLICY=STALE_REVISION_IS_NOT_ACTIVE`

`current_revision_id` moves without rewriting `driver_id` or `vehicle_id` (`transport_execution_successor_repository.go:172-176`). A future decision that recorded a revision id is not ACTIVE when `current_revision_id` has changed. Use requires a new decision against the current revision. `authorizeCommand` does not compare the revision. The revision read at `transport_execution_command_repository.go:86-89` happens after the tenant and driver check and is not a vehicle check. Owner epoch remains a separate, unenforced write fence.

`SHIPMENT_EXECUTION_LINK=PROPOSED`

The future execution half must find `transport.transport_execution_active_shipments` for the shipment tenant and shipment id (`000088:76-81`) and then apply the driver check, and the vehicle check when a vehicle is claimed, on that execution. Historical participant rows are not that link. The comment at `000088:74-75` says the active slot is removed when the shipment leaves the route.

A future shipment-owned decision, still `DESIGNED_NOT_IMPLEMENTED`, must re-check at every use:

- tenant
- shipment
- driver
- vehicle, when the binding claims one
- the current execution assignment, including the active shipment link and the current revision
- a company relationship pin when the business contract for that execution requires one
- that the decision is still current

Historical binding rows are not promoted into that decision. `HISTORICAL_BINDING_POLICY=NOT_A_GRANT`.

## 2. Option A — Gateway authorized binding

Source of truth: the gateway auth context plus a shipment assignment the gateway has just observed.

Trust boundary: `MustAuthContext` (`services/api-gateway/internal/driver/handler.go:360-364`) and the shipment HTTP status from `GetShipment` (`handler.go:240-245`).

Tenant substitution: on the gateway route, the tenant written downstream is the auth-context tenant, not the client header. Tracking still accepts any header on the internal route (`tracking_handler.go:97-104`).

Replay: the gateway has no binding nonce. A repeated location post repeats the headers.

Revocation: the gateway does not write `revoked`. The next request can fail `GetShipment` after reassignment, but an already stored row stays `active` because nothing updates it.

Owner epoch: the gateway does not see `owner_epoch`. PR #375 remains the write fence.

Cross-tenant: the gateway handles one authenticated tenant per request. It does not define how two bindings for one device are read together. `FindActiveBindingByDeviceAnyTenant` still applies `LIMIT 1` (`tracking_repository.go:51-53`).

Service changes: tracking would have to reject any create that did not come from the gateway, and the gateway would have to pass the shipment decision rather than only headers. The internal route would stop accepting a raw tenant and driver.

Transaction limit: the gateway's shipment read and tracking's insert are two HTTP calls. Assignment can change between them.

Unavailable dependency: the gateway already stops when shipment returns 404 or a status at or above 400 (`driver/handler.go:240-245`). A direct tracking caller does not.

Assessment: necessary as the user-entry boundary, not sufficient as the assignment owner. The gateway discards the shipment body. It does not own `transport.shipments` or `transport.transport_executions`.

## 3. Option B — Shipment-owned binding authority

Source of truth: shipment-service's current rows. Driver identity is `transport.drivers` for the gateway user. The existing shipment check is the driver assignment in section 1.1. Execution operating tenant and execution driver are what `authorizeCommand` compares for a driver command. Vehicle match, the active shipment link, the current revision, and owner epoch are proposed additions. They are not that function. A relationship pin, when the business contract for the target execution requires one, stays shipment-owned (`000103_tms_relationship_pins_security_foundation_v0_1.up.sql:128`). Tracking does not consult that pin today. The pin is not required for every binding.

Trust boundary: those rows, read at creation and again at use. The gateway remains the component that turns a JWT into a user id and tenant. Tracking does not accept `X-Driver-ID` as the driver.

Tenant substitution: the tenant used for the driver lookup is the gateway tenant. A header on the tracking route cannot select a different tenant for a trusted row, because a trusted row is written only from the shipment decision.

Replay: authority is the current row, not an old request. After reassignment the same body no longer matches. No new token type is required for that property.

Revocation: a shipment whose `driver_id` no longer matches, or a driver row that is not active, is not a usable scope. The literal `status = 'active'` is not the revoke signal. There is still no production writer of `revoked`.

Owner epoch: a binding decision does not advance the epoch and does not authorize a write. PR #375 order stays on the write. An operating-tenant change fails the execution half of the re-read.

Cross-tenant: each shipment row and each execution row is its own scope. The same person can have one driver row per tenant because `transport.drivers` is keyed by id and filtered by `tenant_id` (`driver_repository.go:86`). Copied UUIDs on executions are not a grant, because `000088:15` has no foreign key.

Service changes, later and not approved here:

- Tracking stops inserting from caller headers.
- The create path asks shipment, or reads the same committed assignment rows, and stores a reference to that decision.
- Target reads re-check the current rows inside the protected read owner from PR #376. `tracking_runtime` still has no table `SELECT`.
- No new service and no new credential.

Transaction limit: shipment-service and tracking-service do not share a transaction. PR #375 recorded that two sessions are not one fence. A same-database read sees the latest commit. The insert is not the grant. The use-time read is. If that read cannot see the assignment, the result is empty.

Unavailable dependency: assignment tables unreadable means no new trusted binding and no target. The path must not fall back to headers or to `status = 'active'`.

Assessment: this is the smallest change that uses an owner the repository already has.

## 4. Option C — Dedicated binding authority

Source of truth: a new component that issues a proof containing tenant, execution, actor, expiry, and version.

Trust boundary: that component. It is not in the tree.

Tenant substitution and replay depend on a proof format that does not exist. Revocation needs a new status source. Owner epoch still belongs to PR #375, so the new component would have to read shipment anyway. Cross-tenant behavior would be re-specified there.

Service changes: a new issuer, a new verification path, and a second place that can disagree with `transport.shipments` and `transport.transport_executions`.

Transaction limit: the same split as option B, plus another hop.

Unavailable dependency: the new component down means fail closed, in addition to shipment being down when the issuer itself depends on shipment.

Assessment: a signed copy of a shipment decision is still option B. A separate owner of assignment would duplicate shipment-service. That is not the smallest architecture change.

## 5. Recommendation

`RECOMMENDED_AUTHORITY_MODEL=B_SHIPMENT_OWNED_NOT_APPROVED`

Option B is the recommendation because shipment-service already resolves the driver and checks the shipment driver assignment, and the execution command path already compares the operating tenant and the driver actor. Vehicle, shipment-to-execution linkage, current revision, and owner epoch stay proposed. Option A remains the required user-entry boundary and does not own those rows. Option C adds an issuer the repository does not have.

The recommendation is not an implementation approval. The controller decides. Until that decision, tracking continues to store caller claims, and those rows are not authority for `ListActiveTargetsForActors`.

## 6. Unproven

- No live check showed that a given shipment tenant equals an execution operating tenant.
- No pin-revocation test was run.
- The column grants for a future read function are not designed as SQL here.
