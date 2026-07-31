# Staging Backend DB Isolation Plan v0.1

## Summary

Plan to unblock live-data demo staging execution by isolating staging backend and staging database from production.

This pack is plan-only and docs-only. It does not change server, Nginx, Docker, DNS, Certbot, backend, API contracts, migrations, source code, database, credentials, users, or seed data.

Base commit: `5f5ad4c` (`docs: record blocked staging demo credentials execution`).

Plan date: 2026-07-31.

## Background

Staging demo credentials and seed data execution was blocked because staging and production currently share API gateway and Postgres data scope.

Read-only inspection (2026-07-31, `161.104.53.221`) confirmed:

- production UI root: `/var/www/bintrans-web-admin`
- staging UI root: `/var/www/staging-bintrans-web-admin`
- production and staging Nginx `/api` and `/health` both proxy to `http://127.0.0.1:8080`
- single API gateway container: `freight_api_gateway` on `0.0.0.0:8080`
- single Postgres container: `freight_postgres` on `0.0.0.0:5432`
- Docker network: `freight-platform-network`
- Postgres volume: `docker-compose_freight_postgres_data`

No env values or secrets were captured during inspection.

## Decision

```text
STAGING_BACKEND_DB_ISOLATION_PLAN_COMPLETE
TARGET_ARCHITECTURE_OPTION_A_SINGLE_VM_ISOLATED_STAGING_STACK
STAGING_WRITES_REMAIN_BLOCKED_UNTIL_ISOLATION_EXECUTION
PRODUCTION_WRITES_NOT_APPROVED
```

## Current Blocker

| Area                    | Current State                       |
| ----------------------- | ----------------------------------- |
| production UI root      | /var/www/bintrans-web-admin         |
| staging UI root         | /var/www/staging-bintrans-web-admin |
| production API proxy    | http://127.0.0.1:8080               |
| staging API proxy       | http://127.0.0.1:8080               |
| production health proxy | http://127.0.0.1:8080/health        |
| staging health proxy    | http://127.0.0.1:8080/health        |
| backend                 | shared (`freight_api_gateway`)      |
| DB                      | shared (`freight_postgres`)         |
| production write risk   | yes                                 |

## Target Architecture

```text
Production stays on the existing backend and DB.
Staging gets separate backend containers, separate DB container, separate DB volume, separate Docker network, separate env file, and separate localhost API gateway port.
```

## Target Production Boundary

| Area                    | Target                       |
| ----------------------- | ---------------------------- |
| production UI root      | /var/www/bintrans-web-admin  |
| production API proxy    | http://127.0.0.1:8080        |
| production health proxy | http://127.0.0.1:8080/health |
| production DB           | existing production DB       |
| production writes       | not approved                 |

## Target Staging Boundary

| Area                   | Target                                    |
| ---------------------- | ----------------------------------------- |
| staging UI root        | /var/www/staging-bintrans-web-admin       |
| staging API proxy      | http://127.0.0.1:18080                    |
| staging health proxy   | http://127.0.0.1:18080/health             |
| staging Docker project | bintrans-staging                          |
| staging DB             | separate staging Postgres                 |
| staging DB volume      | bintrans_staging_postgres_data            |
| staging network        | bintrans-staging-network                  |
| staging env            | server-only `.env.staging`, not committed |

## Architecture Options

### Option A — Recommended now: same VM, isolated staging stack

```text
Production:
- UI root: /var/www/bintrans-web-admin
- Nginx production /api and /health proxy: http://127.0.0.1:8080
- Docker project: existing production project
- API gateway: existing production gateway
- DB: existing production Postgres
- No changes in plan pack

Staging:
- UI root: /var/www/staging-bintrans-web-admin
- Nginx staging /api and /health proxy target: http://127.0.0.1:18080
- Docker project name: bintrans-staging
- API gateway external localhost port: 18080
- Internal service ports: isolated by Docker network
- DB container: bintrans-staging-postgres
- DB volume: bintrans_staging_postgres_data
- DB name/user: staging-specific, secret values not stored in docs
- Env file: server-only .env.staging, not committed
- Network: bintrans-staging-network
```

### Option B — Long-term: separate staging VM

```text
Best isolation:
- separate Selectel VM
- separate DB
- separate network/security group
- separate Nginx/cert
- separate backups
- higher cost, lower production risk
```

### Rejected: shared backend with tenant-only separation

```text
REJECTED_OPTION_SHARED_BACKEND_WITH_TENANT_ONLY

Reason:
- tenant separation is not enough for staging writes;
- staging mistakes could still affect production DB;
- blocked execution already proved shared API/DB risk.
```

## Target Port Map

```text
Production current:
- api-gateway: 127.0.0.1:8080
- postgres: internal/current only
- production Nginx /api -> 127.0.0.1:8080
- production Nginx /health -> 127.0.0.1:8080/health

Staging target:
- staging api-gateway: 127.0.0.1:18080
- staging identity-service: internal only or 18081 if needed
- staging company-service: internal only or 18082 if needed
- staging transport-order-service: internal only or 18083 if needed
- staging rfx-service: internal only or 18084 if needed
- staging shipment-service: internal only or 18085 if needed
- staging document-service: internal only or 18086 if needed
- staging billing-register-service: internal only or 18087 if needed
- staging low-code-service: internal only or 18088 if needed
- staging postgres: internal only, no public bind
- staging Nginx /api -> 127.0.0.1:18080
- staging Nginx /health -> 127.0.0.1:18080/health
```

## Future Execution Phases

```text
1. Backup current server configs and repo state.
2. Create server-only staging env file without committing secrets.
3. Create isolated staging compose/override or execution config.
4. Start staging stack under separate Docker project name.
5. Verify staging API on 127.0.0.1:18080.
6. Run migrations against staging DB only.
7. Update staging Nginx proxy to 127.0.0.1:18080.
8. Reload Nginx only after config test.
9. Verify production endpoints unchanged.
10. Verify staging endpoints use isolated backend.
11. Re-run staging isolation gate.
12. Only then allow demo credentials/seed data staging execution.
```

## Rejected Option

```text
Shared production backend/DB with tenant-only separation is rejected for staging writes.
```

Reason: staging mistakes could still write into production data scope.

## Future Execution Required

```text
STAGING_BACKEND_DB_ISOLATION_EXECUTION_PACK v0.1
```

## Not Approved In This Plan

```text
No server changes.
No Nginx changes/reload.
No Docker restart.
No DB create/drop.
No migrations.
No credentials.
No seed data.
No staging writes.
No production writes.
No source changes.
```

## Safety Result

```text
Production changed in this pack: no
Production deploy executed: no
Staging deploy executed: no
Server changed: no
Nginx changed: no
Nginx reload executed: no
DNS changed: no
Certbot changed: no
Docker restarted: no
Backend changed: no
API contracts changed: no
Migrations changed: no
Database writes executed: no
Source code changed: no
Ports opened: no
Secrets captured: no
Credentials created: no
Seed data created: no
Planning scope: staging backend/DB isolation only
```

## Next Recommended Pack

```text
STAGING_BACKEND_DB_ISOLATION_APPROVAL_PACK v0.1
```

See also:

- `docs/STAGING_BACKEND_DB_ISOLATION_ARCHITECTURE_V0.1.md`
- `docs/STAGING_BACKEND_DB_ISOLATION_NGINX_BOUNDARY_V0.1.md`
- `docs/STAGING_BACKEND_DB_ISOLATION_DATA_SAFETY_V0.1.md`
- `docs/STAGING_BACKEND_DB_ISOLATION_EXECUTION_APPROVAL_CHECKLIST_V0.1.md`
- `docs/STAGING_BACKEND_DB_ISOLATION_ROLLBACK_PLAN_V0.1.md`
- `docs/STAGING_BACKEND_DB_ISOLATION_RISK_MATRIX_V0.1.md`
