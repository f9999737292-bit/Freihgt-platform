# RBAC Role Navigation Staging Deployment Retry Checklist v0.1

## Pre-deployment

| Check | Result |
|---|---|
| user approved retry deployment | yes |
| main synced with origin/main | yes |
| source diff empty before deploy | yes |
| staging root separated | yes |
| resolved roots distinct | yes |
| production baseline healthy | yes |
| staging baseline healthy | yes |
| static artifact generated | yes |
| artifact contains index.html | yes |

## Deployment

| Check | Result |
|---|---|
| artifact uploaded to /tmp | yes |
| staging root backup created | yes |
| deployed only to staging root | yes |
| production root untouched | yes |
| Nginx unchanged | yes |
| backend unchanged | yes |
| DB unchanged | yes |

## Post-deployment

| Check | Result |
|---|---|
| staging / returns 200 | yes |
| staging /login returns 200 | yes |
| staging /health returns 200 | yes |
| production / returns 200 | yes |
| production /login returns 200 | yes |
| production /health returns 200 | yes |
| browser smoke completed | partial |
| production deploy not executed | yes |

## Decision

```text
RBAC_ROLE_NAVIGATION_STAGING_DEPLOYMENT_RETRY_CHECKLIST_CREATED
```
