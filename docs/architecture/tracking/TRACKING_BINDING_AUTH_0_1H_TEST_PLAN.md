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

## 16. T16 — forged presenter

- Setup: a decision records `caller_user_id` U1 from the gateway auth context. User U2, or a SQL argument equal to U1, is presented.
- Authority: `AuthContext.UserID` (`auth_context.go:14`). Not a header and not a function parameter.
- Expected database state: no binding. `consumed_at` stays null.
- Authorization result: `HARD_DENIAL`. `BINDING_WRITE=BLOCKED_UNTIL_TRUSTED_PRESENTER` because that presenter channel is not available to the consume function.
- Oracle: PASS when U2 is refused and a function argument of U1 is not treated as U1. FAIL when `decision_id` alone consumes the row.

## 17. T17 — forged device

- Setup: the decision stores `provider_device_id` taken from the resolved driver. The caller presents a different device id.
- Authority: `DEVICE_IDENTITY_SOURCE=FIXED_IN_DECISION_FROM_RESOLVED_DRIVER`.
- Expected database state: no binding. The decision device is unchanged.
- Authorization result: mismatch. No consume.
- Oracle: PASS when the other device is refused. FAIL when the caller's device UUID replaces the decision value or is accepted as ownership.

## 18. T18 — decision replay

- Setup: one consume committed a binding for decision D, presenter U, device X, and shipment S.
- Authority: the stored binding and the consumed decision row.
- Expected database state: still one binding. `consumed_at` stays the first timestamp.
- Authorization result: the identical tuple returns that binding. A second call that changes S, the vehicle, or the device is a conflict.
- Oracle: PASS on the prior result for the identical tuple and on no change for a different tuple. FAIL when replay inserts a second binding, or when `ON CONFLICT DO NOTHING` is reported as authorization.

## 19. T19 — concurrent decision consumption

- Setup: two sessions present the same decision. One has the matching device. The other presents a different shipment id.
- Authority: the row lock `FOR UPDATE` inside the one consume transaction.
- Expected database state: one binding, for the shipment on the decision, or no binding if the first session also fails the re-read. The second session does not overwrite it.
- Authorization result: the loser waits, then conflicts if its identifiers differ.
- Oracle: PASS when only one commit exists and the different-identifier session changes nothing. FAIL when both insert, or when two service transactions are described as one atomic commit.

## 20. T20 — unauthorized update of consumed_at

- Setup: `tracking_runtime` and `shipment_runtime` attempt `UPDATE` of `consumed_at` and of `shipment_id` on `transport.binding_authority_decisions`.
- Authority: column grants. Only `binding_authority_fence` may update `consumed_at` and `use_status`. Identity columns have no such grant. The table owner remains able to update them.
- Expected database state: runtime updates do not commit. An owner update can commit.
- Authorization result: runtime denied. Owner bypass open.
- Oracle: PASS when the report marks runtime denied and `TABLE_OWNER_DECISION_BYPASS=OPEN`. FAIL when `NOLOGIN` is called a sufficient ban, or when the runtime update is described as already rejected by installed grants. The table does not exist yet.

## 21. T21 — decision identifiers modified

- Setup: a session that can act as `binding_authority_owner` changes `shipment_id` on an issued decision. A runtime role attempts the same change.
- Authority: identity columns are written only by the issue function.
- Expected database state: the runtime change does not commit. The owner change can commit and is a bypass, not a valid decision.
- Authorization result: the altered decision no longer matches the shipment row, so consume refuses it. The owner path stays open.
- Oracle: PASS when consume refuses the altered identifiers and the owner risk stays open. FAIL when a changed `shipment_id` still consumes, or when the runtime is said to be unable to change it because the role is `NOLOGIN`.

## 22. T22 — revision change while driver unchanged

- Setup: a decision records revision R1 and driver D. A successor sets `current_revision_id` to R2 (`transport_execution_successor_repository.go:172-176`). `driver_id` on the execution is still D.
- Authority: the new revision. The unchanged driver column is not a version-independent proof.
- Expected database state: no new location effect and no execution target from the R1 decision.
- Authorization result: `REAUTHORIZATION_REQUIRED` for the location right. `EXECUTION_SCOPE_DENIED` for the target.
- Oracle: PASS when R1 grants neither effect. FAIL when the location right survives because `driver_id` was not rewritten. `DRIVER_ONLY_REVISION_POLICY=REAUTHORIZATION_REQUIRED`.

## 23. T23 — shipment version changed

- Setup: the decision records `shipment.version` 3. A later shipment update commits version 4. The driver is unchanged.
- Authority: `shipment.version` on the decision.
- Expected database state: the old decision does not insert a binding and does not return a target.
- Authorization result: `REAUTHORIZATION_REQUIRED` for location. `EXECUTION_SCOPE_DENIED` for the target.
- Oracle: PASS when version 3 grants no new effect. FAIL when the driver match skips the version. `SHIPMENT_VERSION_POLICY=REAUTHORIZATION_REQUIRED`.

## 24. T24 — owner epoch absent

- Setup: a fresh decision has a current driver, an active shipment link, the current revision, and `owner_epoch=ABSENT`.
- Authority: PR #375 epoch rule. The column is not in the migration head.
- Expected database state: a driver-only location binding may be specified. No execution-target grant is stored or returned.
- Authorization result: `DRIVER_ONLY_ALLOWED` for location after the fresh check. `EXECUTION_SCOPE_DENIED` for the target.
- Oracle: PASS when the target is denied despite the link, revision, and driver. FAIL when those three alone return a target. `EXECUTION_TARGET_AUTHORIZATION=BLOCKED_WHILE_OWNER_EPOCH_ABSENT`.

## 25. T25 — reassignment between issue and consume

- Setup: a decision is issued for driver D1. Before consume, the shipment `driver_id` becomes D2.
- Authority: the re-read inside the consume transaction.
- Expected database state: no binding. The decision stays unconsumed.
- Authorization result: `HARD_DENIAL` for D1. D2 has no decision yet.
- Oracle: PASS when the issued decision does not bind D1 after the change. FAIL when consume uses the decision's copied driver and skips the current row.

## Execution record

| Check | Result |
| --- | --- |
| Tests 1-25 | `NOT_RUN` |
| Product code | unchanged |
| Migrations | none |
| Secrets | unchanged |
| Decision table | not created |
