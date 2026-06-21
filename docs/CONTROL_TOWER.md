# Control Tower Dashboard v0.1

Operational command center for the 7Rights Freight Platform operator console (`web-admin`).

## Purpose

Control Tower aggregates key logistics metrics across companies, users, transport orders, tenders (RFx / freight requests), shipments, documents, and billing registers. It is designed for platform operators who need a single screen to monitor operational health, funnel progress, risks, and quick navigation to module actions.

The dashboard reads data from existing API Gateway endpoints only. No new backend services, Redis, or Kafka are required.

## Modules covered

| Area | Source module | Route |
|------|---------------|-------|
| Companies | company-service | `/companies` |
| Users | identity-service | `/users` |
| Transport orders | transport-order-service | `/transport-orders` |
| RFx events | rfx-service | `/rfx` |
| Freight requests (mini tenders) | rfx-service | `/freight-requests` |
| Shipments | shipment-service | `/shipments` |
| Documents | document-service | `/documents` |
| Billing registers | billing-register-service | `/billing-registers` |
| Platform health | api-gateway + observability | `/health`, Prometheus, Grafana |

## APIs used

All requests go through API Gateway (`NUXT_PUBLIC_API_BASE_URL`, default `http://localhost:8080`).

| Endpoint | Usage |
|----------|--------|
| `GET /api/v1/companies` | KPI, operations status, recent activity |
| `GET /api/v1/users` | KPI, operations status |
| `GET /api/v1/transport-orders` | KPI, transport funnel, recent activity |
| `GET /api/v1/rfx-events` | KPI (active RFx), tender funnel, recent activity |
| `GET /api/v1/freight-requests` | KPI, tender funnel |
| `GET /api/v1/shipments` | KPI, transport funnel, shipment board, risks |
| `GET /api/v1/documents` | KPI, documents summary, risks |
| `GET /api/v1/billing-registers` | KPI, billing summary, revenue total, risks |

`tenant_id` is taken from the tenant store (`useTenantStore`). If not set, the dev fallback tenant `74519f22-ff9b-4a8b-8fff-a958c689682f` is used for queries.

Data loading uses `Promise.allSettled` — each endpoint is independent. Partial API failures show `—` and `unavailable` badges without breaking the page.

> **TODO:** Global bids list endpoint is not wired yet. Bids KPI and tender funnel bid steps show `0` until a list API is available.

## Risks and alerts

Control Tower evaluates best-effort operational risks:

- API Gateway / service availability (companies, shipments, billing)
- Shipments in driver-required statuses without `driver_id`
- Shipments in vehicle-required statuses without `vehicle_id`
- Delivered shipments without linked documents
- Shipments in `READY_FOR_BILLING` not yet in a billing register
- Billing registers `APPROVED` but not signed
- Billing registers `SIGNED_BY_COUNTERPARTY` but not paid

When no risks are found: **No critical risks detected**.

## Open the dashboard

1. Start backend (when Docker is available):

   ```bash
   make platform-up
   make migrate-up
   ```

2. Start web-admin:

   ```bash
   make run-web-admin
   # or
   cd apps/web-admin && npm run dev
   ```

3. Log in with a valid tenant and open:

   **http://localhost:3000/control-tower**

Navigation: sidebar item **Control Tower** (below Dashboard).

## If data is not shown

1. **API Gateway** — check `http://localhost:8080/health` or web-admin `/health`
2. **Tenant** — ensure `tenant_id` is set after login (Settings or login form)
3. **Auth token** — re-login if requests return 401; for local dev `AUTH_ENABLED=false` may be used in docker-compose
4. **Backend services** — verify containers are healthy: `make platform-ps`
5. **Partial outage** — Control Tower still renders; unavailable modules show fallback state

## Frontend structure

```
apps/web-admin/
├── pages/control-tower/index.vue
├── composables/useControlTower.ts
├── types/controlTower.ts
└── components/control-tower/
    ├── KpiCard.vue
    ├── StatusBadge.vue
    ├── OperationsStatusTable.vue
    ├── FunnelBoard.vue
    ├── ShipmentStatusBoard.vue
    ├── DocumentsBillingStatus.vue
    ├── RiskAlerts.vue
    ├── QuickActions.vue
    └── RecentActivity.vue
```

## Related docs

- [TROUBLESHOOTING.md](./TROUBLESHOOTING.md)
- [OBSERVABILITY.md](./OBSERVABILITY.md)
- [DOCKER_DISK_TROUBLESHOOTING.md](./DOCKER_DISK_TROUBLESHOOTING.md)
