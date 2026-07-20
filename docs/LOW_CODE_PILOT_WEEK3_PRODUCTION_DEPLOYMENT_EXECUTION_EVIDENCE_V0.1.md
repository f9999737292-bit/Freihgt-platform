# Production Deployment Execution Evidence v0.1

## Summary

Production deployment execution was attempted for current Selectel VM promotion to production domain.

Execution was blocked at the mandatory production DNS gate. No server-side Nginx or Certbot changes were executed.

## Decision

```text
PRODUCTION_DEPLOYMENT_EXECUTION_FAIL
```

## Failure Reason

```text
PRODUCTION_DEPLOYMENT_EXECUTION_BLOCKED_PRODUCTION_DNS_NOT_READY
```

Production apex domain `xn--80abvubqje.xn--p1ai` (бинтранс.рф) did not resolve to `161.104.53.221` on public resolvers `1.1.1.1` and `8.8.8.8`.

Required DNS A-record before retry:

```text
бинтранс.рф A 161.104.53.221
xn--80abvubqje.xn--p1ai A 161.104.53.221
```

## Target

| Field | Value |
| --- | --- |
| Target environment | current Selectel VM / current staging-to-production promotion |
| Target production domain | бинтранс.рф |
| Target production punycode | xn--80abvubqje.xn--p1ai |
| Server IP | 161.104.53.221 |
| Responsible operator | Феликс Асаев |
| Go/no-go owner | Феликс Асаев |

## Backup / Snapshot Gate

```text
SNAPSHOT_CONFIRMED
```

Backup:

```text
6450ba4f-5e95-4052-a0fc-dea853399dad
```

## Production DNS Gate

| Resolver | Expected A | Result |
| --- | --- | --- |
| 1.1.1.1 | 161.104.53.221 | FAIL — no A record |
| 8.8.8.8 | 161.104.53.221 | FAIL — no A record |

## Pre-execution Staging Sanity

| Check | Result |
| --- | --- |
| Staging HTTPS root `/` | PASS — 200 text/html |
| Staging HTTPS `/login` | PASS — 200 text/html |
| Staging HTTPS `/health` | PASS — 200 |
| Staging API proxy read-only | PASS — 200 |

## Server Changes

| Change | Result |
| --- | --- |
| Nginx backup | NOT EXECUTED — blocked by DNS gate |
| Nginx production site | NOT EXECUTED |
| Nginx reload | NOT EXECUTED |
| Certbot | NOT EXECUTED |
| HTTPS production domain | NOT ENABLED |

## Production Verification

| Check | Result |
| --- | --- |
| Production HTTPS root `/` | NOT EXECUTED |
| Production HTTPS `/login` | NOT EXECUTED |
| Production HTTPS `/health` | NOT EXECUTED |
| Production HTTP -> HTTPS redirect | NOT EXECUTED |
| Production API proxy read-only | NOT EXECUTED |
| Production Cyrillic HTTPS root | NOT EXECUTED |

## Staging Preservation Verification

| Check | Result |
| --- | --- |
| Staging HTTPS root `/` | PASS — 200 text/html (pre-execution) |
| Staging HTTPS `/health` | PASS — 200 (pre-execution) |

## Production Deploy Status

```text
not executed
```

## Rollback

```text
Rollback triggered: no
Rollback allowed: yes
Selectel backup: 6450ba4f-5e95-4052-a0fc-dea853399dad
Server-side Nginx backup path: n/a — server script not executed
```

## Retry

```text
Decision: PRODUCTION_DEPLOYMENT_EXECUTION_RETRY_FAIL
Evidence: docs/LOW_CODE_PILOT_WEEK3_PRODUCTION_DEPLOYMENT_EXECUTION_RETRY_EVIDENCE_V0.1.md
Retry date: 2026-07-20
Retry blocker: PRODUCTION_DEPLOYMENT_EXECUTION_RETRY_SERVER_HTTPS_VERIFICATION_FAIL
Production DNS gate retry: PASS
Production deploy executed: no
Rollback triggered: yes — ROLLBACK_NGINX_RESTORED=PASS
```

## Safety

```text
Backend/frontend source changed during execution: no
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
