# Low-code Pilot Week-2 BILLING_REGISTER Write Validation Checklist v0.1

## Summary

Operator checklist for **controlled BILLING_REGISTER custom field write validation** on demo entity **DEMO-BR-001**. Use with design doc and commands reference before enabling BILLING_REGISTER writes for pilot users.

**This document does not perform writes** — it guides the Controlled Write Validation Pack.

Reference:

- `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_VALIDATION_DESIGN_V0.1.md`
- `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_VALIDATION_COMMANDS_V0.1.md`

## Purpose

- Validate BILLING_REGISTER PUT (API + UI) on one demo entity with rollback
- Confirm audit events after writes
- Confirm **no** core billing/payment/financial side effects
- Gate BILLING_REGISTER user-facing rollout until sign-off

**Not for production.** Prefer staging pilot tenant; dev demo below.

## Preconditions

- [ ] Read-only validation **PASS** (`LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_READONLY_VALIDATION_V0.1.md`)
- [ ] Design doc reviewed (`LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_VALIDATION_DESIGN_V0.1.md`)
- [ ] Product + operator sign-off for **single-entity** write test
- [ ] TRANSPORT_ORDER + SHIPMENT pilot stable — no open P0
- [ ] Auth-on verified on staging (if staging execute)
- [ ] Template exported (`billing_register_default` JSON with date)
- [ ] `make health-check` passes
- [ ] Payloads available:
  - `scripts/dev/payloads/lowcode_billing_register_write_validation_demo.json`
  - `scripts/dev/payloads/lowcode_billing_register_write_validation_restore_placeholder.json`

## Demo/Test Entity

| Field | Value |
|-------|-------|
| **tenant_id** | `74519f22-ff9b-4a8b-8fff-a958c689682f` |
| **entity_type** | `BILLING_REGISTER` |
| **entity_id** | `cf7dbc77-395f-42a2-9717-476e4cd93796` |
| **template_code** | `billing_register_default` |
| **template_id** (dev) | `b3333333-3333-4333-8333-333333333302` |
| **demo name** | **DEMO-BR-001** |

Detail URL (dev): `http://localhost:3000/billing-registers/cf7dbc77-395f-42a2-9717-476e4cd93796`

## Allowed Fields

Only these `billing_register_default` field codes may be changed during write validation:

| field_code | Type |
|------------|------|
| `cost_allocation_code` | TEXT |
| `approval_group` | SELECT |
| `payment_priority` | SELECT |

## Forbidden Operations

Do **not** perform during this checklist (except controlled rollback PUT):

- **production writes** (non-approved tenants/environments)
- **billing/payment status changes** through low-code
- **migration execute**
- **batch migration execute**
- **import execute**
- **template publish**
- **manual DB edits**
- **destructive Docker commands**
- Writes to entities other than DEMO-BR-001 without sign-off
- Committing `LOW_CODE_ADMIN_AUTH_ENABLED=true`

## Financial Safety Checklist

Before and after any approved PUT:

- [ ] Core billing register GET recorded (status, totals, version)
- [ ] Status remains `CLOSING_DOCUMENTS_CREATED` (or documented baseline)
- [ ] `total_with_vat` / `total_without_vat` unchanged
- [ ] UPD documents — no unexpected status change
- [ ] Invoices/acts — no unexpected change
- [ ] Shipment linked item — no financial status change via low-code
- [ ] validation_context treated as advisory only

## Before Write Checklist

- [ ] Record baseline GET custom values for DEMO-BR-001
- [ ] Record baseline core billing register GET
- [ ] Record baseline audit (`audit-events?limit=20`)
- [ ] Update restore payload from baseline if needed
- [ ] Active template GET → **200**, PUBLISHED, `billing_register_default`

```powershell
$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"
$E = "cf7dbc77-395f-42a2-9717-476e4cd93796"
curl.exe -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/low-code/form-templates/active?entity_type=BILLING_REGISTER&template_code=billing_register_default"
curl.exe -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/low-code/custom-field-values?entity_type=BILLING_REGISTER&entity_id=$E&template_code=billing_register_default"
curl.exe -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/billing-registers/$E"
```

## Controlled PUT Checklist

> **DO NOT RUN PUT UNTIL EXPLICITLY APPROVED.**

Run **one controlled PUT** using demo payload. GET verify after PUT.

- [ ] **BR-W-01** — Demo payload (3 fields) → PUT **200**; GET matches
- [ ] **BR-W-02** — `cost_allocation_code` only (optional isolated test)
- [ ] **BR-W-03** — `approval_group` only (optional)
- [ ] **BR-W-04** — `payment_priority` only (optional)
- [ ] **BR-W-05** — validation_context present → no server error
- [ ] **BR-W-06** — Idempotent repeat → no corruption

```powershell
# DO NOT RUN UNTIL EXPLICITLY APPROVED
curl.exe -X PUT `
  -H "Content-Type: application/json" `
  -H "X-Tenant-ID: $T" `
  --data-binary "@scripts/dev/payloads/lowcode_billing_register_write_validation_demo.json" `
  "http://localhost:8080/api/v1/low-code/custom-field-values"
```

## After Write Checklist

- [ ] GET custom values — all 3 field_codes present; values updated
- [ ] Core billing register GET — **status unchanged**
- [ ] Core totals unchanged (`total_with_vat`, `total_without_vat`)
- [ ] UPD/invoices unchanged
- [ ] Audit shows `CUSTOM_FIELD_VALUES_UPDATED`
- [ ] No migration/import/publish audit events
- [ ] Active template unchanged
- [ ] `make integration-smoke-test` **PASS**
- [ ] Entry in pilot daily report (if applicable)

## Audit Checklist

- [ ] Audit GET after PUT returns new event
- [ ] Event entity_type = BILLING_REGISTER
- [ ] Event entity_id = `cf7dbc77-395f-42a2-9717-476e4cd93796`
- [ ] request_id captured if present
- [ ] No audit gap (write without event → **P0**)

## UI Spot-check Checklist

Manual (Execute Pack):

- [ ] Open `/billing-registers/cf7dbc77-395f-42a2-9717-476e4cd93796`
- [ ] Low-code panel renders 3 fields
- [ ] Values match GET after write
- [ ] Core status badge unchanged
- [ ] Totals on page unchanged
- [ ] Browser console — no critical errors
- [ ] `/low-code/custom-field-values` — BILLING_REGISTER + DEMO-BR-001 loads
- [ ] `/low-code/admin/form-templates` — BILLING_REGISTER visible (read-only)

## Stop Conditions

**P0 — stop immediately:**

- [ ] Wrong tenant write
- [ ] Wrong entity write
- [ ] Audit missing after write
- [ ] Core billing register status changed
- [ ] Payment status changed
- [ ] Shipment financial status changed
- [ ] Active template changed unexpectedly
- [ ] low-code-service repeated 5xx
- [ ] Custom values disappeared
- [ ] validation_context false required error
- [ ] UI crash after write
- [ ] integration-smoke-test fails after write

Escalate: **Low-code Runtime Pilot Fix Pack v0.1**

## Rollback Checklist

- [ ] Stop further BILLING_REGISTER writes
- [ ] Inspect audit before correction
- [ ] Update restore payload from pre-write GET if needed
- [ ] PUT restore payload → GET confirms baseline values
- [ ] Core billing register GET unchanged after restore
- [ ] **No** manual DB edits
- [ ] **No** template publish as rollback
- [ ] **No** migration execute as rollback

## Final Decision Checklist

| Criterion | Met |
|-----------|-----|
| Controlled PUT succeeded | |
| Audit present | |
| No financial side effects | |
| No P0 stop conditions | |
| Rollback tested or documented | |
| UI spot-check OK | |

**Decision:** GO_WITH_CONDITIONS / NO_GO / STOPPED

## Commands

See `docs/LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_VALIDATION_COMMANDS_V0.1.md`.
