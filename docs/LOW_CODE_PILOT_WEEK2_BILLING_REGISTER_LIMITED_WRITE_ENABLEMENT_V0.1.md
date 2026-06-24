# Low-code Pilot Week-2 BILLING_REGISTER Limited Write Enablement v0.1

## Summary

Official **limited enablement** for BILLING_REGISTER custom-field **PUT** within pilot scope. Applies to **approved pilot/test entities only** — not broad production rollout.

**Decision: ENABLE_LIMITED_BILLING_REGISTER_WRITE_WITH_CONDITIONS**

Controlled write validation succeeded (PUT HTTP 200, audit present, active template unchanged, financial safety passed). Operator flow documented. **No real operator feedback collected yet** — enablement is **internal/controlled pilot** only.

## Current Commit

| Field | Value |
|-------|-------|
| HEAD | `b85dd8c` — `docs: add billing register operator flow review` |
| Enablement date | 2026-06-24 |
| Branch | `main` |
| Code changed in this pack | **no** |

## Decision

| Field | Value |
|-------|-------|
| **Enablement decision** | **ENABLE_LIMITED_BILLING_REGISTER_WRITE_WITH_CONDITIONS** |
| Broad production BILLING_REGISTER rollout | **Not approved** |
| TRANSPORT_ORDER pilot | **Continues** (primary scope) |
| SHIPMENT limited write pilot | **Continues** (monitored) |

## Evidence Documents

| Document | Status | Notes |
|----------|--------|-------|
| `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_READONLY_VALIDATION_V0.1.md` | **Found** | Read-only GO_WITH_CONDITIONS |
| `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_CONTROLLED_WRITE_VALIDATION_V0.1.md` | **Found** | PUT 200, audit OK, financial safety OK |
| `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_OPERATOR_NOTE_V0.1.md` | **Found** | Restrictions |
| `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_OPERATOR_FLOW_REVIEW_V0.1.md` | **Found** | GO_WITH_CONDITIONS |
| `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_OPERATOR_QUICK_GUIDE_V0.1.md` | **Found** | Operator plain guide |
| `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_VALIDATION_CHECKLIST_V0.1.md` | **Found** | Execute checklist |
| `LOW_CODE_PILOT_OPERATOR_CHECKLIST_V0.1.md` | **Found** | General checklist |

**Controlled write + operator flow evidence present** — enablement allowed with conditions.

### Evidence summary

| Item | Result |
|------|--------|
| Read-only BILLING_REGISTER validation | **PASS** — GO_WITH_CONDITIONS |
| Controlled write PUT | **HTTP 200**, `saved_count: 3` |
| Post-write GET | **PASS** — 3 fields updated |
| Audit | `CUSTOM_FIELD_VALUES_UPDATED` visible |
| Active template | Unchanged — `billing_register_default` PUBLISHED v1 |
| Financial safety | Core status/totals unchanged |
| Operator flow review | **GO_WITH_CONDITIONS** |
| Real operator feedback | **None collected yet** |
| Integration smoke (enablement pack) | **PASS** (`TEST-20260624185121`) |

## Enablement Criteria

| Criterion | Met |
|-----------|-----|
| Controlled PUT succeeded | **yes** |
| Audit event visible | **yes** |
| Active template unchanged | **yes** |
| Financial safety passed | **yes** |
| No P0 stop conditions | **yes** |
| health-check / smoke / build | **yes** |
| Operator flow documented | **yes** |
| Real operator sign-off | **pending** |

## Limited Write Scope

| Dimension | Scope |
|-----------|-------|
| entity_type | **BILLING_REGISTER** only |
| template_code | **billing_register_default** |
| Tenants | **One pilot tenant** (staging/dev demo) |
| Entities | **Approved pilot/test list only** |
| Users | Platform admin + explicitly approved operators |
| Environment | Staging/pilot — **not** broad production |

## Allowed Entities

### Initial enabled entity (dev demo)

| Field | Value |
|-------|-------|
| demo name | **DEMO-BR-001** |
| entity_id | `cf7dbc77-395f-42a2-9717-476e4cd93796` |
| tenant_id | `74519f22-ff9b-4a8b-8fff-a958c689682f` |
| Detail URL (dev) | `http://localhost:3000/billing-registers/cf7dbc77-395f-42a2-9717-476e4cd93796` |

### Future entities

Additional BILLING_REGISTER entities require **explicit written approval** from pilot lead (entity ID + tenant documented in daily report).

## Allowed Fields

| field_code | Type | Valid SELECT options (dev) |
|------------|------|----------------------------|
| `cost_allocation_code` | TEXT | Free text |
| `approval_group` | SELECT | LOGISTICS_FINANCE, FINANCE, OPS, MANAGEMENT |
| `payment_priority` | SELECT | LOW, NORMAL, HIGH |

**No other field_codes** may be written under this enablement.

## Allowed Operations

- `GET .../custom-field-values` for BILLING_REGISTER pilot entities
- `GET .../billing-registers/{id}` — core entity financial baseline
- `PUT .../custom-field-values` for **allowed entities + fields only**
- Audit review after **every** write
- Active template check before/after suspicious activity
- Financial safety confirmation after **every** write
- Operator feedback form on errors

## Forbidden Operations

- Broad production BILLING_REGISTER write rollout
- Billing register core status changes via low-code
- Payment status changes via low-code
- Shipment financial status changes via low-code
- Invoice/act/UPD operations triggered by custom fields
- migration execute
- batch migration execute
- import execute
- template publish without admin review
- manual DB edits
- Writes to fields outside allowed list
- Writes to non-approved entity IDs
- Committing `LOW_CODE_ADMIN_AUTH_ENABLED=true`

## Operator Pre-write Checklist

Before each BILLING_REGISTER write:

- [ ] Correct **tenant_id**
- [ ] `entity_type` = **BILLING_REGISTER**
- [ ] Correct **entity_id** (on approved list)
- [ ] `template_code` = **billing_register_default**
- [ ] Entity is **demo/pilot approved**
- [ ] Active template GET → **200** PUBLISHED
- [ ] Current values loaded (GET)
- [ ] Core billing register GET baseline (status, totals)
- [ ] Field codes are in **allowed list**
- [ ] Valid SELECT options only
- [ ] No billing/payment/core status change intended
- [ ] No invoice/act/UPD operation involved
- [ ] No migration/import/batch in progress
- [ ] Previous Save finished — no double-click
- [ ] Audit page/API accessible

## Operator Post-write Checklist

After each BILLING_REGISTER write:

- [ ] GET values — changed fields only updated
- [ ] All 3 field_codes still present (none disappeared)
- [ ] Correct tenant / entity in response
- [ ] Audit shows `CUSTOM_FIELD_VALUES_UPDATED`
- [ ] No migration/import/publish audit events
- [ ] Active template unchanged (if concern)
- [ ] Core billing register status **unchanged**
- [ ] Core totals (`total_with_vat`, etc.) **unchanged**
- [ ] No shipment financial status side effect
- [ ] Entry in pilot daily report

## Financial Safety Guardrails

| Rule | Requirement |
|------|-------------|
| Custom fields = auxiliary metadata | Not payment/billing source of truth |
| validation_context advisory only | Does not drive financial transitions |
| Source of truth | Core billing register + billing services |
| No status transitions via low-code | Status changes use billing register UI/workflows |
| No invoice/act/UPD via custom fields | Document workflows separate |
| Financial side effect | **P0 stop** — halt all BILLING_REGISTER writes |
| Monitoring required | Stability confirmed before scope expansion |

## Monitoring Rules

### Daily

- [ ] `make health-check`
- [ ] Audit review — BILLING_REGISTER `CUSTOM_FIELD_VALUES_UPDATED`
- [ ] Recent BILLING_REGISTER writes count / entities
- [ ] low-code-service error logs
- [ ] Operator feedback forms
- [ ] Financial side-effect review (core GET spot-check)

### After every write

- [ ] GET after write
- [ ] Audit check
- [ ] Financial safety check (core billing register GET)
- [ ] Record operator / action / entity / field_codes

### Weekly

- [ ] Total BILLING_REGISTER writes vs errors
- [ ] Audit gaps (writes without events)
- [ ] Financial safety incidents count
- [ ] Expand entity list? (product decision)
- [ ] Broader readiness review candidate?

**Next monitoring pack:** `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_MONITORING_V0.1.md` (to be created)

## Stop Conditions

**P0 — stop all BILLING_REGISTER writes immediately:**

| # | Condition |
|---|-----------|
| 1 | Wrong tenant write |
| 2 | Wrong entity write |
| 3 | Audit missing after write |
| 4 | Active template unexpectedly changed |
| 5 | Core billing register status changed |
| 6 | Payment status changed |
| 7 | Shipment financial status changed |
| 8 | Invoice/act/UPD operation triggered |
| 9 | Custom values disappeared |
| 10 | Repeated low-code-service 5xx |
| 11 | integration-smoke-test fails after write |
| 12 | Operator cannot identify correct entity/template |
| 13 | Non-admin admin access (auth-on staging) |

Escalate: **Low-code Runtime Pilot Fix Pack v0.1**

## Rollback / Recovery

1. **Stop** further BILLING_REGISTER writes
2. **Inspect audit** — entity_id, changed_fields, timestamp, request_id
3. **Compare GET** custom values + core billing register before/after
4. **Restore** via PUT using restore payload from pre-write GET:
   - `scripts/dev/payloads/lowcode_billing_register_write_validation_restore_placeholder.json` (update from snapshot)
5. **Local/demo only:** `make seed-lowcode-demo` may skip if values exist — use explicit restore PUT
6. **No** manual DB edits except emergency DBA
7. **Do not** publish template or run migration as rollback
8. **Do not** change billing/payment status as rollback unless billing service supports explicit safe flow

## Known Limitations

- No real operator feedback yet
- Single demo entity enabled initially
- Browser UI sign-off pending
- Auth-on must be verified on staging separately
- SELECT invalid values return 400 — operators must use valid options
- Broad rollout requires separate product approval
- Demo entity may have test values from controlled write validation

## Verification Run (this pack)

| Check | Result |
|-------|--------|
| `make health-check` | **PASS** |
| `make seed-lowcode-demo` | **PASS** |
| `make integration-smoke-test` | **PASS** (`TEST-20260624185121`) |
| `npm run build` | **PASS** |
| PUT in this pack | **no** |

## Next Action

**Low-code Pilot Week-2 BILLING_REGISTER Write Monitoring Pack v0.1**

Reference docs:

- `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_LIMITED_WRITE_APPROVAL_NOTE_V0.1.md`
- `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_OPERATOR_QUICK_GUIDE_V0.1.md`
- `LOW_CODE_PILOT_OPERATOR_CHECKLIST_V0.1.md` — BILLING_REGISTER Limited Write Pilot section
