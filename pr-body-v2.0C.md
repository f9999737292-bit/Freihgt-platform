## Summary

Implements **v2.0C — Pricing Sources + Immutable Transport Order Rate Snapshot + Settlement Integration**:

- RFx internal S2S pricing context API (`RFQ_AWARD`, accepted `SPOT_BID`) with decimal-string money and aggregate-only normalization
- Contract-rate resolver precedence: RFQ_AWARD → SPOT_BID → CONTRACT_RATE → MANUAL_SPOT (no silent RFx fallback)
- Immutable `transport.transport_order_rate_snapshots` persisted atomically with transport order create + generic `Idempotency-Key`
- Award conversion saga: RFx award-scope pricing → transport-order internal create → RFx link-only persistence (no uncommitted cross-service reads)
- Settlement migration to `snapshot.total_amount` with legacy award-link fallback for pre-v2.0C orders only

## Architecture / transaction boundary

Award conversion no longer inserts transport orders inside the RFx transaction. Pricing context is resolved from committed award-scope data; transport-order-service owns TO + snapshot in a single DB transaction; RFx persists the award link after successful TO creation.

## Migrations

- `000051` — rate snapshots, create idempotency, `pricing_model_version`, immutability triggers
- `000052` — settlement `rate_snapshot_id` / `pricing_source`

## Out of scope (v2.0C)

- v2.0D contract/rate workspace UI
- v2.0E public RBAC/OpenAPI hardening
- Billing/Payment pricing calculation changes
- Historical repricing / snapshot UPDATE/DELETE

## Test plan

- [ ] CI: `freight-pricing-snapshot-integration` (PostgreSQL 16)
- [ ] CI: `contract-rate-core-integration`
- [ ] CI: `rfx-award-to-transport-order-integration`
- [ ] CI: `freight-settlement-integration`
- [ ] Unit: `go test ./...` in rfx-service, contract-rate-service, transport-order-service, billing-register-service

## Gates (draft — mark ready when CI green)

- AWARD_TRANSACTION_BOUNDARY=PASS
- RFQ_AWARD / SPOT_BID / CONTRACT_RATE / MANUAL_SPOT flows
- TO idempotency + TO+snapshot atomicity
- Snapshot immutability (UPDATE/DELETE denied)
- Settlement uses `snapshot.total_amount` (no fuel double-count)
- NEW_ORDER_MISSING_SNAPSHOT=FAIL_CLOSED
- LEGACY_AWARD_FALLBACK=PASS
