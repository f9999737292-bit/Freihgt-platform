# Low-code Pilot Daily Report Template v0.1

Copy this template for each pilot day. Save as `docs/pilot-reports/YYYY-MM-DD-daily-report.md` or paste into your tracker.

Reference: `LOW_CODE_PILOT_DAY1_MONITORING_V0.1.md`, `LOW_CODE_PILOT_OPERATOR_CHECKLIST_V0.1.md`.

---

## Date

`YYYY-MM-DD`

## Pilot Day

Day **N** of pilot (Day 1 = first day with pilot users)

## Overall Status

Select one:

- [ ] **GO** — no issues; pilot operating normally
- [ ] **GO_WITH_CONDITIONS** — minor issues logged; pilot continues with watch items
- [ ] **STOPPED** — P0 stop condition triggered; pilot paused

## Health Summary

| Check | Result | Notes |
|-------|--------|-------|
| Morning `make health-check` | PASS / FAIL | |
| Evening `make health-check` | PASS / FAIL | |
| low-code-service | OK / DEGRADED / DOWN | |
| Other services | OK / issues | |

## Usage Summary

| Metric | Count / notes |
|--------|---------------|
| Active pilot users (shipper) | |
| Transport orders with custom field edits | |
| Custom field PUT operations (approx.) | |
| Admin template operations | none / list / export / import preview / other |
| Migration preview runs | |
| Migration execute runs | should be **0** on Day 1 unless approved |
| Batch operations | should be **0** unless approved |

## Audit Summary

| Category | Event count (approx.) | Anomalies |
|----------|----------------------|-----------|
| Custom values updates | | |
| Template admin (export/import/publish) | | |
| Migration (preview/execute) | | |
| Batch migration | | |
| Other | | |

Baseline audit count at day start: ______  
Audit count at day end: ______

## Issues

| ID | Severity | Summary | Status |
|----|----------|---------|--------|
| | P0/P1/P2/P3 | | open / resolved |

## Incidents

| time | area | severity | symptom | decision | owner | status |
|------|------|----------|---------|----------|-------|--------|
| | | | | | | |

(Full columns: see Incident Log Template in `LOW_CODE_PILOT_DAY1_MONITORING_V0.1.md`)

## Stop Conditions Review

Confirm each — **none triggered** unless STOPPED status above:

- [ ] No non-admin admin access
- [ ] No tenant isolation issue
- [ ] No wrong entity/tenant writes
- [ ] Audit present for today's writes
- [ ] No repeated low-code 5xx
- [ ] Health-check passed (morning + evening)
- [ ] Active template still `transport_order_default` PUBLISHED
- [ ] No unexpected import/export/migration execute

**Stop condition triggered?** yes / no  
If yes, describe: _______________________________________________

## Changes Made Today

| Change | Approved by | Notes |
|--------|-------------|-------|
| Template publish | | should be empty Day 1 |
| Import execute | | |
| Migration execute | | |
| Config/env change | | no auth-on commit |
| Other | | |

## Deferred Items

| Item | Severity | Target day / pack |
|------|----------|-------------------|
| | P2/P3 | Week-1 Feedback & Fix Plan |

## Decision for Tomorrow

Select one:

- [ ] **Continue pilot** — GO / GO_WITH_CONDITIONS
- [ ] **Pause pilot** — investigate P0/P1
- [ ] **Expand scope** — not recommended before Week-1 review
- [ ] **Escalate to fix pack** — Runtime Pilot Fix Pack v0.1

Notes: _______________________________________________

## Owner Actions

| Owner | Action | Due |
|-------|--------|-----|
| Operator | | |
| Pilot lead | | |
| DevOps / staging | | |
| Security | | |
| Product | | |

---

**Report filed by:** _______________  
**Report reviewed by:** _______________  
**Time filed:** _______________
