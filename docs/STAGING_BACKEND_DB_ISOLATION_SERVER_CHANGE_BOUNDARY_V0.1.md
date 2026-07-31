# Staging Backend DB Isolation Server Change Boundary v0.1

## Summary

Server-side boundary for future staging isolation execution.

This document does not change the server.

Base commit: `570c3c4`.

## Decision

```text
STAGING_BACKEND_DB_ISOLATION_SERVER_CHANGE_BOUNDARY_APPROVED
```

## Approved Future Server Change Areas

| Area                    | Future Approval Boundary                 |
| ----------------------- | ---------------------------------------- |
| server-only staging env | allowed in execution pack, not committed |
| Docker project          | create/use bintrans-staging only         |
| Docker network          | create/use staging network only          |
| staging Postgres        | create/use staging container/volume only |
| staging API gateway     | bind to 127.0.0.1:18080 only             |
| staging Nginx vhost     | change staging proxy only                |
| backups                 | create backups before changes            |
| logs                    | collect non-secret evidence only         |

## Forbidden Future Server Change Areas

```text
Do not change production vhost.
Do not change production API proxy.
Do not stop production containers.
Do not restart production Docker stack.
Do not write to production DB.
Do not expose staging DB publicly.
Do not expose secrets.
```

## Current Pack Status

```text
Server changed in this pack: no
```
