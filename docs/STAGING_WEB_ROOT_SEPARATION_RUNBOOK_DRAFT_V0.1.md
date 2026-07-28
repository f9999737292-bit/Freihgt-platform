# Staging Web Root Separation Runbook Draft v0.1

## Summary

Draft runbook for a future approved staging web root separation.

This draft must not be executed until explicit approval is granted.

## Target

```text
production root: /var/www/bintrans-web-admin
staging root: /var/www/staging-bintrans-web-admin
```

## Current Baseline (Read-only, 2026-07-28)

```text
production vhost: /etc/nginx/sites-enabled/00-bintrans-production.conf
staging vhost: /etc/nginx/sites-enabled/staging-bintrans.conf
shared root: /var/www/bintrans-web-admin (symlink to release directory)
target staging root exists: no
nginx -t: ok
```

## Future Execution Outline

```text
1. Confirm production/staging endpoints are 200.
2. Backup staging Nginx config (staging-bintrans.conf).
3. Create /var/www/staging-bintrans-web-admin.
4. Copy current /var/www/bintrans-web-admin content to staging root.
5. Change only staging vhost root to /var/www/staging-bintrans-web-admin.
6. Run nginx -t.
7. Reload Nginx only if syntax passes.
8. Verify staging endpoints (/, /login, /health).
9. Verify production endpoints (/, /login, /health).
10. Record evidence.
```

## Future Rollback Outline

```text
1. Restore staging vhost root to /var/www/bintrans-web-admin.
2. Run nginx -t.
3. Reload Nginx only if syntax passes.
4. Verify staging endpoints.
5. Verify production endpoints.
```

## Not Approved Here

```text
This document is a draft only.
Do not execute server changes from this pack.
```

## Decision

```text
STAGING_WEB_ROOT_SEPARATION_RUNBOOK_DRAFT_CREATED
```
