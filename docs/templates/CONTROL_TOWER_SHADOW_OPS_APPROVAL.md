# Control Tower Shadow Observation — Ops Approval

Review before staging deployment and before Day 7 final gate.

**Result:** APPROVED / APPROVED WITH CONDITIONS / REJECTED

---

## Checklist

| # | Item | Pass | Notes |
|---|------|------|-------|
| 1 | Deployment procedure reviewed (`docs/CONTROL_TOWER_SELECTEL_STAGING_DEPLOYMENT_HANDOFF.md`) | | |
| 2 | Migration procedure reviewed (`make migrate-up` → version 18) | | |
| 3 | Consumer pause/resume procedure documented | | |
| 4 | Per-partition offset capture procedure documented | | |
| 5 | Activation confirmation handling reviewed | | |
| 6 | Rollback procedure reviewed | | |
| 7 | Rollback-window closure understood | | |
| 8 | Cleanup authorization process defined | | |
| 9 | Incident response path defined | | |
| 10 | Kill switch identified (disable primary, pause consumer) | | |
| 11 | Public source verification procedure defined | | |
| 12 | Primary disabled verification in daily gate | | |

---

## Operator references

| Document | Path |
|----------|------|
| Deployment handoff | `docs/CONTROL_TOWER_SELECTEL_STAGING_DEPLOYMENT_HANDOFF.md` |
| Operator commands | `docs/CONTROL_TOWER_STAGING_OPERATOR_COMMANDS.md` |
| Daily report | `docs/templates/CONTROL_TOWER_SHADOW_DAILY_REPORT_TEMPLATE.md` |
| Release manifest | `docs/releases/CONTROL_TOWER_SHADOW_OBSERVATION_V0.6_RELEASE_MANIFEST.md` |

---

## Stuck job rule

If jobs are in `IMPORTING`, `ACTIVATING`, or `ROLLING_BACK`:

> Do not automatically change a stuck job state. Stop deployment and perform manual investigation.

---

## Reviewer

| Field | Value |
|-------|-------|
| Reviewer | |
| Date | |
| Result | APPROVED / APPROVED WITH CONDITIONS / REJECTED |
| Conditions | |

---

## Current execution status

**STAGING_EXECUTION_PENDING_OPS_ACCESS**

Observation window: **NOT STARTED**

Primary readiness: **BLOCKED**
