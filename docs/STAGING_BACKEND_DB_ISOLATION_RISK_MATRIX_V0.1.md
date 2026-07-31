# Staging Backend DB Isolation Risk Matrix v0.1

## Summary

Risk matrix for staging backend/database isolation.

Base commit: `5f5ad4c`.

## Decision

```text
STAGING_BACKEND_DB_ISOLATION_RISK_MATRIX_CREATED
```

## Risks

| Risk | Severity | Mitigation |
| --- | --- | --- |
| staging still writes to production | high | separate backend/DB/proxy and re-run isolation gate |
| production Nginx accidentally changed | high | edit staging vhost only, backup, nginx -t |
| port conflict on 18080 | medium | check listening ports before execution |
| staging env secret leak | high | server-only .env.staging, no docs/repo/chat |
| staging migrations hit production DB | high | verify DB target before migration |
| production outage from Docker action | high | separate project name; do not run compose down on production |
| disk pressure from second stack | medium | check disk before execution |
| CPU/RAM pressure | medium | check resources before execution |
| external notifications from staging | high | disable notification integrations |
| overclaiming readiness | medium | keep authenticated workflow not signed off until smoke |

## Required Future Pre-checks

```text
1. disk space
2. memory/CPU headroom
3. port availability
4. Docker project separation
5. DB target verification
6. Nginx config backup
7. endpoint baseline
```
