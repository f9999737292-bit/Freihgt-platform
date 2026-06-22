# Low-code UI Tenant Verification v0.1

## Summary

Runtime verification confirms **backend, seed data, and gateway API work correctly** under dev tenant `74519f22-ff9b-4a8b-8fff-a958c689682f`. Low-code form templates (3) and custom field values for all demo entities are returned as expected.

The user report **«по запросу 74519f22-ff9b-4a8b-8fff-a958c689682f ничего не найдено»** is consistent with **misuse of Tenant ID in a lookup/search context**, not with missing backend data.

**Most likely root cause:** Tenant UUID was entered in the **Entity ID** field on `/low-code/custom-field-values`, or the browser session used a **different / empty tenant** in `localStorage` instead of the dev tenant set at login.

No application code changes were required.

## Environment

| Item | Value |
| ---- | ----- |
| Project | `D:\Projects\freight-platform` |
| Base commit | `3daec09` (entity detail panel) |
| API Gateway | `http://localhost:8080` |
| Low-code service (direct) | `http://localhost:8088` |
| Web-admin dev server | `http://127.0.0.1:3000` (HTTP 200 at verification time) |
| Verification date | 2026-06-22 |

## Tenant Used

```
74519f22-ff9b-4a8b-8fff-a958c689682f
```

Login: `admin@7rights.local` / `Admin123456!`

## Backend Checks

| Command | Result |
| ------- | ------ |
| `make health-check` | **OK** — all 9 services including `low-code-service` |
| `make seed-dev-admin` | **OK** — tenant + admin login verified via gateway |
| `make seed-demo-data` | **OK** — DEMO-TO-001, DEMO-SH-PLANNED, DEMO-BR-001 present |
| `make seed-lowcode-demo` | **OK** — 3 published templates + demo custom field values |

## API Checks

### Form templates (gateway)

```http
GET /api/v1/low-code/form-templates
X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f
```

**Result:** **3 templates** — `TRANSPORT_ORDER`, `SHIPMENT`, `BILLING_REGISTER`.

### Custom field values (gateway)

| entity_type | entity_id | Expected fields | Actual fields | Match |
| ----------- | --------- | --------------- | ------------- | ----- |
| TRANSPORT_ORDER | `2db04b49-665c-469f-bcb1-ffeb1274fedb` | cargo_class, internal_cost_center, loading_window_note | cargo_class, internal_cost_center, loading_window_note | **yes** |
| SHIPMENT | `14d405e2-0152-4030-b356-eec464a3cc66` | temperature_mode, driver_comment, loading_contact_phone | driver_comment, loading_contact_phone, temperature_mode | **yes** |
| BILLING_REGISTER | `cf7dbc77-395f-42a2-9717-476e4cd93796` | payment_priority, approval_group, cost_allocation_code | approval_group, cost_allocation_code, payment_priority | **yes** |

### Negative tests (reproduces empty UI)

| Scenario | API result | UI symptom |
| -------- | ---------- | ---------- |
| Wrong `X-Tenant-ID` (`00000000-0000-4000-8000-000000000001`) | **0 form templates** | «No templates found» on `/low-code/form-templates` |
| Tenant UUID used as `entity_id` | **0 custom field values** | «No custom fields found» on lookup page or entity panel |
| Direct low-code-service `/v1/low-code/form-templates` with dev tenant | **3 templates** | Gateway routing OK |

### Login API

```http
POST /api/v1/auth/login
{ "tenant_id": "74519f22-ff9b-4a8b-8fff-a958c689682f", "email": "admin@7rights.local", "password": "Admin123456!" }
```

**Result:** **OK** — `user.tenant_id` = `74519f22-ff9b-4a8b-8fff-a958c689682f`.

## UI Login Check

Frontend tenant flow (code review, no changes):

1. **Login page** — Tenant ID field → `useAuth().login()` → `tenantStore.setTenant()` → saved to `localStorage` key `freight_admin_tenant_id`.
2. **All API calls** — `useApi()` adds header `X-Tenant-ID: {tenantStore.tenantId}` and low-code composable adds query `tenant_id`.
3. **Header** — shows truncated current tenant (`74519f22-...`) with **Change tenant** modal.
4. **No `.env`** with `NUXT_PUBLIC_DEFAULT_TENANT_ID` in repo — tenant must be entered at login unless already in `localStorage`.

**Important:** Low-code pages do **not** have a tenant search box. `/low-code/form-templates` only filters by **entity_type** (select). `/low-code/custom-field-values` has **Entity ID** (entity UUID), **not** Tenant ID.

## Pages Checked

| Page | URL | Expected under correct tenant |
| ---- | --- | ----------------------------- |
| Low-code hub | `/low-code` | Service status + links |
| Form templates | `/low-code/form-templates` | 3 rows |
| Custom field lookup | `/low-code/custom-field-values` | Values after **Use demo entity** or correct entity UUID |
| Transport order detail | `/transport-orders/2db04b49-665c-469f-bcb1-ffeb1274fedb` | Custom fields panel, 3 fields |
| Shipment detail | `/shipments/14d405e2-0152-4030-b356-eec464a3cc66` | Custom fields panel, 3 fields |
| Billing register detail | `/billing-registers/cf7dbc77-395f-42a2-9717-476e4cd93796` | Custom fields panel, 3 fields |

Automated browser run (Playwright) was **not completed** in this session (browser binary install lock/timeout). API verification uses the **same endpoints and tenant header** as the UI.

## Expected Low-code Fields

See API Checks table above.

## Actual Result

| Check | Result |
| ----- | ------ |
| Backend + seed | **Pass** |
| Gateway API with dev tenant | **Pass** — data present |
| Login API | **Pass** |
| Wrong tenant / wrong entity_id reproduction | **Pass** — explains empty UI |
| Browser UI (automated) | **Not run** — manual check recommended |
| Browser UI (expected with correct login) | **Should pass** — API data + component wiring confirmed |

## Browser Console / Network Errors

Not captured (no successful headless browser session). When debugging manually, check DevTools **Network**:

- Request must include `X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f`
- `GET /api/v1/low-code/form-templates` → 200, `items.length === 3`
- `GET /api/v1/low-code/custom-field-values?entity_type=...&entity_id=...` → 200, non-empty `items` for demo UUIDs

Typical errors if empty:

- **200 + empty items** — wrong tenant header or wrong entity_id (not a bug)
- **401/403** — auth/session issue; re-login
- **502/503** — backend/low-code-service down

## Root Cause If Empty

1. **Tenant UUID pasted into Entity ID field** on `/low-code/custom-field-values`  
   API accepts the UUID format but returns **0 values** (verified). UI shows «No custom fields found».

2. **Session uses wrong tenant** — stale `freight_admin_tenant_id` in `localStorage`, or login without correct Tenant ID field.  
   Form templates list returns **0 items** for other tenants.

3. **Tenant ID confused with search** — user may have expected a global search; low-code list has only **entity_type** filter, not tenant search.

4. **Not a backend/seed issue** — all seeds and API calls pass for dev tenant.

## Recommended Fix

**No code change required for v0.1.** User workflow:

1. Log out → open `/login`
2. Enter **Tenant ID** = `74519f22-ff9b-4a8b-8fff-a958c689682f` (first field, not Entity ID)
3. Login `admin@7rights.local` / `Admin123456!`
4. Confirm header shows `74519f22-...`
5. Open entity detail URLs directly (UUIDs above) — Custom fields panel should populate
6. On lookup page: use **Use demo entity** or paste **entity UUID**, not tenant UUID

Optional future UX improvements (separate pack):

- Warn if `entity_id === tenantId`
- Clearer label: «Entity UUID (not Tenant ID)»
- Prefill `NUXT_PUBLIC_DEFAULT_TENANT_ID` in local `.env`

## Next Action

1. Manual browser smoke with steps above (5 min)
2. If still empty with correct header tenant → capture Network tab for one failing request
3. Consider docs-only update to `apps/web-admin/README.md` with tenant vs entity_id note (optional)

## Verification Commands

```powershell
cd D:\Projects\freight-platform
make health-check
make seed-dev-admin
make seed-demo-data
make seed-lowcode-demo

# Form templates
curl.exe -s -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" http://localhost:8080/api/v1/low-code/form-templates

# Custom field values (transport order)
curl.exe -s -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" "http://localhost:8080/api/v1/low-code/custom-field-values?entity_type=TRANSPORT_ORDER&entity_id=2db04b49-665c-469f-bcb1-ffeb1274fedb"

cd apps/web-admin
npm run dev
```

Manual URLs after login:

- http://localhost:3000/low-code/form-templates
- http://localhost:3000/transport-orders/2db04b49-665c-469f-bcb1-ffeb1274fedb
