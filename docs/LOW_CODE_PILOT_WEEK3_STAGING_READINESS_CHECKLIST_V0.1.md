# Low-code Pilot Week-3 Staging Readiness Checklist v0.1

## Summary

Ops checklist before handoff for **Remote Auth-On Staging Repeat** (PR-GAP-001).

**Default:** unknown items = **PENDING** or **MISSING**.

Reference: `LOW_CODE_PILOT_WEEK3_STAGING_DEPLOY_RUNBOOK_V0.1.md`

## Readiness Table

| Item | Status | Owner | Evidence | Notes |
|------|--------|-------|----------|-------|
| VPS/server available | **MISSING** | Ops | — | Required for Option A |
| DNS configured | **MISSING** | Ops | — | e.g. `staging.7rights.ru` |
| HTTPS configured | **MISSING** | Ops | — | TLS cert valid |
| Repository deployed | **PENDING** | Ops | — | Clone `main` from GitHub |
| Docker installed | **PENDING** | Ops | — | Engine + Compose v2 |
| Compose started | **PENDING** | Ops | — | All services healthy |
| Database available | **PENDING** | Ops | — | Staging DB only |
| Migrations applied | **PENDING** | Ops | — | Initial deploy only |
| LOW_CODE_ADMIN_AUTH_ENABLED=true | **MISSING** | Ops | — | low-code-service env |
| low-code-service restarted | **MISSING** | Ops | — | After auth-on change |
| Admin user created | **MISSING** | Ops / Security | — | PLATFORM_ADMIN |
| Non-admin user created | **MISSING** | Ops / Security | — | Without PLATFORM_ADMIN |
| Tenant created | **MISSING** | Ops | — | Pilot tenant UUID |
| Login/JWT flow confirmed | **MISSING** | Security | — | If gateway requires JWT |
| Read-only permission granted | **MISSING** | Security | — | Must be **yes** for matrix |
| Secrets not committed | **PENDING** | Ops | — | Server-side env only |
| Production data not used | **PENDING** | Ops / PM | — | Staging/test data only |

## Status Legend

| Status | Meaning |
|--------|---------|
| **MISSING** | Not started / not available |
| **PENDING** | In progress or unknown |
| **READY** | Confirmed with evidence |
| **BLOCKED** | Cannot proceed until dependency resolved |
| **NOT_APPLICABLE** | e.g. tunnel-only path for DNS row |

## Ready For Handoff

All **MISSING** → **READY** for required rows, then complete `LOW_CODE_PILOT_WEEK3_STAGING_INPUT_FORM_V0.1.md`.

## Next Pack

**Remote Auth-On Staging Repeat Pack v0.1** — when input form complete and read-only **yes**.
