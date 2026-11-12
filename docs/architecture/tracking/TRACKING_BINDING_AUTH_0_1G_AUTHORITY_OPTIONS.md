# TRACKING-BINDING-AUTHORITY-0.1G — Authority Options

The controller has not chosen a model. This document compares three shapes and recommends one for a later decision. `IMPLEMENTATION_GATE=CLOSED`. No service is changed.

## 1. What must be true before a binding can authorize a target read

A binding used by `ListActiveTargetsForActors` needs two server-owned facts, checked at use time:

1. Shipment assignment: `transport.shipments` for that tenant has this `driver_id` on this shipment, and `transport.drivers` for that tenant and user is active. The existing check is `CanDriverAccessShipment` (`services/shipment-service/internal/domain/driver_operations.go:271-278`) after `GetByUserIDAndTenant` (`services/shipment-service/internal/repository/driver_repository.go:82-86`).
2. Execution assignment, when the read is an execution target: `transport.transport_executions.operating_tenant_id` and `driver_id` or `vehicle_id` (`000088_tms_transport_execution_foundation_v0_1a.up.sql:12-15`) match that scope. Shipment command code already compares those on a locked row (`services/shipment-service/internal/repository/transport_execution_command_repository.go:159-187`). Tracking does not call it.

Shipment tenant and execution operating tenant are different columns. `000088:6-7` says shipment ownership is not moved. A shipment-tenant match is not an execution-owner match.

`X-Tenant-ID`, `X-Driver-ID`, the shared token, a driver UUID, a vehicle UUID, and a stored binding are not substitutes for those rows.

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

Source of truth: shipment-service's current rows. Driver identity is `transport.drivers` for the gateway user. Shipment assignment is `transport.shipments`. Execution assignment is `transport.transport_executions`. A relationship pin, when the target execution has one, stays shipment-owned (`000103_tms_relationship_pins_security_foundation_v0_1.up.sql:128`). Tracking does not consult that pin today.

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

Option B is the recommendation because shipment-service already resolves the driver, checks the shipment assignment, and locks the execution assignment. Option A remains the required user-entry boundary and does not own those rows. Option C adds an issuer the repository does not have.

The recommendation is not an implementation approval. The controller decides. Until that decision, tracking continues to store caller claims, and those rows are not authority for `ListActiveTargetsForActors`.

## 6. Unproven

- No live check showed that a given shipment tenant equals an execution operating tenant.
- No pin-revocation test was run.
- The column grants for a future read function are not designed as SQL here.
