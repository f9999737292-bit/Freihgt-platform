# Payment Service

Canonical SSOT for freight payment obligations, payments, allocations, and reconciliation.

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PAYMENT_SERVICE_PORT` | `8090` | HTTP port |
| `DATABASE_URL` | local postgres | PostgreSQL connection |
| `BILLING_REGISTER_SERVICE_URL` | `http://localhost:8087` | Billing register internal sync target |
| `INTERNAL_SERVICE_TOKEN` | empty | Optional service-to-service auth token |

## Local run

```bash
make run-payment-service
curl http://localhost:8090/health
```

Public routes are exposed through API Gateway at `/api/v1/payments` and `/api/v1/payment-obligations`.

Internal route:

- `POST /internal/v1/payment-obligations/ensure` — idempotent obligation creation for signed billing registers

Money policy: PostgreSQL `NUMERIC(18,2)` with Go `shopspring/decimal`.
