# TRACKING-TENANT-READ-AUTHORITY-0.1F — Test Plan

These tests are specified. None are executed in this stage. Every row below is `NOT_RUN`.

The oracle is the contract in the companion documents. A passing implementation returns only the rows named here and emits no coordinates, driver id, vehicle id, or tenant id in an error body. Current code is not that implementation. `TENANT_ISOLATION_IMPLEMENTED=NO`.

Audit expectation for every test: no audit row exists today. A future denial or allow record must omit coordinates and actor ids. Until that table exists, the audit check is `NOT_IMPLEMENTED`.

## 1. Authorized tenant A

- Setup: one active in-window binding for device D in tenant A. Execution E owned by A. State row for E with the binding's driver and a live stop.
- Trusted identity: gateway `MustAuthContext` tenant A for the binding writer. The read itself receives only provider code and device id.
- Authorization source: that binding joined to E.
- Action: list targets for device D.
- Expected rows: the target for E.
- Expected errors: none.
- Oracle: PASS when the single returned execution id is E. FAIL when a second tenant appears or when the row is produced by an unscoped actor query.

## 2. Unauthorized tenant B

- Setup: the row from test 1. Tenant B has no binding for device D and does not own E.
- Trusted identity: none for B.
- Authorization source: bindings actually loaded for D.
- Action: list targets for D, and attempt `GetState` and `GetExecutionStopETA` with B's id supplied by a caller.
- Expected rows: the list contains E only under A's scope. `GetState` and ETA return no row.
- Expected errors: generic denial or empty result.
- Oracle: PASS when B receives no coordinates and no ETA. FAIL when a caller-supplied tenant B, or a caller-supplied tenant A, is enough to read the row.

## 3. Shared actor across tenants

- Setup: one driver UUID stored on execution E1 in tenant A and execution E2 in tenant B. Two active bindings, one per tenant, or one device active in both tenants as the unique index allows (`000026:28-30`).
- Trusted identity: each binding is its own scope.
- Authorization source: per-binding join.
- Action: list targets for the device.
- Expected rows: E1 under scope A and E2 under scope B, as separate results.
- Expected errors: none for the list. A caller that asks for one tenant without a passing scope for that tenant receives no row.
- Oracle: PASS when the implementation does not collapse the pair with `ORDER BY active_from DESC LIMIT 1`. FAIL when only the newer binding is visible.

## 4. Multiple device bindings

- Setup: device D has two active bindings in tenant A for two shipments, two executions.
- Trusted identity: both bindings.
- Authorization source: each binding joined separately.
- Action: list targets.
- Expected rows: both executions, each tied to its binding.
- Expected errors: none.
- Oracle: PASS when both scopes are present and neither is chosen by `LIMIT 1`. FAIL when the result is an unscoped union by driver id that also includes another tenant.

## 5. Revoked binding

- Setup: the binding row's `status` is `revoked`. An execution still carries the same driver.
- Trusted identity: none from that row.
- Authorization source: active in-window bindings only.
- Action: list targets for the device.
- Expected rows: none from the revoked row.
- Expected errors: empty result.
- Oracle: PASS when the revoked row contributes no target. FAIL when `status = 'active'` is the only filter and a revoked row was left active, or when a revoked row is still selected. A row that is still `active` is not evidence of revocation.

## 6. Stale owner epoch

- Setup: a scope read would have matched at epoch 4. The execution owner epoch is then 5 before a write.
- Trusted identity: the read scope, valid only for the statement that loaded it.
- Authorization source: the write fence in PR #375, not the earlier read.
- Action: attempt the state or ETA write with the epoch-4 result.
- Expected rows: no new state effect.
- Expected errors: the write fails closed. The read response, if already sent, is not a grant.
- Oracle: PASS when the write does not commit. FAIL when the earlier target list is treated as the epoch check. `OWNER_EPOCH_MONOTONICITY` is still `SPECIFIED_NOT_ENFORCED`, so this test cannot pass before that enforcement exists.

## 7. Forged tenant claim

- Setup: tenant A's row exists. The caller sends `X-Tenant-ID` of A, or of B, on the internal ETA route and on a direct `GetState` call.
- Trusted identity: the shared token only, if presented.
- Authorization source: none from the header.
- Action: `GetExecutionStopETA` and `GetState`.
- Expected rows: none.
- Expected errors: generic unauthorized or empty result.
- Oracle: PASS when the header value is not a function argument. FAIL when equality of the header and `operating_tenant_id` returns the ETA.

## 8. Forged execution ID

- Setup: execution E belongs to tenant A. The caller presents E and tenant B, or E and no scope.
- Trusted identity: none.
- Authorization source: execution owner row inside the function.
- Action: `GetState` for E.
- Expected rows: none.
- Expected errors: the same generic error used for an unknown id.
- Oracle: PASS when the response does not reveal that E exists. FAIL when the current `GetState` query (`execution_tracking_repository.go:178-179`) returns coordinates.

## 9. Forged stop ID

- Setup: stop S belongs to tenant A's execution. The caller presents S and a forged tenant header.
- Trusted identity: shared token, if presented.
- Authorization source: none from the stop UUID.
- Action: `GetExecutionStop`.
- Expected rows: none.
- Expected errors: generic error, no ETA timestamp.
- Oracle: PASS when the UUID alone, or the UUID plus a forged header, returns no row. FAIL when `execution_tracking_repository.go:291` returns the row for the header tenant.

## 10. Unauthorized internal service

- Setup: a caller without the shared secret, and a caller with a wrong secret.
- Trusted identity: none.
- Authorization source: none.
- Action: `GetExecutionStop` and the internal driver location route.
- Expected rows: none.
- Expected errors: unauthorized, as `internalTokenOK` already returns when the secret is empty or mismatched (`eta_handler.go:282-288`).
- Oracle: PASS when no ETA body is returned. This check already exists for the secret comparison and does not authorize a tenant.

## 11. Internal service with restricted scope

- Setup: a holder of the shared secret, restricted by this contract to no tenant scope.
- Trusted identity: the secret identifies "some internal caller" only.
- Authorization source: no named service scope exists.
- Action: `GetExecutionStop` for a real stop, with the victim tenant header.
- Expected rows: none.
- Expected errors: generic denial.
- Oracle: PASS when the secret does not widen scope. FAIL when the current handler (`eta_handler.go:291-306`) returns the ETA. The gate stays closed until a named service identity exists.

## 12. Missing identity provider

- Setup: gateway authentication is required and `MustAuthContext` fails.
- Trusted identity: none.
- Authorization source: none.
- Action: driver location ingest through the gateway.
- Expected rows: no new binding and no target.
- Expected errors: unauthorized from `driver/handler.go:360-364`, before tracking is called.
- Oracle: PASS when tracking is not called. FAIL when a development tenant or a header is substituted while authentication is required. The authentication-disabled branch (`handler.go:382-388` on the tracking handler, and the driver handler's development tenant) is a separate configured mode and is not a pass for this test.

## 13. Missing company relationship

- Setup: a carrier relationship row is absent or revoked. An active binding and an execution row still match on tenant and driver.
- Trusted identity: the binding join only.
- Authorization source: the actor read does not consult `company.carrier_relationship_requests` or `transport.transport_execution_carrier_relationship_pins`.
- Action: list targets.
- Expected rows: under the selected actor contract, the binding join still matches. Relationship loss is not an input.
- Expected errors: none from relationship state.
- Oracle: PASS when the test records `RELATIONSHIP_CHECK=NOT_WIRED` and does not claim the relationship failure hidden the row. FAIL when the report calls that result safe because a relationship table exists. A later task that wires the pin may change this oracle. This stage does not.

## 14. Driver reassignment

- Setup: the binding still names driver D1. The execution row now names driver D2. The state row may still show D1 until a fenced write.
- Trusted identity: the current execution row.
- Authorization source: join of binding driver to execution driver.
- Action: list targets.
- Expected rows: none for that pair when the execution driver disagrees.
- Expected errors: empty result.
- Oracle: PASS when the stale binding does not return the execution. FAIL when the state-row driver alone is enough.

## 15. Vehicle reassignment

- Setup: same as test 14 with `vehicle_id`.
- Trusted identity: the current execution row.
- Authorization source: the vehicle half of the same join.
- Action: list targets.
- Expected rows: none when the execution vehicle disagrees and the driver half also disagrees.
- Expected errors: empty result.
- Oracle: PASS on disagreement. FAIL when `actorMatchesTarget` (`eta_ingest_service.go:311-327`) accepts the state-row vehicle with no execution check.

## 16. Cross-tenant ETA request

- Setup: ETA row for stop S in tenant A. Caller is tenant B, or is the shared-token holder forging tenant A.
- Trusted identity: none for this route.
- Authorization source: blocked gate.
- Action: `GetExecutionStop`.
- Expected rows: none.
- Expected errors: generic denial, no estimated arrival.
- Oracle: PASS when both the forged header and the true tenant header return no row to an unscoped caller. FAIL when `operating_tenant_id = $1` is treated as authority.

## 17. Concurrent owner transition

- Setup: two sessions. Session 1 loads a matching binding and execution tenant. Session 2 commits a new operating tenant, or a new owner epoch, before session 1 writes.
- Trusted identity: the row locked by session 2's committed change.
- Authorization source: the execution lock and epoch check, not session 1's earlier read.
- Action: session 1 writes approach or ETA from the stale target.
- Expected rows: no write from session 1.
- Expected errors: the write aborts.
- Oracle: PASS when session 1's transaction does not commit the effect. FAIL when the pre-lock list at `approach.go:69` is the authority. `SHIPMENT_OWNER_RACE` stays open until the fence is implemented.

## 18. No authorized targets

- Setup: location event for a device with no active in-window binding, or bindings whose joins fail.
- Trusted identity: none.
- Authorization source: empty binding set.
- Action: list targets and emit approach.
- Expected rows: none.
- Expected errors: empty result. Approach does not call `MarkApproachEmitted`.
- Oracle: PASS on the empty set. FAIL when `FindActiveBindingByDeviceAnyTenant` picks a binding from another tenant by `active_from`.

## 19. Partial authorization across multiple bindings

- Setup: device D has binding A that joins execution E1, and binding B that does not join any execution.
- Trusted identity: scope A only.
- Authorization source: per-binding join.
- Action: list targets.
- Expected rows: E1 only.
- Expected errors: none for the authorized scope. Scope B adds nothing.
- Oracle: PASS when E1 is returned and B contributes no foreign row. FAIL when the driver ids of A and B are unioned into one unscoped query as `eta_ingest_service.go:182-192` does today.

## 20. Shared privileged DB login bypass

- Setup: the application login can `SELECT` `tracking.execution_tracking_state` and `tracking.execution_stop_eta_state`, which is true while that login owns the tables or holds table `SELECT`.
- Trusted identity: none. The login is not a tenant scope.
- Authorization source: none. `GRANT` and `REVOKE` do not constrain the table owner or a superuser.
- Action: `SELECT` by execution id and by actor id, omitting every function.
- Expected rows: the contract cannot stop this session. The test's expected secured-role rows are none. The privileged session still sees the data.
- Expected errors: none from PostgreSQL for the owner.
- Oracle: PASS when the report marks `SHARED_PRIVILEGED_LOGIN=OPEN` and does not count the function contract as a bypass closure. FAIL when the test is reported as tenant isolation. `ALL_WRITERS_FENCED=NO`. `TENANT_ISOLATION_IMPLEMENTED=NO`.

## Execution record

| Check | Result |
| --- | --- |
| Tests 1-20 | `NOT_RUN` |
| Product code | unchanged |
| Migrations | none |
| Database roles | unchanged |
