# Freight Cost Analytics — Controlled 100K Verification

Deterministic synthetic generator and integration perf gate live in:

`services/freight-cost-service/internal/integration/analytics/performance_100k_integration_test.go`

## Run locally

```powershell
$env:TEST_DATABASE_URL = "postgres://rfx_test:rfx_test@localhost:5432/freight_test?sslmode=disable"
$env:PERF_100K = "1"
cd D:\Projects\freight-platform\services\freight-cost-service
go test -tags=integration ./internal/integration/analytics/... -run TestFC22G1_PERF001 -count=1 -timeout 30m -v
```

## Generator parameters

| Parameter | Value |
|-----------|-------|
| Seed | `220001` |
| Tenant | `11111111-1111-4111-8111-111111110001` |
| Orders | 100,000 |
| Buyers | 1 |
| Carriers | 2 (deterministic UUIDs) |
| Currencies | RUB + EUR (every 50th order EUR) |
| Canonical source | `freight_cost.cost_summary_projection` bulk insert |
| Rebuild path | `AnalyticsProjectionService.RebuildTenant` |

No committed SQL dumps.

## Browser E2E

```powershell
$env:TEST_DATABASE_URL = "postgres://rfx_test:rfx_test@localhost:5432/freight_test?sslmode=disable"
$env:BROWSER_E2E = "1"
cd D:\Projects\freight-platform\services\freight-cost-service
go test -tags=integration ./internal/integration/analytics/... -run TestFC22G1_BrowserE2E -count=1 -timeout 20m -v
```

Playwright specs: `apps/web-procurement/e2e/freight-cost-intelligence/`
