# FREIGHT COST ANALYTICS WORKSPACE v2.1D — Implementation

## GIT

| Field | Value |
|-------|-------|
| Branch | `arch/freight-cost-workspace-v2.1D` (runtime worktree) |
| Scope | `apps/web-procurement/**`, `docs/engineering/FC_D_TEST_INVENTORY.json`, this document |
| Contract | `FREIGHT_COST_ANALYTICS_WORKSPACE_v2.1D_IMPLEMENTATION_PLAN.md` |

## SLICE_BOUNDARY

| Gate | Value |
|------|-------|
| FREIGHT_COST_SERVICE_CHANGED | NO |
| API_GATEWAY_CHANGED | NO |
| MIGRATIONS_ADDED | NO |
| BROWSER_INTERNAL_FREIGHT_COST_API | NO |
| INTERNAL_SERVICE_TOKEN_FRONTEND | NO |
| LIVE_BROWSER_BACKEND_INTEGRATION | DEFERRED_V2_1E |
| PUBLIC_FREIGHT_COST_ROUTES_ADDED | NO |

v2.1D implements the feature-flagged workspace shell, typed public DTO contracts, buyer/carrier UX masks, decimal-string money helpers, RU/EN/ZH i18n, and fail-closed production data source. v2.1E owns public `/api/v1/freight-costs/*` routes, gateway RBAC, server-side masking, and live adapter wiring.

## FEATURE_GATE

| Setting | Value |
|---------|-------|
| FEATURE_FLAG | `NUXT_PUBLIC_FREIGHT_COST_WORKSPACE_ENABLED` |
| runtimeConfig.public.freightCostWorkspaceEnabled | `true` only when env `=== 'true'` |
| DEFAULT_ENABLED | FALSE |
| NAV_GATED | YES — `layouts/default.vue` via `shouldShowFreightCostsNav` |
| DIRECT_ROUTE_DISABLED_STATE | YES — middleware `freight-cost-workspace` → `/freight-costs/unavailable` |

## ROUTES

| Route | Page |
|-------|------|
| `/freight-costs` | Overview KPI shell |
| `/freight-costs/planned-vs-actual` | Planned vs Actual table shell |
| `/freight-costs/shipments` | Shipments list shell |
| `/freight-costs/shipments/{transportOrderId}` | Shipment cost detail shell |
| `/freight-costs/variance` | Variance analysis shell (buyer nav) |
| `/freight-costs/accessorials` | Accessorial spend shell (buyer nav) |
| `/freight-costs/carriers` | Carrier performance shell |
| `/freight-costs/lanes` | Lane performance shell |
| `/freight-costs/unavailable` | Feature disabled state |

## DATA_SOURCE

File: `utils/freightCostDataSource.ts`

| Mode | Behavior |
|------|----------|
| `LIVE_API_V2_1E` (production default) | Fail-closed — throws `FREIGHT_COST_LIVE_UNAVAILABLE` |
| `MOCK` | Vitest-only injected handlers |

File: `composables/useFreightCostsApi.ts` — typed adapter surface for future public gateway paths under `/api/v1/freight-costs/*`. **No live HTTP calls in v2.1D.**

## TYPES

File: `types/freightCost.ts`

- `DecimalString`
- `FreightCostSummaryDTO`, `FreightCostSummaryAggregateDTO`
- Row/detail/aggregate view-models aligned with plan §20
- Frozen enums for finality, reconciliation, accessorial categories

## MONEY

File: `utils/freightCostMoney.ts`

- `formatDecimalMoney` / `formatDecimalPercent` — string-based formatting only
- NULL → em dash; explicit `"0.00"` preserved
- Legacy `utils/format.ts` **not used** for freight cost fields

## PERMISSIONS / MASKS

Files: `utils/freightCostPermissions.ts`, `utils/freightCostWorkspace.ts`

| Actor | UX |
|-------|-----|
| Buyer roles | Full analytics nav + buyer-internal KPI/table/detail sections |
| Carrier reader | Receivable-oriented columns; buyer-internal fields masked client-side |

FRONTEND_MASKING_IS_UX_ONLY=YES — v2.1E gateway enforces server-side DTO scope.

**Forbidden:** Settled Unpaid Exposure KPI; cross-tax-basis subtraction.

**Forecast label:** `freightCosts.kpi.plannedPlusProposedExposure` (displays `forecast_exposure` wire value).

## COMPONENTS

| Component | Purpose |
|-----------|---------|
| `components/ui/KpiCard.vue` | Generic KPI card (no web-admin import) |
| `components/freight-cost/FreightCostShell.vue` | Workspace page wrapper + subnav |
| `components/freight-cost/FreightCostSubnav.vue` | Actor-aware workspace navigation |
| `components/freight-cost/FreightCostOverviewKpis.vue` | Overview KPI grid |
| `components/freight-cost/FreightCostFilters.vue` | Filter shell + chips |
| `components/freight-cost/FreightCostOrdersTable.vue` | Planned vs Actual table |
| `components/freight-cost/FreightCostDetailPanel.vue` | Detail section shell |
| `components/freight-cost/FreightCostLiveUnavailableBanner.vue` | v2.1E live-data banner |

## I18N

`i18n/{ru-RU,en-US,zh-CN}/freightCosts.json` registered in `nuxt.config.ts`. Nav keys added to `nav.json`.

## TESTS

`tests/freightCostWorkspace.test.ts` — **97/97** FC-D tests (IDs FC-D-NAV-001 … FC-D-ERR-006). Inventory: `docs/engineering/FC_D_TEST_INVENTORY.json`.

FC-D-SEC-006..010 deferred to v2.1E backend E2E per frozen plan.

## FINAL_GATES

| Gate | Status |
|------|--------|
| UI_SHELL_IMPLEMENTED | YES |
| FEATURE_FLAG_DEFAULT_OFF | YES |
| NO_INTERNAL_TOKEN_IN_ADAPTER | YES |
| NO_LIVE_FINANCIAL_DATA_IN_PRODUCTION_PAGES | YES |
| FC-D_UNIT_TESTS | YES (97) |
| PUBLIC_GATEWAY_ROUTE | NO (v2.1E) |
| LIVE_BROWSER_BACKEND_INTEGRATION | DEFERRED_V2_1E |

## V2_1E_GAPS

| Gap | Owner |
|-----|-------|
| Public `/api/v1/freight-costs/*` gateway routes | v2.1E |
| Live adapter wiring in `useFreightCostsApi` | v2.1E |
| Server-side RBAC + ApplyViewScope masking | v2.1E |
| Business date filter `date_dimension` contract | v2.1E |
| Financial browser E2E | v2.1E |
