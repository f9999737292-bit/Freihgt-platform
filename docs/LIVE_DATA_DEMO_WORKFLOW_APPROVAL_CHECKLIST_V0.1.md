# Live Data Demo Workflow Approval Checklist v0.1

## Summary

Approval boundary for controlled live-data demo execution.

Base commit: `43ab15f`.

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
| staging-first | recommended — **selected** |
| production-read-only-auth | only if demo users/data already exist |
| production-demo-seed | requires explicit production write approval |
| static-only | no live-data demo yet |

## Approval Boundary Result

```text
LIVE_DATA_DEMO_WORKFLOW_APPROVAL_BOUNDARY_COMPLETE
LIVE_DATA_DEMO_ENVIRONMENT_STAGING_FIRST_APPROVED
LIVE_DATA_DEMO_V0_1_ROLE_SCOPE_APPROVED
LIVE_DATA_DEMO_CREDENTIALS_NOT_APPROVED_YET
LIVE_DATA_DEMO_SEED_DATA_NOT_APPROVED_YET
LIVE_DATA_DEMO_EXECUTION_NOT_APPROVED_YET
PRODUCTION_LIVE_DATA_DEMO_NOT_APPROVED
```

## Approved For Future Planning

| Item                 | Result                                                                                                              |
| -------------------- | ------------------------------------------------------------------------------------------------------------------- |
| environment strategy | staging-first                                                                                                       |
| v0.1 roles           | PLATFORM_ADMIN, SHIPPER_ADMIN, CARRIER_ADMIN, FINANCE_MANAGER                                                       |
| workflow shape       | login → dashboard → companies → RFx → transport orders → shipments → documents → billing → role comparison → logout |
| production use       | static walkthrough only                                                                                             |

## Still Requires Separate Approval

| Item                      | Required |
| ------------------------- | -------- |
| demo credentials          | yes      |
| seed data                 | yes      |
| staging writes            | yes      |
| staging execution         | yes      |
| production live-data demo | yes      |
| cleanup/rollback          | yes      |

## Explicitly Not Approved In This Pack

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
Login execution.
Fake sessions.
```

## Next Pack

```text
DEMO_CREDENTIALS_AND_SEED_DATA_APPROVAL_PACK v0.1
```

## Decision

```text
LIVE_DATA_DEMO_WORKFLOW_APPROVAL_CHECKLIST_COMPLETE
```
