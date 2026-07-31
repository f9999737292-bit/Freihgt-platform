# Staging Backend DB Isolation Execution Runbook v0.1

## Summary

Runbook for executing staging backend/database isolation.

User provided explicit server/Nginx/Docker/DB execution approval.

## Scope

```text
Execution target: staging isolation only
Production backend/DB: unchanged
Production writes: forbidden
Demo credentials/seed data: not created in this pack
```

## Required Gates

```text
1. Git/source safety gate.
2. Production/staging endpoint baseline.
3. Server resource and port pre-check.
4. Nginx backup.
5. Staging compose render check.
6. Staging gateway local health check.
7. Nginx config test before reload.
8. Production endpoint verification after reload.
9. Staging isolation gate re-check.
```

## Target

| Area                   | Target             |
| ---------------------- | ------------------ |
| production API         | 127.0.0.1:8080     |
| staging API            | 127.0.0.1:18080    |
| staging Docker project | bintrans-staging   |
| staging DB             | separate Postgres  |
| staging Nginx change   | staging vhost only |

## Decision

```text
STAGING_BACKEND_DB_ISOLATION_EXECUTION_RUNBOOK_CREATED
```
