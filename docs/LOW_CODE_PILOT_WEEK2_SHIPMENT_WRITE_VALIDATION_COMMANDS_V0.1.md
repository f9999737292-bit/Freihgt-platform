# Low-code Pilot Week-2 SHIPMENT Write Validation Commands v0.1

Quick command reference for **Execute Pack** only. Requires sign-off per `LOW_CODE_PILOT_WEEK2_SHIPMENT_WRITE_VALIDATION_DESIGN_V0.1.md`.

**Do not run PUT on production without approval.**

## Dev reference

| Item | Value |
|------|-------|
| Tenant | `74519f22-ff9b-4a8b-8fff-a958c689682f` |
| Gateway | `http://localhost:8080/api/v1` |
| Entity | DEMO-SH-PLANNED — `14d405e2-0152-4030-b356-eec464a3cc66` |
| Template ID | `b2222222-2222-4222-8222-222222222202` |

## Pre-write (read-only)

```powershell
cd D:\Projects\freight-platform
make health-check

$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"
$GW = "http://localhost:8080/api/v1"

curl.exe -H "X-Tenant-ID: $T" `
  "$GW/low-code/form-templates/active?entity_type=SHIPMENT&template_code=shipment_default"

curl.exe -H "X-Tenant-ID: $T" `
  "$GW/low-code/custom-field-values?entity_type=SHIPMENT&entity_id=14d405e2-0152-4030-b356-eec464a3cc66&template_code=shipment_default"

curl.exe -H "X-Tenant-ID: $T" `
  "$GW/low-code/audit-events?entity_type=SHIPMENT&entity_id=14d405e2-0152-4030-b356-eec464a3cc66&limit=20"
```

Save GET output as rollback baseline.

## Controlled write (demo payload)

```powershell
cd D:\Projects\freight-platform
$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"

curl.exe -X PUT `
  -H "Content-Type: application/json" `
  -H "X-Tenant-ID: $T" `
  --data-binary "@scripts/dev/payloads/lowcode_shipment_write_validation_demo.json" `
  "http://localhost:8080/api/v1/low-code/custom-field-values"
```

Verify:

```powershell
curl.exe -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/low-code/custom-field-values?entity_type=SHIPMENT&entity_id=14d405e2-0152-4030-b356-eec464a3cc66&template_code=shipment_default"
```

## Rollback restore

Update `lowcode_shipment_write_validation_restore_placeholder.json` from baseline GET if values differ, then:

```powershell
curl.exe -X PUT `
  -H "Content-Type: application/json" `
  -H "X-Tenant-ID: $T" `
  --data-binary "@scripts/dev/payloads/lowcode_shipment_write_validation_restore_placeholder.json" `
  "http://localhost:8080/api/v1/low-code/custom-field-values"
```

## Post-write audit

```powershell
curl.exe -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/low-code/audit-events?entity_type=SHIPMENT&entity_id=14d405e2-0152-4030-b356-eec464a3cc66&limit=20"
```

## UI spot-check

```powershell
cd D:\Projects\freight-platform\apps\web-admin
npm run dev
```

- `http://localhost:3000/shipments/14d405e2-0152-4030-b356-eec464a3cc66`
- `http://localhost:3000/low-code/custom-field-values`

Login: `admin@7rights.local` / `Admin123456!`

## Full checklist

See `docs/LOW_CODE_PILOT_WEEK2_SHIPMENT_WRITE_VALIDATION_CHECKLIST_V0.1.md`.
