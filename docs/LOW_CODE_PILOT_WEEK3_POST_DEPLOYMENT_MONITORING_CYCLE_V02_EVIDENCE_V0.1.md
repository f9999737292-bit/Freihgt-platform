# Post-deployment Monitoring Cycle v0.2 Evidence v0.1

## Summary

Post-deployment monitoring cycle v0.2 completed as an optional one-week/no-change read-only check on 2026-07-26.

Production and staging external checks **PASS**. Server read-only checks **PASS**. No P0/P1 alert conditions triggered.

## Decision

```text
POST_DEPLOYMENT_MONITORING_CYCLE_V02_PASS
```

## Pack Context

| Field | Value |
| --- | --- |
| Pack | POST_DEPLOYMENT_MONITORING_CYCLE v0.2 |
| Reason | optional one-week/no-change post-deployment monitoring cycle |
| Previous baseline | POST_DEPLOYMENT_MONITORING_BASELINE_PASS |
| Baseline commit | 27c92bd |
| Monitoring date | 2026-07-26 ~21:58 MSK |
| Branch | `main` |
| HEAD (at pack start) | `27c92bd` — `docs: record post-deployment monitoring baseline v0.1` |
| Write operations | **no** |

## Context

| Item | Value |
| --- | --- |
| Production deployment | CLOSED |
| Production domain | https://бинтранс.рф/ |
| Production punycode | https://xn--80abvubqje.xn--p1ai/ |
| Staging domain | https://staging.бинтранс.рф/ |
| Staging punycode | https://staging.xn--80abvubqje.xn--p1ai/ |
| Server IP | 161.104.53.221 |
| Pilot tenant | `74519f22-ff9b-4a8b-8fff-a958c689682f` |

## Pre-flight

| Check | Result |
| --- | --- |
| Baseline commit present | PASS — `27c92bd` |
| Staged files | none |
| Pack type | read-only monitoring |

## External Verification

| Check | Result |
| --- | --- |
| Production DNS (1.1.1.1) | PASS — 161.104.53.221 |
| Production DNS (8.8.8.8) | PASS — 161.104.53.221 |
| Staging DNS (1.1.1.1) | PASS — 161.104.53.221 |
| Staging DNS (8.8.8.8) | PASS — 161.104.53.221 |
| Production HTTPS root `/` | PASS — 200 text/html |
| Production HTTPS `/login` | PASS — 200 text/html |
| Production HTTPS `/health` | PASS — 200 |
| Production HTTP → HTTPS redirect | PASS — 301 |
| Production API active template TRANSPORT_ORDER | PASS — 200 |
| Production API active template SHIPMENT | PASS — 200 |
| Production API active template BILLING_REGISTER | PASS — 200 |
| Production Cyrillic HTTPS root | PASS — 200 text/html |
| Staging HTTPS root `/` | PASS — 200 text/html |
| Staging HTTPS `/login` | PASS — 200 text/html |
| Staging HTTPS `/health` | PASS — 200 |
| Staging API active template TRANSPORT_ORDER | PASS — 200 |
| Staging API active template SHIPMENT | PASS — 200 |
| Staging API active template BILLING_REGISTER | PASS — 200 |

## Server Read-only Verification

| Check | Result |
| --- | --- |
| `nginx -t` | PASS |
| Production vhost enabled | PASS — `/etc/nginx/sites-enabled/00-bintrans-production.conf` |
| Staging vhost enabled | PASS — `/etc/nginx/sites-enabled/staging-bintrans.conf` |
| `freight-staging` disabled | PASS |
| Production cert exists | PASS — expires 2026-10-18 (83 days) |
| Staging cert exists | PASS — expires 2026-10-15 (80 days) |
| certbot timer | active — next run 2026-07-27 07:51:46 MSK |
| Docker containers | PASS — 10/10 healthy/running |
| Server local production root/login/health | PASS — 200 |
| Server local staging root/login/health | PASS — 200 |

## Alert Condition Review

Reference: `LOW_CODE_PILOT_WEEK3_PRODUCTION_MONITORING_ALERT_CONDITIONS_V0.1.md`

| Alert ID | Condition | Result |
| --- | --- | --- |
| MON-ALERT-001 | low-code-service unavailable | **not triggered** — docker 10/10 healthy |
| MON-ALERT-004 | runtime active templates unavailable | **not triggered** — prod/stg TO/SH/BR 200 |
| MON-ALERT-006 | repeated 5xx on low-code API | **not triggered** — all monitored GETs 200 |
| MON-ALERT-009 | secrets/JWT/tokens in logs/docs | **not triggered** — none captured |

## P0/P1 Alert Review

```text
P0 alerts: none
P1 alerts: none
Escalation required: no
```

## Backup / Rollback Reference

| Item | Value |
| --- | --- |
| Selectel backup | 6450ba4f-5e95-4052-a0fc-dea853399dad |
| Server Nginx backup | /root/prod-deploy-retry-v03-final-backup-20260720_162412 |
| Rollback allowed | yes |
| Rollback triggered during monitoring | no |

## Safety

```text
Backend/frontend source changed during monitoring cycle: no
Docker compose repo changed: no
UFW changed: no
DNS changed: no
Nginx changed: no
Certbot executed: no
Web-admin redeployed: no
Production deploy executed: no
Database writes executed: no
POST/PUT/PATCH/DELETE API calls executed: no
Secrets captured: no
Certificate private key captured: no
```

## Evidence Chain

```text
Baseline evidence: docs/LOW_CODE_PILOT_WEEK3_POST_DEPLOYMENT_MONITORING_EVIDENCE_V0.1.md
Baseline note: docs/LOW_CODE_PILOT_WEEK3_POST_DEPLOYMENT_MONITORING_NOTE_V0.1.md
Closure note: docs/LOW_CODE_PILOT_WEEK3_PRODUCTION_DEPLOYMENT_CLOSURE_NOTE_V0.1.md
Monitoring cadence runbook: docs/LOW_CODE_PILOT_WEEK3_MONITORING_CADENCE_RUNBOOK_V0.1.md
```

## Next

```text
Continue event-based production/staging read-only monitoring per cadence runbook.
Do not run daily packs without incident/change trigger.
Future monitoring cycles may be run by request or if P0/P1 triggers appear.
```
