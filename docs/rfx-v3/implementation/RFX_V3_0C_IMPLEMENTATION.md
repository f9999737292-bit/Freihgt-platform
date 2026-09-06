# RFx v3.0C — Carrier Response Implementation Evidence

**Branch:** `feat/bintrans-enterprise-rfx-carrier-response-v3.0c`  
**Backend frozen head:** `cc8d6dd05396df038d4bd34ea5f8515d996a3f43`  
**Migration:** `000066_rfx_carrier_response_v3_0c.up.sql`

## Backend (frozen)

- Carrier response workspace API (6 routes) via api-gateway
- Atomic PATCH autosave with `save_version` optimistic concurrency
- **422** validation failures; **409** stale version conflicts
- Hidden answer policy: **IGNORE_ON_SAVE**
- SUBMITTED immutability enforced server-side
- Integration gate: `rfx-carrier-response-v3-integration` (PostgreSQL 16)

## Frontend — carrier workspace (`apps/web-procurement`)

- Route: `/carrier/tenders/:id/questionnaire`
- Typed client: `useCarrierResponseApi` + route matrix parity tests
- Workspace composable: autosave FSM, debounced batch merge, 422/409 UX
- Seven wave-1 question types; explicit unsupported-type safety
- Conditional rule UX (SHOW/HIDE/REQUIRE) aligned with v3.0B engine
- i18n: `ru-RU`, `en-US`, `zh-CN` (`carrierResponse.json`)
- Vitest: carrier response unit tests in web-procurement suite

## Buyer preview-as-carrier sandbox (`apps/web-admin`)

**PREVIEW_DATA_ONLY=YES** — interactive sandbox; no carrier-response persistence.

| Invariant | Value |
|---|---|
| `PREVIEW_SOURCE` | Current buyer DRAFT from Studio (`api.studio.value`) |
| `REAL_RESPONSE_CREATED_BY_PREVIEW` | NO |
| `REAL_ANSWER_CREATED_BY_PREVIEW` | NO |
| `PARTICIPANT_MUTATION_BY_PREVIEW` | NO |

### Implementation

- **Route:** `/rfx/:id/studio/preview` — read-only v3.0B preview + **«Пройти как перевозчик»** toggle
- **Component:** `RfxCarrierPreviewSandbox.vue` + pure helpers `utils/rfxPreviewSandbox.ts`
- **State:** Vue reactive `Map` (ephemeral; discarded on close/reset)
- **Rules:** deterministic v3.0B SHOW/HIDE/REQUIRE evaluation (no arbitrary JS)
- **Validation:** local UX simulation (required, min/max, enum, date, conditional required)
- **Submit:** «Проверить отправку» — local validation only; success banner when valid
- **Blocked APIs:** no calls to `/carrier-response/start`, `/answers`, `/validate`, `/submit`

### i18n

`rfx.studio.previewSandbox.*` — `ru-RU`, `en-US`, `zh-CN` parity

### Tests

| Gate | Location |
|---|---|
| Unit / source scan | `apps/web-admin/tests/rfxPreviewSandbox.test.ts` |
| Browser E2E | `apps/web-procurement/e2e/rfx-studio/preview-sandbox.spec.ts` |
| DB count delta proof | `studio/browser_preview_sandbox_db_integration_test.go` (`PREVIEW_SANDBOX_DB_PROOF=1`) |

Browser proof asserts `PREVIEW_CARRIER_RESPONSE_WRITE_REQUEST_COUNT=0` and `REAL_RESPONSE_COUNT_DELTA=0`, `REAL_ANSWER_COUNT_DELTA=0`.

## Browser acceptance — real carrier flow

- CI job: `rfx-carrier-response-browser-e2e`
- Stack: Chromium → web-procurement → production api-gateway → rfx-service → PostgreSQL 16
- HSE fixture: ADR_AVAILABLE, ADR_NUMBER, ADR_EXPIRY, FLEET_COUNT (min 0)
- Distinct from buyer preview sandbox (web-admin)

## Backend defect fixed during frontend gate

- **FRONTEND_DISCOVERED_BACKEND_DEFECT=YES**
- NUMBER `min_value` / `max_value` not enforced on carrier PATCH → fixed in `carrier_answer_validation.go` + unit test

## Known limitations / v3.0D handoff

- Preview validation is UX simulation only — does not replace server-side carrier-response validation
- Scoring, knockout, qualification: **v3.0D** (not started)
- `STOP_AFTER_V3_0C=YES`

## Validation executed locally

| Check | Result |
|---|---|
| web-admin preview sandbox vitest | See FINAL report |
| web-admin full vitest + build | See FINAL report |
| web-procurement vitest + build | See FINAL report |
| rfx-service / api-gateway unit tests | See FINAL report |
| Full CI on FINAL_HEAD | See FINAL report |
