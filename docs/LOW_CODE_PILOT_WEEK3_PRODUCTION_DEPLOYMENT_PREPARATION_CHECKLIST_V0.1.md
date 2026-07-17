# Production Deployment Preparation Checklist v0.1

## Purpose

Checklist for a future production deployment execution pack.

This checklist does not authorize or execute production deploy.

## Gate 0 — Required Owner Approval

| Item | Status |
| --- | --- |
| Production-ready owner approval recorded | yes |
| Deployment preparation owner approval recorded | yes |
| Deployment execution owner approval recorded | yes |
| Production deploy authorized for execution pack | yes — execution pack still required |
| Production deploy executed | no |

Required future execution wording:

```text
OWNER_APPROVES_PRODUCTION_DEPLOYMENT_EXECUTION
```

## Gate 1 — Environment Definition

| Item | Status |
| --- | --- |
| Target production server/environment defined | yes — current Selectel VM / current staging-to-production promotion |
| Production domain defined | yes — бинтранс.рф |
| Deployment window defined | yes — 2026-07-17 23:00–01:00 MSK |
| Responsible operator defined | yes — Феликс Асаев |
| Go/no-go owner defined | yes — Феликс Асаев |
| DNS plan defined | pending — execution pack |
| SSL/HTTPS plan defined | pending — execution pack |
| Firewall/security group plan defined | pending — execution pack |
| Backup/snapshot plan required | yes |
| Rollback trigger criteria required | yes |

## Gate 2 — Pre-deploy Readiness

| Item | Required |
| --- | --- |
| `git status` clean or known noise documented | yes |
| Latest approved commit identified | yes |
| Build artifact source identified | yes |
| Secrets excluded from repo/docs | yes |
| No `.env` values committed | yes |
| No cert private keys committed | yes |
| Pre-deploy health check prepared | yes |

## Gate 3 — Execution Pack Requirements

| Item | Required |
| --- | --- |
| Explicit owner execution approval | yes |
| Exact commands listed | yes |
| Stop conditions listed | yes |
| Backup step before changes | yes |
| Health checks after each step | yes |
| Rollback instructions included | yes |
| Commit/push rules for docs-only evidence | yes |

## Decision

```text
PRODUCTION_DEPLOYMENT_PREPARATION_CHECKLIST_CREATED
```

## References

```text
docs/LOW_CODE_PILOT_WEEK3_PRODUCTION_DEPLOYMENT_PREPARATION_PLAN_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_PRODUCTION_DEPLOYMENT_RUNBOOK_DRAFT_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_PRODUCTION_ROLLBACK_PLAN_V0.1.md
```
