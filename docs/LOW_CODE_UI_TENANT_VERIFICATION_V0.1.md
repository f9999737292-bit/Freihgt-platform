# Low-code UI Tenant Verification v0.1

## Summary

* **Backend/API low-code flow works** under dev tenant `74519f22-ff9b-4a8b-8fff-a958c689682f`.
* **Playwright visual test skipped/failed** due to local Chromium install lock and missing browser binary — **not an app issue**.
* **Manual browser verification required** to confirm UI rendering (Custom fields panel, Read-only badge, lookup page).

No application code, API contracts, migrations, or business logic were changed in this pack.

## Root Cause

* **Tenant ID was likely entered into Entity ID / search field** on `/low-code/custom-field-values` (or confused with a search input). API accepts the UUID format but returns **0 custom field values** when tenant UUID is used as `entity_id`.
* **Or browser session used another tenant** — stale or wrong value in `localStorage` key `freight_admin_tenant_id`. Wrong `X-Tenant-ID` returns **0 form templates**.
* **Correct flow:** login with Tenant ID `74519f22-ff9b-4a8b-8fff-a958c689682f`, confirm tenant in header, then open low-code / entity detail pages. On lookup page use **Use demo entity** or a valid **entity UUID** (not tenant UUID).

## Verified by API

All checks run via PowerShell against gateway `http://localhost:8080` (2026-06-22).

| Check | Result |
| ----- | ------ |
| `make health-check` | **OK** — all services including `low-code-service` |
| `make seed-dev-admin` | **OK** |
| `make seed-demo-data` | **OK** |
| `make seed-lowcode-demo` | **OK** |
| Low-code form templates API | **OK** — 3 published templates |
| Low-code custom field values API | **OK** — all demo entities return expected fields |
| Login API | **OK** — `user.tenant_id` matches dev tenant |

### Form templates

```http
GET /api/v1/low-code/form-templates
X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f
```

**Result:** 3 templates — `TRANSPORT_ORDER`, `SHIPMENT`, `BILLING_REGISTER`.

### Custom field values

| entity_type | entity_id | fields returned |
| ----------- | --------- | ----------------- |
| TRANSPORT_ORDER | `2db04b49-665c-469f-bcb1-ffeb1274fedb` | cargo_class, internal_cost_center, loading_window_note |
| SHIPMENT | `14d405e2-0152-4030-b356-eec464a3cc66` | temperature_mode, driver_comment, loading_contact_phone |
| BILLING_REGISTER | `cf7dbc77-395f-42a2-9717-476e4cd93796` | payment_priority, approval_group, cost_allocation_code |

### Negative reproduction (empty UI)

| Scenario | API result |
| -------- | ---------- |
| Wrong `X-Tenant-ID` | 0 form templates |
| Tenant UUID as `entity_id` | 0 custom field values |

## Manual Visual Check URLs

**Login:** http://localhost:3000/login

| Field | Value |
| ----- | ----- |
| Tenant | `74519f22-ff9b-4a8b-8fff-a958c689682f` |
| Email | `admin@7rights.local` |
| Password | `Admin123456!` |

After login, confirm header shows current tenant `74519f22-...`.

### Transport Order

http://localhost:3000/transport-orders/2db04b49-665c-469f-bcb1-ffeb1274fedb

Expected Custom fields panel:

* cargo_class
* internal_cost_center
* loading_window_note

### Shipment

http://localhost:3000/shipments/14d405e2-0152-4030-b356-eec464a3cc66

Expected Custom fields panel:

* temperature_mode
* driver_comment
* loading_contact_phone

### Billing Register

http://localhost:3000/billing-registers/cf7dbc77-395f-42a2-9717-476e4cd93796

Expected Custom fields panel:

* payment_priority
* approval_group
* cost_allocation_code

### Low-code lookup

http://localhost:3000/low-code/custom-field-values

Expected:

* **Use demo entity** resolves entity UUID and loads values
* Table shows field_code / value / updated_at
* Read-only badge visible

### Additional low-code pages

* http://localhost:3000/low-code — hub / service status
* http://localhost:3000/low-code/form-templates — 3 template rows

## Playwright Note

* Playwright automated browser test **failed** due to Chromium install lock and missing browser binary in the local environment.
* **Do not treat as app failure** — backend/API verification passed; UI uses the same endpoints and `X-Tenant-ID` header.
* Playwright setup can be fixed later in a separate tooling pack if automated UI regression is needed.

## Recommended Next Action

1. **Manual visual check** in browser using URLs above (~5 min).
2. If still empty with correct header tenant → DevTools Network: confirm `X-Tenant-ID` and response payloads.
3. Then proceed to **Low-code Custom Field Values Edit UI Pack v0.1**.

## Verification Commands

```powershell
cd D:\Projects\freight-platform
make health-check
make seed-dev-admin
make seed-demo-data
make seed-lowcode-demo

curl.exe -s -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" http://localhost:8080/api/v1/low-code/form-templates

curl.exe -s -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" "http://localhost:8080/api/v1/low-code/custom-field-values?entity_type=TRANSPORT_ORDER&entity_id=2db04b49-665c-469f-bcb1-ffeb1274fedb"

cd apps/web-admin
npm run dev
```

## Environment

| Item | Value |
| ---- | ----- |
| Project | `D:\Projects\freight-platform` |
| Base commit (panel) | `3daec09` |
| API Gateway | `http://localhost:8080` |
| Low-code service | `http://localhost:8088` |
| Web-admin | `http://localhost:3000` |

## Frontend tenant flow (reference)

* Tenant set at **login** → `localStorage` key `freight_admin_tenant_id`
* API calls send `X-Tenant-ID` via `useApi()` / `useLowCodeApi()`
* Low-code pages have **no tenant search** — only `entity_type` filter and `entity_id` lookup
