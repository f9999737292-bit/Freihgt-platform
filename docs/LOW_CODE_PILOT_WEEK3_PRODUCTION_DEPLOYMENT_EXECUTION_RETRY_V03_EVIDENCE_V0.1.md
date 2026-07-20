# Production Deployment Execution Retry v0.3 Evidence v0.1

## Summary

Production deployment execution retry v0.3 completed after Nginx vhost investigation.

Dedicated production Nginx vhost was enabled for `xn--80abvubqje.xn--p1ai` using the existing Let's Encrypt certificate. Legacy `freight-staging` catch-all site was removed from `sites-enabled` to prevent apex HTTP requests from being proxied to the API gateway.

Production deploy checks PASS. Staging preserved.

## Decision

```text
PRODUCTION_DEPLOYMENT_EXECUTION_RETRY_V03_PASS
```

## Previous Attempts

| Attempt | Decision | Blocker |
| --- | --- | --- |
| v0.1 | PRODUCTION_DEPLOYMENT_EXECUTION_FAIL | production DNS not ready |
| v0.2 | PRODUCTION_DEPLOYMENT_EXECUTION_RETRY_FAIL | freight-staging default vhost + HTTPS verify fail; rollback PASS |
| Investigation | NGINX_VHOST_INVESTIGATION_COMPLETE | no production vhost; freight-staging caught apex HTTP |

## DNS Gate

| Resolver | Result |
| --- | --- |
| 1.1.1.1 | PASS — 161.104.53.221 |
| 8.8.8.8 | PASS — 161.104.53.221 |

## Target

| Field | Value |
| --- | --- |
| Target environment | current Selectel VM / current staging-to-production promotion |
| Target production domain | бинтранс.рф |
| Target production punycode | xn--80abvubqje.xn--p1ai |
| Server IP | 161.104.53.221 |
| Responsible operator | Феликс Асаев |
| Go/no-go owner | Феликс Асаев |

## Snapshot / Backup

```text
SNAPSHOT_CONFIRMED
Selectel backup: 6450ba4f-5e95-4052-a0fc-dea853399dad
Created at: 2026-07-20 14:52 MSK
```

## Server-side Backup

```text
Nginx backup path: /root/prod-deploy-retry-v03-final-backup-20260720_162412
```

## Server Changes

| Change | Result |
| --- | --- |
| Nginx backup | PASS |
| Production site enabled | PASS — `/etc/nginx/sites-enabled/00-bintrans-production.conf` |
| freight-staging removed from sites-enabled | PASS — prevents apex HTTP API gateway catch-all |
| Nginx config test | PASS |
| Nginx reload | PASS |
| Certbot | NOT EXECUTED — existing cert reused |
| Existing production cert used | PASS — expires 2026-10-18 |
| Staging domain preserved | PASS |

## Root Cause Fix Applied

```text
freight-staging in sites-enabled acted as implicit default/catch-all for apex HTTP Host and proxied / to API gateway (404 application/json).
Production vhost enabled with existing cert; freight-staging symlink removed from sites-enabled.
```

## Production Verification

| Check | Result |
| --- | --- |
| Production HTTPS root `/` | PASS — 200 text/html |
| Production HTTPS `/login` | PASS — 200 text/html |
| Production HTTPS `/health` | PASS — 200 |
| Production HTTP -> HTTPS redirect | PASS — 301 |
| Production API proxy read-only | PASS — 200 |
| Production Cyrillic HTTPS root | PASS — 200 text/html |

## Staging Preservation Verification

| Check | Result |
| --- | --- |
| Staging HTTPS root `/` | PASS — 200 text/html |
| Staging HTTPS `/health` | PASS — 200 |

## Production Deploy Status

```text
executed
```

## Rollback

```text
Rollback triggered: no
Rollback allowed: yes
Server-side Nginx backup path: /root/prod-deploy-retry-v03-final-backup-20260720_162412
Selectel backup: 6450ba4f-5e95-4052-a0fc-dea853399dad
```

## Safety

```text
Backend/frontend source changed during retry v0.3: no
Docker compose repo changed: no
UFW changed: no
DNS changed by this pack: no
CORS/.env changed: no
Certbot executed: no
Web-admin redeployed: no
Database writes executed: no
POST/PUT/PATCH/DELETE API calls executed: no
Secrets captured: no
Certificate private key captured: no
```
