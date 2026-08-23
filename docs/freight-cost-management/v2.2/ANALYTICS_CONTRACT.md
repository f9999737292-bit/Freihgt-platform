# FREIGHT COST INTELLIGENCE v2.2 — Analytics Contract (Design Freeze)

**Status:** Contract design only — **routes NOT implemented in v2.2A**  
**Namespace:** `/api/v1/freight-costs/*` (unchanged)  
**Date:** 2026-08-23

---

## 1. Principles

1. **Server-side aggregation** — clients must not aggregate unbounded ledger rows.
2. **Explicit absence** — `NOT_AVAILABLE`, `INSUFFICIENT_SAMPLE`, `MIXED_CURRENCY` are distinct from zero/empty.
3. **Currency safety** — no cross-currency sums without FX; group by `currency_code` or return `MIXED_CURRENCY`.
4. **Tenant isolation** — every query scoped by JWT tenant; company scope via `X-Company-ID` membership.
5. **Freshness transparency** — all aggregate responses include projection metadata.
6. **No N+1 enrichment** — display names and lane labels come from projection snapshots.

---

## 2. Proposed Public Endpoints (v2.2F)

All routes require existing freight-cost feature flag + RBAC. **Not implemented in v2.2A.**

| Method | Path | RBAC Policy | Description |
|--------|------|-------------|-------------|
| GET | `/api/v1/freight-costs/analytics/overview` | PolicyBuyerAnalytics | Period KPIs + top drivers |
| GET | `/api/v1/freight-costs/analytics/lanes` | PolicyBuyerAnalytics | Lane table with benchmark |
| GET | `/api/v1/freight-costs/analytics/carriers` | PolicyBuyerAnalytics | Carrier performance |
| GET | `/api/v1/freight-costs/analytics/accessorials` | PolicyBuyerAnalytics | Accessorial breakdown |
| GET | `/api/v1/freight-costs/opportunities` | PolicyBuyerAnalytics | Explainable savings |

**Carrier-scoped variant (future):** separate handler path or query flag returning receivable-scoped subset — carriers **never** receive buyer benchmark or opportunity endpoints.

Existing workspace routes remain:

- `GET /api/v1/freight-costs` — list summaries
- `GET /api/v1/freight-costs/aggregate` — workspace KPIs
- `GET /api/v1/freight-costs/lanes` — currently `data_capability: NOT_AVAILABLE`
- `GET /api/v1/freight-costs/carriers/performance`
- `GET /api/v1/freight-costs/accessorials/summary` — currently `NOT_AVAILABLE`

---

## 3. Common Request Parameters

| Parameter | Type | Required | Rules |
|-----------|------|----------|-------|
| `from` | date (ISO 8601) | No | Inclusive start; default rolling 90d |
| `to` | date | No | Inclusive end; max span 24 months |
| `currency` | string (ISO 4217) | No | Filter; if omitted and multi-currency → `MIXED_CURRENCY` on totals |
| `date_dimension` | enum | No | `COST_EFFECTIVE` (default), `PICKUP`, `DELIVERY` |
| `limit` | int | No | Default 20, max 100 |
| `offset` | int | No | Default 0 |
| `sort` | string | No | Allowlist per endpoint |
| `transport_mode` | string | No | Filter |
| `equipment_type` | string | No | Filter |
| `carrier_company_id` | uuid | No | Filter (buyer only) |
| `lane_key` | string | No | Filter |

### Headers (unchanged from v2.1E)

- `Authorization` — JWT
- `X-Tenant-ID` — must match JWT tenant
- `X-Company-ID` — active company membership
- `X-Request-ID`, `X-Locale` — platform standard

---

## 4. Common Response Envelope

```json
{
  "currency_code": "RUB",
  "period": { "from": "2026-05-01", "to": "2026-08-01", "date_dimension": "COST_EFFECTIVE" },
  "data_quality": "AVAILABLE",
  "mixed_currency": false,
  "freshness": {
    "calculated_at": "2026-08-23T12:00:00Z",
    "data_through": "2026-08-23T11:55:00Z",
    "projection_version": 3
  },
  "items": [],
  "total": 0,
  "limit": 20,
  "offset": 0
}
```

### data_quality enum

| Value | When |
|-------|------|
| `AVAILABLE` | Data complete for requested scope |
| `PARTIAL` | Some orders missing dimensions (e.g. null weight) — metrics include `coverage_ratio` |
| `NOT_AVAILABLE` | Capability not implemented or source absent |
| `INSUFFICIENT_SAMPLE` | Cohort below min sample for benchmark/opportunity |
| `STALE` | Projection older than configured threshold |
| `MIXED_CURRENCY` | Multiple currencies without explicit `currency` filter |

---

## 5. Money Representation

Reuse v2.1 workspace convention:

- Amounts as **decimal strings** with 2 fractional digits (e.g. `"12345.67"`)
- Always paired with `currency_code`
- Null amount = unavailable (not zero)

```json
{
  "amount": "1500.00",
  "currency_code": "RUB"
}
```

---

## 6. Lane Semantics

### lane_key (canonical)

Deterministic string:

```text
{origin_country}:{origin_city}->{dest_country}:{dest_city}|{transport_mode}|{equipment_type_or_WILD}
```

Example: `RU:MOSCOW->RU:SPB|ROAD|TENT`

### LaneDTO (proposed)

| Field | Type | Notes |
|-------|------|-------|
| `lane_key` | string | Canonical key |
| `lane_label` | string | Human label e.g. "Moscow → Saint Petersburg (ROAD)" |
| `origin_country` | string | ISO 3166-1 alpha-2 |
| `origin_city` | string | Normalized |
| `destination_country` | string | |
| `destination_city` | string | |
| `transport_mode` | string | |
| `equipment_type` | string? | nullable |
| `order_count` | int | Sample size |
| `spend_total` | MoneyDTO | |
| `planned_total` | MoneyDTO? | |
| `median_cost` | MoneyDTO? | `NOT_AVAILABLE` if insufficient sample |
| `benchmark_median` | MoneyDTO? | Tenant cohort median |
| `benchmark_delta` | MoneyDTO? | observed median − cohort median |
| `variance_total` | MoneyDTO? | |
| `accessorial_rate` | string? | decimal ratio 0–1 |
| `carrier_count` | int | |
| `data_quality` | enum | |
| `sample_size` | int | Same as order_count for lane row |

**Directional:** `A→B` ≠ `B→A`.

---

## 7. Carrier Semantics

| Field | Type | Carrier visible |
|-------|------|-----------------|
| `carrier_company_id` | uuid | YES (own id only) |
| `carrier_name` | string | YES (snapshot) |
| `order_count` | int | YES (own orders) |
| `spend_total` | MoneyDTO | **NO** for buyer total; carrier sees `receivable_total` only |
| `median_cost` | MoneyDTO | Buyer only |
| `benchmark_delta` | MoneyDTO | Buyer only |
| `lane_count` | int | Buyer only |
| `accessorial_rate` | string | Scoped |
| `reconciliation_rate` | string | Buyer only |

---

## 8. Accessorial Semantics

| Field | Type | Notes |
|-------|------|-------|
| `normalized_category` | string | From pinned mapping (FUEL, DETENTION, …) |
| `charge_code` | string? | Raw code optional drill-down |
| `total_amount` | MoneyDTO | APPROVED lines default |
| `order_count` | int | Distinct transport orders |
| `share_of_spend` | string | Ratio within filtered scope |
| `carrier_company_id` | uuid? | Optional dimension |
| `lane_key` | string? | Optional dimension |

Default filter: `status=APPROVED`. Query param `include_proposed=true` adds PROPOSED with `data_quality=PARTIAL`.

---

## 9. Benchmark Semantics

| Rule | Value |
|------|-------|
| Scope | **Tenant-only** |
| Cohort | `(tenant_id, lane_key, transport_mode, equipment_type, currency_code, period)` |
| Metrics | count, mean, median, p25, p75, p90, min, max |
| Min sample | Configurable domain policy `freight_cost.analytics.min_benchmark_sample` (default **5**) |
| Below min | Return `data_quality=INSUFFICIENT_SAMPLE`; omit percentile fields |
| Cross-tenant | **Prohibited** |

### Why median / percentiles

Freight rates are skewed by outliers (spot loads, detention spikes). Mean alone mis-ranks lanes and carriers. Median and inter-quartile bands match industry benchmarking practice and align with explainable savings rules.

---

## 10. Savings Opportunity Semantics

Rule-based, explainable only. **No** percentage savings without evidence.

### OpportunityDTO

| Field | Type | Required |
|-------|------|----------|
| `opportunity_id` | uuid | YES |
| `type` | enum | YES |
| `scope` | enum (`LANE`, `CARRIER`, `ORDER`, `ACCESSORIAL`) | YES |
| `entity_key` | string | YES |
| `observed_value` | MoneyDTO | YES |
| `baseline_value` | MoneyDTO | YES |
| `estimated_delta` | MoneyDTO | YES (= observed − baseline) |
| `currency_code` | string | YES |
| `sample_size` | int | YES |
| `evidence` | object | YES — human-readable + machine keys |
| `data_quality` | enum | YES |
| `calculated_at` | timestamp | YES |

### Supported types (v2.2E)

| Type | CLASS | Rule |
|------|-------|------|
| `LANE_COST_OUTLIER` | B | Order cost > P90 lane cohort, sample ≥ min |
| `CARRIER_COST_OUTLIER` | B | Carrier lane-normalized median > tenant median + threshold |
| `COST_ABOVE_LANE_MEDIAN` | B | observed > cohort median, delta reported |
| `HIGH_ACCESSORIAL_RATE` | B | accessorial_rate > tenant P75 |
| `REPEATED_VARIANCE` | A | ≥ N orders same carrier/lane with same variance driver |
| `CLASSIFICATION_ANOMALY` | B | Unmapped charge_code rate above threshold |

**Not included (CLASS C):** ML predictions, market benchmark savings, FX arbitrage.

### Example evidence

```json
{
  "observed_cost": "45000.00",
  "cohort_median": "38000.00",
  "potential_delta": "7000.00",
  "sample_size": 12,
  "currency_code": "RUB",
  "cohort": "RU:MOSCOW->RU:SPB|ROAD|TENT|2026-Q2"
}
```

---

## 11. Overview Endpoint Contract

### GET `/api/v1/freight-costs/analytics/overview`

**Response sections (all optional based on readiness):**

| Section | CLASS | Fields |
|---------|-------|--------|
| Spend summary | A | planned, actual, variance totals |
| Top cost drivers | A/B | variance driver categories |
| Top lanes | B | top 5 by spend |
| Accessorial spend | B | total + rate |
| Savings opportunities | B | count + top 3 |
| Reconciliation | A | mismatch count |

---

## 12. Pagination & Sorting

### Pagination

- `limit` default 20, max 100
- Non-positive `limit` → 400
- `total` always returned for list endpoints

### Sort allowlist

| Endpoint | Allowed `sort` values |
|----------|----------------------|
| lanes | `spend_total`, `order_count`, `median_cost`, `variance_total`, `lane_label` |
| carriers | `spend_total`, `order_count`, `median_cost`, `accessorial_rate` |
| accessorials | `total_amount`, `order_count`, `share_of_spend`, `normalized_category` |
| opportunities | `estimated_delta`, `calculated_at`, `type` |

Prefix `-` for descending. Unknown sort → 400.

---

## 13. Error Responses

Reuse platform error envelope. Analytics-specific codes:

| Code | HTTP | Meaning |
|------|------|---------|
| `ANALYTICS_NOT_AVAILABLE` | 503 | Projection not built / feature off |
| `INSUFFICIENT_SAMPLE` | 200 | Valid request but benchmark omitted (also in body) |
| `MIXED_CURRENCY` | 200 | Totals omitted; per-currency breakdown provided |
| `PROJECTION_STALE` | 200 | Data returned with `data_quality=STALE` |

---

## 14. Dimension Snapshot Policy

```
DIMENSION_SNAPSHOT_POLICY=
  Store order_reference, carrier_display_name, lane_label on order_fact_projection at build time.
  Refresh on scheduled rebuild (daily) or order-scoped incremental update.
  Financial joins always use UUID keys; labels may change on rebuild (rename semantics documented).
  Historical financial amounts never change on rename — only display labels.
```

---

## 15. Forecast Readiness (Contract Stub)

No forecast endpoint in v2.2. Assessment fields for future:

| Prerequisite | Status |
|--------------|--------|
| Historical depth ≥ 12 months | UNKNOWN (tenant-dependent) |
| Seasonality signal | NOT_AVAILABLE (no seasonality model) |
| Planned future orders | PARTIAL (TO draft/assigned exist) |
| Lane stability | PARTIAL |
| Currency consistency | Enforced by policy |

`FORECAST_READY=PARTIAL` — blocked for runtime.

---

## 16. Versioning

- API version: `/api/v1/` unchanged
- `projection_version` in responses for analytics rebuild compatibility
- Breaking DTO changes require OpenAPI bump + FC contract tests (pattern from v2.1E)

---

## 17. OpenAPI

OpenAPI stubs for analytics routes will be added in **v2.2F** to `packages/openapi/freight-cost-service.yaml` via `scripts/openapi/generate_openapi.py`. **Not added in v2.2A.**
