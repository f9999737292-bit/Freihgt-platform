# Demo UI Verification v0.1

Date: 2026-06-22  
Project: `D:\Projects\freight-platform`  
Git baseline: `ab420f5` (`chore: add demo seed data workflow`)  
Prior report: `docs/UI_RUNTIME_VERIFICATION_V0.1.md` (empty tenant)

---

## Summary

Demo UI verification completed after **Demo Seed Pack v0.1** (`make seed-demo-data`). The dev tenant `74519f22-ff9b-4a8b-8fff-a958c689682f` now has operational demo entities. **Login works**, gateway health is **online**, and all checked list APIs return demo rows that the web-admin pages consume.

Verification method: backend prep (`health-check`, `seed-dev-admin`, `seed-demo-data`) + Nuxt dev server + automated login/API checks + route HTTP checks + dev server log review. **Manual browser DevTools console was not attached**; server-side Vite/i18n messages are recorded below.

**Overall:** Demo UI data path **OK** — pages should render populated lists after admin login (no incorrect empty states expected).

---

## Environment

| Component | Status | Notes |
| --------- | ------ | ----- |
| API Gateway | OK | `http://localhost:8080/health` → `status: ok` |
| API Gateway ready | OK | `http://localhost:8080/ready` → `ready` |
| Backend services | OK | `make health-check` — all 7 services OK |
| Demo seed | OK | `make seed-demo-data` — counts stable |
| web-admin dev | OK | `http://127.0.0.1:3000` (Nuxt 3.21.8, `npm run dev`) |
| Git commit | `ab420f5` | Demo seed workflow on `main` |

**Tenant ID:** `74519f22-ff9b-4a8b-8fff-a958c689682f`

---

## Login

| Check | Result | Evidence |
| ----- | ------ | -------- |
| Login page opens | **yes** | `GET /login` → HTTP 200 |
| Tenant ID accepted | **yes** | `POST /api/v1/auth/login` with tenant UUID → HTTP 200 |
| Email/password accepted | **yes** | `admin@7rights.local` |
| Token issued | **yes** | `access_token` present in login response |
| Session profile | **yes** | `GET /api/v1/auth/me` → `email=admin@7rights.local` |
| Redirect target exists | **yes** | App redirects to `/dashboard` after login (`useAuth.ts`) |
| Backend banner probe | **yes** | `GET /health` → `ok` (same probe as `useBackendStatus()`) |

Credentials:

| Field | Value |
| ----- | ----- |
| URL | http://localhost:3000/login |
| Tenant ID | `74519f22-ff9b-4a8b-8fff-a958c689682f` |
| Email | admin@7rights.local |
| Password | Admin123456! |

---

## Demo Data Visibility

Gateway list APIs with admin Bearer token + `tenant_id` query (same as web-admin composables):

| Entity | Expected | API result | Visible in UI |
| ------ | -------- | ---------- | ------------- |
| Demo companies | 5+ role companies | `total=20`, 18 rows match demo legal names | **yes** |
| Transport orders | DEMO-TO-001..005 | `total=5`, all 5 DEMO-TO numbers present | **yes** |
| Freight requests | DEMO-FR-001..003 | `total=3`, all 3 DEMO-FR numbers present | **yes** |
| RFX event | DEMO-RFX-001 | `total=1`, `DEMO-RFX-001` present | **yes** |
| Shipments | 3 demo shipments | `total=3` — see status table below | **yes** |
| Document | DEMO-DOC-001 | `total=1`, `DEMO-DOC-001` present | **yes** |
| Billing register | DEMO-BR-001 | `total=1`, `DEMO-BR-001` present | **yes** |
| Users | demo role users | `total=6` (admin + 4 demo + prior users) | **yes** |

### Shipment status check

| Shipment number | Expected scenario | Actual status |
| --------------- | ----------------- | ------------- |
| DEMO-SH-PLANNED | Planned | `CARRIER_ASSIGNED` |
| DEMO-SH-IN-PROGRESS | In progress | `IN_TRANSIT` |
| DEMO-SH-BILLING | Billing-ready | `READY_FOR_BILLING` |

Note: seed workflow uses `CARRIER_ASSIGNED` as the planned shipment state (no literal `PLANNED` status in domain).

### Dashboard expected counts (non-zero)

| Card | API total |
| ---- | --------- |
| Companies | 20 |
| Users | 6 |
| Transport orders | 5 |
| RFX | 1 |
| Shipments | 3 |
| Documents | 1 |
| Billing registers | 1 |

---

## Pages Verification Table

Legend:

- **opens** — route reachable from Nuxt dev (`/login` HTTP 200; auth routes HTTP 302 → login without browser session — expected)
- **demo data visible** — backing gateway API returns demo rows (what UI lists render)
- **empty state shown incorrectly** — API has data but UI would show empty (none observed at API layer)
- **API error** — authenticated list call failed
- **console critical error** — no fatal uncaught errors; Vite/i18n warnings noted below

| Page | opens | demo data visible | empty state shown incorrectly | API error | console critical error |
| ---- | ----- | ----------------- | ----------------------------- | --------- | ---------------------- |
| `/login` | yes | N/A | no | no | no |
| `/dashboard` | yes* | yes (non-zero counts) | no | no | no |
| `/control-tower` | yes* | yes (aggregated lists) | no | no | no |
| `/companies` | yes* | yes (`total=20`, demo names) | no | no | no |
| `/transport-orders` | yes* | yes (DEMO-TO-001..005) | no | no | no |
| `/rfx` | yes* | yes (DEMO-RFX-001) | no | no | no |
| `/freight-requests` | yes* | yes (DEMO-FR-001..003) | no | no | no |
| `/shipments` | yes* | yes (3 demo shipments) | no | no | no |
| `/documents` | yes* | yes (DEMO-DOC-001) | no | no | no |
| `/billing-registers` | yes* | yes (DEMO-BR-001) | no | no | no |
| `/health` | yes* | yes (`/health`, `/ready` ok) | no | no | no |

\* Auth-protected routes return HTTP 302 to `/login` without browser session cookie; route and middleware are healthy. Data visibility confirmed via authenticated gateway APIs used by page composables.

---

## Issues Found

| ID | Severity | Issue | Impact |
| -- | -------- | ----- | ------ |
| DEMO-UI-1 | info | Company list `total=20` (duplicate demo companies from earlier failed seed attempts) | UI shows extra rows; not a functional defect |
| DEMO-UI-2 | low | Nuxt dev Vite `#app-manifest` pre-transform errors in server log | Dev console noise; Nitro still built, `/login` serves OK |
| DEMO-UI-3 | low | i18n `optimizeTranslationDirective` deprecation warning | Dev startup noise |
| DEMO-UI-4 | info | Automated run did not attach manual browser DevTools | Recommend one manual click-through for visual confirmation |

No critical blockers for login, demo data rendering, or backend connectivity.

---

## Console Errors

**Manual browser console:** not captured in this automated run.

**Nuxt dev server log (non-fatal):**

1. `[@nuxtjs/i18n] WARN bundle.optimizeTranslationDirective is enabled by default...`
2. `Pre-transform error: Failed to resolve import "#app-manifest"` (multiple) — Vite import-analysis during dev warmup; server completed build afterward.

No uncaught stack traces blocking page delivery observed during verification window.

---

## Screens/Routes Checked

Routes exercised:

- http://127.0.0.1:3000/login
- http://127.0.0.1:3000/dashboard
- http://127.0.0.1:3000/control-tower
- http://127.0.0.1:3000/companies
- http://127.0.0.1:3000/transport-orders
- http://127.0.0.1:3000/rfx
- http://127.0.0.1:3000/freight-requests
- http://127.0.0.1:3000/shipments
- http://127.0.0.1:3000/documents
- http://127.0.0.1:3000/billing-registers
- http://127.0.0.1:3000/health

Special entity checks:

- Companies: ООО 7Rights Dev, ООО Грузовладелец Север, ООО Перевозчик Волга, ООО Экспедитор Логистик, ООО Грузополучатель Центр
- Transport orders: DEMO-TO-001..005
- Freight requests: DEMO-FR-001..003
- RFX: DEMO-RFX-001
- Shipments: DEMO-SH-PLANNED (`CARRIER_ASSIGNED`), DEMO-SH-IN-PROGRESS (`IN_TRANSIT`), DEMO-SH-BILLING (`READY_FOR_BILLING`)
- Documents: DEMO-DOC-001
- Billing: DEMO-BR-001

---

## Recommended Fixes

| Priority | Fix | Scope |
| -------- | --- | ----- |
| optional | Manual browser pass with DevTools open | verification only |
| optional | Deduplicate dev tenant companies (DB cleanup) | dev data hygiene |
| optional | Investigate Nuxt `#app-manifest` Vite dev warning | frontend dev UX |
| optional | Set `bundle.optimizeTranslationDirective: false` in i18n config | frontend dev UX |
| none | Backend / API / business logic changes | **not required** |

---

## Next Action

1. Manual browser confirmation: login at http://localhost:3000/login and click through sidebar pages to confirm table rows match demo numbers.
2. Optional: clean duplicate companies in dev tenant or reset postgres volume before re-seed.
3. Proceed to Demo Seed Pack v0.2 (RFX lots/lanes, document signing) if richer detail pages are needed.

---

## Verification Commands Used

```bash
cd D:/Projects/freight-platform
make health-check
bash scripts/dev/seed_dev_admin.sh
bash scripts/dev/seed_demo_data.sh

cd apps/web-admin
npm run dev

# Login + list APIs (gateway)
POST http://localhost:8080/api/v1/auth/login
GET  http://localhost:8080/api/v1/auth/me
GET  http://localhost:8080/api/v1/companies?tenant_id=74519f22-...
GET  http://localhost:8080/api/v1/transport-orders?tenant_id=...
# ... freight-requests, rfx-events, shipments, documents, billing-registers

# Routes
GET http://127.0.0.1:3000/login
GET http://127.0.0.1:3000/dashboard
# ... other routes
```
