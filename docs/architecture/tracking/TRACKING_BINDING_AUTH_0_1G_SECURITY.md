# TRACKING-BINDING-AUTHORITY-0.1G — Security Boundaries

Each case names the evidence that would justify a denial, the denial the proposed option B requires, and the effect on tracking. Option B is not implemented. Current code does not perform these denials unless the case says so. No diagnostic text may include coordinates, a driver id, a vehicle id, or a tenant id.

`J-DML-001` stays `NOT_CLOSED`.

## 1. Forged X-Tenant-ID

Evidence: the header is parsed and stored (`tracking_handler.go:101-104`, `tracking_repository.go:477`). Gateway auth context is a different value (`driver/handler.go:360-367`) and is not re-checked inside tracking.

Denial: no trusted binding is stored. Target search returns an empty set.

Tracking effect: no approach mark and no ETA write from that request. A pre-existing row is not promoted by the forged header.

## 2. Forged driver ID

Evidence: `X-Driver-ID` is parsed at `tracking_handler.go:111-114`. Shipment resolves the driver from user and tenant at `driver_repository.go:82-86`.

Denial: the header driver is not the resolved driver, so no trusted binding is stored and no scope matches.

Tracking effect: no target for the forged driver. The shipment assignment row is unchanged.

## 3. Forged vehicle ID

Evidence: `X-Vehicle-ID` is optional at `tracking_handler.go:116-123`. The insert does not compare it with `transport.shipments.vehicle_id`. The gateway sends no vehicle (`driver/handler.go:264`).

Denial: a vehicle that the shipment decision does not include is not part of a scope. Target search does not use it.

Tracking effect: a driver-only scope can still be evaluated when the driver assignment matches. The forged vehicle adds no execution.

## 4. Forged execution ID

Evidence: `GetState` reads by execution id only (`execution_tracking_repository.go:178-179`). Binding creation does not take an execution id.

Denial: `GetState` stays blocked. An execution id in a location body is not a scope.

Tracking effect: no coordinates are returned. Approach and ETA do not select that execution unless the current execution row matches an ACTIVE scope.

## 5. Binding of another tenant

Evidence: the active unique index is per tenant (`000026:28-30`). `FindActiveBindingByDeviceAnyTenant` can return another tenant's row (`tracking_repository.go:51-53`).

Denial: tenant A's decision does not make tenant B's row ACTIVE.

Tracking effect: B's state is omitted. A's scope is evaluated on its own.

## 6. Shared device

Evidence: two active rows for one device are allowed by the unique index. The any-tenant lookup keeps one (`tracking_repository.go:51-53`).

Denial: `LIMIT 1` is not used. Each row needs its own ACTIVE check.

Tracking effect: zero, one, or two scopes, each tied to its own tenant. Approach and ETA do not pick the newest row.

## 7. Shared actor

Evidence: execution `driver_id` and `vehicle_id` have no foreign key (`000088:14-15`). `actorMatchesTarget` compares UUIDs only (`eta_ingest_service.go:311-327`).

Denial: the UUID is not a grant. Each execution is included only with its own ACTIVE scope.

Tracking effect: the unscoped actor query is not the result set.

## 8. Stale assignment

Evidence: the insert does not re-read `transport.shipments` or `transport.transport_executions`.

Denial: at use time the current driver or vehicle disagrees, so the label is INVALIDATED_BY_REASSIGNMENT.

Tracking effect: no approach mark and no ETA write for that pair.

## 9. Revoked binding

Evidence: the status check allows `revoked` (`000026:18`). No production `UPDATE` sets it. Lookups require `status = 'active'` (`tracking_repository.go:39`).

Denial: a row whose assignment check fails is unused even if `status` is still `active`. A row already set to `revoked` by a future writer is also unused.

Tracking effect: that row contributes no target. Current code still uses an `active` row after the assignment has ended. That gap is why the re-check is required.

## 10. Expired binding

Evidence: `active_to` is not in the lookup predicates (`tracking_repository.go:39`, `execution_tracking_repository.go:374`).

Denial: a missing or elapsed validity interval is not a target grant.

Tracking effect: no target from that row. Current code would still return it while `status = 'active'`.

## 11. Concurrent reassignment

Evidence: shipment read on the gateway and tracking insert are separate calls (`driver/handler.go:240-264`). `ON CONFLICT DO NOTHING` does not prove which shipment was stored (`tracking_repository.go:476-478`).

Denial: the use-time read sees the committed assignment. The loser of the race is not ACTIVE. A later write still takes the execution lock from PR #375.

Tracking effect: no effect is committed from the stale scope. The binding insert is not a fence.

## 12. Owner epoch transition

Evidence: the binding table has no epoch column. PR #375 requires the write order execution lock, epoch check, inbox, state effect, commit. The epoch is specified and not enforced.

Denial: an older scope does not authorize a write after a newer epoch is stored. The read result is not cached as permission.

Tracking effect: the write aborts. This remains `SPECIFIED_NOT_ENFORCED` until the fence exists.

## 13. Company relationship revoked

Evidence: pin storage exists (`000103_tms_relationship_pins_security_foundation_v0_1.up.sql:128`). `CanDriverAccessShipment` does not read it (`driver_operations.go:271-278`). Tracking does not read it.

Denial: when a target execution has a pin, shipment's current decision must include that the pin is still valid. A revoked pin means that execution scope is not ACTIVE. This check is not wired. The design must not claim the current shipment call already does it.

Tracking effect: after the check exists, no target for that execution. Until then the outcome is `RELATIONSHIP_CHECK=NOT_WIRED`.

## 14. Internal token compromised

Evidence: token equality is the only tracking authentication on the driver route (`tracking_handler.go:97-99`). The same secret is shared with other clients (`cmd/server/main.go:71-83`).

Denial: possession of the secret does not mint a shipment decision. The compromised caller can still reach the current insert. That is the open hole. The proposed writer refuses to store a usable scope from the secret alone.

Tracking effect: no new trusted binding and no target from the forged call. Existing historical rows stay non-grants. A database login that can write the tables directly is a separate open risk from PR #376 and is not closed by this denial.

## 15. Unauthorized gateway bypass

Evidence: the internal route is registered on tracking-service (`router.go:69`) and does not call `MustAuthContext`.

Denial: a caller that reaches tracking without the gateway user and the shipment decision does not create a trusted binding.

Tracking effect: the location may be rejected. It must not create an ACTIVE scope. Provider-secret ingest (`tracking_handler.go:138-144`) only looks up a binding; it does not become a create path.

## 16. Service identity unavailable

Evidence: stop ETA has the shared token and no service name (`eta_handler.go:282-288`). `NAMED_SERVICE_IDENTITY_DEPENDENCY=UNRESOLVED`.

Denial: ETA returns no row. Target reads return an empty set when the assignment rows cannot be read.

Tracking effect: fail closed. No fallback to the header tenant or the unscoped actor query.

## 17. Historical unverified binding

Evidence: the current insert stores no decision id, user, or version (`tracking_repository.go:472-477`).

Denial: those rows stay CREATED under the lifecycle labels and are omitted from target search.

Tracking effect: approach and ETA do not use them until a new shipment decision exists. This stage does not delete or rewrite them.

## 18. Privileged database login

Evidence: PR #376 left table-owner and superuser `SELECT` outside `GRANT` and `REVOKE`. `000105` does not create the designed tracking roles.

Denial: the binding contract cannot stop that session.

Tracking effect: `SHARED_FREIGHT_RISK=OPEN`. `ALL_WRITERS_FENCED=NO`. `TENANT_ISOLATION_IMPLEMENTED=NO`.
