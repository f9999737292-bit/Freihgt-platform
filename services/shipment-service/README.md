# shipment-service

Go microservice for managing freight shipments after a carrier is selected.

## Purpose

`shipment-service` handles the operational side of transport:

- Create shipments from transport orders or accepted bids
- Assign carrier, driver, and vehicle
- Track shipment status through pickup, transit, delivery, documents, and billing readiness
- Manage driver and vehicle master data for carriers

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SHIPMENT_SERVICE_PORT` | `8085` | HTTP port |
| `DATABASE_URL` | local postgres URL | PostgreSQL connection string |
| `LOG_LEVEL` | `info` | Log level (`debug`, `info`, `warn`, `error`) |
| `ENVIRONMENT` | `development` | Runtime environment |
| `SHIPMENT_OUTBOX_ENABLED` | `false` | Enable background outbox publisher worker |
| `SHIPMENT_OUTBOX_TRANSPORT` | _(empty)_ | Required when outbox enabled; no broker transport is implemented in v0.1 |
| `SHIPMENT_OUTBOX_POLL_INTERVAL` | `2s` | Worker poll interval |
| `SHIPMENT_OUTBOX_BATCH_SIZE` | `50` | Claim batch size |
| `SHIPMENT_OUTBOX_PUBLISH_TIMEOUT` | `10s` | Per-event publish timeout |
| `SHIPMENT_OUTBOX_LEASE_TIMEOUT` | `60s` | Claim lease timeout (must exceed publish timeout) |
| `SHIPMENT_OUTBOX_MAX_ATTEMPTS` | `5` | Max publish attempts before `FAILED` |
| `SHIPMENT_OUTBOX_WORKER_ID` | generated | Unique worker instance ID |

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| POST | `/v1/shipments/from-transport-order` | Create shipment from transport order |
| POST | `/v1/shipments/from-bid` | Create shipment from accepted bid |
| GET | `/v1/shipments/{id}` | Get shipment by ID (tenant-scoped) |
| GET | `/v1/shipments` | List shipments |
| POST | `/v1/shipments/{id}/assign-driver` | Assign driver |
| POST | `/v1/shipments/{id}/assign-vehicle` | Assign vehicle |
| POST | `/v1/shipments/{id}/accept` | Carrier accepts shipment |
| PATCH | `/v1/shipments/{id}/status` | Update shipment status |
| POST | `/v1/shipments/{id}/cancel` | Cancel shipment |
| GET | `/internal/v1/shipments/{shipmentId}/status-history` | Internal: list persisted status transitions (tenant via `X-Tenant-ID`) |
| POST | `/v1/drivers` | Create driver |
| GET | `/v1/drivers/{id}` | Get driver |
| GET | `/v1/drivers` | List drivers |
| POST | `/v1/vehicles` | Create vehicle |
| GET | `/v1/vehicles/{id}` | Get vehicle |
| GET | `/v1/vehicles` | List vehicles |

## Run locally

```bash
make dev-up
make migrate-up
make run-shipment-service
```

Health check:

```bash
curl http://localhost:8085/health
```

## PostgreSQL outbox integration tests

Requires live PostgreSQL (see `docs/SHIPMENT_STATUS_OUTBOX.md`):

```bash
# PowerShell example — use your local Compose credentials
$env:TEST_DATABASE_URL = "postgres://freight:freight_password@localhost:5432/postgres?sslmode=disable"
make outbox-integration-test
```

Tests create and drop an isolated `freight_platform_outbox_test_*` database automatically.

## Examples

Create driver:

```bash
curl -X POST http://localhost:8085/v1/drivers \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "11111111-1111-1111-1111-111111111111",
    "carrier_company_id": "22222222-2222-2222-2222-222222222222",
    "full_name": "Иван Водитель",
    "phone": "+79990000000",
    "license_number": "77AA123456",
    "license_country": "RU",
    "preferred_locale": "ru-RU"
  }'
```

Create vehicle:

```bash
curl -X POST http://localhost:8085/v1/vehicles \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "11111111-1111-1111-1111-111111111111",
    "carrier_company_id": "22222222-2222-2222-2222-222222222222",
    "plate_number": "А123ВС777",
    "vehicle_type": "TRUCK",
    "equipment_type": "TENT_20T",
    "capacity_weight": 20000,
    "capacity_volume": 82,
    "registration_country": "RU"
  }'
```

Create shipment from transport order:

```bash
curl -X POST http://localhost:8085/v1/shipments/from-transport-order \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: 11111111-1111-1111-1111-111111111111" \
  -H "X-User-ID: 22222222-2222-2222-2222-222222222222" \
  -d '{
    "shipment_number": "SH-2026-000001",
    "transport_order_id": "33333333-3333-3333-3333-333333333333",
    "carrier_company_id": "22222222-2222-2222-2222-222222222222",
    "planned_pickup_at": "2026-07-01T09:00:00Z",
    "planned_delivery_at": "2026-07-03T18:00:00Z"
  }'
```

Create shipment from accepted bid:

```bash
curl -X POST http://localhost:8085/v1/shipments/from-bid \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: 11111111-1111-1111-1111-111111111111" \
  -H "X-User-ID: 22222222-2222-2222-2222-222222222222" \
  -d '{
    "shipment_number": "SH-2026-000002",
    "bid_id": "44444444-4444-4444-4444-444444444444",
    "transport_order_id": "33333333-3333-3333-3333-333333333333",
    "planned_pickup_at": "2026-07-01T09:00:00Z",
    "planned_delivery_at": "2026-07-03T18:00:00Z"
  }'
```

Assign driver:

```bash
curl -X POST http://localhost:8085/v1/shipments/{shipment_id}/assign-driver \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: 11111111-1111-1111-1111-111111111111" \
  -d '{
    "driver_id": "55555555-5555-5555-5555-555555555555"
  }'
```

Assign vehicle:

```bash
curl -X POST http://localhost:8085/v1/shipments/{shipment_id}/assign-vehicle \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: 11111111-1111-1111-1111-111111111111" \
  -d '{
    "vehicle_id": "66666666-6666-6666-6666-666666666666"
  }'
```

Update status:

```bash
curl -X PATCH http://localhost:8085/v1/shipments/{shipment_id}/status \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: 11111111-1111-1111-1111-111111111111" \
  -H "X-User-ID: 22222222-2222-2222-2222-222222222222" \
  -d '{
    "status": "LOADED",
    "actual_time": "2026-07-01T11:00:00Z"
  }'
```

## Shipment mutation trust boundary

**Пользовательские операции Create, Accept, UpdateStatus и Cancel не принимают tenant_id как источник области данных. Tenant определяется только из verified gateway context и используется одинаково для shipment mutation и shipment_status_history.**

- Tenant and actor are not selected via request body or query on user-facing mutations.
- Handlers resolve verified `X-Tenant-ID` and `X-User-ID` (gateway-set from JWT) before JSON decode and before any repository transaction.
- Spoofed `tenant_id` in mutation body returns `400` (strict JSON).
- Missing trusted tenant returns `401` before body parsing.
- Foreign and unknown shipments or source entities return indistinguishable `404`.
- Status updates and `shipment_status_history.tenant_id` always match the verified tenant within one transaction.
- Production clients must use API Gateway; direct shipment-service mutation access from untrusted networks is prohibited.

Gateway route-level RBAC for Create, Accept, UpdateStatus, and Cancel is enforced in API Gateway before proxying. See [SHIPMENT_MUTATION_AUTHORIZATION.md](../../docs/SHIPMENT_MUTATION_AUTHORIZATION.md).

## Tenant isolation (GET /v1/shipments/{id})

See also: [Driver & Vehicle tenant isolation](../../docs/DRIVER_VEHICLE_TENANT_ISOLATION.md).

Shipment detail lookup is tenant-scoped at the repository layer:

```sql
WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
```

Security invariants:

- A single repository query loads shipment by `shipment_id` and verified `tenant_id`.
- Post-fetch tenant comparison is not used as the primary isolation mechanism.
- Missing and foreign-tenant shipments both return `404 Not Found` with the same error shape.
- A second lookup by shipment ID only (to detect foreign tenants) is forbidden.
- **`GET /v1/shipments/{id}` does not accept `tenant_id` from query.** Tenant is supplied only via trusted `X-Tenant-ID` header set by API Gateway.
- Requests without verified tenant return `401 Unauthorized` and do not hit the repository.

## Network trust boundary

In production, external requests must enter only through API Gateway (`8080`). Shipment-service must not have a public ingress or public LoadBalancer.

Local `docker-compose` publishes `8085:8085` for **development only** (direct health checks, local debugging). This is not a production trust model.

Gateway defense-in-depth comparison of `tenant_id` in downstream responses remains in Shipment Event History.

## Driver & Vehicle detail endpoints

`GET /v1/drivers/{id}` and `GET /v1/vehicles/{id}` use the same trust model as shipment detail:

- Tenant from trusted `X-Tenant-ID` header only (no query fallback)
- Repository lookup: `WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`
- Foreign tenant object returns the same `404` as missing object

## Assign driver / assign vehicle

`POST /v1/shipments/{id}/assign-driver` and `POST /v1/shipments/{id}/assign-vehicle` do **not** accept `tenant_id` in the JSON body. Tenant is derived only from trusted `X-Tenant-ID` (same boundary as detail endpoints).

Request body contains only `driver_id` or `vehicle_id`. Unknown `tenant_id` field in body returns `400 Bad Request`.

## Create / list driver / vehicle

`POST /v1/drivers`, `POST /v1/vehicles`, `GET /v1/drivers`, and `GET /v1/vehicles` derive tenant only from trusted `X-Tenant-ID`.

- Create body must not contain `tenant_id` (strict JSON → `400` if present)
- List must not use `tenant_id` query; conflicting query tenant is ignored
- Missing trusted tenant → `401`, repository not called

## Authorization vs tenant isolation

API Gateway performs authentication and route-level authorization for Driver, Vehicle, and fleet assignment routes. Shipment-service performs tenant isolation at handler, service, and SQL repository layers.

| Layer | Responsibility |
|---|---|
| API Gateway | JWT validation, role check via `GET /v1/auth/me`, route allowlists |
| Shipment-service | Trusted `X-Tenant-ID`, tenant-scoped SQL, `404` for foreign resources |

Tenant-scoped SQL does not replace checking whether the authenticated user is allowed to perform the operation. Frontend role restrictions are UX only.

See [Driver & Vehicle tenant isolation](../../docs/DRIVER_VEHICLE_TENANT_ISOLATION.md) for gateway role allowlists and HTTP semantics.

Create driver example:

```bash
curl -X POST http://localhost:8085/v1/drivers \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: 11111111-1111-1111-1111-111111111111" \
  -d '{
    "carrier_company_id": "22222222-2222-2222-2222-222222222222",
    "full_name": "Иван Иванов",
    "license_country": "RU",
    "preferred_locale": "ru-RU"
  }'
```

List drivers example:

```bash
curl -X GET "http://localhost:8085/v1/drivers?limit=20&offset=0" \
  -H "X-Tenant-ID: 11111111-1111-1111-1111-111111111111"
```

## Status history

Persisted status transitions are stored in `transport.shipment_status_history` and exposed internally at `GET /internal/v1/shipments/{shipmentId}/status-history`. See [docs/SHIPMENT_STATUS_HISTORY.md](../../docs/SHIPMENT_STATUS_HISTORY.md).

Each successful status mutation writes a history row in the same PostgreSQL transaction as the shipment update. Assign driver/vehicle without status change does not create history rows.

User-facing mutation handlers require verified `X-User-ID` from API Gateway. Missing user context returns `401` before any repository call; malformed or zero UUID returns `400`. See [docs/SHIPMENT_STATUS_HISTORY.md](../../docs/SHIPMENT_STATUS_HISTORY.md) (Actor provenance).

## Tests

```bash
make test-shipment-service
```
