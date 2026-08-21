# freight-cost-service

Freight Cost Foundation v2.1A — stateless planned-cost read orchestration service.

## Overview

- **Port:** `8092` (`FREIGHT_COST_SERVICE_PORT`, fallback `HTTP_PORT`)
- **Module:** `github.com/freight-platform/freight-cost-service`
- **Persistence:** none (stateless; no database)
- **Internal API:** `GET /internal/v1/freight-cost/transport-orders/{transportOrderId}`

## Configuration

| Variable | Required | Default |
|----------|----------|---------|
| `FREIGHT_COST_SERVICE_PORT` | no | `8092` |
| `ENVIRONMENT` | no | `development` |
| `LOG_LEVEL` | no | `info` |
| `INTERNAL_SERVICE_TOKEN` | yes (non-dev) | — |
| `TRANSPORT_ORDER_SERVICE_URL` | yes | `http://transport-order-service:8083` |

## Internal authentication

All `/internal/v1/freight-cost/*` routes require:

- `X-Internal-Service-Token` — shared S2S token (`INTERNAL_SERVICE_TOKEN`)
- `X-Tenant-ID`, `X-User-ID`, `X-Company-ID` — trusted actor context (validated after S2S)
- `X-Actor-Kind` — `BUYER` or `CARRIER` only (`PLATFORM_ADMIN` denied)

The S2S token authenticates caller class only; actor headers are trusted forwarded context after the token gate succeeds.

## v2.1A behavior

- Returns `data_stage=PLANNED_ONLY` and `financial_finality=NOT_EVALUATED`
- Loads planned cost from transport-order internal rate snapshot
- Reserved monetary fields are always present in JSON (`null` when unknown; never `omitempty`)
- Cross-tenant transport order lookup → `404 NOT_FOUND`
- Same-tenant wrong company → `403 FORBIDDEN`
- Unpriced transport order → `409 CONFLICT`

## Health and metrics

- `/health`, `/ready`, `/metrics` via `shared-go/observability` (no DB ping)
- Domain metrics: `freight_cost_http_requests_total`, `freight_cost_source_requests_total`, `freight_cost_source_errors_total`, `freight_cost_currency_mismatch_total`

## Run locally

```bash
cd services/freight-cost-service
export INTERNAL_SERVICE_TOKEN=dev-token
export TRANSPORT_ORDER_SERVICE_URL=http://localhost:8083
go run ./cmd/server
```

## Tests

```bash
cd services/freight-cost-service
go test ./...
```

Test IDs follow the v2.1A contract: `FC-A-DOM-*`, `FC-A-SEC-*`, `FC-A-SRC-*`, `FC-A-API-*`, `FC-A-E2E-*`.
