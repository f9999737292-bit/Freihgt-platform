# UI Runtime Verification v0.1

Date: 2026-06-21  
Project: `D:\Projects\freight-platform`  
Git baseline: `6633fdb` (working tree: report only, no commit)

---

## Summary

Web-admin UI runtime verification completed against a live local stack (API Gateway + 7 backend services + Nuxt dev server). **Login and all checked routes are reachable.** Backend health banner source (`GET /health`) returns **ok**. List APIs respond correctly for the dev admin tenant; most operational lists are **empty** on tenant `74519f22-…` (expected empty states, not API failures).

Verification method: automated HTTP/API checks + Nuxt dev server log review. **Manual browser DevTools console was not attached** in this session; server-side Vue/i18n warnings are recorded below.

**Overall:** UI runtime **OK** for local dev portal smoke.

---

## Environment

| Component | Status | Notes |
| --------- | ------ | ----- |
| API Gateway | OK | `http://localhost:8080/health` → `{"status":"ok","service":"api-gateway",...}` |
| Backend services | OK | `GET /ready` → `ready`, all 7 services `ok` |
| web-admin dev | OK | `http://127.0.0.1:3000` (Nuxt 3.21.8, already running) |
| Prometheus | OK | `http://localhost:9090/-/healthy` → HTTP 200 |
| Grafana | OK | `http://localhost:3001/api/health` → database ok |
| Git commits | `f0f3e3b`, `6633fdb` | Clean before this report |

**Login credentials used:**

| Field | Value |
| ----- | ----- |
| Tenant ID | `74519f22-ff9b-4a8b-8fff-a958c689682f` |
| Email | `admin@7rights.local` |
| Password | `Admin123456!` |

---

## Login Check

| Check | Result | Evidence |
| ----- | ------ | -------- |
| Tenant ID accepted | **yes** | `POST /api/v1/auth/login` with tenant UUID → HTTP 200 |
| Email/password accepted | **yes** | Response: `user.email=admin@7rights.local` |
| Token issued | **yes** | `access_token` present |
| Session profile | **yes** | `GET /api/v1/auth/me` → email confirmed |
| Redirect target exists | **yes** | `GET /dashboard` → HTTP 200 |
| Login page opens | **yes** | `GET /login` → HTTP 200 |

Expected post-login navigation (app code): `router.push('/dashboard')` in `useAuth.ts`.

---

## Backend Status Banner

| Check | Result |
| ----- | ------ |
| Health probe URL | `http://localhost:8080/health` |
| Gateway online | **yes** (`status: ok`) |
| Login page banner | Client-side `useBackendStatus()` — probe succeeds with current stack |
| Authenticated banner | `BackendStatusBanner.vue` uses same health probe |
| Refresh control | Present on login, dashboard, control-tower, health pages |

---

## Pages Verification Table

Legend: **opens** = route HTTP 200 from Nuxt dev; **backend data** = gateway list API with login token + `tenant_id` query; **empty state** = API OK with `total=0`; **refresh** = refresh control present in page source.

| Page | opens | backend data loads | visible error | empty state works | refresh button |
| ---- | ----- | ------------------ | ------------- | ----------------- | -------------- |
| `/login` | yes | N/A (health probe OK) | no | N/A | yes (backend status) |
| `/dashboard` | yes | yes (counts API) | no | yes (0 counts shown as 0/—) | yes |
| `/control-tower` | yes | yes (aggregated lists) | no | yes (widgets with 0/empty) | yes |
| `/health` | yes | yes (`/health`, `/ready`) | no | yes (table populated from `/ready`) | yes |
| `/companies` | yes | yes (`total=3`) | no | N/A (has data) | via filters/reload on mount |
| `/transport-orders` | yes | yes (`total=0`) | no | yes | on mount |
| `/rfx` | yes | yes (`total=0`) | no | yes | on mount |
| `/freight-requests` | yes | yes (`total=0`) | no | yes | on mount |
| `/shipments` | yes | yes (`total=0`) | no | yes | on mount |
| `/documents` | yes | yes (`total=0`) | no | yes | on mount |
| `/billing-registers` | yes | yes (`total=0`) | no | yes | on mount |

### API totals for dev tenant `74519f22-…`

| API | total | items (limit 5) |
| --- | ----- | ----------------- |
| `/api/v1/companies` | 3 | 3 |
| `/api/v1/users` | 2 | 2 |
| `/api/v1/transport-orders` | 0 | 0 |
| `/api/v1/rfx-events` | 0 | 0 |
| `/api/v1/freight-requests` | 0 | 0 |
| `/api/v1/shipments` | 0 | 0 |
| `/api/v1/documents` | 0 | 0 |
| `/api/v1/billing-registers` | 0 | 0 |

Note: integration smoke test data lives under tenant `91babc18-…` (`test-tenant`), not the dev admin tenant. Empty lists on dev tenant are **data isolation**, not UI/API defects.

---

## Health Page

| Check | Result |
| ----- | ------ |
| API Gateway `/health` | ok, service `api-gateway`, version `0.1.0` |
| API Gateway `/ready` | `ready` |
| Services visible | 7 rows: identity, company, transport-order, rfx, shipment, document, billing-register — all `ok` |
| Metrics links | Prometheus `:9090` and Grafana `:3001` reachable |
| Refresh | `refresh()` re-fetches `/health` + `/ready` (code verified; endpoints OK) |

---

## Control Tower

| Check | Result |
| ----- | ------ |
| Layout/route opens | yes (`HTTP 200`) |
| KPI / funnel / boards | Load via `useControlTower().loadData()` — APIs return 200 |
| Cards/widgets visible | Expected with current data (mostly zeros on dev tenant) |
| Frontend runtime error | **no fatal errors** in verification |
| Backend offline banner | Hidden when gateway health OK |

---

## Browser Console Errors

**Manual browser console:** not captured in this automated run.

**Nuxt dev server log (non-fatal warnings):**

1. `[@nuxtjs/i18n] WARN bundle.optimizeTranslationDirective is enabled by default...`
2. `[Vue warn]: Component .../pages/index.vue is missing template or render function` — root `/` is redirect-only (`navigateTo` login/dashboard); warning on SSR, **not blocking**.

No uncaught stack traces or failed module loads observed in dev server output during page requests.

---

## Issues Found

| ID | Severity | Issue | Impact |
| -- | -------- | ----- | ------ |
| UI-1 | low | `pages/index.vue` redirect page triggers Vue SSR warning | Console noise on `/` |
| UI-2 | low | i18n `optimizeTranslationDirective` deprecation warning | Console noise at dev startup |
| UI-3 | info | Dev tenant has sparse operational data | Empty states on most list pages; not a bug |
| UI-4 | info | Smoke test tenant ≠ dev admin tenant | To see rich data in UI, use tenant with smoke data or re-run smoke under dev tenant |

No critical blockers for login, navigation, or backend connectivity.

---

## Recommended Fixes

| Priority | Fix | Scope |
| -------- | --- | ----- |
| optional | Add minimal template to `pages/index.vue` or use route rule redirect | frontend dev UX (console warning) |
| optional | Set `bundle.optimizeTranslationDirective: false` in i18n config | frontend dev UX (warning) |
| optional | Seed demo data for tenant `74519f22-…` or document which tenant has smoke data | dev experience |
| none | Backend/API changes | **not required** for current UI runtime |

---

## Next Action

1. Manual browser pass (recommended): open `http://localhost:3000/login`, login with credentials above, click through sidebar + refresh on Health and Control Tower.
2. Optional: run `make integration-smoke-test` under dev tenant or seed demo rows for richer UI lists.
3. Commit this report when ready: `docs/UI_RUNTIME_VERIFICATION_V0.1.md` only.

---

## Verification Commands Used

```powershell
# Gateway
Invoke-RestMethod http://localhost:8080/health
Invoke-RestMethod http://localhost:8080/ready

# Login
POST http://localhost:8080/api/v1/auth/login
GET  http://localhost:8080/api/v1/auth/me

# Pages
GET http://127.0.0.1:3000/login
GET http://127.0.0.1:3000/dashboard
# ... other routes

# web-admin (if not running)
cd D:\Projects\freight-platform\apps\web-admin
npm run dev
```
