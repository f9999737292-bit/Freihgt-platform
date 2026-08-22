# FREIGHT COST ANALYTICS WORKSPACE v2.1D — Implementation Plan (FROZEN)

**Status:** PLANNING FREEZE — documentation only  
**Branch:** `arch/freight-cost-workspace-v2.1D`  
**Baseline SHA:** `4e1707000fff38df2d6e7af3b849820682121e23` (main after PR #48 merge)  
**PR #48 merge SHA:** `4e1707000fff38df2d6e7af3b849820682121e23`  
**PR #48 feature HEAD:** `e97c8164a1812498b0651cc03ff9f8929b685dee`  
**Date:** 2026-08-22  
**Contract sources:**

- `docs/engineering/FREIGHT_COST_MANAGEMENT_v2.1_ARCHITECTURE.md`
- `docs/engineering/FREIGHT_COST_ACCRUAL_LEDGER_v2.1B_IMPLEMENTATION.md`
- `docs/engineering/FREIGHT_COST_VARIANCE_RECONCILIATION_v2.1C_IMPLEMENTATION_PLAN.md`
- `docs/engineering/FREIGHT_COST_VARIANCE_RECONCILIATION_v2.1C_IMPLEMENTATION.md`
- `docs/engineering/FREIGHT_CONTRACT_RATE_WORKSPACE_v2.0D_IMPLEMENTATION.md` (frontend pattern reference)

---

## 1. Executive Summary

v2.1D delivers the **Cost Analytics Workspace** — a buyer/carrier-aware Nuxt workspace for freight cost visibility — as **UI architecture, navigation, view-model contracts, and rollout design only**. Runtime implementation is gated on v2.1E public API + gateway RBAC.

v2.1A/B/C established the derived cost projection, ledger ingest, variance/forecast semantics, attribution, reconciliation findings, and internal S2S APIs. **No browser-consumable public freight-cost API exists today.** v2.1D must not expose `X-Internal-Service-Token` or call `/internal/v1/freight-cost/*` from the browser.

**Official v2.1D scope:** `apps/web-procurement/**` workspace shell (future slice), typed public API adapter contracts (documented, not wired), i18n key plan, feature flag, buyer/carrier field masks (UX + server contract), money display rules — **planning freeze in this document**.

**Explicit non-scope:** gateway routes, OpenAPI implementation, migrations, freight-cost-service changes, live E2E data wiring.

---

## 2. Baseline SHA / PR #48 Merge

| Field | Value |
|-------|-------|
| Pre-merge main | `db5c0c793a7259bccc4b0c389f3b9e3b23f73a2f` |
| PR #48 HEAD | `e97c8164a1812498b0651cc03ff9f8929b685dee` |
| Post-merge main | `4e1707000fff38df2d6e7af3b849820682121e23` |
| PR #48 pre-merge CI | Run `32595906470` — **PASS** |
| Post-merge main CI | Run `32596428448` — **PASS** |
| v2.1C migrations through | `000060_freight_cost_mapping_evaluated_at_v2.1C` |
| Frozen FC-C tests | 84/84 |
| Bonus remediation tests | 25/25 |

---

## 3. Discovery

### 3.1 Frontend applications

| App | Role | Freight cost relevance |
|-----|------|------------------------|
| **`apps/web-procurement`** | Buyer + carrier procurement, settlements, billing, payments, contract rates | **PRIMARY OWNER** — already hosts settlement/billing/payment workspaces with buyer/carrier actor split |
| `apps/web-admin` | Internal ops (transport orders, shipments, RFx, control tower) | No freight-cost UI; control-tower KPI pattern exists but not cost analytics |
| `apps/web-shipper` | Minimal scaffold | Not suitable |

**Decision:** `FRONTEND_OWNER_APP = apps/web-procurement` — mirror v2.0D Contract Rate Workspace pattern in the same app.

### 3.2 API / gateway state

| Question | Finding |
|----------|---------|
| Public freight-cost gateway routes? | **NOT_FOUND** — no `/api/v1/freight-cost*` in `services/api-gateway` |
| Internal freight-cost routes? | **YES** — S2S only under `/internal/v1/freight-cost/*` with `X-Internal-Service-Token` |
| Browser → freight-cost-service direct? | **FORBIDDEN** — internal auth middleware rejects browser calls |
| Existing frontend composables for freight cost? | **NOT_FOUND** |
| Procurement API pattern | `useApi()` → `runtimeConfig.public.apiBaseUrl` → `/api/v1/*` gateway |

**Required:** `BROWSER_DIRECT_INTERNAL_SERVICE_CALL = NO`

### 3.3 v2.1C internal data available (post-PR #48)

Single-transport-order read (S2S):

```http
GET /internal/v1/freight-cost/transport-orders/{transportOrderId}
```

`CostSummaryResponse` fields (`services/freight-cost-service/internal/http/dto/cost_summary.go`):

- Identity: `tenant_id`, `transport_order_id`, `shipment_id`, `buyer_company_id`, `carrier_company_id`, `currency_code`
- Stage: `data_stage`, `financial_finality`, `sources_available`
- Money (decimal strings, nullable): `planned_amount`, `accrued_amount`, `forecast_exposure`, `current_actual_amount`, `final_actual_amount`, `billing_register_amount`, `paid_amount`, `current_variance_amount`, `final_variance_amount`
- Provenance: `planned_source`
- Reconciliation: `billing_reconciliation_status`

**Not exposed on any public DTO today:** variance drivers, availability reasons, attribution rows, reconciliation findings list, charge-code mappings, aggregate KPIs, lane/carrier rollups.

### 3.4 Buyer/carrier masking (backend — v2.1A)

`services/freight-cost-service/internal/domain/view_scope.go`:

- `BUYER` → full `CostViewScopeBuyerCost`
- `CARRIER` → `CostViewScopeCarrierReceivable` — **nulls:** `accrued_amount`, `forecast_exposure`, `current_variance_amount`, `final_variance_amount`

Carrier may see: planned (context), current/final actual (receivable), billing register, paid — per v2.1A tests. **Frontend hiding alone is insufficient; v2.1E gateway must enforce actor-scoped DTOs.**

### 3.5 Reusable frontend patterns (web-procurement)

| Pattern | Location |
|---------|----------|
| Feature flag | `NUXT_PUBLIC_CONTRACT_RATE_WORKSPACE_ENABLED` → `useContractRateFeature()` |
| Flag middleware | `middleware/contract-rate-workspace.ts` → `/contracts/unavailable` |
| Nav gating | `layouts/default.vue` — `v-if="showContractsNav"` |
| Table + pagination | `components/ui/Table.vue`, client-side `paginateItems` in contract rates |
| Filters | Reactive filter objects + computed filtered lists |
| Money display (legacy) | `utils/format.ts` — **uses JS `number`** — **must not be used for v2.1D financial fields** |
| i18n | `@nuxtjs/i18n`, locales `ru-RU`, `en-US`, `zh-CN`, JSON namespaces |
| Permissions UX | `usePermissions()` — **UX only**, not security boundary |
| Settlement buyer/carrier actor | `utils/settlement.ts` — `resolveSettlementActor` |
| Empty/loading/error | `EmptyState`, toast pattern, `apiUnavailable` ref |

### 3.6 Export / download

**NOT_FOUND** in web-procurement for CSV/XLSX export primitives. v2.1D plans export as **FUTURE** (v2.1E+ API `Accept` header or dedicated export endpoint).

### 3.7 Lane / carrier aggregation dimensions

| Dimension | Source | Aggregate API |
|-----------|--------|---------------|
| `carrier_company_id` | projection row | **NOT_FOUND** — needs v2.1E list/summary |
| Lane (origin/destination) | transport-order rate snapshot locations | **NOT_FOUND** — needs join read model |
| Date period | `cost_updated_at` / order dates | **NOT_FOUND** — needs v2.1E filters |

---

## 4. Existing Frontend Architecture

Mirror v2.0D slice boundary (`FREIGHT_CONTRACT_RATE_WORKSPACE_v2.0D_IMPLEMENTATION.md`):

```text
Nuxt 3 + Vue 3 + Pinia + @nuxtjs/i18n
@freight-platform/ui AppShell
composables/useApi.ts → gateway /api/v1/*
middleware: auth + feature-flag workspace gate
types/*.ts — backend JSON field names, money as decimal strings
tests/*.test.ts — Vitest unit tests (pure helpers, flag gates)
```

v2.1D adds parallel structure under `/freight-costs/*` when implemented — **not in this planning PR**.

---

## 5. Exact v2.1D Scope

| In scope (planning) | Out of scope |
|---------------------|--------------|
| Information architecture + route map | Runtime Vue/Nuxt pages |
| View-model / future public DTO contracts | Gateway route implementation |
| KPI definitions + NULL/currency rules | Migrations |
| Buyer/carrier visibility matrix | Internal service changes |
| Feature flag specification | Live browser ↔ backend integration |
| i18n key inventory | v2.1E RBAC implementation |
| Frontend test matrix (IDs only) | CSV export implementation |
| Money formatting rules (decimal string) | JS float financial arithmetic |

```text
V2_1D_RUNTIME_STARTED = NO
PUBLIC_API_IN_V2_1D = NO
GATEWAY_RBAC_IN_V2_1D = NO
```

---

## 6. v2.1E Boundary

v2.1E owns:

- `GET /api/v1/freight-costs/*` public routes (architecture §35)
- Gateway JWT → `BUYER`/`CARRIER` actor propagation
- Server-side field masking (reuse `ApplyViewScope`)
- OpenAPI + financial E2E
- Server-side pagination/filtering for enterprise volumes

v2.1D **documents** required v2.1E contracts in §20. **Must not** implement adapter calls that bypass gateway.

Architecture target public routes (v2.1E):

```text
GET /api/v1/freight-costs?company_id=&from=&to=&currency=&carrier_id=&...
GET /api/v1/freight-costs/transport-orders/{id}
GET /api/v1/freight-costs/transport-orders/{id}/variance-detail
GET /api/v1/freight-costs/summary
GET /api/v1/freight-costs/variance
GET /api/v1/freight-costs/accessorials/summary
GET /api/v1/freight-costs/carriers/performance
GET /api/v1/freight-costs/lanes/performance
```

No generic PATCH for money — corrections remain in settlement/billing domains.

---

## 7. Information Architecture

```text
Freight Costs (buyer/carrier — flag-gated nav)
├── Overview                    /freight-costs
├── Planned vs Actual           /freight-costs/planned-vs-actual
├── Shipments                   /freight-costs/shipments
│   └── Detail                  /freight-costs/shipments/{transportOrderId}
├── Variance                    /freight-costs/variance
├── Accessorials                /freight-costs/accessorials
├── Carrier Performance         /freight-costs/carriers
├── Lane Performance            /freight-costs/lanes
└── Unavailable (flag off)      /freight-costs/unavailable
```

Nav label namespace: `nav.freightCosts*` — hidden when `NUXT_PUBLIC_FREIGHT_COST_WORKSPACE_ENABLED !== 'true'`.

---

## 8. Screen Inventory

| Screen | Classification | Rationale |
|--------|----------------|-----------|
| **Overview** | `V2_1D_IN_SCOPE` (UI + KPI layout) / `DEFER_V2_1E_DATA_WIRING` | KPI cards need `GET /api/v1/freight-costs/summary` |
| **Planned vs Actual** | `V2_1D_IN_SCOPE` / `DEFER_V2_1E_DATA_WIRING` | Row list needs paginated `GET /api/v1/freight-costs` |
| **Shipment Cost Detail** | `V2_1D_IN_SCOPE` / `DEFER_V2_1E_DATA_WIRING` | Single TO DTO maps to future public detail endpoint |
| **Variance Analysis** | `V2_1D_IN_SCOPE` (buyer-only nav) / `DEFER_V2_1E_DATA_WIRING` | Drivers/attribution need extended public DTO |
| **Accessorial Spend** | `DEFER_V2_1E_DATA_WIRING` | Category aggregation API missing |
| **Accrual Exposure (dedicated)** | `FUTURE` | KPI on Overview sufficient for v2.1D; dedicated drill-down deferred |
| **Carrier Cost Performance** | `DEFER_V2_1E_DATA_WIRING` | Carrier rollup API + dimension validation required |
| **Lane Cost Performance** | `DEFER_V2_1E_DATA_WIRING` | Lane normalization from snapshot — no aggregate read model |

---

## 9. KPI Definitions (Overview)

All KPIs: **single currency per aggregate**; mixed currency → display "unavailable" / per-currency breakdown — **no FX**.

| KPI | Source field(s) | Aggregation | NULL rule | Cancelled orders |
|-----|-----------------|-------------|-----------|------------------|
| Planned Freight Cost | `planned_amount` | SUM where currency = filter | Exclude NULL rows | Exclude |
| Financial Accrual | `accrued_amount` | SUM | Exclude NULL | Exclude |
| Pending Proposed Exposure | `forecast_exposure` | SUM | Exclude NULL; UNKNOWN status → NULL | Exclude |
| Current Actual | `current_actual_amount` | SUM | Exclude NULL | Exclude |
| Final Actual | `final_actual_amount` | SUM | Exclude NULL | Exclude |
| Current Variance | `current_variance_amount` | SUM | Exclude NULL | Exclude |
| Final Variance | `final_variance_amount` | SUM | Exclude NULL | Exclude |
| Billing Mismatch Count | `billing_reconciliation_status != MATCH` | COUNT | NULL status excluded from mismatch count | Exclude |
| Settled Unpaid Exposure | `final_actual_amount - paid_amount` | SUM per row then aggregate | NULL component → exclude row | Exclude |

**Label freeze:**

- `forecast_exposure` → **"Pending proposed exposure"** (RU/EN/ZH i18n keys)
- **FORBIDDEN labels:** "Expected total cost", "Ultimate liability", "Total expected cost"

**Tax basis:** EX-VAT for variance/accrual/actual comparisons (v2.1C freeze). Display VAT-inclusive billing register separately with explicit label.

---

## 10. Table / Detail Models

### 10.1 Planned vs Actual row (`FreightCostOrderRowVM`)

| Field | Type | Notes |
|-------|------|-------|
| `transport_order_id` | UUID string | Link to detail |
| `shipment_id` | UUID string \| null | |
| `order_reference` | string | From transport-order public read (v2.1E join) |
| `carrier_company_id` | UUID | Filter dimension |
| `carrier_name` | string | Resolved client-side or server join |
| `planned_amount` | decimal string \| null | |
| `accrued_amount` | decimal string \| null | Buyer only |
| `forecast_exposure` | decimal string \| null | Buyer only |
| `current_actual_amount` | decimal string \| null | |
| `final_actual_amount` | decimal string \| null | |
| `current_variance_amount` | decimal string \| null | Buyer only |
| `final_variance_amount` | decimal string \| null | Buyer only |
| `currency_code` | string | |
| `financial_finality` | enum | |
| `billing_reconciliation_status` | enum \| null | |
| `availability_summary` | string[] | Human-readable reasons (from backend) |
| `cost_updated_at` | ISO8601 | Staleness indicator |

### 10.2 Shipment Cost Detail (`FreightCostDetailVM`)

Sections:

1. **Summary strip** — same money fields as row
2. **Planned snapshot** — `planned_source` provenance
3. **Accrual breakdown** — buyer only; accrual = planned + approved (display only, no frontend recompute)
4. **Proposed exposure** — buyer only; label "Pending proposed exposure"
5. **Actual / settlement** — current vs final with finality badge
6. **Variance** — buyer only; sign from backend string
7. **Variance drivers** — buyer only; list from v2.1E attribution endpoint
8. **Reconciliation** — buyer only; findings list + `billing_reconciliation_status`
9. **Source provenance** — `sources_available`, `cost_updated_at`

### 10.3 Accessorial spend row (v2.1E)

| Field | Type |
|-------|------|
| `normalized_category` | enum (frozen taxonomy) |
| `total_amount` | decimal string |
| `currency_code` | string |
| `order_count` | int |

Categories: frozen v2.1C mapping vocabulary; `UNKNOWN` / `OTHER` displayed explicitly.

---

## 11. Filters / Pagination

| Filter | Owner | Notes |
|--------|-------|-------|
| Date period | **Server (v2.1E)** | `from`, `to` on `cost_updated_at` or order date |
| Carrier | **Server** | `carrier_company_id` |
| Lane | **Server** | origin/destination codes from snapshot join |
| Order/shipment status | **Server** | transport-order status join |
| Settlement status | **Server** | derived from projection finality |
| Variance state | **Server** | e.g. `HAS_CURRENT_VARIANCE`, `NULL_ACTUAL` |
| Reconciliation state | **Server** | `billing_reconciliation_status` |
| Currency | **Server** | Required before any SUM KPI |
| Free-text search | **Server** | order reference / shipment number |

Default page size: **20** (match contract rates). Client-side filtering **only** for purely presentational sorts on already-fetched page — not for financial filtering at scale.

---

## 12. Money Formatting

| Rule | Value |
|------|-------|
| Wire format | Decimal string (`"1234.56"`) |
| Display | Locale-aware formatting via dedicated helper `formatDecimalMoney(amount: string \| null, currency: string, locale)` |
| JS `Number()` for money | **PROHIBITED** for storage, comparison, sum |
| Comparison / sort on money | Use decimal library (e.g. `decimal.js`) if client-side sort unavoidable — prefer server sort |
| Percent | Backend-provided decimal string; display with `%` suffix |
| NULL | Render em-dash / `freightCosts.unavailable` — **never `0`** |
| Mixed currency aggregate | Show `freightCosts.mixedCurrencyUnavailable` |

Existing `utils/format.ts` `formatMoney(number)` is **legacy settlement UI only** — do not reuse for freight cost workspace.

---

## 13. NULL / Currency Semantics

```text
NULL = unknown / not yet available / not calculated
0    = known zero (explicit backend zero string "0.00")
NULL_IS_ZERO = NO
FX_CONVERSION = NO
FRONTEND_FLOAT_MONEY_CALCULATION = NO
```

UI must surface `availability_summary` / `forecast_source_status=UNKNOWN` rather than inferring zero.

---

## 14. Buyer / Carrier Visibility

### 14.1 Field matrix (frozen)

| Field / section | Buyer authorized | Carrier |
|-----------------|------------------|---------|
| Planned cost | YES | YES (context) |
| Financial accrual | YES | **NO** |
| Pending proposed exposure | YES | **NO** |
| Current/final actual | YES | YES (receivable view) |
| Billing register / paid | YES | YES (where applicable) |
| Current/final variance | YES | **NO** |
| Variance drivers / attribution | YES | **NO** |
| Reconciliation findings | YES | **NO** |
| Accessorial category analytics (buyer internal) | YES | **NO** |
| Carrier performance (cross-order) | YES (buyer) | Own receivable subset only (v2.1E) |

### 14.2 Enforcement

```text
FRONTEND_MASKING_IS_UX_ONLY = YES
BACKEND_ENFORCEMENT_OWNER = v2.1E gateway + freight-cost ApplyViewScope
CARRIER_CANNOT_SEE_BUYER_INTERNAL_ANALYTICS = YES
```

Variance Analysis nav item: **buyer roles only** at UX level; carrier route returns 403 from API.

---

## 15. Feature Flag

| Setting | Value |
|---------|-------|
| Env var | `NUXT_PUBLIC_FREIGHT_COST_WORKSPACE_ENABLED` |
| `runtimeConfig.public.freightCostWorkspaceEnabled` | `true` only when `=== 'true'` |
| Default | **`false`** |
| Nav visibility | Hidden when disabled |
| Direct route access | Middleware → `/freight-costs/unavailable` |
| Authorization | **Not** a substitute for RBAC — flag is rollout control only |

Pattern reference: `useContractRateFeature`, `middleware/contract-rate-workspace.ts`.

---

## 16. i18n (RU / EN / ZH)

New namespace files (future implementation):

```text
i18n/{ru-RU,en-US,zh-CN}/freightCosts.json
```

Key families:

| Prefix | Content |
|--------|---------|
| `freightCosts.nav.*` | Workspace navigation |
| `freightCosts.kpi.*` | Overview KPI labels including pending proposed exposure |
| `freightCosts.finality.*` | `NOT_EVALUATED`, `DRAFT`, `CURRENT_ACTUAL`, `FINAL_ACTUAL`, `CANCELLED` |
| `freightCosts.reconciliation.*` | `MATCH`, `MISMATCH`, `UNLINKED` |
| `freightCosts.variance.*` | Sign labels, driver types |
| `freightCosts.categories.*` | Frozen charge categories + UNKNOWN/OTHER |
| `freightCosts.filters.*` | Filter labels |
| `freightCosts.empty.*` | Empty states per screen |
| `freightCosts.errors.*` | API failure, forbidden, mixed currency |
| `freightCosts.unavailable.*` | NULL money, flag-off page |

Register in `nuxt.config.ts` `localeFiles` array when implementing.

---

## 17. Loading / Error / Empty States

Per-screen state machine (mirror settlements/contracts):

| State | UX |
|-------|-----|
| `loading` | Skeleton or table spinner |
| `empty` | `EmptyState` with screen-specific i18n |
| `apiUnavailable` | Gateway/network failure toast + retry |
| `forbidden` | 403 — role insufficient |
| `missingCompany` | No tenant company context |
| `mixedCurrency` | KPI card shows unavailable helper |
| `staleProjection` | Optional banner when `cost_updated_at` older than threshold (display only) |

---

## 18. Accessibility

- Nav: `aria-label="Freight cost navigation"`
- KPI cards: `role="group"` with labelled headings
- Tables: `<th scope="col">`, sort buttons with `aria-sort`
- NULL money: `aria-label` explaining unavailable — not silent blank
- Focus management on route change within workspace
- Locale switcher already in AppShell — reuse

---

## 19. Observability (frontend)

- Log analytics page views without money payloads
- Client error boundary for API failures — error code + correlation id only
- Optional `cost_updated_at` displayed for projection freshness (no PII in metrics)

Backend metrics already defined in architecture §42 — no v2.1D changes.

---

## 20. Future API DTO Requirements (v2.1E)

### 20.1 `FreightCostSummaryDTO` (public — per TO)

Align with internal `CostSummaryResponse` + extensions:

```json
{
  "transport_order_id": "uuid",
  "shipment_id": "uuid|null",
  "buyer_company_id": "uuid",
  "carrier_company_id": "uuid",
  "currency_code": "RUB",
  "data_stage": "PLANNED_ONLY",
  "financial_finality": "CURRENT_ACTUAL",
  "sources_available": ["PLANNED", "ACCRUAL"],
  "planned_amount": "1000.00",
  "accrued_amount": "1050.00",
  "forecast_exposure": "1100.00",
  "forecast_source_status": "KNOWN",
  "current_actual_amount": "1050.00",
  "final_actual_amount": null,
  "billing_register_amount": null,
  "paid_amount": null,
  "current_variance_amount": "50.00",
  "final_variance_amount": null,
  "billing_reconciliation_status": "MATCH",
  "cost_updated_at": "2026-08-22T12:00:00Z",
  "availability_reasons": ["FINAL_ACTUAL_PENDING_SETTLEMENT"]
}
```

Carrier-scoped responses **omit** buyer-internal fields server-side (null/absent — not error).

### 20.2 `FreightCostSummaryAggregateDTO`

```json
{
  "currency_code": "RUB",
  "period": { "from": "...", "to": "..." },
  "kpis": {
    "planned_total": "100000.00",
    "accrued_total": "95000.00",
    "forecast_exposure_total": "5000.00",
    "current_actual_total": "90000.00",
    "final_actual_total": "85000.00",
    "current_variance_total": "-10000.00",
    "final_variance_total": "-15000.00",
    "reconciliation_mismatch_count": 3
  },
  "mixed_currency": false
}
```

### 20.3 List endpoints

- Cursor or offset pagination
- Stable sort: `cost_updated_at DESC`, tie-break `transport_order_id`
- Filter query params as §11

### 20.4 Detail extensions

- `GET .../transport-orders/{id}/variance-detail` — drivers, attribution categories, reconciliation findings (buyer only)

---

## 21. Frontend Test Matrix (FROZEN — planning IDs)

**Not implemented in v2.1D planning PR.** Vitest + future Playwright.

| Family | IDs | Coverage |
|--------|-----|----------|
| FC-D-NAV | 001–006 | Flag off hides nav; flag on shows nav; unavailable route; buyer vs carrier nav items |
| FC-D-FLAG | 001–003 | Default off; middleware redirect; env parsing |
| FC-D-OVR | 001–012 | KPI render; NULL not zero; mixed currency card; mismatch count; forecast label |
| FC-D-PVA | 001–010 | Row decimal display; buyer columns; carrier masked columns; sort pagination |
| FC-D-DET | 001–015 | Detail sections; provenance; reconciliation badge; driver list buyer-only |
| FC-D-ACC | 001–006 | Category taxonomy display; UNKNOWN/OTHER |
| FC-D-CAR | 001–005 | Carrier performance table; buyer aggregate |
| FC-D-LAN | 001–005 | Lane dimension display |
| FC-D-FLT | 001–010 | Filter chip UI; server param mapping (mocked) |
| FC-D-MON | 001–008 | `formatDecimalMoney` locales; null; zero string |
| FC-D-I18N | 001–006 | RU/EN/ZH key presence for KPIs and forecast label |
| FC-D-SEC | 001–010 | Carrier API 403; masked fields absent; no internal token in client bundle |
| FC-D-ERR | 001–006 | Loading, empty, API failure |

Total planned: **~82** test IDs across families.

---

## 22. E2E Boundary

| Gate | Value |
|------|-------|
| Live browser ↔ freight-cost E2E | **v2.1E** |
| v2.1D implementation slice | UI + mocked adapter unit tests only |
| Postgres integration | Not applicable to frontend slice |
| Financial correctness proofs | Remain FC-C / FC-B backend suites |

---

## 23. Rollout

1. Merge v2.1E public API + gateway RBAC
2. Implement v2.1D UI behind `NUXT_PUBLIC_FREIGHT_COST_WORKSPACE_ENABLED=false`
3. Internal QA with flag on in staging
4. Per-tenant enable via env/config — not authorization
5. Carrier roles: receivable-only views enabled only after v2.1E mask verified

---

## 24. Acceptance Gates (implementation slice — future)

| Gate | Criterion |
|------|-----------|
| G-D-001 | Feature flag default OFF; nav hidden |
| G-D-002 | No internal service token in frontend bundle |
| G-D-003 | All money from API decimal strings; no float sum |
| G-D-004 | NULL displays unavailable — never zero substitute |
| G-D-005 | Forecast labeled pending proposed exposure |
| G-D-006 | Carrier session cannot fetch buyer variance (API 403 + UI mask) |
| G-D-007 | i18n RU/EN/ZH complete for frozen keys |
| G-D-008 | FC-D-* unit tests pass |
| G-D-009 | web-procurement build CI green |
| G-D-010 | No freight-cost-service / gateway changes in v2.1D PR |

---

## 25. Explicit Deferred Work

| Item | Phase |
|------|-------|
| Public API routes | v2.1E |
| Gateway RBAC + actor headers | v2.1E |
| Live data wiring | v2.1E |
| Aggregate carrier/lane APIs | v2.1E |
| CSV/XLSX export | Future |
| Accrual exposure dedicated screen | Future |
| Cross-tenant analytics | Out of scope |
| FX / multi-currency rollup | Out of scope |
| Frontend float money refactor for legacy settlements | Separate hygiene |

---

## 26. Frozen Decisions

| Decision | Value |
|----------|-------|
| `FRONTEND_OWNER_APP` | `apps/web-procurement` |
| `FEATURE_FLAG` | `NUXT_PUBLIC_FREIGHT_COST_WORKSPACE_ENABLED` |
| `FEATURE_FLAG_DEFAULT` | `false` |
| `BROWSER_DIRECT_INTERNAL_SERVICE_CALL` | **NO** |
| `PUBLIC_API_IN_V2_1D` | **NO** |
| `GATEWAY_RBAC_IN_V2_1D` | **NO** |
| `V2_1D_RUNTIME_STARTED` | **NO** (this PR is planning only) |
| Money wire type | Decimal string |
| NULL semantics | NULL ≠ zero |
| FX | Not in scope |
| Forecast label | Pending proposed exposure |
| Buyer/carrier mask | Backend v2.1E enforces; frontend UX duplicate |
| Pattern reference | v2.0D Contract Rate Workspace |
| Baseline | PR #48 merged @ `4e17070` |

---

## Appendix A — v2.1D Implementation File Plan (future — not this PR)

When runtime begins (post-v2.1E):

| Artifact | Path |
|----------|------|
| Feature composable | `composables/useFreightCostFeature.ts` |
| API adapter | `composables/useFreightCostsApi.ts` |
| Types | `types/freightCost.ts` |
| Money helper | `utils/freightCostMoney.ts` |
| Permissions | `utils/freightCostPermissions.ts` |
| Middleware | `middleware/freight-cost-workspace.ts` |
| Pages | `pages/freight-costs/**` |
| i18n | `i18n/*/freightCosts.json` |
| Tests | `tests/freightCost*.test.ts` |

---

## Appendix B — Related Documents

| Document | Purpose |
|----------|---------|
| `FREIGHT_COST_MANAGEMENT_v2.1_ARCHITECTURE.md` | Phase map §46, KPI §37, workspace §36 |
| `FREIGHT_COST_VARIANCE_RECONCILIATION_v2.1C_IMPLEMENTATION.md` | Runtime field semantics |
| `FREIGHT_CONTRACT_RATE_WORKSPACE_v2.0D_IMPLEMENTATION.md` | Frontend slice pattern |
