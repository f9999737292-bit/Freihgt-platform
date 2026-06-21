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

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| POST | `/v1/shipments/from-transport-order` | Create shipment from transport order |
| POST | `/v1/shipments/from-bid` | Create shipment from accepted bid |
| GET | `/v1/shipments/{id}` | Get shipment by ID |
| GET | `/v1/shipments` | List shipments |
| POST | `/v1/shipments/{id}/assign-driver` | Assign driver |
| POST | `/v1/shipments/{id}/assign-vehicle` | Assign vehicle |
| POST | `/v1/shipments/{id}/accept` | Carrier accepts shipment |
| PATCH | `/v1/shipments/{id}/status` | Update shipment status |
| POST | `/v1/shipments/{id}/cancel` | Cancel shipment |
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
  -d '{
    "tenant_id": "11111111-1111-1111-1111-111111111111",
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
  -d '{
    "tenant_id": "11111111-1111-1111-1111-111111111111",
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
  -d '{
    "tenant_id": "11111111-1111-1111-1111-111111111111",
    "driver_id": "55555555-5555-5555-5555-555555555555"
  }'
```

Assign vehicle:

```bash
curl -X POST http://localhost:8085/v1/shipments/{shipment_id}/assign-vehicle \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "11111111-1111-1111-1111-111111111111",
    "vehicle_id": "66666666-6666-6666-6666-666666666666"
  }'
```

Update status:

```bash
curl -X PATCH http://localhost:8085/v1/shipments/{shipment_id}/status \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "11111111-1111-1111-1111-111111111111",
    "status": "LOADED",
    "actual_time": "2026-07-01T11:00:00Z"
  }'
```

## Tests

```bash
make test-shipment-service
```
