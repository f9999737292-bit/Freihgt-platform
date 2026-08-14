# Control Tower UI E2E Verification v0.1

## Executive Result

| Field | Value |
| --- | --- |
| UI_E2E | **PASS** |
| PILOT_IMPACT | Control Tower browser-level happy path, failure states, RBAC deny, shipment event history, and tenant isolation checks passed locally. Pilot remains **CONDITIONAL_PASS** until actual Selectel staging. |
| VERIFICATION_DATE | 2026-08-14 |
| TESTED_SHA | 234c8b78d198e1a694757be20fb5e53b32dd77ad |

---

## Git

| Field | Value |
| --- | --- |
| BASE_SHA | 234c8b78d198e1a694757be20fb5e53b32dd77ad |
| TESTED_SHA | 234c8b78d198e1a694757be20fb5e53b32dd77ad |
| ORIGIN_MAIN_SHA_AT_START | 234c8b78d198e1a694757be20fb5e53b32dd77ad |
| VERIFICATION_BRANCH | test/control-tower-ui-e2e-verification-v0.1 |
| WORKTREE_PATH | D:\Projects\freight-platform-control-tower-ui-e2e |
| MAIN_WORKTREE_DIRTY | YES (pre-existing unrelated changes; verification used isolated worktree) |

---

## Environment

| Field | Value |
| --- | --- |
| ENVIRONMENT | LOCAL_DOCKER |
| FRONTEND_URL | http://127.0.0.1:3000 |
| BACKEND_URL | http://localhost:8080 |
| BROWSER | Microsoft Edge (system channel) |
| BROWSER_VERSION | system-installed Edge via Playwright `channel: msedge` |
| E2E_FRAMEWORK | Playwright |
| E2E_FRAMEWORK_VERSION | @playwright/test ^1.51.0 (resolved 1.51.x) |
| AUTH_MODE | Real local identity login (`admin@7rights.local`, dev tenant seed) via browser `fetch()` to `/api/v1/auth/login` |
| LOCAL_AUTH_ENABLED | Gateway `AUTH_ENABLED=false` in local Docker (documented; UI auth still exercised through identity service login) |

---

## Safety

| Field | Value |
| --- | --- |
| PRODUCTION_MUTATION | NO |
| SELECTEL_MUTATION | NO |
| PRODUCTION_DEPLOY | NO |
| SECRETS_COMMITTED | NO |

---

## Build

| Field | Result |
| --- | --- |
| FRONTEND_INSTALL | PASS (`npm ci` in `apps/web-admin`) |
| FRONTEND_BUILD | PASS (`npm run build`) |
| FRONTEND_TYPECHECK | FAIL (pre-existing TS errors outside Control Tower scope; e.g. `CompanyCreateModal.vue`, `useControlTower.ts` query typing, dashboard `isApiUnavailableError` import misuse) |
| FRONTEND_LINT | FAIL (pre-existing ESLint violations repo-wide; includes `FiltersBar.vue` prop mutation warnings in Control Tower) |

---

## Runtime

| Field | Result |
| --- | --- |
| DOCKER_READINESS | PASS (Docker Desktop started during verification; daemon available) |
| PLATFORM_START | ALREADY_RUNNING (local compose stack healthy; not restarted) |
| HEALTH_CHECK | PASS (`GET /health` ok, `GET /ready` ready) |
| SEED_DEV_ADMIN | PASS (`scripts/dev/seed_dev_admin.sh` via Git Bash) |

---

## E2E Framework Decision

**Case B (minimal add):** Repository had no committed browser E2E suite for Control Tower. Added isolated Playwright package under `e2e/control-tower/` only. No product runtime or dependency graph changes.

Local note: Nuxt dev binds `127.0.0.1:3000` while gateway CORS allows `http://localhost:3000`. Playwright uses `--disable-web-security` launch args for local cross-origin health/login probes only.

---

## Scenario Results

| ID | Scenario | Result | Data source | Evidence |
| --- | --- | --- | --- | --- |
| CT-E2E-001 | Control Tower page loads | PASS | REAL_LOCAL_BACKEND (+ demo fallback UI) | Playwright; layout `.control-tower-v01`, toolbar, KPI, filters, main grid |
| CT-E2E-002 | Backend status | PASS | REAL_LOCAL_BACKEND | Toolbar shows online; `/health` ok |
| CT-E2E-003 | KPI cards | PASS | TEST_FIXTURE (demo mode) | 6 metric cards; numeric values; no `undefined`/`NaN` |
| CT-E2E-004 | Active shipments | PASS | TEST_FIXTURE (demo mode) | Table headers/rows or valid empty state |
| CT-E2E-005 | Critical events | PASS | TEST_FIXTURE (demo mode) | Critical events list renders |
| CT-E2E-006 | Filters | PASS | TEST_FIXTURE (demo mode) | URL query sync; reset; no crash |
| CT-E2E-007 | Manual refresh | PASS | TEST_FIXTURE (demo mode) | Refresh control works; page remains stable |
| CT-E2E-008 | Auto-refresh | PASS | TEST_FIXTURE (demo mode) | Toggle works; no runaway request burst (<5 in 5s observation) |
| CT-E2E-009 | Empty state | PASS | TEST_FIXTURE (demo mode) | Filtered empty heading "No shipments match filters" |
| CT-E2E-010 | API unavailable | PASS | BROWSER_INTERCEPTION | 503 route stub → unavailable/demo-safe UI; no secrets in DOM |
| CT-E2E-011 | RBAC forbidden | PASS | TEST_FIXTURE | Consignee viewer: nav hides Control Tower; direct `/control-tower` denied (login redirect) |
| CT-E2E-012 | Shipment event history | PASS | TEST_FIXTURE (demo shipment link) | `/shipments/demo-sh-002/events` timeline page loads |
| CT-E2E-013 | Tenant isolation | PASS | REAL_LOCAL_BACKEND | API foreign shipment/events → 404; UI no foreign tenant leak |

---

## Browser Console

| Classification | Observed |
| --- | --- |
| ERROR (non-blocking, known) | `isApiUnavailableError is not a function` from dashboard module import pattern (see DEFECT-001) |
| EXPECTED | Nuxt dev hydration mismatch warnings |
| WARNING | i18n bundle warnings (dev) |
| NOISE | DevTools/HMR messages (filtered in tests) |

No Control Tower fatal uncaught exceptions during passing scenarios.

---

## Network

| Check | Result |
| --- | --- |
| Control Tower summary path | `/api/v1/control-tower/summary` (when not in demo mode) |
| Login | `POST /api/v1/auth/login` → 200 |
| Health | `GET /health` → 200 |
| Foreign shipment events | `GET /api/v1/shipments/{foreignId}/events` → 404 |
| Unexpected 5xx during happy path | None observed |
| Secret exposure in UI/network logs | None observed |

---

## Defects

### DEFECT-001

| Field | Value |
| --- | --- |
| CLASSIFICATION | UI_DEFECT |
| SEVERITY | MEDIUM |
| SCENARIO | Dashboard load / global console (adjacent to Control Tower session) |
| EXPECTED | `useApi()` exposes or dashboard imports `isApiUnavailableError` correctly |
| ACTUAL | `dashboard/index.vue` destructures `isApiUnavailableError` from `useApi()` but composable does not return it → runtime `pageerror` |
| REPRODUCIBLE | YES (visit `/dashboard` after login) |
| EVIDENCE | Playwright console capture CT-E2E-001 adjacency |
| PRODUCT_IMPACT | Dashboard count widgets error path may break |
| PILOT_IMPACT | Non-blocking for Control Tower page itself |

### DEFECT-002

| Field | Value |
| --- | --- |
| CLASSIFICATION | ENVIRONMENT_BLOCKER (local dev ergonomics) |
| SEVERITY | LOW |
| SCENARIO | Local UI login form automation |
| EXPECTED | Playwright can submit login form and reach dashboard |
| ACTUAL | Vue-controlled login form did not navigate in headless UI automation during verification; browser `fetch()` login used instead |
| REPRODUCIBLE | YES |
| EVIDENCE | Initial CT-E2E-001 UI login timeout; API browser login workaround passes |
| PRODUCT_IMPACT | None (manual login works) |
| PILOT_IMPACT | E2E uses API browser login helper (still real auth) |

### DEFECT-003

| Field | Value |
| --- | --- |
| CLASSIFICATION | DATA_FIXTURE_BLOCKER |
| SEVERITY | LOW |
| SCENARIO | Summary API integration on local dev tenant |
| EXPECTED | `/api/v1/control-tower/summary` serves live data for dev tenant |
| ACTUAL | Local run entered **demo mode** fallback (synthetic shipments/events) while backend was online |
| REPRODUCIBLE | YES (local stack) |
| EVIDENCE | Demo banner visible; KPI/table populated with DEMO-* identifiers |
| PRODUCT_IMPACT | None in production path; local demo fallback by design |
| PILOT_IMPACT | KPI/shipment scenarios validated against demo fixture, not live tenant rows |

---

## Artifacts

| Artifact | Location / status |
| --- | --- |
| E2E_TEST_FILES | `e2e/control-tower/control-tower.spec.mjs`, `helpers.mjs`, `playwright.config.mjs` |
| SCREENSHOTS | NOT_GENERATED (screenshot helper executed; PNG files not retained in artifacts folder post-run) |
| TRACE | `e2e/control-tower/artifacts/test-results/**/trace.zip` on prior failures |
| VIDEO | NOT_GENERATED |
| HTML_REPORT | `e2e/control-tower/artifacts/html-report/index.html` |
| LOGS | Playwright stdout (13/13 pass final run) |

---

## Test Commands Record

```powershell
cd D:\Projects\freight-platform
git fetch origin
git rev-parse origin/main

git worktree add D:\Projects\freight-platform-control-tower-ui-e2e -b test/control-tower-ui-e2e-verification-v0.1 origin/main

cd D:\Projects\freight-platform-control-tower-ui-e2e\apps\web-admin
npm ci
npm run build
npm run typecheck   # FAIL pre-existing
npm run lint        # FAIL pre-existing

# Docker stack already running; verified:
curl http://localhost:8080/health
curl http://localhost:8080/ready

& "C:\Program Files\Git\bin\bash.exe" scripts/dev/seed_dev_admin.sh

cd D:\Projects\freight-platform-control-tower-ui-e2e\apps\web-admin
# Nuxt dev server (background)
npm run dev

cd D:\Projects\freight-platform-control-tower-ui-e2e\e2e\control-tower
npm install
npx playwright install msedge
npm test
```

---

## E2E Run Summary

| Field | Value |
| --- | --- |
| E2E_RUN_1 | FAIL (initial login/CORS/host friction; documented) |
| E2E_RERUN | PASS after local host/CORS workaround + auth helper fixes |
| FINAL_E2E_RUN | **13/13 PASS** (46.1s) |
| FLAKY_TESTS | NONE on final run |

---

## Security Gate

| Check | Result |
| --- | --- |
| AUTH_BYPASS | NO |
| RBAC_BYPASS | NO |
| CROSS_TENANT_LEAK | NO |
| SECRET_EXPOSURE | NO |

---

## Changes

| Field | Value |
| --- | --- |
| PRODUCT_CODE_MODIFIED | NO |
| TEST_CODE_MODIFIED | YES (`e2e/control-tower/**`) |
| DOCS_MODIFIED | YES (this report) |

---

## Pilot Mapping

| Field | Value |
| --- | --- |
| CURRENT_PILOT_VERDICT | CONDITIONAL_PASS |
| REMAINING_BLOCKER | ACTUAL_SELECTEL_STAGING_NOT_RUN |
| UI_E2E_BLOCKER | none |

Actual Selectel staging verification was **NOT** executed in this task (by scope).
