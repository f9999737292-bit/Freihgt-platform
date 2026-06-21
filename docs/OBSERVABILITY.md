# Observability — Freight Platform v0.1

Production hardening layer: structured logging, request correlation, health/readiness probes, Prometheus metrics, Grafana dashboards, and a web-admin health page.

## What was added

| Component | Description |
|-----------|-------------|
| JSON logs | Unified structured logging via `packages/shared-go` (`timestamp`, `level`, `service`, `request_id`, …) |
| Request ID | `X-Request-ID` header propagated through API Gateway to downstream services |
| Access logs | Middleware logs method, path, status, duration, remote_addr, user_agent |
| Recover | Panic recovery with `INTERNAL_ERROR` JSON response (no stack trace to clients) |
| `GET /health` | Liveness: status, service, version, uptime, timestamp |
| `GET /ready` | Readiness: DB ping for data services; downstream checks for API Gateway |
| `GET /metrics` | Prometheus HTTP metrics (no auth in local dev; `/metrics` is public on gateway) |
| Prometheus | Scrapes all 8 backend services on ports 8080–8087 |
| Grafana | Pre-provisioned datasource + 3 dashboards |
| Web Admin | `/health` page with gateway and service status |

## Quick start

### Backend without observability

`make platform-up` starts PostgreSQL and all backend services only. Prometheus and Grafana are **not** started automatically (Docker Compose profile `observability`).

```bash
make platform-build
make platform-up
make migrate-up
make health-check
```

### Observability (optional)

When Docker Hub is reachable and you need dashboards:

```bash
make observability-up
```

Addresses:

- Prometheus: http://localhost:9090
- Grafana: http://localhost:3001 (admin / admin)

Verify:

```bash
make health-check
make ready-check
make metrics-check
```

## Addresses

| Service | URL |
|---------|-----|
| API Gateway | http://localhost:8080 |
| Swagger UI | http://localhost:8080/docs |
| Prometheus | http://localhost:9090 |
| Grafana | http://localhost:3001 |
| Web Admin Health | http://localhost:3000/health |

### Grafana login

- **User:** `admin`
- **Password:** `admin`

## Metrics

Each Go service exposes:

- `http_requests_total{service,method,path,status}`
- `http_request_duration_seconds{service,method,path}`
- `http_in_flight_requests{service}`
- `db_query_duration_seconds{service,repository,operation,status}`

Check all endpoints:

```bash
make metrics-check
```

Example Prometheus query:

```promql
http_requests_total
```

## Logs

Follow platform logs:

```bash
make platform-logs
```

Search by request ID:

```bash
docker compose -f infrastructure/docker-compose/docker-compose.yml logs | grep "{request_id}"
```

Replace `{request_id}` with the actual UUID from `X-Request-ID`.

Sensitive fields (passwords, JWT, Authorization headers) are not logged.

## Database query metrics

Метрика:

`db_query_duration_seconds`

Labels:

* `service`
* `repository`
* `operation`
* `status`

### Instrumented services (validated)

| Service | Port | Key operations |
|---------|------|----------------|
| company-service | 8082 | `list_companies`, `get_company`, `create_company` |
| transport-order-service | 8083 | `list_transport_orders`, `get_transport_order`, `create_transport_order` |
| shipment-service | 8085 | `list_shipments`, `get_shipment`, `create_shipment` |
| billing-register-service | 8087 | `list_billing_registers`, `get_billing_register`, `create_billing_register`, `calculate_billing_register` |

Slow queries (>500ms) emit a warning log with `repository`, `operation`, and `duration_ms` (no SQL parameters).

### Prometheus queries

DB query count:

```promql
db_query_duration_seconds_count
```

DB query p95 latency:

```promql
histogram_quantile(0.95, sum(rate(db_query_duration_seconds_bucket[5m])) by (le, service, repository, operation))
```

DB query errors:

```promql
sum(rate(db_query_duration_seconds_count{status="error"}[5m])) by (service, repository, operation)
```

DB query rate by service:

```promql
sum(rate(db_query_duration_seconds_count[5m])) by (service)
```

### Generate and verify DB metrics

```bash
make generate-db-metrics-traffic
make db-metrics-check
```

If `db_query_duration_seconds` is not visible:

1. Call an API endpoint that hits the database (e.g. list companies).
2. Check service metrics: `curl http://localhost:8082/metrics | grep db_query_duration_seconds`
3. Run `make generate-db-metrics-traffic` then `make db-metrics-check`

Expected metric lines after traffic:

* `db_query_duration_seconds_bucket`
* `db_query_duration_seconds_sum`
* `db_query_duration_seconds_count`

### TODO: extended rollout

Repository instrumentation for `identity-service`, `rfx-service`, and `document-service` exists in code but is not included in `make db-metrics-check` yet. Extend validation and Makefile checks when those services are load-tested.

## DB metrics verification

Static checks (no Docker required):

1. Confirm `packages/shared-go/metrics/metrics.go` defines `db_query_duration_seconds`, `ObserveDBQuery`, and `MeasureDBQuery` with labels `service`, `repository`, `operation`, `status`.
2. Confirm repository instrumentation in:
   - `services/company-service/internal/repository/`
   - `services/transport-order-service/internal/repository/`
   - `services/shipment-service/internal/repository/`
   - `services/billing-register-service/internal/repository/`
3. Run unit tests:

```bash
go test ./packages/shared-go/metrics/...
cd services/company-service && go test ./...
cd services/transport-order-service && go test ./...
cd services/shipment-service && go test ./...
cd services/billing-register-service && go test ./...
```

4. Search project for instrumentation:

```bash
grep -R "MeasureDBQuery" services packages
grep -R "db_query_duration_seconds" services packages
```

### Runtime verification (requires Docker)

Full end-to-end verification needs a running platform (PostgreSQL + services). Start Docker Desktop, then:

```bash
make platform-build
make platform-up
make migrate-up
make generate-db-metrics-traffic
make db-metrics-check
```

Manual check per service (cross-platform):

```bash
make generate-db-metrics-traffic
make db-metrics-check
```

Or run the Python script directly:

```bash
python scripts/dev/check_db_metrics.py
```

On Linux/macOS you may also use `curl` and `grep`:

```bash
curl http://localhost:8082/metrics | grep db_query_duration_seconds
```

Expected lines in `/metrics` after API traffic:

* `db_query_duration_seconds_bucket`
* `db_query_duration_seconds_sum`
* `db_query_duration_seconds_count`

Prometheus query:

```promql
db_query_duration_seconds_count
```

Optional: start observability stack and confirm scrape:

```bash
make observability-up
```

Then open http://localhost:9090 and run `db_query_duration_seconds_count`.

## Observability commands

```bash
make platform-up            # backend only (no Prometheus/Grafana)
make observability-up       # start Prometheus + Grafana (profile observability)
make observability-down     # stop Prometheus + Grafana
make observability-logs     # follow monitoring logs
make health-check           # Python /health on all services
make ready-check            # curl API Gateway /ready
make metrics-check          # verify /metrics on all services
make db-metrics-check       # Python check for db_query_duration_seconds
make db-pool-metrics-check  # Python check for db_pool_* metrics
make generate-db-metrics-traffic  # Python sample API calls for DB metrics
make postgres-logs          # PostgreSQL slow query / server logs
```

## PostgreSQL connection pool metrics

Pool metrics (v0.2) on `/metrics` for data services:

- `db_pool_open_connections{service}`
- `db_pool_in_use_connections{service}`
- `db_pool_idle_connections{service}`
- `db_pool_wait_count_total{service}`
- `db_pool_wait_duration_seconds_total{service}`
- `db_pool_max_open_connections{service}`

Pool env vars: `DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS`, `DB_CONN_MAX_LIFETIME_SECONDS`, `DB_CONN_MAX_IDLE_TIME_SECONDS`.

## Slow query diagnostics

- `db_query_duration_seconds` — histogram per repository operation
- `DB_SLOW_QUERY_THRESHOLD_MS=500` — application warning log (set `0` to disable)
- PostgreSQL local dev: `log_min_duration_statement=500` in docker-compose

```bash
make generate-db-metrics-traffic
make db-metrics-check
make db-pool-metrics-check
make postgres-logs
```

## Windows metrics check

On Windows, `make db-metrics-check` and `make generate-db-metrics-traffic` use Python scripts in `scripts/dev/` (stdlib only, no `grep` or `curl` required).

```bash
make generate-db-metrics-traffic
make db-metrics-check
```

Requires Python on `PATH` as `python` (default on Windows). Override if needed: `make db-metrics-check PYTHON=python3`.

## If `make platform-up` fails pulling Prometheus/Grafana

Prometheus and Grafana are in the Docker Compose profile `observability`. A normal `make platform-up` does **not** pull or start them.

If you previously ran the stack with monitoring included, stop old containers: `make platform-down`, then `make platform-up` again.

Start monitoring separately when the network allows: `make observability-up`.

## Troubleshooting a down service

1. Check containers: `make platform-ps` or `docker compose -f infrastructure/docker-compose/docker-compose.yml ps`
2. Check logs: `make platform-logs` or logs for a specific service
3. Check readiness: `curl http://localhost:8080/ready`
4. For DB-backed services, verify `DATABASE_URL` and PostgreSQL health
5. Check individual service: `curl http://localhost:8082/ready`

## Architecture

```
web-admin (/health) ──► api-gateway ──► microservices
                              │
Prometheus ◄── scrape /metrics on :8080–8087
     │
Grafana (dashboards)
```

## TODO (future)

- Loki / centralized log aggregation
- OpenTelemetry distributed tracing
- Alerting rules
