# Production Deployment Closure Note v0.1

## Summary

Production deployment closure review completed after successful execution retry v0.3.

## Decision

```text
PRODUCTION_DEPLOYMENT_CLOSED
```

## Production Status

```text
Production deploy: executed
Production domain: https://бинтранс.рф/
Production punycode: https://xn--80abvubqje.xn--p1ai/
```

## Staging Status

```text
Staging preserved: yes
Staging domain: https://staging.бинтранс.рф/
```

## Evidence Chain

```text
Execution retry v0.3 evidence: docs/LOW_CODE_PILOT_WEEK3_PRODUCTION_DEPLOYMENT_EXECUTION_RETRY_V03_EVIDENCE_V0.1.md
Execution evidence: docs/LOW_CODE_PILOT_WEEK3_PRODUCTION_DEPLOYMENT_EXECUTION_EVIDENCE_V0.1.md
Nginx investigation evidence: docs/LOW_CODE_PILOT_WEEK3_NGINX_VHOST_INVESTIGATION_EVIDENCE_V0.1.md
Snapshot confirmation: docs/LOW_CODE_PILOT_WEEK3_SNAPSHOT_CONFIRMATION_CAPTURE_V0.1.md
```

## Backup / Rollback

| Item | Value |
| --- | --- |
| Selectel backup | 6450ba4f-5e95-4052-a0fc-dea853399dad |
| Server Nginx backup | /root/prod-deploy-retry-v03-final-backup-20260720_162412 |
| Rollback allowed | yes |
| Rollback triggered during successful v0.3 | no |

## External Closure Verification

| Check | Result |
| --- | --- |
| Production DNS | PASS — 161.104.53.221 |
| Production HTTPS root `/` | PASS — 200 text/html |
| Production HTTPS `/login` | PASS — 200 text/html |
| Production HTTPS `/health` | PASS — 200 |
| Production HTTP -> HTTPS | PASS — 301 |
| Production API proxy read-only | PASS — 200 |
| Production Cyrillic HTTPS root | PASS — 200 text/html |
| Staging HTTPS root `/` | PASS — 200 text/html |
| Staging HTTPS `/health` | PASS — 200 |
| Staging API proxy read-only | PASS — 200 |

## Server Closure Verification

| Check | Result |
| --- | --- |
| nginx -t | PASS |
| production site enabled | PASS — `/etc/nginx/sites-enabled/00-bintrans-production.conf` |
| staging site enabled | PASS — `/etc/nginx/sites-enabled/staging-bintrans.conf` |
| freight-staging disabled | PASS |
| production cert exists | PASS — expires 2026-10-18 |
| staging cert exists | PASS — expires 2026-10-15 |
| certbot timer | active — next run 2026-07-21 00:00:54 MSK |
| docker containers | PASS — 10/10 healthy/running |
| server local production HTTPS root | PASS — 200 text/html |
| server local production HTTPS login | PASS — 200 text/html |
| server local production HTTPS health | PASS — 200 |
| server local staging HTTPS root | PASS — 200 text/html |
| server local staging HTTPS health | PASS — 200 |

## Safety

```text
Backend/frontend source changed during closure review: no
Docker compose repo changed: no
UFW changed: no
DNS changed during closure review: no
Nginx changed during closure review: no
Certbot executed during closure review: no
Web-admin redeployed during closure review: no
Database writes executed: no
POST/PUT/PATCH/DELETE API calls executed: no
Secrets captured: no
Certificate private key captured: no
```
