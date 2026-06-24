# Low-code Pilot Week-2 BILLING_REGISTER Write Validation Design v0.1

## Summary

Design pack for **controlled BILLING_REGISTER custom field write validation** before enabling runtime PUT for pilot users. Builds on read-only validation (`LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_READONLY_VALIDATION_V0.1.md` — **GO_WITH_CONDITIONS**).

**This pack is design/docs only** — no BILLING_REGISTER PUT, migration execute, import execute, or template publish is performed here.

**Next step:** execute controlled writes using `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_VALIDATION_CHECKLIST_V0.1.md` on dev/staging with explicit approval.

## Current Commit

| Field | Value |
|-------|-------|
| HEAD | `72a2ff6` — `docs: add billing register read-only pilot validation` |
| Design date | 2026-06-24 |
| Branch | `main` |
| Code changed in this pack | **no** |

## Scope

**In scope**

- Write validation objectives and preconditions
- Demo/test entity and allowed fields
- Financial safety rules
- Sample PUT payloads (documentation only — verify against current PUT contract before execution)
- Pre-write and post-write checks
- Audit requirements
- Frontend validation plan
- Rollback and stop conditions

**Out of scope**

- Actual PUT execution (Controlled Write Validation Pack)
- BILLING_REGISTER user-facing rollout
- API contract changes
- Migrations
- Batch migration execute
- Template publish
- Billing/payment status changes

## Evidence Documents

| Document | Status | Notes |
|----------|--------|-------|
| `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_READONLY_VALIDATION_V0.1.md` | **Found** | GO_WITH_CONDITIONS |
| `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_SCOPE_EXPANSION_NOTE_V0.1.md` | **Found** | Write restricted until design + execute |
| `LOW_CODE_ENTITY_INTEGRATION_V0.2.md` | **Found** | BILLING_REGISTER validation_context builder |
| `LOW_CODE_RUNTIME_PILOT_STAGING_CHECKLIST_V0.1.md` | **Found** | Staging checklist |
| `LOW_CODE_PILOT_RELEASE_PACKAGE_V0.1.md` | **Found** | Release package |
| `LOW_CODE_PILOT_OPERATOR_CHECKLIST_V0.1.md` | **Found** | Operator checklist |

**Missing evidence docs:** none.

## Preconditions

All must be true before Controlled Write Validation Pack:

| # | Precondition |
|---|--------------|
| 1 | Read-only validation **PASS** (`LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_READONLY_VALIDATION_V0.1.md`) |
| 2 | TRANSPORT_ORDER + monitored SHIPMENT pilot **stable** — no open P0 |
| 3 | Active BILLING_REGISTER template PUBLISHED: `billing_register_default` |
| 4 | Auth-on verified on **staging** (if staging execute) |
| 5 | Operator + product sign-off for **single-entity** write test |
| 6 | Export backup of `billing_register_default` template JSON |
| 7 | Rollback plan acknowledged (restore payload or seed — no manual SQL) |
| 8 | Audit baseline captured |
| 9 | Core billing register baseline captured (status, totals, UPD) |

## Demo/Test Entity

| Field | Value |
|-------|-------|
| **tenant_id** | `74519f22-ff9b-4a8b-8fff-a958c689682f` |
| **entity_type** | `BILLING_REGISTER` |
| **entity_id** | `cf7dbc77-395f-42a2-9717-476e4cd93796` |
| **demo name** | **DEMO-BR-001** |
| **template_code** | `billing_register_default` |
| **template_id** (dev) | `b3333333-3333-4333-8333-333333333302` |
| **Detail URL** | `http://localhost:3000/billing-registers/cf7dbc77-395f-42a2-9717-476e4cd93796` |
| **Gateway API** | `http://localhost:8080/api/v1` |

### Core entity baseline (dev, pre-write — do not change via low-code)

| Field | Baseline value |
|-------|----------------|
| register_number | DEMO-BR-001 |
| status | `CLOSING_DOCUMENTS_CREATED` |
| total_with_vat | 119400 |
| total_without_vat | 99500 |
| currency_code | RUB |
| period_from / period_to | 2026-08-01 / 2026-08-15 |

## Fields Allowed For Validation

| field_code | Type | field_id (dev seed) | Baseline value |
|------------|------|---------------------|----------------|
| cost_allocation_code | TEXT | `b3333333-3333-4333-8333-333333333304` | `FIN-LOG-001` |
| approval_group | SELECT | `b3333333-3333-4333-8333-333333333305` | `LOGISTICS_FINANCE` |
| payment_priority | SELECT | `b3333333-3333-4333-8333-333333333306` | `NORMAL` |

**Only these 3 field_codes** may be changed during write validation.

## Financial Safety Rules

| Rule | Requirement |
|------|-------------|
| Custom fields ≠ financial source of truth | Core billing register API remains authoritative for totals, status, UPD |
| No status transitions via low-code | Do not attempt to change `status`, payment state, or UPD status through custom fields |
| No invoice/UPD/ act operations | PUT must not trigger or substitute billing document workflows |
| validation_context advisory only | `amount`, `currency`, `entity_status` in context are for conditional rules — not payment commands |
| Demo entity only | No production billing registers |
| Audit mandatory | Every PUT must produce audit event |
| Pre/post core GET | Compare billing register core GET before and after — status and totals unchanged |

## Operations Allowed

- `GET .../form-templates/active` for BILLING_REGISTER
- `GET .../custom-field-values` for DEMO-BR-001
- `GET .../billing-registers/{id}` — core entity baseline
- `GET .../audit-events`
- Admin template list + export (read-only)
- **Execute pack only:** one controlled `PUT .../custom-field-values` with approved payload
- Rollback via restore payload or idempotent `make seed-lowcode-demo` (dev)

## Operations Forbidden

- Broad BILLING_REGISTER production rollout
- Production BILLING_REGISTER writes
- Billing/payment status changes through low-code
- migration execute
- batch migration execute
- import execute
- template publish / edit
- manual DB edits
- Committing `LOW_CODE_ADMIN_AUTH_ENABLED=true`

## Pre-write Checks

```powershell
cd D:\Projects\freight-platform
make health-check
make seed-lowcode-demo

$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"
$GW = "http://localhost:8080/api/v1"
$E = "cf7dbc77-395f-42a2-9717-476e4cd93796"

# Active template
curl.exe -i -H "X-Tenant-ID: $T" `
  "$GW/low-code/form-templates/active?entity_type=BILLING_REGISTER&template_code=billing_register_default"

# Current custom values
curl.exe -i -H "X-Tenant-ID: $T" `
  "$GW/low-code/custom-field-values?entity_type=BILLING_REGISTER&entity_id=$E&template_code=billing_register_default"

# Core billing register baseline
curl.exe -i -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/billing-registers/$E"

# Audit before
curl.exe -i -H "X-Tenant-ID: $T" `
  "$GW/low-code/audit-events?limit=20"
```

Save GET outputs as rollback baseline. Update restore payload if values differ from placeholder.

## Proposed Write Payload

**File:** `scripts/dev/payloads/lowcode_billing_register_write_validation_demo.json`

**Verify against current PUT contract before execution.**

| Check | Value |
|-------|-------|
| entity_type | BILLING_REGISTER |
| entity_id | `cf7dbc77-395f-42a2-9717-476e4cd93796` |
| form_template_id | `b3333333-3333-4333-8333-333333333302` |
| Fields in PUT | 3 — all allowed field_codes |
| Core billing fields | **none** |
| validation_context | entity_status, period, amount, currency from core GET |

### Proposed test values (demo only)

| field_code | Baseline | Proposed test value |
|------------|----------|---------------------|
| cost_allocation_code | FIN-LOG-001 | FIN-PILOT-002 |
| approval_group | LOGISTICS_FINANCE | FINANCE_OPS |
| payment_priority | NORMAL | HIGH |

## Post-write Checks

| # | Check | Pass criteria |
|---|-------|---------------|
| 1 | PUT response | HTTP **200** (or documented success) |
| 2 | GET after PUT | Updated values match PUT payload |
| 3 | All 3 field_codes present | None disappeared |
| 4 | Audit event | `CUSTOM_FIELD_VALUES_UPDATED` for entity |
| 5 | Active template | Still `billing_register_default` PUBLISHED |
| 6 | No migration audit | No unexpected migration/import/publish events |
| 7 | Core billing register GET | **status unchanged**; totals unchanged |
| 8 | UPD/invoices | No unexpected status/version change |
| 9 | integration-smoke-test | **PASS** |
| 10 | npm build | **PASS** |
| 11 | UI panel | BILLING_REGISTER detail still renders |

## Audit Requirements

| Requirement | Detail |
|-------------|--------|
| Event type | `CUSTOM_FIELD_VALUES_UPDATED` |
| Entity match | `entity_type=BILLING_REGISTER`, `entity_id=cf7dbc77-...` |
| Timing | Visible within audit GET immediately after PUT |
| request_id | Capture if present for correlation |
| Gap check | Write without audit → **P0 stop** |

## Frontend Validation Plan

Execute pack UI check (manual):

1. Login: `admin@7rights.local` / `Admin123456!`
2. Open `http://localhost:3000/billing-registers/cf7dbc77-395f-42a2-9717-476e4cd93796`
3. Verify low-code panel renders 3 fields with baseline values
4. **Perform write only if explicitly approved in Execute Pack**
5. Reload page — verify values after write
6. Check browser console — no critical errors
7. Verify `validation_context` does not crash panel
8. Verify core status badge and totals unchanged on page
9. Optional: `/low-code/custom-field-values` — BILLING_REGISTER filter loads DEMO-BR-001

## Stop Conditions

**P0 — stop validation immediately:**

| # | Condition |
|---|-----------|
| 1 | PUT writes to wrong tenant |
| 2 | PUT writes to wrong entity |
| 3 | Audit missing after write |
| 4 | Core billing register **status** changes unexpectedly |
| 5 | Payment status changes unexpectedly |
| 6 | Shipment financial status changes unexpectedly |
| 7 | Active template changes unexpectedly |
| 8 | low-code-service repeated **5xx** |
| 9 | Custom values disappear after PUT |
| 10 | validation_context causes false required error blocking save |
| 11 | Runtime API 401/403 unexpectedly in default-off dev mode |
| 12 | UI crashes after write |
| 13 | integration-smoke-test fails after write |

**On P0:** stop validation → **Low-code Runtime Pilot Fix Pack v0.1** — do not broaden BILLING_REGISTER write.

## Rollback / Recovery Plan

| Step | Action |
|------|--------|
| 1 | **Stop** further BILLING_REGISTER writes |
| 2 | **Inspect audit** — entity_id, changed_fields, timestamp |
| 3 | **Compare GET** custom values + core billing register before/after |
| 4 | **Restore custom values** via PUT using restore payload from pre-write GET: |
| | `scripts/dev/payloads/lowcode_billing_register_write_validation_restore_placeholder.json` |
| | Update placeholder from baseline snapshot if values differ |
| 5 | **Dev only:** `make seed-lowcode-demo` may skip if values exist — use explicit restore PUT if seed is idempotent skip |
| 6 | **Never** manual DB edits except emergency DBA |
| 7 | **Do not** publish template or run migration as rollback |
| 8 | **Do not** alter billing/payment status as rollback unless billing service supports explicit safe flow |

## Security Review

| Check | Design status |
|-------|---------------|
| Demo/test entity only | **yes** |
| Tenant header required | **yes** (`X-Tenant-ID`) |
| No production data in payloads | **yes** |
| No import execute | **yes** |
| No migration execute | **yes** |
| No batch execute | **yes** |
| No template publish | **yes** |
| No manual DB edit | **yes** |
| Audit required after write | **yes** |
| Source of truth | Tenant-scoped billing register API |
| validation_context advisory | **yes** — not financial source of truth |
| Custom fields must not drive payment/billing status | **documented** |

## Decision

| Field | Value |
|-------|-------|
| **Decision** | **READY_FOR_CONTROLLED_WRITE_VALIDATION** |
| Read-only evidence | **Present** — GO_WITH_CONDITIONS |
| Blockers | **None** |
| PUT in this pack | **no** |

## Next Action

**Low-code Pilot Week-2 BILLING_REGISTER Controlled Write Validation Pack v0.1**

Reference:

- `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_VALIDATION_COMMANDS_V0.1.md`
- `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_VALIDATION_CHECKLIST_V0.1.md`
- `scripts/dev/payloads/lowcode_billing_register_write_validation_demo.json`
- `scripts/dev/payloads/lowcode_billing_register_write_validation_restore_placeholder.json`

If P0 during design review:

**Low-code Runtime Pilot Fix Pack v0.1**

## Verification Run (this pack)

| Check | Result | Run ID / notes |
|-------|--------|----------------|
| `make health-check` | **PASS** | All 9 services OK |
| `make seed-lowcode-demo` | **PASS** | BILLING_REGISTER values exist |
| `make integration-smoke-test` | **PASS** | `TEST-20260624182920` |
| `npm run build` | **PASS** | web-admin build complete |
| PUT in this pack | **no** | |
