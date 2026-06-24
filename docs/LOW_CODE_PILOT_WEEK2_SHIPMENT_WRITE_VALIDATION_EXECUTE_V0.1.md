# Low-code Pilot Week-2 SHIPMENT Write Validation Execute v0.1

## Summary

Controlled **SHIPMENT custom field values PUT** executed once on demo entity **DEMO-SH-PLANNED**. PUT **succeeded** (HTTP 200, `saved_count: 5`). Post-write GET, audit, and active template checks **passed**. No stop conditions triggered.

**Decision: GO_WITH_CONDITIONS** — controlled internal SHIPMENT write validation succeeded. Broad production SHIPMENT rollout **not approved**.

## Current Commit

| Field | Value |
|-------|-------|
| Baseline HEAD | `95d3983` — `docs: add shipment write validation checklist` |
| Execute date | 2026-06-24 |
| Branch | `main` |
| Backend / frontend changed | **no** |

## Scope

**Executed**

- One controlled PUT on demo SHIPMENT entity
- GET before/after, audit before/after, active template after
- Regression: health-check, integration smoke, npm build

**Not executed**

- Production writes
- migration / batch / import execute
- template publish
- manual DB edits
- UI browser session (API + static route evidence only)

## Demo/Test Entity

| Field | Value |
|-------|-------|
| tenant_id | `74519f22-ff9b-4a8b-8fff-a958c689682f` |
| entity_type | SHIPMENT |
| entity_id | `14d405e2-0152-4030-b356-eec464a3cc66` |
| demo name | DEMO-SH-PLANNED |
| template_code | shipment_default |
| form_template_id | `b2222222-2222-4222-8222-222222222202` |

## Payload Used

File: `scripts/dev/payloads/lowcode_shipment_write_validation_demo.json`

| Check | Result |
|-------|--------|
| entity_type | SHIPMENT |
| entity_id | `14d405e2-0152-4030-b356-eec464a3cc66` |
| form_template_id | `b2222222-2222-4222-8222-222222222202` |
| Allowed field_codes only | **yes** (5 fields) |
| Core shipment fields | **none** |
| Payload format | **unchanged** — matched PUT contract |

Fields in PUT:

- `loading_contact_phone` → `+7 900 111-22-33`
- `planned_pickup_date` → `2026-09-15`
- `declared_value` → `{ amount: 140000, currency: RUB }`
- `handling_flags` → `["FRAGILE"]`
- `temperature_mode` → `CHILLED`

`driver_comment` **not** in payload — preserved from baseline.

## Baseline Results

### Active template (before)

| Field | Value |
|-------|-------|
| HTTP | **200** |
| code | `shipment_default` |
| status | PUBLISHED |
| version | 1 |
| is_active | true |

### GET before write

| Field | Value |
|-------|-------|
| HTTP | **200** |
| Values present | **yes** — 6 fields |
| field_codes | `declared_value`, `driver_comment`, `handling_flags`, `loading_contact_phone`, `planned_pickup_date`, `temperature_mode` |

Before snapshot (key values):

| field_code | value (before) |
|------------|----------------|
| temperature_mode | AMBIENT |
| loading_contact_phone | +7 900 000-00-01 |
| planned_pickup_date | 2026-08-15 |
| declared_value | { amount: 125000, currency: RUB } |
| handling_flags | ["FRAGILE"] |
| driver_comment | Позвонить за 1 час до прибытия |

### Audit before (global limit=20)

| Field | Value |
|-------|-------|
| HTTP | **200** |
| Latest events | Mostly `FORM_TEMPLATE_EXPORTED` / import preview — no recent CUSTOM_FIELD_VALUES_UPDATED for demo entity |

## PUT Execution Result

```powershell
curl.exe -i -X PUT -H "Content-Type: application/json" `
  -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  --data-binary "@scripts/dev/payloads/lowcode_shipment_write_validation_demo.json" `
  "http://localhost:8080/api/v1/low-code/custom-field-values"
```

| Field | Value |
|-------|-------|
| HTTP | **200 OK** |
| Response | `{"status":"ok","saved_count":5,...}` |
| X-Request-Id | `361a39d3-dc28-439e-94e5-32f78c2e857a` |
| 401/403 | **no** (default-off dev) |
| 5xx | **no** |
| Retries | **0** — first PUT succeeded |

## Post-write GET Result

| Check | Result |
|-------|--------|
| HTTP | **200** |
| Field count | **6** — none disappeared |
| tenant_id / entity_id | Correct |
| Updated fields match PUT | **yes** |

| field_code | value (after) | changed |
|------------|---------------|---------|
| temperature_mode | CHILLED | yes |
| loading_contact_phone | +7 900 111-22-33 | yes |
| planned_pickup_date | 2026-09-15 | yes |
| declared_value | { amount: 140000, currency: RUB } | yes |
| handling_flags | ["FRAGILE"] | unchanged value, updated_at refreshed |
| driver_comment | preserved (Cyrillic text) | no (not in PUT) |

## Audit Result

Query: `audit-events?entity_type=SHIPMENT&entity_id=14d405e2-0152-4030-b356-eec464a3cc66&limit=10`

| Check | Result |
|-------|--------|
| Custom values update event | **yes** |
| action | `CUSTOM_FIELD_VALUES_UPDATED` |
| entity_id | `14d405e2-0152-4030-b356-eec464a3cc66` |
| request_id | `361a39d3-dc28-439e-94e5-32f78c2e857a` (matches PUT) |
| changed_fields | 5 fields — matches PUT |
| old_values / new_values | Present and consistent with GET |
| Migration execute event | **no** |
| Import execute event | **no** |
| Template publish event | **no** |

## Active Template After Write

| Field | Before | After |
|-------|--------|-------|
| code | shipment_default | shipment_default |
| status | PUBLISHED | PUBLISHED |
| version | 1 | 1 |
| is_active | true | true |

**Unchanged** — as expected.

## Frontend Spot-check

**Method:** Option B (API-aligned static evidence) — browser DevTools session not run in agent.

| Check | Evidence |
|-------|----------|
| Route exists | `/shipments/14d405e2-0152-4030-b356-eec464a3cc66` |
| Custom values page | `/low-code/custom-field-values` — SHIPMENT + entity UUID |
| GET after write | Values visible via API (same as UI would load) |
| Save in UI | **Not performed** — write done via controlled API PUT |
| Browser console | **Pending operator manual check** |

Operator manual follow-up: open shipment detail page and confirm updated values render without console errors.

## Regression Results

| Check | Result | Notes |
|-------|--------|-------|
| `make health-check` (post-write) | **PASS** | All services OK |
| `make integration-smoke-test` | **PASS** | `TEST-20260624174949` |
| `npm run build` | **PASS** | |
| Backend code changed | **no** | |

## Stop Conditions Review

| Condition | Triggered |
|-----------|-----------|
| Wrong tenant write | **no** |
| Wrong entity write | **no** |
| Audit missing after write | **no** |
| Active template changed | **no** |
| low-code-service 5xx | **no** |
| Custom values disappeared | **no** |
| UI crash | **not tested** (API pass) |
| Integration smoke fail | **no** |
| Production write | **no** |

## Issues Found

| ID | Severity | Issue | Action |
|----|----------|-------|--------|
| I-1 | P2 | Browser UI spot-check not automated | Operator manual verify |
| I-2 | P3 | Demo entity left with test values | Optional rollback PUT per checklist |

**No P0/P1 blockers.**

## Blockers

**None.**

## Decision

| Field | Value |
|-------|-------|
| **Decision** | **GO_WITH_CONDITIONS** |
| Controlled PUT | **SUCCESS** |
| SHIPMENT user rollout | **Not approved** |
| Rollback | Optional — use restore payload if reverting demo state |

## Recommended Next Steps

1. Operator browser spot-check on shipment detail page
2. Optional rollback PUT to restore baseline demo values
3. Proceed to **Low-code Pilot Week-2 SHIPMENT Operator Flow Review Pack v0.1**
4. Repeat controlled write on **staging** pilot tenant before user-facing SHIPMENT scope

## Verification Commands

```powershell
cd D:\Projects\freight-platform
make health-check
make seed-lowcode-demo

$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"
$E = "14d405e2-0152-4030-b356-eec464a3cc66"

# GET after execute (expect updated values)
curl.exe -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/low-code/custom-field-values?entity_type=SHIPMENT&entity_id=$E&template_code=shipment_default"

# Audit
curl.exe -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/low-code/audit-events?entity_type=SHIPMENT&entity_id=$E&limit=10"

# Rollback (optional)
curl.exe -X PUT -H "Content-Type: application/json" -H "X-Tenant-ID: $T" `
  --data-binary "@scripts/dev/payloads/lowcode_shipment_write_validation_restore_placeholder.json" `
  "http://localhost:8080/api/v1/low-code/custom-field-values"
```

## Next Action

**Low-code Pilot Week-2 SHIPMENT Operator Flow Review Pack v0.1**
