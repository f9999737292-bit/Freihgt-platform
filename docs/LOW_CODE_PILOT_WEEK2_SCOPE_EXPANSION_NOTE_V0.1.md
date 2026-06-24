# Low-code Pilot Week-2 Scope Expansion Note v0.1

## Summary

Scope guidance after **SHIPMENT read-only validation** (`LOW_CODE_PILOT_WEEK2_SHIPMENT_READONLY_VALIDATION_V0.1.md`).

**Recommendation:** Controlled SHIPMENT **read-only / internal validation** may continue. SHIPMENT **write/save** and user-facing rollout remain **restricted**.

**Decision:** **GO_WITH_CONDITIONS**

## Recommendation

| Area | Week 2 status |
|------|---------------|
| TRANSPORT_ORDER pilot (runtime write) | **Continue** — primary pilot scope |
| SHIPMENT read-only validation | **Continue** — API + static UI evidence OK |
| SHIPMENT write/save for users | **Not yet** — requires Write Validation Design Pack |
| SHIPMENT user-facing pilot | **Not yet** — product sign-off required |
| BILLING_REGISTER | **Defer** — after SHIPMENT write validation or explicit approval |

## What Was Checked

Read-only validation completed 2026-06-24:

- Active template: `shipment_default` PUBLISHED (`b2222222-2222-4222-8222-222222222202`)
- Custom values GET: 6 fields on DEMO-SH-PLANNED (`14d405e2-0152-4030-b356-eec464a3cc66`)
- Admin list + export: `schema_version: lowcode.template.export.v1`, entity SHIPMENT, no custom values in export
- Shipment detail page: `LowCodeCustomFieldsPanel` + `validation_context` wired
- No PUT / execute / publish performed

Full report: `LOW_CODE_PILOT_WEEK2_SHIPMENT_READONLY_VALIDATION_V0.1.md`.

## What Is Allowed Next

- Repeat read-only GET checks on staging pilot tenant
- Operator browser spot-check (no Save):
  - `/shipments/14d405e2-0152-4030-b356-eec464a3cc66`
  - `/low-code/custom-field-values` (SHIPMENT filter)
  - `/low-code/admin/form-templates` (preview + export button review)
- Design SHIPMENT write validation scenarios (next pack)
- Continue TRANSPORT_ORDER pilot user writes

## What Remains Restricted

- **No SHIPMENT production writes** (PUT custom field values)
- **No batch migration execute**
- **No import execute** (except approved DRAFT on admin review)
- **No template publish** without admin review
- **No broad rollout** beyond one pilot tenant
- **No BILLING_REGISTER** user pilot
- **No mobile driver app**
- **No ЭТрН/ЭПД** integration in this pilot phase

## Risks

| Risk | Mitigation |
|------|------------|
| Assuming read-only = write-ready | Separate write validation pack before user writes |
| SHIPMENT visibility rules (cold chain) | Test write + preview rules in write pack |
| Rich fields (MONEY, DATE, MULTI_SELECT) | Include in write validation test matrix |
| Staging-only evidence | Repeat curls + UI on staging tenant |
| Scope creep to SHIPMENT users | Product sign-off gate |

## Required Conditions Before SHIPMENT Write

Before enabling SHIPMENT custom field PUT for any user:

1. **Write Validation Design Pack** completed with test matrix
2. **Controlled write test** on staging (single entity, audit verified)
3. **Audit event** confirmed for SHIPMENT PUT
4. **No P0** open; P1 write blockers resolved
5. **Operator + product sign-off**
6. **Rollback plan** documented (keep TO pilot stable)

## Next Action

**Low-code Pilot Week-2 SHIPMENT Write Validation Design Pack v0.1**

If P0 discovered during staging read-only repeat:

**Low-code Runtime Pilot Fix Pack v0.1**
