# RBAC Role Navigation Staging Acceptance Checklist v0.1

## Pre-deployment Checks

| Check | Required |
|---|---|
| main is synced with origin/main | yes |
| build passes locally | yes |
| staging deployment approval exists | yes |
| production deployment not approved | yes |
| no backend/API/migration changes | yes |
| no secrets captured | yes |

## Staging Post-deployment Checks

| Check | Required |
|---|---|
| staging / returns 200 | yes |
| staging /login returns 200 | yes |
| staging /health returns 200 | yes |
| app is not blank | yes |
| login fields are production-safe | yes |
| admin navigation renders | yes |
| RBAC sidebar does not crash app | yes |
| production still returns 200 | yes |

## Explicit Non-goals

```text
No production deployment.
No backend deployment.
No database migration.
No role app deployment.
No pilot user onboarding.
```

## Decision

```text
RBAC_ROLE_NAVIGATION_STAGING_ACCEPTANCE_CHECKLIST_CREATED
```
