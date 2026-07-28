# RBAC Role Navigation Staging Deployment Checklist v0.1

## Pre-deployment

| Check | Result |
|---|---|
| user approved staging deploy | yes |
| production deploy not approved | yes |
| main synced with origin/main | yes |
| source diff empty before deploy | yes |
| build passed | yes |
| production/staging baseline checked | yes |
| staging root distinct from production root | **no — blocked** |

## Deployment

| Check | Result |
|---|---|
| artifact created outside repo | no — blocked before upload |
| artifact uploaded to /tmp | no |
| staging backup created | no |
| staging web root updated | **blocked** |
| production web root untouched | yes |

## Post-deployment

| Check | Result |
|---|---|
| staging / returns 200 | not deployed |
| staging /login returns 200 | not deployed |
| staging /health returns 200 | not deployed |
| production / returns 200 | baseline 200 |
| production /login returns 200 | baseline 200 |
| production /health returns 200 | baseline 200 |
| no backend deploy | yes |
| no migrations | yes |
| no secrets captured | yes |

## Decision

```text
RBAC_ROLE_NAVIGATION_STAGING_DEPLOYMENT_CHECKLIST_CREATED
```
