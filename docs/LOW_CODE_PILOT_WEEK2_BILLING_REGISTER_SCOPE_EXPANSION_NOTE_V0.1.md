# Low-code Pilot Week-2 BILLING_REGISTER Scope Expansion Note v0.1

## Summary

Scope guidance after **BILLING_REGISTER read-only validation** (`LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_READONLY_VALIDATION_V0.1.md`).

**Recommendation:** Controlled BILLING_REGISTER **read-only / internal validation** may continue. BILLING_REGISTER **write/save** and user-facing rollout remain **restricted**.

**Decision:** **GO_WITH_CONDITIONS**

## Recommendation

| Area | Week-2 status |
|------|---------------|
| TRANSPORT_ORDER pilot (runtime write) | **Continue** — primary pilot scope |
| SHIPMENT limited write pilot | **Continue with monitoring** — approved entities only |
| BILLING_REGISTER read-only validation | **Continue** — API + static UI evidence OK |
| BILLING_REGISTER write/save | **Not yet** — requires Write Validation Design Pack |
| BILLING_REGISTER user-facing pilot | **Not yet** — product sign-off required |

## What Was Checked

Read-only validation completed 2026-06-24:

- Active template: `billing_register_default` PUBLISHED (`b3333333-3333-4333-8333-333333333302`)
- Custom values GET: 3 fields on DEMO-BR-001 (`cf7dbc77-395f-42a2-9717-476e4cd93796`)
- Core billing register GET: HTTP 200 for detail page prerequisite
- Admin list + export: `schema_version: lowcode.template.export.v1`, entity BILLING_REGISTER, no custom values in export
- Billing register detail page: `LowCodeCustomFieldsPanel` + `validation_context` wired
- No PUT / execute / publish performed

Full report: `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_READONLY_VALIDATION_V0.1.md`.

## What Is Allowed Next

- Repeat read-only GET checks on staging pilot tenant
- Operator browser spot-check (no Save):
  - `/billing-registers/cf7dbc77-395f-42a2-9717-476e4cd93796`
  - `/low-code/custom-field-values` (BILLING_REGISTER filter)
  - `/low-code/admin/form-templates` (preview + export button review)
- Design BILLING_REGISTER write validation scenarios (next pack)
- Continue TRANSPORT_ORDER and monitored SHIPMENT limited write pilot

## What Remains Restricted

- **No BILLING_REGISTER production writes** (PUT custom field values)
- **No billing status changes through low-code**
- **No batch migration execute**
- **No import execute** (except approved DRAFT on admin review)
- **No template publish** without admin review
- **No broad rollout** beyond one pilot tenant
- **No mobile driver app**
- **No ЭТрН/ЭПД** integration in this pilot phase

## Risks

| Risk | Mitigation |
|------|------------|
| Assuming read-only = write-ready | Separate write validation design + execute packs before user writes |
| Financial fields (`amount`, `currency` in validation_context) | Treat as advisory; core billing register remains source of truth |
| Approval/payment priority fields | Include in write validation test matrix |
| Operator confusion on billing detail page | Manual UI spot-check before write pack |
| Cross-entity pilot load (TO + SH + BR) | Keep BILLING_REGISTER internal until write validation passes |

## Required Conditions Before BILLING_REGISTER Write

1. **Write Validation Design Pack** — scenarios, payloads, rollback plan
2. **Controlled execute pack** — one demo PUT on DEMO-BR-001 with audit verification
3. **Operator flow review** — plain-language guide + checklist section
4. **No P0 stop conditions** from read-only or execute validation
5. **Product sign-off** for limited write enablement (separate enablement note)
6. **Staging auth-on verification** if admin routes involved

## Next Action

**Low-code Pilot Week-2 BILLING_REGISTER Write Validation Design Pack v0.1**

If P0 blocker found during read-only or write validation:

**Low-code Runtime Pilot Fix Pack v0.1**

Reference docs:

- `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_READONLY_VALIDATION_V0.1.md`
- `LOW_CODE_ENTITY_INTEGRATION_V0.2.md`
- `LOW_CODE_PILOT_WEEK2_SHIPMENT_WRITE_MONITORING_V0.1.md`
