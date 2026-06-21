# Performance Report — Freight Platform v0.1

**Date:** 2026-06-18  
**Scope:** High Load / Performance v0.1 readiness  
**Report version:** v0.1

---

## 1. Summary

### What was added (v0.1)

- k6 performance tests (`tests/performance/k6/`) — smoke, module load, full-flow
- PostgreSQL index analysis (`scripts/performance/analyze_indexes.sql`, migration `000010_performance_indexes`)
- API Gateway rate limiting (`RATE_LIMIT_*`) and request body size limit (`MAX_REQUEST_BODY_BYTES`)
- Optional pprof endpoints (`PPROF_ENABLED=true`) in api-gateway and 4 data services
- Cross-platform dev scripts: `scripts/dev/check_backend_health.py`, `check_db_metrics.py`, `generate_db_metrics_traffic.py`
- Prometheus/Grafana moved to optional Docker Compose profile `observability`
- Documentation: [PERFORMANCE.md](./PERFORMANCE.md)

### Checks performed

| Check | Result |
|-------|--------|
| `make platform-up` (backend) | **PASS** — 9 containers healthy (postgres + 8 services) |
| Backend `/health` (all 8 services) | **PASS** — all OK |
| DB metrics traffic + `/metrics` | **PASS** — 4 instrumented services OK |
| `make performance-index-check` | **PASS** — expected filter indexes present (0 missing) |
| Migration `000010_performance_indexes` | **APPLIED** |

### Checks not performed

| Check | Reason |
|-------|--------|
| `make health-check` | Python not on `PATH` on verification host (Make exits 9009) |
| `make db-metrics-check` | Same — requires Python; equivalent HTTP checks passed manually |
| `make performance-smoke` | k6 not installed |
| `make performance-load` | k6 not installed (optional 5-minute test) |
| Rate limit manual curl test | Not run against live gateway in this session (unit tests pass in CI/local `go test`) |
| pprof browser check | Not run in this session |
| `make observability-up` | Prometheus/Grafana images not pulled (network TLS timeout to Docker Hub in prior run) |

### Current limitations

- Backend runs **without** Prometheus/Grafana by default; observability is optional.
- k6 and Python are required for Makefile performance and health scripts on Windows.
- Load test results depend on local hardware, Docker Desktop overhead, and seed data volume.
- DB metrics appear only after API traffic hits instrumented repositories.
- Rate limiting and pprof require **rebuilt** api-gateway image or local `go run` with env vars — Docker containers from an earlier build may not include v0.1 gateway changes until `make platform-up --build`.

---

## 2. Environment

| Parameter | Value |
|-----------|-------|
| OS | Windows 10 (build 26200) |
| Shell | PowerShell / GnuWin32 Make |
| Docker available | **yes** — Docker Desktop, backend containers running |
| k6 installed | **no** |
| Python on PATH (`make` scripts) | **no** (Windows Store stub only) |
| Backend running | **yes** — `make platform-up` state |
| Observability running | **no** — prometheus/grafana not started |
| API Gateway URL | http://localhost:8080 |
| Tenant ID (tests) | `74519f22-ff9b-4a8b-8fff-a958c689682f` |
| AUTH_ENABLED (compose) | `false` |

---

## 3. Backend health check

**Command:** `make health-check`  
**Makefile status:** NOT CHECKED (Python missing)  
**Equivalent verification:** `GET /health` on ports 8080–8087 — **2026-06-18**

| Service | Endpoint | Status |
|---------|----------|--------|
| api-gateway | http://localhost:8080/health | **OK** |
| identity-service | http://localhost:8081/health | **OK** |
| company-service | http://localhost:8082/health | **OK** |
| transport-order-service | http://localhost:8083/health | **OK** |
| rfx-service | http://localhost:8084/health | **OK** |
| shipment-service | http://localhost:8085/health | **OK** |
| document-service | http://localhost:8086/health | **OK** |
| billing-register-service | http://localhost:8087/health | **OK** |

All services returned JSON `{"status":"ok",...}` with HTTP 200.

---

## 4. DB metrics check

**Commands:**

```bash
make generate-db-metrics-traffic
make db-metrics-check
```

**Makefile status:** NOT CHECKED via Make (Python missing)  
**Equivalent verification:** API Gateway list requests + `/metrics` scrape — **2026-06-18**

Traffic endpoints (all HTTP 200):

- `GET /api/v1/companies?tenant_id=...`
- `GET /api/v1/transport-orders?tenant_id=...`
- `GET /api/v1/shipments?tenant_id=...`
- `GET /api/v1/billing-registers?tenant_id=...`

| Service | Port | `db_query_duration_seconds_bucket` | `db_query_duration_seconds_sum` | `db_query_duration_seconds_count` | Status |
|---------|------|-----------------------------------|--------------------------------|----------------------------------|--------|
| company-service | 8082 | present | present | present | **OK** |
| transport-order-service | 8083 | present | present | present | **OK** |
| shipment-service | 8085 | present | present | present | **OK** |
| billing-register-service | 8087 | present | present | present | **OK** |

---

## 5. k6 smoke test

**Command:** `make performance-smoke`  
**Status:** **NOT CHECKED — k6 not installed**

Install: https://k6.io/docs/get-started/installation/

Expected scenario (when k6 is available):

| Parameter | Value |
|-----------|-------|
| Script | `tests/performance/k6/smoke.js` |
| VUs | 1 |
| Duration | 30s |
| Thresholds | `http_req_failed < 5%`, `p95 < 500ms` |

| Metric | Value |
|--------|-------|
| Total requests | — |
| http_req_failed | — |
| p95 latency | — |
| p99 latency | — |
| Result | **NOT CHECKED** |

---

## 6. k6 load test

**Command:** `make performance-load`  
**Status:** **NOT CHECKED — optional 5 minute test; k6 not installed**

Expected scenario (when k6 is available):

| Parameter | Value |
|-----------|-------|
| Script | `tests/performance/k6/full-flow-load.js` |
| VUs | 20 |
| Duration | 5m |
| Thresholds | `http_req_failed < 2%`, `p95 < 1000ms`, `p99 < 2000ms` |

| Metric | Value |
|--------|-------|
| Total requests | — |
| http_req_failed | — |
| p95 latency | — |
| p99 latency | — |
| Result | **NOT CHECKED** |

---

## 7. Rate limit check

**Procedure:**

```bash
RATE_LIMIT_ENABLED=true RATE_LIMIT_RPS=1 RATE_LIMIT_BURST=1 make run-api-gateway
```

Then repeat quickly:

```bash
curl "http://localhost:8080/api/v1/companies?tenant_id=74519f22-ff9b-4a8b-8fff-a958c689682f"
```

**Expected:** first request HTTP 200, subsequent rapid requests HTTP 429 with:

```json
{
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "too many requests",
    "details": {}
  }
}
```

**Status:** **NOT CHECKED** (manual curl against live gateway not run in this session)

**Note:** `go test` for rate limit middleware in `api-gateway` — **PASS** (unit test confirms 429 on burst exceed).

---

## 8. pprof check

**Procedure:**

```bash
PPROF_ENABLED=true make run-api-gateway
```

**URL:** http://localhost:8080/debug/pprof/

**Expected endpoints:** `/debug/pprof/`, `/debug/pprof/profile`, `/debug/pprof/heap`, `/debug/pprof/goroutine`

**Status:** **NOT CHECKED** (browser/curl verification not run in this session)

**Note:** `packages/shared-go/pprof` integrated in api-gateway, company-service, transport-order-service, shipment-service, billing-register-service. Disabled by default (`PPROF_ENABLED=false`).

---

## 9. Known issues

1. **Prometheus/Grafana image pull** may fail on slow or restricted networks (TLS timeout to Docker Hub). Backend does not depend on them.
2. **Observability** starts separately: `make observability-up` (profile `observability`).
3. **Windows Makefile** — `grep`/`curl`/`true` are unreliable in GnuWin32 Make; health and DB metrics checks use **Python scripts** (`scripts/dev/`).
4. **Python on PATH** — required for `make health-check`, `make db-metrics-check`, `make performance-smoke` (via `run_k6.py`). Install Python 3 explicitly on Windows.
5. **k6** — required for `make performance-smoke` and `make performance-load`.
6. **DB metrics** — histogram series appear only after API requests execute DB queries (`make generate-db-metrics-traffic` first).
7. **AUTH_ENABLED=true** — traffic generator returns 401 without JWT; use login or temporarily `AUTH_ENABLED=false` for local metrics checks.
8. **Gateway v0.1 features** (rate limit, body limit, pprof) — require rebuild or local `go run`; existing Docker image may predate changes until `make platform-up` with rebuild.

---

## 10. Recommendations

### Keep

- Prometheus/Grafana in **optional** `observability` profile — backend starts without monitoring images.
- **Python scripts** instead of `grep` for Windows/Linux compatibility.
- Run **`make performance-smoke`** after significant backend changes (once k6 is installed).
- Run **`make performance-load`** before demo or release candidate.

### Monitor

- `http_request_duration_seconds` — API latency by path
- `db_query_duration_seconds` — repository query time
- `http_requests_total` — volume and status codes
- `http_rate_limited_total` — gateway throttling (after gateway rebuild)
- Error rate — target **< 2%** under load

### Next performance steps

1. Add **connection pool metrics** (pgx pool stats per service)
2. Enable **PostgreSQL slow query log** in dev/staging (threshold e.g. 300ms)
3. Add **Redis** only after measured cache need — not before
4. Re-run **`make performance-index-check`** after production-scale seed data
5. Extend DB metrics validation to identity-service, rfx-service, document-service
6. Automate performance report generation in CI (smoke only) when k6 is available

---

## 11. Final status

| Area | Status |
|------|--------|
| Backend health | **OK** |
| DB metrics | **OK** |
| k6 smoke | **NOT CHECKED** |
| k6 load | **NOT CHECKED** |
| Rate limit | **NOT CHECKED** |
| pprof | **NOT CHECKED** |
| Prometheus/Grafana | **OPTIONAL** (not running) |
| PostgreSQL indexes | **OK** (`performance-index-check`, migration 10 applied) |

### Overall v0.1 readiness

**Backend and observability foundation: READY**  
**Load test validation: PENDING** (install k6 + Python, rebuild gateway, run smoke/load)

---

## Related documents

- [PERFORMANCE.md](./PERFORMANCE.md) — how to run tests and interpret metrics
- [OBSERVABILITY.md](./OBSERVABILITY.md) — metrics, Grafana, optional stack
- [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) — Windows, Docker Hub, Python
