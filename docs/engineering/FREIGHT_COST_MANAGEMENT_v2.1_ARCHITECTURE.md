# FREIGHT COST MANAGEMENT v2.1 — Architecture & Current-State Discovery

**Status:** Architecture freeze — **review-amended** (documentation only)
**Branch:** `arch/freight-cost-management-v2.1`
**Review branch:** `review/freight-cost-management-v2.1`
**Base SHA:** `ea7721c188b4cf2e10f40f1a8a4dd5e57104a2be` (v2.0E merged)
**Date:** 2026-08-21
**Review date:** 2026-08-21 — see `docs/architecture/FREIGHT_COST_MANAGEMENT_v2.1_FINAL_REVIEW.md`

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
| Billing register aggregate | Billing register (closing basis) |
| Invoice document amount | `billing.invoices` / `billing.vat_invoices` (when created) |
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

## 7. Billing / Closing Documents (review-amended)

| Field | Value |
|-------|-------|
| `BILLING_SOURCE_OF_AMOUNT` | Settlement snapshot at **include time** — item `base_amount` ← settlement.base_freight; `extra_charges` ← approved_accessorial_total |
| `BILLING_FINALITY` | Register status machine through CLOSED |
| `BILLING_ADJUSTMENTS` | Manual register items still supported (legacy) |
| `BILLING_CURRENCY` | Register `currency_code`; must match settlement on include |
| `BILLING_LINK_TO_SETTLEMENT` | `billing_register_items.settlement_id` |
| `CAN_BILLING_RECALCULATE_FREIGHT_PRICE?` | **NO** — copies settlement at include; register totals = SUM(items) |

**Terminology correction:** **Billing register ≠ invoice document.**

| Amount type | Owner | Source |
|-------------|-------|--------|
| `BILLING_REGISTER_AMOUNT` (aggregate) | billing-register-service | `billing_registers.total_without_vat` / `total_with_vat` |
| `BILLED_SNAPSHOT_AMOUNT` (line) | billing-register-service | `billing_register_items` copied at include — **frozen** |
| `INVOICE_DOCUMENT_AMOUNT` | billing-register-service | `billing.invoices.total_amount`, `billing.vat_invoices.amount_with_vat` created from approved register via closing-document package |
| `PAYMENT_OBLIGATION_SOURCE` | payment-service | `billing_registers.total_with_vat` only |

Payment obligation is based on **register payable total**, not invoice row directly — but invoice/act/VAT/UPD are separate document metadata SSOT (`FREIGHT_BILLING_CLOSING_v1.8_ARCHITECTURE.md`).

### Settlement ↔ billing divergence (OQ-004 RESOLVED)

Repository behavior verified:

- Inclusion allowed when settlement status ∈ {APPROVED, DOCUMENTS_READY, READY_FOR_PAYMENT} and no open dispute (`settlement_billing_eligibility.go`).
- Register item amounts are **copied once** at include; **no auto-sync** if settlement later changes.
- After APPROVED, new accessorials **cannot** be proposed; monetary drift from accessorial workflow is blocked post-approval.
- **APPROVED → DISPUTED** is allowed **after** register inclusion; register item **stays frozen** unless buyer removes settlement while register still DRAFT/CALCULATED (`ValidateDeleteItemRegisterStatus`).
- `RemoveSettlement` clears settlement register link and deletes item; only when register status ∈ {DRAFT, CALCULATED}.

**Frozen divergence model:**

| State | Meaning |
|-------|---------|
| `SETTLEMENT_BILLING_MATCH` | Linked settlement ex-VAT total equals billed line ex-VAT (`item.amount_without_vat`) |
| `SETTLEMENT_BILLING_MISMATCH` | Linked but totals differ (e.g. post-include dispute, manual ops) or settlement disputed while register item frozen |
| `SETTLEMENT_BILLING_UNLINKED` | Settlement not in register |

v2.1 projection must surface `billing_reconciliation_status` — **never silently equate** `CURRENT_SETTLEMENT_AMOUNT` and `BILLED_SNAPSHOT_AMOUNT`.

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

## 13. Actual Cost Definition (FROZEN for v2.1 — review-amended)

Repository evidence: `ValidateSettlementTransition` (`freight_settlement.go`) allows **APPROVED → DISPUTED**; **DOCUMENTS_READY → READY_FOR_PAYMENT** only (no return to DISPUTED). Accessorial propose/approve is blocked after APPROVED (`ProposeAccessorial` DRAFT/UNDER_REVIEW only). Settlement `base_freight_amount` is immutable after create; `total_without_vat` recalculates only from APPROVED accessorial changes while still PROPOSED.

v2.1 distinguishes **three settlement amount concepts** (do not conflate):

| Concept | Definition | When available |
|---------|------------|----------------|
| `CURRENT_SETTLEMENT_AMOUNT` | Live `freight_settlements.total_without_vat` | Settlement exists and status ≠ CANCELLED |
| `CURRENT_ACTUAL_COST` | Same ex-VAT field when financially accepted | Status ∈ {APPROVED, DOCUMENTS_READY, READY_FOR_PAYMENT} **and** open_disputes = 0 |
| `FINAL_ACTUAL_COST` | Same ex-VAT field at terminal settlement confidence | Status = **READY_FOR_PAYMENT** only |

**Frozen status semantics (OQ-006 RESOLVED):**

```text
ACTUAL_COST_AVAILABLE_STATUSES = APPROVED, DOCUMENTS_READY, READY_FOR_PAYMENT
  (requires open_disputes = 0)

FINAL_ACTUAL_COST_STATUSES = READY_FOR_PAYMENT

ACTUAL_COST_NULL_STATUSES = DRAFT, UNDER_REVIEW, DISPUTED (or any status with open dispute), CANCELLED

CANCELLED_ACTUAL_COST_SEMANTICS = NULL for actual/final; planned snapshot remains historical read-only fact
```

Before settlement exists: **`CURRENT_ACTUAL_COST = NULL`**, **`FINAL_ACTUAL_COST = NULL`** (not zero).

| Concept | Canonical owner | Field |
|---------|-----------------|-------|
| CURRENT / FINAL actual freight (ex-VAT) | billing-register-service | `freight_settlements.total_without_vat` |
| Billing register aggregate (ex-VAT) | billing-register-service | `billing_registers.total_without_vat` |
| Billing register aggregate (payable) | billing-register-service | `billing_registers.total_with_vat` |
| Invoice document amount | billing-register-service | `billing.invoices.total_amount` / `billing.vat_invoices.amount_with_vat` |
| Payment obligation (payable) | payment-service | `payment_obligations.original_amount` ← register `total_with_vat` |
| Paid amount (cash applied) | payment-service | `payment_obligations.paid_amount` (persisted derived field updated on allocation TX) |

**FMC-INV-014:** Payment status / paid amount does **not** change actual freight cost.

---

## 14. Accrual Model (v2.1 design — OQ-001 RESOLVED)

**Current:** `ACCRUAL_MODEL=NOT_FOUND` in runtime.

**Frozen financial accrual (conservative):**

```text
FINANCIAL_ACCRUAL (ex-VAT) =
  PLANNED_COST (snapshot.total_amount)
  + SUM(settlement accessorials WHERE status = APPROVED)
  when settlement exists and currency matches; else PLANNED only if shipment exists; else NULL
```

```text
ACCRUAL_INCLUDES_PROPOSED = NO
ACCRUAL_INCLUDES_APPROVED = YES
ACCRUAL_INCLUDES_DISPUTED = NO
ACCRUAL_INCLUDES_REJECTED = NO
```

**Separate operational concept (not canonical financial accrual):**

```text
FORECAST_EXPOSURE (ex-VAT, non-ledger KPI) =
  PLANNED_COST + SUM(accessorials WHERE status = PROPOSED)
```

Forecast is UI/ops only; must not feed payment, billing, or canonical accrual.

**Persistence:** Option D — immutable `freight_cost.cost_entry` + rebuildable projection; accrual is **derived**, not a second settlement.

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

## 16. Ledger Semantics (recommended — review-amended)

**Ledger authority:** **Option D — derived event journal / reconciliation ledger**. It is **NOT** a canonical financial writer.

```text
LEDGER_CANONICAL_FINANCIAL_WRITER = NO
LEDGER_AUTHORITY = DERIVED_EVENT_JOURNAL
CANONICAL_SOURCE_AUTHORITY = domain services (snapshot, settlement, billing, payment)
RECONCILIATION_BEHAVIOR = on mismatch, canonical source wins; projection/ledger marked MISMATCH; never mutate canonical
DERIVED_LEDGER_CAN_CORRECT_CANONICAL_SOURCE = NO
```

| Field | Value |
|-------|-------|
| `LEDGER_ENTRY_IMMUTABLE` | **YES** — append-only |
| `LEDGER_DELETE_ALLOWED` | **NO** — reversal entry only |
| `CORRECTION_MODEL` | `REVERSAL` entry referencing `supersedes_entry_id`; canonical correction remains in source domain |
| `LEDGER_IDEMPOTENCY_KEY` | **`UNIQUE (tenant_id, source_event_id)`** — NOT `(source_id, entry_kind)` alone |
| `SOURCE_REVISION_MODEL` | `source_revision` = monotonic domain version (`settlement.version`, allocation id, audit sequence); replays use same `source_event_id`; new revisions get new event id |
| `REPLAY_DUPLICATE` | **DENY** (same source_event_id) |
| `NEW_REVISION_ACCEPTED` | **YES** (new source_event_id + optional supersedes link) |
| `OUT_OF_ORDER_REVISION_SAFE` | **YES** — apply only if `source_revision` > projection `last_source_revision` for aggregate |

**Stored amounts in ledger:** `amount` is **`DERIVED_SNAPSHOT_VALUE`** copied at ingest time for audit/rebuild speed — traceability required:

```text
source_service, source_type, source_id, source_version, source_event_id, source_occurred_at
entry_kind, amount, currency_code, supersedes_entry_id
```

Do **not** treat ledger amount as new canonical price if it diverges from canonical read API.

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

## 19. Variance Model (review-amended)

All variance comparisons use **ex-VAT** amounts only.

```text
current_variance_amount = current_actual_cost - planned_cost
final_variance_amount   = final_actual_cost   - planned_cost

current_variance_percent = current_variance_amount / planned_cost * 100  (planned_cost > 0)
final_variance_percent   = final_variance_amount   / planned_cost * 100  (planned_cost > 0)
```

| Edge case | Behavior |
|-----------|----------|
| planned_cost = 0 | variance_percent = NULL |
| current/final actual NULL | variance = NULL |
| currency mismatch | variance = NULL; fail closed |
| cancelled order | exclude from active variance; historical read retains planned |
| settlement disputed (open) | current_actual NULL → current_variance NULL |

**Sign:** positive = over plan; negative = saving.

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

## 25. Event / Integration Model (OQ-002 RESOLVED)

| Event | Current transport | v2.1 target |
|-------|-------------------|-------------|
| TO snapshot created | Sync create; **no outbox** | **Rebuild:** transport-order internal read API; optional outbox in v2.1B |
| Settlement / accessorial change | Audit append-only | **New transactional outbox** from billing-register-service (v2.1B) |
| Billing register include/remove | Audit append-only | Same outbox or register audit with `source_event_id` |
| Payment allocation / paid | `billing.payment_outbox` exists | **Consume existing** `payment_obligation.paid` |

**Frozen (hybrid D):**

```text
SETTLEMENT_CHANGE_TRANSPORT = transactional outbox (preferred for v2.1B ongoing ingest)
SNAPSHOT_CHANGE_TRANSPORT   = internal read API + optional future outbox
PAYMENT_CHANGE_TRANSPORT    = existing payment_outbox
REBUILD_SOURCE              = canonical domain read APIs (NOT ledger, NOT audit polling alone)
```

Audit tables may bootstrap historical backfill once; ongoing ingest must not rely on polling alone (lost-event risk).

**Principle:** `CROSS_SERVICE_FINANCIAL_READ = internal API or canonical event/projection` — freight-cost-service must not use ad-hoc cross-schema SQL in production.

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

## 27. Rebuild / Reconciliation (review-amended)

**Authoritative rebuild procedure (no circular dependency):**

```text
REBUILD_ROOT_SOURCE = canonical domain read APIs
  1. transport-order-service: snapshot by TO id
  2. billing-register-service: settlement + accessorials + register links
  3. payment-service: obligation + allocations by register id
  2→ derive cost_entry journal (idempotent by source_event_id)
  3→ rebuild transport_order_cost_summary projection
  4→ reconciliation checksum vs canonical APIs; emit mismatch metric
```

Ledger is **not** rebuild root — it is rebuilt **from** canonical APIs/events.

```text
EVENT_REPLAY_DOUBLE_COUNT = DENY
DUPLICATE_EVENT_DOUBLE_COUNT = DENY
```

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

## 30. Tax Boundary (review-amended)

Evidence: v2.0 architecture freeze `RATE_SNAPSHOT_VAT = EXCLUDED` — VAT owned by settlement/billing, not snapshot. RFx pricing API uses pre-VAT `total_amount` (`pricing_integration_test.go`). Settlement applies `vat_rate` separately.

**Frozen tax bases:**

| Amount | Tax basis |
|--------|-----------|
| `PLANNED_COST` | **EX-VAT commercial freight** (`snapshot.total_amount`) |
| `CURRENT_ACTUAL_COST` / `FINAL_ACTUAL_COST` | **EX-VAT** (`settlement.total_without_vat`) |
| `FINANCIAL_ACCRUAL` | **EX-VAT** |
| `BILLING_REGISTER_AMOUNT` (freight analytics) | **EX-VAT** when comparing to planned/actual; **with-VAT** for payable views |
| `PAYMENT_OBLIGATION` / `PAID_AMOUNT` | **WITH-VAT payable** (from register `total_with_vat`) |

```text
PLANNED_ACTUAL_TAX_BASIS_COMPATIBLE = YES
  (planned vs current/final variance uses ex-VAT fields only)

PAYABLE_ANALYTICS_SEPARATE = YES
  (do not subtract paid_with_vat from planned_ex_vat)
```

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
cost_entry (append-only derived journal)
  id, tenant_id, transport_order_id, shipment_id,
  entry_kind, amount NUMERIC(18,2), currency_code,
  source_service, source_type, source_id, source_version,
  source_event_id, source_occurred_at,
  supersedes_entry_id, created_at
  UNIQUE (tenant_id, source_event_id)

transport_order_cost_summary (projection)
  ...
  planned_amount, accrued_amount,
  current_actual_amount, final_actual_amount,
  billed_snapshot_amount, invoiced_document_amount, paid_amount,
  current_variance_amount, final_variance_amount,
  billing_reconciliation_status,  -- MATCH | MISMATCH | UNLINKED
  last_source_revision, cost_updated_at
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

## 46. Recommended Implementation Slices (review-amended)

### v2.1A — Freight Cost Foundation
**Entry gate:** OQ-006, tax basis, ledger authority, idempotency, visibility — frozen in final review.

- Domain types, finality enums, source reference model, invariants
- Internal read API skeleton; tenant/company isolation design

### v2.1B — Accrual & Cost Ledger
**Entry gate:** OQ-002 transport; decimal ingest; rebuild root defined.

- Schema + settlement/billing outbox ingest + payment outbox consumer
- Ledger with `UNIQUE(tenant_id, source_event_id)`

### v2.1C — Planned vs Actual / Variance
- Current vs final variance (ex-VAT, NULL-safe)
- Accessorial semantic classification for double-count prevention
- Canonical API reconciliation

### v2.1D — Cost Analytics Workspace
- Feature flag OFF; buyer vs carrier field masks
- Billing mismatch indicators

### v2.1E — Public API / RBAC / E2E / Hardening
- Gateway routes, strict DTOs, financial E2E

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
| Actual/settled cost canonical owner | **billing-register-service** / `total_without_vat` |
| Billing register amount owner | **billing-register-service** / register + item snapshot |
| Invoice document amount owner | **billing-register-service** / `invoices`, `vat_invoices` |
| Paid amount owner | **payment-service** / `payment_obligations.paid_amount` |
| Variance owner | **freight-cost-service** (current + final derived) |
| Cost ledger required | **YES** (derived journal) |
| Ledger canonical financial writer | **NO** |
| Ledger immutable | **YES** (append-only + reversal) |
| Settlement/billing divergence | **MATCH / MISMATCH / UNLINKED** |
| Planned vs actual tax basis | **COMPATIBLE (EX-VAT)** |
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
| Approved accessorial (execution) | billing-register-service | `settlement_accessorials` APPROVED | NO | NO |
| Proposed accessorial | billing-register-service | `settlement_accessorials` PROPOSED | NO | NO |
| Financial accrual | freight-cost-service | projection | N/A | **YES** |
| Current actual (ex-VAT) | billing-register-service | `freight_settlements.total_without_vat` | PARTIAL | NO |
| Final actual (ex-VAT) | billing-register-service | same; status=READY_FOR_PAYMENT | PARTIAL | NO |
| Billed snapshot (line) | billing-register-service | `billing_register_items` at include | YES | NO |
| Billing register aggregate | billing-register-service | `billing_registers.total_with_vat` | Until closed | NO |
| Invoice document | billing-register-service | `billing.invoices` / `vat_invoices` | After issue | NO |
| Payment obligation | payment-service | `payment_obligations.original_amount` | YES | NO |
| Paid amount | payment-service | `payment_obligations.paid_amount` | Updated on allocation TX | PARTIAL |
| Ledger entry | freight-cost-service | `cost_entry` | YES | **YES** (derived snapshot) |
| Variance | freight-cost-service | projection | N/A | **YES** |

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

## 51. Open Questions / Blockers (post-review)

| ID | Status | Resolution |
|----|--------|------------|
| OQ-001 | **RESOLVED** | Accrual = planned + APPROVED accessorials only; PROPOSED → forecast_exposure |
| OQ-002 | **RESOLVED** | Hybrid: outbox for settlement/billing changes; API rebuild root; payment outbox consumed |
| OQ-003 | **RESOLVED (boundary)** | freight-cost-service decimal-only; float64 legacy remains in billing-register domain until separate migration |
| OQ-004 | **RESOLVED** | SETTLEMENT_BILLING_MATCH/MISMATCH/UNLINKED; frozen register item at include |
| OQ-005 | **OPEN (LOW)** | charge_code convention for auto variance — defer to v2.1C |
| OQ-006 | **RESOLVED** | current_actual vs final_actual status sets frozen (§13) |

No **OPEN_BLOCKER** or **OPEN_HIGH** for architecture approval.

---

## Document control

| Field | Value |
|-------|-------|
| Author | Architecture discovery agent |
| Review status | Pending independent architecture + financial-integrity review |
| Runtime changes | **NONE** |
| Next step | Architecture review → then v2.1A implementation planning |
