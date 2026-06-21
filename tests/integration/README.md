# Integration Smoke Test

End-to-end smoke test for the Freight Platform business flow:

```
tenant → companies → user → transport order → RFx/bid → shipment → document → billing register → close
```

The script calls all seven backend services via HTTP and verifies the full financial closing cycle.

## Prerequisites

- **bash** (Makefile uses Git Bash / WSL / Linux / macOS)
- **curl**
- **jq**
- **docker** (for PostgreSQL tenant bootstrap)
- All services running locally

### Windows

`make integration-smoke-test` requires **bash**. On Windows install [Git for Windows](https://git-scm.com/download/win) and run from **Git Bash**, or use WSL:

```bash
# Git Bash
bash tests/integration/smoke-test.sh

# WSL
make integration-smoke-test
```

Ensure `jq` and `curl` are available in your shell PATH.

### 1. Start PostgreSQL

```bash
make dev-up
```

### 2. Apply migrations

```bash
make migrate-up
```

### 3. Start services (separate terminals)

```bash
make run-identity-service          # 8081
make run-company-service           # 8082
make run-transport-order-service   # 8083
make run-rfx-service               # 8084
make run-shipment-service          # 8085
make run-document-service          # 8086
make run-billing-register-service  # 8087
```

### 4. Run smoke test

```bash
make integration-smoke-test
```

Or directly:

```bash
bash tests/integration/smoke-test.sh
```

## Configuration

Copy the example env file if you need custom URLs or PostgreSQL settings:

```bash
cp tests/integration/env.example tests/integration/.env
```

| Variable | Default | Description |
|----------|---------|-------------|
| `IDENTITY_SERVICE_URL` | `http://localhost:8081` | identity-service |
| `COMPANY_SERVICE_URL` | `http://localhost:8082` | company-service |
| `TRANSPORT_ORDER_SERVICE_URL` | `http://localhost:8083` | transport-order-service |
| `RFX_SERVICE_URL` | `http://localhost:8084` | rfx-service |
| `SHIPMENT_SERVICE_URL` | `http://localhost:8085` | shipment-service |
| `DOCUMENT_SERVICE_URL` | `http://localhost:8086` | document-service |
| `BILLING_REGISTER_SERVICE_URL` | `http://localhost:8087` | billing-register-service |
| `POSTGRES_CONTAINER` | `freight_postgres` | Docker container name |
| `TENANT_CODE` | `test-tenant` | Tenant code inserted via SQL |
| `SMOKE_RUN_ID` | timestamp | Suffix for unique `TEST-*` numbers |

## What the script does

1. Health check for all 7 services (fails fast with a clear message if a service is down)
2. Create or reuse tenant `test-tenant` in PostgreSQL
3. Create shipper, consignee, carrier companies
4. Create user and link to shipper via membership
5. Create locations, cargo, transport order, submit
6. Create freight request, publish, bid, submit, accept
7. Create driver, vehicle, shipment from bid
8. Assign driver and vehicle, advance shipment to `READY_FOR_BILLING`
9. Create POD document, signing session, mock signature
10. Create billing register, add shipment, calculate, approve
11. Create UPD, mock EDO flow: sent → signed → paid → closed
12. Print `SMOKE TEST PASSED` and all created entity IDs

## Test data conventions

- All business numbers use prefix `TEST` (e.g. `TO-TEST-20260617120000`)
- Email uses run suffix: `logist-{RUN_ID}@test.local` (allows re-runs in the same tenant)
- No real personal data
- Tenant is reused across runs; each run creates new entities with a unique `SMOKE_RUN_ID`

## Payload templates

See `tests/integration/payloads/` for reference JSON bodies used by the scenario.

## Troubleshooting

| Error | Fix |
|-------|-----|
| `Service X is not running on port Y` | Start the service with `make run-*` |
| `jq is required` | Install jq |
| `PostgreSQL container is not running` | Run `make dev-up` |
| Shipment status transition failed | Check shipment-service logs; strict status chain is enforced |
| Document signature failed | User and company must exist in the same tenant |
| Billing item rejected | Shipment must be `READY_FOR_BILLING` or `DOCUMENTS_COMPLETED` |

## Full-flow smoke test alias

```bash
make full-flow-smoke-test
```

`tests/integration/full-flow-smoke-test.sh` is currently an alias wrapper around `smoke-test.sh` and should be expanded later when a dedicated extended scenario is needed.

## Known limitations

- Services are **not** started by the smoke test — you must run them manually
- Services are **not** in Docker Compose yet
- EDO / 1C / payment gateway steps are mocked
- `shipment.status` is not updated to `FINANCIALLY_CLOSED` on register close (TODO in billing-register-service)

## Membership endpoint

`POST /v1/companies/{company_id}/members` is implemented in company-service and included in the smoke test.
