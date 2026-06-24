# Low-code Pilot Week-2 SHIPMENT Write Validation Design v0.1

## Summary

Design pack for **controlled SHIPMENT custom field write validation** before enabling runtime PUT for pilot users. Builds on read-only validation (`LOW_CODE_PILOT_WEEK2_SHIPMENT_READONLY_VALIDATION_V0.1.md` — **GO_WITH_CONDITIONS**).

**This pack is design/docs only** — no SHIPMENT PUT, migration execute, import execute, or template publish is performed here.

**Next step:** execute controlled writes using `LOW_CODE_PILOT_WEEK2_SHIPMENT_WRITE_VALIDATION_CHECKLIST_V0.1.md` on staging (or dev with explicit approval).

## Current Commit

| Field | Value |
|-------|-------|
| HEAD | `e25bcce` — `docs: add shipment read-only pilot validation` |
| Design date | 2026-06-24 |
| Branch | `main` |
| Code changed in this pack | **no** |

## Scope

**In scope**

- Write validation objectives and preconditions
- Test matrix (fields, UI, API, audit, security)
- Sample PUT payloads (documentation only)
- validation_context scenarios
- Visibility / rich-field scenarios
- Rollback and stop conditions
- Execute checklist reference

**Out of scope**

- Actual PUT execution (Execute Pack)
- SHIPMENT user-facing rollout
- API contract changes
- Migrations
- Batch migration execute
- Template publish

## Evidence Documents

| Document | Status |
|----------|--------|
| `LOW_CODE_PILOT_WEEK2_SHIPMENT_READONLY_VALIDATION_V0.1.md` | **Found** (`e25bcce`) |
| `LOW_CODE_PILOT_WEEK2_SCOPE_EXPANSION_NOTE_V0.1.md` | **Found** (`e25bcce`) |
| `LOW_CODE_PILOT_WEEK2_PLAN_V0.1.md` | **Found** |
| `LOW_CODE_CUSTOM_FIELD_VALUES_API_V0.1.md` | **Found** |
| `LOW_CODE_RICH_FIELD_EDITORS_V0.1.md` | **Found** |
| `LOW_CODE_PREVIEW_VISIBILITY_RULES_V0.1.md` | **Found** |
| `LOW_CODE_ENTITY_INTEGRATION_V0.2.md` | **Found** |
| `LOW_CODE_AUDIT_LOG_V0.1.md` | **Found** |

**Missing evidence docs:** none.

## Preconditions Before Any SHIPMENT Write

All must be true before Execute Pack:

| # | Precondition |
|---|--------------|
| 1 | Read-only validation **PASS** (`LOW_CODE_PILOT_WEEK2_SHIPMENT_READONLY_VALIDATION_V0.1.md`) |
| 2 | `transport_order_default` TO pilot **stable** — no open P0 |
| 3 | Active SHIPMENT template PUBLISHED: `shipment_default` |
| 4 | Auth-on verified on **staging** (if staging execute) |
| 5 | Operator + product sign-off for **single-entity** write test |
| 6 | Export backup of `shipment_default` template JSON |
| 7 | Rollback plan acknowledged (revert values via PUT or DBA — no manual SQL) |
| 8 | Audit baseline captured (`GET audit-events?entity_type=SHIPMENT&limit=20`) |

## Dev Reference Values

Replace with staging pilot values for production-like execute.

| Item | Value |
|------|-------|
| Tenant | `74519f22-ff9b-4a8b-8fff-a958c689682f` |
| Template ID | `b2222222-2222-4222-8222-222222222202` |
| Template code | `shipment_default` |
| Primary test entity | DEMO-SH-PLANNED — `14d405e2-0152-4030-b356-eec464a3cc66` |
| Create-first candidate | DEMO-SH-IN-PROGRESS or DEMO-SH-BILLING (verify no/low custom values first) |
| Detail URL | `http://localhost:3000/shipments/14d405e2-0152-4030-b356-eec464a3cc66` |
| Gateway API | `http://localhost:8080/api/v1` |

### SHIPMENT field catalog (`shipment_default`)

| field_code | Type | field_id (dev seed) | Notes |
|------------|------|---------------------|-------|
| temperature_mode | SELECT | `b2222222-2222-4222-8222-222222222204` | Affects visibility of `driver_comment` |
| loading_contact_phone | TEXT | `b2222222-2222-4222-8222-222222222205` | Status-based visibility |
| driver_comment | TEXT | `b2222222-2222-4222-8222-222222222206` | Visible when cold chain modes |
| planned_pickup_date | DATE | `b2222222-2222-4222-8222-222222222207` | Rich editor |
| declared_value | MONEY | `b2222222-2222-4222-8222-222222222208` | `{ amount, currency }` |
| handling_flags | MULTI_SELECT | `b2222222-2222-4222-8222-222222222209` | Array of strings |

## Write Validation Objectives

| # | Objective | Pass criteria |
|---|-----------|---------------|
| W1 | Scalar field PUT | GET reflects new value; HTTP 200 |
| W2 | Rich field PUT (DATE, MONEY, MULTI_SELECT) | Typed values persist correctly |
| W3 | validation_context advisory PUT | Save succeeds; conditional rules evaluated |
| W4 | UI save from entity detail | Panel save → GET match; no console critical error |
| W5 | UI save from custom-values page | Same entity/fields persist |
| W6 | Audit on write | Audit event for SHIPMENT entity after PUT |
| W7 | Idempotent re-save | Second identical PUT safe |
| W8 | Tenant isolation | Wrong tenant header → no cross-tenant write |
| W9 | Visibility after write | UI preview hides/shows fields per rules |
| W10 | Error clarity | Invalid type → readable error (no raw stack) |

## Test Matrix

### API PUT scenarios (Execute Pack)

| ID | Scenario | Entity | Fields | Expected |
|----|----------|--------|--------|----------|
| SH-W-01 | Update scalar TEXT | DEMO-SH-PLANNED | `loading_contact_phone` | 200; GET matches |
| SH-W-02 | Update SELECT | DEMO-SH-PLANNED | `temperature_mode` → `CHILLED` | 200; visibility may change |
| SH-W-03 | Update DATE | DEMO-SH-PLANNED | `planned_pickup_date` | 200; `YYYY-MM-DD` |
| SH-W-04 | Update MONEY | DEMO-SH-PLANNED | `declared_value` | 200; amount+currency object |
| SH-W-05 | Update MULTI_SELECT | DEMO-SH-PLANNED | `handling_flags` | 200; array persisted |
| SH-W-06 | Multi-field single PUT | DEMO-SH-PLANNED | 2–3 fields together | 200; all saved |
| SH-W-07 | validation_context present | DEMO-SH-PLANNED | any + context | 200; no server error |
| SH-W-08 | Idempotent repeat | DEMO-SH-PLANNED | same as SH-W-01 | 200; no duplicate corruption |
| SH-W-09 | Invalid MONEY shape | DEMO-SH-PLANNED | bad `declared_value` | 4xx; clear error |
| SH-W-10 | Wrong tenant header | DEMO-SH-PLANNED | any | 403/404 or empty — no cross-tenant |

### UI scenarios (Execute Pack — manual)

| ID | Scenario | Page | Expected |
|----|----------|------|----------|
| SH-UI-01 | Entity detail edit | `/shipments/{id}` | Save works; values reload |
| SH-UI-02 | Rich editors | DEMO-SH-PLANNED detail | Date/money/multi-select controls work |
| SH-UI-03 | Visibility toggle | Change `temperature_mode` | `driver_comment` show/hide |
| SH-UI-04 | Custom values page | `/low-code/custom-field-values` | SHIPMENT filter + save |
| SH-UI-05 | No double-submit | Rapid double-click Save | One request; button disabled in-flight |
| SH-UI-06 | Create-first (optional) | Entity without values | First save creates values |

### Audit scenarios

| ID | Check | Expected |
|----|-------|----------|
| SH-A-01 | After SH-W-01 | Audit event with SHIPMENT entity_id |
| SH-A-02 | Actor/request_id | Present if auth-on staging |
| SH-A-03 | Audit filter UI | `/low-code/audit?entity_type=SHIPMENT` shows event |

## Sample PUT Payload (documentation — do not run in Design Pack)

Gateway path: `PUT /api/v1/low-code/custom-field-values`

```json
{
  "entity_type": "SHIPMENT",
  "entity_id": "14d405e2-0152-4030-b356-eec464a3cc66",
  "form_template_id": "b2222222-2222-4222-8222-222222222202",
  "validation_context": {
    "entity_type": "SHIPMENT",
    "entity_id": "14d405e2-0152-4030-b356-eec464a3cc66",
    "entity_status": "PLANNED",
    "route": { "from": "Moscow", "to": "Kazan" }
  },
  "values": [
    { "field_code": "loading_contact_phone", "value_json": "+7 900 111-22-33" },
    { "field_code": "planned_pickup_date", "value_json": "2026-09-01" },
    { "field_code": "declared_value", "value_json": { "amount": 130000, "currency": "RUB" } },
    { "field_code": "handling_flags", "value_json": ["FRAGILE", "TOP_LOAD_ONLY"] }
  ]
}
```

PowerShell (Execute Pack only, with approval):

```powershell
$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"
curl.exe -X PUT `
  -H "Content-Type: application/json" `
  -H "X-Tenant-ID: $T" `
  --data-binary "@scripts/dev/payloads/shipment_write_validation_sample.json" `
  "http://localhost:8080/api/v1/low-code/custom-field-values"
```

Optional payload file may be added in Execute Pack — **not created in this Design Pack**.

## validation_context Design Notes

Shipment detail builds context via `buildShipmentValidationContext()`:

- `entity_status` from shipment.status
- `route.from` / `route.to` from resolved location names (fallback to id)
- `transport_order_id`, `carrier_id`, `driver_id`, `vehicle_id` when present

**Write validation should confirm:**

- PUT with compact context does not break save
- Missing route data does not crash UI save
- Conditional required (if any on SHIPMENT fields) respects context — advisory only per policy

Reference: `apps/web-admin/utils/lowCodeValidationContext.ts`, `pages/shipments/[id].vue`.

## Visibility Rules (write impact)

| Field | Rule (demo seed) | Write test |
|-------|------------------|------------|
| `driver_comment` | visible when `temperature_mode` in FROZEN, CHILLED | SH-W-02 + SH-UI-03 |
| `loading_contact_phone` | visible when status in transit/delivery | Use DEMO-SH-IN-PROGRESS if status matches |

After changing `temperature_mode` to `AMBIENT`, `driver_comment` may hide in preview — value may remain stored (document observed behavior).

## Security Design

| Control | Execute verification |
|---------|---------------------|
| Tenant header required | SH-W-10 |
| Auth-on staging for admin paths | Unchanged — runtime PUT tenant-scoped |
| No cross-entity write | entity_id in URL/body must match |
| Audit on every write | SH-A-01 |
| No manual DB | Policy |
| Rollback via PUT restore or export baseline | Document restore values before test |

## Rollback Plan

1. **Before write test:** export template + note current GET values for test entity
2. **If bad write:** PUT restore previous values from step 1
3. **If template issue:** do not publish; keep PUBLISHED template active
4. **If P0:** stop SHIPMENT write pilot; TO pilot may continue if isolated
5. **DB restore:** DBA backup only — no ad-hoc SQL

## Stop Conditions (Execute Pack)

Stop SHIPMENT write validation if:

- Cross-tenant write observed
- Audit missing after PUT
- GET does not reflect PUT (data loss)
- Repeated 5xx on PUT
- UI save causes wrong entity write
- Invalid write accepted (validation bypass)

Escalate to **Low-code Runtime Pilot Fix Pack v0.1**.

## Decision Gates

| Gate | Criteria | Outcome |
|------|----------|---------|
| Design complete | This doc + checklist | Proceed to Execute Pack |
| Execute API pass | SH-W-01..08 pass | Allow UI execute tests |
| Execute UI pass | SH-UI-01..05 pass | Recommend limited internal write |
| Product gate | Operator sign-off | SHIPMENT write for pilot users (future) |

**Design Pack decision:** **GO_WITH_CONDITIONS** — ready for controlled Execute Pack on staging/dev.

## Execute Checklist

Operator step-by-step: `LOW_CODE_PILOT_WEEK2_SHIPMENT_WRITE_VALIDATION_CHECKLIST_V0.1.md`

## Verification Run (Design Pack — read-only)

| Check | Result | Notes |
|-------|--------|-------|
| `make health-check` | **PASS** | All services OK |
| `make seed-lowcode-demo` | **PASS** | SHIPMENT template + demo values |
| `make integration-smoke-test` | **PASS** | `TEST-20260624172721` |
| `npm run build` | **PASS** | |
| SHIPMENT PUT executed | **No** | Design pack only |

## Next Action

**Low-code Pilot Week-2 SHIPMENT Write Validation Execute Pack v0.1**

If P0 found during execute:

**Low-code Runtime Pilot Fix Pack v0.1**
