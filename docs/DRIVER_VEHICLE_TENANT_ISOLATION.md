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
| Driver/Vehicle list GET | `tenant_id` query (legacy internal pattern — documented risk) |
| Assign driver/vehicle | Trusted `X-Tenant-ID` header only (no `tenant_id` in JSON body) |
| Create driver/vehicle | `tenant_id` in JSON body |

Detail endpoints reject query-only tenant with `401 Unauthorized` and do not call the repository.

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

`GET /v1/drivers` and `GET /v1/vehicles` still require `tenant_id` query parameter. This assignment does not migrate list endpoints to header-only tenant. Direct access to shipment-service with a spoofed list query remains an internal-network risk.

## Remaining risks

- List endpoints use query `tenant_id` without gateway-only enforcement at shipment-service
- No service-to-service header signing
- Development port `8085` exposes shipment-service on localhost

## Related documentation

- Shipment detail tenant isolation: `services/shipment-service/README.md`
- Shipment event history tenant notes: `docs/SHIPMENT_EVENT_HISTORY.md`
