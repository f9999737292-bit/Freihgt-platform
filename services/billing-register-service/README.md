# billing-register-service

Go microservice for billing registers and closing documents in Freight Platform.

## Purpose

`billing-register-service` handles financial closing of shipments:

- Create billing registers and add shipment items
- Calculate totals and approve registers
- Create closing documents: invoices, acts, VAT invoices, UPD
- Track financial closing status (mock EDO / payment flow)

Business flow:

`Shipment READY_FOR_BILLING` → Billing Register → Items → Calculate → Approve → Closing Documents → EDO / 1C (later)

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `BILLING_REGISTER_SERVICE_PORT` | `8087` | HTTP port |
| `DATABASE_URL` | local postgres URL | PostgreSQL connection string |
| `LOG_LEVEL` | `info` | Log level (`debug`, `info`, `warn`, `error`) |
| `ENVIRONMENT` | `development` | Runtime environment |

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| POST | `/v1/billing-registers` | Create billing register |
| GET | `/v1/billing-registers/{id}` | Get register with items and closing documents |
| GET | `/v1/billing-registers` | List registers |
| POST | `/v1/billing-registers/{id}/items` | Add shipment to register |
| GET | `/v1/billing-registers/{id}/items` | List register items |
| DELETE | `/v1/billing-registers/{register_id}/items/{item_id}` | Remove item from register |
| POST | `/v1/billing-registers/{id}/calculate` | Calculate register totals |
| POST | `/v1/billing-registers/{id}/approve` | Approve register |
| POST | `/v1/billing-registers/{id}/closing-document-package` | Create closing document package |
| POST | `/v1/billing-registers/{id}/invoices` | Create invoice |
| POST | `/v1/billing-registers/{id}/acts` | Create act |
| POST | `/v1/billing-registers/{id}/vat-invoices` | Create VAT invoice |
| POST | `/v1/billing-registers/{id}/upd` | Create UPD |
| POST | `/v1/billing-registers/{id}/mark-sent-to-edo` | Mark as sent to EDO (mock) |
| POST | `/v1/billing-registers/{id}/mark-signed` | Mark as signed by counterparty (mock) |
| POST | `/v1/billing-registers/{id}/mark-paid` | Mark as paid |
| POST | `/v1/billing-registers/{id}/close` | Close register |

## Run locally

```bash
make dev-up
make migrate-up
make run-billing-register-service
```

Health check:

```bash
curl http://localhost:8087/health
```

## Examples

Create billing register:

```bash
curl -X POST http://localhost:8087/v1/billing-registers \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "11111111-1111-1111-1111-111111111111",
    "register_number": "BR-2026-000001",
    "customer_company_id": "22222222-2222-2222-2222-222222222222",
    "contractor_company_id": "33333333-3333-3333-3333-333333333333",
    "period_from": "2026-07-01",
    "period_to": "2026-07-15",
    "currency_code": "RUB",
    "vat_rate": 20
  }'
```

Add shipment to register:

```bash
curl -X POST http://localhost:8087/v1/billing-registers/{register_id}/items \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "11111111-1111-1111-1111-111111111111",
    "shipment_id": "44444444-4444-4444-4444-444444444444",
    "route_description": "Москва — Казань",
    "pickup_date": "2026-07-01",
    "delivery_date": "2026-07-03",
    "base_amount": 100000,
    "extra_charges": 5000,
    "penalties": 0,
    "vat_rate": 20
  }'
```

Calculate register:

```bash
curl -X POST http://localhost:8087/v1/billing-registers/{register_id}/calculate \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "11111111-1111-1111-1111-111111111111"
  }'
```

Approve register:

```bash
curl -X POST http://localhost:8087/v1/billing-registers/{register_id}/approve \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "11111111-1111-1111-1111-111111111111",
    "approved_by": "55555555-5555-5555-5555-555555555555"
  }'
```

Create invoice:

```bash
curl -X POST http://localhost:8087/v1/billing-registers/{register_id}/invoices \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "11111111-1111-1111-1111-111111111111",
    "invoice_number": "INV-2026-000001",
    "invoice_date": "2026-07-16",
    "seller_company_id": "33333333-3333-3333-3333-333333333333",
    "buyer_company_id": "22222222-2222-2222-2222-222222222222"
  }'
```

Create act:

```bash
curl -X POST http://localhost:8087/v1/billing-registers/{register_id}/acts \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "11111111-1111-1111-1111-111111111111",
    "act_number": "ACT-2026-000001",
    "act_date": "2026-07-16",
    "seller_company_id": "33333333-3333-3333-3333-333333333333",
    "buyer_company_id": "22222222-2222-2222-2222-222222222222",
    "service_description": "Транспортные услуги по реестру BR-2026-000001"
  }'
```

Create VAT invoice:

```bash
curl -X POST http://localhost:8087/v1/billing-registers/{register_id}/vat-invoices \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "11111111-1111-1111-1111-111111111111",
    "vat_invoice_number": "SF-2026-000001",
    "vat_invoice_date": "2026-07-16",
    "seller_company_id": "33333333-3333-3333-3333-333333333333",
    "buyer_company_id": "22222222-2222-2222-2222-222222222222"
  }'
```

Create UPD:

```bash
curl -X POST http://localhost:8087/v1/billing-registers/{register_id}/upd \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "11111111-1111-1111-1111-111111111111",
    "upd_number": "UPD-2026-000001",
    "upd_date": "2026-07-16",
    "seller_company_id": "33333333-3333-3333-3333-333333333333",
    "buyer_company_id": "22222222-2222-2222-2222-222222222222",
    "function_code": "СЧФДОП"
  }'
```

Mark as paid:

```bash
curl -X POST http://localhost:8087/v1/billing-registers/{register_id}/mark-paid \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "11111111-1111-1111-1111-111111111111"
  }'
```

## Docker

Build from monorepo root:

```bash
docker build -f services/billing-register-service/Dockerfile -t freight-platform/billing-register-service .
```

## Tests

```bash
make test-billing-register-service
```
