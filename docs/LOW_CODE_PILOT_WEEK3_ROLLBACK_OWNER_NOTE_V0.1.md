# Low-code Pilot Week-3 Rollback Owner Note v0.1

## Summary

Documents **required rollback owner** for PR-GAP-003 and approval rules. Plan docs created; **owner assignment pending**.

**Decision:** **ROLLBACK_OWNER_REQUIRED**

**PR-GAP-003:** **ROLLBACK_PLAN_CREATED_PENDING_OWNER_APPROVAL**

## Required Owner

| Field | Value |
|-------|-------|
| Role | **Tech Lead / Ops** |
| Scope | Authorize and oversee low-code rollback procedure |
| Gap | PR-GAP-003 |

## Current Owner Status

**TBD**

| Field | Value |
|-------|-------|
| Named owner | **not assigned** |
| Delegate | **not assigned** |
| Approval date | — |

PM action: assign owner before production rollback plan can be **approved** (gap closure).

## Owner Responsibilities

1. Approve rollback decision gate before procedure steps
2. Confirm scope stays within low-code templates/config (not core financial/legal auto-rollback)
3. Sign off verification checklist (runtime, admin auth, audit)
4. Approve resume vs continue rollback
5. Escalate P0/P1 to Security and Runtime Pilot Fix Pack
6. Participate in Rollback Owner Approval Pack v0.1

## Approval Rules

| Rule | Detail |
|------|--------|
| Plan approval | Owner reviews plan + procedure + checklist |
| Execution approval | Separate per-incident decision gate |
| DBA involvement | Owner + DBA lead for any DB restore |
| Production-ready | Owner approval **does not** imply production-ready |
| Docs | Owner name may be recorded; no credentials in repo |

## Escalation Rules

| Condition | Escalate to |
|-----------|-------------|
| P0 security (auth bypass, tenant leak) | Security + PM |
| Owner unavailable > 4h during P0 | PM → program sponsor |
| DBA restore needed | DBA on-call + Security |
| Operator impact | PM **Феликс Асаев** |

## Missing Decisions

| # | Decision | Status |
|---|----------|--------|
| 1 | Named Tech Lead / Ops rollback owner | **TBD** |
| 2 | Formal approval of rollback plan v0.1 | **pending owner** |
| 3 | Rollback drill schedule (staging) | **optional / TBD** |

## Next Step

**Low-code Pilot Week-3 Rollback Owner Approval Pack v0.1**

Trigger: **Rollback owner assigned / approved**

Reference: `LOW_CODE_PILOT_WEEK3_PRODUCTION_ROLLBACK_PLAN_V0.1.md`
