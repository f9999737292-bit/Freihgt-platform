# Low-code Pilot Week-3 Low-code Rollback Procedure v0.1

## Purpose

Step-by-step **documented** rollback procedure for low-code incidents. **Do not execute** as part of this docs pack unless authorized by rollback owner during a real incident.

Reference: `LOW_CODE_PILOT_WEEK3_PRODUCTION_ROLLBACK_PLAN_V0.1.md`

## Preconditions

- [ ] Rollback trigger confirmed (see plan triggers)
- [ ] Incident severity P0/P1/P2 documented
- [ ] Rollback owner **assigned and available** (or emergency delegate)
- [ ] Impacted tenant / entity type identified
- [ ] Read-only verification tools available (curl, health-check)
- [ ] **No** migration execute / publish / import / batch execute during incident

## Decision Gate

**STOP** unless rollback owner (Tech Lead / Ops) approves:

| Question | Required answer |
|----------|-----------------|
| Is low-code the root cause or primary mitigation path? | yes |
| Is scope limited to low-code templates/config (not core financial/legal)? | yes |
| Is manual DB edit avoided unless DBA emergency? | yes |

If **no** → escalate to Runtime Pilot Fix Pack; do not proceed with low-code rollback steps.

## Step 1 — Stop New Risky Changes

1. Pause template **publish**, **import execute**, **migration execute**, **batch execute**
2. Notify operators: no new low-code admin changes
3. If admin auth incident: consider `LOW_CODE_ADMIN_AUTH_ENABLED=false` + restart low-code-service (Ops)
4. Document time and owner in incident record

## Step 2 — Identify Impacted Tenant / Entity Type / Template

1. Record `X-Tenant-ID`
2. Record `entity_type` (TRANSPORT_ORDER / SHIPMENT / BILLING_REGISTER / …)
3. Record `template_code` and version if known
4. Query audit GET (read-only): filter by entity, time window, `batch_id` if migration-related
5. Identify **last known safe** published template (export or admin list)

## Step 3 — Switch To Known Safe Template / Disable Risky Config

**Templates**

1. **Do not publish** bad DRAFT
2. Ensure last good **PUBLISHED** template remains active for runtime
3. Use **clone-to-draft** from last good published if edit needed later
4. Bad import DRAFT: leave unpublished; archive when admin path available

**Custom fields / config**

1. Revert template field definitions via new DRAFT from last good published (future controlled change — not during P0 without owner)
2. Policy pause: instruct operators to ignore affected custom fields until fix

**Auth / admin**

1. If admin lockout: `LOW_CODE_ADMIN_AUTH_ENABLED=false`; restart service
2. Verify admin accessible per environment policy

## Step 4 — Verify Runtime Read-Only Behavior

Read-only GET only:

```text
GET /api/v1/low-code/form-templates/active?entity_type={TYPE}&template_code={CODE}
GET /api/v1/low-code/custom-field-values?entity_type={TYPE}&entity_id={ID}&template_code={CODE}
```

Expected: **200** for valid tenant/entity; no new writes.

## Step 5 — Verify Admin Access Control

Per auth-on policy:

| Check | Expected |
|-------|----------|
| Admin + PLATFORM_ADMIN | 200 on admin list |
| Non-admin | 403 on admin routes |
| No user (auth-on) | 401 |

Reference: `LOW_CODE_PILOT_WEEK3_AUTH_ON_STAGING_RUNBOOK_V0.1.md`

## Step 6 — Verify Audit Events

1. `GET /api/v1/low-code/audit-events?limit=20` (+ filters)
2. Confirm rollback-related actions appear or document gap
3. **Do not delete** audit records

## Step 7 — Communicate Status

1. PM notifies operators (scope, workaround, ETA)
2. Update incident ticket
3. Log entry in operator feedback log if cross-entity governance event

## Step 8 — Decide Resume / Continue Rollback

| Outcome | Action |
|---------|--------|
| Runtime stable | Mark checklist **PASS**; schedule fix pack |
| Still broken | Continue rollback (DBA backup restore — emergency only) |
| P0 persists | Runtime Pilot Fix Pack v0.1 — **STOP** writes |

Owner signs resume or continue decision.

## Evidence Required

| Evidence | Storage |
|----------|---------|
| Trigger + severity | Ticket / feedback log |
| Tenant, entity, template IDs | Checklist (non-secret) |
| HTTP status from verification GETs | Checklist |
| Owner approval timestamp | Owner note / ticket |
| Communication sent | Ticket reference |

**No passwords, JWT, or tokens in committed docs.**

## Forbidden Actions

- Direct DB edits unless **emergency approval** exists
- Deletion of audit records
- Production data mutation without approval
- Template publish during active incident (unless owner explicit exception)
- Migration execute during rollback
- Import execute during rollback
- Batch migration execute during rollback
- Committing secrets to git or docs
- Claiming production-ready as result of rollback

Checklist: `LOW_CODE_PILOT_WEEK3_ROLLBACK_CHECKLIST_V0.1.md`
