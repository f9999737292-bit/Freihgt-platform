# Project Audit Report v0.1

**Project:** freight-platform  
**Date:** 2026-06-19  
**Scope:** Full-stack audit of backend, frontend, infrastructure, observability, performance, documentation, and runtime verification.

**Audit method:** Static codebase inspection + limited local commands (Docker, Go, Node.js). No backend business logic was changed during this audit.

**Environment note:** Docker Desktop was available, but only `freight_postgres` was running at audit time. Backend microservice containers were not up (leftover from prior Windows port-binding failures). Runtime checks that require `:8080`–`:8087` are marked **BLOCKED BY ENVIRONMENT**.

---

## 1. Project structure / navigation

### Documentation files

| Item | Status | Comment |
| ---- | ------ | ------- |
| `docs/PROJECT_MAP.md` | OK | Exists |
| `docs/FILE_INDEX.md` | OK | Exists |
| `docs/DEVELOPER_HANDBOOK.md` | OK | Exists; references some Makefile targets not yet implemented |
| `docs/QUICK_START.md` | OK | Exists; documents `seed-dev-admin` and `full-flow-smoke-test` as planned |
| `docs/TROUBLESHOOTING.md` | OK | Exists |

### Makefile navigation commands

| Item | Status | Comment |
| ---- | ------ | ------- |
| `make project-map` | MISSING | Target not defined in `Makefile`; documented in `DEVELOPER_HANDBOOK.md` |
| `make tree-project` | MISSING | Target not defined in `Makefile` |
| `make find-service NAME=company` | MISSING | Target not defined; handbook shows `make find-service NAME=company` |
| `make find-text TEXT=READY_FOR_BILLING` | MISSING | Target not defined; handbook shows `make find-text` examples |

**Section status:** **PARTIAL** — docs exist; navigation Makefile targets are documented but not implemented.

---

## 2. Backend services

| Service | Folder | Health | Ready | Metrics | Docker | Status |
| ------- | ------ | ------ | ----- | ------- | ------ | ------ |
| api-gateway | `services/api-gateway` | OK | OK | OK | OK | OK |
| identity-service | `services/identity-service` | OK | OK | OK | OK | OK |
| company-service | `services/company-service` | OK | OK | OK | OK | OK |
| transport-order-service | `services/transport-order-service` | OK | OK | OK | OK | OK |
| rfx-service | `services/rfx-service` | OK | OK | OK | OK | OK |
| shipment-service | `services/shipment-service` | OK | OK | OK | OK | OK |
| document-service | `services/document-service` | OK | OK | OK | OK | OK |
| billing-register-service | `services/billing-register-service` | OK | OK | OK | OK | OK |

**Per-service checklist:**

| Check | Result |
| ----- | ------ |
| `cmd/server/main.go` | OK for all 8 services |
| `/health`, `/ready`, `/metrics` | OK via `packages/shared-go/observability` (gateway implements explicitly) |
| `Dockerfile` | OK for all 8 |
| `README.md` | OK for all 8 |
| docker-compose wiring | OK |
| Ports 8080–8087 | OK in `infrastructure/docker-compose/docker-compose.yml` |

**Section status:** **OK**

---

## 3. Docker Compose

**File:** `infrastructure/docker-compose/docker-compose.yml`

### Core services (required by `make platform-up`)

| Service | Status |
| ------- | ------ |
| postgres | OK |
| api-gateway | OK |
| identity-service | OK |
| company-service | OK |
| transport-order-service | OK |
| rfx-service | OK |
| shipment-service | OK |
| document-service | OK |
| billing-register-service | OK |

### Observability (optional profile)

| Service | Profile | Status |
| ------- | ------- | ------ |
| prometheus | `observability` | OK |
| grafana | `observability` | OK |

| Requirement | Status | Comment |
| ----------- | ------ | ------- |
| `make platform-up` does not require Prometheus/Grafana | OK | `platform-up` runs `docker compose up -d --build` without `--profile observability` |
| `make observability-up` starts Prometheus/Grafana separately | OK | Uses `--profile observability up -d prometheus grafana` |
| `make observability-down` | OK | Defined |
| `make observability-logs` | OK | Defined |
| `make platform-ps` | OK | Defined; at audit time only `freight_postgres` was Up |
| `make platform-logs` | OK | Defined |

**PostgreSQL slow query (Performance v0.2):** `log_min_duration_statement=500` configured in compose.

**Section status:** **OK** (compose design); **runtime PARTIAL** (only postgres running during audit).

---

## 4. Database migrations

**Path:** `infrastructure/migrations/` (flat files, not subfolders)

| Migration | Up | Down | Status |
| --------- | -- | ---- | ------ |
| 000001_create_schemas | OK | OK | OK |
| 000002_create_core_tables | OK | OK | OK |
| 000003_create_transport_tables | OK | OK | OK |
| 000004_create_rfx_tables | OK | OK | OK |
| 000005_create_documents_tables | OK | OK | OK |
| 000006_create_billing_tables | OK | OK | OK |
| 000007_create_triggers | OK | OK | OK |
| 000008_seed_permissions | OK | OK | OK |
| 000009_seed_roles | OK | OK | OK |
| 000010_performance_indexes | OK | OK | OK |

**Notes:**

- All migrations have paired `.up.sql` / `.down.sql`.
- `000010_performance_indexes` is a separate migration (idempotent index on `rfx.freight_requests.request_type`).
- Broader index validation: `scripts/performance/analyze_indexes.sql` + `make performance-index-check`.

**Section status:** **OK**

---

## 5. API Gateway

**Path:** `services/api-gateway`

| Feature | Status | Notes |
| ------- | ------ | ----- |
| Reverse proxy routes | OK | `/api/*` → 7 downstream services |
| `/health` | OK | |
| `/ready` | OK | Aggregates downstream readiness |
| `/metrics` | OK | |
| `/docs` | OK | Swagger UI |
| `/openapi.yaml` | OK | |
| `/openapi.json` | OK | |
| `/openapi` (alias routes) | OK | |
| request_id middleware | OK | `sharedmiddleware.RequestID` |
| access logs | OK | `sharedmiddleware.AccessLog` |
| recover middleware | OK | `sharedmiddleware.Recover` |
| optional auth middleware | OK | `AUTH_ENABLED` (default `false` in compose) |
| rate limiting | OK | `RATE_LIMIT_ENABLED`, `RATE_LIMIT_RPS`, `RATE_LIMIT_BURST` |
| request body size limit | OK | `MAX_REQUEST_BODY_BYTES` |
| pprof | PARTIAL | Implemented via `PPROF_ENABLED=true` (default off) |

**Section status:** **OK** (pprof is opt-in by design)

---

## 6. Auth / RBAC

### Backend (identity-service)

| Item | Status | Comment |
| ---- | ------ | ------- |
| `POST /v1/auth/login` | OK | |
| `GET /v1/auth/me` | OK | |
| Users API | OK | `/v1/users` |
| Roles API | OK | `/v1/roles` |
| Permissions | PARTIAL | Via `GET /v1/roles/{role_id}/permissions`; no standalone `/v1/permissions` list in router |
| JWT | OK | Documented in identity-service README |

### Frontend (web-admin)

| Item | Status | Comment |
| ---- | ------ | ------- |
| `stores/auth.ts` | OK | Token + user in localStorage |
| Login page | OK | `pages/login.vue` |
| `useApi` adds `Authorization` | OK | |
| `useApi` adds `X-Tenant-ID` | OK | |
| Logout | OK | `useAuth().logout()` |
| 401 redirect to login | PARTIAL | Route middleware redirects unauthenticated users; no global handler for API `401` responses |
| Role/permission helpers | MISSING | No `hasPermission` / `hasRole` composables in web-admin source |

### Dev admin

| Item | Status | Comment |
| ---- | ------ | ------- |
| Tenant `74519f22-ff9b-4a8b-8fff-a958c689682f` | OK | Documented in QUICK_START, web-admin `.env.example` |
| Email `admin@7rights.local` | OK | Documented |
| Password `Admin123456!` | OK | Documented |
| `make seed-dev-admin` | MISSING | Documented in QUICK_START as planned; script `scripts/dev/seed_dev_admin.sh` not present |
| Mock auth fallback | OK | `NUXT_PUBLIC_MOCK_AUTH=true` default |

**Section status:** **PARTIAL**

---

## 7. Frontend web-admin modules

| Page | Exists | Uses API | Fallback | Status |
| ---- | ------ | -------- | -------- | ------ |
| `/login` | Yes | Yes (`useAuth`) | Mock auth + toast | OK |
| `/dashboard` | Yes | Yes (`useApi`) | Silent catch → 0 | PARTIAL |
| `/control-tower` | Yes | Yes (`useControlTower`) | Per-endpoint `emptyResult`, unavailable badges | OK |
| `/health` | Yes | Yes (fetch) | `gatewayUnavailable` empty state | OK |
| `/companies` | Yes | Yes (`useCompanies`) | `isApiUnavailableError` | OK |
| `/companies/[id]` | Yes | Yes | `isApiUnavailableError` | OK |
| `/users` | Yes | Yes (`useUsersApi`) | `isApiUnavailableError` | OK |
| `/users/[id]` | Yes | Yes | `isApiUnavailableError` | OK |
| `/transport-orders` | Yes | Yes (`useApi`) | Silent catch → `[]` | PARTIAL |
| `/transport-orders/[id]` | Yes | Yes | Silent catch → null | PARTIAL |
| `/rfx` | Yes | Yes (`useRfxApi`) | `isApiUnavailableError` | OK |
| `/rfx/[id]` | Yes | Yes | `isApiUnavailableError` | OK |
| `/freight-requests` | Yes | Yes | `isApiUnavailableError` | OK |
| `/freight-requests/[id]` | Yes | Yes | `isApiUnavailableError` | OK |
| `/shipments` | Yes | Yes (`useShipmentsApi`) | `isApiUnavailableError` | OK |
| `/shipments/[id]` | Yes | Yes | `isApiUnavailableError` | OK |
| `/documents` | Yes | Yes (`useDocumentsApi`) | `isApiUnavailableError` | OK |
| `/documents/[id]` | Yes | Yes | `isApiUnavailableError` | OK |
| `/billing-registers` | Yes | Yes (`useApi`) | Silent catch → `[]` | PARTIAL |
| `/billing-registers/[id]` | Yes | Yes | Silent catch → null | PARTIAL |

**Infrastructure:**

| Item | Status |
| ---- | ------ |
| `useApi` | OK |
| `useAuth` | OK |
| Tenant store | OK |
| i18n ru/en/zh | OK (`ru-RU.json`, `en-US.json`, `zh-CN.json`) |
| Navigation menu | OK (includes Control Tower after Dashboard) |

**Section status:** **OK** with minor fallback gaps on dashboard, transport-orders, billing-registers.

---

## 8. Control Tower Dashboard

| Item | Status | Comment |
| ---- | ------ | ------- |
| `pages/control-tower/index.vue` | OK | |
| `components/control-tower/KpiCard.vue` | OK | |
| `components/control-tower/StatusBadge.vue` | OK | |
| `components/control-tower/OperationsStatusTable.vue` | OK | |
| `components/control-tower/FunnelBoard.vue` | OK | |
| `components/control-tower/ShipmentStatusBoard.vue` | OK | |
| `components/control-tower/DocumentsBillingStatus.vue` | OK | Combined documents + billing block |
| `components/control-tower/RiskAlerts.vue` | OK | |
| `components/control-tower/QuickActions.vue` | OK | |
| `components/control-tower/RecentActivity.vue` | OK | |
| `composables/useControlTower.ts` | OK | `Promise.allSettled`, per-endpoint isolation |
| `docs/CONTROL_TOWER.md` | OK | |

### Dashboard sections

| Section | Status |
| ------- | ------ |
| KPI cards | OK |
| Operations status | OK |
| Transport funnel | OK |
| RFx / Tender funnel | OK (bids count TODO = 0) |
| Shipment status board | OK |
| Documents and billing status | OK |
| Alerts / Risks | OK |
| Quick actions | OK |
| Recent activity | OK |
| System links | OK |

**Section status:** **OK**

---

## 9. Observability v0.1

| Item | Status | Comment |
| ---- | ------ | ------- |
| JSON logs | OK | shared-go logging |
| request_id | OK | Middleware on gateway + services |
| access logs | OK | |
| recover middleware | OK | |
| `/metrics` on services | OK | |
| Prometheus optional | OK | `profiles: observability` |
| Grafana optional | OK | `profiles: observability` |
| web-admin health dashboard | OK | `/health` page |
| `docs/OBSERVABILITY.md` | OK | |

### HTTP metrics (expected)

| Metric | Status |
| ------ | ------ |
| `http_requests_total` | OK (in `packages/shared-go/metrics/metrics.go`) |
| `http_request_duration_seconds` | OK |
| `http_in_flight_requests` | OK |

### Makefile

| Command | Status |
| ------- | ------ |
| `make health-check` | OK (defined; runtime blocked) |
| `make ready-check` | OK |
| `make metrics-check` | OK |
| `make observability-up` | OK |

**Section status:** **OK** (runtime verification blocked)

---

## 10. DB query metrics

| Item | Status | Comment |
| ---- | ------ | ------- |
| `packages/shared-go/metrics/metrics.go` | OK | |
| `db_query_duration_seconds` | OK | Labels: service, repository, operation, status |
| company-service instrumentation | OK | repository `measureDB` |
| transport-order-service instrumentation | OK | |
| shipment-service instrumentation | OK | |
| billing-register-service instrumentation | OK | |
| `make generate-db-metrics-traffic` | OK | Python script |
| `make db-metrics-check` | OK | Python script |

**Expected metric lines:** `db_query_duration_seconds_bucket`, `_sum`, `_count` — validated by `scripts/dev/check_db_metrics.py` for 4 services.

**Note:** identity, rfx, document services also instrumented but not in `db-metrics-check` scope.

**Section status:** **OK**

---

## 11. Windows compatibility

### Python scripts (`scripts/dev/`)

| Script | Cross-platform | Notes |
| ------ | -------------- | ----- |
| `check_backend_health.py` | OK | stdlib urllib |
| `check_db_metrics.py` | OK | |
| `check_db_pool_metrics.py` | OK | |
| `generate_db_metrics_traffic.py` | OK | |

**Makefile:** `PYTHON ?= python` on `Windows_NT`; no `grep` in db-metrics targets.

### Runtime on audit host

| Command | Result |
| ------- | ------ |
| `make health-check` | BLOCKED — Python on PATH is Windows Store stub (exit 9009) |
| `make generate-db-metrics-traffic` | BLOCKED — same |
| `make db-metrics-check` | BLOCKED — same |
| `make db-pool-metrics-check` | BLOCKED — same |

**Other Windows notes:**

- `Makefile` uses `SHELL := /bin/bash` — requires Git Bash/GNU Make.
- `make build-web-admin` failed due to Cyrillic path encoding in `CURDIR` when invoking npm; direct `npm run build` from `apps/web-admin` succeeded.
- Integration tests require bash + curl + jq.

**Section status:** **PARTIAL** — scripts are Windows-friendly; tooling/path issues remain.

---

## 12. Performance v0.1

| Item | Status |
| ---- | ------ |
| `tests/performance/k6/smoke.js` | OK |
| `tests/performance/k6/companies-load.js` | OK |
| `tests/performance/k6/transport-orders-load.js` | OK |
| `tests/performance/k6/rfx-load.js` | OK |
| `tests/performance/k6/shipments-load.js` | OK |
| `tests/performance/k6/billing-load.js` | OK |
| `tests/performance/k6/full-flow-load.js` | OK |
| `scripts/performance/run_k6.py` | OK |
| `scripts/performance/analyze_indexes.sql` | OK |
| `docs/PERFORMANCE.md` | OK |
| `docs/PERFORMANCE_REPORT_V0.1.md` | OK |

### Makefile targets

| Command | Status |
| ------- | ------ |
| `make performance-smoke` | OK |
| `make performance-load` | OK |
| `make performance-companies` | OK |
| `make performance-transport-orders` | OK |
| `make performance-rfx` | OK |
| `make performance-shipments` | OK |
| `make performance-billing` | OK |
| `make performance-index-check` | OK |

### Gateway hardening (v0.1)

| Feature | Status |
| ------- | ------ |
| Rate limit | OK |
| Body size limit | OK |
| pprof | PARTIAL (opt-in) |
| Performance indexes | OK (migration 000010 + analyze script) |

**k6 runtime:** SKIPPED — `k6` not installed on audit host.

**Section status:** **OK** (artifacts); runtime **SKIPPED**

---

## 13. Performance v0.2

| Item | Status | Comment |
| ---- | ------ | ------- |
| DB pool config | OK | `packages/shared-go/database/pool_config.go` |
| `DB_MAX_OPEN_CONNS` | OK | |
| `DB_MAX_IDLE_CONNS` | OK | |
| `DB_CONN_MAX_LIFETIME_SECONDS` | OK | |
| `DB_CONN_MAX_IDLE_TIME_SECONDS` | OK | |
| `DB_SLOW_QUERY_THRESHOLD_MS` | OK | In metrics package |
| PostgreSQL slow query log | OK | `log_min_duration_statement=500` in compose |
| `packages/shared-go/metrics/db_pool.go` | OK | |
| `docs/PERFORMANCE_REPORT_V0.2.md` | OK | |

### Pool metrics

| Metric | In code | In `db-pool-metrics-check` |
| ------ | ------- | -------------------------- |
| `db_pool_open_connections` | OK | OK |
| `db_pool_in_use_connections` | OK | OK |
| `db_pool_idle_connections` | OK | OK |
| `db_pool_max_open_connections` | OK | OK |
| `db_pool_wait_count_total` | OK | Not checked by script |
| `db_pool_wait_duration_seconds_total` | OK | Not checked by script |

| Command | Status |
| ------- | ------ |
| `make db-pool-metrics-check` | OK (defined; runtime blocked) |
| `make postgres-logs` | OK |

**Section status:** **PARTIAL** — implementation OK; validation script covers 4 of 6 pool metrics; runtime blocked.

---

## 14. Docker disk troubleshooting

| Item | Status |
| ---- | ------ |
| `docs/DOCKER_DISK_TROUBLESHOOTING.md` | OK |
| `make docker-disk-usage` | OK |
| `make docker-clean-safe` | OK |
| `make docker-volumes` | OK |
| `docker-clean-safe` does not delete volumes | OK — no `docker volume prune` |
| No automatic `docker volume prune` in Makefile | OK |

**Section status:** **OK**

---

## 15. OpenAPI / Swagger

| Item | Status |
| ---- | ------ |
| `packages/openapi/openapi.yaml` | OK |
| `packages/openapi/openapi.json` | OK |
| Per-service YAML files | OK (8 service specs + schemas/) |
| `scripts/openapi/yaml_to_json.py` | OK |
| `scripts/openapi/validate_openapi.py` | OK |
| `scripts/openapi/generate_openapi.py` | OK |
| API Gateway `/docs` | OK |
| API Gateway `/openapi.yaml` | OK |
| API Gateway `/openapi.json` | OK |
| `make openapi-check` | OK |

**Section status:** **OK**

---

## 16. Integration tests

| Item | Status | Comment |
| ---- | ------ | ------- |
| `tests/integration/smoke-test.sh` | OK | Full E2E chain including UPD and billing close |
| `tests/integration/full-flow-smoke-test.sh` | MISSING | Not in repo |
| `tests/integration/README.md` | OK | |
| `tests/integration/env.example` | OK | |
| `tests/integration/payloads/` | OK | |
| `make integration-smoke-test` | OK | Runs `bash tests/integration/smoke-test.sh` |
| `make full-flow-smoke-test` | MISSING | Documented in QUICK_START as planned |

### Full flow chain in `smoke-test.sh`

| Step | Covered |
| ---- | ------- |
| Company | OK |
| User | OK |
| Transport Order | OK |
| Mini Tender / Bid | OK |
| Shipment | OK |
| Document | OK |
| Billing Register | OK |
| UPD | OK |
| Closed (billing `CLOSED`) | OK |
| Shipment `FINANCIALLY_CLOSED` | PARTIAL — noted as TODO in integration README |

**Section status:** **PARTIAL**

---

## 17. Documentation completeness

| Document | Exists | Linked from README.md |
| -------- | ------ | --------------------- |
| `docs/PROJECT_MAP.md` | OK | No |
| `docs/FILE_INDEX.md` | OK | No |
| `docs/DEVELOPER_HANDBOOK.md` | OK | No |
| `docs/QUICK_START.md` | OK | No |
| `docs/TROUBLESHOOTING.md` | OK | No |
| `docs/OBSERVABILITY.md` | OK | Yes |
| `docs/PERFORMANCE.md` | OK | Yes |
| `docs/PERFORMANCE_REPORT_V0.1.md` | OK | Yes |
| `docs/PERFORMANCE_REPORT_V0.2.md` | OK | Yes |
| `docs/DOCKER_DISK_TROUBLESHOOTING.md` | OK | Yes |
| `docs/CONTROL_TOWER.md` | OK | Yes |

**Section status:** **PARTIAL** — all docs exist; README links only a subset.

---

## 18. Build checks

| Check | Result | Notes |
| ----- | ------ | ----- |
| `make platform-build` | SKIPPED | Only postgres running; prior Windows Hyper-V port reservation blocked `:8080`–`:8087`; not re-run during audit |
| `cd apps/web-admin && npm run build` | OK | Build complete via portable Node `.tools/node` v22.14.0 |
| `make build-web-admin` | FAILED | Path encoding issue with Cyrillic username in `CURDIR` |
| `go test ./packages/shared-go/...` | OK | |
| `go test ./services/...` (full) | PARTIAL | Some packages reported `[build failed]` during batch run; individual packages compile when isolated |

**Section status:** **PARTIAL**

---

## 19. Runtime checks

| Command | Result | Notes |
| ------- | ------ | ----- |
| `make platform-up` | BLOCKED BY ENVIRONMENT | Docker disk / WSL / Windows port binding issues from prior sessions |
| `make migrate-up` | PARTIAL | Postgres healthy; full stack not up |
| `make health-check` | BLOCKED | Python stub + backend services down |
| `make generate-db-metrics-traffic` | BLOCKED | Same |
| `make db-metrics-check` | BLOCKED | Same |
| `make db-pool-metrics-check` | BLOCKED | Same |
| `make performance-smoke` | SKIPPED | k6 not installed |
| `docker ps` at audit | Only `freight_postgres` Up (healthy) | |

**Section status:** **BLOCKED BY ENVIRONMENT**

---

## 20. Final audit table

| Area                        | Status                 | Notes |
| --------------------------- | ---------------------- | ----- |
| Project navigation docs     | PARTIAL                | Docs OK; `project-map`, `tree-project`, `find-service`, `find-text` Makefile targets missing |
| Backend services            | OK                     | All 8 services complete |
| Docker Compose              | OK                     | Observability profile correct; runtime only postgres |
| Database migrations         | OK                     | 000001–000010 with up/down |
| API Gateway                 | OK                     | pprof opt-in |
| Auth/RBAC                   | PARTIAL                | No `seed-dev-admin`; no frontend role helpers; API 401 handler partial |
| Web-admin modules           | OK                     | All pages exist; minor fallback gaps |
| Control Tower               | OK                     | v0.1 delivered |
| Observability v0.1          | OK                     | Runtime not verified |
| DB query metrics            | OK                     | 4-service check scripts |
| Windows compatibility       | PARTIAL                | Python scripts OK; PATH/bash/k6/path encoding issues |
| Performance v0.1            | OK                     | k6 runtime skipped |
| Performance v0.2            | PARTIAL                | Pool wait metrics not in check script |
| Docker disk troubleshooting | OK                     | Safe cleanup preserves volumes |
| OpenAPI/Swagger             | OK                     | |
| Integration tests           | PARTIAL                | `smoke-test.sh` OK; `full-flow-smoke-test` missing |
| Documentation               | PARTIAL                | All docs exist; README missing several links |
| Runtime verification        | BLOCKED                | Docker disk / WSL / Windows ports / Python PATH |

---

## 21. Executive summary

### 1. Fully completed

- 8 Go microservices with health/ready/metrics, Dockerfiles, README, compose ports 8080–8087
- Database migrations 000001–000010 (schemas, domains, seeds, performance indexes)
- API Gateway reverse proxy, Swagger, middleware stack (auth, rate limit, body size, logging)
- web-admin: all module pages, i18n ru/en/zh, Control Tower v0.1 dashboard
- Observability artifacts: metrics, optional Prometheus/Grafana, health dashboard, OBSERVABILITY.md
- DB query + pool metrics in shared-go and 4 core services
- Performance k6 suite, PERFORMANCE docs/reports, index analysis
- Docker disk troubleshooting docs and safe cleanup Makefile targets
- OpenAPI package + validation/generation scripts
- Integration smoke test script with full billing/UPD/close flow

### 2. Partially completed

- Project navigation Makefile helpers documented but not implemented
- Auth/RBAC: mock auth works; `seed-dev-admin` and frontend permission helpers missing
- Some web-admin pages use silent API error handling (dashboard, transport-orders, billing)
- Performance v0.2 pool metrics check covers 4/6 metrics
- README links only part of the documentation set
- Windows DX: bash Make, Python PATH, Cyrillic path in `make build-web-admin`
- Integration: single `smoke-test.sh` covers full flow; separate `full-flow-smoke-test` not added

### 3. Not found

- `make project-map`, `make tree-project`, `make find-service`, `make find-text`
- `make seed-dev-admin` and `scripts/dev/seed_dev_admin.sh`
- `tests/integration/full-flow-smoke-test.sh`
- `make full-flow-smoke-test`
- Frontend `hasRole` / `hasPermission` helpers

### 4. Blocked by environment

- **BLOCKED BY ENVIRONMENT: Docker disk / WSL disk image / Windows TCP port reservation (8056–8155)**
- Backend platform not running at audit time (only PostgreSQL)
- Python health/metrics Make targets (Windows Store python stub)
- k6 performance smoke not executed
- `make platform-build` / full runtime verification chain not completed

### 5. Recommended next 5 actions

1. **Restore Docker runtime:** free disk (`make docker-clean-safe`), reset Windows port reservation (`net stop winnat` / `net start winnat` as admin), then `make platform-up && make migrate-up && make health-check`.
2. **Fix Windows tooling:** install Python 3.12 on PATH, use portable Node via `make setup-node`, verify `make health-check` and `make db-pool-metrics-check` pass.
3. **Implement navigation Makefile targets** (`project-map`, `find-service`, `find-text`) or update handbook to remove references — reduces onboarding confusion.
4. **Add `seed-dev-admin` script + Make target** (or document mock-only workflow consistently) so dev login credentials work without manual API calls.
5. **Align integration naming:** either add `full-flow-smoke-test.sh` / `make full-flow-smoke-test` alias to existing `smoke-test.sh`, or remove planned references from QUICK_START.

---

## Audit Fix Pack v0.1

Исправления по результатам аудита (2026-06-19). Без изменения бизнес-логики backend.

| Gap | Fix Status | Notes |
| --- | --- | --- |
| Makefile navigation targets | OK | `project-map`, `tree-project`, `find-service`, `find-text`; bash/Git Bash/WSL — см. TROUBLESHOOTING |
| seed-dev-admin | PARTIAL | `scripts/dev/seed_dev_admin.sh` + `make seed-dev-admin`; runtime не проверен (Docker blocked) |
| frontend role/permission helpers | PARTIAL | `usePermissions.ts`; roles/permissions из `/auth/me` — TODO |
| explicit API unavailable fallback | OK | `ApiUnavailableState.vue`; dashboard, transport-orders, billing-registers |
| README docs links | OK | Раздел Documentation в README.md |
| db-pool-metrics-check 6 metrics | OK | 6 метрик, 7 сервисов (8081–8087) |
| full-flow-smoke-test alias | OK | Wrapper на `smoke-test.sh` |
| Windows Cyrillic path docs | OK | Раздел в TROUBLESHOOTING.md |

---

## Environment Readiness Pack v0.1

Диагностика окружения для Windows/Git Bash/WSL (2026-06-19). Без изменения бизнес-логики.

| Item | Status | Notes |
| --- | --- | --- |
| Python detection | OK | `scripts/dev/check_python.py`, `make python-check`, `make python-check-win` |
| Configurable PYTHON in Makefile | OK | `PYTHON ?= python`; override `make PYTHON="py -3" ...` |
| Docker readiness check | PARTIAL | `scripts/dev/check_docker_readiness.py`; runtime зависит от Docker Desktop |
| Ports check | OK | `scripts/dev/check_ports.py`, `make ports-check` |
| Windows environment docs | OK | `docs/WINDOWS_ENVIRONMENT.md` |
| README environment links | OK | Раздел Environment checks + ссылка на WINDOWS_ENVIRONMENT |
| Troubleshooting updates | OK | Python PATH + Windows port reservation |

---

## Frontend Backend Status UX Pack v0.1

Web-admin UX: mock mode vs real backend (2026-06-19). Без изменения backend бизнес-логики.

| Item | Status | Notes |
| ---- | ------ | ----- |
| Frontend Backend Status UX | OK | Mock mode is explicit; backend offline is visible |

---

*Report generated by static audit. Re-run runtime section after Docker and Windows port issues are resolved.*
