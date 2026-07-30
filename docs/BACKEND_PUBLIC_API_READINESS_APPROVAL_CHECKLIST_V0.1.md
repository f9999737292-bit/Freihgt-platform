# Backend Public API Readiness Approval Checklist v0.1

## Approval Boundary Checks

| Check | Result |
|---|---|
| plan committed | yes — `83a730d` |
| routing audit committed | yes — `83a730d` |
| root cause classified | yes — gateway route missing + browser/runtime TBD |
| Nginx change selected as primary path | no |
| browser/runtime diagnostic required | yes |
| browser/runtime diagnostic completed | no — deferred to diagnostic pack |
| gateway route normalization candidate recorded | yes — Candidate B |
| source/backend execution approved now | no |
| production execution approved now | no |

## Required Before Execution

| Check | Required |
|---|---|
| browser health diagnostic complete | yes |
| exact fix path selected | yes |
| owner execution approval | yes |
| rollback plan | yes |
| endpoint baseline | yes |
| security boundary | yes |
| post-deploy review plan | yes |

## Planning Inputs (from audit + approval v0.1)

| Item | Current value |
|---|---|
| Nginx /api proxy | present on production and staging |
| Nginx /health proxy | present on production and staging |
| gateway health path | `/health` |
| gateway v1 API prefixes | `/api/v1/*` |
| frontend health path | `{apiBaseUrl}/health` |
| frontend API path | `{apiBaseUrl}/api/v1/*` |
| deployed mockAuth | false |
| deployed apiBaseUrl | `https://xn--80abvubqje.xn--p1ai` |
| selected strategy | `API_READINESS_BROWSER_FIRST_GATEWAY_NORMALIZATION` |

## Possible Future Execution Scopes

Only one should be selected after browser diagnostic:

| Option | Scope |
|---|---|
| A | browser/runtime health fix (CORS, apiBaseUrl alignment, parsing) |
| B | gateway `/api/health` alias |
| C | documentation-only canonical path (`/health` + `/api/v1/*`) |
| D | controlled static demo only |

Nginx `/api/` proxy change is **not selected** — proxy already present.

## Current Decision

```text
BACKEND_PUBLIC_API_READINESS_APPROVAL_BOUNDARY_COMPLETE
BACKEND_PUBLIC_API_EXECUTION_NOT_APPROVED_YET
```

## Explicitly Not Approved

```text
Production execution.
Nginx edits/reload.
Backend deploy.
Source changes.
Frontend artifact refresh.
Docker restart.
Database writes.
Opening ports.
Public API exposure changes without separate execution approval.
```
