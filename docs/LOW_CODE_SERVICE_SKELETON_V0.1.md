# Low-code Service Skeleton v0.1

## Summary

Minimal skeleton for **`low-code-service`**: observability endpoints, PostgreSQL connectivity, and readiness checks for the `lowcode` schema (migration `000011`). No Form Builder, Custom Fields, Rule Engine, or business APIs yet.

## Service Name

`low-code-service`

## Port

**8088** (`LOW_CODE_SERVICE_PORT`)

## Endpoints

| Method | Path | Description |
| ------ | ---- | ----------- |
| GET | `/health` | Liveness — always 200 when process is up |
| GET | `/ready` | Readiness — Postgres + `lowcode` schema + `lowcode.form_templates` |
| GET | `/metrics` | Prometheus metrics (HTTP + pgx pool) |

### GET /health

```json
{
  "status": "ok",
  "service": "low-code-service"
}
```

### GET /ready (success)

```json
{
  "status": "ready",
  "service": "low-code-service",
  "database": "ok",
  "schema": "lowcode"
}
```

### GET /ready (failure)

HTTP **503** with JSON body, e.g.:

```json
{
  "status": "not_ready",
  "service": "low-code-service",
  "database": "down",
  "error": "..."
}
```

## Readiness Checks

1. PostgreSQL ping (`DATABASE_URL`)
2. Schema `lowcode` exists (`information_schema.schemata`)
3. Table `lowcode.form_templates` exists (`information_schema.tables`)

Requires migration `000011` applied (`make migrate-up`).

## Docker Compose

Service: `low-code-service`

- Container: `freight_low_code_service`
- Port: `8088:8088`
- Depends on: `postgres` (healthy)
- Healthcheck: `wget http://localhost:8088/health`

`api-gateway` depends on `low-code-service` (healthy) and receives `LOW_CODE_SERVICE_URL=http://low-code-service:8088`.

## API Gateway Route

| Gateway prefix | Downstream |
| -------------- | ---------- |
| `/api/v1/low-code/*` | `low-code-service` → `/v1/low-code/*` |

No business routes under `/v1/low-code` yet — proxy is wired for future APIs. Gateway `/ready` includes `low-code-service`.

## What Is Implemented

- Service skeleton (`cmd/server`, config, HTTP router)
- `/health`, `/ready`, `/metrics`
- Postgres pool via `shared-go/database`
- Custom readiness for `lowcode` schema + `form_templates`
- Dockerfile + docker-compose entry
- Makefile targets (`run-low-code-service`, `test-low-code-service`, platform build)
- `go.work` module entry
- Health-check script port 8088
- Prometheus scrape job

## What Is Not Implemented Yet

- Form Builder API
- Custom Fields API
- Rule Engine
- BPMN / No-code Connectors
- Repository CRUD
- Business validation
- OpenAPI business paths (only gateway proxy prefix)
- RLS / tenant middleware

## Verification Commands

```powershell
cd D:\Projects\freight-platform

# Build and start
make platform-build-service SERVICE=low-code-service
make platform-build-service SERVICE=api-gateway
make platform-up-no-build

# Health
make health-check
curl http://localhost:8088/health
curl http://localhost:8088/ready
curl http://localhost:8088/metrics

# Gateway proxy (404 expected until routes exist)
curl http://localhost:8080/api/v1/low-code/health

# Tests
cd services/low-code-service
go test ./...

# Core flow unchanged
make seed-dev-admin
make seed-demo-data
make integration-smoke-test
```

## Next Action

1. Read-only repository for `form_templates` (separate pack).
2. `GET /v1/low-code/form-templates` with OpenAPI placeholder.
3. Tenant header middleware (app-level filtering v0.1).
4. Wire Form Builder MVP per `docs/LOW_CODE_MVP_SCOPE_V0.1.md`.
