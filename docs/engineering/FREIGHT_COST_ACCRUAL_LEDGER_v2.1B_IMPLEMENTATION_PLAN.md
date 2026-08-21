# Freight Cost Accrual & Cost Ledger v2.1B — Implementation Plan & Contract Freeze

**Status:** PLANNING / CONTRACT FREEZE (no runtime in this document)  
**Base SHA:** `1cc3ca6c01e981b5fb220811a0f2701b9cf1a4c0` (main after PR #44 / v2.1A)  
**Architecture baseline:**
- `docs/engineering/FREIGHT_COST_MANAGEMENT_v2.1_ARCHITECTURE.md`
- `docs/architecture/FREIGHT_COST_MANAGEMENT_v2.1_FINAL_REVIEW.md`
- `docs/engineering/FREIGHT_COST_FOUNDATION_v2.1A_IMPLEMENTATION_PLAN.md`
- `docs/engineering/FREIGHT_COST_FOUNDATION_v2.1A_IMPLEMENTATION.md`

**Purpose:** Make subsequent v2.1B implementation mechanical and unambiguous. This slice defines contracts only — **do not implement runtime in the planning PR**.

---

## 1. Objective

Deliver **Freight Cost Accrual & Cost Ledger (v2.1B)**: the persistent derived financial layer behind `freight-cost-service`:

- `freight_cost` PostgreSQL schema;
- append-only derived `cost_entry` journal;
- source revision / idempotency;
- settlement / accessorial / billing transactional outbox publication;
- payment outbox consumption (reuse + minimal extension);
- accrual / current / final actual / billed / paid projection persistence;
- canonical-source rebuild;
- billing reconciliation state.

v2.1B **does not** create a second financial source of truth. Canonical facts remain in transport-order, billing-register, and payment-service.

**Business question (frozen):**

> What is the current recognized freight cost exposure and actual cost, where did every value come from, and can the derived projection be rebuilt deterministically from canonical financial systems?

---

## 2. Architecture baseline

### 2.1 Precondition verification (frozen at `1cc3ca6`)

| Gate | Value |
|------|-------|
| v2.1A merged (PR #44) | YES @ `1cc3ca6` |
| `freight-cost-service` exists | YES @ port **8092**, stateless |
| Internal planned read API | YES |
| Transport internal snapshot API | YES |
| Domain pure functions (finality, accrual, reconciliation) | YES (v2.1A) |
| Provider interfaces declared | YES (transport wired; settlement/billing/payment stubs) |
| `LEDGER_AUTHORITY` | `DERIVED_EVENT_JOURNAL` |
| `LEDGER_SECOND_SSOT` | NO |
| `REBUILD_ROOT` | canonical domain read APIs |
| `PLANNED_COST_OWNER` | transport-order-service |
| `FINAL_ACTUAL_STATUS` | READY_FOR_PAYMENT |
| `MIXED_CURRENCY_AGGREGATION` | DENY |
| `CROSS_COMPANY_COST_ACCESS` | DENY |
| v2.1A OPEN_BLOCKER / OPEN_HIGH | 0 |

### 2.2 v2.1A code discovery (actual on base)

| Item | Finding |
|------|---------|
| `FREIGHT_COST_SERVICE_PORT` | **8092** |
| `FREIGHT_COST_DATABASE_USAGE` | **NO** — stateless; `DB: nil` in observability mount |
| `FREIGHT_COST_LEDGER_EXISTS` | **NO** |
| `FREIGHT_COST_EVENT_CONSUMER_EXISTS` | **NO** |
| Transport route | `GET /internal/v1/transport-orders/{id}/rate-snapshot` |
| Planned source | `transport.transport_order_rate_snapshots.total_amount` |
| Downstream validation | tenant/order binding, UUID non-zero, `SNAPSHOT_V1`, decimal string, `>= 0` |

Do **not** reopen frozen v2.1A decisions without a documented BLOCKER.

---

## 3. Discovery

### 3.1 Settlement model (billing-register-service)

| Field | Value |
|-------|-------|
| Table | `billing.freight_settlements` (migration `000042`) |
| Snapshot link | `rate_snapshot_id UUID` (migration `000052`) |
| Amount fields (DB) | `base_freight_amount`, `approved_accessorial_total`, `total_without_vat`, `vat_amount`, `total_with_vat` — all `NUMERIC(18,2)` |
| Amount fields (Go domain) | **`float64`** in `freight_settlement.go` |
| Status | `DRAFT`, `UNDER_REVIEW`, `DISPUTED`, `APPROVED`, `DOCUMENTS_READY`, `READY_FOR_PAYMENT`, `CANCELLED` |
| Revision | `version INTEGER DEFAULT 1` (optimistic lock, monotonic on successful update) |
| Disputes | `billing.settlement_disputes`; open count drives finality |
| Internal read API | **NOT_FOUND** — public routes require actor context |

### 3.2 Accessorial model

| Field | Value |
|-------|-------|
| Table | `billing.settlement_accessorials` (migration `000042`) |
| Owner | billing-register-service |
| ID | row `id UUID` |
| Status field | `status VARCHAR(50)` — `PROPOSED`, `APPROVED`, `REJECTED`, `DISPUTED` |
| Amount field | `amount NUMERIC(18,2) CHECK (amount >= 0)` |
| Currency | settlement `currency_code` (same settlement) |
| Tax basis | **EX_VAT** (accessorial amounts roll into settlement ex-VAT totals) |
| TO link | via `freight_settlements.transport_order_id` |
| Settlement link | `settlement_id` FK |

### 3.3 Billing register model

| Field | Value |
|-------|-------|
| Tables | `billing.billing_registers`, `billing.billing_register_items` |
| Register totals | `total_without_vat`, `total_with_vat` — domain `float64`; DB `NUMERIC(18,2)` |
| Item snapshot at include | frozen line amounts on `billing_register_items` |
| Settlement link | `freight_settlements.billing_register_id`, `billing_register_item_id` |
| Include mutation | `IncludeInRegister` — same transaction boundary as settlement update |
| Remove mutation | register item removal APIs exist |
| Internal route | `POST /internal/v1/billing-registers/{id}/sync-paid` (payment projection only) |

### 3.4 Payment outbox

| Field | Value |
|-------|-------|
| Table | `billing.payment_outbox` (migration `000046`) |
| Event ID | outbox row `id UUID` |
| Aggregate | `aggregate_type=PAYMENT_OBLIGATION`, `aggregate_id=obligation_id` |
| Event type (today) | `payment_obligation.paid` only |
| Payload (today) | `{tenant_id, obligation_id, register_id}` — **no money fields** |
| Revision on outbox | **NO** — `schema_version` only |
| Obligation revision | `payment_obligations.version INTEGER` optimistic lock |
| Money in payment-service | **`shopspring/decimal.Decimal`** throughout |
| Reuse assessment | **PARTIAL** — pattern reusable; cost ingest needs additional event types + versioned financial snapshot payload |

### 3.5 Outbox patterns (repository)

| Service | Outbox table | Publisher |
|---------|--------------|-----------|
| shipment-service | `transport.shipment_event_outbox` | Kafka-capable worker; envelope with `eventId`, `aggregate_version` |
| payment-service | `billing.payment_outbox` | HTTP publisher → billing `sync-paid` |
| billing-register-service | **NONE** | v2.1B must add |

Reuse **payment/shipment outbox worker conventions**: `PENDING` → claim → publish → `PUBLISHED` / retry / `FAILED`; partial index on pending.

### 3.6 Migration sequence (current)

| Area | Latest migration |
|------|------------------|
| Repo-wide | **000053** |
| freight-cost | **none** (new in v2.1B) |
| billing-register / settlement | 000052 (+ 042–044) |
| payment | 000047 (+ 045–046) |
| transport-order snapshot | 000051 (+ 053) |

**Next migration numbers (planning — assign at implementation time after re-verify):**

| Service | Next | Purpose |
|---------|------|---------|
| freight-cost-service | **000054** | `freight_cost` schema: `cost_entry`, `source_cursor`, `cost_summary_projection` |
| billing-register-service | **000054** or **000055** if shared sequence conflicts | transactional outbox + optional revision hardening |
| payment-service | **000048** only if payload extension required | additional outbox event types / schema_version bump |

---

## 4. Scope — `V2_1B_SCOPE`

v2.1B implementation **SHOULD include:**

| # | Deliverable |
|---|-------------|
| 1 | `freight_cost` PostgreSQL schema + migrations |
| 2 | Append-only `cost_entry` derived journal |
| 3 | Source-event identity + revision handling |
| 4 | Replay / idempotency (`UNIQUE (tenant_id, source_event_id)`) |
| 5 | `source_cursor` per revision stream |
| 6 | `cost_summary_projection` persistence |
| 7 | Accrual persistence / projection |
| 8 | Current actual persistence / projection |
| 9 | Final actual persistence / projection |
| 10 | Billed snapshot + reconciliation inputs |
| 11 | Paid / payable projection (WITH_VAT semantics) |
| 12 | billing-register transactional outbox for settlement/accessorial/billing mutations |
| 13 | Consume / extend payment outbox for cost facts |
| 14 | Provider implementations (HTTP canonical read adapters) |
| 15 | Rebuild from canonical APIs (`RebuildTransportOrder`) |
| 16 | Consumer transaction + out-of-order policy |
| 17 | Observability / DLQ strategy aligned with repo patterns |
| 18 | Test matrix FC-B-* (see §33) |
| 19 | CI matrix updates for DB-backed freight-cost-service |
| 20 | Populate v2.1A internal read API fields from projection (no new public routes) |

---

## 5. Out-of-scope — `V2_1B_OUT_OF_SCOPE`

| Area | Deferred to |
|------|-------------|
| Variance reason attribution | v2.1C |
| `charge_code` semantic classification | v2.1C |
| Cost analytics frontend | v2.1D |
| Public `/api/v1` gateway routes | v2.1E |
| FX conversion | Out of v2.1 |
| ML / predictive cost | Later |
| Budget / benchmark / profitability engines | Later |
| Invoice generation changes unrelated to cost facts | Out |
| Forecast in cost ledger | **NO** (never) |
| General ledger / double-entry | Out of v2.1 |
| Bulk tenant-wide rebuild orchestration UI | Optional defer; minimum = per-TO rebuild command |

---

## 6. Canonical owners

| FACT | CANONICAL_OWNER | CANONICAL_TABLE/AGGREGATE | CANONICAL_FIELD | TAX_BASIS | CURRENCY | FINALITY | V2_1B_LEDGER_KIND | REBUILD_API |
|------|-----------------|---------------------------|-----------------|-----------|----------|----------|-------------------|-------------|
| Planned cost | transport-order-service | `transport.transport_order_rate_snapshots` | `total_amount` | EX_VAT | snapshot `currency_code` | Immutable at pricing | `PLANNED_COST_SNAPSHOT` | `GET /internal/v1/transport-orders/{id}/rate-snapshot` |
| Approved accessorial (execution) | billing-register-service | `billing.settlement_accessorials` | `amount` WHERE `status=APPROVED` | EX_VAT | settlement currency | Mutable until settlement recalc | *(feeds `ACCRUAL_COST_SNAPSHOT`)* | Settlement internal read |
| Financial accrual | freight-cost-service **DERIVED** | `freight_cost.cost_summary_projection` | `accrued_amount` | EX_VAT | TO/settlement currency | Derived snapshot | `ACCRUAL_COST_SNAPSHOT` | Rebuild derives from planned + approved lines |
| Current actual | billing-register-service | `billing.freight_settlements` | `total_without_vat` | EX_VAT | settlement currency | APPROVED/DOCUMENTS_READY/READY_FOR_PAYMENT, no open disputes | `CURRENT_ACTUAL_COST_SNAPSHOT` | Settlement internal read |
| Final actual | billing-register-service | `billing.freight_settlements` | `total_without_vat` | EX_VAT | settlement currency | `READY_FOR_PAYMENT` only | `FINAL_ACTUAL_COST_SNAPSHOT` | Settlement internal read |
| Billed line snapshot | billing-register-service | `billing.billing_register_items` | frozen ex-VAT line at include | EX_VAT | register currency | Frozen at include | `BILLED_COST_SNAPSHOT` | Billing internal read |
| Register payable total | billing-register-service | `billing.billing_registers` | `total_with_vat` | WITH_VAT | register currency | Until closed | `PAYABLE_AMOUNT_SNAPSHOT` | Billing internal read |
| Paid amount | payment-service | `billing.payment_obligations` | `paid_amount` | WITH_VAT | obligation currency | Updated on allocation TX | `PAID_AMOUNT_SNAPSHOT` | Payment internal read |
| Forecast exposure | freight-cost-service **DERIVED KPI** | projection (optional) | `forecast_exposure` | EX_VAT | same | Non-canonical | **NOT in ledger** | Defer persistence v2.1C |

**Frozen:**

```text
LEDGER_CANONICAL_SSOT=NO
LEDGER_DERIVED=YES
CANONICAL_SERVICES_WIN_ON_MISMATCH=YES
ACCRUAL_IS_CANONICAL_SSOT=NO
ACCRUAL_IS_DERIVED=YES
```

---

## 7. Money / tax basis

### 7.1 Money contract (carry forward v2.1A)

| Layer | Type |
|-------|------|
| freight-cost domain | `shopspring/decimal.Decimal` |
| Database | `NUMERIC(18,2)` |
| Wire / event JSON | **decimal string** (scale 2, no scientific notation) |
| Unknown | SQL `NULL` / Go `nil *Money` / JSON `null` |
| Known zero | `"0.00"` / non-nil Money |

```text
FREIGHT_COST_FLOAT64_MONEY=DENY
UNKNOWN_AMOUNT_EQUALS_ZERO=NO
```

### 7.2 Tax basis enum

```go
type TaxBasis string

const (
    TaxBasisExVAT   TaxBasis = "EX_VAT"
    TaxBasisWithVAT TaxBasis = "WITH_VAT"
)
```

Every `cost_entry` row **MUST** carry explicit `tax_basis`.

| Fact | Tax basis |
|------|-----------|
| planned, accrual, current/final actual, billed ex-VAT line | EX_VAT |
| payable obligation, paid amount | WITH_VAT |

```text
MIXED_TAX_BASIS_SUM=DENY
PAID_AMOUNT_MUST_NOT_BE_DIRECTLY_COMPARED_TO_PLANNED_EX_VAT=YES
```

Comparisons (variance v2.1C, reconciliation diagnostics) must filter by compatible basis.

---

## 8. Legacy float64 boundary — CRITICAL

### 8.1 Discovery

billing-register settlement/register/accessorial Go domain uses **`float64`** while DB is `NUMERIC(18,2)`. Snapshot principal load in repository already uses `decimal.Decimal` → `StringFixed(2)` for INSERT.

payment-service and freight-cost-service use `decimal` end-to-end.

### 8.2 Frozen mitigation (Option A + C hybrid)

**freight-cost-service MUST NEVER consume billing monetary facts over a float64 wire boundary.**

| Boundary | Frozen wire format |
|----------|-------------------|
| `SETTLEMENT_INTERNAL_WIRE_MONEY` | **decimal string** |
| `BILLING_INTERNAL_WIRE_MONEY` | **decimal string** |
| `PAYMENT_INTERNAL_WIRE_MONEY` | **decimal string** |

Implementation approach:

1. **New billing-register internal read handlers** scan DB `NUMERIC` directly into `decimal.Decimal` / emit JSON strings — **do not serialize domain `float64` structs**.
2. Existing public APIs may remain float64 until separate billing migration; cost ingest uses **internal DTOs only**.
3. Where repository already has decimal helpers (`parseSettlementAmountDecimal`, obligation lookup), reuse them.

```text
JSON float → freight-cost decimal = DENY
```

**Finding resolution:** RISK-B-001 resolved in planning — internal read contract is decimal-string-only.

---

## 9. Accrual semantics

```text
FINANCIAL_ACCRUAL (ex-VAT) =
  PLANNED_COST (snapshot.total_amount)
  + SUM(settlement_accessorials.amount WHERE status = APPROVED)
  when currency matches; else fail closed
```

| Rule | Value |
|------|-------|
| PROPOSED in accrual | **NO** |
| REJECTED in accrual | **NO** |
| DISPUTED in accrual | **NO** |
| Unknown planned | accrual unknown (`NULL`) |
| Currency mismatch | fail closed |

Ledger stores **`ACCRUAL_COST_SNAPSHOT`** derived snapshot value, not individual accessorial delta lines.

**Authoritative approved set (F-B-HIGH-003 closed):**

```text
ACCRUAL_APPROVED_SET_SOURCE=billing.settlement_accessorials WHERE status='APPROVED' AND settlement_id=?
ACCRUAL_EVENT_MONEY_SOURCE=EXACT_NUMERIC_APPROVED_ACCESSORIAL_SET
ACCRUAL_USES_LEGACY_FLOAT_AGGREGATE=NO
FREIGHT_COST_FLOAT_INGEST=NO
```

billing-register-service MUST compute accrual event money using **NUMERIC/decimal-safe** path:

```sql
SELECT COALESCE(SUM(amount), 0)::text
FROM billing.settlement_accessorials
WHERE tenant_id = $1 AND settlement_id = $2 AND status = 'APPROVED'
```

→ parse with `shopspring/decimal` inside billing service → emit decimal string in outbox payload.

**Do not trust** legacy `approved_accessorial_total` float aggregate or `::float8` recalculation path for v2.1B financial events.

freight-cost-service MUST NOT query `settlement_accessorials` directly (cross-service DB reads = 0).

**Frozen scenario FC-B-ACC-006:**

- planned = `1000.00`; accessorial A APPROVED `100.00` → accrual `1100.00`
- accessorial A → DISPUTED → accrual `1000.00` (historical `1100.00` journal row remains)

---

## 10. Finality / actual semantics

### 10.1 Current actual

Available when:

- `status IN (APPROVED, DOCUMENTS_READY, READY_FOR_PAYMENT)`
- `open_dispute_count = 0`
- `total_without_vat` present

### 10.2 Final actual

Available only when:

- `status = READY_FOR_PAYMENT`
- `open_dispute_count = 0`

### 10.3 Backward transitions

Example: `APPROVED → DISPUTED`

| Effect | Behavior |
|--------|----------|
| `current_actual_amount` | becomes **NULL** (unknown/unavailable) |
| `final_actual_amount` | remains NULL unless valid final state |
| Historical `cost_entry` rows | **immutable** — remain in journal |
| New source revision | supersedes projection state via cursor policy |

```text
DISPUTE_NULLIFICATION=YES
CANCELLED_NULLIFICATION=YES (actual fields NULL; planned retained)
PROJECTION_NULLIFICATION_SUPPORTED=YES
```

Cancelled settlement: actual NULL — **not zero**.

---

## 11. Forecast exposure

```text
FORECAST_EXPOSURE = PLANNED + SUM(accessorials WHERE status = PROPOSED)
```

| Rule | Value |
|------|-------|
| `FORECAST_IN_COST_LEDGER` | **NO** |
| `FORECAST_PROJECTION_V2_1B` | **DEFER_V2_1C** (compute-on-read or v2.1C persistence) |

Proposed accessorial changes must never be interpreted as financial accrual in v2.1B.

---

## 11A. Delivery identity vs canonical financial fact identity

Architecture review finding **F-B-HIGH-001** (closed): `UNIQUE(tenant_id, source_event_id)` alone cannot prevent the same canonical financial fact from entering the ledger twice when the same fact arrives via **LIVE_OUTBOX** and later via **CANONICAL_REBUILD** with different delivery IDs.

### A. Delivery identity — `source_event_id`

| Field | Value |
|-------|-------|
| Meaning | Identity of **this event delivery** (outbox row / synthetic rebuild delivery) |
| Live value | Canonical outbox row `id` (UUID) |
| Rebuild value | Deterministic delivery UUID (distinct namespace from fact id) |
| Uniqueness | `UNIQUE (tenant_id, source_event_id)` |
| Purpose | Broker replay, crash-after-commit redelivery, outbox idempotency |

```text
DELIVERY_IDENTITY=source_event_id
DELIVERY_UNIQUE_KEY=(tenant_id, source_event_id)
REBUILD_DELIVERY_ID=UUIDv5(NAMESPACE_FREIGHT_COST_REBUILD_DELIVERY, tenant_id|source_fact_id)
```

`event_origin` is **metadata only** — never part of delivery or fact identity.

### B. Canonical financial fact identity — `source_fact_id`

| Field | Value |
|-------|-------|
| Meaning | Stable identity of **one derived canonical financial fact**, independent of arrival path |
| Uniqueness | `UNIQUE (tenant_id, source_fact_id)` |
| Purpose | Live/rebuild semantic dedup; equal-revision duplicate suppression |

**Frozen derivation:**

```text
source_fact_id = UUIDv5(
  NAMESPACE_FREIGHT_COST_CANONICAL_FACT,
  tenant_id
  | source_service
  | source_type
  | source_id
  | source_revision_semantic
  | entry_kind
)
```

| Rule | Value |
|------|-------|
| `SOURCE_FACT_ID_INCLUDES_EVENT_ORIGIN` | **NO** |
| `LIVE_REBUILD_SEMANTIC_DEDUP` | **YES** |
| `CANONICAL_FACT_IDEMPOTENCY_KEY` | `(tenant_id, source_fact_id)` |

**`source_revision_semantic` values:**

| Source | Semantic token |
|--------|----------------|
| TO rate snapshot (immutable) | literal `IMMUTABLE` |
| Settlement financial facts | decimal string of `freight_settlements.version` |
| Billed link facts | decimal string of `freight_settlements.billing_link_revision` |
| Payment paid facts | decimal string of `payment_obligations.version` |

### C. Live / rebuild convergence

For the **same** canonical fact (same settlement revision 7, same `CURRENT_ACTUAL`):

| Path | `source_event_id` | `source_fact_id` |
|------|-------------------|------------------|
| Live outbox | `OUTBOX_UUID_A` | `deterministic(S7\|CURRENT_ACTUAL)` |
| Later rebuild | `REBUILD_DELIVERY_UUID` | **same** `deterministic(S7\|CURRENT_ACTUAL)` |

```text
LIVE_THEN_REBUILD_SAME_FACT_LEDGER_ROWS=1
REBUILD_THEN_LIVE_SAME_FACT_LEDGER_ROWS=1
LIVE_REBUILD_SEMANTIC_DEDUP=YES
```

Consumer rule: if `(tenant_id, source_fact_id)` already exists → **NO-OP** (success, no duplicate row) regardless of `source_event_id` or `event_origin`.

---

## 12. Ledger model

```text
LEDGER_SECOND_SSOT=NO
LEDGER_APPEND_ONLY=YES
LEDGER_AMOUNT_MODE=DERIVED_SNAPSHOT_VALUE
CORRECTION_MODEL=append new entry; optional supersedes_entry_id; never UPDATE/DELETE historical rows
```

Canonical correction remains in source domain (settlement recalc, allocation void, etc.). freight-cost repairs **only** its derived projection/ledger via new events or rebuild.

## 12. Ledger model

| `entry_kind` | Canonical source | Tax basis | Amount nullable | Triggers new entry | Projection field |
|--------------|-------------------|-----------|-----------------|--------------------|------------------|
| `PLANNED_COST_SNAPSHOT` | TO rate snapshot | EX_VAT | NO (zero allowed) | snapshot identity / rebuild | `planned_amount` |
| `ACCRUAL_COST_SNAPSHOT` | derived from planned + approved accessorials | EX_VAT | YES if inputs unknown | settlement/accessorial revision | `accrued_amount` |
| `CURRENT_ACTUAL_COST_SNAPSHOT` | settlement totals + status | EX_VAT | YES when unavailable | settlement status/total revision | `current_actual_amount` |
| `FINAL_ACTUAL_COST_SNAPSHOT` | settlement totals + status | EX_VAT | YES when not final | settlement reaches/loses READY_FOR_PAYMENT | `final_actual_amount` |
| `BILLED_COST_SNAPSHOT` | settlement billing link (frozen ex-VAT) | EX_VAT | YES when unlinked | include / remove / relink | `billing_register_amount` (ex-VAT line snapshot) |
| `PAYABLE_AMOUNT_SNAPSHOT` | register `total_with_vat` | WITH_VAT | YES | register approval/total change | internal payable field (not mixed into ex-VAT variance) |
| `PAID_AMOUNT_SNAPSHOT` | obligation `paid_amount` | WITH_VAT | YES | allocation / void affecting paid | `paid_amount` |

**Availability flag:** when canonical state explicitly invalidates a fact, emit entry with `amount_availability=UNAVAILABLE` (see §21) rather than omitting journal record on rebuild.

---

## 14. Cost entry schema (conceptual — planning only)

Schema: `freight_cost`

Table: `freight_cost.cost_entry`

| Column | Type | Required | Notes |
|--------|------|----------|-------|
| `id` | UUID PK | YES | generated |
| `tenant_id` | UUID | YES | all queries scoped |
| `transport_order_id` | UUID | YES | primary aggregate key |
| `shipment_id` | UUID | NULL | when known from settlement |
| `buyer_company_id` | UUID | YES | denormalized for access control audit |
| `carrier_company_id` | UUID | YES | denormalized |
| `entry_kind` | VARCHAR | YES | enum values §13 |
| `amount` | NUMERIC(18,2) | NULL | NULL when `amount_availability=UNAVAILABLE` |
| `currency_code` | CHAR(3) | YES | |
| `tax_basis` | VARCHAR | YES | EX_VAT / WITH_VAT |
| `amount_availability` | VARCHAR | YES | `AVAILABLE` / `UNAVAILABLE` |
| `source_service` | VARCHAR | YES | e.g. `billing-register-service` |
| `source_type` | VARCHAR | YES | e.g. `FREIGHT_SETTLEMENT` |
| `source_id` | UUID | YES | aggregate id |
| `source_revision` | BIGINT | YES | monotonic per source aggregate / link dimension |
| `source_fact_id` | UUID | YES | canonical financial fact identity (§11A) |
| `source_event_id` | UUID | YES | delivery identity (§11A) |
| `source_occurred_at` | TIMESTAMPTZ | YES | from canonical event |
| `supersedes_entry_id` | UUID | NULL | audit chain (same entry_kind + source aggregate) |
| `event_origin` | VARCHAR | YES | `LIVE_OUTBOX` / `CANONICAL_REBUILD` — **not** part of uniqueness |
| `recorded_at` | TIMESTAMPTZ | YES | ingest time |
| `metadata` | JSONB | NULL | minimal audit; deleted register item id allowed here only |

**Constraints (frozen — no partial origin scope):**

```sql
UNIQUE (tenant_id, source_event_id)
UNIQUE (tenant_id, source_fact_id)
CHECK (amount_availability <> 'AVAILABLE' OR amount IS NOT NULL)
CHECK (amount IS NULL OR amount >= 0)
```

**Removed:** rebuild-only partial unique index on `event_origin`. Origin-independent semantic dedup is enforced by `UNIQUE (tenant_id, source_fact_id)`.

**Consumer insert order:**

1. Compute `source_fact_id`.
2. If `(tenant_id, source_fact_id)` exists → NO-OP success.
3. Else insert `cost_entry` (delivery id may differ from prior attempts).
4. Update projection only per cursor policy (§17).

---

## 15. Source events — one event → one cost entry

Architecture frozen:

```text
UNIQUE (tenant_id, source_event_id)
```

One settlement mutation may affect accrual + current actual + final actual simultaneously.

**Frozen resolution (Option A — preferred):**

```text
SOURCE_EVENT_TO_COST_ENTRY_CARDINALITY=ONE_TO_ONE
```

Canonical owner publishes **one distinct outbox event per financial fact**, each with its own `source_event_id`, sharing the same `source_revision` on the aggregate.

Example settlement approved @ version 12:

| Outbox event | `source_event_id` | `source_fact_id` | entry_kind |
|--------------|-------------------|------------------|------------|
| `freight_settlement.accrual_snapshot.v1` | UUID A | `hash(S\|12\|ACCRUAL)` | ACCRUAL |
| `freight_settlement.current_actual_snapshot.v1` | UUID B | `hash(S\|12\|CURRENT_ACTUAL)` | CURRENT_ACTUAL |
| `freight_settlement.final_actual_snapshot.v1` | UUID C | `hash(S\|12\|FINAL_ACTUAL)` | FINAL_ACTUAL |

Same revision, distinct entry kinds → **three** ledger rows (valid). Same revision + same entry_kind → **one** row (semantic dedup).

**NULLIFICATION:** when actual becomes unavailable (e.g. APPROVED→DISPUTED), emit `CURRENT_ACTUAL` snapshot with `amount_availability=UNAVAILABLE`, `amount=null`, and deterministic `source_fact_id` for that revision/kind — not "no event".

---

## 16. Source revision matrix

| SOURCE | CURRENT_REVISION_FIELD | MONOTONIC | NEW_REVISION_ON | REBUILD_API_EXPOSES |
|--------|------------------------|-----------|-----------------|---------------------|
| TO rate snapshot | **none** (immutable) | N/A | never | `snapshot_id` only |
| Freight settlement | `freight_settlements.version` | YES (optimistic) | any successful mutating TX | YES |
| Settlement accessorial (accrual dimension) | settlement `version` after exact recalc | YES (via settlement) | approve/reject/dispute/recalc | via settlement read |
| Billed link (per settlement) | `freight_settlements.billing_link_revision` | YES | include/remove/relink only | YES |
| Billing register aggregate (payable) | `billing_registers.version` | YES | register recalc/approval | YES |
| Payment obligation | `payment_obligations.version` | YES | allocation/void | YES |
| Payment outbox delivery | outbox row `id` (UUID) | N/A | each publish | delivery id only |

```text
SOURCE_REVISION_TYPE=BIGINT
REVISION_SOURCE_SETTLEMENT=freight_settlements.version
REVISION_SOURCE_ACCESSORIAL=freight_settlements.version (after approved-set change)
REVISION_SOURCE_BILLING_LINK=freight_settlements.billing_link_revision
REVISION_SOURCE_REGISTER_PAYABLE=billing_registers.version
REVISION_SOURCE_PAYMENT=payment_obligations.version
```

**Billing link revision (F-B-HIGH-002 closed — Option C):**

Repository fact: `billing_register_items` row is **DELETE**d on `RemoveSettlement`; item id cannot be canonical identity.

| Field | Frozen value |
|-------|--------------|
| `BILLED_SOURCE_TYPE` | `FREIGHT_SETTLEMENT_BILLING_LINK` |
| `BILLED_SOURCE_ID` | `settlement_id` |
| `BILLED_SOURCE_REVISION` | `freight_settlements.billing_link_revision` |
| `BILLED_LINK_STATE` | `LINKED` \| `UNLINKED` |
| `BILLED_CURSOR_DIMENSION` | `(tenant, transport_order_id, source_service, FREIGHT_SETTLEMENT_BILLING_LINK, settlement_id, BILLED_COST_SNAPSHOT)` |

**New column (billing-register migration):**

`billing.freight_settlements.billing_link_revision BIGINT NOT NULL DEFAULT 0`

Increment in **same transaction** as:

- `billing_register_items` INSERT (include / relink)
- `billing_register_items` DELETE (remove)
- settlement register link columns update

**Options evaluated (frozen — Option C):**

| Option | Identity | Verdict |
|--------|----------|---------|
| A | `source_id=billing_register_item_id`, `revision=billing_registers.version` | **REJECT** — item row deleted on remove; cannot represent UNLINKED rebuild state |
| B | `source_id=settlement_id`, `revision=freight_settlements.version` | **REJECT** — settlement version bumps on non-billing mutations; false billed history |
| C | `source_id=settlement_id`, `revision=billing_link_revision` | **SELECT** — survives remove/relink; monotonic per billing-link dimension only |

`billing_registers.version` remains the revision stream for **register-level payable** facts only — not reused for per-settlement billed link facts.

**Rules:**

| Case | Policy |
|------|--------|
| Same `source_event_id` replay | **NO-OP** (unique constraint) |
| Same `source_fact_id`, different delivery | **NO-OP** (semantic dedup) |
| New fact, higher revision | **ACCEPT** — apply projection if cursor allows |
| New fact, equal revision + same entry_kind | **SEMANTIC DUPLICATE** — same `source_fact_id` → NO-OP |
| New fact, equal revision + different entry_kind | **ALLOWED** — distinct `source_fact_id` |
| New fact, lower revision (unseen) | Journal once; projection **NO_CHANGE** (§17) |
| Timestamp as revision | **DENY** |

---

## 17. Out-of-order semantics

Per source cursor key (§19):

| Condition | `source_fact_id` already exists? | Ledger | Projection |
|-----------|----------------------------------|--------|------------|
| Exact replay (same fact id) | YES | **NO-OP** | **NO_CHANGE** |
| Equal revision + same entry_kind | YES (by derivation) | **NO-OP** | **NO_CHANGE** |
| Equal revision + different entry_kind | NO | INSERT | apply if revision > cursor |
| Higher revision | NO | INSERT | **APPLY** if revision > cursor |
| Lower revision (valid, unseen fact) | NO | **INSERT once** (audit) | **NO_CHANGE** |

```text
REPLAY_POLICY=NO_OP_ON_EXISTING_SOURCE_FACT_ID
EQUAL_REVISION_SAME_ENTRY_KIND=SEMANTIC_DUPLICATE_NO_NEW_LEDGER_ROW
EQUAL_REVISION_DIFFERENT_ENTRY_KIND=ALLOWED_DISTINCT_FACT
HIGHER_REVISION_POLICY=APPLY_IF_REVISION_GREATER_THAN_CURSOR
LOWER_REVISION_LEDGER_POLICY=JOURNAL_VALID_UNSEEN_FACT_ONCE
LOWER_REVISION_PROJECTION_POLICY=NO_CHANGE
OUT_OF_ORDER_PROJECTION_POLICY=APPLY_ONLY_IF_REVISION_GREATER
OUT_OF_ORDER_LEDGER_POLICY=JOURNAL_VALID_UNSEEN_FACT_ONCE
REPLAY_DOUBLE_COUNT=DENY
ONE_SOURCE_EVENT_ONE_COST_ENTRY=YES
ONE_AGGREGATE_REVISION_MULTIPLE_ENTRY_KINDS=YES
```

Live consumer ack **after** DB commit:

```text
EVENT_ACK_BEFORE_DB_COMMIT=NO
```

---

## 18. Projection model

Table: `freight_cost.cost_summary_projection`

| Field | Persist v2.1B | Notes |
|-------|---------------|-------|
| `tenant_id`, `transport_order_id` | YES | PK / unique |
| `buyer_company_id`, `carrier_company_id` | YES | |
| `currency_code` | YES | single comparable currency for ex-VAT dims |
| `planned_amount` | YES | |
| `accrued_amount` | YES | |
| `forecast_exposure` | DEFER v2.1C | |
| `current_actual_amount` | YES | |
| `final_actual_amount` | YES | |
| `billing_register_amount` | YES | ex-VAT billed line snapshot |
| `payable_amount` | YES | WITH_VAT — separate field |
| `paid_amount` | YES | WITH_VAT |
| `billing_reconciliation_status` | YES | MATCH/MISMATCH/UNLINKED |
| `financial_finality` | YES | derived enum |
| `data_stage` | YES | highest stage + `sources_available[]` |
| `current_variance_amount` | DEFER v2.1C | |
| `final_variance_amount` | DEFER v2.1C | |

**Data stage evolution:** use **`data_stage` (highest milestone) + `sources_available[]`** rather than a single linear enum that breaks when paid exists but actual later disputed.

Suggested `data_stage` precedence (highest wins for display):

`PAID` > `BILLING_LINKED` > `FINAL_ACTUAL_AVAILABLE` > `CURRENT_ACTUAL_AVAILABLE` > `ACCRUAL_AVAILABLE` > `PLANNED_ONLY`

`financial_finality` remains settlement-driven (v2.1A domain functions).

---

## 19. Source cursor

Table: `freight_cost.source_cursor`

| Column | Purpose |
|--------|---------|
| `tenant_id` | scope |
| `transport_order_id` | scope |
| `source_service` | e.g. billing-register-service |
| `source_type` | e.g. FREIGHT_SETTLEMENT |
| `source_id` | aggregate UUID |
| `entry_kind` | financial dimension |
| `last_source_revision` | BIGINT |
| `last_source_event_id` | UUID |
| `last_cost_entry_id` | UUID |
| `updated_at` | TIMESTAMPTZ |

**Primary key:** `(tenant_id, transport_order_id, source_service, source_type, source_id, entry_kind)`

```text
SOURCE_REVISION_STREAMS_INDEPENDENT=YES
```

Settlement revision stream ≠ payment obligation stream ≠ TO snapshot identity.

---

## 20. NULL transitions

When settlement moves `APPROVED → DISPUTED`, consumer emits `CURRENT_ACTUAL_COST_SNAPSHOT` with:

```json
{
  "amount_availability": "UNAVAILABLE",
  "amount": null,
  "source_revision": 8
}
```

Projection sets `current_actual_amount = NULL` while prior journal row remains.

Distinguish:

| State | Meaning |
|-------|---------|
| NULL — source not yet loaded | pre-ingest / rebuild partial |
| NULL — explicit UNAVAILABLE entry | canonical invalidation |

Use `sources_available` + latest `amount_availability` on cursor/entry metadata.

---

## 21. Supersedes chain

```text
SUPERSEDES_USED_FOR_AUDIT=YES
SUPERSEDES_USED_AS_SSOT=NO
```

On ingest, optional `supersedes_entry_id` = latest prior **`cost_entry.id`** for same semantic dimension:

`(tenant_id, transport_order_id, entry_kind, source_service, source_type, source_id)`

Audit navigation only — **not** current-state authority (projection + cursor wins).

---

## 22. Event origin

```text
EVENT_ORIGIN_FIELD_REQUIRED=YES
```

| Value | Meaning |
|-------|---------|
| `LIVE_OUTBOX` | Kafka/HTTP consumer received canonical outbox event |
| `CANONICAL_REBUILD` | synthetic deterministic ingest from rebuild API |

Rebuild must not fabricate live outbox IDs.

---

## 23. Planned snapshot without TO outbox

TO snapshot is immutable; no revision stream.

**Frozen fact identity:**

| Field | Value |
|-------|-------|
| `source_id` | `rate_snapshot_id` |
| `source_revision_semantic` | `IMMUTABLE` |
| `entry_kind` | `PLANNED_COST_SNAPSHOT` |
| `source_fact_id` | `UUIDv5(NAMESPACE, tenant\|transport-order-service\|TO_RATE_SNAPSHOT\|snapshot_id\|IMMUTABLE\|PLANNED)` |

**Delivery identity:**

| Path | `source_event_id` |
|------|-------------------|
| Rebuild | `UUIDv5(NAMESPACE_FREIGHT_COST_REBUILD_DELIVERY, tenant\|source_fact_id)` |

- `source_revision` column stores `1` (constant semantic marker in DB row)
- Repeated rebuild of unchanged snapshot → **one** ledger fact (same `source_fact_id`)

Live TO outbox: **not required** in v2.1B. First planned fact may arrive via rebuild or lazy ingest.

```text
PLANNED_IMMUTABLE_FACT_ID=YES
PLANNED_SOURCE_REVISION_STRATEGY=IMMUTABLE
PLANNED_REBUILD_DELIVERY_ID=deterministic per source_fact_id
```

---

## 24. Billing-register transactional outbox (new)

New table (conceptual): `billing.freight_cost_outbox` — follow payment outbox column shape.

**Mutations requiring same-TX outbox insert:**

| Mutation | EMIT | Event kinds (distinct IDs) |
|----------|------|----------------------------|
| Settlement created | YES | accrual + actual snapshots (mostly UNAVAILABLE) |
| Settlement status → APPROVED/DOCUMENTS_READY/READY_FOR_PAYMENT | YES | accrual, current_actual, final_actual |
| Settlement → DISPUTED / CANCELLED | YES | actual snapshots → UNAVAILABLE |
| Settlement totals recalculated | YES | accrual + actual snapshots |
| Accessorial approved/rejected/disputed | YES | accrual (+ actual if settlement totals changed) |
| Include settlement in register | YES | billed snapshot (LINKED) |
| Remove register item | YES | billed snapshot (UNLINKED) |
| Register totals recalculated | YES | payable snapshot |

**Propose accessorial:** does **NOT** emit v2.1B accrual event (forecast deferred v2.1C; approved set unchanged).

```text
TRANSACTIONAL_OUTBOX=YES
REGISTER_REMOVE_TOMBSTONE_SEMANTICS=YES
REGISTER_REMOVE_REBUILDABLE=YES
```

---

## 24A. Billing link events (frozen)

**LINKED event** (same TX as include):

- `billing_link_revision` incremented
- `billing_link_state=LINKED`
- payload includes: `billing_register_id`, `billing_register_item_id`, `amount_without_vat` (decimal string), `currency_code`, `tax_basis=EX_VAT`

**UNLINKED event** (same TX as remove):

- `billing_link_revision` incremented
- `billing_link_state=UNLINKED`
- `amount=null`, `amount_availability=UNAVAILABLE`
- former `billing_register_item_id` MAY appear in audit `metadata` only — **not** in `source_id` / `source_fact_id`

**Rebuild after item deletion:** internal settlement read returns `billing_link_state=UNLINKED` + last known link revision — reconstructs UNLINKED without requiring live item row.

**Remove → relink:** each include/remove increments `billing_link_revision` monotonically; cursor advances; historical LINKED rows remain immutable.

---

## 24B. Accessorial mutation event matrix (repository-verified)

| MUTATION | OLD→NEW STATUS | CHANGES APPROVED SET | SETTLEMENT VERSION BUMP | ACCRUAL EVENT | CURRENT_ACTUAL EVENT | FINAL_ACTUAL EVENT | FORECAST | v2.1B |
|----------|----------------|----------------------|-------------------------|---------------|----------------------|--------------------|---------|-------|
| ProposeAccessorial | → PROPOSED | NO | NO | NO | NO | NO | defer v2.1C | NO |
| ReviewAccessorial approve | PROPOSED→APPROVED | YES | YES (+ exact recalc) | YES | if actual-available state | if final state | defer | YES |
| ReviewAccessorial reject | PROPOSED→REJECTED | NO | YES (recalc no-op on sum) | NO | NO | NO | defer | NO |
| RaiseDispute (accessorial) | *→DISPUTED | YES (removes from APPROVED set) | YES (status) | YES (exact approved set) | YES (→ UNAVAILABLE if disputed) | YES if needed | defer | YES |
| RaiseDispute (settlement-only) | status→DISPUTED | NO | YES | if totals changed | YES UNAVAILABLE | YES UNAVAILABLE | defer | YES |
| ResolveDispute | dispute closed | maybe | YES when returning to review | when approved set changes | when re-enabled | when re-enabled | defer | YES |
| recalculateSettlementTotals | — | — | YES | when emitted by caller TX | when emitted | when emitted | defer | YES |

Repository evidence: `ReviewAccessorial` calls `recalculateSettlementTotals`; `RaiseDispute` with accessorial marks DISPUTED **without** recalc today — v2.1B hardening required (§24C).

---

## 24C. Settlement exact-recalc hardening (v2.1B billing-register)

```text
RECALCULATE_EXACT_TOTALS_BEFORE_ACTUAL_REENABLE=YES
```

**Required v2.1B changes in billing-register-service:**

1. Replace `COALESCE(SUM(amount),0)::float8` in `recalculateSettlementTotals` with NUMERIC → decimal string path.
2. On accessorial APPROVED→DISPUTED (RaiseDispute), invoke exact recalc **before** emitting financial outbox snapshots in the **same transaction**.
3. Before any transition to APPROVED / DOCUMENTS_READY / READY_FOR_PAYMENT after approved-set membership changed, enforce exact recalc + version bump + snapshot emission in **one transaction**.

This prevents re-enabled actual amounts from using stale pre-dispute totals (FC-B-ACT-006).

---

## 25. Event payload contract

```text
EVENT_PAYLOAD_MODE=VERSIONED_FINANCIAL_SNAPSHOT
LIVE_EVENT_REQUIRES_REVISION_MATCHED_PAYLOAD=YES
```

Payload MUST include exact financial facts for **that revision**, not thin notification-only:

```json
{
  "event_id": "uuid",
  "event_type": "freight_settlement.current_actual_snapshot.v1",
  "schema_version": 1,
  "tenant_id": "uuid",
  "transport_order_id": "uuid",
  "source_service": "billing-register-service",
  "source_type": "FREIGHT_SETTLEMENT",
  "source_id": "uuid",
  "source_revision": 7,
  "occurred_at": "2026-08-21T12:00:00Z",
  "currency_code": "RUB",
  "tax_basis": "EX_VAT",
  "amount_availability": "AVAILABLE",
  "amount": "150000.00",
  "settlement_status": "APPROVED",
  "open_dispute_count": 0
}
```

All money fields: **decimal strings**. No JSON numbers for money.

---

## 26. Event type naming (frozen candidates)

Follow `{aggregate}.{fact_snapshot}.v{schema}` pattern:

| Event type | entry_kind |
|------------|------------|
| `freight_settlement.accrual_snapshot.v1` | ACCRUAL |
| `freight_settlement.current_actual_snapshot.v1` | CURRENT_ACTUAL |
| `freight_settlement.final_actual_snapshot.v1` | FINAL_ACTUAL |
| `billing_register.settlement_billing_link_snapshot.v1` | BILLED |
| `billing_register.payable_snapshot.v1` | PAYABLE |
| `payment_obligation.paid_snapshot.v1` | PAID (new — extend payment outbox) |

Existing `payment_obligation.paid` remains for billing sync; cost consumer may subscribe to extended event or derive paid snapshot from obligation read — **prefer new typed snapshot event with decimal payload** to avoid thin payload race (RISK-B-004).

---

## 27. Payment outbox reuse

```text
PAYMENT_OUTBOX_REUSE=PARTIAL
```

**Additive event (frozen):** `payment_obligation.paid_snapshot.v1`

Minimum payload:

- `tenant_id`, `obligation_id`, `register_id`
- `source_revision` (= `payment_obligations.version` at publish TX)
- `paid_amount`, `obligation_amount` (decimal strings)
- `currency_code`, `tax_basis=WITH_VAT`, `occurred_at`
- unique `source_event_id` (= outbox row id)

**Paid fact identity:**

```text
source_id=payment_obligation_id
source_revision=payment_obligations.version
entry_kind=PAID
source_fact_id=UUIDv5(NAMESPACE, tenant|payment-service|PAYMENT_OBLIGATION|obligation_id|version|PAID)
```

Do **not** derive historical payment revision by fetching "latest" after the event.

If PAYABLE is a separate entry_kind, emit separate event/`source_event_id` as required.

| Today | v2.1B delta |
|-------|-------------|
| `payment_obligation.paid` thin payload | Add `payment_obligation.paid_snapshot.v1` with revision + decimal paid/obligated amounts |
| Unique `(tenant, event_type, aggregate_id)` | New event types get distinct type strings |
| HTTP publish to billing | Keep; cost consumer is separate worker in freight-cost-service |

---

## 28. Internal read APIs (planning contracts)

### 28.1 Transport (exists — v2.1A)

```http
GET /internal/v1/transport-orders/{transportOrderId}/rate-snapshot
X-Internal-Service-Token
X-Tenant-ID
```

### 28.2 Settlement (new — billing-register-service)

```http
GET /internal/v1/freight-settlements/by-transport-order/{transportOrderId}
X-Internal-Service-Token
X-Tenant-ID
```

Response (decimal strings):

- `settlement_id`, `transport_order_id`, `tenant_id`, `buyer_company_id`, `carrier_company_id`
- `status`, `open_dispute_count`, `version` (settlement source_revision)
- `billing_link_revision`, `billing_link_state` (`LINKED` \| `UNLINKED`)
- `currency_code`
- `base_freight_amount`, `accrual_amount_ex_vat` (exact decimal from approved set — **not** legacy float aggregate)
- `total_without_vat` (EX_VAT, for actual snapshots)
- `rate_snapshot_id`
- `updated_at`

404 cross-tenant scoped.

### 28.3 Settlement billing link (new)

```http
GET /internal/v1/freight-settlements/{settlementId}/billing-link
```

Returns `billing_link_revision`, `billing_link_state` (`LINKED` \| `UNLINKED`), frozen ex-VAT amount (decimal string when LINKED), `billing_register_id`, `currency_code`, `tax_basis=EX_VAT`. Rebuildable after register item deletion.

### 28.4 Payment (new)

```http
GET /internal/v1/payment-obligations/by-billing-register/{billingRegisterId}
```

Returns obligation id, `paid_amount`, `original_amount` (WITH_VAT strings), currency, status, version.

**Cross-service DB reads from freight-cost:** **0** — HTTP only.

---

## 29. Rebuild

```text
REBUILD_ROOT=canonical domain read APIs
REBUILD_GRANULARITY=RebuildTransportOrder(tenantID, transportOrderID)
REBUILD_IDEMPOTENT=YES
LIVE_REBUILD_PROJECTION_EQUIVALENCE=YES
```

Process:

1. Call transport, settlement, billing, payment internal reads.
2. Normalize to source facts @ current canonical revision.
3. Derive deterministic rebuild `source_event_id` per entry kind.
4. Upsert journal (idempotent) + recompute projection from cursors.
5. Run reconciliation compare.

Historical journal entry **count** may differ from live path; **current projection must match**.

Internal admin route (S2S only):

```http
POST /internal/v1/freight-cost/transport-orders/{transportOrderId}/rebuild
```

---

## 30. Billing reconciliation

Carry forward v2.1A pure functions:

| Status | Rule |
|--------|------|
| UNLINKED | no billing link |
| MATCH | linked, equal ex-VAT amounts, same currency, no open dispute |
| MISMATCH | linked + (amount/currency differ OR dispute) |

Billed line is **frozen** at include — settlement changes after include → MISMATCH, not overwrite.

Register removal → UNLINKED; historical BILLED entry remains.

---

## 31. Security

Carry forward v2.1A:

- S2S token authenticates caller class only
- Actor headers trusted after token gate
- Wrong tenant → 404
- Same-tenant wrong company → 403
- PLATFORM_ADMIN → deny

Persistence:

- Every table includes `tenant_id`
- Repository methods require `tenantID` parameter — no naked ID lookups
- Carrier projection masks buyer-internal fields even when persisted

```text
RAW_LEDGER_PUBLIC_API=NO
RAW_LEDGER_CARRIER_API=NO
CROSS_SERVICE_DB_READS=0
```

---

## 32. Observability

Metrics (no money/tenant/TO/event_id in labels):

- `freight_cost_events_received_total{event_type,result}`
- `freight_cost_events_applied_total{entry_kind,result}`
- `freight_cost_events_replayed_total`
- `freight_cost_events_out_of_order_total`
- `freight_cost_events_invalid_total{reason}`
- `freight_cost_projection_updates_total{entry_kind}`
- `freight_cost_rebuild_total{result}`
- `freight_cost_reconciliation_mismatch_total`

---

## 33. Failure / DLQ strategy

Align with payment/shipment outbox retry:

| Failure | Handling |
|---------|----------|
| Malformed envelope / invalid decimal | non-retryable; quarantine + metric |
| Currency mismatch | non-retryable |
| Lower revision (unseen valid fact) | journal once; projection unchanged |
| DB unavailable | retry with backoff |
| Unknown schema version | non-retryable until upgraded |

If platform DLQ topic exists for shipment — reuse pattern; else document FAILED outbox row + manual replay tool.

---

## 34. Migration plan (planning)

| Service | Next # | Purpose | Tables | Rollback |
|---------|--------|---------|--------|----------|
| freight-cost-service | 000054 | `freight_cost` schema | `cost_entry`, `source_cursor`, `cost_summary_projection` | DROP SCHEMA |
| billing-register-service | 000054/055 | cost outbox + billing_link_revision | `billing.freight_cost_outbox`, `freight_settlements.billing_link_revision` | DROP COLUMN/TABLE |
| payment-service | 000048 optional | paid snapshot event support | none if payload-only | n/a |

**Lock/risk:** freight_cost indexes on `(tenant_id, transport_order_id)` — low contention expected; outbox insert colocated with settlement TX — brief row locks.

---

## 35. Index plan

**cost_entry:**

- UNIQUE `(tenant_id, source_event_id)`
- UNIQUE `(tenant_id, source_fact_id)`
- INDEX `(tenant_id, transport_order_id, recorded_at DESC)`
- INDEX `(tenant_id, entry_kind, transport_order_id)`
- INDEX `(tenant_id, source_service, source_type, source_id, source_revision)`

**cost_summary_projection:**

- UNIQUE `(tenant_id, transport_order_id)`
- INDEX `(tenant_id, buyer_company_id)`, `(tenant_id, carrier_company_id)`

**source_cursor:**

- PRIMARY KEY as §19

---

## 36. Retention

```text
COST_LEDGER_RETENTION=NO_AUTO_DELETE_V2_1B
```

Financial journal append-only indefinitely in v2.1B. Outbox follows existing archive conventions.

---

## 37. Test matrix (frozen IDs)

| Family | IDs | Count |
|--------|-----|-------|
| FC-B-LED | 001–013 | 13 |
| FC-B-MON | 001–005 | 5 |
| FC-B-ACC | 001–007 | 7 |
| FC-B-ACT | 001–006 | 6 |
| FC-B-BIL | 001–008 | 8 |
| FC-B-PAY | 001–003 | 3 |
| FC-B-RBL | 001–006 | 6 |
| FC-B-SEC | 001–004 | 4 |
| FC-B-OUT | 001–005 | 5 |
| **Total** | | **57** |

**New remediation tests:**

| ID | Scenario |
|----|----------|
| FC-B-LED-009 | live event then rebuild same canonical fact → exactly one `cost_entry` |
| FC-B-LED-010 | rebuild then live same canonical fact → exactly one `cost_entry` |
| FC-B-LED-011 | different `source_event_id` + same `source_fact_id` → no duplicate |
| FC-B-LED-012 | equal revision + same entry_kind → semantic duplicate / no new row |
| FC-B-LED-013 | equal revision + different entry_kind → distinct facts allowed |
| FC-B-RBL-006 | unchanged rebuild after successful live ingestion adds no journal row |
| FC-B-BIL-006 | register removal → explicit UNLINKED state; history retained |
| FC-B-BIL-007 | remove → relink keeps monotonic `billing_link_revision` |
| FC-B-BIL-008 | rebuild after item deletion reconstructs UNLINKED |
| FC-B-ACC-006 | APPROVED accessorial → DISPUTED removes from accrual |
| FC-B-ACC-007 | accrual event uses exact decimal approved-set total |
| FC-B-ACT-006 | actual cannot re-enable with stale pre-dispute totals |
| FC-B-OUT-005 | two origins (LIVE + REBUILD) for same fact cannot duplicate ledger |

---

## 38. Cross-service change matrix

| SERVICE | AREA | CHANGE | WHY | CANONICAL/DERIVED | MIGRATION | EVENT | HTTP | RISK |
|---------|------|--------|-----|-------------------|-----------|-------|------|------|
| freight-cost-service | schema/repo/consumer | NEW persistence + consumer + rebuild | derived ledger home | DERIVED | YES | consume | extend internal read output | HIGH |
| billing-register-service | outbox + internal reads | publish + decimal-safe reads | canonical fact source | CANONICAL | YES | emit | YES internal | HIGH |
| payment-service | outbox payload + internal read | paid snapshot events | canonical paid facts | CANONICAL | MAYBE | emit extend | YES internal | MEDIUM |
| transport-order-service | — | **none preferred** | v2.1A endpoint sufficient | CANONICAL | NO | NO | existing | LOW |
| api-gateway | — | NO | — | — | NO | NO | NO | — |
| frontend | — | NO | — | — | NO | NO | NO | — |

---

## 39. Implementation order

1. Decimal-safe internal read DTOs (billing settlement/billing/payment)
2. Source revision contract + internal API responses expose `version`
3. billing-register transactional outbox + publisher
4. freight_cost schema migrations
5. cost_entry + source_cursor + projection repositories
6. Consumer envelope validation + idempotency insert
7. Accrual + actual event paths
8. Billed + payable event paths
9. Payment paid snapshot path
10. Canonical rebuild providers (HTTP clients)
11. Rebuild service + internal rebuild route
12. Reconciliation + drift detection
13. Populate v2.1A cost summary HTTP from projection
14. Security + integration tests
15. CI (postgres) + implementation doc

---

## 40. Risk register

| ID | Sev | Risk | Mitigation | Test |
|----|-----|------|------------|------|
| RISK-B-001 | HIGH | billing float64 corrupts decimal boundary | internal read scans NUMERIC → decimal string | FC-B-MON-002 |
| RISK-B-002 | HIGH | missing monotonic revision | use settlement.version; add if gap found | FC-B-LED-003 |
| RISK-B-003 | HIGH | one mutation → multiple facts vs unique event id | Option A: distinct events per fact | FC-B-OUT-004 |
| RISK-B-004 | HIGH | thin event + advanced canonical state | VERSIONED_FINANCIAL_SNAPSHOT payload | FC-B-OUT-001 |
| RISK-B-005 | HIGH | rebuild duplicate journal rows | deterministic rebuild event ids | FC-B-RBL-002 |
| RISK-B-006 | HIGH | known→NULL not representable | amount_availability UNAVAILABLE | FC-B-LED-008 |
| RISK-B-007 | HIGH | out-of-order regresses projection | cursor APPLY_ONLY_IF_GREATER | FC-B-LED-004 |
| RISK-B-008 | HIGH | ledger becomes SSOT | rebuild from canonical; mismatch flags | FC-B-RBL-004 |
| RISK-B-009 | MED | billed stale vs settlement | frozen billed + MISMATCH | FC-B-BIL-003 |
| RISK-B-010 | MED | WITH_VAT vs EX_VAT compare | tax_basis on every entry | FC-B-MON-005 |
| RISK-B-011 | MED | forecast treated as accrual | FORECAST_IN_COST_LEDGER=NO | FC-B-ACC-002 |
| RISK-B-012 | MED | carrier sees buyer analytics | view filter on read | FC-B-SEC-003 |

**All HIGH resolved in this plan.**

---

## 41. Acceptance gates (v2.1B implementation)

| Gate | Required |
|------|----------|
| FREIGHT_COST_SCHEMA | PASS |
| COST_ENTRY_APPEND_ONLY | PASS |
| DERIVED_LEDGER_NOT_SSOT | PASS |
| DECIMAL_SAFE_SOURCE_BOUNDARIES | PASS |
| SOURCE_EVENT_IDEMPOTENCY | PASS |
| SOURCE_REVISION_MONOTONICITY | PASS |
| OUT_OF_ORDER_POLICY | PASS |
| TRANSACTIONAL_OUTBOX | PASS |
| PAYMENT_OUTBOX_REUSE_OR_MINIMAL_DELTA | PASS |
| ACCRUAL_PERSISTENCE | PASS |
| CURRENT_ACTUAL_PROJECTION | PASS |
| FINAL_ACTUAL_PROJECTION | PASS |
| BILLING_RECONCILIATION | PASS |
| REBUILD_FROM_CANONICAL_APIS | PASS |
| LIVE_REBUILD_EQUIVALENCE | PASS |
| CROSS_SERVICE_DB_READS | 0 |
| CROSS_TENANT_ACCESS | DENY |
| PUBLIC_API_ADDED | NO |
| FRONTEND_CHANGED | NO |

---

## 42. Review questions (answers)

| # | Question | Answer |
|---|----------|--------|
| 1 | What rows in cost_entry? | One row per ingested financial snapshot fact with source metadata |
| 2 | Snapshot values or deltas? | **DERIVED_SNAPSHOT_VALUE** |
| 3 | One settlement → several facts? | **Multiple outbox events** (Option A), one entry each |
| 4 | What is source_event_id? | **Delivery id** — outbox row UUID (live) or deterministic rebuild delivery UUID |
| 4b | What is source_fact_id? | **Canonical financial fact id** — UUIDv5(tenant, service, type, id, revision_semantic, entry_kind) |
| 5 | How is source_revision produced? | Domain aggregate `version` or `billing_link_revision` (BIGINT) |
| 6 | Duplicate delivery? | Unique `(tenant, source_event_id)` → no-op |
| 6b | Duplicate canonical fact? | Unique `(tenant, source_fact_id)` → no-op |
| 7 | Lower revision unseen fact? | Journal once; projection NO_CHANGE |
| 8 | Equal revision, same entry_kind? | Semantic duplicate — no new ledger row |
| 8b | Equal revision, different entry_kind? | Allowed — distinct source_fact_id |
| 9 | Actual becomes NULL? | `amount_availability=UNAVAILABLE` entry + projection NULL |
| 10 | Planned without TO outbox? | Rebuild deterministic source_fact_id from snapshot_id + IMMUTABLE |
| 11 | Rebuild idempotent? | YES — source_fact_id dedup |
| 12 | Rebuild root? | Canonical HTTP read APIs only |
| 13 | Live vs rebuild converge? | YES — one row per canonical fact; equivalent projection |
| 14 | Decimal despite float64? | Internal reads bypass float64 wire |
| 15 | Which mutations publish outbox? | §24 table |
| 16 | Payment outbox sufficient? | PARTIAL — extend with paid snapshot event |
| 17 | Forecast persisted? | DEFER v2.1C; not in ledger |
| 18 | Billed vs settlement? | Frozen billed line + live settlement → reconciliation |
| 19 | WITH_VAT separate? | YES — tax_basis + separate projection fields |
| 20 | Carrier sees accrual? | NO — masked on read |
| 21 | Cross-service SQL? | NO |
| 22 | Ledger overwrite canonical? | NO |
| 23 | Replay double count? | DENY |
| 24 | Out-of-order regress? | DENY via cursor |

---

## 43. Decision table (frozen)

| Decision | Value |
|----------|-------|
| `V2_1B_DATABASE_REQUIRED` | **YES** |
| `LEDGER_AUTHORITY` | DERIVED_EVENT_JOURNAL |
| `LEDGER_SECOND_SSOT` | NO |
| `LEDGER_AMOUNT_MODE` | DERIVED_SNAPSHOT_VALUE |
| `LEDGER_APPEND_ONLY` | YES |
| `DELIVERY_IDENTITY` | `source_event_id` |
| `DELIVERY_UNIQUE_KEY` | `(tenant_id, source_event_id)` |
| `CANONICAL_FACT_IDENTITY` | `source_fact_id` |
| `CANONICAL_FACT_UNIQUE_KEY` | `(tenant_id, source_fact_id)` |
| `SOURCE_FACT_ID_INCLUDES_EVENT_ORIGIN` | **NO** |
| `LIVE_REBUILD_SEMANTIC_DEDUP` | **YES** |
| `COST_ENTRY_UNIQUE_EVENT_KEY` | `(tenant_id, source_event_id)` |
| `COST_ENTRY_UNIQUE_FACT_KEY` | `(tenant_id, source_fact_id)` |
| `SOURCE_EVENT_TO_COST_ENTRY_CARDINALITY` | ONE_TO_ONE (Option A) |
| `SOURCE_REVISION_TYPE` | BIGINT |
| `OUT_OF_ORDER_LEDGER_POLICY` | JOURNAL_VALID_UNSEEN_FACT_ONCE |
| `OUT_OF_ORDER_PROJECTION_POLICY` | APPLY_ONLY_IF_REVISION_GREATER |
| `EQUAL_REVISION_SAME_ENTRY_KIND` | SEMANTIC_DUPLICATE_NO_NEW_LEDGER_ROW |
| `EQUAL_REVISION_DIFFERENT_ENTRY_KIND` | ALLOWED_DISTINCT_FACT |
| `PLANNED_REBUILD_FACT_ID_STRATEGY` | UUIDv5 with IMMUTABLE revision semantic |
| `REBUILD_DELIVERY_ID_STRATEGY` | UUIDv5(NAMESPACE_REBUILD_DELIVERY, tenant\|source_fact_id) |
| `EVENT_ORIGIN_FIELD_REQUIRED` | YES (metadata only) |
| `SETTLEMENT_INTERNAL_WIRE_MONEY` | decimal string |
| `BILLING_INTERNAL_WIRE_MONEY` | decimal string |
| `PAYMENT_INTERNAL_WIRE_MONEY` | decimal string |
| `BILLED_SOURCE_TYPE` | FREIGHT_SETTLEMENT_BILLING_LINK |
| `BILLED_SOURCE_ID` | settlement_id |
| `BILLED_SOURCE_REVISION` | freight_settlements.billing_link_revision |
| `REGISTER_REMOVE_TOMBSTONE` | YES |
| `ACCRUAL_APPROVED_SET_SOURCE` | billing.settlement_accessorials WHERE status=APPROVED |
| `ACCRUAL_EVENT_MONEY_SOURCE` | EXACT_NUMERIC |
| `ACCRUAL_USES_LEGACY_FLOAT_AGGREGATE` | NO |
| `RECALCULATE_BEFORE_ACTUAL_REENABLE` | YES |
| `TRANSACTIONAL_OUTBOX` | YES (billing-register) |
| `EVENT_PAYLOAD_MODE` | VERSIONED_FINANCIAL_SNAPSHOT |
| `FORECAST_IN_COST_LEDGER` | NO |
| `FORECAST_PROJECTION_V2_1B` | DEFER_V2_1C |
| `CURRENT_ACTUAL_SOURCE` | billing-register settlement |
| `FINAL_ACTUAL_SOURCE` | billing-register settlement |
| `BILLED_SOURCE` | settlement billing link snapshot |
| `PAID_SOURCE` | payment obligation |
| `PAYMENT_OUTBOX_REUSE` | PARTIAL |
| `PAID_SNAPSHOT_EVENT` | payment_obligation.paid_snapshot.v1 |
| `PAYMENT_SOURCE_REVISION` | payment_obligations.version |
| `PAYMENT_MONEY_WIRE` | decimal string |
| `REBUILD_ROOT` | canonical read APIs |
| `LIVE_REBUILD_PROJECTION_EQUIVALENCE` | YES |
| `CROSS_SERVICE_DB_READS` | 0 |
| `PUBLIC_API` | NO (v2.1E) |
| `FRONTEND` | NO |
| `FX` | NO |

---

## 44. Findings register (final independent review)

### Closed HIGH (remediation pass)

| ID | Finding | Resolution |
|----|---------|------------|
| **F-B-HIGH-001** | LIVE_OUTBOX and CANONICAL_REBUILD used different `source_event_id`; same canonical fact could duplicate ledger | §11A — `source_fact_id` + `UNIQUE(tenant_id, source_fact_id)`; live/rebuild semantic dedup |
| **F-B-HIGH-002** | Billing revision unfrozen; register item deleted on remove | §16, §24A — `billing_link_revision` on settlement; `BILLED_SOURCE_ID=settlement_id` |
| **F-B-HIGH-003** | Dispute path alters approved set without exact-decimal accrual contract | §9, §24B/C — exact NUMERIC approved-set sum; recalc hardening on dispute |

### Closed MEDIUM

| ID | Finding | Resolution |
|----|---------|------------|
| **F-B-MED-001** | Equal/lower revision conflated delivery with semantic fact identity | §11A, §17 — separate keys; equal same-kind = semantic duplicate; lower unseen = journal once |

### Open items

| Class | Count | Notes |
|-------|-------|-------|
| BLOCKER | 0 | |
| HIGH | 0 | F-B-HIGH-001..003 closed |
| MEDIUM | 0 | F-B-MED-001 closed |
| LOW | 1 | charge_code / variance reason semantics → v2.1C (architecture OQ-005) |
| NIT | 0 | |

```text
OPEN_BLOCKER=0
OPEN_HIGH=0
OPEN_MEDIUM=0
```

---

## 45. v2.1C handoff

v2.1B delivers persisted accrual/actual/billed/paid projection + ledger.

v2.1C adds:

- current/final **variance** computation + API fields
- accessorial charge_code semantic classification
- forecast_exposure persistence (optional)
- reconciliation background jobs / drift automation
- richer analytics dimensions

Do not implement v2.1C in v2.1B PRs.

---

## 46. Final planning review

| Field | Value |
|-------|-------|
| `V2_1A_STATUS` | CLOSED @ `1cc3ca6` |
| `V2_1B_PLAN_STATUS` | **READY_FOR_FINAL_REVIEW** |
| `V2_1B_IMPLEMENTATION_AUTHORIZATION` | **NOT_AUTHORIZED** (requires planning PR merge + final review) |
| Runtime in this PR | **NO** |
| Migrations in this PR | **NO** |

**Planning approval criteria met:** delivery vs canonical fact identity frozen; billing link revision frozen; exact-decimal accrual frozen; live/rebuild dedup; out-of-order policy unambiguous; NULL transitions explicit; 57 FC-B-* tests planned.

---

## Document control

| Field | Value |
|-------|-------|
| Author | v2.1B planning agent |
| Base SHA | `1cc3ca6c01e981b5fb220811a0f2701b9cf1a4c0` |
| Review status | Final remediation — ready for independent review |
| Runtime changes | **NONE** |
