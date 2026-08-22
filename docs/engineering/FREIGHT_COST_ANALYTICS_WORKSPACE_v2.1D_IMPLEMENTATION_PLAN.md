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

v2.1D delivers the **Cost Analytics Workspace** — a buyer/carrier-aware Nuxt workspace for freight cost visibility. This planning PR freezes architecture, contracts, and semantics only.

v2.1A/B/C established the derived cost projection, ledger ingest, variance/forecast semantics, attribution, reconciliation findings, and internal S2S APIs. **No browser-consumable public freight-cost API exists today.** v2.1D must not expose `X-Internal-Service-Token` or call `/internal/v1/freight-cost/*` from the browser.

**Phase split (R49-005):**

| Phase | Owns |
|-------|------|
| **v2.1D runtime** | Feature-flagged workspace shell, routes/navigation, typed frontend models, money display helpers, RU/EN/ZH, loading/empty/error states, buyer/carrier UX masks, **mocked** API adapter tests |
| **v2.1E** | Public `/api/v1/freight-cost*` routes, strict DTOs, gateway RBAC, server-side masking, server-side filtering/pagination, **live** adapter wiring, browser-to-gateway financial E2E |

```text
V2_1D_CAN_IMPLEMENT_BEFORE_V2_1E=YES
V2_1D_LIVE_DATA_WIRING=NO
V2_1D_BROWSER_INTERNAL_API=NO
V2_1E_LIVE_DATA_OWNER=YES
```

**Official v2.1D scope (this PR):** planning freeze — IA, view-model contracts, KPI semantics, test matrix, rollout order. **Explicit non-scope:** gateway routes, OpenAPI implementation, migrations, freight-cost-service changes, live E2E data wiring.

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
| `apps/web-finance` | Skeleton portal (`pages/index.vue` only) | **REJECTED** — no finance workspace pattern, no stronger ownership case |
| `apps/web-shipper` | Minimal scaffold | Not suitable |

**Decision:** `FRONTEND_OWNER_APP = apps/web-procurement` — mirror v2.0D Contract Rate Workspace pattern in the same app.

#### Frontend owner freeze

| Gate | Value |
|------|-------|
| `WEB_PROCUREMENT_OWNER` | **YES** |
| `WEB_FINANCE_OWNER` | **NO** |
| `WEB_FINANCE_REJECTED_REASON` | Freight-cost workspace continues the existing procurement settlement / billing / payment / contract-rate workflows; no stronger ownership case exists for `apps/web-finance` |
| New app creation | **PROHIBITED** |
| App migration / move | **PROHIBITED** |

Do not create or move apps. v2.1D implementation stays under `apps/web-procurement/**` only.

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

**Not exposed on any public DTO today:** variance percent fields (present in projection domain only), variance drivers, availability reasons, attribution rows, reconciliation findings list, charge-code mappings, aggregate KPIs, lane/carrier rollups.

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
| KPI cards (design reference) | `apps/web-admin/components/control-tower/MetricCard.vue` — **reference only**; typed to `ControlTowerKpiMetric`; not a web-procurement dependency |
| Table + pagination | `components/ui/Table.vue`, client-side `paginateItems` in contract rates |
| Filters | Reactive filter objects + computed filtered lists |
| Money display (legacy) | `utils/format.ts` — **uses JS `number`** — **must not be used for v2.1D financial fields** |
| i18n | `@nuxtjs/i18n`, locales `ru-RU`, `en-US`, `zh-CN`, JSON namespaces |
| Permissions UX | `usePermissions()` — **UX only**, not security boundary |
| Settlement buyer/carrier actor | `utils/settlement.ts` — `resolveSettlementActor` |
| Empty/loading/error | `EmptyState`, toast pattern, `apiUnavailable` ref |

#### KPI component reuse freeze (R49-004)

`apps/web-admin/components/control-tower/MetricCard.vue` is **app-private** and typed to `ControlTowerKpiMetric`. It is **not** a shared web-procurement dependency.

| Rule | Value |
|------|-------|
| `WEB_ADMIN_METRIC_CARD_DIRECT_IMPORT` | **NO** |
| `METRIC_CARD_REFERENCE_ONLY` | **YES** — use admin `MetricCard` as a **design reference** for layout/a11y only |
| `CROSS_APP_COMPONENT_IMPORT` | **NO** — do not import files from `apps/web-admin/**` |
| Implementation priority | (1) reuse `@freight-platform/ui` shared KPI card if suitable; (2) else reuse web-procurement generic local `Card`/equivalent; (3) else smallest generic component required **within web-procurement** |
| `FreightCostMetricCard` branding-only wrapper | **PROHIBITED** |
| `NEW_KPI_COMPONENT_REQUIRES_DOCUMENTED_GAP` | **YES** |
| Shared UI refactor in planning PR | **PROHIBITED** |

Do not introduce a new `FreightCostMetricCard` merely for branding. Do not move/refactor shared UI in this planning PR.

### 3.6 Export / download

**NOT_FOUND** in web-procurement for CSV/XLSX export primitives. v2.1D plans export as **FUTURE** (v2.1E+ API `Accept` header or dedicated export endpoint).

### 3.7 Lane / carrier aggregation dimensions

| Dimension | Source | Aggregate API |
|-----------|--------|---------------|
| `carrier_company_id` | projection row | **NOT_FOUND** — needs v2.1E list/summary |
| Lane (origin/destination) | transport-order rate snapshot locations | **NOT_FOUND** — needs join read model |
| Date period (business) | transport-order / settlement business dates | **NOT_FOUND** — v2.1E must define canonical dimension; `cost_updated_at` is projection freshness only |

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

| In scope (planning + future v2.1D runtime) | Out of scope |
|--------------------------------------------|--------------|
| Information architecture + route map | Gateway route implementation |
| View-model / future public DTO contracts | Migrations |
| KPI definitions + NULL/currency rules | Internal service changes |
| Buyer/carrier visibility matrix (UX) | Live browser ↔ backend integration |
| Feature flag specification | v2.1E RBAC / server-side masking implementation |
| i18n key inventory | CSV export implementation |
| Frontend test matrix (IDs + ownership) | JS float financial arithmetic |
| Money formatting rules (decimal string) | Cross-tax-basis KPI invention |
| **Future v2.1D runtime:** UI shell, mocked adapters, Vitest | **v2.1E:** live wiring + financial E2E |

```text
V2_1D_RUNTIME_STARTED = NO   (planning PR only)
V2_1D_CAN_IMPLEMENT_BEFORE_V2_1E = YES
V2_1D_LIVE_DATA_WIRING = NO
PUBLIC_API_IN_V2_1D = NO
GATEWAY_RBAC_IN_V2_1D = NO
V2_1E_LIVE_DATA_OWNER = YES
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
| Planned + Proposed Exposure | `forecast_exposure` | SUM | Exclude NULL; UNKNOWN status → NULL | Exclude |
| Current Actual | `current_actual_amount` | SUM | Exclude NULL | Exclude |
| Final Actual | `final_actual_amount` | SUM | Exclude NULL | Exclude |
| Current Variance | `current_variance_amount` | SUM | Exclude NULL | Exclude |
| Final Variance | `final_variance_amount` | SUM | Exclude NULL | Exclude |
| Billing Mismatch Count | `billing_reconciliation_status != MATCH` | COUNT | NULL status excluded from mismatch count | Exclude |

**Removed (R49-001):** ~~Settled Unpaid Exposure~~ — `final_actual_amount - paid_amount` is **forbidden** because operands use incompatible tax bases (`final_actual_amount` = settlement EX-VAT; `paid_amount` = payable/VAT-inclusive billing register basis).

```text
SETTLED_UNPAID_EXPOSURE_KPI=DEFERRED
CROSS_TAX_BASIS_SUBTRACTION=DENY
```

If a future unpaid-exposure KPI is required, v2.1E must expose a **server-derived** amount whose operands share the same tax basis. Do not invent the formula in v2.1D.

### Forecast exposure semantics (R49-002)

Backend formula (v2.1C freeze):

```text
FORECAST_EXPOSURE_FORMULA=PLANNED + SUM(PROPOSED accessorials EX_VAT)
FORECAST_EXPOSURE_TOTAL=SUM(backend forecast_exposure per eligible row)
```

`forecast_exposure` is **not** merely the incremental proposed-accessorial amount — it includes planned principal plus pending proposed accessorial exposure.

| Concept | Field | UI label (EN example) | i18n key |
|---------|-------|----------------------|----------|
| Full forecast exposure | `forecast_exposure` | **"Planned + proposed exposure"** | `freightCosts.kpi.plannedPlusProposedExposure` |
| Incremental proposed only (optional future KPI) | `pending_proposed_accessorial_amount` / `pending_proposed_accessorial_total` | Separate label TBD in v2.1E | v2.1E contract — **backend-derived decimal string** |

**FORBIDDEN labels (any field):** "Expected total cost", "Ultimate liability", "Total expected cost", "Pending proposed exposure" (when displaying `forecast_exposure` itself — ambiguous/misleading).

Do not derive `pending_proposed_accessorial_total` in Vue using `Number` arithmetic.

**Tax basis:** EX-VAT for variance/accrual/actual/forecast comparisons (v2.1C freeze). Display VAT-inclusive billing register and paid amounts separately with explicit labels — never subtract across bases.

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
4. **Planned + proposed exposure** — buyer only; label `freightCosts.kpi.plannedPlusProposedExposure` (displays `forecast_exposure` wire value)
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
| Date period (business) | **Server (v2.1E contract)** | UI may define date-filter control shape in v2.1D; v2.1E must select a **single canonical business date dimension** or expose explicit `date_dimension` parameter — do not silently use `cost_updated_at` as business/accounting period |
| Projection freshness | Display only | `cost_updated_at` — staleness banner; **not** a business-period filter |
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
| Planned + proposed exposure (`forecast_exposure`) | YES | **NO** |
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
| `freightCosts.kpi.*` | Overview KPI labels including planned + proposed exposure |
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
  "current_variance_percent": "5.00",
  "final_variance_percent": null,
  "billing_reconciliation_status": "MATCH",
  "cost_updated_at": "2026-08-22T12:00:00Z",
  "availability_reasons": ["FINAL_ACTUAL_PENDING_SETTLEMENT"]
}
```

Carrier-scoped responses **omit** buyer-internal fields server-side (null/absent — not error).

**Variance percent fields (v2.1E addition):**

| Field | Type | Notes |
|-------|------|-------|
| `current_variance_percent` | decimal string \| null | Backend-derived; buyer-only |
| `final_variance_percent` | decimal string \| null | Backend-derived; buyer-only |

```text
VARIANCE_PERCENT_CALCULATED_BY_BACKEND=YES
FRONTEND_VARIANCE_PERCENT_ARITHMETIC=NO
JS_NUMBER_FINANCIAL_CALCULATION=NO
```

Percent values are computed by freight-cost-service (same rules as v2.1C domain variance percent). The frontend displays the wire string with locale-aware formatting only — no `(variance / planned) * 100` in Vue/JS.

### 20.2 `FreightCostSummaryAggregateDTO`

```json
{
  "currency_code": "RUB",
  "period": { "from": "...", "to": "...", "date_dimension": "TRANSPORT_ORDER_CREATED_AT" },
  "kpis": {
    "planned_total": "100000.00",
    "accrued_total": "95000.00",
    "forecast_exposure_total": "105000.00",
    "pending_proposed_accessorial_total": "5000.00",
    "current_actual_total": "90000.00",
    "final_actual_total": "85000.00",
    "current_variance_total": "-10000.00",
    "final_variance_total": "-15000.00",
    "reconciliation_mismatch_count": 3
  },
  "mixed_currency": false
}
```

**Aggregate consistency (R49-002):** `forecast_exposure_total` = `SUM(forecast_exposure)` where each row's `forecast_exposure` already embeds planned principal. Example: if `planned_total = 100000.00` and aggregate pending proposed increment = `5000.00`, then `forecast_exposure_total = 105000.00` (not `5000.00`). `pending_proposed_accessorial_total` is an optional **separate** v2.1E KPI for incremental proposed exposure only — backend-derived; not computed in Vue.

### 20.3 List endpoints

- Cursor or offset pagination
- Stable sort: `cost_updated_at DESC`, tie-break `transport_order_id`
- Filter query params as §11

### 20.4 Detail extensions

- `GET .../transport-orders/{id}/variance-detail` — drivers, attribution categories, reconciliation findings (buyer only)

---

## 21. Frontend Test Matrix (FROZEN — planning IDs)

**Not implemented in v2.1D planning PR.** Vitest (mocked adapters) + v2.1E Playwright/financial E2E.

### 21.1 Ownership rules (R49-003)

| Owner | Scope |
|-------|-------|
| `V2_1D_FRONTEND` | UI rendering, feature flag, navigation, mocked adapter responses, UX masks, bundle static analysis, decimal formatting helpers |
| `V2_1E_BACKEND_E2E` | Live API authorization (403), server-side DTO masking, gateway RBAC, cross-tenant deny, browser-to-gateway financial E2E |

v2.1D **may** test: buyer-only controls hidden for carrier actor; mocked 403 handling; buyer fields not rendered when absent from mocked DTO; no internal service token in bundle.

v2.1D **must not** claim: live carrier 403 from protected buyer endpoint; server-side mask enforcement; gateway RBAC — those belong to v2.1E.

### 21.2 Family inventory

| Family | IDs | Count | Owner | Coverage |
|--------|-----|------:|-------|----------|
| FC-D-NAV | 001–006 | 6 | V2_1D_FRONTEND | Flag off hides nav; flag on shows nav; unavailable route; buyer vs carrier nav items |
| FC-D-FLAG | 001–003 | 3 | V2_1D_FRONTEND | Default off; middleware redirect; env parsing |
| FC-D-OVR | 001–012 | 12 | V2_1D_FRONTEND | KPI render; NULL not zero; mixed currency card; mismatch count; planned+proposed label |
| FC-D-PVA | 001–010 | 10 | V2_1D_FRONTEND | Row decimal display; buyer columns; carrier masked columns; sort pagination (mocked page) |
| FC-D-DET | 001–015 | 15 | V2_1D_FRONTEND | Detail sections; provenance; reconciliation badge; driver list buyer-only (mocked) |
| FC-D-ACC | 001–006 | 6 | V2_1D_FRONTEND | Category taxonomy display; UNKNOWN/OTHER |
| FC-D-CAR | 001–005 | 5 | V2_1D_FRONTEND | Carrier performance table layout; buyer aggregate (mocked fixtures) |
| FC-D-LAN | 001–005 | 5 | V2_1D_FRONTEND | Lane dimension display (mocked fixtures) |
| FC-D-FLT | 001–010 | 10 | V2_1D_FRONTEND | Filter chip UI; server param mapping (mocked adapter contract) |
| FC-D-MON | 001–008 | 8 | V2_1D_FRONTEND | `formatDecimalMoney` locales; null; zero string |
| FC-D-I18N | 001–006 | 6 | V2_1D_FRONTEND | RU/EN/ZH key presence for KPIs and forecast label |
| FC-D-SEC | 001–005 | 5 | V2_1D_FRONTEND | UX mask; mocked 403; absent-field render; no internal token in bundle; flag/nav security UX |
| FC-D-SEC | 006–010 | 5 | V2_1E_BACKEND_E2E | Live carrier 403; server DTO mask; gateway RBAC; cross-tenant deny; carrier response field absence |
| FC-D-ERR | 001–006 | 6 | V2_1D_FRONTEND | Loading, empty, API failure (mocked) |

### 21.3 Frozen totals

```text
ALL_TEST_IDS_UNIQUE=YES
TOTAL_TEST_IDS=102
V2_1D_TEST_TOTAL=97
V2_1E_DEFERRED_SECURITY_TEST_TOTAL=5
```

No approximate totals in this frozen plan.

---

## 22. E2E Boundary

| Gate | Value |
|------|-------|
| Live browser ↔ freight-cost E2E | **v2.1E** |
| v2.1D implementation slice | UI shell + mocked adapter unit tests (`V2_1D_TEST_TOTAL=97`) |
| v2.1E security/financial E2E | Live API tests (`V2_1E_DEFERRED_SECURITY_TEST_TOTAL=5` + broader E2E suite) |
| Postgres integration | Not applicable to frontend slice |
| Financial correctness proofs | Remain FC-C / FC-B backend suites |

---

## 23. Rollout (R49-005)

Recommended execution order:

1. **Merge v2.1D planning** (this PR #49)
2. **Implement v2.1D UI shell** behind `NUXT_PUBLIC_FREIGHT_COST_WORKSPACE_ENABLED=false` — routes, i18n, mocked adapters, UX masks; **no live data**
3. **Merge/review v2.1D UI** implementation PR(s)
4. **Plan/implement v2.1E** public API + gateway RBAC + server-side masking/filtering
5. **Wire live data** + financial E2E; enable flag in staging only after all gates pass
6. **Per-tenant production enable** via env/config — rollout control, not authorization

```text
Do not expose internal S2S APIs to browser at any stage.
V2_1D_LIVE_DATA_WIRING=NO
V2_1E_LIVE_DATA_OWNER=YES
```

---

## 24. Acceptance Gates (implementation slice — future)

| Gate | Criterion |
|------|-----------|
| G-D-001 | Feature flag default OFF; nav hidden |
| G-D-002 | No internal service token in frontend bundle |
| G-D-003 | All money from API decimal strings; no float sum |
| G-D-004 | NULL displays unavailable — never zero substitute |
| G-D-005 | Forecast KPI labeled planned + proposed exposure (not ambiguous incremental label) |
| G-D-006 | Carrier UX hides buyer-internal fields; live API 403 verified in v2.1E only |
| G-D-007 | i18n RU/EN/ZH complete for frozen keys |
| G-D-008 | FC-D-* unit tests pass |
| G-D-009 | web-procurement build CI green |
| G-D-010 | No freight-cost-service / gateway changes in v2.1D PR |

---

## 25. Explicit Deferred Work

| Item | Phase |
|------|-------|
| Settled Unpaid Exposure KPI | **DEFERRED** — cross-tax-basis subtraction forbidden (R49-001) |
| `pending_proposed_accessorial_total` aggregate KPI | v2.1E — optional separate field |
| Business date filter contract (`date_dimension`) | v2.1E |
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

### R49 independent review closure

| ID | Resolution |
|----|------------|
| R49-001 | Removed Settled Unpaid Exposure KPI; `CROSS_TAX_BASIS_SUBTRACTION=DENY` |
| R49-002 | Separated `forecast_exposure` (planned+proposed) from incremental proposed KPI; corrected aggregate DTO example |
| R49-003 | Exact test totals: 102 unique IDs; 97 v2.1D / 5 v2.1E security |
| R49-004 | Admin MetricCard reference-only; no cross-app import |
| R49-005 | v2.1D UI may precede v2.1E; live wiring owned by v2.1E |
| R49-006 | Business date filter deferred to v2.1E contract; `cost_updated_at` = freshness only |

### Core gates

| Decision | Value |
|----------|-------|
| `FRONTEND_OWNER_APP` | `apps/web-procurement` |
| `WEB_PROCUREMENT_OWNER` | **YES** |
| `WEB_FINANCE_OWNER` | **NO** |
| `SETTLED_UNPAID_EXPOSURE_KPI` | **DEFERRED** |
| `CROSS_TAX_BASIS_SUBTRACTION` | **DENY** |
| `FORECAST_EXPOSURE_FORMULA` | `PLANNED + SUM(PROPOSED accessorials EX_VAT)` |
| `FORECAST_EXPOSURE_UI_LABEL` | Planned + proposed exposure (`freightCosts.kpi.plannedPlusProposedExposure`) |
| `INCREMENTAL_PROPOSED_KPI` | `pending_proposed_accessorial_total` — v2.1E backend-derived; optional |
| `WEB_ADMIN_METRIC_CARD_DIRECT_IMPORT` | **NO** |
| `METRIC_CARD_REFERENCE_ONLY` | **YES** |
| `CROSS_APP_COMPONENT_IMPORT` | **NO** |
| `NEW_KPI_COMPONENT_REQUIRES_DOCUMENTED_GAP` | **YES** |
| `V2_1D_CAN_IMPLEMENT_BEFORE_V2_1E` | **YES** |
| `V2_1D_LIVE_DATA_WIRING` | **NO** |
| `V2_1D_BROWSER_INTERNAL_API` | **NO** |
| `V2_1E_LIVE_DATA_OWNER` | **YES** |
| `DATE_FILTER_BUSINESS_DIMENSION` | **DEFER_V2_1E_CONTRACT** |
| `V2_1D_TEST_TOTAL` | **97** |
| `V2_1E_DEFERRED_SECURITY_TEST_TOTAL` | **5** |
| `ALL_TEST_IDS_UNIQUE` | **YES** |
| `VARIANCE_PERCENT_CALCULATED_BY_BACKEND` | **YES** |
| `FRONTEND_VARIANCE_PERCENT_ARITHMETIC` | **NO** |
| `JS_NUMBER_FINANCIAL_CALCULATION` | **NO** |
| `NULL_IS_ZERO` | **NO** |
| `FX_CONVERSION` | **NO** |
| `MIXED_CURRENCY_SUM` | **DENY** |
| `CARRIER_CANNOT_SEE_BUYER_INTERNAL_ANALYTICS` | **YES** |
| `FEATURE_FLAG` | `NUXT_PUBLIC_FREIGHT_COST_WORKSPACE_ENABLED` |
| `FEATURE_FLAG_DEFAULT` | `false` |
| `BROWSER_DIRECT_INTERNAL_SERVICE_CALL` | **NO** |
| `PUBLIC_API_IN_V2_1D` | **NO** |
| `GATEWAY_RBAC_IN_V2_1D` | **NO** |
| `V2_1D_RUNTIME_STARTED` | **NO** (planning PR only) |
| Money wire type | Decimal string |
| Buyer/carrier mask | Backend v2.1E enforces; frontend UX duplicate |
| Pattern reference | v2.0D Contract Rate Workspace |
| Baseline | PR #48 merged @ `4e17070` |

---

## Appendix A — v2.1D Implementation File Plan (future — not this PR)

v2.1D UI implementation **may begin before v2.1E** using mocked adapters. Live adapter wiring remains v2.1E.

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
