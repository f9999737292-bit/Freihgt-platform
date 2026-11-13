# TRACKING-BINDING-AUTHORITY-DECISION-0.1H — Test Plan

These tests are specified. None are executed. Every row is `NOT_RUN`. The oracle is this contract. The current insert and the current target query fail any oracle that requires a decision. That failure is expected and is not a pass.

`CROSS_TENANT_TEST_PLAN=SPECIFIED_NOT_RUN`

Errors contain no coordinates, driver id, vehicle id, or tenant id.

## 1. Valid shipment-owned decision

- Setup: gateway user U resolves to active driver D. Shipment S in tenant A has `driver_id` D and a version. An active link points at execution E on revision R. E's driver is D. No vehicle is claimed.
- Input: the checker is asked for U, S, and no vehicle.
- Oracle: PASS when one decision row records A, S, D, `DRIVER_ONLY`, E, E's operating tenant, and R, and `owner_epoch=ABSENT` denies execution-target rights while the location right is present. FAIL when the decision copies a header tenant that was not on the shipment row, or when `ABSENT` epoch still grants a target read.

## 2. Forged tenant

- Setup: the gateway tenant is A. The caller also sends tenant B.
- Input: a check and a binding write for S.
- Oracle: PASS when the decision tenant is the shipment row's tenant and a header B creates no decision and no binding. FAIL when `tracking_handler.go:101-104` stores B as authority.

## 3. Forged vehicle

- Setup: D is assigned to shipment A. Vehicle V is current on shipment B.
- Input: a check for A that claims V.
- Oracle: PASS when the decision is denied and no binding records V for A. FAIL when the driver match authorizes V, or when `X-Vehicle-ID` is stored as a grant. `EXISTING_VEHICLE_CHECK=NOT_IMPLEMENTED_IN_THIS_PATH`.

## 4. Driver-only authorization

- Setup: S has driver D and a null `vehicle_id`. Execution E has driver D and some other vehicle.
- Input: a decision with no claimed vehicle, then a target read.
- Oracle: PASS when the location right exists and the target is kept only by `driver_id`, with E's vehicle ignored as a key. FAIL when the null vehicle matches every vehicle, or when the missing vehicle denies the driver assignment.

## 5. Vehicle reassignment

- Setup: a decision claimed V1. `AssignVehicle` then sets the shipment to V2 (`shipment_repository.go:472-475`).
- Input: reuse of that decision for a write, and a target read.
- Oracle: PASS when the write inserts nothing and the target set omits V1. FAIL when the stored binding still authorizes V1. `VEHICLE_REASSIGNMENT_POLICY=CLAIMED_VEHICLE_MUST_MATCH_CURRENT_ROW`.

## 6. Execution revision change

- Setup: the decision records revision R1. A successor sets `current_revision_id` to R2 (`transport_execution_successor_repository.go:172-176`). Driver and vehicle columns stay as they were.
- Input: a target read and a binding write that still cites R1.
- Oracle: PASS when execution scope is denied until a new decision names R2. FAIL when unchanged driver and vehicle columns keep R1 active. `EXECUTION_REVISION_POLICY=STALE_REVISION_IS_NOT_ACTIVE`.

## 7. Direct binding insert bypass

- Setup: a shared-token caller sends a tenant, driver, and vehicle with no `decision_id`.
- Input: the internal location route and a direct `INSERT`.
- Oracle: PASS when the future function and the `REVOKE` write nothing, and the report still marks the current path `DIRECT_BINDING_INSERT_BYPASS=OPEN`. FAIL when the current insert is described as already closed, or when the token is accepted as a decision.

## 8. Duplicate binding race

- Setup: two writes for the same tenant and device. One decision is for shipment S1. The other is for S2. Only S1's assignment is current.
- Input: both calls, overlapping.
- Oracle: PASS when S2 observes conflict and the stored shipment is S1. FAIL when `ON CONFLICT DO NOTHING` (`tracking_repository.go:476`) lets S2 observe success. `DUPLICATE_BINDING_POLICY=CONFLICT_UNLESS_SAME_DECISION_SHIPMENT`.

## 9. Revoked assignment

- Setup: a decision was valid. The shipment `driver_id` then changes, or the driver row is no longer active.
- Input: the unused decision is presented for a write, and a target read runs.
- Oracle: PASS when both the write and the target omit the scope. FAIL when `status = 'active'` on an older binding is enough. `REVOCATION_POLICY=RECHECK_AND_OMIT`.

## 10. Shared driver across tenants

- Setup: tenant A and tenant B each have a current driver assignment and an active device binding. The driver UUID may be the same or different.
- Input: a target read for that device.
- Oracle: PASS when the result contains two labeled scopes, or only the scopes whose current rows match, and `LIMIT 1` is not applied. FAIL when `tracking_repository.go:51-53` keeps one tenant.

## 11. Shared vehicle across tenants

- Setup: vehicle V is current on an execution in A and is also stored on an execution in B. Only A's decision claimed V. B is driver-only or claims a different vehicle.
- Input: a target read.
- Oracle: PASS when B is not returned because of V. FAIL when `actorMatchesTarget` (`eta_ingest_service.go:311-327`) grants B from the vehicle UUID.

## 12. Untrusted service identity

- Setup: a caller presents the shared tracking token, or integration-token headers without that token.
- Input: a request for a binding decision and for `GetExecutionStop`.
- Oracle: PASS when no decision is issued and no ETA body is returned. FAIL when the shared token or a client service-name header is a named identity. `NAMED_SERVICE_IDENTITY=UNRESOLVED_NO_EXISTING_MECHANISM`.

## 13. Expired authority decision

- Setup: a decision whose `expires_at` has passed. The shipment assignment is unchanged.
- Input: a binding write using that `decision_id`.
- Oracle: PASS when nothing is inserted and a new check is required. FAIL when the unexpired assignment bypasses `expires_at`, or when the same `decision_id` can be consumed twice.

## 14. Concurrent reassignment

- Setup: session 1 has evaluated scope A. Session 2 commits a new shipment driver or a new revision before session 1 writes or finishes the read.
- Input: session 1's write and target result.
- Oracle: PASS when the version mismatch drops the scope and no effect is committed from the stale view. FAIL when the earlier decision or the pre-lock target list remains the authority. The execution command lock is not this read.

## 15. Authorization source unavailable

- Setup: shipment or execution rows cannot be read.
- Input: a check, a binding write, and a target read.
- Oracle: PASS when no decision is stored, no binding is inserted, and the target set is empty, with no call to the unscoped actor query. FAIL when headers or `status = 'active'` are the fallback. `USE_TIME_RECHECK=CURRENT_ROWS_OR_EMPTY`.

## Execution record

| Check | Result |
| --- | --- |
| Tests 1-15 | `NOT_RUN` |
| Product code | unchanged |
| Migrations | none |
| Secrets | unchanged |
