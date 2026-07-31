# Staging Backend DB Isolation Execution Evidence v0.1

## Summary

Evidence for staging backend/database isolation execution.

## Result

```text
STAGING_BACKEND_DB_ISOLATION_EXECUTION_COMPLETE
```

## Pre-flight

| Check                       | Result     |
| --------------------------- | ---------- |
| branch                      | main       |
| HEAD                        | c521ed2    |
| HEAD = origin/main          | yes        |
| staged files before pack    | none       |
| source diff                 | empty      |
| explicit execution approval | given      |

## Endpoint Baseline

| Endpoint                     | Before              | After               |
| ---------------------------- | ------------------- | ------------------- |
| production /                 | 200 text/html       | 200 text/html       |
| production /login            | 200 text/html       | 200 text/html       |
| production /health           | 200 application/json | 200 application/json |
| production /api/v1/companies | 400 application/json | 400 application/json |
| production /api/health       | 404 application/json | 404 application/json |
| staging /                    | 200 text/html       | 200 text/html       |
| staging /login               | 200 text/html       | 200 text/html       |
| staging /health              | 200 application/json | 200 application/json |
| staging /api/v1/companies    | n/a                 | 400 application/json |

## Server Evidence

| Item                            | Result                  |
| ------------------------------- | ----------------------- |
| backup dir                      | /root/staging-backend-db-isolation-backup-20260731_125924 |
| port 18080 before               | available               |
| port 18080 after                | listening               |
| staging Docker project          | bintrans-staging        |
| staging API gateway             | bintrans-staging-api-gateway |
| staging DB container            | bintrans-staging-postgres |
| staging DB volume               | bintrans_staging_postgres_data |
| staging DB public exposure      | no                      |
| production compose status after | healthy                 |
| Nginx test                      | pass                    |
| Nginx reload                    | executed                |
| rollback executed               | no                      |

## Isolation Gate

| Check                                            | Result |
| ------------------------------------------------ | ------ |
| production API proxy                             | 127.0.0.1:8080 |
| staging API proxy                                | 127.0.0.1:18080 |
| production gateway                               | freight_api_gateway running |
| staging gateway                                  | bintrans-staging-api-gateway running |
| production DB/volume                             | freight_postgres / docker-compose_freight_postgres_data |
| staging DB/volume                                | bintrans-staging-postgres / bintrans_staging_postgres_data |
| separate staging backend proven                  | yes    |
| separate staging DB proven                       | yes    |
| production write risk removed for staging writes | yes    |

## Safety Result

```text
Production changed in this pack: no
Production deploy executed: no
Production writes executed: no
Production backend changed: no
Production DB changed: no
Staging backend stack created: yes
Staging DB created: yes
Staging Nginx proxy changed: yes
Nginx reload executed: yes
Docker project created: yes
Migrations executed against staging only: yes
Demo credentials created: no
Seed data created: no
Secrets captured: no
Source code changed: no
API contracts changed: no
Ports opened publicly: no
```
