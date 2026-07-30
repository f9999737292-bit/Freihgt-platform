# Production Login Prefill Artifact Fix Execution Evidence v0.1

## Summary

Production static web-admin artifact refresh executed to remove pre-existing login credential prefill.

Deployment updated only production static content under `/var/www/bintrans-web-admin`. Staging, Nginx, DNS, Certbot, backend, API, and database were not modified.

This execution also promoted the QA-signed RBAC role navigation frontend artifact to production static UI.

## Decision

```text
PRODUCTION_LOGIN_PREFILL_ARTIFACT_FIX_EXECUTION_COMPLETE
```

## User Approval

```text
Подтверждаю PRODUCTION_LOGIN_PREFILL_ARTIFACT_FIX_EXECUTION_PACK. Разрешаю обновить только production static artifact в /var/www/bintrans-web-admin для удаления login prefill. Nginx/DNS/Certbot/backend/API/DB не трогать.
```

## Source / Commits

| Item | Value |
|---|---|
| execution HEAD | `a2d378a` — docs: approve production login prefill artifact fix |
| plan commit | `bc46bfb` — docs: plan production login prefill artifact fix |
| approval commit | `a2d378a` — docs: approve production login prefill artifact fix |
| RBAC staging QA signoff | `e0ee658` |
| selected path | Option A — QA-signed web-admin artifact refresh |
| source diff before execution | empty |

## Build

| Item | Value |
|---|---|
| build command | `nuxi generate` |
| build env | `NUXT_PUBLIC_API_BASE_URL=https://xn--80abvubqje.xn--p1ai`, `NUXT_PUBLIC_DEFAULT_TENANT_ID=74519f22-ff9b-4a8b-8fff-a958c689682f`, `NUXT_PUBLIC_MOCK_AUTH=false` |
| build result | pass |
| artifact contains index.html | yes |
| local artifact path | `C:\Users\Пользователь\AppData\Local\Temp\bintrans-web-admin-production-prefill-fix-20260730_200547.tar.gz` |
| artifact size | 285746 bytes |
| remote artifact path | `/tmp/bintrans-web-admin-production-prefill-fix.tar.gz` |
| demo@7rights in built JS | not found |

## Root Safety

| Item | Value |
|---|---|
| production root | `/var/www/bintrans-web-admin` |
| staging root | `/var/www/staging-bintrans-web-admin` |
| PROD_REAL | `/var/www/bintrans-web-admin-release-20260717_193920` |
| STG_REAL | `/var/www/staging-bintrans-web-admin` |
| resolved roots distinct | yes |
| safety gate | `ROOTS_DISTINCT_PASS` |

## Server Deployment

| Item | Result |
|---|---|
| deploy executed | yes |
| deploy target | `/var/www/bintrans-web-admin` only |
| production backup path | `/root/production-login-prefill-fix-backup-20260730_200750` |
| staging root modified | no |
| Nginx changed | no |
| Nginx reload executed | no |
| DNS changed | no |
| Certbot changed | no |
| backend deploy executed | no |
| migrations executed | no |
| database writes executed | no |

## Endpoint Verification

| Check | Before | After |
|---|---|---|
| production / | 200 | 200 |
| production /login | 200 | 200 |
| production /health | 200 | 200 |
| staging / | 200 | 200 |
| staging /login | 200 | 200 |
| staging /health | 200 | 200 |

## Browser Smoke

| Check | Result |
|---|---|
| production /login opens | pass |
| production login prefill not observed | pass |
| production demo/mock banner not observed in fetched HTML | pass |
| staging /login remains healthy | pass |
| staging login prefill not observed | pass |
| authenticated sidebar smoke | not tested in this pack |

Notes:

- Production `/login` HTML fetch after deploy shows no `demo@7rights`, `123456`, or mock-mode markers.
- Staging remained unchanged and healthy.

## Safety Result

```text
Production changed: yes, static web-admin content only
Production deploy executed: yes
Production root modified: yes
Staging changed: no
Server changed: yes, production static web root content only
Nginx changed: no
Nginx reload executed: no
DNS changed: no
Certbot changed: no
Backend changed: no
API contracts changed: no
Migrations changed: no
Database writes executed: no
Source code changed: no
Secrets captured: no
```

## Rollback Reference

If rollback is required, restore from:

```text
/root/production-login-prefill-fix-backup-20260730_200750
```

Rollback trade-off: may restore older artifact and pre-existing login prefill.
