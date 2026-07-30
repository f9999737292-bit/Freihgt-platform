# Backend Public API Readiness Approval Checklist v0.1

## Approval Required Before Execution

| Check | Required |
|---|---|
| root cause classified | yes |
| selected fix option documented | yes |
| public API path selected | yes |
| gateway target selected | yes |
| frontend API base URL decision documented | yes |
| Nginx change needed yes/no documented | yes |
| backend/source change needed yes/no documented | yes |
| rollback plan documented | yes |
| security guardrails documented | yes |
| post-deploy review planned | yes |
| browser login backend-status verification planned | yes |

## Possible Future Execution Scopes

Only one should be selected in approval pack:

| Option | Scope |
|---|---|
| A | Nginx `/api/` proxy/rewrite to gateway — **likely not needed; already present** |
| B | production static frontend API base URL refresh — **only if browser origin mismatch confirmed** |
| C | gateway route normalization — **primary candidate: `/api/health` alias or documented `/health` canonical path** |
| D | keep API disabled and prepare static-only demo |

## Planning Inputs (from audit v0.1)

| Item | Current value |
|---|---|
| Nginx /api proxy | present on production and staging |
| Nginx /health proxy | present on production and staging |
| gateway health path | `/health` |
| gateway v1 API prefixes | `/api/v1/*` (auth, users, companies, transport-orders, …) |
| frontend health path | `{apiBaseUrl}/health` |
| frontend API path | `{apiBaseUrl}/api/v1/*` |
| deployed mockAuth | false |

## Explicitly Not Approved In This Plan

```text
Nginx edits.
Nginx reload.
Backend deploy.
Source code changes.
Frontend artifact refresh.
Database migrations/writes.
Docker restart.
Opening ports.
Public API exposure changes.
```

## Decision

```text
BACKEND_PUBLIC_API_READINESS_APPROVAL_CHECKLIST_CREATED
```
