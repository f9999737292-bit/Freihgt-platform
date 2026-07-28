# Staging Web Root Separation Execution Approval Checklist v0.1

## Approval Checks

| Check | Result |
|---|---|
| shared root blocker recorded | yes |
| separation plan committed | yes |
| user approval for approval pack present | yes |
| production deploy not approved | yes |
| staging-only Nginx root change scope documented | yes |
| rollback boundary documented | yes |

## Future Execution Checks

| Check | Required |
|---|---|
| production endpoint baseline healthy | yes |
| staging endpoint baseline healthy | yes |
| staging vhost identified | yes |
| production vhost identified and not edited | yes |
| target staging root created | yes |
| staging root populated from current baseline | yes |
| nginx -t passes before reload | yes |
| production verified after reload | yes |
| staging verified after reload | yes |

## Non-goals

```text
No production deploy.
No backend deploy.
No database migration.
No DNS change.
No Certbot.
No RBAC staging deploy in this approval pack.
```

## Decision

```text
STAGING_WEB_ROOT_SEPARATION_EXECUTION_APPROVAL_CHECKLIST_CREATED
```
