# Production Deployment Execution Retry Evidence v0.1

## Summary

Production deployment execution retry was attempted after production DNS gate was fixed.

Server-side Nginx and Certbot steps partially executed. Final production HTTPS verification failed and automatic Nginx rollback was triggered.

Production deploy is not complete.

## Decision

```text
PRODUCTION_DEPLOYMENT_EXECUTION_RETRY_FAIL
```

## Failure Reason

```text
PRODUCTION_DEPLOYMENT_EXECUTION_RETRY_SERVER_HTTPS_VERIFICATION_FAIL
```

Machine-captured server output:

```text
prod_http_root=404 application/json
prod_http_health=200
certbot: Successfully received certificate for xn--80abvubqje.xn--p1ai
prod_https_root=000
ERROR_LINE=174
ROLLBACK_TRIGGERED=yes
ROLLBACK_NGINX_RESTORED=PASS
```

Temporary HTTP production root returned `404 application/json` instead of expected `200 text/html`. Final server-side HTTPS root check returned `000` (curl SSL error 60). Automatic Nginx rollback restored pre-retry configuration.

Certbot certificate for `xn--80abvubqje.xn--p1ai` was issued and remains on server, but production Nginx site was rolled back and is not enabled.

## Previous Attempt

```text
Previous decision: PRODUCTION_DEPLOYMENT_EXECUTION_FAIL
Previous blocker: PRODUCTION_DEPLOYMENT_EXECUTION_BLOCKED_PRODUCTION_DNS_NOT_READY
Previous commit: 8f16d2d
```

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
Backup type: Полный
Size: 9 ГБ
```

## Pre-retry Staging Sanity

| Check | Result |
| --- | --- |
| Staging HTTPS root `/` | PASS — 200 text/html |
| Staging HTTPS `/login` | PASS — 200 text/html |
| Staging HTTPS `/health` | PASS — 200 |
| Staging API proxy read-only | PASS — 200 |

## Server-side Backup

```text
Nginx backup path: /root/prod-deploy-retry-backup-20260720_154539
```

## Server Changes

| Change | Result |
| --- | --- |
| Nginx backup | PASS |
| Temporary HTTP production site | PARTIAL — prod_http_root 404 application/json |
| Nginx config test | PASS |
| Nginx reload | PASS |
| Certbot certificate for xn--80abvubqje.xn--p1ai | PASS — issued, expires 2026-10-18 |
| Final HTTPS production site | PARTIAL — server HTTPS verify FAIL |
| Automatic Nginx rollback | PASS — ROLLBACK_NGINX_RESTORED=PASS |
| Production Nginx site after rollback | REMOVED |
| Staging domain preserved | PASS — external verify 200 |

## Production Verification

| Check | Result |
| --- | --- |
| Production HTTPS root `/` | FAIL — 000 |
| Production HTTPS `/login` | FAIL — 000 |
| Production HTTPS `/health` | FAIL — 000 |
| Production HTTP -> HTTPS redirect | FAIL — HTTP 404 |
| Production API proxy read-only | FAIL — 000 |
| Production Cyrillic HTTPS root | FAIL — 000 |

## Staging Preservation Verification

| Check | Result |
| --- | --- |
| Staging HTTPS root `/` | PASS — 200 text/html |
| Staging HTTPS `/health` | PASS — 200 |

## Production Deploy Status

```text
not executed
```

## Rollback

```text
Rollback triggered: yes
Rollback allowed: yes
Rollback result: ROLLBACK_NGINX_RESTORED=PASS
Server-side Nginx backup path: /root/prod-deploy-retry-backup-20260720_154539
Selectel backup: 6450ba4f-5e95-4052-a0fc-dea853399dad
```

## Safety

```text
Backend/frontend source changed during retry: no
Docker compose repo changed: no
UFW changed: no
DNS changed by this pack: no
CORS/.env changed: no
Certbot executed: yes, for xn--80abvubqje.xn--p1ai
Web-admin redeployed: no
Database writes executed: no
POST/PUT/PATCH/DELETE API calls executed: no
Secrets captured: no
Certificate private key captured: no
```
