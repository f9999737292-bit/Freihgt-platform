# rfx-service

RFx, freight request, and bid management service for the freight platform.

## Responsibilities

- RFx events (RFI, RFQ, RFP, tenders): create, list, update, publish, cancel
- Lots, lanes, participants, and responses
- Freight requests linked to transport orders
- Carrier bids with VAT calculation, submit, and accept

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `RFX_SERVICE_PORT` | `8084` | HTTP port |
| `DATABASE_URL` | local postgres URL | PostgreSQL connection string |
| `LOG_LEVEL` | `info` | Log level |
| `ENVIRONMENT` | `development` | Runtime environment |

## Run locally

```bash
make run-rfx-service
```

Requires PostgreSQL from `make dev-up` and applied migrations.

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| POST | `/v1/rfx-events` | Create RFx event |
| GET | `/v1/rfx-events/{id}` | Get RFx event |
| GET | `/v1/rfx-events` | List RFx events |
| PATCH | `/v1/rfx-events/{id}` | Update RFx (DRAFT only) |
| POST | `/v1/rfx-events/{id}/publish` | Publish RFx |
| POST | `/v1/rfx-events/{id}/cancel` | Cancel RFx |
| POST | `/v1/rfx-events/{id}/lots` | Create lot |
| GET | `/v1/rfx-events/{id}/lots` | List lots |
| POST | `/v1/rfx-lots/{lot_id}/lanes` | Create lane |
| POST | `/v1/rfx-events/{id}/participants` | Add participant |
| GET | `/v1/rfx-events/{id}/participants` | List participants |
| POST | `/v1/rfx-events/{id}/responses` | Create response |
| POST | `/v1/rfx-responses/{response_id}/submit` | Submit response |
| POST | `/v1/freight-requests/from-transport-order` | Create freight request |
| POST | `/v1/freight-requests/{id}/publish` | Publish freight request |
| GET | `/v1/freight-requests/{id}` | Get freight request |
| GET | `/v1/freight-requests` | List freight requests |
| POST | `/v1/freight-requests/{id}/bids` | Create bid |
| GET | `/v1/freight-requests/{id}/bids` | List bids |
| POST | `/v1/bids/{id}/submit` | Submit bid |
| POST | `/v1/bids/{id}/accept` | Accept bid |

## Examples

### Create RFQ

```bash
curl -X POST http://localhost:8084/v1/rfx-events \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "TENANT_ID",
    "rfx_number": "RFX-2026-000001",
    "rfx_type": "RFQ",
    "category": "FREIGHT",
    "title": "Закупка перевозок Москва — Казань",
    "description": "Нужно получить ставки перевозчиков на июль 2026",
    "owner_company_id": "COMPANY_ID",
    "currency_code": "RUB",
    "valid_from": "2026-07-01",
    "valid_to": "2026-07-31",
    "response_deadline": "2026-12-31T18:00:00Z"
  }'
```

### Publish RFx

```bash
curl -X POST "http://localhost:8084/v1/rfx-events/RFX_EVENT_ID/publish?tenant_id=TENANT_ID"
```

### Add participant

```bash
curl -X POST http://localhost:8084/v1/rfx-events/RFX_EVENT_ID/participants \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "TENANT_ID",
    "company_id": "CARRIER_COMPANY_ID",
    "participant_type": "CARRIER"
  }'
```

### Create freight request from transport order

```bash
curl -X POST http://localhost:8084/v1/freight-requests/from-transport-order \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "TENANT_ID",
    "transport_order_id": "TRANSPORT_ORDER_ID",
    "freight_request_number": "FR-2026-000001",
    "request_type": "MINI_TENDER",
    "shipper_company_id": "SHIPPER_COMPANY_ID",
    "response_deadline": "2026-12-31T18:00:00Z",
    "currency_code": "RUB"
  }'
```

### Create bid

```bash
curl -X POST http://localhost:8084/v1/freight-requests/FREIGHT_REQUEST_ID/bids \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "TENANT_ID",
    "carrier_company_id": "CARRIER_COMPANY_ID",
    "bid_number": "BID-2026-000001",
    "currency_code": "RUB",
    "vat_rate": 20,
    "valid_until": "2026-12-31T18:00:00Z",
    "items": [{
      "description": "Москва — Казань, тент 20 тонн",
      "base_amount": 100000,
      "fuel_surcharge": 5000,
      "toll_amount": 3000,
      "extra_charges": 0,
      "vat_rate": 20,
      "comment": "Цена за один рейс"
    }]
  }'
```

### Submit bid

```bash
curl -X POST "http://localhost:8084/v1/bids/BID_ID/submit?tenant_id=TENANT_ID"
```

### Accept bid

```bash
curl -X POST "http://localhost:8084/v1/bids/BID_ID/accept?tenant_id=TENANT_ID"
```

Health check:

```bash
curl http://localhost:8084/health
```

## Tests

```bash
make test-rfx-service
```

## Docker

```bash
docker build -f services/rfx-service/Dockerfile -t freight-platform/rfx-service .
```
