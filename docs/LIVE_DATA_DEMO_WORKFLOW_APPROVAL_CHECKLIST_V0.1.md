# Live Data Demo Workflow Approval Checklist v0.1

## Summary

Approval boundary for controlled live-data demo execution.

Base commit: `86368ca`.

## Approval Required Before Execution

| Check | Required |
|---|---|
| target environment selected | yes |
| demo tenant approved | yes |
| demo users approved | yes |
| demo roles approved | yes |
| seed dataset approved | yes |
| credentials handling approved | yes |
| write permissions policy approved | yes |
| API smoke scope approved | yes |
| rollback/cleanup plan approved | yes |
| external demo disclaimer approved | yes |

## Environment Decision

Choose one in approval pack:

| Option | Meaning |
|---|---|
| staging-first | recommended |
| production-read-only-auth | only if demo users/data already exist |
| production-demo-seed | requires explicit production write approval |
| static-only | no live-data demo yet |

## Explicitly Not Approved In This Plan

```text
Credentials creation.
Seed data creation.
Production writes.
Staging writes.
Source changes.
Backend/API changes.
Nginx changes.
Deploys.
Migrations/database writes.
```

## Next Pack

```text
LIVE_DATA_DEMO_WORKFLOW_APPROVAL_PACK v0.1
```

## Decision

```text
LIVE_DATA_DEMO_WORKFLOW_APPROVAL_CHECKLIST_CREATED
```
