# Production Login Prefill Artifact Fix Rollback Boundary v0.1

## Summary

Rollback boundary for a future production login prefill artifact fix.

This document is planning/approval evidence only. It does not execute rollback or production changes.

## Future Backup Requirement

```text
Before replacing production static content, create a full backup of /var/www/bintrans-web-admin.
The backup path should use /root/production-login-prefill-fix-backup-<timestamp>.
```

## Future Rollback Scope

Allowed only if future production artifact fix execution breaks production:

```text
1. Restore /var/www/bintrans-web-admin from the execution backup.
2. Do not change Nginx.
3. Do not reload Nginx unless separately justified and approved.
4. Do not change DNS/Certbot.
5. Do not change backend/API/database.
6. Verify production and staging endpoints.
7. Record rollback evidence.
```

## Rollback Trade-off

```text
Rollback may restore the older production artifact and therefore may restore the demo credential prefill.
If rollback is used, the login prefill issue remains open.
```

## Not Approved

```text
Rollback is not executed in this pack.
Production execution is not approved in this pack.
```

## Decision

```text
PRODUCTION_LOGIN_PREFILL_ARTIFACT_FIX_ROLLBACK_BOUNDARY_CREATED
```
