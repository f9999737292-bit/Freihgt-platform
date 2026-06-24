# Low-code Pilot Week-2 BILLING_REGISTER Limited Write Approval Note v0.1

## Approval Summary

**Approved:** Limited BILLING_REGISTER custom-field **write** for **controlled pilot/test scope only**.

**Decision:** **ENABLE_LIMITED_BILLING_REGISTER_WRITE_WITH_CONDITIONS**

**Not approved:** Broad production BILLING_REGISTER rollout to all users/entities.

**Review date:** 2026-06-24

## What Is Approved

- BILLING_REGISTER custom field **read and write** on **approved pilot entities**
- Template: `billing_register_default` (PUBLISHED)
- Three allowed custom fields (see Field Scope)
- Audit review after every write
- Financial safety check after every write
- Internal QA and approved operators on staging/dev demo

## Who Can Operate

| Role | BILLING_REGISTER limited write |
|------|--------------------------------|
| Platform admin | **Yes** — on approved entities |
| Approved pilot operator (finance) | **Yes** — after pilot lead assigns entity |
| General finance users | **No** — until separate expansion approval |
| All other roles | **No** |

## Entity Scope

**Initially enabled (dev demo):**

- **DEMO-BR-001** — `cf7dbc77-395f-42a2-9717-476e4cd93796`
- Tenant: `74519f22-ff9b-4a8b-8fff-a958c689682f`

**Any additional entity** requires pilot lead written approval before first write.

## Field Scope

Operators may edit **only**:

- `cost_allocation_code` (TEXT)
- `approval_group` (SELECT: LOGISTICS_FINANCE, FINANCE, OPS, MANAGEMENT)
- `payment_priority` (SELECT: LOW, NORMAL, HIGH)

## What Remains Blocked

- All other BILLING_REGISTER entities (without approval)
- All production tenants outside pilot
- Billing/payment status changes via low-code
- Invoice/act/UPD operations via custom fields
- Migration / batch / import execute
- Template publish without review
- Manual database changes
- Broad production rollout

## Financial Safety Conditions

1. Custom fields are **auxiliary metadata only**
2. Core billing register remains **source of truth** for status and totals
3. Any financial side effect after custom field save → **P0 stop**
4. Core billing register GET compared before/after every write during monitoring
5. validation_context is **advisory** — not a payment command

## Stop Conditions

Stop BILLING_REGISTER writes immediately if:

- Wrong tenant or entity
- Audit missing after save
- Values disappear or corrupt
- Billing status or totals change unexpectedly
- Template changes unexpectedly
- Repeated service errors

Full list: `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_LIMITED_WRITE_ENABLEMENT_V0.1.md`

## Review Date

**Next review:** End of Week 2 BILLING_REGISTER write monitoring pack or after first operator feedback batch.

**Approver role:** Pilot lead / product owner (sign-off pending for production staging).

## Next Expansion Candidate

| Candidate | Prerequisite |
|-----------|--------------|
| Second BILLING_REGISTER pilot entity | Monitoring clean + operator sign-off |
| Finance operator write (broader role) | Separate approval |
| BILLING_REGISTER write monitoring pack | **Next** — immediate |
| Broad BILLING_REGISTER rollout | **Not scheduled** — product gate |

**Next pack:** Low-code Pilot Week-2 BILLING_REGISTER Write Monitoring Pack v0.1
