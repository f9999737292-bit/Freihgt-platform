# Staging Web Root Separation Approval v0.1

## Summary

Approval prepared for a future controlled server change that separates staging and production web roots.

This approval pack does not execute server changes, does not edit Nginx, does not deploy, and does not change production or staging.

## Decision

```text
STAGING_WEB_ROOT_SEPARATION_APPROVED_FOR_EXECUTION_PACK
```

## User Approval

```text
Подтверждаю STAGING_WEB_ROOT_SEPARATION_APPROVAL_PACK. Разрешаю подготовить approval на изменение только staging Nginx root. Production не трогать.
```

## Current Blocker

```text
Staging and production currently share /var/www/bintrans-web-admin.
RBAC staging deployment remains blocked until roots are separated.
```

## Current State

| Environment | Vhost                         | Current Root                |
| ----------- | ----------------------------- | --------------------------- |
| production  | `00-bintrans-production.conf` | `/var/www/bintrans-web-admin` |
| staging     | `staging-bintrans.conf`       | `/var/www/bintrans-web-admin` |

Read-only confirmation (2026-07-28): production root is symlink to release directory; `/var/www/staging-bintrans-web-admin` does not exist.

Endpoint baseline: production and staging root/login/health all HTTP 200. `nginx -t`: ok.

## Approved Target State

| Environment | Target Root                         |
| ----------- | ----------------------------------- |
| production  | `/var/www/bintrans-web-admin`       |
| staging     | `/var/www/staging-bintrans-web-admin` |

## Approved Future Execution Scope

The next execution pack may do only the following:

```text
1. Backup staging Nginx config.
2. Create /var/www/staging-bintrans-web-admin if missing.
3. Copy current /var/www/bintrans-web-admin content to /var/www/staging-bintrans-web-admin as baseline.
4. Update only staging Nginx vhost root to /var/www/staging-bintrans-web-admin.
5. Run nginx -t.
6. Reload Nginx only if nginx -t passes.
7. Verify staging root/login/health.
8. Verify production root/login/health.
9. Record evidence.
```

## Forbidden Future Execution Scope

```text
Production web root update.
Production deploy.
Backend deploy.
Database migrations.
API contract changes.
Docker compose changes.
DNS changes.
Certbot actions.
SSL certificate changes.
Role apps deployment.
Reading/copying secrets or private keys.
```

## Rollback Boundary

```text
Rollback may restore only the staging vhost root back to /var/www/bintrans-web-admin.
Rollback must not alter production web root content.
Rollback must run nginx -t before reload.
```

## Production Boundary

```text
Production must remain on /var/www/bintrans-web-admin.
Production deploy is not approved.
Production static content must not be modified.
```

## Related Commits

```text
828cb2a docs: plan staging production web root separation
7cbaf4f docs: record blocked RBAC staging deployment
```

## Next Recommended Pack

```text
STAGING_WEB_ROOT_SEPARATION_EXECUTION_PACK v0.1
```
