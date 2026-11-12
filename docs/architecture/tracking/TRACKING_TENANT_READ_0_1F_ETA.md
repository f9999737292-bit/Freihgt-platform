# TRACKING-TENANT-READ-AUTHORITY-0.1F — GetExecutionStopETA

## 1. Current route

`GET /internal/v1/tracking/execution-stops/{executionStopId}/eta` is registered at `services/tracking-service/internal/http/router.go:75`.

`GetExecutionStop` (`services/tracking-service/internal/http/handlers/eta_handler.go:291-306`) does two things before the query:

1. `internalTokenOK` (`eta_handler.go:282-288`) compares `X-Internal-Service-Token` with the configured secret.
2. `tenantFromRequest` (`tracking_handler.go:249-254`) reads `X-Tenant-ID`.

The query is `operating_tenant_id = $1 AND execution_stop_id = $2` (`execution_tracking_repository.go:287-292`). `$1` is the header value.

No service under `services/` calls this route. The gateway tracking client has no method for it.

## 2. What the token proves

The token proves the caller presented the shared secret loaded at `config.go:89` and passed in at `cmd/server/main.go:71`. The comparison is equality (`eta_handler.go:287-288`). The secret is not a service name, not a tenant, and not a relationship to an execution or a stop.

The same secret is handed to the shipment, shipper, and carrier clients (`main.go:76-83`). Possession does not identify which of those callers is asking.

`X-Tenant-ID` on this route is the caller's header. The route is not behind `MustAuthContext`.

## 3. What a stop UUID proves

The stop id selects a row only together with the header tenant. A caller who knows both values receives the ETA. A role with table `SELECT` can omit the tenant predicate. Knowing the UUID is not a grant.

The in-transaction read at `eta_ingest_service.go:275` uses `locked.OperatingTenantID`. That value was reached through `ListActiveTargetsForActors` without a tenant predicate (`eta_ingest_service.go:192`). It is the stored tenant of a row chosen by actor id. It is not a separate authorization of the stop.

## 4. Decision

`GET_EXECUTION_STOP_ETA_AUTHORIZATION=BLOCKED_TOKEN_IS_NOT_TENANT`

System-wide read for any stop is not granted to the shared token.

The blocked contract, specified and not created:

- `tracking_runtime` has no table `SELECT` on `tracking.execution_stop_eta_state`.
- The protected function does not take a caller tenant.
- It returns a row only when a scope the function loads says the stop belongs to an execution, and that execution's `operating_tenant_id` is one of the scopes already authorized for this caller.
- For an internal service, "authorized" means a named service identity plus a scope issued by the execution owner. The shared token is not that identity. This stage does not define a new token format.
- Until a named caller and a server-owned scope exist, the function returns no row.
- A forged `X-Tenant-ID` therefore cannot select another tenant's ETA, because the header is not an argument.
- The client-visible error is the same for an unknown stop, a stop in another tenant, and a caller without a scope. The body contains no coordinates, driver id, vehicle id, ETA timestamp, or tenant id.
- An empty result is also the result when the authorization source is unavailable. The handler must not fall back to the header predicate.

The gateway driver path is not a caller of this route. Reusing `MustAuthContext` would require a new route. That route is not added here.

## 5. What each side can enforce

| Check | Where |
| --- | --- |
| Stop id belongs to the execution id on the ETA row | PostgreSQL join |
| Execution `operating_tenant_id` matches a scope row the function loaded | PostgreSQL |
| The HTTP caller is a specific service rather than any holder of the shared secret | Not enforceable with the current token. Blocking gate. |
| The header tenant is the gateway tenant | Not true of this route. |

## 6. Owner epoch

Reading an ETA does not authorize a later ETA write. The write path still has to lock the execution and compare epochs under PR #375 and PR #376. A stop id observed before that lock can belong to a different owner by commit time. The write rechecks. The read result is not cached as permission.

## 7. Unresolved

- Which existing service, if any, is the intended caller. None was found.
- How a named service identity should be distinguished from the shared secret without inventing a parallel credential. Left closed until an existing mechanism is chosen by a later task.
