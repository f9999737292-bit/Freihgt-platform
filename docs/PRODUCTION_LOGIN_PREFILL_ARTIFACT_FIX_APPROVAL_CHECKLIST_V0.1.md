# Production Login Prefill Artifact Fix Approval Checklist v0.1

## Approval Required Before Execution

| Check | Required |
|---|---|
| owner explicitly approves production artifact fix | yes |
| production deployment scope selected | yes |
| rollback plan documented | yes |
| production root backup path planned | yes |
| artifact source identified | yes |
| endpoint checks planned | yes |
| no Nginx/DNS/Certbot change required | yes |
| no backend/API/DB change required | yes |

## Candidate Execution Scope

Allowed only after explicit approval:

```text
1. Build or select approved web-admin static artifact.
2. Backup production web root.
3. Deploy only approved static artifact to /var/www/bintrans-web-admin.
4. Do not change Nginx.
5. Do not reload Nginx.
6. Verify production /, /login, /health.
7. Verify production login fields are empty.
8. Verify staging remains healthy.
```

## Forbidden Execution Scope

```text
Nginx config edits.
DNS changes.
Certbot actions.
Backend deploy.
Database migrations/writes.
API contract changes.
Source code changes unless separately approved.
Role app deployment.
Secrets/private key handling.
```

## Decision

```text
PRODUCTION_LOGIN_PREFILL_ARTIFACT_FIX_APPROVAL_CHECKLIST_CREATED
```
