# Staging Backend DB Isolation Rollback Plan v0.1

## Summary

Rollback plan for future staging backend/DB isolation execution.

This document does not execute rollback.

Base commit: `5f5ad4c`.

## Decision

```text
STAGING_BACKEND_DB_ISOLATION_ROLLBACK_PLAN_CREATED
```

## Backup Requirements Before Future Execution

```text
1. Backup Nginx site configs.
2. Record current enabled Nginx sites.
3. Record current Docker compose ps.
4. Record current production/staging endpoint baseline.
5. Record current Git HEAD.
6. Do not backup or expose secret values in docs.
```

## Rollback Triggers

| Trigger                       | Action                                                         |
| ----------------------------- | -------------------------------------------------------------- |
| production / fails            | revert Nginx staging change if related; check production vhost |
| production /health fails      | revert change and inspect gateway                              |
| staging /health fails         | revert staging proxy or stop staging stack                     |
| Nginx test fails              | do not reload                                                  |
| Docker staging stack fails    | stop/remove staging stack only                                 |
| production container affected | stop immediately and restore from backup                       |
| secret exposure risk          | stop and rotate affected secrets                               |

## Rollback Actions

```text
1. Restore staging Nginx config from backup if needed.
2. Stop staging Docker project only.
3. Remove staging containers only if approved.
4. Keep production containers untouched.
5. Keep production DB untouched.
6. Verify production endpoints.
7. Record rollback evidence.
```

## Not Approved

```text
No rollback is executed here.
No destructive delete is approved here.
No production rollback is approved here.
```
