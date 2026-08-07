# Driver & Vehicle Tenant Isolation v0.1

## Location

Driver and Vehicle master data live in **shipment-service** (`services/shipment-service`):

| Entity | Table | Internal API |
|---|---|---|
| Driver | `transport.drivers` | `GET /v1/drivers/{id}` |
| Vehicle | `transport.vehicles` | `GET /v1/vehicles/{id}` |

Public access is through **API Gateway** proxy routes:

| Public route | Internal route |
|---|---|
| `GET /api/v1/drivers/{id}` | `GET /v1/drivers/{id}` |
| `GET /api/v1/vehicles/{id}` | `GET /v1/vehicles/{id}` |

Assignment operations are shipment commands:

| Public route | Internal route |
|---|---|
| `POST /api/v1/shipments/{id}/assign-driver` | `POST /v1/shipments/{id}/assign-driver` |
| `POST /api/v1/shipments/{id}/assign-vehicle` | `POST /v1/shipments/{id}/assign-vehicle` |

## Trust boundary

External clients reach shipment-service only through API Gateway. Gateway validates JWT, strips spoofed identity headers (`X-Tenant-ID`, `X-User-ID`, etc.), and sets trusted outbound headers from `AuthContext`.

In production, shipment-service must not be exposed on a public ingress. Local docker-compose port `8085` is development-only.

There is **no dedicated service-to-service authentication** on internal headers in v0.1; network isolation and gateway boundary are the primary controls.

## Tenant source

| Operation | Tenant source |
|---|---|
| Driver/Vehicle detail GET | Trusted `X-Tenant-ID` header only (no `tenant_id` query) |
| Driver/Vehicle list GET | Trusted `X-Tenant-ID` header only (no `tenant_id` query) |
| Assign driver/vehicle | Trusted `X-Tenant-ID` header only (no `tenant_id` in JSON body) |
| Create driver/vehicle | Trusted `X-Tenant-ID` header only (no `tenant_id` in JSON body) |

Create и List операции Driver/Vehicle не принимают tenant_id как источник области данных. Tenant определяется только из verified gateway context.

Detail, create, list, and assignment endpoints reject query-only or body-only tenant with `401 Unauthorized` and do not call the repository.

## Repository invariant

```text
Driver и Vehicle загружаются для tenant-bound операций только по комбинации object_id + verified tenant_id. Проверка tenant после unscoped загрузки не используется как основной механизм isolation.
```

SQL pattern:

```sql
WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
```

Tables use global UUID primary keys (`id UUID PRIMARY KEY`). Existing `idx_drivers_tenant_id` and `idx_vehicles_tenant_id` support list queries; no new composite index is required for PK detail lookup.

## Detail lookup

- One repository query per detail request
- Missing, foreign-tenant, and soft-deleted objects return the same `404 Not Found`
- No second lookup by ID only
- No distinguishable foreign-tenant error codes

## Create tenant invariant

`POST /v1/drivers` and `POST /v1/vehicles`:

1. Resolve verified tenant via `resolveVerifiedTenant(r)`
2. Strict JSON decode (unknown `tenant_id` field → `400`)
3. Validate business fields only
4. Pass verified tenant separately to service/repository INSERT

## List tenant invariant

`GET /v1/drivers` and `GET /v1/vehicles`:

1. Resolve verified tenant via header only
2. Parse business filters (`carrier_company_id`, `status`, `limit`, `offset`)
3. Ignore `tenant_id` query parameter if present
4. SQL always includes `WHERE tenant_id = $1 AND deleted_at IS NULL`

## Carrier company validation

Before create, service checks `CompanyExists(carrier_company_id, verified_tenant_id)` against `core.companies` in the same database (`WHERE id = $1 AND tenant_id = $2`). Foreign-tenant company returns `404 carrier_company_id not found`.

## RBAC

Route-level authorization for Driver, Vehicle, and fleet assignment operations is enforced by **API Gateway** before proxying to shipment-service.

| Operation | Gateway route | Allowed roles |
|---|---|---|
| View driver/vehicle | `GET /api/v1/drivers`, `GET /api/v1/drivers/{id}`, `GET /api/v1/vehicles`, `GET /api/v1/vehicles/{id}` | `PLATFORM_ADMIN`, `CARRIER_ADMIN`, `CARRIER_DISPATCHER` |
| Create driver/vehicle | `POST /api/v1/drivers`, `POST /api/v1/vehicles` | `PLATFORM_ADMIN`, `CARRIER_ADMIN` |
| Assign driver/vehicle | `POST /api/v1/shipments/{id}/assign-driver`, `POST /api/v1/shipments/{id}/assign-vehicle` | `PLATFORM_ADMIN`, `CARRIER_ADMIN`, `CARRIER_DISPATCHER` |

Roles are fetched from identity-service via `GET /v1/auth/me` using the verified bearer token. JWT does not contain roles. Incoming identity headers (`X-User-Roles`, `X-Tenant-ID`, etc.) are stripped before RBAC.

| Scenario | HTTP |
|---|---|
| Unauthenticated | `401` |
| Authenticated, insufficient role | `403` (downstream not called) |
| Identity dependency unavailable | `503` (`AUTH_DEPENDENCY_UNAVAILABLE`) |
| Allowed role | Request proxied to shipment-service |
| Foreign resource after authorization | `404` from shipment-service |

Shipment-service does **not** duplicate endpoint-level RBAC in v0.1. It continues to require trusted `X-Tenant-ID` and perform tenant-scoped SQL. Frontend role restrictions are UX only and are not a security control.

## Authorization vs tenant isolation

API Gateway performs authentication and route-level authorization. Shipment-service performs tenant isolation at handler, service, and SQL repository layers. Tenant-scoped SQL does not replace checking whether the authenticated user is allowed to perform the operation.

- **Authorization (gateway):** Can this user call this route?
- **Tenant isolation (shipment-service):** Does this resource belong to the verified tenant?

Both controls are required and independent.

## Assignment tenant trust boundary

Assign Driver и Assign Vehicle не принимают `tenant_id` из JSON body. Tenant определяется только trusted gateway context (`X-Tenant-ID`).

- Request body contains only `driver_id` or `vehicle_id`
- Body-only tenant without trusted header → `401 Unauthorized`
- Conflicting `tenant_id` in body → `400 Bad Request` (strict JSON decoder rejects unknown fields)
- Detail and assignment endpoints share the same tenant trust boundary via `resolveVerifiedTenant`

## Assignment invariant

`AssignDriver` and `AssignVehicle`:

1. Load shipment via `GetByIDAndTenant(shipment_id, tenant_id)`
2. Load driver/vehicle via `GetByIDAndTenant(resource_id, tenant_id)`
3. Validate carrier company match
4. Execute UPDATE with `WHERE id = ... AND tenant_id = ...`

Foreign driver or vehicle at a known UUID returns the same `404` as a missing resource.

## Update/delete paths

v0.1 exposes **Create**, **GetByID**, and **List** for Driver/Vehicle. No update/delete/archive endpoints exist. Shipment mutation SQL already includes tenant predicates.

## Not Found semantics

| Condition | HTTP | Error shape |
|---|---|---|
| Unknown ID | 404 | `NOT_FOUND` |
| Foreign tenant | 404 | Same `NOT_FOUND` |
| Soft-deleted | 404 | Same `NOT_FOUND` |
| Missing tenant (detail) | 401 | `UNAUTHORIZED` |
| Malformed UUID | 400 | `VALIDATION_ERROR` |

## List endpoint limitations

~~`GET /v1/drivers` and `GET /v1/vehicles` still require `tenant_id` query parameter.~~ Migrated to header-only tenant in v0.1 create/list boundary fix.

## Remaining risks

- No service-to-service header signing
- Endpoint-level RBAC is gateway-only; direct shipment-service access bypasses role checks (mitigated by network boundary in production)
- Development port `8085` exposes shipment-service on localhost
- Frontend permission checks are not a security control; backend `403` is authoritative

## Related documentation

- Shipment detail tenant isolation: `services/shipment-service/README.md`
- Shipment event history tenant notes: `docs/SHIPMENT_EVENT_HISTORY.md`
