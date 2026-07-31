# Staging Backend DB Isolation Architecture v0.1

## Summary

Target architecture for separating staging backend and database from production.

Base commit: `5f5ad4c`.

## Decision

```text
STAGING_BACKEND_DB_ISOLATION_ARCHITECTURE_CREATED
```

## Current Architecture

```text
Production UI root -> production Nginx -> 127.0.0.1:8080 -> freight_api_gateway -> shared services -> freight_postgres
Staging UI root -> staging Nginx -> 127.0.0.1:8080 -> freight_api_gateway -> shared services -> freight_postgres
```

Read-only server evidence (2026-07-31):

| Component | Current Name / Binding |
| --------- | ---------------------- |
| API gateway | `freight_api_gateway` — `0.0.0.0:8080` |
| Postgres | `freight_postgres` — `0.0.0.0:5432` |
| Identity | `freight_identity_service` — `8081` |
| Company | `freight_company_service` — `8082` |
| Transport order | `freight_transport_order_service` — `8083` |
| RFx | `freight_rfx_service` — `8084` |
| Shipment | `freight_shipment_service` — `8085` |
| Document | `freight_document_service` — `8086` |
| Billing register | `freight_billing_register_service` — `8087` |
| Low-code | `freight_low_code_service` — `8088` |
| Network | `freight-platform-network` |
| Postgres volume | `docker-compose_freight_postgres_data` |

Local compose reference: `infrastructure/docker-compose/docker-compose.yml` (not modified in this pack).

## Target Architecture

```text
Production UI root -> production Nginx -> 127.0.0.1:8080 -> production api-gateway -> production services -> production Postgres

Staging UI root -> staging Nginx -> 127.0.0.1:18080 -> staging api-gateway -> staging services -> staging Postgres
```

## Isolation Requirements

| Layer       | Requirement                                             |
| ----------- | ------------------------------------------------------- |
| Nginx       | production and staging proxy to different backend ports |
| API Gateway | separate staging gateway process/container              |
| Services    | separate staging service containers                     |
| Database    | separate staging Postgres container and volume          |
| Env         | separate server-only staging env values                 |
| Network     | separate Docker project/network                         |
| Logs        | separate staging logs/container names                   |
| Backups     | backup before change; staging DB backup policy separate |
| Secrets     | no secrets in repo/docs/chat                            |

## Proposed Names

| Resource            | Proposed Name                               |
| ------------------- | ------------------------------------------- |
| Docker project      | bintrans-staging                            |
| Network             | bintrans-staging-network                    |
| Postgres container  | bintrans-staging-postgres                   |
| Postgres volume     | bintrans_staging_postgres_data              |
| API gateway port    | 127.0.0.1:18080                             |
| Staging env file    | server-only `.env.staging` (not committed)  |
| Staging Nginx proxy | http://127.0.0.1:18080                      |

## Long-term Alternative

```text
Separate staging VM is the strongest isolation model and should be considered before external pilots.
```
