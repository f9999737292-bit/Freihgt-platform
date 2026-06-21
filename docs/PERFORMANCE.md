# Performance and High Load — Freight Platform v0.1

Baseline load testing, index review, and gateway hardening for local high-load readiness.

## 1. Start backend

Backend runs without Prometheus/Grafana:

```bash
make platform-up
make migrate-up
make health-check
```

Optional observability:

```bash
make observability-up
```

## 2. Smoke performance test

Requires [k6](https://k6.io/docs/get-started/installation/).

```bash
make performance-smoke
```

Or:

```bash
./scripts/performance/run_k6_smoke.sh
```

Smoke scenario (1 VU, 30s):

- `GET /health`, `GET /ready`
- `GET /api/v1/companies?tenant_id=...`
- `GET /api/v1/transport-orders?tenant_id=...`
- `GET /api/v1/shipments?tenant_id=...`
- `GET /api/v1/billing-registers?tenant_id=...`

Thresholds: `http_req_failed < 5%`, `p95 < 500ms`.

Environment:

| Variable | Default |
|----------|---------|
| `API_BASE_URL` | `http://localhost:8080` |
| `TENANT_ID` | `74519f22-ff9b-4a8b-8fff-a958c689682f` |

## 3. Load tests

```bash
make performance-load              # full-flow, 20 VU, 5m
make performance-companies
make performance-transport-orders
make performance-rfx
make performance-shipments
make performance-billing
```

Full-flow thresholds: `http_req_failed < 2%`, `p95 < 1000ms`, `p99 < 2000ms`.

Module tests: see [tests/performance/k6/README.md](../tests/performance/k6/README.md).

If k6 is not installed, Make prints:

```
k6 not found. Install from https://k6.io/docs/get-started/installation/
```

## 4. PostgreSQL index check

```bash
make performance-index-check
```

Runs `scripts/performance/analyze_indexes.sql` against `freight_postgres`:

1. Table sizes
2. Indexes in `core`, `transport`, `rfx`, `documents`, `billing`
3. Tables with `tenant_id` but no index on it
4. Expected filter columns — reports `MISSING` if no matching index

Migration `000010_performance_indexes` adds `idx_freight_requests_request_type` if missing.

## 5. pprof (dev only)

Disabled by default. Enable per service:

```bash
PPROF_ENABLED=true make run-api-gateway
```

Endpoints (when enabled):

- `GET /debug/pprof/`
- `GET /debug/pprof/profile`
- `GET /debug/pprof/heap`
- `GET /debug/pprof/goroutine`

Enabled on: api-gateway, company-service, transport-order-service, shipment-service, billing-register-service.

Do not enable in production without network isolation and authentication.

## 6. API Gateway rate limiting

Environment (docker-compose or local):

```bash
RATE_LIMIT_ENABLED=true
RATE_LIMIT_RPS=50
RATE_LIMIT_BURST=100
```

Applied to `/api/v1/*` only. Excluded: `/health`, `/ready`, `/metrics`, `/docs`, `/openapi*`.

Response on limit:

```json
{
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "too many requests",
    "details": {}
  }
}
```

Metric: `http_rate_limited_total{service="api-gateway"}`.

Local test:

```bash
RATE_LIMIT_ENABLED=true RATE_LIMIT_RPS=1 RATE_LIMIT_BURST=1 make run-api-gateway
curl "http://localhost:8080/api/v1/companies?tenant_id=74519f22-ff9b-4a8b-8fff-a958c689682f"
# repeat quickly → expect 429
```

## 7. Request body size limit

```bash
MAX_REQUEST_BODY_BYTES=10485760   # default 10 MB
```

Oversized body → HTTP 413:

```json
{
  "error": {
    "code": "REQUEST_BODY_TOO_LARGE",
    "message": "request body is too large",
    "details": {}
  }
}
```

## 8. Metrics

| Source | URL |
|--------|-----|
| API Gateway | http://localhost:8080/metrics |
| Prometheus (optional) | http://localhost:9090 |
| Grafana (optional) | http://localhost:3001 |

Key series:

- `http_request_duration_seconds` — API latency
- `db_query_duration_seconds` — repository query time
- `db_pool_*` — PostgreSQL connection pool (v0.2)
- `http_rate_limited_total` — rate limit hits

Generate DB traffic before checking DB metrics:

```bash
make generate-db-metrics-traffic
make db-metrics-check
make db-pool-metrics-check
make postgres-logs
```

## PostgreSQL connection pool metrics

Pool configuration (all PostgreSQL services):

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_MAX_OPEN_CONNS` | `25` | Maximum open connections |
| `DB_MAX_IDLE_CONNS` | `10` | Minimum idle connections (pgx `MinConns`) |
| `DB_CONN_MAX_LIFETIME_SECONDS` | `300` | Max connection lifetime |
| `DB_CONN_MAX_IDLE_TIME_SECONDS` | `60` | Max idle time before close |

Prometheus metrics (label `service`):

| Metric | Description |
|--------|-------------|
| `db_pool_open_connections` | Open connections in pool |
| `db_pool_in_use_connections` | Connections currently in use |
| `db_pool_idle_connections` | Idle connections |
| `db_pool_wait_count_total` | Total pool wait events |
| `db_pool_wait_duration_seconds_total` | Total time waiting for connections |
| `db_pool_max_open_connections` | Configured max open connections |

Prometheus queries:

```promql
db_pool_open_connections
db_pool_in_use_connections
db_pool_wait_count_total
db_pool_wait_duration_seconds_total
```

Services with pool metrics: company, transport-order, shipment, billing-register (required); identity, rfx, document (also enabled).

Implementation: `packages/shared-go/metrics/db_pool.go` — `RegisterDBPoolMetrics` (`database/sql`) and `RegisterPgxPoolMetrics` (`pgxpool`).

## Slow query diagnostics

Repository-level duration is recorded in `db_query_duration_seconds`.

Application slow query warnings:

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_SLOW_QUERY_THRESHOLD_MS` | `500` | Log warning when query exceeds threshold; `0` disables |

Log format (no SQL or PII):

```json
{
  "level": "warn",
  "service": "shipment-service",
  "repository": "shipment_repository",
  "operation": "list_shipments",
  "duration_ms": 743,
  "threshold_ms": 500,
  "message": "slow database query"
}
```

PostgreSQL local dev (docker-compose) logs server-side slow queries:

- `log_min_duration_statement=500` (ms)
- `log_statement=none`
- `log_connections=off`
- `log_disconnections=off`

View PostgreSQL logs:

```bash
make postgres-logs
```

## 9. MVP performance thresholds

| Metric | Target |
|--------|--------|
| API p95 latency | < 1000 ms |
| API p99 latency | < 2000 ms |
| Error rate | < 2% |
| DB p95 (list/get) | < 300 ms |
| API Gateway local | sustain ~50 RPS |

## 10. Slow request playbook

1. Inspect `db_query_duration_seconds` in `/metrics` or Grafana
2. Inspect `db_pool_in_use_connections` and `db_pool_wait_count_total`
3. Inspect `http_request_duration_seconds` by path
4. Run `make performance-index-check`
5. Run `make db-pool-metrics-check`
6. Look for N+1 in repository code
7. Verify list endpoints use pagination and limits
8. Check slow query warnings in service logs (`DB_SLOW_QUERY_THRESHOLD_MS`)
9. Check PostgreSQL slow query log: `make postgres-logs`
10. With `PPROF_ENABLED=true`, capture CPU/heap profiles under load

## 11. Performance report template

After a test run, record results:

```markdown
# Performance report — YYYY-MM-DD

## Environment
- Host / OS:
- `make platform-ps` snapshot:
- k6 version:
- Migration version:

## Tests run
- [ ] performance-smoke
- [ ] performance-load
- [ ] performance-index-check

## k6 results
| Test | VUs | Duration | http_req_failed | p95 | p99 | Pass |
|------|-----|----------|-----------------|-----|-----|------|
| smoke | 1 | 30s | | | | |
| full-flow | 20 | 5m | | | | |

## DB metrics (after traffic)
- company-service:
- transport-order-service:
- shipment-service:
- billing-register-service:

## Index check
- Missing indexes:
- Largest tables:

## Findings
1.
2.

## Actions
1.
2.
```

Save reports under `docs/performance-reports/` (optional, not committed by default).

**Completed report (v0.1):** [PERFORMANCE_REPORT_V0.1.md](./PERFORMANCE_REPORT_V0.1.md)  
**Completed report (v0.2):** [PERFORMANCE_REPORT_V0.2.md](./PERFORMANCE_REPORT_V0.2.md)

## Related docs

- [OBSERVABILITY.md](./OBSERVABILITY.md)
- [TROUBLESHOOTING.md](./TROUBLESHOOTING.md)
