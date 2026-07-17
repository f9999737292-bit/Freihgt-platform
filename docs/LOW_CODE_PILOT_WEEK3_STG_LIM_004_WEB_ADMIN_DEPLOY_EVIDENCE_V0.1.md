# STG-LIM-004 Web-admin Deploy Evidence v0.1

## Summary

Web-admin was deployed to the HTTPS staging domain on 2026-07-17.

Static build from `apps/web-admin` was generated with `nuxi generate` and served via Nginx at `/var/www/bintrans-web-admin`. `/health` and `/api/` remain proxied to API Gateway `127.0.0.1:8080`.

Production-ready is not claimed.

## Domain

Display domain:

```text
staging.бинтранс.рф
```

Technical / punycode domain:

```text
staging.xn--80abvubqje.xn--p1ai
```

Target IP:

```text
161.104.53.221
```

## Preconditions

```text
STG-LIM-001: CLOSED — DNS verified
STG-LIM-002: CLOSED — HTTPS / Certbot verified
STG-LIM-003: CLOSED — SSH SG /32 verified
```

## Build

```text
apps/web-admin build: PASS — nuxi generate (Nitro preset static)
Build env (shell only, not committed):
  NUXT_PUBLIC_API_BASE_URL=https://staging.xn--80abvubqje.xn--p1ai
  NUXT_PUBLIC_DEFAULT_TENANT_ID=74519f22-ff9b-4a8b-8fff-a958c689682f
  NUXT_PUBLIC_MOCK_AUTH=false
Static output path: apps/web-admin/.output/public
dist/index.html exists: n/a — Nuxt static output uses .output/public/index.html (yes)
Build archive created: yes — web-admin-dist-staging.tar.gz (not committed)
```

## Server Deployment

```text
Nginx backup created: yes
Backup path: /root/web-admin-deploy-backup-20260717_193918
Web root: /var/www/bintrans-web-admin
Release path: /var/www/bintrans-web-admin-release-20260717_193920
Nginx config updated: yes — /etc/nginx/sites-available/staging-bintrans.conf
Nginx config test: PASS
Nginx reload: PASS
```

## Routing

```text
SPA root served from HTTPS: PASS — 200 text/html
/login SPA fallback: PASS — 301 to /login/ then 200 text/html
/health proxied to API gateway: PASS — 200
/api/ proxied to API gateway: PASS — 200
HTTP -> HTTPS redirect: PASS — 301
```

## Verification Matrix

| Check                                        | Result | Notes                                              |
| -------------------------------------------- | ------ | -------------------------------------------------- |
| HTTPS root `/`                               | PASS   | 200 text/html                                      |
| HTTPS `/login`                               | PASS   | 301 to /login/ then 200 text/html (Nuxt trailing)  |
| HTTPS `/health`                              | PASS   | 200 — proxied to API gateway                       |
| HTTP root redirect                           | PASS   | 301 to HTTPS                                       |
| Cyrillic HTTPS root                          | PASS   | 200 text/html                                      |
| API proxy read-only low-code active template | PASS   | 200 with X-Tenant-ID                               |
| Server nginx -t                              | PASS   | nginx -t successful                                |
| Docker containers                            | PASS   | 10 containers Up (healthy)                         |

## Machine-captured Output (sanitized)

External verification:

```text
HTTPS /: 200 text/html
HTTPS /login: 301 -> /login/ -> 200 text/html
HTTPS /health: 200
HTTP / redirect: 301 https://staging.xn--80abvubqje.xn--p1ai/
HTTPS Cyrillic /: 200 text/html
API GET /api/v1/low-code/form-templates/active?entity_type=TRANSPORT_ORDER: 200
```

## Decision

```text
STG_LIM_004_WEB_ADMIN_DEPLOY_PASS
```

## STG-LIM-004

```text
READY_FOR_CLOSURE_REVIEW
```

## Safety

```text
Backend/frontend source changed: no
Docker compose repo changed: no
UFW changed: no
CORS/.env changed: no
POST/PUT/PATCH/DELETE executed: no
Secrets captured: no
Certificate private key captured: no
Production-ready claimed: no
```

## Operator Follow-up

```text
1. Prepare STG-LIM-004 closure review pack
2. Rollback available from /root/web-admin-deploy-backup-20260717_193918 if needed
```
