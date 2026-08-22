# FREIGHT_COST_ACCRUAL_LEDGER v2.1B — Implementation Notes

This document describes the **actual implementation** delivered in the v2.1B worktree after the planning doc `FREIGHT_COST_ACCRUAL_LEDGER_v2.1B_IMPLEMENTATION_PLAN.md`.

## Summary

v2.1B adds a derived, append-only freight cost ledger (`freight_cost` schema), projection persistence, billing transactional outbox emission, payment paid-snapshot outbox versioning, canonical HTTP read adapters, rebuild, and 57 FC-B-* integration tests.

## Schema & migrations (through `000056`)

| Migration | Service scope | Purpose |
|-----------|---------------|---------|
| `000054_freight_cost_ledger_v2.1B` | freight-cost | `freight_cost.cost_entry`, `source_cursor`, `cost_summary_projection` |
| `000055_billing_freight_cost_outbox_v2.1B` | billing-register | `billing.freight_cost_outbox`, `billing_link_revision` on settlements |
| `000056_payment_outbox_aggregate_version_v2.1B` | payment | `aggregate_version` + partial unique indexes for versioned paid snapshots |

Integration helpers apply migrations through **`000056_payment_outbox_aggregate_version_v2.1B.up.sql`**.

## freight-cost-service

### Ingest (`IngestService`)

- Dual identity: `source_event_id` (delivery) + `source_fact_id` (canonical UUIDv5)
- Idempotency: `UNIQUE(tenant_id, source_event_id)` and `UNIQUE(tenant_id, source_fact_id)`
- Out-of-order: journal unseen facts once; projection updates only when `source_revision > cursor`
- `amount_availability=UNAVAILABLE` represents NULL financial amounts
- Entry kinds: planned, accrual, current/final actual, billed, payable, paid

### Rebuild (`RebuildService`)

- Root: canonical HTTP reads (transport rate snapshot, settlement, billing link, register payable, payment obligation)
- Delivery IDs: `DeriveRebuildDeliveryID(tenant, source_fact_id)` — distinct from live outbox UUIDs
- Idempotent on `(tenant_id, source_fact_id)`

### Internal HTTP

- `GET /internal/v1/freight-cost/transport-orders/{id}` — projection-backed cost summary (v2.1A fields populated)
- `POST /internal/v1/freight-cost/source-events` — ingest envelope
- `POST /internal/v1/freight-cost/transport-orders/{id}/rebuild` — canonical rebuild

## billing-register-service

### Internal reads (decimal strings, EX_VAT where applicable)

- `GET /internal/v1/freight-settlements/by-transport-order/{transportOrderId}`
- `GET /internal/v1/freight-settlements/{settlementId}/billing-link`
- `GET /internal/v1/billing-registers/{registerId}/payable`

### Outbox

Transactional emission on settlement/accessorial/billing-link/register mutations via `FreightCostOutboxEmitter`.

Accrual money uses **exact NUMERIC approved-set sum** from `billing.settlement_accessorials WHERE status='APPROVED'`.

### Accrual semantics (F-002 closure)

Frozen v2.1B accrual formula:

```text
FINANCIAL_ACCRUAL = rate_snapshot.total_amount + SUM(APPROVED accessorials)
```

Runtime implementation path:

```text
ACCRUAL_PRINCIPAL_SOURCE=RATE_SNAPSHOT_TOTAL_AMOUNT
SETTLEMENT_BASE_FREIGHT_ROLE=IMMUTABLE_COPY_OF_RATE_SNAPSHOT_PRINCIPAL
ACCRUAL_APPROVED_SET_SOURCE=EXACT_NUMERIC_APPROVED_ACCESSORIALS
LIVE_ACCRUAL_FORMULA=base_freight_amount + approved_set_sum
REBUILD_ACCRUAL_FORMULA=base_freight_amount + approved_set_sum (internal settlement read)
LIVE_REBUILD_SEMANTICS_MATCH=YES
```

For `SNAPSHOT_V1` settlements:

- `CreateSettlement` copies `transport_order_rate_snapshots.total_amount` into `base_freight_amount` and stores `rate_snapshot_id`.
- `base_freight_amount` has **no production UPDATE path** after INSERT (`recalculateSettlementTotals` updates totals only).
- Therefore `base_freight_amount == snapshot.total_amount` for the settlement lifetime, and accrual via `base + approved` is equivalent to `planned + approved`.

Legacy pre-`SNAPSHOT_V1` orders (no rate snapshot) use award-link provenance at settlement create (`LoadShipmentContext` legacy branch). v2.1B ledger accrual for those settlements uses the immutable award-derived `base_freight_amount`, not transport snapshot reads. New `SNAPSHOT_V1` settlement creation **fail-closes** when the rate snapshot is missing.

Regression tests: `freightcostledger/accrual_semantics_integration_test.go` (FC-B-ACC-SEM A–F), `freightsettlement/snapshot_settlement_integration_test.go` (CSet001–011).

`billing_link_revision` monotonic on link/unlink/relink.

## payment-service

### Internal read (new)

```http
GET /internal/v1/payment-obligations/by-billing-register/{billingRegisterId}
X-Internal-Service-Token
X-Tenant-ID
```

Returns decimal-string `original_amount` / `paid_amount` with `tax_basis=WITH_VAT`, plus obligation/register/version/status and transport-order context via register-item → settlement join.

### Outbox versioning

- Legacy `payment_obligation.paid` — one row per obligation (partial unique index)
- `payment_obligation.paid_snapshot.v1` — one row per `(obligation, obligation.version)` via `aggregate_version`

## Integration tests

### FC-B matrix (57 tests)

Location: `services/freight-cost-service/internal/integration/ledger/`

| File | Family | Count |
|------|--------|------:|
| `ledger_led_integration_test.go` | FC-B-LED | 13 |
| `ledger_mon_integration_test.go` | FC-B-MON | 5 |
| `ledger_acc_integration_test.go` | FC-B-ACC | 7 |
| `ledger_act_integration_test.go` | FC-B-ACT | 6 |
| `ledger_bil_integration_test.go` | FC-B-BIL | 8 |
| `ledger_pay_integration_test.go` | FC-B-PAY | 3 |
| `ledger_rbl_integration_test.go` | FC-B-RBL | 6 |
| `ledger_sec_integration_test.go` | FC-B-SEC | 4 |
| `ledger_out_integration_test.go` | FC-B-OUT | 5 |

Run: `go test -tags=integration ./internal/integration/ledger/...` with `TEST_DATABASE_URL`.

### Billing v2.1B tests

`services/billing-register-service/internal/integration/freightcostledger/` — billing link revision monotonicity, approved-set accrual SQL, internal billing-link read.

### Payment v2.1B tests

`services/payment-service/internal/integration/freightpaymentscore/`:

- `paid_snapshot_v2.1B_integration_test.go` — versioned outbox snapshots
- `payment_internal_read_integration_test.go` — internal GET by billing register

## CI

Job **`freight-cost-ledger-integration`** in `.github/workflows/ci.yml`:

1. freight-cost ledger tests (`./internal/integration/ledger/...`)
2. billing freightcostledger tests
3. payment FC_B_PAY* tests

## Cross-service constraints (verified)

- **0** cross-service DB reads from freight-cost (HTTP only)
- Append-only ledger (DB triggers deny UPDATE/DELETE)
- Carrier read masking on accrual/analytics fields
- Wrong tenant → 404 on reads; source-event tenant mismatch → 400

## Not in v2.1B

Forecast in ledger, variance attribution, public API, frontend — deferred per planning doc §5 / v2.1C handoff.
