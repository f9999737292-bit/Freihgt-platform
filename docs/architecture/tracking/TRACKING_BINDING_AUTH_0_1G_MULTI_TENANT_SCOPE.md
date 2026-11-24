# TRACKING-BINDING-AUTHORITY-0.1G — Multi-Tenant Actor Scope

A shared driver or vehicle is not one tenant. This document sets the scope rule for a future `ListActiveTargetsForActors`. It does not change the function.

## 1. Current collapse

`FindActiveBindingByDeviceAnyTenant` keeps one active row, the latest `active_from`, across all tenants (`services/tracking-service/internal/repository/tracking_repository.go:51-53`). The unique index does not stop a second tenant from having its own active row (`000026_add_shipment_telemetry_foundation_v0.7.0.up.sql:28-30`).

`ListActiveBindingsByDevice` returns every active row and drops the binding id (`execution_tracking_repository.go:370-375`). ETA ingest unions the driver and vehicle ids (`eta_ingest_service.go:182-192`) and then requires a single execution id (`:201-203`). That requirement is not a per-tenant authorization.

`transport.drivers.id` is the primary key (`000003_create_transport_tables.up.sql:100`) and the row has `tenant_id` (`:101`). `transport.transport_executions.driver_id` has no foreign key (`000088_tms_transport_execution_foundation_v0_1a.up.sql:15`). The same UUID can be stored on more than one execution. The UUID is not a read right.

## 2. Proposed scope rule

`SHARED_ACTOR_SCOPE=SEPARATE_VERIFIED_SCOPES`

`MULTI_TENANT_SCOPE=PER_SCOPE_RECHECK_NO_LIMIT_1`

- Each candidate is one binding row plus one shipment assignment plus, for an execution target, one execution row.
- A scope is usable only when it is ACTIVE as defined in the lifecycle document.
- Scopes are not merged into one tenant and not reduced with `LIMIT 1`.
- An actor id without a passing scope returns nothing.
- A REVOKED, EXPIRED, or INVALIDATED_BY_REASSIGNMENT candidate is omitted.
- The caller does not send the tenant list. The function loads candidates from the device and discards those whose current assignment fails.
- A stored binding without a shipment decision is omitted, including every row the current insert would have written.

Two passing scopes for one device stay two results. The reader must be able to tell which tenant and which execution each result belongs to. Coordinates stay inside the scope that passed. They are not copied onto the other scope.

## 3. Approach

Approach today calls the unscoped list (`approach.go:69`) and continues only when exactly one target remains (`approach.go:83-85`).

Proposed: build the authorized scope set first. If that set has one execution, approach may evaluate distance for that execution. If it has zero or more than one, approach does not call `MarkApproachEmitted`. The current count check can remain only as this refusal to guess. It cannot choose among unauthorized rows, and it cannot prefer the newest binding.

## 4. ETA

ETA ingest today uses the same unscoped actor list (`eta_ingest_service.go:192`) and rejects unless one execution id remains (`:201-203`). The later lock compares the stored tracking tenant with itself (`eta_ingest_service.go:256-257`), which does not add a shipment decision.

Proposed: the same authorized scope set as approach. A stop id is accepted only when the single authorized scope names that stop. Two authorized scopes, or a stop that belongs to a scope that failed, produce no ETA write. The provider payload's tenant, if any, is not a scope.

## 5. What this does not allow

- Reading every execution that shares a driver UUID.
- Treating `FindActiveBindingByDeviceAnyTenant` as the tenant choice.
- Using a binding in tenant A to return an execution owned by tenant B.
- Using a shipment-verified binding as an execution grant when the operating tenant or the execution actor disagrees.

## 6. Unproven

The rule is specified. The queries are unchanged. `TENANT_ISOLATION_IMPLEMENTED=NO`.
