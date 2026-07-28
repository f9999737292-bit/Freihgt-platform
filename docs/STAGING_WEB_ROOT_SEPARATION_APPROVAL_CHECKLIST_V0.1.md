# Staging Web Root Separation Approval Checklist v0.1

## Before Approval

| Check | Required |
|---|---|
| blocked RBAC staging deployment recorded | yes |
| shared root confirmed | yes |
| target staging root defined | yes |
| production root remains unchanged | yes |
| rollback path documented | yes |
| no production deploy approved | yes |

## Future Approved Action Scope

Allowed only after explicit approval:

```text
Create /var/www/staging-bintrans-web-admin if missing.
Copy current static web content into staging root.
Update staging Nginx vhost root only (staging-bintrans.conf).
Run nginx -t.
Reload Nginx only if nginx -t passes.
Verify staging and production endpoints.
```

## Future Forbidden Action Scope

```text
Production web root update.
Production deploy.
Backend deploy.
Database migrations.
API contract changes.
DNS changes.
Certbot actions.
Role app deployment.
Reading/copying secrets or private keys.
```

## Future Rollback Requirement

```text
Rollback must restore staging Nginx vhost root to the previous value:
/var/www/bintrans-web-admin

Rollback must not alter production root content.
```

## Decision

```text
STAGING_WEB_ROOT_SEPARATION_APPROVAL_CHECKLIST_CREATED
```
