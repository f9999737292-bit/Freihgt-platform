# FREIGHT COST MANAGEMENT v2.1 — Architecture & Current-State Discovery

**Status:** Architecture freeze (documentation only)  
**Branch:** `arch/freight-cost-management-v2.1`  
**Base SHA:** `ea7721c188b4cf2e10f40f1a8a4dd5e57104a2be` (v2.0E merged)  
**Date:** 2026-08-21

---

## 1. Executive Summary

Freight Cost Management v2.1 answers, truthfully and auditably:

> What was freight expected to cost, what is currently accrued, what was finally settled, what was invoiced, what was paid, and why are those values different?

The repository already contains a **partial but strong commercial chain** from v2.0A–E and v1.7–v1.9:

```text
contract-rate / RFx resolve
  → immutable Transport Order pricing snapshot (v2.0C)
  → shipment execution (no native price)
  → freight settlement + accessorials (billing-register-service)
  → billing register / closing documents (v1.8)
  → payment obligations / allocations (v1.9)
```

**What is missing for v2.1:**

| Capability | Current state |
|------------|---------------|
| Unified planned vs actual cost view | **NOT_FOUND** |
| Accrual / estimated liability before settlement finality | **NOT_FOUND** |
| Variance analytics with controlled reasons | **NOT_FOUND** |
| Cost ledger / projection rebuild model | **NOT_FOUND** |
| Dedicated cost analytics API / workspace | **NOT_FOUND** |

**Recommended architecture:** **HYBRID** bounded context — introduce a dedicated **`freight-cost-service`** that owns **derived cost projections and an append-only cost ledger**, while **all canonical financial facts remain in existing domain services** (transport-order snapshot, settlement, billing register, payment). v2.1 must **not** create a second settlement SSOT or duplicate immutable snapshot amounts.

---

## 2. Business Objective

Enable enterprise finance and operations to trace freight money across the lifecycle:

| Question | v2.1 target answer source |
|----------|---------------------------|
| Expected cost at order time | Immutable TO pricing snapshot |
| Liability before invoice | Accrual projection (planned + approved execution charges) |
| Final commercial liability | Settlement (billing-register-service) |
| Invoiced amount | Billing register |
| Paid amount | Payment obligation / allocations |
| Why different | Deterministic variance + reason attribution |

Target flow:

```text
RFx / Contract Rate
        ↓
Transport Order pricing snapshot  ← PLANNED (frozen)
        ↓
Shipment execution
        ↓
Operational cost events / approved accessorials
        ↓
Accrual / estimated liability     ← NEW (derived + ledger)
        ↓
Settlement                        ← ACTUAL / SETTLED (canonical)
        ↓
Billing Register / Closing Documents
        ↓
Payment
        ↓
Planned vs Actual / variance analytics  ← NEW (read model)
```

---

## 3. Current-State Discovery

### 3.1 Precondition — v2.0 in base

| Gate | Value |
|------|-------|
| `V2_0_CONTAINED_IN_BASE` | **YES** — base `ea7721c` merges PR #40 (v2.0E) |
| `V2_0_PRICING_SNAPSHOT_AVAILABLE` | **YES** — migration `000051`, transport-order-service |
| `V2_0_SETTLEMENT_INTEGRATION_AVAILABLE` | **YES** — migration `000052`, billing-register-service |
| `V2_0_PUBLIC_RATE_API_AVAILABLE` | **YES** — api-gateway `/api/v1/*` contract-rate routes (v2.0E) |

Merged v2.0 sequence on base:

| Phase | PR | Merge SHA |
|-------|-----|-----------|
| v2.0A | #36 | `292ebad0d505c70b463b0c13925c575a049f4f59` |
| v2.0B | #37 | `b0f41c1087eb48afefca1126e22a168481e18b0e` |
| v2.0C | #38 | `225c79fe61e2599b4e8cde6a89a1fc04864484e6` |
| v2.0D | #39 | `b6c2c7fe08c43d7a6bc9a2e76466c6cf46575358` |
| v2.0E | #40 | `ea7721c188b4cf2e10f40f1a8a4dd5e57104a2be` |

### 3.2 Service map (discovered)

| Service | Role in cost chain | Schema |
|---------|-------------------|--------|
| `contract-rate-service` | Master rates, resolution | `contract_rate.*` |
| `rfx-service` | Bids, awards, RFQ pricing context | `rfx.*` |
| `transport-order-service` | TO + **immutable pricing snapshot** | `transport.*` |
| `shipment-service` | Execution status only — **no price columns** | `transport.*` |
| `billing-register-service` | **Settlement**, billing registers, closing | `billing.*` |
| `payment-service` | Obligations, payments, allocations | `billing.*` (shared schema) |
| `document-service` | POD, acts, UPD — **not freight price SSOT** | `documents.*` |
| Dedicated `settlement-service` | **NOT_FOUND** | — |
| `freight-cost-service` | **NOT_FOUND** | — |

### 3.3 Discovery status summary

| Area | Status | Notes |
|------|--------|-------|
| Contract / rate model | **FOUND** | v2.0A–B, `contract_rate.*` |
| TO pricing snapshot | **FOUND** | v2.0C, immutable |
| Shipment cost model | **NOT_FOUND** | No monetary columns on shipments |
| Settlement model | **FOUND** | Embedded in billing-register-service |
| Billing cost model | **FOUND** | Register aggregates from settlement items |
| Payment cost model | **FOUND** | Obligation from register total only |
| Accessorial model | **PARTIAL** | Settlement accessorials only; no shipment-native waiting/detention |
| Accrual model | **NOT_FOUND** | No tables, APIs, or docs |
| Cost ledger | **NOT_FOUND** | — |
| FX / conversion | **NOT_FOUND** | Single currency per fact; no FX provider |

---

## 4. Current Money Map

Evidence-based catalog. **Canonical?** = authoritative SSOT for that business concept in current architecture.

| Business concept | Owning service | Schema/table | Column/field | Money type | Currency | Scope | Source | Mutable? | Canonical? | Consumers | Notes |
|------------------|----------------|--------------|--------------|------------|----------|-------|--------|----------|------------|-----------|-------|
| RFx bid quote | rfx-service | `rfx.bids` | `total_amount` | NUMERIC(18,2); domain float64 | bid currency | tenant/bid | carrier bid | YES (bid lifecycle) | NO (superseded by snapshot) | RFx UI, evaluation | Legacy float64 domain |
| RFx bid line components | rfx-service | `rfx.bid_items` | `base_amount`, `fuel_surcharge`, etc. | NUMERIC(18,2) | bid | bid item | carrier | YES | NO | RFx | — |
| RFx award → TO link amount | rfx-service | `rfx.rfx_award_transport_orders` | `amount` | NUMERIC(18,2) | award | tenant/TO | award | YES | NO for SNAPSHOT_V1 TOs | Legacy settlement loader | Legacy path only |
| Contract rate component | contract-rate-service | `contract_rate.rate_component` | `amount`, `percent_value` | NUMERIC(18,2)/(9,6) | contract | tenant/rate line | contract master | YES (draft) | NO (master data) | resolver | ACTIVE version immutable |
| Resolver output total | contract-rate-service | — (transient) | `TotalAmount` string | decimal string | request | resolve call | CONTRACT_RATE etc. | N/A | NO | TO snapshot builder | Not persisted except via snapshot |
| **TO planned/agreed price** | transport-order-service | `transport.transport_order_rate_snapshots` | `total_amount` | NUMERIC(18,2); domain decimal | snapshot | tenant/TO | resolve at create | **NO** (DB trigger) | **YES** | settlement loader, future cost | Authoritative planned cost |
| Snapshot base breakdown | transport-order-service | same | `base_amount` | NUMERIC(18,2) nullable | snapshot | tenant/TO | resolver | **NO** | PARTIAL | analytics breakdown | NULL when UNAVAILABLE |
| Snapshot components JSON | transport-order-service | same | `components` | JSONB | snapshot | tenant/TO | resolver | **NO** | PARTIAL | UI/analytics | Includes BASE_FREIGHT, FUEL_SURCHARGE, WAITING, DETENTION rules |
| Snapshot accessorial rules | transport-order-service | same | `accessorial_rules` | JSONB | snapshot | tenant/TO | resolver | **NO** | PARTIAL | pre-execution unit rates | Not accrued amounts |
| Settlement principal | billing-register-service | `billing.freight_settlements` | `base_freight_amount` | NUMERIC(18,2) | settlement | tenant/shipment | **snapshot.total_amount** copy at create | **NO** after INSERT | **YES** (settled principal) | register include | Never from snapshot.base_amount alone |
| Settlement approved accessorials | billing-register-service | `billing.settlement_accessorials` | `amount` (APPROVED sum → `approved_accessorial_total`) | NUMERIC(18,2) | settlement | tenant/settlement | operator/carrier propose + approve | YES (workflow) | **YES** (approved charges) | settlement totals | Free-form `charge_code` |
| Settlement freight ex-VAT | billing-register-service | `billing.freight_settlements` | `total_without_vat` | NUMERIC(18,2) | settlement | tenant/shipment | base + approved accessorials | YES (recalc) | **YES** | billing include | — |
| Settlement VAT | billing-register-service | same | `vat_amount`, `total_with_vat` | NUMERIC(18,2) | settlement | tenant/shipment | calculated | YES | **YES** (payable) | payment obligation | Tax boundary — see §30 |
| Billing register item | billing-register-service | `billing.billing_register_items` | `base_amount`, `extra_charges`, `penalties` | NUMERIC(18,2) | register | tenant/item | **copied at include** from settlement | Fixed at include | **YES** (invoiced line) | register totals | No auto-sync if settlement changes after include |
| Billing register total | billing-register-service | `billing.billing_registers` | `total_with_vat` | NUMERIC(18,2) | register | tenant/register | SUM(items) | YES (recalc) | **YES** (invoiced aggregate) | payment obligation | Does not re-read snapshot |
| Payment obligation | payment-service | `billing.payment_obligations` | `original_amount` | NUMERIC(18,2); decimal domain | obligation | tenant/register | register `total_with_vat` | NO after create | **YES** (payable) | allocations | Source type BILLING_REGISTER only |
| Payment received | payment-service | `billing.payments` | `amount` | NUMERIC(18,2); decimal | payment | tenant | bank/cash entry | NO (void only) | **YES** (cash) | reconciliation | — |
| Obligation paid | payment-service | `billing.payment_obligations` | `paid_amount` | NUMERIC(18,2) | obligation | tenant | SUM(allocations) | YES (derived) | **YES** (paid status) | dashboards | — |
| Manual spot audit | contract-rate-service | `contract_rate.manual_spot_audit` | amount fields | NUMERIC | audit | tenant | authorized manual | append-only | PARTIAL | snapshot provenance | — |
| Shipment execution cost | shipment-service | — | — | — | — | — | — | — | **NOT_FOUND** | — | — |
| Accrual | — | — | — | — | — | — | — | — | **NOT_FOUND** | — | v2.1 to introduce |
| Claims / cargo penalties | — | — | — | — | — | — | — | — | **NOT_FOUND** as freight cost | settlement `penalties` on register item defaults 0 | Out of MVP scope |

---

## 5. Existing Pricing Snapshot (v2.0C)

| Field | Value |
|-------|-------|
| `SNAPSHOT_OWNER` | **transport-order-service** |
| `SNAPSHOT_TABLE` | `transport.transport_order_rate_snapshots` |
| `SNAPSHOT_CREATED_WHEN` | Priced TO create (`POST` with Idempotency-Key) or award-scope conversion — single DB TX with TO |
| `SNAPSHOT_IMMUTABLE` | **YES** — triggers deny UPDATE/DELETE (`000051`) |
| `SNAPSHOT_SOURCE_TYPES` | `RFQ_AWARD`, `SPOT_BID`, `CONTRACT_RATE`, `MANUAL_SPOT` |
| `SNAPSHOT_TOTAL_AMOUNT` | `total_amount NUMERIC(18,2) NOT NULL` — **authoritative agreed commercial total** |
| `SNAPSHOT_BASE_AMOUNT` | `base_amount NUMERIC(18,2) NULL` — breakdown only; NULL when `component_breakdown_status=UNAVAILABLE` |
| `SNAPSHOT_COMPONENTS` | `components JSONB` — resolver output when AVAILABLE |
| `SNAPSHOT_CURRENCY` | `currency_code CHAR(3) NOT NULL` |
| `SNAPSHOT_RATE_VERSION` | `rate_version_id`, `rate_card_id`, `contract_id`, `rate_line_id` (CONTRACT_RATE) |
| `SNAPSHOT_RFX_PROVENANCE` | `award_link_id`, `rfx_event_id`, `rfx_lot_id`, `bid_id`, `manual_spot_audit_id` |

Domain: `services/transport-order-service/internal/domain/rate_snapshot.go` — `TotalAmount decimal.Decimal`, `BaseAmount *decimal.Decimal`.

### Can snapshot serve as PLANNED_FREIGHT_COST?

**YES, with precise semantics** (see §12).

Settlement intentionally uses **`total_amount`**, not `base_amount`, as principal (`LoadShipmentContext` → `AgreedFreightAmount`; v2.0C doc: avoids fuel double-count when components exist).

For aggregate-only RFQ awards: `base_amount=NULL`, `component_breakdown_status=UNAVAILABLE`, **`total_amount` still authoritative**.

---

## 6. Current Settlement Model

| Field | Value |
|-------|-------|
| `SETTLEMENT_OWNER` | **billing-register-service** (no separate settlement-service) |
| `SETTLEMENT_TRIGGER` | Manual create per shipment (`POST /api/v1/freight-settlements`) after execution milestones |
| `SETTLEMENT_INPUTS` | Shipment → TO → snapshot (SNAPSHOT_V1) or legacy award link; POD check optional |
| `SETTLEMENT_OUTPUT` | `freight_settlements` row + accessorials + audit events |
| `SETTLEMENT_PRINCIPAL_SOURCE` | `transport_order_rate_snapshots.total_amount` (SNAPSHOT_V1) or `rfx_award_transport_orders.amount` (legacy) |
| `SETTLEMENT_ACCESSORIAL_SOURCE` | `billing.settlement_accessorials` — PROPOSED → APPROVED workflow |
| `SETTLEMENT_ADJUSTMENT_MODEL` | Accessorial approve/reject + disputes; **no separate adjustments table** |
| `SETTLEMENT_FINALITY` | Status machine: DRAFT → … → READY_FOR_PAYMENT; inclusion in billing register |
| `SETTLEMENT_MUTABILITY` | `base_freight_amount` immutable after create; totals recalc on accessorial changes |
| `SETTLEMENT_CURRENCY` | Single `currency_code` per settlement from snapshot/award |
| `SETTLEMENT_AUDIT` | `billing.settlement_audit_events` append-only |

| Value type | Behavior |
|------------|----------|
| `base_freight_amount` | **Copied** from snapshot total at create — not recalculated from latest rate |
| `approved_accessorial_total` | **Derived** from APPROVED accessorials |
| `total_without_vat` | **Calculated** base + accessorials |
| VAT fields | **Calculated** from VAT rate |
| Accessorial amounts | **Manually entered** (proposed) then **approved** |

| Invariant | Verified |
|-----------|----------|
| `SETTLEMENT_RECALCULATES_FROM_LATEST_RATE` | **NO** — integration tests + no code path re-resolves contract rate |

### Settlement as ACTUAL_FREIGHT_COST?

**PARTIAL — best canonical candidate for final commercial freight liability, but not equivalent to "executed cost" or "paid cost".**

| Term | Meaning in current system |
|------|---------------------------|
| Executed cost | **NOT_FOUND** as persisted fact — only accessorial proposals approximate |
| Approved cost | Settlement `base_freight_amount` + `approved_accessorial_total` (ex-VAT) |
| Settled cost | Settlement totals when status ≥ APPROVED / included in register |
| Invoiced cost | Billing register item/register totals (copy at include time) |
| Paid cost | Payment obligation `paid_amount` — **payment status ≠ freight actual** |

**v2.1 recommendation:** Canonical **settled freight cost (ex-VAT)** = `freight_settlements.total_without_vat` when settlement reaches approved/final states; canonical **invoiced** = register; canonical **paid** = payment obligation.

---

## 7. Billing / Closing Documents

| Field | Value |
|-------|-------|
| `BILLING_SOURCE_OF_AMOUNT` | Settlement snapshot at **include time** — `base_amount` ← settlement.base_freight; `extra_charges` ← approved_accessorial_total |
| `BILLING_FINALITY` | Register status machine (calculate → approve → sent → signed → paid → closed) |
| `BILLING_ADJUSTMENTS` | Manual register items still supported (legacy); settlement path canonical for v2.0 chain |
| `BILLING_CURRENCY` | Register `currency_code`; must match settlement on include |
| `BILLING_LINK_TO_SETTLEMENT` | `billing_register_items.settlement_id`; reverse link on settlement |
| `BILLING_LINK_TO_SHIPMENT` | `shipment_id`, `transport_order_id` on item |
| `BILLING_LINK_TO_ORDER` | `transport_order_id` on item |
| `CAN_BILLING_RECALCULATE_FREIGHT_PRICE?` | **NO** — does not re-read TO snapshot or contract rates; copies settlement; register totals = SUM(items) |

**Gap:** If settlement accessorials change **after** register inclusion, register item amounts are **not** auto-updated (**NOT_FOUND** back-sync).

---

## 8. Payment Discovery

| Field | Value |
|-------|-------|
| `PAYMENT_OBLIGATION_SOURCE` | `billing_registers.id` — source_type `BILLING_REGISTER` only |
| `PAYMENT_AMOUNT_SOURCE` | `billing_registers.total_with_vat` → obligation `original_amount` |
| `PAYMENT_ALLOCATION_MODEL` | Append-only allocations; void via `voided_at`; updates obligation paid/outstanding |
| `PAYMENT_VOID_MODEL` | Payment void blocked if reconciled; allocation void blocked if obligation PAID |
| `PAYMENT_RECONCILIATION_MODEL` | Sum(allocations) = payment.amount; sets reconciled_at |
| `PAYMENT_FINALITY` | Obligation status derived from paid vs original; register sync on PAID via outbox |
| `PAYMENT_RECALCULATES_FREIGHT_COST` | **NO** — confirmed; no snapshot/settlement/rate reads in payment-service |

---

## 9. Accessorial / Execution Cost Discovery

| Concept | Status | Domain owner | Notes |
|---------|--------|--------------|-------|
| Contract WAITING/DETENTION unit rates | **FOUND** | contract-rate-service | In rate components + snapshot `accessorial_rules` — **commercial rules**, not incurred amounts |
| Settlement accessorial | **FOUND** | billing-register-service | `charge_code` + `amount`; statuses PROPOSED/APPROVED/REJECTED/DISPUTED |
| Shipment waiting/detention events | **NOT_FOUND** | — | No execution-time cost events |
| Operational toll/ferry/redelivery | **NOT_FOUND** | — | Could be proposed as settlement accessorial only |
| Penalties on settlement | **PARTIAL** | billing-register | `penalties` column on register items; default 0 on settlement include |
| Claims / damage | **NOT_FOUND** | — | Out of v2.1 MVP |

### Stage classification (current)

| Stage | Representation today |
|-------|---------------------|
| A. Commercial component before execution | Snapshot `components` / `accessorial_rules` |
| B. Operational event | **NOT_FOUND** |
| C. Requested charge | Settlement accessorial PROPOSED |
| D. Approved charge | Settlement accessorial APPROVED |
| E. Settled charge | Included in settlement totals |
| F. Invoiced charge | Billing register item (copy) |

---

## 10. Double-Count Analysis

| Threat | Current mitigation | v2.1 invariant |
|--------|-------------------|----------------|
| Contract fuel + settlement fuel re-add | Settlement uses snapshot **total_amount** as base, not base_amount + fuel again | **FMC-INV-008 FUEL_DOUBLE_COUNT=DENY** |
| Snapshot total + separate base freight | Cost projection must not add `base_amount` when `total_amount` already includes fuel | **FMC-INV-009** |
| Contract detention in snapshot rules + accessorial DETENTION | Snapshot rules are unit **rates**, not incurred totals; accessorial is execution charge — **different semantics**; ledger must tag source | Manual reason attribution required |
| RFx aggregate + component reconstruction | `base_amount=NULL`, no infer-from-total (v2.0C) | Use total_amount only |
| Settlement total + billing re-add | Billing copies settlement; register SUM only | **FMC-INV-010 SETTLEMENT_BILLING** — copy once at include |
| Billing + payment double count | Obligation from register total; allocations ≤ obligation | **FMC-INV-005** |
| Correction + original | Allocation void append-only; settlement accessorial status changes recalc | Reversal entries in v2.1 ledger |
| Event replay | Payment outbox idempotent; v2.1 ledger needs source_event_id UNIQUE | **FMC-INV-010 EVENT_REPLAY** |

Each invariant names canonical source:

- **Planned principal:** `transport_order_rate_snapshots.total_amount`
- **Settled principal:** `freight_settlements.base_freight_amount` (equal to snapshot total at create)
- **Approved execution extras:** SUM(APPROVED accessorials)
- **Invoiced:** billing register at include
- **Paid:** payment obligation allocations

---

## 11. Financial Stages Matrix

| Stage | Business meaning | Current source | Persist new in v2.1? | Mutable? | Final? |
|-------|------------------|----------------|----------------------|----------|--------|
| ESTIMATED | Pre-commitment quote (bid/RFQ) | rfx.bids | NO — not TO planned | YES | NO |
| PLANNED | Commercial freight frozen at TO create | TO rate snapshot | NO (exists) | NO | YES at snapshot |
| COMMITTED | Same as PLANNED for v2.1 MVP | snapshot | NO | NO | YES |
| ACCRUED | Expected liability before settlement final | **NOT_FOUND** | YES — projection/ledger | Derived | NO |
| APPROVED | Accessorials approved on settlement | settlement_accessorials APPROVED | NO (exists) | YES until settled | PARTIAL |
| SETTLED | Commercial freight liability agreed | freight_settlements | NO (exists) | PARTIAL (accessorial recalc) | YES at APPROVED+ |
| INVOICED | Amount on closing register | billing_registers | NO (exists) | Until register closed | YES when signed |
| PAID | Cash applied to obligation | payment_obligations | NO (exists) | Allocations append-only | YES when PAID |

**MVP equivalence:** ESTIMATED ≠ PLANNED; COMMITTED ≡ PLANNED for v2.1.

---

## 12. Planned Cost Definition (FROZEN for v2.1)

**Selected definition:** **A — commercial freight amount frozen on the Transport Order pricing snapshot at order creation / pricing resolution.**

| Question | Answer |
|----------|--------|
| Includes all snapshot rate components? | **YES semantically** — encoded in authoritative `total_amount`; breakdown in `components` when AVAILABLE |
| Includes fuel? | **YES when included in resolver total** (CONTRACT_RATE path); not separately added in analytics |
| Includes expected accessorials? | **NO for incurred amounts** — snapshot `accessorial_rules` are unit **rates**, not accrued waiting/detention totals |
| Includes waiting/detention before occurrence? | **NO** — only commercial rules, not execution charges |
| RFx aggregate-only? | **`total_amount` authoritative**; `base_amount=NULL`; do not reconstruct components |
| base unknown but total authoritative? | **Use total_amount only** (v2.0C invariant) |
| Can planned cost change after TO create? | **NO** — snapshot immutable |
| Material scope change? | Requires new TO / new snapshot (existing TO architecture) — **no in-place repricing** |
| After cancellation? | Planned remains historical fact; analytics filter by TO/shipment status |
| Multi-currency? | One currency per snapshot; no cross-currency planned cost |

```text
PLANNED_COST = transport_order_rate_snapshots.total_amount
PLANNED_CURRENCY = transport_order_rate_snapshots.currency_code
PLANNED_COST_FROM_IMMUTABLE_PRICING_SNAPSHOT = YES  (verified)
```

---

## 13. Actual Cost Definition (FROZEN for v2.1)

**Canonical settled freight cost (ex-VAT):**

```text
SETTLED_FREIGHT_COST = freight_settlements.total_without_vat
  (when settlement status in final-approved set: APPROVED, DOCUMENTS_READY, READY_FOR_PAYMENT, and not CANCELLED)
```

Before settlement exists: **`ACTUAL_COST = NULL`** (unknown — not zero).

| Concept | Canonical owner | Field |
|---------|-----------------|-------|
| ACTUAL / SETTLED freight (ex-VAT) | billing-register-service | `freight_settlements.total_without_vat` |
| INVOICED amount (with VAT) | billing-register-service | `billing_registers.total_with_vat` or item `amount_with_vat` |
| PAID amount | payment-service | `payment_obligations.paid_amount` |

**FMC-INV-014:** Payment status does **not** change settled freight cost.

---

## 14. Accrual Model (v2.1 design)

**Current:** `ACCRUAL_MODEL=NOT_FOUND`

**Business question:** Shipment executed, settlement not final — what liability should finance expect?

**Recommended v2.1 formula (derived, deterministic):**

```text
ACCRUED_FREIGHT_COST =
  PLANNED_COST (snapshot.total_amount)
  + SUM(settlement accessorials WHERE status = APPROVED)
  (same currency; NULL if snapshot missing)
```

If no settlement yet but shipment exists: `ACCRUED = PLANNED` (no approved execution charges).

**Persistence recommendation:** **Option D — immutable ledger entries + rebuildable projection**

- Append-only `freight_cost.cost_entry` rows referencing source IDs
- Projection table `freight_cost.transport_order_cost_summary` rebuilt from entries + canonical reads
- **NOT** a second settlement; entries are **references + derived snapshots**, not mutable money authority

---

## 15. Cost Ledger Architecture Options

| Option | Assessment |
|--------|------------|
| **A. Dedicated freight-cost-service** | **Recommended** — clear analytics boundary; no settlement coupling; event-driven projection |
| B. Extend settlement service | Risk: conflate operational settlement workflow with analytics; coupling to billing-register |
| C. Distributed + read model only | Possible but lacks accrual ledger idempotency home |

**Principle:** `NEW_COST_LAYER_MUST_NOT_REPLACE_EXISTING_CANONICAL_FINANCIAL_FACTS=YES`

**Decision:** **HYBRID** — dedicated `freight-cost-service` for ledger + projections; settlement/billing/payment remain canonical writers for their facts.

---

## 16. Ledger Semantics (recommended)

| Field | Value |
|-------|-------|
| `LEDGER_ENTRY_IMMUTABLE` | **YES** — append-only |
| `LEDGER_DELETE_ALLOWED` | **NO** — soft suppression via reversal only |
| `CORRECTION_MODEL` | Reversal entry + superseding entry referencing original `entry_id` |
| `IDEMPOTENCY_MODEL` | UNIQUE `(tenant_id, source_service, source_type, source_id, entry_kind)` |
| `SOURCE_EVENT_UNIQUENESS` | One ledger entry per canonical source event |

Evaluated entry kinds (minimal set):

| Entry kind | Source |
|------------|--------|
| `PLANNED_SNAPSHOT` | TO rate snapshot created |
| `APPROVED_ACCESSORIAL` | settlement accessorial APPROVED |
| `SETTLEMENT_FINALIZED` | settlement reached approved state |
| `BILLING_INCLUDED` | register include |
| `PAYMENT_ALLOCATED` | allocation created |
| `REVERSAL` | void/correction events |

Do **not** copy full settlement row — store amount + currency + source pointers only.

---

## 17. Money Model

| Layer | Current canonical | Legacy exceptions |
|-------|-------------------|-------------------|
| PostgreSQL | `NUMERIC(18,2)` | — |
| Go (TO, contract-rate, payment) | `shopspring/decimal` | — |
| Go (settlement, billing, rfx domain) | **`float64` + round2()** | Snapshot **principal load** uses decimal → StringFixed(2) for INSERT |
| API JSON | decimal **strings** (v2.0 public); mixed elsewhere | RFx public may expose numbers |

**v2.1 rules (freeze):**

```text
NO_FLOAT64_CANONICAL_MONEY in freight-cost-service (new code)
DATABASE_MONEY_TYPE = NUMERIC(18,2)
API_MONEY_TYPE = decimal string
DETERMINISTIC_ROUNDING = half-up scale 2
SINGLE_CURRENCY_AGGREGATION_ONLY = YES
```

Migrate float64 in cost projection inputs via decimal parse at boundary — do not aggregate float64.

---

## 18. Multi-Currency Policy

| Question | Answer |
|----------|--------|
| TO snapshot single currency? | **YES** — `currency_code CHAR(3)` |
| Accessorials another currency? | **NO** — same settlement `currency_code` enforced |
| Settlement aggregate currencies? | **NO** |
| FX conversion exists? | **NOT_FOUND** |
| FX rate provider? | **NOT_FOUND** |
| Historical FX persisted? | **NOT_FOUND** |
| Payment multi-currency obligations? | One currency per obligation (from register) |

```text
CROSS_CURRENCY_VARIANCE_AGGREGATION = NOT_ALLOWED
FX_CONVERSION_V2_1 = NOT_IN_SCOPE
```

Dashboards: group/filter by currency; never sum mixed currencies.

---

## 19. Variance Model

```text
variance_amount = settled_freight_cost_ex_vat - planned_cost
variance_percent = variance_amount / planned_cost * 100   (when planned_cost > 0)
```

| Edge case | Behavior |
|-----------|----------|
| planned_cost = 0 | variance_percent = NULL; flag ZERO_PLANNED |
| planned_cost NULL | variance = NULL |
| actual NULL (no settlement) | variance = NULL |
| currency mismatch | FAIL CLOSED — no variance row |
| cancelled order | exclude or show NULL with status CANCELLED |
| partial settlement | use current settlement totals; mark PARTIAL |
| corrected settlement | use latest settlement totals; preserve history via ledger reversals |

**Sign convention:** positive = over plan; negative = saving.

---

## 20. Variance Reasons

| Reason | Evidence source | Auto/manual | In v2.1 MVP |
|--------|-------------------|-------------|-------------|
| FUEL | snapshot components vs settlement | AUTO when breakdown AVAILABLE | YES |
| WAITING | APPROVED accessorial charge_code match | AUTO partial (code convention) | PARTIAL |
| DETENTION | same | PARTIAL | PARTIAL |
| ACCESSORIAL | settlement accessorial | AUTO | YES |
| MANUAL_ADJUSTMENT | settlement accessorial without rule match | MANUAL tag | YES |
| RATE_CHANGE | N/A if snapshot immutable | — | NO (prevented by architecture) |
| ROUTE_CHANGE | **NOT_FOUND** execution fact | — | NO |
| CANCELLATION | TO/shipment status | AUTO | YES |
| OTHER | manual reason code | MANUAL | YES |

Manual reasons **never alter canonical money** — attribution only.

---

## 21. Planned vs Actual Read Model (conceptual)

| Field | Available now | v2.1 source |
|-------|---------------|-------------|
| tenant_id | YES | all tables |
| transport_order_id | YES | snapshot, settlement |
| shipment_id | YES | settlement, shipment |
| carrier_company_id | YES | snapshot, settlement |
| buyer_company_id | YES | snapshot, settlement |
| currency | YES | snapshot |
| planned_cost | YES | snapshot.total_amount |
| accrued_cost | NO | derived v2.1 |
| settled_cost | PARTIAL | settlement.total_without_vat |
| invoiced_cost | PARTIAL | register |
| paid_cost | PARTIAL | obligation |
| variance_amount | NO | derived v2.1 |
| pricing_source | YES | snapshot.pricing_source |
| contract_id / rate_version_id | YES | snapshot |
| rfx provenance | YES | snapshot award/bid fields |
| settlement_status | YES | freight_settlements.status |
| billing_status | YES | billing_registers.status |
| payment_status | YES | payment_obligations.status |
| forwarder_company_id | **PARTIAL** — actor context only | membership |
| lane (origin/dest) | YES | snapshot location IDs |
| cost_updated_at | NO | projection metadata v2.1 |

---

## 22. Enterprise Analytics Dimensions

| Dimension | Status | Notes |
|-----------|--------|-------|
| tenant | AVAILABLE | |
| shipper (buyer) | AVAILABLE | buyer_company_id |
| carrier | AVAILABLE | |
| forwarder | PARTIAL | role-based actor, not always persisted on cost facts |
| customer/consignee | PARTIAL | on billing register items |
| transport order | AVAILABLE | |
| shipment | AVAILABLE | |
| lane (origin/dest UUID) | AVAILABLE | snapshot |
| equipment | AVAILABLE | snapshot |
| contract / rate card / version | AVAILABLE | CONTRACT_RATE snapshots |
| RFx event | AVAILABLE | RFQ_AWARD/SPOT_BID provenance |
| date (planned) | AVAILABLE | snapshot.resolved_at, pricing_date |
| date (settled) | PARTIAL | settlement timestamps |
| service level | NOT_AVAILABLE | |
| warehouse/facility | NOT_AVAILABLE as cost dimension | |

**v2.1 MVP analytics possible:** cost per shipment/TO, planned vs actual, variance by carrier/lane/equipment/contract, accessorial spend, unbilled accrual (derived), settled-unpaid exposure.

**Later work:** customer margin, route optimization cost, CO2, revenue pairing.

---

## 23. Tenant / Company Security Model

Existing gateway RBAC pattern (v2.0E model):

- JWT tenant + membership-verified `X-Company-ID`
- Buyer mutate vs carrier read-only splits

**v2.1 cost analytics views:**

| View | Audience | Content |
|------|----------|---------|
| BUYER_COST_VIEW | Shipper/procurement/finance | Full planned/actual/variance |
| CARRIER_RECEIVABLE_VIEW | Carrier | Settlement/register amounts owed **to carrier** — not buyer internal analytics |
| PLATFORM_ADMIN_VIEW | Platform admin | Tenant-scoped all |

```text
CROSS_COMPANY_INTERNAL_COST_DISCLOSURE = DENY
CROSS_TENANT_COST_ACCESS = DENY
```

Projections must filter by verified company membership same as settlement/billing guards.

---

## 24. RBAC Discovery

| Area | Enforcement | Location |
|------|-------------|----------|
| Settlements | Gateway roles | `settlementrbac/guard.go` |
| Billing | Gateway roles | `billingrbac/guard.go` |
| Payments | Gateway roles + FINANCE_MANAGER | `paymentrbac/guard.go` |
| Contract rates | Gateway + `rates.manual_spot.use` | v2.0E |
| DB permissions settlement/payment | **NOT_FOUND** | — |
| Cost analytics | **NOT_FOUND** | Propose v2.1E: `freight_cost.read`, `freight_cost.export` |

Avoid role hardcoding in cost service — follow permission codes pattern from identity seed.

---

## 25. Event / Integration Model

| Event | Status | Owner | Mechanism |
|-------|--------|-------|-----------|
| TO created + snapshot | **PARTIAL** | transport-order-service | No Kafka outbox found; synchronous create |
| Shipment status changed | **FOUND** | shipment-service | `transport.shipment_event_outbox` |
| Accessorial approved | **NOT_FOUND** outbox | billing-register-service | Audit event only |
| Settlement created/finalized | **NOT_FOUND** outbox | billing-register-service | Audit event only |
| Billing register events | **NOT_FOUND** outbox | billing-register-service | Audit event only |
| Payment reconciled / paid | **FOUND** | payment-service | `billing.payment_outbox` → `payment_obligation.paid` |

**v2.1 recommendation:** Add outbox events (or poll audit with idempotency) for:

- `transport_order.rate_snapshot.created`
- `freight_settlement.accessorial.approved`
- `freight_settlement.finalized`
- `billing_register.settlement.included`

**Principle:** `CROSS_SERVICE_FINANCIAL_READ = internal API or canonical event/projection` — freight-cost-service must not use ad-hoc cross-schema SQL in production (settlement loader cross-schema read is existing exception for create-time copy only).

---

## 26. Consistency Model

| Derived amount | Consistency |
|----------------|-------------|
| TO planned snapshot | **Strong / immutable** (same TX as TO create) |
| Settlement principal copy | **Strong** at settlement create (same service TX) |
| Billing include copy | **Strong** at include TX |
| Payment obligation | **Strong** at ensure (reads register) |
| Accrual projection | **Eventual** — acceptable lag seconds/minutes |
| Dashboard aggregates | **Eventual / rebuildable** |
| Variance | **Eventual** — must reconcile to settlement |

---

## 27. Rebuild / Reconciliation

If projection used:

1. Truncate projection tables (tenant-scoped)
2. Replay ledger entries in `created_at` order
3. Re-fetch canonical amounts from source services for verification checksum
4. Compare `SUM(entries)` vs settlement/register/obligations — emit `cost_reconciliation_mismatch_total`

```text
EVENT_REPLAY_DOUBLE_COUNT = DENY (idempotency keys)
DUPLICATE_EVENT_DOUBLE_COUNT = DENY
```

Out-of-order: use source fact timestamp + monotonic version per aggregate.

---

## 28. Correction / Reversal Model

| Domain | Existing behavior |
|--------|-------------------|
| Settlement accessorial | Status change recalculates totals — **mutable workflow** |
| Settlement base | **Immutable** after create |
| Billing register item | Fixed at include — correction = new register cycle (manual ops) |
| Payment allocation | Void append-only |
| TO snapshot | **Immutable** — no correction path |

v2.1 ledger: append REVERSAL when settlement totals change materially post-approval (if allowed) or on void — **do not override existing settlement semantics**.

---

## 29. Time Semantics

| Timestamp | Exists | Use for analytics |
|-----------|--------|-------------------|
| Order / snapshot created | YES — `snapshot.resolved_at`, `created_at` | Planned cost date |
| Shipment execution | YES — shipment status timestamps | Execution timeline — not cost incurred |
| Accessorial approved | PARTIAL — `updated_at` on accessorial | Accrual timing |
| Settled | PARTIAL — settlement status transitions | Actual cost date |
| Invoiced | PARTIAL — register status | Invoiced date |
| Paid | YES — allocation/reconcile timestamps | Paid date |
| Cost incurred at | **NOT_FOUND** | Do not fake from created_at |

---

## 30. Tax Boundary

Settlement and billing carry VAT (`vat_rate`, `vat_amount`, `total_with_vat`).

**Freight cost analytics (v2.1):** use **ex-VAT** amounts (`total_without_vat`, `amount_without_vat`) unless explicitly reporting tax-inclusive payable views.

```text
FREIGHT_COST_ANALYTICS_EXCLUDES_TAX_UNLESS_CANONICAL_TAX_FACT_EXISTS = YES
```

Payment obligations use **with-VAT** register totals — label separately as payable, not planned/actual freight.

---

## 31. Claims / Penalties Boundary

| Concept | Status | v2.1 MVP |
|---------|--------|----------|
| Operational freight expense | Settlement + accessorials | **IN SCOPE** |
| Service penalty | register item `penalties` (defaults 0) | **OUT OF SCOPE** unless provenance added |
| Cargo claim / damage | **NOT_FOUND** | **OUT OF SCOPE** |
| Payment adjustment | Allocation void | **Separate from freight cost** |

---

## 32. Accounting / ERP Boundary

Freight Cost Management v2.1 **does not own:**

- General ledger, chart of accounts, double-entry
- Tax accounting / VAT engine redesign
- Bank ledger, ERP posting, FX revaluation, revenue recognition

Provides **logistics freight cost facts** suitable for future ERP export.

---

## 33. Target Domain Flows

### FLOW A — Contract rate transport
Contract resolve → TO snapshot (PLANNED) → shipment → accessorial approve → accrual ↑ → settlement (ACTUAL) → billing → payment → variance if accessorials added.

### FLOW B — RFx award transport
RFQ_AWARD snapshot (possibly aggregate-only) → same as A.

### FLOW C — Manual spot
Requires `manual_spot_audit_id` in snapshot — same chain; no public forgery (v2.0E).

### FLOW D — No accessorials
Planned ≈ settled (ex-VAT); variance ≈ 0.

### FLOW E — Detention added
Planned = snapshot total; settled = planned + approved DETENTION accessorial; variance > 0.

### FLOW F — Settlement correction
Accessorial reject after approve → settlement recalc → ledger REVERSAL + new entry; planned unchanged.

### FLOW G — Partial/unpaid
Settled/invoiced > paid until allocations complete — separate KPI "settled unpaid exposure".

---

## 34. Target State Machine

**Do not create monolithic `cost_status`.** Independent dimensions:

- Shipment execution status (shipment-service)
- Settlement status (billing-register-service)
- Billing register status
- Payment obligation status
- Cost confidence (analytical): PLANNED → ACCRUING → SETTLED → INVOICED → PAID (projection-only)

---

## 35. API Boundary (design only)

Future reads (conceptual — align with gateway in v2.1E):

```text
GET /api/v1/freight-costs?company_id=&from=&to=
GET /api/v1/freight-costs/transport-orders/{id}
GET /api/v1/freight-costs/shipments/{id}
GET /api/v1/freight-costs/summary
GET /api/v1/freight-costs/variance
```

**No generic PATCH for money.** Corrections remain in settlement/billing domains.

Internal S2S for projection workers:

```text
GET /internal/v1/freight-costs/rebuild?tenant_id=
POST /internal/v1/freight-costs/projections/reconcile
```

---

## 36. Future Cost Analytics Workspace (conceptual)

| Screen | Backend facts needed | Available | Phase |
|--------|---------------------|-----------|-------|
| Freight Cost Overview | planned/settled/paid aggregates | PARTIAL | v2.1D |
| Planned vs Actual | snapshot + settlement | PARTIAL | v2.1C/D |
| Shipment Cost Detail | snapshot, settlement, accessorials | PARTIAL | v2.1D |
| Variance Analysis | variance projection | NO | v2.1C |
| Accessorial Spend | settlement accessorials | YES | v2.1D |
| Accrual Exposure | accrual projection | NO | v2.1B/D |
| Carrier Cost Performance | aggregates by carrier | PARTIAL | v2.1D |
| Lane Cost Performance | snapshot locations | PARTIAL | v2.1D |

Feature flag pattern: `NUXT_PUBLIC_FREIGHT_COST_WORKSPACE_ENABLED` default OFF (mirror v2.0D).

---

## 37. KPI Definitions

| KPI | Numerator | Denominator | Currency | NULL behavior | Cancelled |
|-----|-----------|-------------|----------|---------------|-----------|
| Planned Freight Cost | SUM(planned_cost) | — | single currency filter | exclude NULL | exclude |
| Accrued Freight Cost | SUM(accrued) | — | single currency | NULL if no snapshot | exclude |
| Settled Freight Cost | SUM(settled ex-VAT) | — | single currency | NULL if no settlement | exclude |
| Invoiced | SUM(register total_with_vat) | — | single currency | NULL if not billed | exclude |
| Paid | SUM(paid_amount) | — | single currency | NULL if no obligation | exclude |
| Absolute Variance | settled - planned | — | must match | NULL if either NULL | exclude |
| Variance % | variance / planned * 100 | planned > 0 | — | NULL if planned ≤ 0 | exclude |
| Accessorial Cost % | SUM(approved accessorials) / settled | settled > 0 | single currency | NULL | exclude |
| Unbilled Accrual | SUM(accrued where not invoiced) | — | single currency | — | exclude |
| Settled Unpaid Exposure | SUM(settled - paid) | — | single currency | NULL components excluded | exclude |

---

## 38. Financial Invariants

| ID | Invariant | Status |
|----|-----------|--------|
| FMC-INV-001 | PLANNED_COST uses immutable pricing snapshot | **PASS** (existing) |
| FMC-INV-002 | Latest contract rate does not reprice existing order | **PASS** (v2.0) |
| FMC-INV-003 | Settlement does not recalculate latest rate | **PASS** |
| FMC-INV-004 | Billing does not recalculate freight rate | **PASS** |
| FMC-INV-005 | Payment does not recalculate freight rate | **PASS** |
| FMC-INV-006 | No canonical float64 money in new cost layer | **TARGET** (legacy float in settlement domain) |
| FMC-INV-007 | No mixed currency aggregation without FX | **TARGET** |
| FMC-INV-008 | Fuel double count deny | **PASS** (settlement uses total_amount) |
| FMC-INV-009 | Accessorial double count deny | **TARGET** (ledger source tags) |
| FMC-INV-010 | Event replay double count deny | **TARGET** (v2.1B) |
| FMC-INV-011 | Cross-tenant cost access deny | **TARGET** (v2.1A) |
| FMC-INV-012 | Cross-company internal cost disclosure deny | **TARGET** (v2.1E) |
| FMC-INV-013 | Financial correction preserves history | **PARTIAL** (payment void yes; settlement workflow mutable) |
| FMC-INV-014 | Actual cost ≠ payment status | **PASS** |
| FMC-INV-015 | Analytics projection not canonical financial writer | **TARGET** |

---

## 39. Failure Modes

| Condition | Behavior |
|-----------|----------|
| Pricing snapshot absent | Fail closed; planned=NULL; no zero substitute |
| Currency absent | Reject aggregation |
| Settlement unavailable | actual=NULL; accrual may = planned + approved only |
| Duplicate source event | Idempotency skip |
| Mixed currency | Reject variance |
| Projection lag | Serve stale with `cost_updated_at` + lag metric |
| UNKNOWN_AMOUNT | **≠ ZERO** |

---

## 40. NULL / Zero Semantics

```text
NULL = unknown / not yet available / not calculated
0    = known zero (e.g. zero approved accessorials)
```

Never COALESCE financial NULL to 0 in analytics SQL.

---

## 41. Performance / Scale (architecture notes)

Expected hot paths:

- Lookup by `(tenant_id, transport_order_id)` — snapshot already indexed
- Lookup by `(tenant_id, shipment_id)` — settlement unique per shipment
- Dashboard aggregates by `(tenant_id, buyer_company_id, date range, currency)`

Recommend:

- Projection table indexes: `(tenant_id, buyer_company_id, currency_code, planned_at)`, `(tenant_id, carrier_company_id, ...)`
- Batch internal API for register/settlement hydration — avoid N+1
- No cross-tenant scans

---

## 42. Observability (conceptual metrics)

```text
freight_cost_projection_event_total{result}
freight_cost_projection_error_total{reason}
freight_cost_duplicate_event_total
freight_cost_reconciliation_mismatch_total
freight_cost_unknown_source_total
freight_cost_currency_mismatch_total
freight_cost_projection_lag_seconds
```

No monetary payloads in logs.

---

## 43. Test Strategy (future IDs)

| Family | Example scenarios |
|--------|-------------------|
| FC-PLAN-001 | Contract rate snapshot → planned cost |
| FC-PLAN-002 | RFx aggregate-only snapshot |
| FC-ACCRUAL-001 | Planned + approved accessorial |
| FC-ACTUAL-001 | Settlement ex-VAT canonical |
| FC-VAR-001 | Detention increases actual |
| FC-VAR-002 | NULL actual before settlement |
| FC-LEDGER-001 | Duplicate event idempotency |
| FC-LEDGER-002 | Replay rebuild |
| FC-SEC-001 | Cross-tenant deny |
| FC-SEC-002 | Carrier cannot read buyer variance internal |
| FC-E2E-001 | Full chain TO → settlement → billing → payment → projection |

---

## 44. Migration Strategy (design only — NOT IMPLEMENTED)

Conceptual schema `freight_cost`:

```text
cost_entry (append-only)
  id, tenant_id, transport_order_id, shipment_id,
  entry_kind, amount NUMERIC(18,2), currency_code,
  source_service, source_type, source_id,
  supersedes_entry_id, created_at
  UNIQUE (tenant_id, source_service, source_type, source_id, entry_kind)

transport_order_cost_summary (projection)
  tenant_id, transport_order_id, shipment_id,
  buyer_company_id, carrier_company_id, currency_code,
  planned_amount, accrued_amount, settled_amount, invoiced_amount, paid_amount,
  variance_amount, variance_percent,
  pricing_source, rate_snapshot_id, settlement_id, billing_register_id,
  cost_updated_at, projection_version
```

No cross-service FKs — UUID references only.

---

## 45. Bounded Context Decision

| Field | Value |
|-------|-------|
| `FREIGHT_COST_BOUNDED_CONTEXT` | **HYBRID** |
| `CANONICAL_PLANNED_COST_OWNER` | **transport-order-service** (`transport_order_rate_snapshots.total_amount`) |
| `CANONICAL_ACCRUAL_OWNER` | **freight-cost-service** (derived ledger — **new**) |
| `CANONICAL_SETTLED_COST_OWNER` | **billing-register-service** (`freight_settlements.total_without_vat`) |
| `CANONICAL_INVOICED_AMOUNT_OWNER` | **billing-register-service** (`billing_registers.total_with_vat`) |
| `CANONICAL_PAID_AMOUNT_OWNER` | **payment-service** (`payment_obligations.paid_amount`) |
| `VARIANCE_PROJECTION_OWNER` | **freight-cost-service** (derived, not canonical writer) |

---

## 46. Recommended Implementation Slices

### v2.1A — Freight Cost Foundation
- Domain types, source reference model, invariants
- Internal read API skeleton (no public gateway yet)
- Tenant/company isolation design
- Idempotency contract for entries

### v2.1B — Accrual & Cost Ledger
- `freight-cost-service` schema + append-only ledger
- Ingest snapshot + accessorial + settlement events
- Accrual projection rebuild
- Decimal-safe boundaries

### v2.1C — Planned vs Actual / Variance
- Variance engine (NULL-safe, single currency)
- Reason attribution (auto + manual tags)
- Reconciliation job vs settlement totals

### v2.1D — Cost Analytics Workspace
- web-procurement or web-admin workspace (feature flag OFF)
- Overview, planned vs actual, shipment detail, accessorial spend
- RU/EN/ZH i18n

### v2.1E — Public API / RBAC / E2E / Hardening
- api-gateway public routes (mirror v2.0E pattern)
- Strict DTOs, OpenAPI, PostgreSQL integration tests
- Cross-company denial tests, financial E2E

---

## 47. Out of Scope for v2.1 MVP

General ledger, chart of accounts, double-entry, bank integration, tax/VAT engine redesign, FX, ERP posting, revenue/margin, dynamic pricing AI, claims management, CO2 accounting, route optimization.

---

## 48. Architecture Decision Table

| Decision | Value |
|----------|-------|
| Freight cost bounded context | **HYBRID** (dedicated projection service + existing canonical domains) |
| Planned cost canonical owner | **transport-order-service** / TO rate snapshot |
| Accrual canonical owner | **freight-cost-service** (derived ledger) |
| Actual/settled cost canonical owner | **billing-register-service** / freight_settlements |
| Invoiced amount owner | **billing-register-service** / billing_registers |
| Paid amount owner | **payment-service** / payment_obligations |
| Variance owner | **freight-cost-service** (derived) |
| Cost ledger required | **YES** |
| Ledger immutable | **YES** (append-only + reversal) |
| Mixed currency aggregation | **NOT_ALLOWED** |
| FX conversion v2.1 | **NOT_IN_SCOPE** |
| Cost corrections | **Reversal entries** referencing canonical domain corrections |
| Historical repricing | **NO** |
| Settlement repricing from latest rate | **NO** |
| Billing repricing | **NO** |
| Payment repricing | **NO** |
| Analytics projection canonical writer | **NO** (read-only derived) |

---

## 49. Source-of-Truth Matrix

| Financial fact | Canonical owner | Persistent source | Immutable? | Derived? |
|----------------|-----------------|-------------------|------------|----------|
| RFx bid | rfx-service | `rfx.bids.total_amount` | NO | NO |
| RFx award (legacy) | rfx-service | `rfx.rfx_award_transport_orders.amount` | NO | NO |
| Contract rate (master) | contract-rate-service | `contract_rate.rate_component` | ACTIVE version yes | NO |
| TO planned price | transport-order-service | `transport.transport_order_rate_snapshots.total_amount` | **YES** | NO |
| Accrued cost | freight-cost-service (v2.1) | `freight_cost.cost_entry` / projection | YES (entries) | **YES** |
| Approved accessorial | billing-register-service | `billing.settlement_accessorials` | NO (workflow) | NO |
| Settlement (ex-VAT) | billing-register-service | `billing.freight_settlements.total_without_vat` | PARTIAL | NO |
| Invoice/billing amount | billing-register-service | `billing.billing_registers.total_with_vat` | After include | NO |
| Payment obligation | payment-service | `billing.payment_obligations.original_amount` | YES | NO |
| Paid amount | payment-service | `billing.payment_obligations.paid_amount` | Derived from allocations | PARTIAL |
| Planned vs actual variance | freight-cost-service (v2.1) | projection | N/A | **YES** |

---

## 50. Design Review / Self-Challenge

| # | Question | Answer |
|---|----------|--------|
| 1 | Second settlement SSOT? | **NO** — ledger references settlement |
| 2 | Duplicating TO snapshot? | **NO** — store snapshot_id + planned amount once in ledger |
| 3 | Fuel double count? | **Prevented** — use total_amount; ledger must not add components on top |
| 4 | Event replay duplicate? | **Prevented** — idempotency keys (v2.1B) |
| 5 | Contract activation alters historical cost? | **NO** (v2.0) |
| 6 | Payment status changes actual? | **NO** |
| 7 | Billing reprices shipment? | **NO** |
| 8 | Carrier sees buyer internal analytics? | **Denied by design** — separate projections |
| 9 | Company A sees Company B analytics? | **Denied** — membership scoped |
| 10 | Actual before settlement? | **NULL** — not zero |
| 11 | NULL actual meaning? | Unknown / not yet settled |
| 12 | Settlement corrected? | Settlement recalc + ledger reversal |
| 13 | Different currencies? | Fail closed — no variance |
| 14 | Projections rebuildable? | **YES** — from ledger + canonical APIs |
| 15 | Dashboard reconcilable to sources? | **YES** — reconciliation job required |

---

## 51. Open Questions / Blockers

| ID | Severity | Question | Evidence | Decision needed | Blocks v2.1A |
|----|----------|----------|----------|-----------------|--------------|
| OQ-001 | MEDIUM | Should accrual include PROPOSED accessorials or only APPROVED? | Settlement workflow exists | Finance policy — recommend APPROVED only | NO |
| OQ-002 | MEDIUM | Emit outbox from billing-register or poll audit tables? | Audit exists; no settlement outbox | Integration pattern | NO (blocks v2.1B) |
| OQ-003 | LOW | Migrate settlement domain from float64 to decimal? | float64 in billing-register domain | Parallel tech debt | NO |
| OQ-004 | MEDIUM | Auto-sync billing register if settlement changes post-include? | **NOT_FOUND** today | Product decision — recommend manual re-include v2.1 | NO |
| OQ-005 | LOW | Standardize accessorial charge_code enum for WAITING/DETENTION auto attribution? | Free-form string today | Convention doc | NO |
| OQ-006 | HIGH | Which settlement statuses count as "final" for actual cost? | Status enum in migration 000042 | Freeze set: APPROVED+ | NO — resolve in v2.1A |

No **OPEN_BLOCKER** for architecture review.

---

## Document control

| Field | Value |
|-------|-------|
| Author | Architecture discovery agent |
| Review status | Pending independent architecture + financial-integrity review |
| Runtime changes | **NONE** |
| Next step | Architecture review → then v2.1A implementation planning |
