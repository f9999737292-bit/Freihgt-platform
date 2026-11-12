# TRACKING-TENANT-READ-AUTHORITY-0.1F — GetState

## 1. Current behavior

`GetState` loads `tracking.execution_tracking_state` by `execution_id` (`services/tracking-service/internal/repository/execution_tracking_repository.go:177-179`). The selected columns include `operating_tenant_id`, coordinates, `driver_id`, and `vehicle_id` (`execution_tracking_repository.go:171-174`).

There is no production caller. The only in-tree call is `execution_stop_tracking_integration_test.go:661`.

The method takes no tenant, no user, and no binding. Any session that can `SELECT` the table can read any execution's coordinates and actor ids.

## 2. Why a tenant predicate is not the control

PR #376 considered a protected function that compared `operating_tenant_id` with a caller-supplied tenant. This stage rejects that comparison as the authorization decision. The caller who can pass the argument can pass the victim's tenant. The SQL then returns the victim's coordinates, driver, and vehicle.

The shipment command check at `transport_execution_command_repository.go:159-161` has the same shape for a command tenant: it compares the command value with the locked row. It also checks the driver actor against the locked driver (`:181-187`). Tracking `GetState` does not call that function, and a read is not a command.

## 3. Decision

`GET_STATE_AUTHORIZATION=BLOCKED_NO_TRUSTED_CALLER`

The blocked contract, specified and not created:

- `tracking_runtime` has no table `SELECT` on `tracking.execution_tracking_state`. This stays aligned with PR #376.
- No protected function is exposed to `tracking_runtime` for this read until a caller exists whose tenant and user were copied from gateway `MustAuthContext` (`services/api-gateway/internal/tracking/handler.go:370-380`) and whose execution participation was confirmed by shipment-owned data.
- A function parameter named tenant, user, or execution is not that confirmation.
- The shipment-owned confirmation, when a later task wires it, is a join the function owner performs: the execution row's `operating_tenant_id`, or an existing participant row, must match the gateway tenant, and the gateway user must have an active `core.company_memberships` row for that tenant (`000002:79-93`). Tracking-service does not consult that table today.
- Until that caller and that join exist, the read returns no row to every caller, including the application role.
- The error shown to a client is the same for an unknown execution and for a denied execution. The error text contains no coordinates, driver id, vehicle id, or tenant id.
- `LockExecutionTrackingStateTx` (`execution_tracking_repository.go:186-189`) is a write lock, not a read grant. The runtime role does not regain it.

Direct table `SELECT` by the table owner or a superuser is outside this function. PR #376 leaves that bypass open. This document does not close it.

## 4. What each side can enforce

| Check | Where |
| --- | --- |
| No row when the execution id is unknown | PostgreSQL |
| No row when the execution owner disagrees with a scope the function itself loaded | PostgreSQL, after a trusted scope row exists |
| The argument was copied from the gateway auth context rather than from `X-Tenant-ID` | The gateway and the tracking handler. PostgreSQL does not see the JWT. |
| Membership is active | PostgreSQL can read `core.company_memberships` inside the function owner. The user id is trusted only when the server copied it from `MustAuthContext`. |

## 5. Owner epoch

A successful read is not an owner-epoch grant. A later write still uses the PR #375 order. `GetState` does not take `transport.lock_execution_owner`.

## 6. Unresolved

- Which shipment participant table is the correct public-read source for a tenant that operates the execution versus a tenant that only participates. This stage does not guess.
- No gateway route calls `GetState`. Adding one is an implementation task and is out of scope.
