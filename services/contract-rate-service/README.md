# contract-rate-service

Internal S2S service for Freight Contract & Rate Management v2.0A.

- Contract lifecycle (DRAFT/ACTIVE/SUSPENDED/TERMINATED/EXPIRED/CANCELLED)
- Rate card and draft rate version metadata
- Append-only audit trail
- **No public gateway routes in v2.0A**

## Run locally

```bash
export DATABASE_URL=postgres://freight:freight_password@localhost:5432/freight_platform?sslmode=disable
export INTERNAL_SERVICE_TOKEN=dev-internal-token
make run-contract-rate-service
```

## Internal API

All routes under `/internal/v1/*` require header `X-Internal-Service-Token` plus trusted identity headers (`X-Tenant-ID`, `X-User-ID`, `X-Company-ID`, `X-Actor-Kind`).

Default port: `8091`.

## Tests

```bash
cd services/contract-rate-service && go test ./...
TEST_DATABASE_URL=... go test -tags=integration ./internal/integration/contractrate/...
```
