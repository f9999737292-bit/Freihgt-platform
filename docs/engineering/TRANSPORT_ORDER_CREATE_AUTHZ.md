# Transport Order Create Authorization (Wave 2)

## Route

`POST /api/v1/transport-orders` — priced transport order creation (v2.0C pricing path).

## Request chain

```text
JWT validation (api-gateway auth middleware)
→ strip spoofed identity headers (X-Tenant-ID, X-User-ID, X-Company-ID, X-Actor-Kind)
→ verified AuthContext (tenant, user, bearer token, requested company hint)
→ GET identity /users/{id}/companies (membership lookup using verified bearer)
→ derive ActorKind server-side from company type + membership roles
→ transport-order RBAC policy (PolicyCreate)
→ reverse proxy to transport-order-service with trusted headers only
```

Downstream `transport-order-service` requires trusted:

- `X-Tenant-ID` (from JWT)
- `X-User-ID` (from JWT `sub`)
- `X-Company-ID` (verified membership)
- `X-Actor-Kind` (derived server-side; never trusted from client)

## Allowed actor / company combinations

| Company type | Derived actor kind | Allowed company roles | Wave 2 path |
|--------------|-------------------|----------------------|-------------|
| `SHIPPER` | `BUYER` | `SHIPPER_ADMIN`, `PROCUREMENT_MANAGER`, `FORWARDER_MANAGER` | Primary buyer/shipper path |
| `FORWARDER` | `BUYER` | `FORWARDER_MANAGER` | Forwarder-as-buyer |
| Any | `BUYER` | tenant `PLATFORM_ADMIN` (via `/users/{id}/roles`) | Platform override |

**Denied:**

- `CARRIER` companies (`ActorKind=CARRIER`) — carriers cannot create transport orders
- Membership not containing requested `X-Company-ID`
- Company roles outside the create policy (e.g. `SHIPPER_LOGIST` alone)
- Missing `X-Company-ID` on request (400 validation)

Policies are defined in `services/api-gateway/internal/transportorderrbac/policies.go` and registered in `services/api-gateway/internal/http/router.go` **before** the generic `/api/*` catch-all proxy.

## Canonical roles (repository)

Aligned with RFx buyer-manage and contract-rate mutate paths:

- `PLATFORM_ADMIN` (tenant role)
- `PROCUREMENT_MANAGER`
- `SHIPPER_ADMIN`
- `FORWARDER_MANAGER`

## HTTP semantics

| Scenario | Response |
|----------|----------|
| Missing/invalid JWT | `401 Unauthorized` |
| Missing `X-Company-ID` | `400 Validation` |
| Valid JWT, company not in membership | `403 Forbidden` |
| Valid JWT, carrier actor kind | `403 Forbidden` |
| Valid JWT, role not allowed | `403 Forbidden` |
| Spoofed `X-Company-ID` / `X-Actor-Kind` | Ignored; verified values injected |
| Allowed buyer context | Proxy invoked with trusted headers |

## Related Wave 2 bootstrap

After priced transport order creation:

`POST /api/v1/freight-requests/from-transport-order` (RFx buyer-manage policy) creates the freight request for RFx publish/bid/award flow.
