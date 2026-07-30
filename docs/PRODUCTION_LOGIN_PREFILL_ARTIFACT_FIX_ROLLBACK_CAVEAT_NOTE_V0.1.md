# Production Login Prefill Artifact Fix Rollback Caveat Note v0.1

## Summary

Post-deploy review identified a rollback caveat for the production login prefill artifact fix execution backup.

## Caveat

```text
Backup path exists: /root/production-login-prefill-fix-backup-20260730_200750.
The backup appears to be a symlink copy of the production root symlink, not a detached snapshot.
BACKUP_REAL resolves to the same path as PROD_REAL: /var/www/bintrans-web-admin-release-20260717_193920.
```

## Impact

```text
This is not a current production blocker because production is healthy and the login prefill issue is fixed.
However, if rollback is needed later, the rollback plan must verify whether this backup can restore the prior artifact safely.
```

## Required Future Rollback Rule

```text
Do not rely on this path as a detached artifact snapshot without re-verification.
Before any rollback, verify backup contents and target path behavior.
Rollback must not change Nginx/DNS/Certbot/backend/API/database unless separately approved.
```

## Current Status

```text
Production status: healthy
Production login prefill: removed
Rollback required now: no
```

## Decision

```text
PRODUCTION_LOGIN_PREFILL_ARTIFACT_FIX_ROLLBACK_CAVEAT_RECORDED
```
