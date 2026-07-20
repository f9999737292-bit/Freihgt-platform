# Post-Deployment Monitoring Evidence v0.1

## Summary

First read-only post-deployment monitoring baseline after **PRODUCTION_DEPLOYMENT_CLOSED**.

Production and staging external checks **PASS**. Server read-only checks **PASS**. No P0/P1 alert conditions triggered.

## Decision

```text
POST_DEPLOYMENT_MONITORING_BASELINE_PASS
```

## Pack Context

| Field | Value |
| --- | --- |
| Pack | POST_DEPLOYMENT_MONITORING_PACK v0.1 |
| Prior decision | PRODUCTION_DEPLOYMENT_CLOSED |
| Monitoring date | 2026-07-20 ~16:38 MSK |
| Branch | `main` |
| HEAD (at pack start) | `394c803` — `docs: close production deployment` |
| Write operations | **no** |

## Target Environment

| Field | Value |
| --- | --- |
| Server IP | 161.104.53.221 |
| Production domain | https://бинтранс.рф/ |
| Production punycode | https://xn--80abvubqje.xn--p1ai/ |
| Staging domain | https://staging.бинтранс.рф/ |
| Staging punycode | https://staging.xn--80abvubqje.xn--p1ai/ |
| Pilot tenant | `74519f22-ff9b-4a8b-8fff-a958c689682f` |

## Pre-flight

| Check | Result |
| --- | --- |
| Closure commit present | PASS — `394c803` |
| Staged files | none |
| Pack type | read-only monitoring |

## Production External Monitoring

| Check | Result |
| --- | --- |
| HTTPS root `/` | PASS — 200 text/html |
| HTTPS `/login` | PASS — 200 text/html |
| HTTPS `/health` | PASS — 200 |
| HTTP → HTTPS redirect | PASS — 301 |
| API proxy read-only (TRANSPORT_ORDER) | PASS — 200 |
| Active template TRANSPORT_ORDER | PASS — 200 |
| Active template SHIPMENT | PASS — 200 |
| Active template BILLING_REGISTER | PASS — 200 |
| Cyrillic HTTPS root | PASS — 200 text/html |

## Staging External Monitoring

| Check | Result |
| --- | --- |
| HTTPS root `/` | PASS — 200 text/html |
| HTTPS `/health` | PASS — 200 |
| API proxy read-only (TRANSPORT_ORDER) | PASS — 200 |
| Active template TRANSPORT_ORDER | PASS — 200 |
| Active template SHIPMENT | PASS — 200 |
| Active template BILLING_REGISTER | PASS — 200 |

## Server Read-only Monitoring

| Check | Result |
| --- | --- |
| `nginx -t` | PASS |
| Production site enabled | PASS — `/etc/nginx/sites-enabled/00-bintrans-production.conf` |
| Staging site enabled | PASS — `/etc/nginx/sites-enabled/staging-bintrans.conf` |
| `freight-staging` disabled | PASS |
| Production cert metadata | PASS — expires 2026-10-18 (89 days) |
| Staging cert metadata | PASS — expires 2026-10-15 (87 days) |
| certbot timer | active — next run 2026-07-21 00:00:54 MSK |
| Docker containers | PASS — 10/10 healthy/running |

## Alert Condition Review

Reference: `LOW_CODE_PILOT_WEEK3_PRODUCTION_MONITORING_ALERT_CONDITIONS_V0.1.md`

| Alert ID | Condition | Result |
| --- | --- | --- |
| MON-ALERT-001 | low-code-service unavailable | **not triggered** — docker 10/10 healthy |
| MON-ALERT-004 | runtime active templates unavailable | **not triggered** — prod/stg TO/SH/BR 200 |
| MON-ALERT-006 | repeated 5xx on low-code API | **not triggered** — all monitored GETs 200 |
| MON-ALERT-009 | secrets/JWT/tokens in logs/docs | **not triggered** — none captured |

Auth bypass / tenant isolation checks were **out of scope** for this baseline pack (read-only GET smoke only).

## P0/P1 Summary

```text
P0 triggered: no
P1 triggered: no
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
Backend/frontend source changed: no
Docker compose repo changed: no
UFW changed: no
DNS changed: no
Nginx changed: no
Certbot executed: no
Web-admin redeployed: no
Database writes executed: no
POST/PUT/PATCH/DELETE API calls executed: no
Secrets captured: no
Certificate private key captured: no
```

## Evidence Chain

```text
Closure note: docs/LOW_CODE_PILOT_WEEK3_PRODUCTION_DEPLOYMENT_CLOSURE_NOTE_V0.1.md
Retry v0.3 evidence: docs/LOW_CODE_PILOT_WEEK3_PRODUCTION_DEPLOYMENT_EXECUTION_RETRY_V03_EVIDENCE_V0.1.md
Monitoring policy: docs/LOW_CODE_PILOT_WEEK3_PRODUCTION_MONITORING_POLICY_V0.1.md
Monitoring cadence runbook: docs/LOW_CODE_PILOT_WEEK3_MONITORING_CADENCE_RUNBOOK_V0.1.md
```

## Next

```text
Continue event-based production/staging read-only monitoring per cadence runbook.
Do not run full monitoring packs without a trigger event.
Next optional pack: post-deployment monitoring cycle v0.2 (on schedule or stakeholder request).
```
