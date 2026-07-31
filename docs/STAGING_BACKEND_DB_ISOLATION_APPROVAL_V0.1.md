# Staging Backend DB Isolation Approval v0.1

## Summary

Approval boundary for future staging backend and database isolation execution.

This pack is approval-only and docs-only. It does not change server, Nginx, Docker, DNS, Certbot, backend, API contracts, migrations, source code, database, credentials, users, seed data, ports, or secrets.

Base commit: `570c3c4` (`docs: plan staging backend db isolation`).

Approval date: 2026-07-31.

Read-only confirmation (2026-07-31, `161.104.53.221`):

- production API proxy: `http://127.0.0.1:8080`
- staging API proxy: `http://127.0.0.1:8080`
- shared gateway: `freight_api_gateway`
- shared DB: `freight_postgres`
- network: `freight-platform-network`
- postgres volume: `docker-compose_freight_postgres_data`

No env values or secrets were captured.

## Decision

```text
STAGING_BACKEND_DB_ISOLATION_APPROVAL_COMPLETE
STAGING_BACKEND_DB_ISOLATION_OPTION_A_APPROVED
SAME_VM_ISOLATED_STAGING_STACK_APPROVED_FOR_FUTURE_EXECUTION
STAGING_API_TARGET_127_0_0_1_18080_APPROVED
SEPARATE_STAGING_POSTGRES_APPROVED_FOR_FUTURE_EXECUTION
STAGING_NGINX_PROXY_CHANGE_APPROVED_FOR_FUTURE_EXECUTION_ONLY
PRODUCTION_BACKEND_DB_UNCHANGED_BOUNDARY_APPROVED
STAGING_CREDENTIALS_SEED_REMAIN_BLOCKED_UNTIL_REVERIFY
EXECUTION_NOT_PERFORMED_IN_THIS_PACK
```

## Approved Target Architecture

```text
Option A: same VM, isolated staging Docker stack.
```

## Current Blocker

| Area                  | Current               |
| --------------------- | --------------------- |
| production API proxy  | http://127.0.0.1:8080 |
| staging API proxy     | http://127.0.0.1:8080 |
| gateway               | shared                |
| Postgres              | shared                |
| production write risk | yes                   |

## Approved Future Target

| Area                   | Target                         |
| ---------------------- | ------------------------------ |
| production API proxy   | http://127.0.0.1:8080          |
| staging API proxy      | http://127.0.0.1:18080         |
| staging Docker project | bintrans-staging               |
| staging DB             | separate staging Postgres      |
| staging DB volume      | bintrans_staging_postgres_data |
| staging env            | server-only, not committed     |
| production backend/DB  | unchanged                      |

## Approved Future Execution Boundary

```text
Future execution may create an isolated staging backend/DB stack on the same VM and update only the staging Nginx proxy to use 127.0.0.1:18080, after backup and checks.
```

## Not Executed In This Pack

```text
No server changes.
No Nginx edits.
No Nginx reload.
No Docker changes.
No DB/volume creation.
No migrations.
No source changes.
No credentials.
No seed data.
No staging writes.
No production writes.
```

## Required Next Pack

```text
STAGING_BACKEND_DB_ISOLATION_EXECUTION_PACK v0.1
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
Docker project created: no
DB/volume created: no
Backend changed: no
API contracts changed: no
Migrations changed: no
Database writes executed: no
Source code changed: no
Ports opened: no
Secrets captured: no
Credentials created: no
Seed data created: no
Approval scope: future staging backend/DB isolation execution only
```

See also:

- `docs/STAGING_BACKEND_DB_ISOLATION_OPTION_A_DECISION_V0.1.md`
- `docs/STAGING_BACKEND_DB_ISOLATION_EXECUTION_BOUNDARY_V0.1.md`
- `docs/STAGING_BACKEND_DB_ISOLATION_SERVER_CHANGE_BOUNDARY_V0.1.md`
- `docs/STAGING_BACKEND_DB_ISOLATION_PORT_ENV_POLICY_V0.1.md`
- `docs/STAGING_BACKEND_DB_ISOLATION_STOP_CONDITIONS_V0.1.md`
