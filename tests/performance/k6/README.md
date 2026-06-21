# Performance tests (k6)

Load and smoke tests for the Freight Platform API Gateway.

## Prerequisites

- Backend running: `make platform-up && make migrate-up && make health-check`
- [k6](https://k6.io/docs/get-started/installation/) installed

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `API_BASE_URL` | `http://localhost:8080` | API Gateway base URL |
| `TENANT_ID` | `74519f22-ff9b-4a8b-8fff-a958c689682f` | Tenant for list endpoints |
| `COMPANY_ID` | — | Optional GET by id in `companies-load.js` |
| `TRANSPORT_ORDER_ID` | — | Optional GET in `transport-orders-load.js` |
| `SHIPMENT_ID` | — | Optional GET in `shipments-load.js` |
| `BILLING_REGISTER_ID` | — | Optional GET in `billing-load.js` |
| `CREATE_DATA` | `false` | Set `true` to POST companies in load test |

## Scripts

| File | VUs | Duration | Purpose |
|------|-----|----------|---------|
| `smoke.js` | 1 | 30s | Health + core list endpoints |
| `companies-load.js` | 10 | 2m | Companies module |
| `transport-orders-load.js` | 10 | 2m | Transport orders |
| `rfx-load.js` | 10 | 2m | RFX events + freight requests |
| `shipments-load.js` | 10 | 2m | Shipments |
| `billing-load.js` | 10 | 2m | Billing registers |
| `full-flow-load.js` | 20 | 5m | Cross-module read flow |

## Make targets

```bash
make performance-smoke
make performance-load
make performance-companies
make performance-transport-orders
make performance-rfx
make performance-shipments
make performance-billing
```

## Shell helpers

```bash
./scripts/performance/run_k6_smoke.sh
./scripts/performance/run_k6_load.sh
```

See [docs/PERFORMANCE.md](../../docs/PERFORMANCE.md) for thresholds and troubleshooting.
