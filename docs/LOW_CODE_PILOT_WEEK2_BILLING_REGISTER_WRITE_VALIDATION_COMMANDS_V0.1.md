# Low-code Pilot Week-2 BILLING_REGISTER Write Validation Commands v0.1

Quick command reference for **Controlled Write Validation Pack** only. Requires sign-off per `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_VALIDATION_DESIGN_V0.1.md`.

**DO NOT RUN PUT UNTIL EXPLICITLY APPROVED.**

## Purpose

Safe read-only and controlled-write command reference for BILLING_REGISTER custom field validation on demo entity **DEMO-BR-001**.

## Preconditions

- Read-only validation **PASS** (`LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_READONLY_VALIDATION_V0.1.md`)
- Design doc reviewed and signed off
- `make health-check` passes
- Baseline GET saved for rollback

## Dev reference

| Item | Value |
|------|-------|
| Tenant | `74519f22-ff9b-4a8b-8fff-a958c689682f` |
| Gateway | `http://localhost:8080/api/v1` |
| Entity | DEMO-BR-001 — `cf7dbc77-395f-42a2-9717-476e4cd93796` |
| Template ID | `b3333333-3333-4333-8333-333333333302` |
| Template code | `billing_register_default` |

## Safe Commands

```powershell
cd D:\Projects\freight-platform
make health-check
make seed-lowcode-demo
make integration-smoke-test
```

## Pre-write GET

```powershell
$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"
$GW = "http://localhost:8080/api/v1"
$E = "cf7dbc77-395f-42a2-9717-476e4cd93796"

curl.exe -i -H "X-Tenant-ID: $T" `
  "$GW/low-code/form-templates/active?entity_type=BILLING_REGISTER&template_code=billing_register_default"

curl.exe -i -H "X-Tenant-ID: $T" `
  "$GW/low-code/custom-field-values?entity_type=BILLING_REGISTER&entity_id=$E&template_code=billing_register_default"

curl.exe -i -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/billing-registers/$E"

curl.exe -i -H "X-Tenant-ID: $T" `
  "$GW/low-code/audit-events?limit=20"
```

Save GET output as rollback baseline. Update restore payload if values differ.

## Write Payload

File: `scripts/dev/payloads/lowcode_billing_register_write_validation_demo.json`

Verify against current PUT contract before execution.

## PUT Command Placeholder

> **DO NOT RUN UNTIL EXPLICITLY APPROVED** in Controlled Write Validation Pack.

```powershell
cd D:\Projects\freight-platform
$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"

curl.exe -X PUT `
  -H "Content-Type: application/json" `
  -H "X-Tenant-ID: $T" `
  --data-binary "@scripts/dev/payloads/lowcode_billing_register_write_validation_demo.json" `
  "http://localhost:8080/api/v1/low-code/custom-field-values"
```

## Post-write GET

```powershell
curl.exe -i -H "X-Tenant-ID: $T" `
  "$GW/low-code/custom-field-values?entity_type=BILLING_REGISTER&entity_id=$E&template_code=billing_register_default"

curl.exe -i -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/billing-registers/$E"
```

## Audit Check

```powershell
curl.exe -i -H "X-Tenant-ID: $T" `
  "$GW/low-code/audit-events?entity_type=BILLING_REGISTER&entity_id=$E&limit=20"
```

Expect: `CUSTOM_FIELD_VALUES_UPDATED` for entity after approved PUT.

## Financial Safety Checks

After approved PUT only:

```powershell
# Core register — status and totals must be unchanged
curl.exe -s -H "X-Tenant-ID: $T" "http://localhost:8080/api/v1/billing-registers/$E" | python -c "import sys,json; r=json.load(sys.stdin); print('status', r.get('status')); print('total_with_vat', r.get('total_with_vat')); print('version', r.get('version'))"
```

| Check | Expected |
|-------|----------|
| status | `CLOSING_DOCUMENTS_CREATED` (unchanged) |
| total_with_vat | 119400 (unchanged) |
| UPD status | No unexpected change |

## Rollback restore

Update `lowcode_billing_register_write_validation_restore_placeholder.json` from baseline GET if needed:

```powershell
curl.exe -X PUT `
  -H "Content-Type: application/json" `
  -H "X-Tenant-ID: $T" `
  --data-binary "@scripts/dev/payloads/lowcode_billing_register_write_validation_restore_placeholder.json" `
  "http://localhost:8080/api/v1/low-code/custom-field-values"
```

## Verification

```powershell
cd D:\Projects\freight-platform
make health-check
make integration-smoke-test

cd apps\web-admin
npm run build
```

## UI spot-check

```powershell
cd D:\Projects\freight-platform\apps\web-admin
npm run dev
```

- `http://localhost:3000/billing-registers/cf7dbc77-395f-42a2-9717-476e4cd93796`
- `http://localhost:3000/low-code/custom-field-values`

Login: `admin@7rights.local` / `Admin123456!`

## Stop Conditions

Stop immediately (P0) if:

- Wrong tenant or entity write
- Audit missing after write
- Core billing register status/totals changed
- Payment or shipment financial status changed
- Active template changed unexpectedly
- Repeated low-code-service 5xx
- Custom values disappeared
- integration-smoke-test fails after write

Escalate: **Low-code Runtime Pilot Fix Pack v0.1**

## Full checklist

See `docs/LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_VALIDATION_CHECKLIST_V0.1.md`.
