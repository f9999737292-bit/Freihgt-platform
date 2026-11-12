# TRACKING-BINDING-AUTHORITY-0.1G — Binding Creation Inventory

Status: discovery. Current behavior and the proposed authority are separate. Nothing in this file is implemented.

Baseline at document time:

- `origin/main` = `3f306fc407aeda4bdc652d825954292f4a54fad6`
- migration head = `infrastructure/migrations/000105_platform_postgres_role_foundation_v0_2_i1.up.sql`
- PR #377 Draft at `d959a39edfe3d69f0d8d0a89540814a196890e7e`
- PR #376 Draft at `c662f562db958f92ad6b69e12fab228b00c93308`
- PR #375 Draft at `9f31b6cfd96a06ece04eb4b946787f9813bfeeb4`
- PR #374 Draft at `89b509797ae61558582f4c198647ba13d21d9a47`
- PR #357 Draft at `0cf21055dd2a91c5ef071d6982c1fcc70cd11670`

`J-DML-001` stays `NOT_CLOSED`.

## 1. Production call chain

The only production `INSERT` into `tracking.shipment_tracking_binding` found in this tree is `services/tracking-service/internal/repository/tracking_repository.go:472-477`. A test insert exists at `services/tracking-service/internal/integration/driverlocation/execution_stop_tracking_integration_test.go:617`. No `UPDATE` of this table was found in `services/` or `infrastructure/migrations/`.

### 1.1 Gateway driver route

`POST /api/v1/driver/me/shipments/{shipmentId}/locations` is registered at `services/api-gateway/internal/http/router.go:218`.

`services/api-gateway/internal/driver/handler.go:229-264`:

1. `buildRequestContext` (`handler.go:360-364`) copies tenant and user from `MustAuthContext` when authentication is enabled.
2. `ensureDriverAccess` (`handler.go:341-357`) calls `driverrbac.CanAccessDriverRoutes` (`services/api-gateway/internal/driverrbac/rbac.go:9`).
3. `GetShipment` (`handler.go:240-245`) calls shipment `GET /v1/driver/me/shipments/{id}` (`services/api-gateway/internal/driver/client.go:46-47`). The gateway treats HTTP 404 as not found and any status at or above 400 as unavailable. It does not read the shipment body.
4. `GetMe` (`handler.go:246-257`) calls `GET /v1/driver/me` (`client.go:34-35`) and reads `id`.
5. `IngestDriverLocation` (`handler.go:264`) sends that tenant, that driver id, and an empty vehicle id.

The outbound tracking call sets the headers itself (`services/api-gateway/internal/tracking/client.go:192-204`): `X-Tenant-ID`, `X-Driver-ID`, optional `X-Vehicle-ID`, and `X-Internal-Service-Token`.

### 1.2 Shipment checks on that gateway path

Shipment `GetShipment` (`services/shipment-service/internal/http/handlers/driver_operations_handler.go:95-111`) uses `resolveDriverContext` (`:301-310`).

`resolveVerifiedTenant` (`services/shipment-service/internal/http/handlers/tenant.go:13-16`) reads `X-Tenant-ID`. The comment says the gateway sets it. Shipment-service does not verify a JWT.

`ResolveDriver` (`services/shipment-service/internal/service/driver_operations_service.go:64-78`) loads `transport.drivers` by `user_id` and `tenant_id` (`services/shipment-service/internal/repository/driver_repository.go:82-86`). The driver must be active and the stored user id must equal the caller user id.

`GetByIDAndDriver` (`services/shipment-service/internal/repository/shipment_repository.go:292-299`) loads the shipment in that tenant and applies `CanDriverAccessShipment` (`services/shipment-service/internal/domain/driver_operations.go:271-278`): the shipment tenant must match and `shipment.driver_id` must equal the resolved driver id. A mismatch is returned as not found.

This is a server-owned shipment assignment check. It is not forwarded to tracking. The gateway discards the body.

### 1.3 Tracking internal route

`POST /internal/v1/tracking/driver/shipments/{shipmentId}/locations` is registered at `services/tracking-service/internal/http/router.go:69`.

`IngestDriverLocation` (`services/tracking-service/internal/http/handlers/tracking_handler.go:96-135`):

- Authentication: `X-Internal-Service-Token` must be non-empty and equal the configured secret (`:97-99`). The secret is loaded at `services/tracking-service/internal/config/config.go:89` and passed at `services/tracking-service/cmd/server/main.go:74`. This document does not record the value.
- Tenant: `tenantFromRequest` (`tracking_handler.go:249-254`) parses `X-Tenant-ID`.
- Shipment: the URL path.
- Driver: `X-Driver-ID` (`:111-114`).
- Vehicle: `X-Vehicle-ID` when present (`:116-123`). The gateway sends an empty vehicle, so this field is absent on the gateway path.

`IngestDriverMobileLocation` (`services/tracking-service/internal/service/ingest_service.go:58-67`) calls `EnsureActiveDriverMobileBinding` with those four values, then `IngestProviderLocations` for provider `driver_mobile`.

`EnsureActiveDriverMobileBinding` (`tracking_repository.go:456-478`):

- Device id is `driverID.String()` (`:461`).
- Lookup is tenant-scoped (`:462`, query at `:34-41`).
- The same shipment returns success (`:464-465`).
- A different shipment in that tenant returns an error (`:467`).
- Otherwise it inserts `status = 'active'` (`:472-477`) with `ON CONFLICT DO NOTHING`.
- The insert result is not checked for rows affected. Two concurrent inserts can leave the first shipment in place while the second call returns success.

The error string is returned to the client (`ingest_service.go:65`). It does not include coordinates.

### 1.4 Use after insert

`IngestProviderLocations` does not use the tenant it just stored. It calls `FindActiveBindingByDeviceAnyTenant` (`ingest_service.go:101`, query at `tracking_repository.go:46-55`). That query is `ORDER BY active_from DESC LIMIT 1` and has no tenant predicate. The event then copies tenant, shipment, vehicle, and driver from that one row (`ingest_service.go:133-136`). Approach runs from `AcceptLocation` when enabled (`ingest_service.go:157`) and lists targets with no tenant predicate (`services/tracking-service/internal/service/approach.go:69`).

`ListActiveBindingsByDevice` (`services/tracking-service/internal/repository/execution_tracking_repository.go:370-375`) is the ETA and multi-binding read. It also has no tenant predicate. ETA ingest collects actor ids and calls `ListActiveTargetsForActors` (`services/tracking-service/internal/service/eta_ingest_service.go:177-192`).

Provider location, ETA, and slot ingest authenticate a provider secret (`tracking_handler.go:138-144`) and only look up an existing binding. They do not insert one. `FindActiveBindingByDeviceAnyTenant` is also used at `eta_ingest_service.go:100` and `services/tracking-service/internal/service/slot_ingest_service.go:94`.

## 2. Field trust on the insert

| Field | Current source | Classification |
| --- | --- | --- |
| Tenant | `X-Tenant-ID` after the shared-token check (`tracking_handler.go:101-104`) | CLAIM. Gateway copies `MustAuthContext` on its own route (`driver/handler.go:360-367`). Tracking cannot tell those calls apart. |
| Driver | `X-Driver-ID` (`tracking_handler.go:111-114`) | CLAIM. Shipment can resolve a driver from user and tenant (`driver_repository.go:82-86`). Tracking stores the header. |
| Vehicle | `X-Vehicle-ID` or null (`tracking_handler.go:116-123`) | CLAIM when present. NOT_VERIFIED against `transport.shipments.vehicle_id`. The gateway path sends no vehicle. |
| Shipment | URL (`tracking_handler.go:106-109`) | CLAIM. Shipment's `CanDriverAccessShipment` (`driver_operations.go:271-278`) is not repeated. |
| Caller authentication | shared token equality (`tracking_handler.go:97-99`) | NOT a tenant, driver, or shipment grant. |
| Tenant authorization | none inside tracking | NOT_VERIFIED |
| Carrier assignment | `transport.drivers.carrier_company_id` exists (`000003_create_transport_tables.up.sql:99-102`). The insert does not read it. | NOT_VERIFIED |
| Operating tenant | `transport.transport_executions.operating_tenant_id` (`000088_tms_transport_execution_foundation_v0_1a.up.sql:10-14`). The insert does not read it. `000088:6-7` says shipment ownership is not moved. | NOT_VERIFIED |
| Revocation | status check allows `revoked` (`000026_add_shipment_telemetry_foundation_v0.7.0.up.sql:18`). No production writer sets it. | NOT_VERIFIED |
| Foreign identifiers | any token holder can supply another tenant, driver, vehicle, and shipment | the insert will store them when the tenant-scoped device lookup does not already point at a different shipment |

No field on the insert is TRUSTED inside tracking-service. The shipment assignment check is TRUSTED only for the shipment HTTP call that the gateway makes and then discards.

## 3. What an existing row proves

`status = 'active'` means the insert wrote that literal (`tracking_repository.go:475`). The active unique index is per tenant (`000026:28-30`), so one device can be active in several tenants. `active_to` is not in the lookup `WHERE` (`tracking_repository.go:39`, `execution_tracking_repository.go:374`). A row does not name a user, a decision, a version, or an execution.

## 4. Limits

- Direct calls to the internal route bypass the gateway and the shipment assignment check.
- `ON CONFLICT DO NOTHING` can report success for a shipment that did not win the race.
- Provider bindings have no production creator in this tree.
- This inventory does not change the table.
