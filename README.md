# Freight Platform

Production-ready monorepo skeleton for the Freight Platform logistics system.

Open in Cursor: **File → Open Workspace from File** → `freight-platform.code-workspace`

## Structure

```
freight-platform/
├── apps/                         # Nuxt 3 frontend applications
│   ├── web-admin/                # port 3000
│   ├── web-shipper/              # port 3001
│   ├── web-carrier/              # port 3002
│   ├── web-consignee/            # port 3003
│   ├── web-finance/              # port 3004
│   └── web-procurement/          # port 3005
├── services/                     # Go backend microservices
│   ├── api-gateway/              # port 8080
│   ├── identity-service/         # port 8081
│   ├── company-service/          # port 8082
│   ├── transport-order-service/  # port 8083
│   ├── rfx-service/              # port 8084
│   ├── shipment-service/         # port 8085
│   ├── document-service/         # port 8086
│   └── billing-register-service/ # port 8087
├── packages/
│   ├── shared-go/                # Go: config, logging, health, server
│   ├── shared-ts/                # TypeScript shared types
│   ├── openapi/                  # OpenAPI specifications
│   ├── proto/                    # Protobuf definitions
│   ├── ui/                       # Shared Vue components
│   └── i18n/                     # Shared locales (ru-RU, en-US, zh-CN)
├── infrastructure/               # Docker Compose + SQL migrations (preserved)
│   ├── docker-compose/
│   └── migrations/
└── docs/
    ├── architecture/             # blueprint-v0.2.md
    ├── database/                 # database-schema-v0.1.md
    ├── api/                      # api-design-v0.1.md
    ├── events/                   # event-catalog-v0.1.md
    ├── security/
    ├── procurement/
    └── billing/
```

## Prerequisites

- Go 1.22+
- Node.js 20+ and pnpm 9+
- Docker Desktop (PostgreSQL for local dev)
- GNU Make

## Quick start

### 1. Infrastructure (existing, do not modify migrations)

```bash
make env-init
make dev-up
make migrate-up
make db-check
```

Expected schemas: `core`, `transport`, `rfx`, `documents`, `billing`.

## Запуск всей backend-платформы

Запуск PostgreSQL и всех backend-микросервисов одной командой через Docker Compose. **Prometheus и Grafana не запускаются** — для них см. [Observability](#observability).

```bash
make platform-build
make platform-up
make migrate-up
make health-check
```

On Windows, if `make platform-up` fails with Docker EOF or WSL crash (`0xc00000fd`), use the safe serial workflow:

```bash
make platform-up-safe
```

Or build only failed services, then start without rebuild:

```bash
make platform-build-service SERVICE=document-service
make platform-build-service SERVICE=billing-register-service
make platform-build-service SERVICE=api-gateway
make platform-up-no-build
```

See [docs/DOCKER_WSL_STABILITY.md](docs/DOCKER_WSL_STABILITY.md).

Проверка API Gateway:

```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready
curl http://localhost:8080/routes
```

Проверка отдельных сервисов:

```bash
curl http://localhost:8081/health
curl http://localhost:8082/health
curl http://localhost:8083/health
curl http://localhost:8084/health
curl http://localhost:8085/health
curl http://localhost:8086/health
curl http://localhost:8087/health
```

Дополнительные команды:

```bash
make platform-ps       # статус контейнеров
make platform-logs     # логи всех сервисов
make platform-down     # остановить платформу
make platform-restart  # перезапустить платформу
```

## Порты сервисов

| Port | Service |
|------|---------|
| 8080 | api-gateway |
| 8081 | identity-service |
| 8082 | company-service |
| 8083 | transport-order-service |
| 8084 | rfx-service |
| 8085 | shipment-service |
| 8086 | document-service |
| 8087 | billing-register-service |
| 5432 | postgres |

## API Documentation

Unified OpenAPI specs live in `packages/openapi/`. The API Gateway serves Swagger UI and spec files.

Commands:

```bash
make openapi-check
make run-api-gateway
```

Addresses (when gateway is running on port 8080):

- Swagger UI: http://localhost:8080/docs
- OpenAPI YAML: http://localhost:8080/openapi.yaml
- OpenAPI JSON: http://localhost:8080/openapi.json
- OpenAPI index: http://localhost:8080/openapi

With Docker Compose:

```bash
make platform-up
make api-docs-open
```

## Frontend Admin

Administrative Nuxt 3 app for platform operators.

Commands:

```bash
make install-web-admin
make run-web-admin
```

Address: http://localhost:3000

Control Tower: [docs/CONTROL_TOWER.md](docs/CONTROL_TOWER.md) — http://localhost:3000/control-tower

Health dashboard: http://localhost:3000/health

## Observability

Production hardening v0.1: JSON logs, request ID, `/health`, `/ready`, `/metrics`, optional Prometheus + Grafana.

- Documentation: [docs/OBSERVABILITY.md](docs/OBSERVABILITY.md)
- Web Admin Health Dashboard: http://localhost:3000/health

**Backend only** (no image pull for Prometheus/Grafana):

```bash
make platform-up
make migrate-up
make health-check
```

**Observability stack** (when network allows):

```bash
make observability-up
```

- Prometheus: http://localhost:9090
- Grafana: http://localhost:3001 (admin / admin)

**DB metrics** (Windows-friendly, Python scripts):

```bash
make generate-db-metrics-traffic
make db-metrics-check
```

Other checks:

```bash
make metrics-check
make ready-check
```

## Performance and High Load

Load testing, index review, rate limiting, and pprof for local high-load readiness v0.1.

```bash
make performance-smoke
make performance-load
make performance-index-check
make db-pool-metrics-check
```

Requires [k6](https://k6.io/docs/get-started/installation/) for load tests. If k6 is missing, Make prints install instructions.

Documentation:

- [docs/PERFORMANCE.md](docs/PERFORMANCE.md)
- [docs/PERFORMANCE_REPORT_V0.1.md](docs/PERFORMANCE_REPORT_V0.1.md) — performance report (2026-06-18)
- [docs/PERFORMANCE_REPORT_V0.2.md](docs/PERFORMANCE_REPORT_V0.2.md) — pool metrics & slow query log (2026-06-18)

Mock auth is enabled by default (`NUXT_PUBLIC_MOCK_AUTH=true` in `apps/web-admin/.env.example`). Copy to `.env` and adjust as needed.

### 2. Backend services (local run)

Each Go service provides:

- `GET /health` (also `/healthz`, `/ready`)
- config from environment variables
- structured JSON logging
- graceful shutdown

```bash
make run-identity-service
# or
cd services/identity-service && go run ./cmd/server
```

Build all services:

```bash
make go-build
```

### 3. Frontend apps

Nuxt 3 + Vue 3 + TypeScript with i18n (`ru-RU`, `en-US`, `zh-CN`).

```bash
pnpm install
pnpm dev:admin
```

| App | Port | Command |
|-----|------|---------|
| web-admin | 3000 | `pnpm dev:admin` |
| web-shipper | 3001 | `pnpm dev:shipper` |
| web-carrier | 3002 | `pnpm dev:carrier` |
| web-consignee | 3003 | `pnpm dev:consignee` |
| web-finance | 3004 | `pnpm dev:finance` |
| web-procurement | 3005 | `pnpm dev:procurement` |

## Windows Docker/WSL safe startup

If `make platform-up` fails with Docker EOF or WSL crash, use:

```bash
make platform-up-safe
```

Or build failed services one by one:

```bash
make platform-build-service SERVICE=document-service
make platform-build-service SERVICE=billing-register-service
make platform-build-service SERVICE=api-gateway
make platform-up-no-build
```

Details: [docs/DOCKER_WSL_STABILITY.md](docs/DOCKER_WSL_STABILITY.md).

## Make targets

```bash
make help              # list all commands
make dev-up            # start PostgreSQL only
make platform-up       # start backend platform in Docker (no Prometheus/Grafana)
make platform-up-safe  # Windows/WSL: serial build then start (no parallel build)
make observability-up  # optional: Prometheus + Grafana
make performance-smoke # k6 smoke test (requires k6)
make migrate-up        # apply migrations
make db-check          # verify schemas
make go-build          # build all Go services
make run-api-gateway   # run a single service locally
```

## Documentation

| Document | Path |
|----------|------|
| Project map | [docs/PROJECT_MAP.md](docs/PROJECT_MAP.md) |
| File index | [docs/FILE_INDEX.md](docs/FILE_INDEX.md) |
| Developer handbook | [docs/DEVELOPER_HANDBOOK.md](docs/DEVELOPER_HANDBOOK.md) |
| Quick start | [docs/QUICK_START.md](docs/QUICK_START.md) |
| Troubleshooting | [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) |
| Observability | [docs/OBSERVABILITY.md](docs/OBSERVABILITY.md) |
| Performance | [docs/PERFORMANCE.md](docs/PERFORMANCE.md) |
| Performance report v0.1 | [docs/PERFORMANCE_REPORT_V0.1.md](docs/PERFORMANCE_REPORT_V0.1.md) |
| Performance report v0.2 | [docs/PERFORMANCE_REPORT_V0.2.md](docs/PERFORMANCE_REPORT_V0.2.md) |
| Docker disk troubleshooting | [docs/DOCKER_DISK_TROUBLESHOOTING.md](docs/DOCKER_DISK_TROUBLESHOOTING.md) |
| Docker / WSL stability (Windows) | [docs/DOCKER_WSL_STABILITY.md](docs/DOCKER_WSL_STABILITY.md) |
| Control Tower | [docs/CONTROL_TOWER.md](docs/CONTROL_TOWER.md) |
| Project audit report v0.1 | [docs/PROJECT_AUDIT_REPORT_V0.1.md](docs/PROJECT_AUDIT_REPORT_V0.1.md) |
| Auth & RBAC | [docs/AUTH_RBAC.md](docs/AUTH_RBAC.md) |
| Windows environment | [docs/WINDOWS_ENVIRONMENT.md](docs/WINDOWS_ENVIRONMENT.md) |
| Runtime verification report v0.1 | [docs/RUNTIME_VERIFICATION_REPORT_V0.1.md](docs/RUNTIME_VERIFICATION_REPORT_V0.1.md) |
| Frontend backend status | [docs/FRONTEND_BACKEND_STATUS.md](docs/FRONTEND_BACKEND_STATUS.md) |

## Environment checks

Cross-platform diagnostics before runtime verification:

```bash
make python-check
make docker-readiness
make ports-check
```

Windows alternative (when `python` is not in PATH):

```bash
make PYTHON="py -3" docker-readiness
make PYTHON="py -3" ports-check
make PYTHON="py -3" health-check
```

See [docs/WINDOWS_ENVIRONMENT.md](docs/WINDOWS_ENVIRONMENT.md).

## Documentation placeholders

| Document | Path |
|----------|------|
| Blueprint v0.2 | `docs/architecture/blueprint-v0.2.md` |
| Database schema v0.1 | `docs/database/database-schema-v0.1.md` |
| API design v0.1 | `docs/api/api-design-v0.1.md` |
| Event catalog v0.1 | `docs/events/event-catalog-v0.1.md` |

## Architecture notes

- **Local dev**: all DB schemas in one PostgreSQL instance
- **Production (target)**: each service owns its database
- **Business logic**: not implemented — skeleton only

See `infrastructure/README.md` and `docs/` for details.

## Docker

Build a service image from the monorepo root:

```bash
docker build -f services/identity-service/Dockerfile -t freight-platform/identity-service .
```

Docker disk troubleshooting (Windows / WSL): [docs/DOCKER_DISK_TROUBLESHOOTING.md](docs/DOCKER_DISK_TROUBLESHOOTING.md)

```bash
make docker-disk-usage
make docker-clean-safe   # does not remove volumes
make docker-volumes
```
