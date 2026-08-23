# FREIGHT COST INTELLIGENCE v2.2G — Performance Report

**Status:** Integration-scale certified; synthetic 100k deferred  
**Date:** 2026-08-23

---

## 1. Executive summary

v2.2G proves **no N+1 enrichment** on the analytics rebuild path at integration scale (120 orders), validates **500-ID batch chunking** for company display enrichment, documents **tenant-scoped indexes** from v2.2 migrations, and enforces **bounded public API pagination** (max limit 100). Full 100k-order tenant load testing is deferred to a controlled environment; CI runs integration-scale gates only.

---

## 2. N+1 proof (120 orders)

**Test:** `TestFC22G_NPlusOne001EnrichmentUsesBatchNotPerOrder`  
**File:** `services/freight-cost-service/internal/integration/analytics/n_plus_one_integration_test.go`

| Parameter | Value |
|-----------|-------|
| Order count | **120** (`nPlusOneOrderCount`) |
| Operation | Full `RebuildTenant` with counting wrappers on company, dimension, settlement providers |
| Pass criteria | `company_calls`, `dimension_calls`, `settlement_calls` each **< 120** (strictly batch, not per-order) |
| ID coverage | Batch must still resolve ≥120 dimension/settlement IDs |

Instrumented providers: `counting_providers.go` in same package.

**Result:** PASS — rebuild issues O(1) batch HTTP/DB calls per provider type regardless of order count at this scale.

---

## 3. Batch size 500

**Constant:** `batchSize = 500` in `services/freight-cost-service/internal/client/company/client.go`

**Test:** `TestFC22G_BatchGetCompanyDisplayChunksAt500`  
**File:** `services/freight-cost-service/internal/client/company/client_batch_test.go`

| Input | Expected HTTP calls |
|-------|---------------------|
| 750 company IDs | **2** batches (500 + 250) |
| 0 IDs | 0 calls (no-op) |

Enrichment rebuild path uses `BatchGetCompanyDisplay` — never per-order company HTTP.

---

## 4. Index and query notes (migrations)

All analytics projections are **tenant-scoped** with composite indexes supporting period + company filters.

### 000061 — v2.2B projection core

| Index | Table | Columns |
|-------|-------|---------|
| `idx_cost_analytics_order_fact_tenant_period` | `cost_analytics_order_fact` | `(tenant_id, buyer_company_id, period_start, period_grain, currency_code)` |
| `idx_cost_analytics_period_tenant_calculated` | `cost_analytics_period_projection` | `(tenant_id, calculated_at DESC)` |
| `idx_analytics_projection_dirty_poll` | `analytics_dirty_queue` | dirty poll for incremental worker |

### 000062 — v2.2C lane/carrier

| Index | Table | Columns |
|-------|-------|---------|
| `idx_cost_analytics_order_fact_lane_period` | `cost_analytics_order_fact` | lane aggregation support |
| `idx_cost_analytics_order_fact_carrier_period` | `cost_analytics_order_fact` | carrier aggregation support |
| `idx_lane_period_tenant_calculated` | `cost_analytics_lane_period_projection` | `(tenant_id, calculated_at DESC)` |
| `idx_carrier_period_tenant_calculated` | `cost_analytics_carrier_period_projection` | `(tenant_id, calculated_at DESC)` |

### 000063 — v2.2D accessorial

| Index | Table | Columns |
|-------|-------|---------|
| `idx_accessorial_fact_tenant_period` | accessorial fact | tenant + period reads |
| `idx_accessorial_fact_tenant_order` | accessorial fact | per-order enrichment |
| `idx_accessorial_period_tenant_calculated` | accessorial period projection | freshness queries |

### 000064 — v2.2E benchmark/opportunity

| Index | Table | Columns |
|-------|-------|---------|
| `idx_benchmark_tenant_period` | `benchmark_projection` | tenant period listing |
| `idx_benchmark_tenant_lane` | `benchmark_projection` | lane cohort lookup |
| `idx_opportunity_tenant_type` | `opportunity_projection` | type filter |
| `idx_opportunity_tenant_period` | `opportunity_projection` | period filter |

**Query pattern:** Public API reads filter by `(tenant_id, buyer_company_id, period_start, currency_code)` — aligned with PK/index design. No cross-tenant scans.

**Migration gate:** `TestFC22G_MigrationGateV22UpDown` validates 000061–000064 up/down reversibility.

---

## 5. Public API bounds

| Control | Value | Source |
|---------|-------|--------|
| Default limit | 20 | `analyticsDefaultLimit` |
| Max limit | **100** | `analyticsMaxLimit` in `analytics_public_service.go` |
| Over-max behavior | Silently capped to 100 | `TestFC22G_ParseAnalyticsPublicQueryPaginationAbuse` |
| Unbounded query | **Not possible** — limit always applied server-side | `ParseAnalyticsPublicQuery` |

Date range: default rolling 90 days; max ~24 months (validated in service).

---

## 6. Controlled 100K verification (v2.2G.1)

**Status:** Harness implemented; execution requires `PERF_100K=1` on disposable Postgres (not run in default CI job).

| Item | Value |
|------|-------|
| Test ID | `FC22G1-PERF-001` |
| Test function | `TestFC22G1_PERF001_100kAnalyticsRebuild` |
| File | `services/freight-cost-service/internal/integration/analytics/performance_100k_integration_test.go` |
| Generator seed | `220001` |
| Tenant | `11111111-1111-4111-8111-111111110001` |
| Order count | **100,000** |
| Canonical source | Bulk insert into `cost_summary_projection` |
| Rebuild path | `RebuildTenant` (full analytics service) |
| Public API timing | In-test warm-up + 10 iterations (overview, lanes 20/100, carriers 20/100, accessorials, opportunities) |
| EXPLAIN | `FC22G1-PERF-003` — `EXPLAIN (ANALYZE, BUFFERS)` on lane/carrier/accessorial/opportunity/benchmark/period queries |
| Runbook | `tests/performance/freight-cost-analytics/README.md` |

```powershell
$env:TEST_DATABASE_URL = "postgres://rfx_test:rfx_test@localhost:5432/freight_test?sslmode=disable"
$env:PERF_100K = "1"
go test -tags=integration ./internal/integration/analytics/... -run TestFC22G1_PERF001 -count=1 -timeout 30m -v
```

**Verdict rule:** `CONTROLLED_100K_VERIFICATION=PASS` only after successful harness run records rebuild duration and bounded public queries. This is **not** a production SLA certification.

Previous §6 “100k deferred” wording is superseded by this harness; actual numbers are recorded at execution time in CI/controlled run logs.

---

## 7. Worker throughput configuration

| Env | Default | Role |
|-----|---------|------|
| `FREIGHT_COST_ANALYTICS_DIRTY_BATCH_SIZE` | 50 | Incremental dirty processing |
| `FREIGHT_COST_ANALYTICS_DIRTY_POLL_INTERVAL` | 5s | Poll frequency |
| `FREIGHT_COST_ANALYTICS_REBUILD_INTERVAL` | 24h | Scheduled full tenant rebuild |

Concurrent rebuilds for same tenant serialize via `pg_advisory_xact_lock` — see `REBUILD_RUNBOOK.md`.

---

## 8. Metrics (observability)

Prometheus counters/histograms (`internal/platform/metrics/metrics.go`):

- `freight_cost_analytics_rebuild_total` / `_duration_seconds`
- `freight_cost_analytics_benchmark_rebuild_failures_total`

No tenant/company/lane/carrier labels on commercial metrics (ADR compliance).

**References:** `TEST_INVENTORY.md` (FC22G-N+1-001, FC22G-BATCH-500), `REBUILD_RUNBOOK.md`.
