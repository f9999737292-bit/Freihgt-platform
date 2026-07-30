# Production Login Prefill Artifact Fix Runbook Draft v0.1

## Summary

Draft runbook for a future approved production artifact fix.

This draft must not be executed until explicit production execution approval is granted.

## Preconditions

```text
1. Production execution approval received.
2. Approved artifact source selected.
3. Production root backup prepared.
4. Production and staging endpoints healthy before execution.
```

## Draft Execution Outline

```text
1. Confirm branch and approved commit.
2. Confirm source/static artifact has production-safe login behavior.
3. Build or select approved web-admin static artifact.
4. Confirm artifact contains index.html.
5. Verify production /, /login, /health before.
6. Backup /var/www/bintrans-web-admin.
7. Deploy approved static artifact to /var/www/bintrans-web-admin only.
8. Do not change Nginx.
9. Do not reload Nginx.
10. Verify production /, /login, /health after.
11. Verify production /login fields are empty.
12. Verify staging remains healthy.
13. Record evidence.
```

## Draft Rollback Outline

```text
1. Restore production root from backup if production is broken.
2. Do not change Nginx.
3. Verify production /, /login, /health.
4. Verify staging remains healthy.
5. Record rollback evidence.
```

## Not Approved Here

```text
This is a draft only.
Do not execute production changes from this pack.
```

## Decision

```text
PRODUCTION_LOGIN_PREFILL_ARTIFACT_FIX_RUNBOOK_DRAFT_CREATED
```
