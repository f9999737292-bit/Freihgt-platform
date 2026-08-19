# Payment Service

Canonical SSOT for freight payment obligations, payments, allocations, and reconciliation.

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PAYMENT_SERVICE_PORT` | `8090` | HTTP port |
| `DATABASE_URL` | local postgres | PostgreSQL connection |
| `BILLING_REGISTER_SERVICE_URL` | `http://localhost:8087` | Billing register internal sync target |
| `INTERNAL_SERVICE_TOKEN` | empty | Service-to-service auth token |
| `PAYMENT_OUTBOX_ENABLED` | `false` | Enable in-process PAID projection outbox worker |
| `PAYMENT_OUTBOX_POLL_INTERVAL` | `2s` | Worker poll interval |
| `PAYMENT_OUTBOX_BATCH_SIZE` | `50` | Max events claimed per poll |
| `PAYMENT_OUTBOX_PUBLISH_TIMEOUT` | `10s` | HTTP delivery timeout per event |
| `PAYMENT_OUTBOX_LEASE_TIMEOUT` | `60s` | Claim lease duration (must exceed publish timeout) |
| `PAYMENT_OUTBOX_MAX_ATTEMPTS` | `5` | Max delivery attempts before FAILED |
| `PAYMENT_OUTBOX_WORKER_ID` | hostname-uuid | Worker identity for lease ownership |

## PAID projection outbox (v1.9.2A)

When a payment obligation first transitions to **PAID**, the allocation transaction inserts a durable
`billing.payment_outbox` row (`payment_obligation.paid`). After commit, payment-service may attempt
immediate billing sync (Option B). The outbox worker retries until billing projection is satisfied.

### Outbox statuses

| Status | Meaning |
|--------|---------|
| `PENDING` | Delivery not yet confirmed; worker will retry |
| `PUBLISHED` | Billing projection satisfied (including CLOSED=already-satisfied) |
| `FAILED` | Permanent failure or max attempts exhausted — inspect `last_error_code` |

### Operational notes

- Do **not** manually edit outbox rows in production.
- Do **not** insert a second `payment_obligation.paid` row for the same obligation (unique constraint).
- `FAILED` events require operator investigation; obligation PAID state remains canonical SSOT.
- Billing register **CLOSED** with canonical obligation PAID counts as projection already satisfied.

## Local run

```bash
make run-payment-service
curl http://localhost:8090/health
```

Public routes are exposed through API Gateway at `/api/v1/payments` and `/api/v1/payment-obligations`.

Internal route:

- `POST /internal/v1/payment-obligations/ensure` — idempotent obligation creation for signed billing registers

Money policy: PostgreSQL `NUMERIC(18,2)` with Go `shopspring/decimal`.
