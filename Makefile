# Prefer Git Bash on Windows (GnuWin32 make does not resolve /bin/bash).
ifeq ($(wildcard C:/Program Files/Git/bin/bash.exe),)
SHELL := /bin/bash
else
SHELL := "C:/Program Files/Git/bin/bash.exe"
endif

include .env
export

COMPOSE_FILE ?= infrastructure/docker-compose/docker-compose.yml
COMPOSE=docker compose -f $(COMPOSE_FILE)

# Backend containers started by platform-up (serial build uses this list).
BACKEND_SERVICES := \
	api-gateway \
	identity-service \
	company-service \
	transport-order-service \
	rfx-service \
	shipment-service \
	document-service \
	billing-register-service
PYTHON ?= python
MIGRATIONS_PATH=infrastructure/migrations
POSTGRES_USER ?= freight
POSTGRES_PASSWORD ?= freight_password
POSTGRES_DB ?= freight_platform
POSTGRES_PORT ?= 5432
# In-container URL (compose network). Used by migrate service.
MIGRATE_DB_URL=postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@postgres:5432/$(POSTGRES_DB)?sslmode=disable
# Host URL for tools running outside compose (e.g. local psql).
DB_URL?=postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@localhost:$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=disable
MIGRATE_RUN=$(COMPOSE) --profile tools run --rm migrate

GO_SERVICES := api-gateway identity-service company-service localization-service \
	transport-order-service shipment-service rfx-service document-service billing-register-service

K6 ?= k6

.PHONY: help env-init dev-up dev-down dev-restart dev-logs ps db-shell db-check \
	migrate-up migrate-down migrate-version migrate-force migrate-drop clean \
	platform-build platform-build-serial platform-build-service \
	platform-up platform-up-no-build platform-up-safe platform-up-backend-only \
	platform-down platform-restart platform-logs platform-ps platform-health \
	observability-up observability-down observability-logs metrics-check health-check ready-check \
	db-metrics-check generate-db-metrics-traffic db-pool-metrics-check postgres-logs \
	python-check python-check-win docker-readiness ports-check \
	docker-disk-usage docker-clean-safe docker-volumes \
	performance-smoke performance-load performance-companies performance-transport-orders \
	performance-rfx performance-shipments performance-billing performance-index-check \
	go-build go-test \
	run-api-gateway run-identity-service run-company-service run-localization-service \
	run-transport-order-service run-shipment-service run-rfx-service \
	run-document-service run-billing-register-service \
	test-company-service test-identity-service test-transport-order-service test-rfx-service test-shipment-service test-document-service test-billing-register-service test-api-gateway \
	integration-smoke-test full-flow-smoke-test seed-dev-admin seed-demo-data \
	project-map tree-project find-service find-text \
	openapi-generate openapi-generate-json openapi-validate openapi-check api-docs-open \
	install-web-admin run-web-admin build-web-admin test-web-admin setup-node

help:
	@echo "Infrastructure:"
	@echo "  make env-init          Create .env from .env.example"
	@echo "  make dev-up            Start local infrastructure"
	@echo "  make dev-down          Stop local infrastructure"
	@echo "  make migrate-up        Apply all migrations"
	@echo "  make db-check          Check schemas and tables"
	@echo ""
	@echo "Platform (Docker Compose):"
	@echo "  make platform-build    Build all backend container images (parallel)"
	@echo "  make platform-up       Start PostgreSQL + backend services (parallel build; fast mode)"
	@echo "  make platform-up-safe  Windows/WSL safe: serial build then start (no rebuild)"
	@echo "  make platform-build-serial  Build backend images one service at a time"
	@echo "  make platform-build-service SERVICE=name  Build one backend service"
	@echo "  make platform-up-no-build   Start platform without rebuilding images"
	@echo "  make platform-up-backend-only  Start postgres + backend only (no rebuild)"
	@echo "  make platform-down     Stop platform containers"
	@echo "  make platform-restart  Restart platform containers"
	@echo "  make platform-logs     Follow platform logs"
	@echo "  make platform-ps       Show platform container status"
	@echo "  make platform-health   Curl health endpoints for all services"
	@echo ""
	@echo "Observability:"
	@echo "  make observability-up  Start Prometheus + Grafana"
	@echo "  make observability-down Stop Prometheus + Grafana"
	@echo "  make observability-logs Follow Prometheus/Grafana logs"
	@echo "  make metrics-check     Verify /metrics on all services"
	@echo "  make health-check      Python /health check on all services"
	@echo "  make ready-check       Curl API Gateway /ready"
	@echo "  make db-metrics-check  Python check for db_query_duration_seconds"
	@echo "  make db-pool-metrics-check Python check for db_pool_* metrics"
	@echo "  make postgres-logs       Follow PostgreSQL container logs"
	@echo ""
	@echo "Environment checks:"
	@echo "  make python-check        Verify Python >= 3.10"
	@echo "  make python-check-win    Windows: py -3 scripts/dev/check_python.py"
	@echo "  make docker-readiness    Docker CLI/daemon/disk readiness"
	@echo "  make ports-check         Check platform port availability"
	@echo "  make health-check PYTHON=\"py -3\"   Override Python on Windows"
	@echo ""
	@echo "Docker disk:"
	@echo "  make docker-disk-usage   Show Docker disk usage (docker system df)"
	@echo "  make docker-clean-safe   Safe cleanup (no volume prune)"
	@echo "  make docker-volumes      List Docker volumes"
	@echo ""
	@echo "Performance:"
	@echo "  make performance-smoke           k6 smoke test (health + list APIs)"
	@echo "  make performance-load            k6 full-flow load test"
	@echo "  make performance-companies       k6 companies load test"
	@echo "  make performance-transport-orders k6 transport orders load test"
	@echo "  make performance-rfx             k6 RFX load test"
	@echo "  make performance-shipments       k6 shipments load test"
	@echo "  make performance-billing         k6 billing load test"
	@echo "  make performance-index-check     PostgreSQL index analysis"
	@echo ""
	@echo "Backend:"
	@echo "  make go-build          Build all Go services"
	@echo "  make go-test           Run Go tests"
	@echo "  make run-api-gateway   Run api-gateway (port 8080)"
	@echo "  make run-identity-service        Run identity-service (8081)"
	@echo "  make run-company-service         Run company-service (8082)"
	@echo "  make run-localization-service    Run localization-service (8083)"
	@echo "  make run-transport-order-service Run transport-order-service (8083)"
	@echo "  make run-shipment-service        Run shipment-service (8085)"
	@echo "  make run-rfx-service             Run rfx-service (8084)"
	@echo "  make run-document-service        Run document-service (8086)"
	@echo "  make run-billing-register-service Run billing-register-service (8087)"
	@echo ""
	@echo "Integration:"
	@echo "  make integration-smoke-test   Run end-to-end smoke test (all services must be up)"
	@echo "  make seed-dev-admin           Create dev tenant admin (idempotent)"
	@echo "  make seed-demo-data           Create demo UI data for dev tenant (idempotent)"
	@echo ""
	@echo "API Documentation:"
	@echo "  make openapi-check       Validate OpenAPI and regenerate openapi.json"
	@echo "  make api-docs-open       Print Swagger UI URL"
	@echo ""
	@echo "Frontend Admin:"
	@echo "  make setup-node          Download portable Node.js to .tools/node (Windows)"
	@echo "  make install-web-admin   Install web-admin dependencies"
	@echo "  make run-web-admin       Run web-admin (port 3000)"
	@echo "  make build-web-admin     Build web-admin for production"
	@echo "  make test-web-admin      Lint web-admin"

openapi-generate:
	python scripts/openapi/generate_openapi.py || (cd scripts/openapi && go run ./cmd/generate/)

openapi-generate-json:
	python scripts/openapi/yaml_to_json.py packages/openapi/openapi.yaml packages/openapi/openapi.json || (cd scripts/openapi && go run ./cmd/yamltojson ../../packages/openapi/openapi.yaml ../../packages/openapi/openapi.json)

openapi-validate:
	python scripts/openapi/validate_openapi.py packages/openapi/openapi.yaml || (cd scripts/openapi && go run ./cmd/validate ../../packages/openapi/openapi.yaml)

openapi-check: openapi-validate openapi-generate-json

api-docs-open:
	@echo "Open http://localhost:8080/docs"

env-init:
	@if [ ! -f .env ]; then cp .env.example .env; echo ".env created"; else echo ".env already exists"; fi

dev-up:
	$(COMPOSE) up -d postgres

dev-down:
	$(COMPOSE) down

dev-restart: dev-down dev-up

dev-logs:
	$(COMPOSE) logs -f

ps:
	$(COMPOSE) ps

db-shell:
	docker exec -it freight_postgres psql -U $(POSTGRES_USER) -d $(POSTGRES_DB)

db-check:
	docker exec -i freight_postgres psql -U $(POSTGRES_USER) -d $(POSTGRES_DB) -c "SELECT schema_name FROM information_schema.schemata WHERE schema_name IN ('core','transport','rfx','documents','billing') ORDER BY schema_name;"
	docker exec -i freight_postgres psql -U $(POSTGRES_USER) -d $(POSTGRES_DB) -c "SELECT table_schema, table_name FROM information_schema.tables WHERE table_schema IN ('core','transport','rfx','documents','billing') ORDER BY table_schema, table_name;"

migrate-up: dev-up
	$(MIGRATE_RUN) -path=/migrations -database "$(MIGRATE_DB_URL)" up

migrate-down:
	$(MIGRATE_RUN) -path=/migrations -database "$(MIGRATE_DB_URL)" down 1

migrate-version:
	$(MIGRATE_RUN) -path=/migrations -database "$(MIGRATE_DB_URL)" version

migrate-force:
	@if [ -z "$(version)" ]; then echo "Usage: make migrate-force version=N"; exit 1; fi
	$(MIGRATE_RUN) -path=/migrations -database "$(MIGRATE_DB_URL)" force $(version)

migrate-drop:
	$(MIGRATE_RUN) -path=/migrations -database "$(MIGRATE_DB_URL)" drop -f

clean:
	$(COMPOSE) down -v

platform-build:
	$(COMPOSE) build

# platform-up — fast mode: parallel compose build + start (may crash Docker/WSL on Windows).
platform-up:
	$(COMPOSE) up -d --build

# platform-up-safe — recommended on Windows/WSL when parallel build causes EOF or 0xc00000fd.
platform-build-serial:
	@echo "Building backend services sequentially to avoid Docker/WSL parallel build crashes..."
	@$(MAKE) platform-build-service SERVICE=api-gateway
	@$(MAKE) platform-build-service SERVICE=identity-service
	@$(MAKE) platform-build-service SERVICE=company-service
	@$(MAKE) platform-build-service SERVICE=transport-order-service
	@$(MAKE) platform-build-service SERVICE=rfx-service
	@$(MAKE) platform-build-service SERVICE=shipment-service
	@$(MAKE) platform-build-service SERVICE=document-service
	@$(MAKE) platform-build-service SERVICE=billing-register-service
	@echo "Serial build completed"

platform-up-no-build:
	$(COMPOSE) up -d --no-build

platform-up-safe: platform-build-serial platform-up-no-build

platform-build-service:
	@echo "Usage: make platform-build-service SERVICE=document-service"
ifeq ($(strip $(SERVICE)),)
	@echo "SERVICE is required"
	@exit 1
endif
	$(COMPOSE) build --progress=plain $(SERVICE)

platform-up-backend-only:
	$(COMPOSE) up -d --no-build postgres $(BACKEND_SERVICES)

platform-down:
	$(COMPOSE) down

platform-restart: platform-down
	$(COMPOSE) up -d --build

platform-logs:
	$(COMPOSE) logs -f

platform-ps:
	$(COMPOSE) ps

platform-health:
	@curl -s http://localhost:8080/health && echo
	@curl -s http://localhost:8080/ready && echo
	@curl -s http://localhost:8081/health && echo
	@curl -s http://localhost:8082/health && echo
	@curl -s http://localhost:8083/health && echo
	@curl -s http://localhost:8084/health && echo
	@curl -s http://localhost:8085/health && echo
	@curl -s http://localhost:8086/health && echo
	@curl -s http://localhost:8087/health && echo

observability-up:
	$(COMPOSE) --profile observability up -d prometheus grafana

observability-down:
	$(COMPOSE) --profile observability stop prometheus grafana

observability-logs:
	$(COMPOSE) --profile observability logs -f prometheus grafana

metrics-check:
	@echo "Checking metrics endpoints..."
	@curl -sf http://localhost:8080/metrics >/dev/null
	@curl -sf http://localhost:8081/metrics >/dev/null
	@curl -sf http://localhost:8082/metrics >/dev/null
	@curl -sf http://localhost:8083/metrics >/dev/null
	@curl -sf http://localhost:8084/metrics >/dev/null
	@curl -sf http://localhost:8085/metrics >/dev/null
	@curl -sf http://localhost:8086/metrics >/dev/null
	@curl -sf http://localhost:8087/metrics >/dev/null
	@echo "Metrics OK"

health-check:
	$(PYTHON) scripts/dev/check_backend_health.py

ready-check:
	@echo "Checking readiness..."
	@curl -s http://localhost:8080/ready && echo

db-metrics-check:
	$(PYTHON) scripts/dev/check_db_metrics.py

db-pool-metrics-check:
	$(PYTHON) scripts/dev/check_db_pool_metrics.py

python-check:
	$(PYTHON) scripts/dev/check_python.py

python-check-win:
	py -3 scripts/dev/check_python.py

docker-readiness:
	$(PYTHON) scripts/dev/check_docker_readiness.py

ports-check:
	$(PYTHON) scripts/dev/check_ports.py

postgres-logs:
	$(COMPOSE) logs -f postgres

# Safe cleanup: does NOT remove volumes (PostgreSQL data preserved).
docker-disk-usage:
	docker system df

docker-clean-safe:
	docker builder prune -af
	docker container prune -f
	docker image prune -af
	docker network prune -f

docker-volumes:
	docker volume ls

generate-db-metrics-traffic:
	$(PYTHON) scripts/dev/generate_db_metrics_traffic.py

define k6-run
	$(PYTHON) scripts/performance/run_k6.py $(1)
endef

performance-smoke:
	$(call k6-run,tests/performance/k6/smoke.js)

performance-load:
	$(call k6-run,tests/performance/k6/full-flow-load.js)

performance-companies:
	$(call k6-run,tests/performance/k6/companies-load.js)

performance-transport-orders:
	$(call k6-run,tests/performance/k6/transport-orders-load.js)

performance-rfx:
	$(call k6-run,tests/performance/k6/rfx-load.js)

performance-shipments:
	$(call k6-run,tests/performance/k6/shipments-load.js)

performance-billing:
	$(call k6-run,tests/performance/k6/billing-load.js)

performance-index-check:
	docker exec -i freight_postgres psql -U $(POSTGRES_USER) -d $(POSTGRES_DB) < scripts/performance/analyze_indexes.sql

go-build:
	@echo Building api-gateway...
	@go build -o services/api-gateway/bin/server ./services/api-gateway/cmd/server
	@echo Building identity-service...
	@go build -o services/identity-service/bin/server ./services/identity-service/cmd/server
	@echo Building company-service...
	@go build -o services/company-service/bin/server ./services/company-service/cmd/server
	@echo Building localization-service...
	@go build -o services/localization-service/bin/server ./services/localization-service/cmd/server
	@echo Building transport-order-service...
	@go build -o services/transport-order-service/bin/server ./services/transport-order-service/cmd/server
	@echo Building shipment-service...
	@go build -o services/shipment-service/bin/server ./services/shipment-service/cmd/server
	@echo Building rfx-service...
	@go build -o services/rfx-service/bin/server ./services/rfx-service/cmd/server
	@echo Building document-service...
	@go build -o services/document-service/bin/server ./services/document-service/cmd/server
	@echo Building billing-register-service...
	@go build -o services/billing-register-service/bin/server ./services/billing-register-service/cmd/server

go-test:
	@go test ./packages/shared-go/... \
		./services/api-gateway/... \
		./services/identity-service/... \
		./services/company-service/... \
		./services/localization-service/... \
		./services/transport-order-service/... \
		./services/shipment-service/... \
		./services/rfx-service/... \
		./services/document-service/... \
		./services/billing-register-service/...

run-api-gateway:
	@cd services/api-gateway && go run ./cmd/server

test-api-gateway:
	@cd services/api-gateway && go test ./...

run-identity-service:
	@cd services/identity-service && go run ./cmd/server

test-identity-service:
	@cd services/identity-service && go test ./...

run-company-service:
	@cd services/company-service && go run ./cmd/server

test-company-service:
	@cd services/company-service && go test ./...

run-localization-service:
	@cd services/localization-service && go run ./cmd/server

run-transport-order-service:
	@cd services/transport-order-service && go run ./cmd/server

test-transport-order-service:
	@cd services/transport-order-service && go test ./...

run-shipment-service:
	@cd services/shipment-service && go run ./cmd/server

test-shipment-service:
	@cd services/shipment-service && go test ./...

run-rfx-service:
	@cd services/rfx-service && go run ./cmd/server

test-rfx-service:
	@cd services/rfx-service && go test ./...

run-document-service:
	@cd services/document-service && go run ./cmd/server

test-document-service:
	@cd services/document-service && go test ./...

run-billing-register-service:
	@cd services/billing-register-service && go run ./cmd/server

test-billing-register-service:
	@cd services/billing-register-service && go test ./...

integration-smoke-test:
	bash tests/integration/smoke-test.sh

full-flow-smoke-test:
	bash tests/integration/full-flow-smoke-test.sh

seed-dev-admin:
	bash scripts/dev/seed_dev_admin.sh

seed-demo-data:
	bash scripts/dev/seed_demo_data.sh

project-map:
	@echo "Project documentation:"
	@echo " - docs/PROJECT_MAP.md"
	@echo " - docs/FILE_INDEX.md"
	@echo " - docs/DEVELOPER_HANDBOOK.md"
	@echo " - docs/QUICK_START.md"
	@echo " - docs/TROUBLESHOOTING.md"
	@echo " - docs/PROJECT_AUDIT_REPORT_V0.1.md"

tree-project:
	@echo "Project structure:"
	@find . -maxdepth 3 -type d \
		-not -path "./.git*" \
		-not -path "./node_modules*" \
		-not -path "./apps/web-admin/node_modules*" \
		-not -path "./apps/web-admin/.nuxt*" \
		| sort

find-service:
	@echo "Usage: make find-service NAME=company"
	@if [ -z "$(NAME)" ]; then echo "NAME is required"; exit 1; fi
	@find services -iname "*$(NAME)*" -o -path "*/$(NAME)-service/*"

find-text:
	@echo "Usage: make find-text TEXT=READY_FOR_BILLING"
	@if [ -z "$(TEXT)" ]; then echo "TEXT is required"; exit 1; fi
	@grep -R "$(TEXT)" . --exclude-dir=node_modules --exclude-dir=.git --exclude-dir=dist --exclude-dir=.nuxt || true

# Prefer portable Node.js in .tools/node when present (Windows-friendly).
NPM := npm
NODE_DIR := $(CURDIR)/.tools/node
ifeq ($(OS),Windows_NT)
  ifneq (,$(wildcard .tools/node/npm.cmd))
    NPM := "$(NODE_DIR)/npm.cmd"
    export PATH := $(NODE_DIR):$(PATH)
  endif
endif

setup-node:
	powershell -ExecutionPolicy Bypass -File scripts/setup-node.ps1

install-web-admin:
	cd apps/web-admin && $(NPM) install

run-web-admin:
	cd apps/web-admin && $(NPM) run dev

build-web-admin:
	cd apps/web-admin && $(NPM) run build

test-web-admin:
	cd apps/web-admin && $(NPM) run lint
