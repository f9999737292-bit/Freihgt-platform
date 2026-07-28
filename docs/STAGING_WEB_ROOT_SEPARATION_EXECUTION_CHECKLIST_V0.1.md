# Staging Web Root Separation Execution Checklist v0.1

## Pre-execution

| Check | Result |
|---|---|
| user approved execution | yes |
| main synced with origin/main | yes |
| production baseline healthy | yes |
| staging baseline healthy | yes |
| staging vhost identified | yes |
| production vhost identified | yes |
| production deploy not approved | yes |

## Execution

| Check | Result |
|---|---|
| staging config backed up | yes |
| target staging root created | yes |
| target staging root populated | yes |
| only staging vhost root changed | yes |
| production vhost not edited | yes |
| nginx -t passed before reload | yes |
| nginx reloaded | yes |

## Post-execution

| Check | Result |
|---|---|
| staging / returns 200 | yes |
| staging /login returns 200 | yes |
| staging /health returns 200 | yes |
| production / returns 200 | yes |
| production /login returns 200 | yes |
| production /health returns 200 | yes |
| RBAC deploy not executed | yes |
| production deploy not executed | yes |
| DNS unchanged | yes |
| Certbot unchanged | yes |
| secrets not captured | yes |

## Decision

```text
STAGING_WEB_ROOT_SEPARATION_EXECUTION_CHECKLIST_CREATED
```
