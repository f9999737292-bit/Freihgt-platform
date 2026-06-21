# Performance Report — Freight Platform v0.2

**Date:** 2026-06-18  
**Scope:** PostgreSQL connection pool metrics & slow query diagnostics  
**Previous report:** [PERFORMANCE_REPORT_V0.1.md](./PERFORMANCE_REPORT_V0.1.md)

---

## 1. What was added

| Component | Description |
|-----------|-------------|
| DB pool configuration | Env vars `DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS`, `DB_CONN_MAX_LIFETIME_SECONDS`, `DB_CONN_MAX_IDLE_TIME_SECONDS` via `packages/shared-go/database` |
| DB pool metrics | Prometheus gauges/counters: `db_pool_open_connections`, `db_pool_in_use_connections`, `db_pool_idle_connections`, `db_pool_wait_count_total`, `db_pool_wait_duration_seconds_total`, `db_pool_max_open_connections` |
| Shared helpers | `packages/shared-go/metrics/db_pool.go` — `RegisterDBPoolMetrics` (`database/sql`), `RegisterPgxPoolMetrics` (`pgxpool`) |
| Service integration | Pool config + metrics in all 7 PostgreSQL services |
| Slow query threshold | `DB_SLOW_QUERY_THRESHOLD_MS` (default `500`, `0` = disabled) in `ObserveDBQuery` |
| PostgreSQL local slow log | docker-compose `log_min_duration_statement=500`, `log_statement=none` |
| Dev scripts | `scripts/dev/check_db_pool_metrics.py` |
| Makefile | `make db-pool-metrics-check`, `make postgres-logs` |

**Not added (by design):** Redis, Kafka, new microservices, API changes.

---

## 2. Environment

| Parameter | Value |
|-----------|-------|
| OS | Windows 10 (build 26200) |
| Docker available | **yes** |
| Backend running | **yes** (pre-v0.2 images until rebuild) |
| Observability | **no** (optional profile) |
| Python on PATH | **no** (Make scripts need Python 3) |
| k6 installed | **no** |
| API Gateway | http://localhost:8080 |

---

## 3. Verification commands

| Command | Status | Notes |
|---------|--------|-------|
| `make platform-up` | **OK** | Backend containers healthy |
| `make migrate-up` | **OK** | No new migration in v0.2 |
| `make health-check` | **NOT CHECKED via Make** | Equivalent HTTP check: **OK** (8/8) |
| `make generate-db-metrics-traffic` | **NOT CHECKED via Make** | Equivalent API traffic: **OK** |
| `make db-metrics-check` | **NOT CHECKED via Make** | Equivalent `/metrics` check: **OK** (4/4) |
| `make db-pool-metrics-check` | **NOT CHECKED** | Requires Python + **rebuilt** service images |

### Backend health (equivalent to `make health-check`)

| Service | Endpoint | Status |
|---------|----------|--------|
| api-gateway | `/health` | **OK** |
| identity-service | `/health` | **OK** |
| company-service | `/health` | **OK** |
| transport-order-service | `/health` | **OK** |
| rfx-service | `/health` | **OK** |
| shipment-service | `/health` | **OK** |
| document-service | `/health` | **OK** |
| billing-register-service | `/health` | **OK** |

### DB metrics (equivalent to `make db-metrics-check`)

| Service | `db_query_duration_seconds_*` | Status |
|---------|------------------------------|--------|
| company-service | bucket, sum, count | **OK** |
| transport-order-service | bucket, sum, count | **OK** |
| shipment-service | bucket, sum, count | **OK** |
| billing-register-service | bucket, sum, count | **OK** |

### DB pool metrics (`make db-pool-metrics-check`)

| Service | Expected metrics | Status |
|---------|------------------|--------|
| company-service | `db_pool_*` | **NOT CHECKED** — running container predates v0.2 build |
| transport-order-service | `db_pool_*` | **NOT CHECKED** |
| shipment-service | `db_pool_*` | **NOT CHECKED** |
| billing-register-service | `db_pool_*` | **NOT CHECKED** |

**Action required:** `make platform-up` with rebuild when Docker Hub is reachable:

```bash
make platform-up
make generate-db-metrics-traffic
make db-pool-metrics-check
```

Local compile verification: `go build ./cmd/server` — **PASS** for company-service and transport-order-service.

Docker rebuild on 2026-06-18 failed: TLS handshake timeout to Docker Hub (same issue as v0.1 observability pull).

---

## 4. Expected metrics after rebuild

On `http://localhost:8082/metrics` (and 8083, 8085, 8087):

```text
db_query_duration_seconds_count{...}
db_pool_open_connections{service="company-service"}
db_pool_in_use_connections{service="company-service"}
db_pool_idle_connections{service="company-service"}
db_pool_max_open_connections{service="company-service"}
db_pool_wait_count_total{service="company-service"}
db_pool_wait_duration_seconds_total{service="company-service"}
```

Manual check (Windows):

```powershell
curl http://localhost:8082/metrics | findstr db_pool
```

Linux / WSL / Git Bash:

```bash
curl http://localhost:8082/metrics | grep db_pool
```

---

## 5. Slow query diagnostics

| Layer | Mechanism | Status |
|-------|-----------|--------|
| Application | `DB_SLOW_QUERY_THRESHOLD_MS=500` → JSON warn log | **Implemented** |
| Prometheus | `db_query_duration_seconds` histogram | **OK** (verified) |
| PostgreSQL local | `log_min_duration_statement=500` in docker-compose | **Configured** — requires postgres container recreate to apply |
| Logs | `make postgres-logs` | **Available** |

Slow log applies on **new** postgres container start (`make platform-down` then `make platform-up`).

---

## 6. Pool configuration defaults

| Variable | Default |
|----------|---------|
| `DB_MAX_OPEN_CONNS` | 25 |
| `DB_MAX_IDLE_CONNS` | 10 |
| `DB_CONN_MAX_LIFETIME_SECONDS` | 300 |
| `DB_CONN_MAX_IDLE_TIME_SECONDS` | 60 |
| `DB_SLOW_QUERY_THRESHOLD_MS` | 500 |

---

## 7. Recommendations

### Do now

1. **Rebuild backend** after network is stable: `make platform-up` (pull/build images).
2. **Install Python 3** on Windows for `make db-pool-metrics-check`.
3. **Recreate postgres** once to enable server-side slow query log (if not already recreated with v0.2 compose).

### Do not do yet

- **Redis** — add only when real latency/cache pressure is measured, not preemptively.
- **Kafka** — out of scope.

### When metrics are available

| Signal | Action |
|--------|--------|
| `db_pool_wait_count_total` rising | Increase `DB_MAX_OPEN_CONNS` or reduce query hold time |
| `db_pool_in_use_connections` ≈ `db_pool_max_open_connections` sustained | Check connection leaks, long transactions, missing pagination |
| `db_query_duration_seconds` p95 > 300ms on list/get | Run `make performance-index-check`, review indexes and N+1 |
| Application slow query warnings | Correlate with `make postgres-logs` and repository `operation` label |

### Process

- Keep Prometheus/Grafana **optional** (`make observability-up`).
- Use **Python scripts** for Windows-compatible checks.
- Run `make performance-smoke` after major backend changes (when k6 installed).
- Collect **real metrics** under load before infrastructure changes.

### Next performance steps (v0.3 candidates)

1. Connection pool metrics in Grafana dashboard panels
2. Automated `db-pool-metrics-check` in CI (smoke stage)
3. Extend pool metrics validation to identity, rfx, document in Makefile check
4. PostgreSQL `pg_stat_statements` for local dev (optional)

---

## 8. Final status

| Area | Status |
|------|--------|
| Backend health | **OK** |
| DB query metrics | **OK** |
| DB pool metrics (runtime) | **NOT CHECKED** (rebuild pending) |
| DB pool metrics (code) | **OK** (implemented + local `go build` pass) |
| Slow query app logging | **OK** (implemented) |
| PostgreSQL slow query log | **CONFIGURED** (recreate postgres to apply) |
| k6 smoke / load | **NOT CHECKED** (unchanged from v0.1) |
| Prometheus/Grafana | **OPTIONAL** |

### Overall v0.2 readiness

**Code & configuration: READY**  
**Runtime verification: PENDING** — Docker image rebuild + `make db-pool-metrics-check`

---

## Related documents

- [PERFORMANCE.md](./PERFORMANCE.md)
- [OBSERVABILITY.md](./OBSERVABILITY.md)
- [PERFORMANCE_REPORT_V0.1.md](./PERFORMANCE_REPORT_V0.1.md)
