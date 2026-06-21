# transport-order-service

Transport order management service for the freight platform (`transport.locations`, `transport.cargoes`, `transport.transport_orders`).

## Responsibilities

- Create and manage warehouse/customer locations
- Create cargo with line items
- Create, read, list, and update transport orders
- Submit orders to sourcing (`DRAFT` → `READY_FOR_SOURCING`)
- Cancel orders in allowed statuses

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `TRANSPORT_ORDER_SERVICE_PORT` | `8083` | HTTP port |
| `DATABASE_URL` | local postgres URL | PostgreSQL connection string |
| `LOG_LEVEL` | `info` | Log level |
| `ENVIRONMENT` | `development` | Runtime environment |

## Run locally

From monorepo root:

```bash
make run-transport-order-service
```

Requires PostgreSQL from `make dev-up` and applied migrations.

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| POST | `/v1/locations` | Create location |
| GET | `/v1/locations/{id}` | Get location |
| GET | `/v1/locations` | List locations |
| POST | `/v1/cargoes` | Create cargo |
| GET | `/v1/cargoes/{id}` | Get cargo |
| POST | `/v1/transport-orders` | Create transport order |
| GET | `/v1/transport-orders/{id}` | Get transport order |
| GET | `/v1/transport-orders` | List transport orders |
| PATCH | `/v1/transport-orders/{id}` | Update transport order (DRAFT only) |
| POST | `/v1/transport-orders/{id}/submit` | Submit to sourcing |
| POST | `/v1/transport-orders/{id}/cancel` | Cancel transport order |

## Examples

Create a tenant and companies first via SQL / company-service, then:

### Create location

```bash
curl -X POST http://localhost:8083/v1/locations \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "TENANT_ID",
    "company_id": "COMPANY_ID",
    "location_type": "WAREHOUSE",
    "name": "Склад Москва",
    "country_code": "RU",
    "region": "Московская область",
    "city": "Москва",
    "address_line": "ул. Примерная, 1",
    "postal_code": "101000",
    "lat": 55.7558000,
    "lon": 37.6173000,
    "timezone": "Europe/Moscow"
  }'
```

### Create cargo

```bash
curl -X POST http://localhost:8083/v1/cargoes \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "TENANT_ID",
    "cargo_type": "FMCG",
    "description": "Продукты питания",
    "gross_weight": 20000,
    "net_weight": 19500,
    "volume": 82,
    "temperature_min": 2,
    "temperature_max": 6,
    "dangerous_goods_flag": false,
    "customs_required": false,
    "items": [
      {
        "sku": "SKU-001",
        "name": "Товар 1",
        "quantity": 100,
        "unit": "PALLET",
        "weight": 20000,
        "volume": 82,
        "package_type": "PALLET"
      }
    ]
  }'
```

### Create transport order

```bash
curl -X POST http://localhost:8083/v1/transport-orders \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "TENANT_ID",
    "order_number": "TO-2026-000001",
    "shipper_company_id": "SHIPPER_COMPANY_ID",
    "consignee_company_id": "CONSIGNEE_COMPANY_ID",
    "origin_location_id": "ORIGIN_LOCATION_ID",
    "destination_location_id": "DESTINATION_LOCATION_ID",
    "cargo_id": "CARGO_ID",
    "requested_pickup_date": "2026-07-01",
    "requested_delivery_date": "2026-07-03",
    "transport_mode": "ROAD",
    "equipment_type": "TENT_20T",
    "source_system": "manual",
    "external_reference": "ERP-123"
  }'
```

### Submit transport order

```bash
curl -X POST http://localhost:8083/v1/transport-orders/TRANSPORT_ORDER_ID/submit
```

Health check:

```bash
curl http://localhost:8083/health
```

## Tests

```bash
go test ./...
```

Or from monorepo root:

```bash
make test-transport-order-service
```

## Docker

Build from monorepo root:

```bash
docker build -f services/transport-order-service/Dockerfile -t freight-platform/transport-order-service .
```
