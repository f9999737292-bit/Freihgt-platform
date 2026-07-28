# Staging Web Root Separation Execution Evidence v0.1

## Summary

Controlled server-side staging web root separation completed.

Only the staging Nginx vhost root was changed. Production web root and production vhost were not changed. RBAC staging deployment was not executed in this pack.

## Decision

```text
STAGING_WEB_ROOT_SEPARATION_EXECUTION_COMPLETE
```

## User Approval

```text
Подтверждаю STAGING_WEB_ROOT_SEPARATION_EXECUTION_PACK. Разрешаю изменить только staging Nginx root на /var/www/staging-bintrans-web-admin. Production не трогать.
```

## Scope

| Item                             | Result |
| -------------------------------- | ------ |
| staging root separation executed | yes    |
| staging Nginx vhost edited       | yes    |
| production vhost edited          | no     |
| production web root modified     | no     |
| RBAC staging deploy executed     | no     |
| production deploy executed       | no     |
| backend deploy executed          | no     |
| migrations executed              | no     |
| database writes executed         | no     |
| DNS changed                      | no     |
| Certbot executed                 | no     |

## Roots

| Environment | Before                      | After                               |
| ----------- | --------------------------- | ----------------------------------- |
| production  | /var/www/bintrans-web-admin | /var/www/bintrans-web-admin         |
| staging     | /var/www/bintrans-web-admin | /var/www/staging-bintrans-web-admin |

## Nginx

| Item                   | Value                       |
| ---------------------- | --------------------------- |
| staging vhost          | staging-bintrans.conf       |
| production vhost       | 00-bintrans-production.conf |
| nginx -t before reload | pass                        |
| nginx reload executed  | yes                         |
| nginx -t after reload  | pass                        |

## Backup

| Item              | Value                                                      |
| ----------------- | ---------------------------------------------------------- |
| backup path       | /root/staging-web-root-separation-backup-20260728_214238   |
| rollback executed | no                                                         |

## Endpoint Verification

| Check              | Before    | After |
| ------------------ | --------- | ----- |
| production /       | 200       | 200   |
| production /login  | 200       | 200   |
| production /health | 200       | 200   |
| staging /          | 200       | 200   |
| staging /login     | 200       | 200   |
| staging /health    | 200       | 200   |

## Result

```text
Web root separation complete.
RBAC staging deployment can now be retried using a separate staging deployment pack.
```

## Safety Result

```text
Production changed: no
Staging changed: yes, staging Nginx root only
Server changed: yes, staging Nginx root and staging web root baseline only
Nginx changed: yes, staging vhost root only
DNS changed: no
Certbot changed: no
Backend changed: no
API contracts changed: no
Migrations changed: no
Database writes executed: no
Secrets captured: no
Deploy executed: no RBAC deploy; root separation only
```

## Next Recommended Pack

```text
RBAC_ROLE_NAVIGATION_STAGING_DEPLOYMENT_RETRY_PACK v0.1
```
