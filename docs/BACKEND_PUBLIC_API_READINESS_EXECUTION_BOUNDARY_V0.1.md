# Backend Public API Readiness Execution Boundary v0.1

## Summary

Execution boundary for future backend public API readiness work.

No execution is approved in this document.

Base commit: `83a730d`.

## Allowed Future Execution Types

Only one may be selected after diagnostic and explicit approval:

| Type | Scope |
|---|---|
| browser/runtime health fix | source/frontend/backend targeted change, if confirmed |
| gateway route normalization | add/enable `/api/health` alias, if required |
| documentation-only canonical path | no runtime change, if `/health` + `/api/v1/*` is sufficient |
| controlled static demo | leave live-data partial and demo UI only |

## Current Non-Execution Decision

```text
BACKEND_PUBLIC_API_EXECUTION_NOT_APPROVED_YET
```

## Nginx Boundary

```text
Nginx changes are not selected as primary path because current evidence shows /api/* reaches the gateway and fails with gateway/application ROUTE_NOT_FOUND.
Production and staging already have:
  location = /health → proxy_pass http://127.0.0.1:8080/health
  location /api/   → proxy_pass http://127.0.0.1:8080
Any future Nginx change requires separate explicit approval and must not be assumed from this boundary.
```

## Security Boundary

```text
Do not open internal service ports publicly.
Do not expose unprotected service health or admin endpoints unintentionally.
Public API exposure must remain behind Nginx/gateway and must preserve auth/security assumptions.
If CORS origins are expanded, limit to approved production/staging hosts only.
```

## Data Boundary

```text
No database migration/write is expected for public API readiness.
If a future fix requires DB migration, stop and create a separate migration approval pack.
```

## Rollback / No-Change Boundary

| Scenario | Rollback / no-change action |
|---|---|
| gateway CORS change | revert env/config; redeploy gateway only if execution approved |
| gateway `/api/health` alias | revert route; redeploy gateway |
| frontend apiBaseUrl rebuild | restore prior static artifact from backup path |
| documentation-only (Candidate C) | no runtime rollback needed |
| no fix selected | keep static UI demo signed off; live-data remains partial |

Production static artifact backup reference: `/root/production-login-prefill-fix-backup-20260730_200750` (symlink copy caveat retained).

## Required Future Checks

```text
1. production endpoints before/after.
2. staging endpoints before/after.
3. browser health/banner behavior (Unicode + punycode hosts).
4. /health.
5. /api/health if selected.
6. /api/v1/* representative route.
7. no internal ports publicly exposed.
8. rollback path documented and tested read-only where possible.
```

## Decision

```text
BACKEND_PUBLIC_API_READINESS_EXECUTION_BOUNDARY_CREATED
```
