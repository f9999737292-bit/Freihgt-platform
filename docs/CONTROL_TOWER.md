# Control Tower Dashboard v0.1

Operational command center for the 7Rights Freight Platform operator console (`web-admin`).

## Purpose

Control Tower aggregates logistics KPIs, active shipments, SLA heuristics, critical events, and filter options for dispatchers and logistics managers.

From v0.1.1 the primary data source is a single BFF endpoint in API Gateway:

```http
GET /api/v1/control-tower/summary
```

Legacy multi-endpoint client aggregation remains as a development fallback when the summary API is disabled or returns `404 Not Found`.

## Architecture

```text
web-admin
   └── GET /api/v1/control-tower/summary
           └── api-gateway (BFF aggregation)
                 ├── shipment-service      (required)
                 ├── transport-order-service (optional)
                 ├── company-service       (optional)
                 └── document-service      (optional)
```

The gateway does **not** query downstream PostgreSQL directly. All data is fetched through existing service HTTP APIs using the caller JWT / tenant headers.

Implementation package: `services/api-gateway/internal/controltower/`

## Data sources

| Dependency | Endpoint | Role |
|------------|----------|------|
| Shipments | `GET /v1/shipments` | Required. KPI, rows, SLA, events |
| Transport orders | `GET /v1/transport-orders` | Order numbers for search/display |
| Companies | `GET /v1/companies` | Shipper/carrier names and filter options |
| Documents | `GET /v1/documents` | Document completeness and missing-doc events |

If shipments cannot be loaded the endpoint returns `503 Service Unavailable` with error code `CONTROL_TOWER_SHIPMENTS_UNAVAILABLE`. Other dependencies may fail partially with `200 OK` and `dataFreshness.partial=true`.

## Tenant isolation

- Tenant is taken only from verified auth context populated by gateway JWT middleware.
- Incoming client headers such as `X-Tenant-ID`, `X-User-ID`, `X-User-Email`, `X-Company-ID`, and `X-User-Roles` are stripped before JWT validation and must not be trusted by Control Tower handlers.
- The summary endpoint does **not** accept `tenant_id` or `company_id` query parameters.
- Downstream list calls receive tenant/user/request IDs written by the gateway after authentication.

## Security invariants

1. Tenant is determined from verified auth context (JWT claims stored in server-side `AuthContext`), not from client-supplied identity headers.
2. Incoming `X-Tenant-ID` is **not** a trusted input for Control Tower; spoofed values are removed by auth middleware when `AUTH_ENABLED=true`.
3. Query parameter `tenant_id` is not supported and is ignored for authorization and downstream fetches.
4. Downstream identity headers (`X-Tenant-ID`, `X-User-ID`, `Authorization`, `X-Request-ID`) are formed by the gateway from verified context.
5. RBAC uses a minimal operational role allowlist (see RBAC section below); finance/procurement roles are excluded by default.
6. When `AUTH_ENABLED=false`, only the documented development policy applies: tenant comes from `DEV_TENANT_ID` configuration, never from client headers.
7. Required shipment dependency failure returns endpoint-specific `503` with code `CONTROL_TOWER_SHIPMENTS_UNAVAILABLE`.
8. Global reverse proxy upstream failures continue to use `502 Bad Gateway` via `SERVICE_UNAVAILABLE`.

## RBAC

When `AUTH_ENABLED=true`, the handler calls identity-service `GET /v1/auth/me` because JWT access tokens in this project do not embed role claims. Roles are checked against the minimal operational allowlist:

- `PLATFORM_ADMIN`
- `CARRIER_DISPATCHER`
- `SHIPPER_ADMIN`
- `SHIPPER_LOGIST`
- `FORWARDER_MANAGER`

`FINANCE_MANAGER` and `PROCUREMENT_MANAGER` are intentionally excluded from automatic Control Tower access.

Responses:

- No authentication → `401`
- Authenticated but no allowed role → `403`
- Identity dependency unavailable → `503` (`AUTH_DEPENDENCY_UNAVAILABLE`)

When `AUTH_ENABLED=false` (local development), RBAC checks are skipped. Tenant must be configured via `DEV_TENANT_ID`.

## SLA rules v0.1

**Important:** SLA calculator v0.1 is an operational heuristic and is **not** a contractual SLA Rules Engine.

Priority (highest wins):

```text
CRITICAL > DELAYED > AT_RISK > ON_TIME > UNKNOWN
```

Statuses:

| Status | Meaning |
|--------|---------|
| `UNKNOWN` | Missing planned dates, unknown shipment status, insufficient data |
| `ON_TIME` | Active shipment on plan, or completed within planned delivery |
| `AT_RISK` | Deadline within threshold, or stale updates (warning level) |
| `DELAYED` | Planned pickup/delivery overdue below critical threshold, or non-critical late completion |
| `CRITICAL` | Critical delay, cancelled shipment, stale critical threshold, technical problem |

Stale updates apply only to in-progress statuses (pre-pickup and in-transit groups).

## SLA reason codes

Machine-readable `slaReason` values returned by the API:

- `MISSING_PLANNED_DATES`
- `ON_SCHEDULE`
- `PICKUP_AT_RISK`
- `PICKUP_OVERDUE`
- `DELIVERY_AT_RISK`
- `DELIVERY_OVERDUE`
- `STALE_UPDATES`
- `CANCELLED`
- `TECHNICAL_PROBLEM`
- `COMPLETED_ON_TIME`
- `COMPLETED_LATE`
- `UNKNOWN_STATUS`

## Threshold configuration

API Gateway environment variables (defaults work without `.env` changes):

| Variable | Default | Description |
|----------|---------|-------------|
| `CONTROL_TOWER_AT_RISK_MINUTES` | `120` | Minutes before deadline to mark AT_RISK |
| `CONTROL_TOWER_CRITICAL_DELAY_MINUTES` | `240` | Overdue minutes before CRITICAL |
| `CONTROL_TOWER_STALE_WARNING_MINUTES` | `120` | Stale data AT_RISK threshold |
| `CONTROL_TOWER_STALE_CRITICAL_MINUTES` | `360` | Stale data CRITICAL threshold |

## Critical events v0.1

Derived events with deterministic IDs (`sha256(shipmentId:eventType:timestamp)` prefix):

| Type | Severity |
|------|----------|
| `PICKUP_DELAY` | WARNING / CRITICAL |
| `DELIVERY_DELAY` | WARNING / CRITICAL |
| `STALE_UPDATES` | WARNING / CRITICAL |
| `MISSING_DOCUMENTS` | WARNING |
| `SHIPMENT_CANCELLED` | CRITICAL |
| `TECHNICAL_PROBLEM` | CRITICAL |
| `UNKNOWN_CRITICAL_EVENT` | CRITICAL |

## Partial response

When optional dependencies fail, the API returns `200 OK` with:

```json
{
  "dataFreshness": {
    "partial": true,
    "warnings": ["COMPANIES_UNAVAILABLE"]
  }
}
```

Safe warning codes:

- `TRANSPORT_ORDERS_UNAVAILABLE`
- `COMPANIES_UNAVAILABLE`
- `DOCUMENTS_UNAVAILABLE`
- `KPI_CALCULATED_FROM_LIMITED_DATASET`
- `FILTER_OPTIONS_INCOMPLETE`

Internal exception text is never exposed in warnings.

## Pagination and filtering

Query parameters:

| Parameter | Description |
|-----------|-------------|
| `q` | Search shipment / order / company names |
| `status` | Shipment status |
| `sla_status` | SLA status enum |
| `shipper_id` | Shipper company UUID |
| `carrier_id` | Carrier company UUID |
| `date_from` / `date_to` | `YYYY-MM-DD` range |
| `critical_only` | Boolean |
| `page` | Default `1` |
| `limit` | Default `50`, max `200` |

KPI counts reflect the **filtered** dataset, not only the current page.

Downstream list APIs are capped at 200 records per request. When total shipments exceed fetched rows, `KPI_CALCULATED_FROM_LIMITED_DATASET` is included.

Some SLA and text search filters are applied in the gateway after downstream fetch because shipment list APIs do not yet expose all filter dimensions.

## Frontend feature flag

```bash
NUXT_PUBLIC_CONTROL_TOWER_SUMMARY_API_ENABLED=true
```

Default: enabled. Set to `false` to force legacy multi-fetch behaviour.

Runtime config key: `runtimeConfig.public.controlTowerSummaryApiEnabled`

## Legacy fallback

Legacy client-side aggregation (`useControlTower` multi-fetch + `controlTowerLogic.ts`) is retained when:

- feature flag is disabled, **or**
- development mode and summary endpoint returns `404 Not Found`

Fallback is **not** used for `503 Service Unavailable` — the UI shows backend unavailable state.

Demo synthetic data is shown only in development when core APIs are unavailable (legacy path).

## Production SLA Rules Engine (future)

Contractual SLA requires:

- Configurable per-tenant / per-contract rules
- Persistent SLA breach records and audit trail
- Integration with legal/commercial SLA definitions
- Escalation workflows and notifications
- Versioned rule sets with effective dates

Current v0.1 calculator must not be used for billing disputes or contractual enforcement.

## GPS / geolocation events (future)

Real-time Control Tower events require:

- GPS/telematics ingestion pipeline
- Geofence definitions for pickup/delivery locations
- Route deviation detection against planned route
- Event stream (Kafka or equivalent) for `NO_GEOLOCATION` / `ROUTE_DEVIATION`
- Map visualization layer in web-admin

v0.1 does not call external GPS providers.

## OpenAPI

Documented in `packages/openapi/openapi.yaml` under tag **Control Tower**.

## Related frontend files

- `apps/web-admin/composables/useControlTower.ts`
- `apps/web-admin/types/controlTower.ts`
- `apps/web-admin/pages/control-tower/index.vue`
- `apps/web-admin/utils/controlTowerLogic.ts` (legacy fallback heuristics)
