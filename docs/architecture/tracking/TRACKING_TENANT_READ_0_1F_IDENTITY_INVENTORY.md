# TRACKING-TENANT-READ-AUTHORITY-0.1F — Identity Inventory

Status: discovery record. No identity mechanism is added by this document.

Baseline observed while writing this file:

- `origin/main` = `3f306fc407aeda4bdc652d825954292f4a54fad6`
- migration head = `infrastructure/migrations/000105_platform_postgres_role_foundation_v0_2_i1.up.sql`
- PR #376 remains Draft at `c662f562db958f92ad6b69e12fab228b00c93308`
- PR #375 remains Draft at `9f31b6cfd96a06ece04eb4b946787f9813bfeeb4`
- PR #374 remains Draft at `89b509797ae61558582f4c198647ba13d21d9a47`
- PR #357 remains Draft at `0cf21055dd2a91c5ef071d6982c1fcc70cd11670`

`J-DML-001` stays `NOT_CLOSED`. A stored tenant value that equals a caller-supplied tenant value is a predicate. It is not proof that the caller may read the row.

## 1. Production callers of the three reads

| Read | Production caller | Evidence |
| --- | --- | --- |
| `GetState` | none | Definition `services/tracking-service/internal/repository/execution_tracking_repository.go:177-183`. The only in-tree call is the integration test `services/tracking-service/internal/integration/driverlocation/execution_stop_tracking_integration_test.go:661`. |
| `ListActiveTargetsForActors` | approach emission and ETA ingest | `services/tracking-service/internal/service/approach.go:69`. `services/tracking-service/internal/service/eta_ingest_service.go:192`. |
| `GetExecutionStopETA` | internal HTTP route | `services/tracking-service/internal/http/router.go:75` calls `services/tracking-service/internal/http/handlers/eta_handler.go:291-306`. No other service under `services/` calls this path. |

`GetExecutionStopETATx` at `execution_tracking_repository.go:282-283` is the in-transaction form used by ETA ingest at `eta_ingest_service.go:275`. Its tenant argument is `locked.OperatingTenantID` from the tracking state row locked at `eta_ingest_service.go:256-257`.

## 2. Existing mechanisms

### 2.1 Gateway principal and tenant

Owner: `services/api-gateway`.

Source: the authenticated context, when authentication is enabled. `services/api-gateway/internal/tracking/handler.go:370-380` reads `MustAuthContext` and copies `ac.TenantID` and `ac.UserID`. It does not read the client `X-Tenant-ID` header in that branch. When authentication is disabled, the same function uses the configured development tenant (`handler.go:382-388`).

The driver location route is `services/api-gateway/internal/http/router.go:218`. `services/api-gateway/internal/driver/handler.go:229-264` takes the tenant from that context, checks driver route access (`handler.go:341-357`), loads the driver id from shipment `GetMe` (`handler.go:246-257`), and only then calls tracking. The outbound call sets `X-Tenant-ID` and `X-Driver-ID` itself (`services/api-gateway/internal/tracking/client.go:192-204`).

Boundary: HTTP at the gateway. Tracking-service is behind that call and does not see the JWT.

Check: gateway role lookup in `ensureDriverAccess`. Tracking-service does not repeat it.

Unavailable: the gateway returns unauthorized or service unavailable and does not call tracking (`driver/handler.go:240-250`).

Substitution: a client header is not the source on this gateway path while authentication is enabled. The development-tenant branch is a configured substitute and is not a user claim.

This gateway context is not attached to `GetState`, `ListActiveTargetsForActors`, or `GetExecutionStop`.

### 2.2 Tracking-service header tenant

Owner: the caller of tracking-service.

Source: `X-Tenant-ID`, via `packages/shared-go/lowcode/headers.go:11` and `headers.go:18`. `services/tracking-service/internal/http/handlers/tracking_handler.go:249-254` parses that header and returns a validation error when it is empty. It does not look up a user, a company, or a membership.

Boundary: the tracking HTTP handler.

Check: non-empty UUID only.

Unavailable: validation error, no row.

Substitution: any caller that can reach the handler can choose the header. On the internal routes the shared service token is the only additional check.

### 2.3 Shared internal service token

Owner: deployment configuration of tracking-service.

Source: `services/tracking-service/internal/config/config.go:89` loads `TRACKING_INTERNAL_SERVICE_TOKEN`, with a fallback environment name. The source tree contains a default. This document does not record that value. `services/tracking-service/cmd/server/main.go:71-75` passes that one value to the ETA, slot, ingest, and internal handlers.

Check: `services/tracking-service/internal/http/handlers/eta_handler.go:282-288`. An empty expected token fails. A provided token must equal the expected token. The header name is `X-Internal-Service-Token`.

What it proves: possession of the shared secret. It does not name a service, a tenant, a user, an execution, or a stop.

Unavailable: missing or mismatched token returns unauthorized (`eta_handler.go:292-294`).

Substitution: every holder of the secret presents the same identity. Shipment context, shipper, and carrier clients are wired with the same configuration value (`main.go:76-83`).

### 2.4 Device binding

Owner: tracking-service, table `tracking.shipment_tracking_binding`.

Source: `infrastructure/migrations/000026_add_shipment_telemetry_foundation_v0.7.0.up.sql:5-30`. Columns include `tenant_id`, `shipment_id`, `vehicle_id`, `driver_id`, `provider_code`, `provider_device_id`, `status`, `active_from`, and `active_to`. Status is constrained to `active`, `inactive`, or `revoked` (`000026:18`). The active unique index is `(tenant_id, provider_code, provider_device_id)` where `status = 'active'` (`000026:28-30`). The same device can be active in more than one tenant.

Reads:

- Tenant-scoped: `services/tracking-service/internal/repository/tracking_repository.go:34-44`.
- Any tenant, one row: `tracking_repository.go:46-55`, ordered by `active_from DESC`, `LIMIT 1`.
- Any tenant, all active rows: `execution_tracking_repository.go:370-375`. The selected columns are `tenant_id`, `shipment_id`, `vehicle_id`, `driver_id` (`:372-373`). The returned struct has those four fields (`:391-396`). It has no binding id, principal, company, or relationship id.

Write used by driver mobile: `tracking_repository.go:456-478`. The tenant, shipment, driver, and vehicle come from the caller. `ON CONFLICT DO NOTHING` does not prove which row won.

`active_to` is not part of any of these `WHERE` clauses. A row with `status = 'active'` and a past `active_to` still matches.

`BindingStatusRevoked` exists at `services/tracking-service/internal/domain/telemetry.go:29`. No assignment of that status was found under `services/tracking-service`. A revoked row is excluded only when some other writer has already changed `status`. This inventory does not claim that writer exists.

Unavailable: no row, or a database error. Provider ingest rejects the item (`ingest_service.go:102-105`). ETA ingest rejects the item (`eta_ingest_service.go:178-180`).

Substitution: the stored `tenant_id` is whatever the binding writer supplied. On the internal driver route that value is the `X-Tenant-ID` header after the shared-token check (`tracking_handler.go:97-105` and `:249-254`).

### 2.5 Driver and vehicle assignment

Owner of the driver row: `transport.drivers`, created at `infrastructure/migrations/000003_create_transport_tables.up.sql:99-116`. `id` is the primary key. `tenant_id` is a column on that row. This inventory does not treat the primary key as a foreign key from execution state.

Execution assignment columns: `infrastructure/migrations/000088_tms_transport_execution_foundation_v0_1a.up.sql:14-15` define `vehicle_id` and `driver_id` as nullable UUIDs. That migration does not declare a foreign key to `transport.drivers` or a vehicle table. The same UUID can be stored on executions of more than one tenant.

The shipment command check is `services/shipment-service/internal/repository/transport_execution_command_repository.go:159-189`. It compares the command tenant with the locked execution tenant, then compares a driver actor id with the locked `driver_id`. Tracking-service does not call `authorizeCommand`.

`actorMatchesTarget` at `eta_ingest_service.go:311-327` compares driver and vehicle UUIDs only.

A separate push-device table, `transport.driver_device`, has `revoked_at` (`000034_add_driver_task_platform_v0.2.up.sql:74-88`). The location and ETA paths in this inventory do not read it.

### 2.6 Company membership

Owner: identity and the `core.company_memberships` table, `infrastructure/migrations/000002_create_core_tables.up.sql:79-93`. Uniqueness is `(company_id, user_id)`.

`services/identity-service/internal/service/membership_service.go:29` lists a user's companies. A search of `services/tracking-service` found no call to company membership.

Unavailable and substitution do not arise on these three reads, because the reads never consult the table.

### 2.7 Company relationship

Owner: company-service and shipment pin tables. Examples: `infrastructure/migrations/000100_company_carrier_relationship_foundation_v0_1c.up.sql:6` and `infrastructure/migrations/000103_tms_relationship_pins_security_foundation_v0_1.up.sql:128`.

A search of `services/tracking-service` found no call to these tables. Loss of a relationship does not, by itself, change the result of the three reads.

### 2.8 Service-to-service authorization and RBAC

The gateway driver route uses `driverrbac.CanAccessDriverRoutes` (`driver/handler.go:354`). Transport-order and shipment guards set `X-Tenant-ID` from the authenticated context after their own policy check, for example `services/api-gateway/internal/transportorderrbac/guard.go:154`.

Those guards are not on `GET /internal/v1/tracking/execution-stops/{executionStopId}/eta`. That route is registered only inside tracking-service (`router.go:67-75`).

Tracking-service has no ABAC evaluation on `GetState`, `ListActiveTargetsForActors`, or `GetExecutionStopETA`.

### 2.9 Trusted internal service identity

The shared token is the only service credential on these routes. It is not a per-service identity. A future named service identity is not invented here.

### 2.10 Database login

`000103` creates `NOLOGIN` roles and leaves the shared application login in place. `000105` does not grant `tracking` schema privileges to a dedicated tracking login. PR #376 designs `tracking_runtime` and `tracking_read_fence` and does not apply them. A login that owns the tables, or a superuser, reads every tenant without using a function. That fact is unchanged.

## 3. Attributes actually available on the actor path

| Attribute | Available | Where |
| --- | --- | --- |
| Tenant id | stored on the binding and on the state row | binding `tenant_id`; state `operating_tenant_id` |
| Operating company | no | not selected by `ListActiveBindingsByDevice` |
| Principal | no | not selected |
| Driver id | yes, nullable | binding and state |
| Vehicle id | yes, nullable | binding and state |
| Device binding id | on the single-row lookup only | `FindActiveBindingByDeviceAnyTenant` selects `id` (`tracking_repository.go:48`). `ListActiveBindingsByDevice` does not (`execution_tracking_repository.go:372`). |
| Relationship id | no | not selected |
| Authorization scope | no stored scope | the queries do not join membership, relationship, or execution assignment |

## 4. Limits

- Gateway JWT tenant is real and is not an input of the three reads.
- Membership and relationship tables are real and are not inputs of the three reads.
- The binding tenant is server-stored and was accepted from the binding writer.
- No new identity provider, token type, or relationship type is proposed.
