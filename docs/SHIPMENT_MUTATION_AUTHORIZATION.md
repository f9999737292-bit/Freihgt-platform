# Shipment Mutation Authorization v0.1

## Invariant

**Shipment mutation выполняется только после успешной аутентификации и route-level authorization в API Gateway. Shipment-service отдельно обеспечивает tenant isolation, actor provenance и атомарность status/history.**

## Request chain

```text
JWT validation
→ strip spoofed identity headers
→ verified AuthContext (tenant, user, bearer token, request ID)
→ GET /v1/auth/me (verified roles)
→ shipment mutation RBAC policy (per route)
→ reverse proxy
→ shipment-service tenant isolation
→ status / shipment_status_history transaction
```

On `403 Forbidden` the proxy and shipment-service are **not** called. No status change and no history row are created.

## Authentication

- Public clients authenticate with Bearer JWT at API Gateway.
- Gateway middleware sets trusted headers from JWT claims:
  - `tenant_id` → `X-Tenant-ID`
  - `sub` → `X-User-ID`
- Spoofed client identity headers are removed before JWT parsing.

## Role source

Roles are loaded **only** from identity-service:

```http
GET /v1/auth/me
Authorization: Bearer <same user token>
```

Gateway packages `internal/routeauth`, `internal/fleetrbac`, and `internal/shipmentrbac` share one `/auth/me` client implementation. Roles are **not** read from:

- `X-User-Roles`
- query parameters
- request body
- unverified JWT custom claims

Multi-role semantics: **allow if at least one verified role matches the route policy** (exact uppercase match).

## Shipment mutation policies (v0.1)

Policies are defined in `services/api-gateway/internal/shipmentrbac/policies.go` and registered in `services/api-gateway/internal/http/router.go` **before** the generic `/api/*` catch-all proxy.

| Policy | Routes | Allowed roles |
|--------|--------|---------------|
| **Create** | `POST /api/v1/shipments/from-transport-order`, `POST /api/v1/shipments/from-bid` | `PLATFORM_ADMIN`, `SHIPPER_ADMIN`, `SHIPPER_LOGIST`, `FORWARDER_MANAGER` |
| **Accept** | `POST /api/v1/shipments/{id}/accept` | `PLATFORM_ADMIN`, `CARRIER_ADMIN`, `CARRIER_DISPATCHER` |
| **UpdateStatus** | `PATCH /api/v1/shipments/{id}/status` | `PLATFORM_ADMIN`, `CARRIER_ADMIN`, `CARRIER_DISPATCHER` |
| **Cancel** | `POST /api/v1/shipments/{id}/cancel` | `PLATFORM_ADMIN`, `SHIPPER_ADMIN`, `SHIPPER_LOGIST`, `FORWARDER_MANAGER` |

### FORWARDER_MANAGER rationale

`FORWARDER_MANAGER` is included in **Create** and **Cancel** because the web-admin forwarder flow creates shipments from accepted bids (`FreightRequestBidsTable` → shipments create-from-bid) and may cancel on behalf of the shipper side. It is **excluded from Accept and UpdateStatus** because carrier operational status transitions remain carrier-only in v0.1.

`FORWARDER_MANAGER` is seeded idempotently in migration `000013_seed_forwarder_manager_role` so identity-service can assign it via `core.roles` and return it from `/v1/auth/me`. Gateway Create/Cancel policies and web-admin forwarder flows rely on this role.

## HTTP semantics

| Scenario | Response |
|----------|----------|
| Missing/invalid JWT | `401 Unauthorized` |
| Valid JWT, role not allowed | `403 Forbidden` |
| Identity `/auth/me` network/5xx/malformed JSON | `503 AUTH_DEPENDENCY_UNAVAILABLE` |
| Identity `/auth/me` 401 | `401 Unauthorized` |
| Identity `/auth/me` 403 | `403 Forbidden` |
| Empty roles | `403 Forbidden` |
| Allowed role | Proxy invoked once |
| Foreign shipment/order/bid downstream | `404 Not Found` |
| Invalid body downstream | `400 Bad Request` |
| Optimistic lock conflict downstream | existing `409` semantics |

Error responses do not expose identity-service URLs, raw upstream bodies, stack traces, or bearer tokens.

## Downstream trust boundary

After gateway RBAC succeeds, shipment-service continues to enforce:

- verified tenant from `X-Tenant-ID` only (see [SHIPMENT_STATUS_HISTORY.md](./SHIPMENT_STATUS_HISTORY.md))
- verified USER actor from `X-User-ID`
- tenant-scoped repository lookups and mutations
- atomic shipment status + `shipment_status_history` writes

Direct shipment-service access from untrusted networks remains prohibited by production network policy.

## Frontend permissions (UX only)

`apps/web-admin/composables/usePermissions.ts` mirrors gateway allowlists:

- `canCreateShipment()`
- `canAcceptShipment()`
- `canUpdateShipmentStatus()`
- `canCancelShipment()`

UI hides disallowed actions; backend `403` remains authoritative.

On `403`, users see localized `common.insufficientPermission` via `formatApiErrorForUser()` in `useApi.ts`.

## Related RBAC (unchanged)

| Area | Package | Routes |
|------|---------|--------|
| Fleet view/create | `fleetrbac` | drivers, vehicles |
| Fleet assign | `fleetrbac` | assign-driver, assign-vehicle |
| Shipment events read | `shipmentevents` | `GET /api/v1/shipments/{id}/events` |
| Control Tower | `controltower` | `GET /api/v1/control-tower/summary` |

## Database permissions (future)

Migration `000008_seed_permissions` defines `shipment.create`, `shipment.read`, `shipment.update`, `shipment.assign_carrier`, but `core.role_permissions` is not seeded and gateway v0.1 uses role allowlists only.

## v0.1 limitations and migration path

- Role allowlists are hard-coded per route; no permission-based RBAC yet.
- `GET /api/v1/shipments` list still accepts client `tenant_id` query (read path).
- `CONSIGNEE_OPERATOR` / `CONSIGNEE_VIEWER` cannot mutate shipments in v0.1.
- Planned v0.2+: map gateway policies to seeded `core.permissions` / `role_permissions`, unify identity role catalog with gateway allowlists.

See also:

- [SHIPMENT_STATUS_HISTORY.md](./SHIPMENT_STATUS_HISTORY.md)
- [SHIPMENT_EVENT_HISTORY.md](./SHIPMENT_EVENT_HISTORY.md)
- [services/shipment-service/README.md](../services/shipment-service/README.md)
