# Low-code Pilot Week-1 Review Template v0.1

Fill at end of Week 1 (Day 5). Save as `docs/pilot-reports/week-1-review-YYYY-MM-DD.md`.

Reference: `LOW_CODE_PILOT_WEEK1_FEEDBACK_FIX_PLAN_V0.1.md`, `LOW_CODE_PILOT_DAILY_REPORT_TEMPLATE_V0.1.md`.

---

## Week

Pilot Week **1** — dates: `YYYY-MM-DD` to `YYYY-MM-DD`

## Overall Status

Select one:

- [ ] **GO** — pilot healthy; ready for Week 2 planning
- [ ] **GO_WITH_CONDITIONS** — minor issues; continue with watch items
- [ ] **STOPPED** — P0 occurred; pilot paused (document stop date)

## Usage Summary

| Metric | Week 1 total |
|--------|--------------|
| Pilot days operational | / 5 |
| Active pilot users (shipper) | |
| Transport orders with custom field edits | |
| Custom field PUT operations (approx.) | |
| Admin template operations | |
| Import preview / execute (execute → DRAFT) | |
| Migration preview / execute | |
| Batch migration preview / execute | |
| Operator feedback forms filed | |
| Daily reports filed | / 5 |

## Health Summary

| Check | Mon | Tue | Wed | Thu | Fri | Notes |
|-------|-----|-----|-----|-----|-----|-------|
| Morning health-check | | | | | | |
| Evening health-check | | | | | | |
| low-code-service status | | | | | | |
| Integration smoke (if run) | | | | | | Run ID: |

**Health incidents this week:** _______________

## Audit Summary

| Category | Approx. event count | Anomalies |
|----------|---------------------|-----------|
| Custom values updates | | |
| Template admin | | |
| Import/export | | |
| Migration | | |
| Batch migration | | |

**Audit gaps found?** yes / no — details: _______________

## Issues by Severity

| Severity | Open at week start | New this week | Resolved | Open at week end |
|----------|-------------------|---------------|----------|------------------|
| P0 | | | | |
| P1 | | | | |
| P2 | | | | |
| P3 | | | | |

## P0 Incidents

| ID | Date | Summary | Resolution | Pilot stopped? |
|----|------|---------|------------|----------------|
| | | | | yes / no |

If none: **None this week.**

## P1 Fixes

| ID | Summary | Fix applied | Commit / pack | Verified |
|----|---------|-------------|---------------|----------|
| | | | | |

If none: **None required.**

## P2 Backlog

| ID | Summary | Target |
|----|---------|--------|
| | | Week 2+ / polish sprint |

## User Feedback

Summary of runtime user (shipper) feedback:

_______________________________________________

_______________________________________________

Top 3 themes:

1. _______________
2. _______________
3. _______________

## Operator Feedback

Summary of operator/admin feedback:

_______________________________________________

Runbook/checklist gaps:

_______________________________________________

## Security Review

- [ ] No non-admin admin access observed
- [ ] Tenant isolation confirmed
- [ ] Auth-on still enabled on staging (not committed to repo)
- [ ] No manual DB edits
- [ ] No raw sensitive data in logs

**Security issues:** none / describe: _______________

## Tenant Isolation Review

- [ ] All API calls scoped to pilot tenant
- [ ] No cross-tenant data in UI or audit
- [ ] Custom values writes on correct entity IDs

**Issues:** none / describe: _______________

## Recommended Scope Change

Select one for Week 2:

- [ ] **Continue TO only** (recommended default)
- [ ] **Add SHIPMENT** entity panels (Phase 2)
- [ ] **Add BILLING_REGISTER** entity panels
- [ ] **Pause pilot** for fix pack
- [ ] **Other:** _______________

Rationale:

_______________________________________________

## Decision for Week 2

| Field | Value |
|-------|-------|
| Decision | GO / GO_WITH_CONDITIONS / STOPPED / PAUSE |
| Scope | TO only / + SHIPMENT / + BILLING_REGISTER |
| Next pack | Week-1 Review Pack v0.1 / Runtime Pilot Fix Pack v0.1 |

Notes:

_______________________________________________

## Owner Actions

| Owner | Action | Due |
|-------|--------|-----|
| Pilot lead | | |
| Operator | | |
| DevOps | | |
| Frontend (if P1 fixes) | | |
| Backend (if approved) | | |
| Product | | |

---

**Review completed by:** _______________  
**Review date:** _______________  
**Approved by:** _______________
