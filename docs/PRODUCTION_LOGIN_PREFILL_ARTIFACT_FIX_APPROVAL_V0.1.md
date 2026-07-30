# Production Login Prefill Artifact Fix Approval v0.1

## Summary

Approval boundary prepared for a future production artifact fix that removes the pre-existing production login credential prefill.

This pack does not execute production changes, does not deploy, does not change source code, does not change server/Nginx/DNS/Certbot/backend/API/database, and does not modify production.

## Decision

```text
PRODUCTION_LOGIN_PREFILL_ARTIFACT_FIX_APPROVED_FOR_EXECUTION_PACK
```

## User Approval

```text
Подтверждаю PRODUCTION_LOGIN_PREFILL_ARTIFACT_FIX_APPROVAL_PACK. Разрешаю подготовить approval boundary для будущего production artifact fix. Production пока не менять.
```

## Issue

```text
Production /login has a pre-existing demo credential prefill from an older production artifact.
Staging /login is production-safe after RBAC staging deployment.
```

## Selected Future Path

```text
Option A — QA-signed web-admin artifact refresh.
```

This future path means:

* build/select a production static artifact from the current QA-signed web-admin source;
* deploy it only to production static root after explicit execution approval;
* do not change Nginx/DNS/Certbot/backend/API/database.

## Approved Future Execution Scope

Allowed only in the next execution pack after explicit execution approval:

```text
1. Build/select approved web-admin static artifact from current QA-signed source.
2. Confirm artifact contains index.html.
3. Verify production /, /login, /health before execution.
4. Verify staging /, /login, /health before execution.
5. Backup production web root /var/www/bintrans-web-admin.
6. Deploy approved static artifact only to /var/www/bintrans-web-admin.
7. Do not change Nginx.
8. Do not reload Nginx.
9. Verify production /, /login, /health after execution.
10. Verify production /login has empty credential fields.
11. Verify staging remains healthy.
12. Record evidence.
```

## Explicitly Not Approved

```text
Nginx edits.
Nginx reload.
DNS changes.
Certbot actions.
Backend deploy.
API contract changes.
Database migrations/writes.
Source code changes.
Role app deployment.
Secrets/private key handling.
Any production change outside /var/www/bintrans-web-admin static artifact content.
```

## Production Boundary

```text
Future execution may modify only production web-admin static content under /var/www/bintrans-web-admin.
It must not modify staging root /var/www/staging-bintrans-web-admin.
It must not modify production Nginx config.
It must not modify certificates or DNS.
```

## Rollback Boundary

```text
Future execution must create a production root backup before artifact replacement.
Rollback may restore /var/www/bintrans-web-admin from that backup if production is broken.
Rollback must not change Nginx/DNS/Certbot/backend/API/database.
Rollback may restore the old artifact, which may also restore the pre-existing login prefill; this trade-off must be recorded if rollback is used.
```

## Next Recommended Pack

```text
PRODUCTION_LOGIN_PREFILL_ARTIFACT_FIX_EXECUTION_PACK v0.1
```
