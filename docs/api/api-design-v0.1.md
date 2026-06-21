# API Design v0.1

> Placeholder — REST/gRPC API design for freight-platform.

## Status

Skeleton only. OpenAPI specs will live under `packages/openapi/specs/`.

## Conventions (draft)

| Topic | Rule |
|-------|------|
| Base path | `/api/v1` via `api-gateway` |
| Auth | Bearer JWT (identity-service) |
| Errors | RFC 7807 Problem Details |
| Pagination | `page`, `pageSize`, `total` |
| Idempotency | `Idempotency-Key` header for writes |

## Service endpoints (planned)

| Service | Port | Health |
|---------|------|--------|
| api-gateway | 8080 | `GET /health` |
| identity-service | 8081 | `GET /health` |
| company-service | 8082 | `GET /health` |
| localization-service | 8083 | `GET /health` |
| transport-order-service | 8084 | `GET /health` |
| shipment-service | 8085 | `GET /health` |
| rfx-service | 8086 | `GET /health` |
| document-service | 8087 | `GET /health` |
| billing-register-service | 8088 | `GET /health` |

## Next steps

- Publish OpenAPI per bounded context
- Define gateway routing table
- Add authentication flows
