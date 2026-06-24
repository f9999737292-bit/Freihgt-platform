# Low-code Pilot Week-2 SHIPMENT Write Validation Checklist v0.1

## Summary

Operator checklist for **controlled SHIPMENT custom field write validation** on a single demo entity. Use with design doc and commands reference before enabling SHIPMENT writes for pilot users.

**This document does not perform writes** — it guides the Execute Pack.

Reference:

- `LOW_CODE_PILOT_WEEK2_SHIPMENT_WRITE_VALIDATION_DESIGN_V0.1.md`
- `LOW_CODE_PILOT_WEEK2_SHIPMENT_WRITE_VALIDATION_COMMANDS_V0.1.md`

## Purpose

- Validate SHIPMENT PUT (API + UI) on one entity with rollback
- Confirm audit events after writes
- Confirm rich fields and visibility rules behave as expected
- Gate SHIPMENT user-facing rollout until sign-off

**Not for production.** Prefer staging pilot tenant; dev demo below.

## Preconditions

- [ ] Read-only validation **PASS** (`LOW_CODE_PILOT_WEEK2_SHIPMENT_READONLY_VALIDATION_V0.1.md`)
- [ ] Design doc reviewed (`LOW_CODE_PILOT_WEEK2_SHIPMENT_WRITE_VALIDATION_DESIGN_V0.1.md`)
- [ ] Product + operator sign-off for **single-entity** write test
- [ ] TRANSPORT_ORDER pilot stable — no open P0
- [ ] Auth-on verified on staging (if staging execute)
- [ ] Template exported (`shipment_default` JSON with date)
- [ ] `make health-check` passes
- [ ] Payloads available:
  - `scripts/dev/payloads/lowcode_shipment_write_validation_demo.json`
  - `scripts/dev/payloads/lowcode_shipment_write_validation_restore_placeholder.json`

## Demo/Test Entity

| Field | Value |
|-------|-------|
| **tenant_id** | `74519f22-ff9b-4a8b-8fff-a958c689682f` |
| **entity_type** | `SHIPMENT` |
| **entity_id** | `14d405e2-0152-4030-b356-eec464a3cc66` |
| **template_code** | `shipment_default` |
| **template_id** (dev) | `b2222222-2222-4222-8222-222222222202` |
| **demo name** | **DEMO-SH-PLANNED** |

Detail URL (dev): `http://localhost:3000/shipments/14d405e2-0152-4030-b356-eec464a3cc66`

## Allowed Fields

Only these `shipment_default` field codes may be changed during write validation:

| field_code | Type |
|------------|------|
| `temperature_mode` | SELECT |
| `loading_contact_phone` | TEXT |
| `driver_comment` | TEXT |
| `planned_pickup_date` | DATE |
| `declared_value` | MONEY |
| `handling_flags` | MULTI_SELECT |

## Forbidden Operations

Do **not** perform during this checklist (except controlled rollback PUT):

- **production writes** (non-approved tenants/environments)
- **migration execute**
- **batch migration execute**
- **import execute**
- **template publish**
- **manual DB edits**
- **destructive Docker commands** (volume prune, reset/purge)
- Writes to entities other than approved test entity without sign-off
- Committing `LOW_CODE_ADMIN_AUTH_ENABLED=true`

## Before Write Checklist

- [ ] Record baseline GET custom values for demo entity
- [ ] Record baseline audit (`audit-events?entity_type=SHIPMENT&entity_id=...&limit=20`)
- [ ] Update restore payload from baseline if needed
- [ ] Active template GET → **200**, PUBLISHED, `shipment_default`

```powershell
$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"
curl.exe -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/low-code/form-templates/active?entity_type=SHIPMENT&template_code=shipment_default"
curl.exe -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/low-code/custom-field-values?entity_type=SHIPMENT&entity_id=14d405e2-0152-4030-b356-eec464a3cc66&template_code=shipment_default"
```

## Controlled PUT Checklist

Run **one scenario at a time**. GET verify after each PUT.

- [ ] **SH-W-01** — `loading_contact_phone` (TEXT) → test value; PUT **200**; GET matches
- [ ] **SH-W-02** — `temperature_mode` → `CHILLED`; note visibility for UI
- [ ] **SH-W-03** — `planned_pickup_date` → valid `YYYY-MM-DD`
- [ ] **SH-W-04** — `declared_value` → `{ amount, currency }`
- [ ] **SH-W-05** — `handling_flags` → valid array
- [ ] **SH-W-06** — Multi-field PUT (demo payload) — all fields saved
- [ ] **SH-W-08** — Idempotent repeat of prior PUT — no corruption
- [ ] **SH-W-09** (optional) — Invalid MONEY → **4xx**, readable error; GET unchanged

## After Write Checklist

- [ ] GET reflects all PUT changes for demo entity
- [ ] No unexpected fields modified
- [ ] No cross-tenant data in GET response
- [ ] Entity detail page loads with updated values (UI)
- [ ] Custom values page shows same values as entity detail

## Audit Checklist

- [ ] Audit GET after writes returns new events
- [ ] Events reference correct `entity_type=SHIPMENT` and `entity_id`
- [ ] Actor / request_id present if auth-on staging
- [ ] UI audit page `/low-code/audit` shows write events (optional filter)

```powershell
curl.exe -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/low-code/audit-events?entity_type=SHIPMENT&entity_id=14d405e2-0152-4030-b356-eec464a3cc66&limit=20"
```

## UI Spot-check Checklist

Login: `admin@7rights.local` / `Admin123456!`

- [ ] **SH-UI-01** — `/shipments/14d405e2-0152-4030-b356-eec464a3cc66` — panel renders; save works; reload persists
- [ ] **SH-UI-02** — Rich editors (date, money, multi-select) save correctly
- [ ] **SH-UI-03** — `temperature_mode` AMBIENT ↔ CHILLED — `driver_comment` visibility toggles
- [ ] **SH-UI-04** — `/low-code/custom-field-values` — SHIPMENT filter + save
- [ ] Save button disables during in-flight request
- [ ] Browser console — no critical errors

## Stop Conditions

**Stop immediately** and file P0 incident (`LOW_CODE_PILOT_OPERATOR_FEEDBACK_FORM_V0.1.md`):

- Cross-tenant data visible
- Wrong entity_id or tenant written
- Audit missing after PUT
- GET result ≠ PUT intent (data loss/corruption)
- Repeated low-code-service **5xx**
- UI save writes to wrong entity

Escalate: **Low-code Runtime Pilot Fix Pack v0.1**

## Rollback Checklist

- [ ] Restore payload updated from pre-test baseline
- [ ] PUT restore executed (`lowcode_shipment_write_validation_restore_placeholder.json`)
- [ ] GET confirms baseline values restored
- [ ] Audit records restore write (if applicable)
- [ ] Document rollback in daily report / feedback form

## Final Decision Checklist

| Item | Pass / Fail | Notes |
|------|-------------|-------|
| Before write checks | | |
| Controlled PUT (SH-W-01..06) | | |
| After write GET/UI | | |
| Audit | | |
| UI spot-check | | |
| Rollback | | |
| P0 incidents | **none** | |

### Decision

- [ ] **PASS** — SHIPMENT controlled write validation OK; internal use only (not user rollout)
- [ ] **FAIL** — do not expand SHIPMENT write scope

**Signed:** _______________ **Date:** _______________

## Commands

Full copy-paste commands: `docs/LOW_CODE_PILOT_WEEK2_SHIPMENT_WRITE_VALIDATION_COMMANDS_V0.1.md`

Quick reference:

```powershell
cd D:\Projects\freight-platform
make health-check

$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"

# Demo write (Execute Pack only — with sign-off)
curl.exe -X PUT -H "Content-Type: application/json" -H "X-Tenant-ID: $T" `
  --data-binary "@scripts/dev/payloads/lowcode_shipment_write_validation_demo.json" `
  "http://localhost:8080/api/v1/low-code/custom-field-values"

# Rollback
curl.exe -X PUT -H "Content-Type: application/json" -H "X-Tenant-ID: $T" `
  --data-binary "@scripts/dev/payloads/lowcode_shipment_write_validation_restore_placeholder.json" `
  "http://localhost:8080/api/v1/low-code/custom-field-values"
```
