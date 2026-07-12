# Low-code Pilot Week-3 Web-admin Deploy Checklist v0.1

## Summary

Checklist for deploying `apps/web-admin` to Selectel staging.

## Phase 1 — Prerequisites

| Step | Action | Done |
| ---- | ------ | ---- |
| 1 | API `/health` returns 200 | ☐ |
| 2 | Read-only API smoke passed | ☐ |
| 3 | Trusted SSH available | ☐ |
| 4 | Repo at `/opt/bintrans/freight-platform` | ☐ |
| 5 | Operator approval for staging deploy | ☐ |

## Phase 2 — Build

| Step | Action | Done |
| ---- | ------ | ---- |
| 6 | `npm ci` in `apps/web-admin` | ☐ |
| 7 | Set `NUXT_PUBLIC_API_BASE_URL` to staging API | ☐ |
| 8 | Set `NUXT_PUBLIC_DEFAULT_TENANT_ID` | ☐ |
| 9 | `NUXT_PUBLIC_MOCK_AUTH=false` | ☐ |
| 10 | `npm run build` succeeds | ☐ |

## Phase 3 — Nginx

| Step | Action | Done |
| ---- | ------ | ---- |
| 11 | Nginx serves web-admin static from `.output/public` | ☐ |
| 12 | `/api/` proxied to api-gateway :8080 | ☐ |
| 13 | Port 3000 not exposed publicly | ☐ |
| 14 | `nginx -t` passes | ☐ |

## Phase 4 — Verify

| Step | Action | Done |
| ---- | ------ | ---- |
| 15 | UI loads at staging URL | ☐ |
| 16 | Login flow reachable (no secrets in docs) | ☐ |
| 17 | Capture evidence pack | ☐ |

## Blockers

| ID | Status |
| -- | ------ |
| STG-LIM-001 | OPEN — DNS pending |
| STG-LIM-002 | OPEN — HTTPS pending |
| STG-LIM-003 | OPEN — deferred |
| STG-LIM-004 | OPEN — plan created, execution pending |

## Status

```text
WEB_ADMIN_DEPLOY_PLAN_CREATED_PENDING_EXECUTION
```

## Production-ready

```text
not claimed
```
