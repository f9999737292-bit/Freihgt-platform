# FREIGHT PRICING SNAPSHOT & SETTLEMENT v2.0C — Implementation

## GIT

| Field | Value |
|-------|-------|
| Branch | `feat/freight-pricing-snapshot-settlement-v2.0C` |
| Base SHA | `b0f41c1087eb48afefca1126e22a168481e18b0e` |
| PR #37 | MERGED (v2.0B manual spot hardening) |

## DISCOVERY

| Fact | Verified state |
|------|----------------|
| `LATEST_MIGRATION_BEFORE_V2_0C` | `000050` |
| `V2_0C_MIGRATION_START` | `000051` |
| `TO_PRICING_CURRENT` | Legacy create without resolver/snapshot |
| `TO_IDEMPOTENCY_CURRENT` | None before v2.0C |
| `AWARD_CONVERSION_CURRENT` | Direct `transport.transport_orders` insert in rfx TX |
| `SETTLEMENT_PRICING_CURRENT` | `rfx_award_transport_orders.amount::float8` |
| `RFX_BID_MONEY_CURRENT` | float64 domain; NUMERIC in DB |

## TRANSACTION_BOUNDARY_GATE

**Problem:** Uncommitted `rfx_award_transport_orders` link is not visible to contract-rate → rfx HTTP chain.

**Approved design (saga, no distributed TX):**

1. RFx award scope pricing is readable from **committed** award data via `GET /internal/v1/pricing/award-scope/{eventId}?lot_id=`.
2. rfx-service prepares lane/cargo, then **HTTP** calls transport-order-service `POST /internal/v1/transport-orders/from-award-scope` (TO + snapshot in single TO DB TX).
3. rfx-service then inserts **link only** in its TX.
4. Idempotency: `award-conv:{tenant}:{event}:{lotKey}`.

| Gate | Status |
|------|--------|
| `AWARD_SOURCE_ID_STABLE_BEFORE_SNAPSHOT` | YES — event/lot scope |
| `RFX_PRICING_CONTEXT_READABLE_BEFORE_TO_COMMIT` | YES — award-scope API |
| `TO_PLUS_SNAPSHOT_SINGLE_DB_TX` | YES |
| `AWARD_RETRY_IDEMPOTENT` | YES |
| `NO_UNCOMMITTED_HTTP_DEPENDENCY` | YES |
| `NO_CONTRACT_RATE_DIRECT_RFX_SQL` | YES |
| `NO_CLIENT_MONEY_AUTHORITY` | YES |
| `NO_DISTRIBUTED_TX_CLAIM` | YES |

## RFx PRICING API

Internal S2S routes (shared-go `internalauth`):

- `GET /internal/v1/pricing/award-context/{sourceId}`
- `GET /internal/v1/pricing/award-scope/{eventId}?lot_id=`
- `GET /internal/v1/pricing/bid-context/{bidId}`

Money: PostgreSQL NUMERIC → text → decimal string JSON. **No float round-trip.**

Aggregate RFQ award: `base_amount=NULL`, `components=[]`, `component_breakdown_status=UNAVAILABLE`.

## RESOLVER PRECEDENCE

1. RFQ_AWARD (link or scope)
2. SPOT_BID (ACCEPTED only, pre-VAT `total_amount`)
3. CONTRACT_RATE
4. MANUAL_SPOT
5. RATE_NOT_FOUND

Invalid explicit RFx → fail closed, no fallback.

## TO COMMERCIAL CONTEXT

- `pricing_context` on priced create
- `Idempotency-Key` header (scoped: tenant + actor company + key)
- `buyer_company_id` = shipper; carrier from context or trusted RFx resolve result
- `equipment_type` required for priced create

## SNAPSHOT MODEL

Migration `000051`: `transport.transport_order_rate_snapshots`, `transport_order_create_idempotency`, `transport_orders.pricing_model_version`.

- `SNAPSHOT_V1` marks snapshot-required orders
- UNIQUE `(tenant_id, transport_order_id)`
- DB triggers deny UPDATE/DELETE

## SETTLEMENT MIGRATION

Migration `000052`: `billing.freight_settlements.rate_snapshot_id`, `pricing_source`.

Loader: snapshot `total_amount` first for `SNAPSHOT_V1`; legacy award fallback when `pricing_model_version IS NULL`; fail closed when snapshot required but missing.

Principal path uses `shopspring/decimal` end-to-end for snapshot-based settlement (`AgreedFreightAmount`); NUMERIC text parse and DB bind via `StringFixed(2)` — **no float64 round-trip on snapshot principal** (`SETTLEMENT_SNAPSHOT_FLOAT_ROUNDTRIP=NO`).

## COMMERCIAL INTEGRITY (PR #38 hardening)

| Gate | Fix |
|------|-----|
| Public unpriced TO create | Denied — `POST /v1/transport-orders` requires `Idempotency-Key` + priced path only |
| Equipment case | TrimSpace only; exact case match (no ToUpper/EqualFold) |
| RFx fail-closed | No default RUB/TAUTLINER; missing currency/equipment/lane → `MISSING_PRICING_CONTEXT` |
| Award scope ambiguity | Multi-lot without lot → `PRICING_SOURCE_AMBIGUOUS` |
| Idempotency concurrency | PostgreSQL advisory lock + unique-violation re-read |
| Tenant-safe FK | Migration `000053` composite FK on snapshots + idempotency |
| Observability | `pricing_source_total`, `snapshot_persist_*`, `to_pricing_resolution_total`, `legacy_settlement_pricing_fallback_total` |

Full C-RFX / C-RES / C-SNAP / C-TO / C-AWARD / C-SET matrices in integration tests.

## TESTS & CI

- Unit: rfx, contract-rate, transport-order, billing-register
- Integration: `freight-pricing-snapshot-integration` CI job
- Updated `rfx-award-to-transport-order-integration` (TO stub + snapshot assertions)

## OUT OF SCOPE

v2.0D UI, v2.0E public routes, Billing/Payment pricing logic changes, historical repricing, snapshot UPDATE/DELETE paths.

## FINAL GATES (target)

See PR checklist §42. PR remains **DRAFT** until all CI gates green.
