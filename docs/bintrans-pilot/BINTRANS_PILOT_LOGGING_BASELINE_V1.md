# BINTRANS Pilot Logging Baseline v1

**Wave:** P0.1 — documentation only (no new centralized logging stack)

## Current state

| Field | Value |
|---|---|
| APPLICATION_LOG_RETENTION | Container stdout via Docker json-file driver |
| CONTAINER_LOG_ROTATION | Docker default (no explicit max-size configured in compose) |
| AUDIT_LOG_RETENTION | Not centralized; per-container logs on host |
| BACKUP_LOG_RETENTION | Operator-retained backup script output only |

## Disk risk

- Host root filesystem monitored via `BintransPilotDiskPressure` alert (node-exporter, <15% free for 10m)
- Current staging headroom: adequate (~76G free at P0 probe)

## Recommended pilot policy (PROPOSED — not approved)

| Log type | Recommended retention | Notes |
|---|---|---|
| Application container logs | 7 days or 500MB per container | Requires Docker log opts or host rotation policy |
| Incident evidence | Sanitized notes in pilot incident log | No secrets; Telegram not sole audit log |
| Backup operator logs | 30 days | Under `/protected/bintrans/backups/` metadata |

## Required monitoring

- Disk pressure alert active when observability stack deployed
- Manual weekly check of `docker system df` during pilot

```text
LOG_RETENTION_DOCUMENTED=YES
LOG_ROTATION_ACTIVE=DOCKER_DEFAULT
```
