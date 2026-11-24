# TRACKING-BINDING-AUTHORITY-0.1G — Test Plan

These tests are specified against the option B contract. None are executed. Every row is `NOT_RUN`. Current code is not that contract. A PASS oracle describes the future behavior. It is not a result.

Audit rows are not implemented. A future record must omit coordinates, driver ids, vehicle ids, and tenant ids.

## 1. Authorized binding creation

- Setup: gateway authentication enabled. User U maps to active driver D in tenant A. Shipment S in A has `driver_id` D.
- Authoritative source: `transport.drivers` and `transport.shipments` through `GetByUserIDAndTenant` and `CanDriverAccessShipment`.
- Input: driver location for S from U's gateway session.
- Expected database state: one binding row for A, S, and D, with a shipment decision reference. No vehicle unless the shipment decision includes one.
- Expected authorization outcome: shipment half VERIFIED.
- Expected tracking effect: the location may be stored for that shipment. An execution target still needs the execution half.
- Oracle: PASS when the stored driver is D from the driver row, not from a client driver header. FAIL when the insert uses `X-Driver-ID` alone.

## 2. Forged tenant rejected

- Setup: the session tenant is A. The internal call also sends tenant B.
- Authoritative source: gateway `MustAuthContext` tenant, then the shipment row in that tenant.
- Input: location post with `X-Tenant-ID` B and a token.
- Expected database state: no trusted binding for B.
- Expected authorization outcome: denied.
- Expected tracking effect: no target in B.
- Oracle: PASS when B gains no row that target search will use. FAIL when `tracking_handler.go:101-104` stores B.

## 3. Forged actor rejected

- Setup: resolved driver is D1. The header says D2.
- Authoritative source: `driver_repository.go:82-86`.
- Input: location post with `X-Driver-ID` D2.
- Expected database state: no binding for D2.
- Expected authorization outcome: denied.
- Expected tracking effect: no target for D2.
- Oracle: PASS when D2 is not stored as a usable scope. FAIL when the header is inserted.

## 4. Shipment assignment mismatch

- Setup: driver D is active in tenant A. Shipment S is in A and its `driver_id` is someone else.
- Authoritative source: `CanDriverAccessShipment` (`driver_operations.go:271-278`).
- Input: location for S from D's user.
- Expected database state: no trusted binding.
- Expected authorization outcome: shipment half fails. Gateway already sees not found from `GetByIDAndDriver` (`shipment_repository.go:297-298`).
- Expected tracking effect: tracking is not given a decision.
- Oracle: PASS when no usable binding appears. FAIL when the internal route stores S and D together.

## 5. Shared driver, two tenants

- Setup: driver row D-A in tenant A assigned to shipment SA. Driver row D-B in tenant B assigned to shipment SB. The person has two sessions, or the same UUID has been copied onto two executions.
- Authoritative source: each shipment row and each execution row, checked separately.
- Input: a device or actor id that appears in both.
- Expected database state: two candidate rows, neither rewritten into the other tenant.
- Expected authorization outcome: two scopes when both assignments still match, otherwise only the matching ones.
- Expected tracking effect: targets stay labeled by their own tenant and execution.
- Oracle: PASS when both scopes survive as separate results. FAIL when `ORDER BY active_from DESC LIMIT 1` keeps one.

## 6. Shared vehicle, two tenants

- Setup: one vehicle UUID on execution E1 in tenant A and execution E2 in tenant B. Only A's shipment decision names that vehicle.
- Authoritative source: the shipment decision's vehicle and the execution row.
- Input: target read for the device bound in both tenants.
- Expected database state: no change that copies A's vehicle onto B.
- Expected authorization outcome: B is omitted if B's decision does not name the vehicle and B's driver half also fails.
- Expected tracking effect: E2 is not returned on the strength of the UUID.
- Oracle: PASS when the vehicle UUID alone returns no foreign execution. FAIL when `actorMatchesTarget` (`eta_ingest_service.go:311-327`) is the grant.

## 7. Revoked binding not usable

- Setup: a previously matching assignment is ended. The binding row is still `status = 'active'`, or a later writer has set `revoked`.
- Authoritative source: current shipment and driver rows.
- Input: location or target read for that device.
- Expected database state: the old row is not a grant. This test does not require a new status value.
- Expected authorization outcome: not ACTIVE.
- Expected tracking effect: no approach mark and no ETA write.
- Oracle: PASS when `status = 'active'` is insufficient. FAIL when the lookup at `tracking_repository.go:39` returns the row and approach uses it.

## 8. Expired binding not usable

- Setup: a decision whose validity interval is in the past. `active_to` may also be in the past.
- Authoritative source: the decision interval. Absence of an interval on a historical row is the same denial.
- Input: target read.
- Expected database state: unchanged.
- Expected authorization outcome: EXPIRED.
- Expected tracking effect: no target.
- Oracle: PASS when the row is omitted. FAIL when the current `WHERE status = 'active'` returns it.

## 9. Reassignment invalidates old scope

- Setup: binding decision names driver D1. The shipment or the execution now names D2.
- Authoritative source: the current `driver_id` column.
- Input: target read and a following approach or ETA write.
- Expected database state: no effect written for D1.
- Expected authorization outcome: INVALIDATED_BY_REASSIGNMENT.
- Expected tracking effect: `MarkApproachEmitted` is not called for the old pair.
- Oracle: PASS on the empty authorized set. FAIL when the stored binding driver is enough.

## 10. Tenant ownership changed

- Setup: the shipment half still matches tenant A. The execution `operating_tenant_id` is now C.
- Authoritative source: `transport.transport_executions.operating_tenant_id`.
- Input: target read for the device.
- Expected database state: no tracking state moved to C by this read.
- Expected authorization outcome: shipment half may be VERIFIED. Execution half fails. The scope is not ACTIVE.
- Expected tracking effect: no target and no ETA.
- Oracle: PASS when shipment tenant A is not treated as operating tenant C. FAIL when the binding tenant is copied onto the execution target.

## 11. Authorization source unavailable

- Setup: shipment assignment rows cannot be read.
- Authoritative source: none available.
- Input: create and target read.
- Expected database state: no new trusted binding.
- Expected authorization outcome: fail closed.
- Expected tracking effect: empty target set. No fallback to `ListActiveTargetsForActors`.
- Oracle: PASS on the empty set and the absence of a new grant. FAIL when the header path inserts anyway.

## 12. Duplicated binding creation

- Setup: an ACTIVE decision already stored for tenant A, device D, shipment S.
- Authoritative source: the same shipment decision.
- Input: the same location post again.
- Expected database state: still one active row for that tenant and device. The shipment id remains S.
- Expected authorization outcome: the repeat is the same scope, not a second grant.
- Expected tracking effect: no second target.
- Oracle: PASS when the row count for that tenant and device stays one. FAIL when a second active row is inserted for the same tenant, provider, and device.

## 13. Concurrent binding creation

- Setup: two requests in tenant A for device D, shipments S1 and S2, arriving together. Only one shipment assignment is true.
- Authoritative source: the shipment row and the row observed after the write.
- Input: both posts.
- Expected database state: the stored shipment is the one the assignment allows. The other call does not observe success for its shipment.
- Expected authorization outcome: one scope, or none if neither assignment holds.
- Expected tracking effect: the loser does not emit approach for its shipment.
- Oracle: PASS when `ON CONFLICT DO NOTHING` success without checking the winner (`tracking_repository.go:476-478`) is not treated as a grant. FAIL when the loser returns success and later reads targets.

## 14. Device linked to multiple bindings

- Setup: device D active in tenant A and tenant B, each with its own passing assignment and execution.
- Authoritative source: two independent decisions.
- Input: provider or driver ingest for D.
- Expected database state: both rows remain.
- Expected authorization outcome: two scopes.
- Expected tracking effect: approach and ETA do not emit, because more than one authorized execution remains. They also do not drop one with `LIMIT 1`.
- Oracle: PASS when the result lists both and the side effect is withheld. FAIL when `FindActiveBindingByDeviceAnyTenant` feeds a single tenant into approach.

## 15. Unauthorized protected read

- Setup: caller has the shared token, a known execution id, and a known stop id. No ACTIVE scope.
- Authoritative source: none.
- Input: `GetState`, `GetExecutionStop`, and the actor list.
- Expected database state: unchanged.
- Expected authorization outcome: denied.
- Expected tracking effect: no row and no coordinates.
- Oracle: PASS when all three return an empty or generic denial. FAIL when `GetState` (`execution_tracking_repository.go:178-179`) or the ETA predicate (`execution_tracking_repository.go:291`) returns data.

## 16. Authorized Approach target read

- Setup: one ACTIVE scope. One execution. One live stop inside the approach radius.
- Authoritative source: the shipment decision and the matching execution row.
- Input: a fresh location for that device.
- Expected database state: approach may be marked for that execution and stop only.
- Expected authorization outcome: one scope.
- Expected tracking effect: `MarkApproachEmitted` only for that pair.
- Oracle: PASS when the mark matches the authorized execution. FAIL when a second tenant's execution is marked, or when the mark happens from the unscoped list.

## 17. Authorized ETA target read

- Setup: one ACTIVE scope whose live stop equals the provider stop id.
- Authoritative source: the same pair of current rows.
- Input: ETA ingest for that device and stop.
- Expected database state: an ETA write only for that execution, tenant, and stop, and only through the later write fence.
- Expected authorization outcome: one scope.
- Expected tracking effect: no write when the stop belongs to a different scope.
- Oracle: PASS when the stop and the scope agree. FAIL when `eta_ingest_service.go:192` selects the execution by actor id and the uniqueness count alone.

## 18. Cross-tenant read denied

- Setup: scope A is ACTIVE. The caller asks for tenant B's execution or stop.
- Authoritative source: A's decision.
- Input: target list and ETA read.
- Expected database state: B unchanged.
- Expected authorization outcome: B is absent.
- Expected tracking effect: no coordinates and no ETA for B.
- Oracle: PASS on the omission. FAIL when any caller-supplied tenant B, or a matching header, returns B's row.

## 19. Shared token without tenant authority

- Setup: a caller that has only the shared secret.
- Authoritative source: none. `eta_handler.go:282-288` proves the secret.
- Input: driver location and `GetExecutionStop` with a real stop and a tenant header.
- Expected database state: no trusted binding.
- Expected authorization outcome: denied for both the create and the ETA.
- Expected tracking effect: no target and no ETA body.
- Oracle: PASS when the secret does not widen scope. FAIL when the current driver route inserts or the ETA route returns a row. Named service identity remains unresolved, so this test does not accept a substitute header.

## 20. Historical binding without proof

- Setup: a row inserted by the current statement (`tracking_repository.go:472-477`) or by the integration-test insert. It has `status = 'active'` and no decision reference.
- Authoritative source: none on the row.
- Input: target read for that device.
- Expected database state: the historical row is not rewritten by this stage.
- Expected authorization outcome: CREATED, not ACTIVE.
- Expected tracking effect: no approach and no ETA from that row.
- Oracle: PASS when target search ignores it. FAIL when `status = 'active'` is enough.

## 21. T21 — Forged vehicle

- Setup: driver D is assigned to shipment A. Vehicle V is the current `vehicle_id` of a different shipment B. The caller asks for a binding of D on A that also names V.
- Authoritative source: current `transport.shipments.vehicle_id` for A. `CanDriverAccessShipment` does not read it (`driver_operations.go:271-278`). `AssignVehicle` is the shipment writer (`shipment_service.go:274-299`) and is not called by tracking.
- Input: location post with D and `X-Vehicle-ID` V.
- Expected database state: no trusted binding that records V for A. The current insert is not this oracle.
- Expected authorization outcome: the future decision denies the claimed vehicle. The driver assignment on A can still be true and does not carry V.
- Expected tracking effect: no vehicle-scoped target for V. A driver-only scope is a different decision and is not created by attaching V.
- Oracle: PASS when V is rejected for A. FAIL when the header vehicle is stored as authorized, or when a driver match is treated as a vehicle match. `EXISTING_VEHICLE_CHECK=NOT_IMPLEMENTED_IN_THIS_PATH`. The current route would store V. That current behavior is a FAIL against this oracle and is not a pass.

## 22. T22 — Driver-only authorization

- Setup: driver D is assigned to shipment A. `vehicle_id` on A is null. The gateway sends no vehicle (`driver/handler.go:264`).
- Authoritative source: `CanDriverAccessShipment` for the driver. No vehicle row is required.
- Input: location post for A from D, with no vehicle.
- Expected database state: a future decision may record D and A with no vehicle. It must not invent one.
- Expected authorization outcome: `DRIVER_ONLY_POLICY=ALLOWED_FOR_DRIVER_MATCH_ONLY`. The driver scope is eligible. It does not authorize a vehicle id.
- Expected tracking effect: location for A is the allowed shipment effect once the decision exists. Approach and ETA may see an execution whose current `driver_id` is D. They may not select an execution by vehicle id alone.
- Oracle: PASS when a null vehicle is accepted as driver-only and a vehicle-only target is omitted. FAIL when a missing vehicle denies the driver, or when the empty vehicle matches every execution vehicle.

## 23. T23 — Vehicle reassignment

- Setup: a future decision named vehicle V1 on shipment A. `AssignVehicle` then sets A's `vehicle_id` to V2 (`shipment_repository.go:472-475`).
- Authoritative source: the current shipment vehicle column, and the current execution vehicle column when the scope is an execution target.
- Input: reuse of the stored binding that still says V1.
- Expected database state: the old row is not rewritten into a grant by this stage.
- Expected authorization outcome: `VEHICLE_REASSIGNMENT_POLICY=CLAIMED_VEHICLE_MUST_MATCH_CURRENT_ROW`. V1 fails. A new decision is required for V2.
- Expected tracking effect: no approach and no ETA from the V1 claim.
- Oracle: PASS when reuse without a new check is denied. FAIL when `status = 'active'` keeps V1 authorized. `BINDING_IS_AUTHORIZATION_PROOF=NO`.

## 24. T24 — Different execution revision

- Setup: a future decision recorded execution E, revision R1, driver D, and, if claimed, vehicle V. A successor sets `current_revision_id` to R2 (`transport_execution_successor_repository.go:172-176`) without assigning a new `vehicle_id`.
- Authoritative source: `transport.transport_executions.current_revision_id`. This column is not `owner_epoch`. `authorizeCommand` does not compare it (`transport_execution_command_repository.go:159-187`).
- Input: reuse of the decision that names R1.
- Expected database state: no tracking write from the stale decision.
- Expected authorization outcome: `EXECUTION_REVISION_POLICY=STALE_REVISION_IS_NOT_ACTIVE`. The scope is not ACTIVE until a decision is taken against R2. Owner epoch remains a separate unenforced write check.
- Expected tracking effect: no approach mark and no ETA write from R1.
- Oracle: PASS when R1 is refused after R2 is current. FAIL when unchanged driver and vehicle columns are treated as proof that the old revision is still authorized.

## 25. T25 — Direct internal insert bypass

- Setup: a caller holds the shared token and does not pass through the gateway. Shipment A is assigned to driver D. The caller sends tenant A, driver D or another driver, and a vehicle that belongs to assignment B.
- Authoritative source: none on this route. Authentication is token equality (`tracking_handler.go:97-99`). The insert stores the header values (`tracking_repository.go:472-477`).
- Input: `POST /internal/v1/tracking/driver/shipments/{shipmentId}/locations` with that vehicle header.
- Expected database state: the current path can insert the claimed vehicle. The future contract must not record it as a decision.
- Expected authorization outcome: `DIRECT_BINDING_INSERT_BYPASS=OPEN`. The existing path is not protected. The future contract requires the authoritative decision before a claimed vehicle is usable.
- Expected tracking effect: the inserted row remains `BINDING_IS_AUTHORIZATION_PROOF=NO` and is omitted from target search.
- Oracle: PASS when the report marks the current insert open and the future contract denies use of the forged vehicle. FAIL when this test is reported as a closed control, or when the token plus the header is called a vehicle grant.

## Execution record

| Check | Result |
| --- | --- |
| Tests 1-25 | `NOT_RUN` |
| Product code | unchanged |
| Migrations | none |
| Database roles | unchanged |
