# System Architecture

High-level map of the freight platform monorepo. Business logic is not implemented yet — only production-ready skeletons.

## Monorepo layout

```
apps/          → Nuxt 3 frontends (one per business role)
services/      → Go microservices (HTTP + health endpoints)
packages/      → shared-go, shared-ts, openapi, proto
infrastructure/→ PostgreSQL, migrations
docs/          → architecture, database, API, events
```

## Service map

| Layer | Component | Port | Responsibility (planned) |
|-------|-----------|------|--------------------------|
| Edge | api-gateway | 8080 | Routing, auth termination |
| Core | identity-service | 8081 | Auth, users, sessions |
| Core | company-service | 8082 | Companies, memberships |
| Core | localization-service | 8083 | i18n, translations |
| Domain | transport-order-service | 8084 | Transport orders |
| Domain | shipment-service | 8085 | Shipments, execution |
| Domain | rfx-service | 8086 | RFx, bids, procurement |
| Domain | document-service | 8087 | Documents, signatures |
| Domain | billing-register-service | 8088 | Billing, invoices |
| UI | web-admin … web-procurement | 3000–3005 | Role-specific UIs |

## Data architecture

- **Local dev**: single PostgreSQL instance, schemas `core`, `transport`, `rfx`, `documents`, `billing`
- **Production (target)**: each service owns its database; no cross-service table access

See `docs/database/` and `infrastructure/migrations/` for the current schema.

## Cross-cutting (skeleton today)

| Concern | Implementation |
|---------|----------------|
| Config | env vars via `packages/shared-go/config` |
| Logging | structured JSON via `packages/shared-go/logger` |
| Health | `/health`, `/healthz`, `/ready` on every Go service |
| Shutdown | graceful HTTP shutdown in `cmd/server/main.go` |

## Integration patterns (planned)

- HTTP/REST between gateway and services
- gRPC definitions in `packages/proto`
- OpenAPI specs in `packages/openapi`
- Async events documented in `docs/events`
