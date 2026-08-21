# FREIGHT CONTRACT RATE WORKSPACE v2.0D — Implementation

## GIT

| Field | Value |
|-------|-------|
| Branch | `feat/freight-contract-rate-workspace-v2.0D` |
| Base SHA | `225c79fe61e2599b4e8cde6a89a1fc04864484e6` |
| Scope | `apps/web-procurement/**`, this document |

## DISCOVERY

| Pattern | Choice |
|---------|--------|
| FRONTEND_FRAMEWORK | Nuxt 3.15, Vue 3.5, Pinia |
| UI_COMPONENT_LIBRARY | Local `components/ui/*` + `@freight-platform/ui` shell |
| API_PATTERN | `useApi()` composable → `/api/v1/*` via `runtimeConfig.public.apiBaseUrl` |
| PERMISSION_PATTERN | `usePermissions()` role helpers (UX only) |
| I18N_PATTERN | `@nuxtjs/i18n`, JSON namespaces under `i18n/{locale}/` |
| TEST_PATTERN | Vitest unit tests in `tests/*.test.ts` (node env) |

## SLICE_BOUNDARY

| Gate | Value |
|------|-------|
| CONTRACT_RATE_SERVICE_CHANGED | NO |
| API_GATEWAY_CHANGED | NO |
| MIGRATIONS_ADDED | NO |
| BROWSER_INTERNAL_RATE_API_ACCESS | NO |
| INTERNAL_SERVICE_TOKEN_FRONTEND | NO |
| LIVE_BROWSER_BACKEND_INTEGRATION | DEFERRED_V2_0E |
| GATEWAY_RATE_ROUTES_ADDED | NO |

v2.0D implements UI + typed public API adapter contract only. v2.0E owns gateway routes, RBAC, OpenAPI exposure, and live E2E.

## FEATURE_GATE

| Setting | Value |
|---------|-------|
| FEATURE_FLAG | `NUXT_PUBLIC_CONTRACT_RATE_WORKSPACE_ENABLED` |
| runtimeConfig.public.contractRateWorkspaceEnabled | `true` only when env `=== 'true'` |
| DEFAULT_ENABLED | FALSE |
| NAV_GATED | YES — `layouts/default.vue` |
| DIRECT_ROUTE_DISABLED_STATE | YES — middleware `contract-rate-workspace` → `/contracts/unavailable` |

## ROUTES

| Route | Page |
|-------|------|
| `/contracts` | `pages/contracts/index.vue` |
| `/contracts/{id}` | `pages/contracts/[id]/index.vue` |
| `/contracts/{id}/rates` | `pages/contracts/[id]/rates/index.vue` |
| `/contracts/{id}/rates/simulate` | `pages/contracts/[id]/rates/simulate.vue` |
| `/contracts/unavailable` | Feature disabled state |

## API_ADAPTER

File: `composables/useContractRatesApi.ts`

Targets future public gateway paths:

- `/api/v1/transport-contracts` (+ lifecycle actions)
- `/api/v1/transport-contracts/{id}/rate-cards`
- `/api/v1/rate-cards/{id}/versions`
- `/api/v1/rate-card-versions/{id}` (+ activate, rate-lines)
- `/api/v1/rate-lines/{id}` (+ components)
- `/api/v1/rate-components/{id}`
- `/api/v1/rates/resolve`

Uses standard `useApi()` — no internal token, no `/internal/v1`.

## TYPES

File: `types/contractRate.ts`

Backend JSON field names preserved. Money as decimal strings. Equipment TrimSpace only, case-sensitive.

## PERMISSIONS

Extended `usePermissions.ts` + pure helpers in `utils/contractRatePermissions.ts`.

| Role group | UX |
|------------|-----|
| PLATFORM_ADMIN, PROCUREMENT_MANAGER, SHIPPER_ADMIN, FORWARDER_MANAGER | Buyer mutation |
| CARRIER_ADMIN, CARRIER_DISPATCHER, CARRIER_ACCOUNTANT | Read-only when no buyer mutate role |
| SHIPPER_LOGIST | Read, no commercial mutation |

FRONTEND_PERMISSION_IS_UX_ONLY=YES

## CONTRACT_LIST

Enterprise table with client-side filters (status, carrier, search). Backend list has no query params — filters applied safely in `utils/contractRate.filterContracts`.

States: loading, empty, backend unavailable, 403, missing company.

## CONTRACT_CREATE

Modal form; always DRAFT via backend. No status/audit fields exposed.

## CONTRACT_DETAIL

Overview + lifecycle actions + links to rates/simulation.

## LIFECYCLE

Frozen transitions with confirmation modals. Termination warns historical snapshots unchanged.

## RATE_CARDS / VERSION_HISTORY / LANE_EDITOR / COMPONENT_EDITOR

Master-detail layout on `/contracts/{id}/rates`. DRAFT-only mutation via:

- `patchRateCardVersion` — draft validity edit
- `patchRateLine` / `deleteRateLine` — lane edit/delete with confirmation
- `createRateComponent` / `patchRateComponent` / `deleteRateComponent` — component CRUD
- `buildRateComponentPayload` — deterministic UNIT_RATE payloads (WAITING/DETENTION include amount + unit_code)

Duplicate component add buttons hidden when type already exists on lane.

## ACTIVE / SUSPENDED METADATA EDIT

`canShowContractEdit` decoupled from lifecycle actions. ACTIVE/SUSPENDED buyers may edit `description` and `external_reference` only via `buildPatchContractPayload`.

## TERMINAL HISTORY ACCESS

Rate and simulation navigation visible for TERMINATED/EXPIRED/CANCELLED contracts. Lifecycle mutation buttons remain hidden.

## TEST HARDENING

Placeholder assertions removed. Added D-FIX-001..010 remediation gates testing production helpers in `utils/contractRate.ts` and `utils/contractRateWorkspace.ts`.

## VERSION_DIFF

Client-side diff via `utils/contractRate.diffRateVersions` — lane key: origin|destination|equipment|ROAD (case-sensitive equipment).

## RATE_SIMULATION

Read-only contract-rate preview. ROAD fixed. No manual spot / RFx award controls.

## MONEY

Decimal strings in types/forms. No JS float canonical arithmetic.

## I18N

`contracts.json` added for RU / EN / ZH. `nav.contracts` in all locales.

## ACCESSIBILITY

Semantic buttons, form labels, table headers, loading states, confirmation modals.

## RESPONSIVE

Grid/table-scroll patterns from existing workspace pages.

## ERROR_HANDLING

ApiError mapping for 400/401/403/404/409/503. Domain codes localized under `contracts.errors.*`.

## TESTS

`tests/contractRateWorkspace.test.ts` — matrices D-UI-001..018, D-RATE-001..020, D-DIFF-001..006, D-SIM-001..009, D-FLAG-001..005.

## CI

Existing `frontend-web-procurement-check` covers typecheck, test, build.

## SECURITY

Verified: no `/internal/v1`, no `INTERNAL_SERVICE_TOKEN` in public runtime config or adapter.

## V2_0E_API_GAPS

| Gap | Notes |
|-----|-------|
| Public gateway contract-rate routes | v2.0E |
| Server-side RBAC on contract-rate | v2.0E |
| Live browser integration | v2.0E enable flag + E2E |
| Backend list filters/pagination | Optional v2.0E if needed |

## OUT_OF_SCOPE

- api-gateway route registration
- contract-rate-service changes
- public RBAC enforcement
- load tests / production enablement

## FINAL_GATES

| Gate | Status |
|------|--------|
| UI_IMPLEMENTED | YES |
| TYPED_PUBLIC_API_CONTRACT | YES |
| PUBLIC_GATEWAY_ROUTE | NO (v2.0E) |
| LIVE_BROWSER_BACKEND_INTEGRATION | DEFERRED_V2_0E |
| WORKSPACE_DEFAULT_PUBLIC_STATE | DISABLED |
| BUYER_ACTIVATION_UI_FLOW | PASS (mocked) |
| MOCKED_PUBLIC_API_CONTRACT | PASS |
