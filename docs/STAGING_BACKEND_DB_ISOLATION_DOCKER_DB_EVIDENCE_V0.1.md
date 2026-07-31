# Staging Backend DB Isolation Docker DB Evidence v0.1

## Summary

Evidence for isolated staging Docker project and staging database.

## Result

```text
STAGING_DOCKER_DB_ISOLATION_ACTIVE
```

## Docker Evidence

| Item                         | Result                |
| ---------------------------- | --------------------- |
| production project           | docker-compose (default) |
| staging project              | bintrans-staging        |
| production gateway           | freight_api_gateway (0.0.0.0:8080) |
| staging gateway              | bintrans-staging-api-gateway (127.0.0.1:18080) |
| production Postgres          | freight_postgres        |
| staging Postgres             | bintrans-staging-postgres |
| production volume            | docker-compose_freight_postgres_data |
| staging volume               | bintrans_staging_postgres_data |
| staging API port             | 127.0.0.1:18080         |
| staging Postgres public bind | no                      |

## Server-only artifacts (not committed)

```text
/opt/bintrans/freight-platform/.env.staging
/opt/bintrans/freight-platform/infrastructure/docker-compose/docker-compose.staging.yml
```

## Safety

```text
Production containers stopped/restarted: no
Production DB changed: no
Production volume changed: no
Staging DB separate: yes
Secrets captured: no
```
