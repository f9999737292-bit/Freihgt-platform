# Low-code Pilot Week-3 Production Rollback Plan v0.1

## Summary

Production **rollback plan** for low-code module readiness gap **PR-GAP-003**. Describes **what to do** if errors appear after enabling/publishing low-code templates or custom fields.

**Decision:** **PRODUCTION_ROLLBACK_PLAN_CREATED**

**PR-GAP-003:** **ROLLBACK_PLAN_CREATED_PENDING_OWNER_APPROVAL**

**Production-ready claimed:** **no**

**Controlled pilot:** **CONTROLLED_PILOT_APPROVED** — active

**Docs-only pack** — **no rollback executed**; no code, DB, or production/staging writes.

## Purpose

Close the production readiness gap for **documented rollback steps** for low-code templates, custom fields, and runtime config — pending **Tech Lead / Ops owner approval**.

Reference: `LOW_CODE_PILOT_WEEK3_PRODUCTION_READINESS_GAP_CLOSURE_PLAN_V0.1.md`

## Scope

**In scope**

- Rollback triggers, owners, procedure reference, verification, communication, audit requirements
- Controlled pilot and future production low-code operations

**Out of scope**

- Executing rollback in this pack
- Backend/frontend/API changes
- Migration execute, publish, import, batch execute
- Manual DB edits
- Production-ready approval

## Current Status

| Field | Value |
|-------|-------|
| HEAD (baseline) | `b4189ab` — staging deployment runbook |
| Pack date | 2026-06-26 |
| PR-GAP-003 before | PENDING |
| PR-GAP-003 after | **ROLLBACK_PLAN_CREATED_PENDING_OWNER_APPROVAL** |
| Rollback owner | **TBD** |

## What Can Be Rolled Back

| Area | Rollback action (documented — not executed here) |
|------|--------------------------------------------------|
| Template activation decision | Revert to last known **PUBLISHED** template; do not publish bad DRAFT |
| Latest published template usage | Point runtime to previous published version via admin (when supported) or policy pause |
| Custom field visibility/configuration | Disable risky fields via template edit + republish from last good version |
| Imported template DRAFT | **Do not publish**; discard or archive DRAFT |
| Admin UI risky actions | Pause publish/migrate/import via policy; disable auth-on if admin lockout |
| Runtime low-code display | Feature flag / config: `LOW_CODE_ADMIN_AUTH_ENABLED=false`; gateway policy (last resort) |

Reference: `LOW_CODE_RUNTIME_PILOT_STAGING_CHECKLIST_V0.1.md` (Rollback Plan section)

## What Must Not Be Rolled Back Automatically

| Domain | Rule |
|--------|------|
| Core shipment status | Low-code rollback **must not** mutate core SH lifecycle |
| Billing register status | **Must not** auto-revert BR core status |
| Payment status | **Out of scope** for low-code rollback |
| Signed documents | **No** automatic rollback |
| Legal documents | **No** automatic rollback |
| Financial source-of-truth records | Low-code fields are **advisory** unless separately approved (PR-GAP-010) |
| Production database manually | **No** direct SQL without emergency DBA approval |

## Rollback Triggers

| # | Trigger | Typical severity |
|---|---------|------------------|
| 1 | P0 incident in low-code runtime | P0 |
| 2 | P1 incident in admin template management | P1 |
| 3 | Tenant isolation issue suspected | P0/P1 |
| 4 | Admin authorization bypass suspected | P0 |
| 5 | Incorrect published template affects operator flow | P1/P2 |
| 6 | Custom fields create operational confusion | P2 |
| 7 | Audit events missing for low-code writes | P1 |
| 8 | Performance degradation related to low-code runtime | P1/P2 |
| 9 | Security owner requests rollback | P1+ |

On P0/P1: also invoke **Low-code Runtime Pilot Fix Pack v0.1** parallel track.

## Rollback Decision Owners

| Role | Responsibility |
|------|----------------|
| **Rollback owner (Tech Lead / Ops)** | Authorize rollback steps — **TBD** |
| Security | Auth/tenant/isolation rollback decisions |
| PM / Pilot lead | Operator communication |
| DBA (emergency) | Approved DB restore only |

See `LOW_CODE_PILOT_WEEK3_ROLLBACK_OWNER_NOTE_V0.1.md`

## Rollback Procedure

Detailed steps: `LOW_CODE_PILOT_WEEK3_LOW_CODE_ROLLBACK_PROCEDURE_V0.1.md`

Checklist: `LOW_CODE_PILOT_WEEK3_ROLLBACK_CHECKLIST_V0.1.md`

**This pack does not execute the procedure.**

## Verification After Rollback

| Check | Method |
|-------|--------|
| Runtime GET | `GET /form-templates/active`, custom-field-values — read-only |
| Admin auth | Admin list 200/403/401 per auth-on policy |
| Audit | `GET /audit-events` — rollback actions logged |
| Health | `make health-check` or target env equivalent |
| Operator smoke | PM confirms TO/SH/BR demo flows if in scope |

## Communication Plan

| Audience | When | Content |
|----------|------|---------|
| Operators | Within 1h of rollback decision | What changed, what is paused, who to contact |
| PM / Pilot lead | Immediate | Incident summary, gap status |
| Security | If auth/tenant trigger | Findings + rollback scope |
| Ops | During procedure | Step completion from checklist |

No secrets in communications or committed docs.

## Audit Requirements

- All rollback decisions recorded (ticket / feedback log entry)
- Audit GET reviewed for affected `entity_type`, `entity_id`, `batch_id`
- **Do not delete** audit records
- Evidence: HTTP status, timestamps, owner sign-off — no JWT/passwords in docs

## Risks

| Risk | Mitigation |
|------|------------|
| Plan not owner-approved | Rollback Owner Approval Pack v0.1 |
| Rollback without owner | Decision gate in procedure |
| Manual DB panic edits | Forbidden without DBA emergency approval |
| Core domain corrupted by rollback script | Scope limits — low-code only |

## Open Questions

| # | Question | Owner |
|---|----------|-------|
| 1 | Named rollback owner (Tech Lead / Ops)? | PM |
| 2 | Production template archive path availability? | Backend lead |
| 3 | Gateway-level disable for low-code admin routes? | Ops |
| 4 | DBA emergency contact for pilot tenant? | Ops |

## Decision

**PRODUCTION_ROLLBACK_PLAN_CREATED**

**Not selected:** production-ready approval; rollback execution.

## Next Steps

1. **Rollback Owner Approval Pack v0.1** — assign and approve owner
2. Optional rollback drill on staging (read-only verification only) when staging available
3. Continue parallel gaps (PR-GAP-001 staging, etc.)

Related: `LOW_CODE_PILOT_WEEK3_PRODUCTION_READINESS_GAP_TRACKER_V0.1.md` (PR-GAP-003)
