# Staging Backend DB Isolation Option A Decision v0.1

## Summary

Decision record for selecting Option A: same VM, isolated staging Docker stack.

Base commit: `570c3c4`.

## Decision

```text
STAGING_BACKEND_DB_ISOLATION_OPTION_A_APPROVED
```

## Selected Option

```text
Option A: run a separate staging backend and staging database stack on the existing Selectel VM.
```

## Rationale

```text
Option A is faster and lower cost than a separate staging VM, while still separating staging API, services, DB, volume, network, and Nginx proxy target from production.
```

## Option A Requirements

| Layer          | Requirement                            |
| -------------- | -------------------------------------- |
| Docker project | separate project: bintrans-staging     |
| API gateway    | separate staging gateway               |
| API port       | 127.0.0.1:18080                        |
| DB             | separate staging Postgres              |
| DB volume      | bintrans_staging_postgres_data         |
| network        | separate staging Docker network        |
| env            | server-only staging env, not committed |
| Nginx          | staging vhost proxy only               |
| production     | unchanged                              |

## Option B Deferred

```text
Separate staging VM remains the stronger long-term isolation option and may be used later for external pilots.
```

## Rejected Option

```text
Shared production backend/DB with tenant-only staging separation remains rejected.
```

## Execution Status

```text
Approved for future execution boundary only.
Not executed in this pack.
```
