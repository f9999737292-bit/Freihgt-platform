# api-gateway

Single entry point for the Freight Platform HTTP API. The gateway accepts requests on port **8080**, applies cross-cutting middleware, and reverse-proxies traffic to backend microservices.

## Purpose

Clients (frontend, integrations, smoke tests) should call the API Gateway instead of individual service ports:

```
Client → api-gateway:8080/api/v1/* → microservice:808x/v1/*
```

Features:

- Reverse proxy with `/api` prefix stripping
- Health and readiness aggregation
- Route map discovery
- Request ID propagation
- Structured access logs
- CORS for local frontends
- Optional JWT auth (`AUTH_ENABLED`)

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `API_GATEWAY_PORT` | `8080` | HTTP listen port |
| `LOG_LEVEL` | `info` | Log level |
| `ENVIRONMENT` | `development` | Runtime environment |
| `IDENTITY_SERVICE_URL` | `http://localhost:8081` | identity-service |
| `COMPANY_SERVICE_URL` | `http://localhost:8082` | company-service |
| `TRANSPORT_ORDER_SERVICE_URL` | `http://localhost:8083` | transport-order-service |
| `RFX_SERVICE_URL` | `http://localhost:8084` | rfx-service |
| `SHIPMENT_SERVICE_URL` | `http://localhost:8085` | shipment-service |
| `DOCUMENT_SERVICE_URL` | `http://localhost:8086` | document-service |
| `BILLING_REGISTER_SERVICE_URL` | `http://localhost:8087` | billing-register-service |
| `AUTH_ENABLED` | `false` | Enable JWT protection |
| `JWT_SECRET` | `dev_secret_change_me` | JWT HMAC secret (must match identity-service) |
| `CORS_ALLOWED_ORIGINS` | localhost:3000,3001,5173 | Comma-separated allowed origins |
| `PROXY_TIMEOUT_SECONDS` | `30` | Downstream proxy timeout |
| `READY_CHECK_TIMEOUT_MS` | `2000` | Per-service readiness timeout |
| `RATE_LIMIT_ENABLED` | `true` | Enable IP rate limit on `/api/v1/*` |
| `RATE_LIMIT_RPS` | `50` | Sustained requests per second per IP |
| `RATE_LIMIT_BURST` | `100` | Burst size per IP |
| `MAX_REQUEST_BODY_BYTES` | `10485760` | Max request body (10 MB) |
| `PPROF_ENABLED` | `false` | Enable `/debug/pprof/*` (dev only) |

## Gateway endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Gateway health |
| GET | `/ready` | Aggregated downstream health |
| GET | `/routes` | Route map |
| `*` | `/api/v1/*` | Proxied API routes |

## Route map

| Gateway prefix | Service | Downstream target |
|----------------|---------|-----------------|
| `/api/v1/auth/*` | identity-service | `/v1/auth/*` |
| `/api/v1/users/*` | identity-service | `/v1/users/*` |
| `/api/v1/roles/*` | identity-service | `/v1/roles/*` |
| `/api/v1/companies/*` | company-service | `/v1/companies/*` |
| `/api/v1/locations/*` | transport-order-service | `/v1/locations/*` |
| `/api/v1/cargoes/*` | transport-order-service | `/v1/cargoes/*` |
| `/api/v1/transport-orders/*` | transport-order-service | `/v1/transport-orders/*` |
| `/api/v1/rfx-events/*` | rfx-service | `/v1/rfx-events/*` |
| `/api/v1/rfx-lots/*` | rfx-service | `/v1/rfx-lots/*` |
| `/api/v1/rfx-responses/*` | rfx-service | `/v1/rfx-responses/*` |
| `/api/v1/freight-requests/*` | rfx-service | `/v1/freight-requests/*` |
| `/api/v1/bids/*` | rfx-service | `/v1/bids/*` |
| `/api/v1/shipments/*` | shipment-service | `/v1/shipments/*` |
| `/api/v1/drivers/*` | shipment-service | `/v1/drivers/*` |
| `/api/v1/vehicles/*` | shipment-service | `/v1/vehicles/*` |
| `/api/v1/documents/*` | document-service | `/v1/documents/*` |
| `/api/v1/signing-sessions/*` | document-service | `/v1/signing-sessions/*` |
| `/api/v1/billing-registers/*` | billing-register-service | `/v1/billing-registers/*` |

Path rewrite example:

```
GET http://localhost:8080/api/v1/companies
→ GET http://localhost:8082/v1/companies
```

## Run locally

Start all backend services, then:

```bash
cp .env.example .env
make run-api-gateway
```

Or from monorepo root:

```bash
make run-api-gateway
```

## Examples

Health:

```bash
curl http://localhost:8080/health
```

Readiness (requires all downstream services):

```bash
curl http://localhost:8080/ready
```

Route map:

```bash
curl http://localhost:8080/routes
```

Login via gateway:

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "YOUR_TENANT_ID",
    "email": "user@example.com",
    "password": "StrongPassword123!"
  }'
```

List companies via gateway:

```bash
curl "http://localhost:8080/api/v1/companies?tenant_id=YOUR_TENANT_ID"
```

List transport orders via gateway:

```bash
curl "http://localhost:8080/api/v1/transport-orders?tenant_id=YOUR_TENANT_ID"
```

## Auth

When `AUTH_ENABLED=true`:

- Public routes: `GET /health`, `GET /ready`, `GET /routes`, `POST /api/v1/auth/login`, `POST /api/v1/users`
- Other routes require `Authorization: Bearer {token}`
- Valid tokens forward `X-User-ID`, `X-Tenant-ID`, `X-User-Email` to downstream services

## Tests

```bash
make test-api-gateway
```

## Docker

Build from monorepo root:

```bash
docker build -f services/api-gateway/Dockerfile -t freight-platform/api-gateway .
```

Not included in docker-compose yet.

## Timeouts

- Server read timeout: 10s
- Server write timeout: 30s
- Downstream proxy timeout: 30s
- Ready check per service: 2s
