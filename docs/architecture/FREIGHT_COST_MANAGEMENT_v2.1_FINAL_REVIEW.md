# FREIGHT COST MANAGEMENT v2.1 — Final Architecture & Financial Integrity Review

**Review date:** 2026-08-21  
**Architecture PR:** #41 (`arch/freight-cost-management-v2.1`)  
**Architecture head reviewed:** `4974d8f5a09e542987d7953b0c54c7f75b77001a`  
**Review branch:** `review/freight-cost-management-v2.1`  
**Base:** `ea7721c188b4cf2e10f40f1a8a4dd5e57104a2be` (v2.0E merged)  
**Reviewer:** Independent architecture / financial-integrity review (documentation only)

---

## Executive verdict

**FINAL_VERDICT: PASS** (after architecture amendments in this review)

The initial v2.1 architecture document was directionally correct (HYBRID bounded context, snapshot-based planned cost, settlement as commercial SSOT) but had **four HIGH gaps** in finality semantics, ledger idempotency, billing/invoice terminology, and settlement–billing divergence modeling. All are **remediated** in the amended `FREIGHT_COST_MANAGEMENT_v2.1_ARCHITECTURE.md`.

**Not approved for implementation yet** — this review approves **architecture only**. v2.1A entry gates are now defined.

---

## Findings summary

| Severity | Count (initial) | Open after remediation |
|----------|-----------------|------------------------|
| BLOCKER | 0 | 0 |
| HIGH | 4 | 0 |
| MEDIUM | 3 | 0 |
| LOW | 2 | 1 (OQ-005 charge_code convention) |
| NIT | 1 | 1 |

---

## Findings register

### F-HIGH-001 — Actual cost finality ambiguous (OQ-006)

| Field | Value |
|-------|-------|
| Section | §13 Actual Cost |
| Evidence | `ValidateSettlementTransition`: APPROVED→DISPUTED allowed; READY_FOR_PAYMENT terminal; accessorial propose blocked after APPROVED (`freight_settlement.go`, `freight_settlement_repository.go`) |
| Problem | Initial doc treated APPROVED+ as uniformly "actual/settled" |
| Impact | Premature variance; disputed settlements shown as final |
| Remediation | Introduced `CURRENT_ACTUAL_COST` vs `FINAL_ACTUAL_COST`; frozen status sets in architecture §13 |
| Status | **RESOLVED** |

### F-HIGH-002 — Ledger idempotency prevents source revisions

| Field | Value |
|-------|-------|
| Section | §16 / §44 |
| Evidence | Initial `UNIQUE(tenant_id, source_service, source_type, source_id, entry_kind)` |
| Problem | Settlement is versioned; reversals/superseding entries blocked |
| Impact | EVENT_REPLAY / revision handling broken; ledger could not represent dispute lifecycle |
| Remediation | `UNIQUE(tenant_id, source_event_id)` + `source_revision` monotonic guard |
| Status | **RESOLVED** |

### F-HIGH-003 — Billing register conflated with invoice

| Field | Value |
|-------|-------|
| Section | §7, terminology |
| Evidence | `billing.invoices`, `billing.vat_invoices`, `billing.acts` (migration 000006); `FREIGHT_BILLING_CLOSING_v1.8_ARCHITECTURE.md` |
| Problem | "Invoiced amount = register total" overstated equivalence |
| Impact | Wrong KPI semantics; payment obligation source obscured |
| Remediation | Separate BILLING_REGISTER_AMOUNT vs INVOICE_DOCUMENT_AMOUNT; obligation from register `total_with_vat` |
| Status | **RESOLVED** |

### F-HIGH-004 — Settlement/billing divergence not modeled (OQ-004)

| Field | Value |
|-------|-------|
| Section | §7 |
| Evidence | `IncludeSettlement` copies amounts once; no back-sync; APPROVED→DISPUTED after include; `RemoveSettlement` only DRAFT/CALCULATED register |
| Problem | Architecture implied register always equals current settlement |
| Impact | Silent financial inconsistency in analytics |
| Remediation | MATCH / MISMATCH / UNLINKED reconciliation states |
| Status | **RESOLVED** |

### F-MED-001 — Accrual included ambiguous accessorial states (OQ-001)

| Field | Value |
|-------|-------|
| Section | §14 |
| Evidence | Accessorial statuses PROPOSED/APPROVED/DISPUTED/REJECTED |
| Problem | Unclear whether PROPOSED counts toward financial accrual |
| Remediation | Accrual = APPROVED only; PROPOSED → separate `forecast_exposure` |
| Status | **RESOLVED** |

### F-MED-002 — Event transport undecided (OQ-002)

| Field | Value |
|-------|-------|
| Section | §25 |
| Evidence | Payment outbox exists; settlement audit-only |
| Problem | Audit polling alone insufficient for atomic ingest |
| Remediation | Hybrid: outbox for settlement/billing; API rebuild root; consume payment outbox |
| Status | **RESOLVED** |

### F-MED-003 — float64 boundary unspecified (OQ-003)

| Field | Value |
|-------|-------|
| Section | §17 |
| Evidence | billing-register domain float64; payment decimal |
| Problem | Unclear whether freight-cost-service may ingest float64 |
| Remediation | `FREIGHT_COST_SERVICE_ACCEPTS_FLOAT_CANONICAL_MONEY=NO`; parse NUMERIC/decimal at boundary |
| Status | **RESOLVED (boundary)** — full settlement decimal migration out of v2.1 scope |

### F-LOW-001 — Single variance metric insufficient

| Field | Value |
|-------|-------|
| Section | §19 |
| Remediation | `current_variance` vs `final_variance` |
| Status | **RESOLVED** |

### F-LOW-002 — charge_code auto attribution (OQ-005)

| Field | Value |
|-------|-------|
| Section | §20 |
| Problem | Free-form charge_code |
| Status | **OPEN** — defer to v2.1C; not architecture blocker |

### F-NIT-001 — Stale "invoiced" label in KPI table

| Status | **RESOLVED** in amended architecture §37 |

---

## Frozen decisions (post-review)

### Planned cost

```text
PLANNED_COST_OWNER = transport-order-service
PLANNED_COST = transport_order_rate_snapshots.total_amount
PLANNED_COST_IMMUTABLE = YES
PLANNED_COST_TAX_BASIS = EX_VAT_COMMERCIAL (RATE_SNAPSHOT_VAT=EXCLUDED)
```

### Actual / settlement

```text
ACTUAL_COST_OWNER = billing-register-service

ACTUAL_COST_AVAILABLE_STATUSES = APPROVED, DOCUMENTS_READY, READY_FOR_PAYMENT
  (requires open_disputes = 0)

FINAL_ACTUAL_COST_STATUSES = READY_FOR_PAYMENT

ACTUAL_COST_NULL_STATUSES = DRAFT, UNDER_REVIEW, DISPUTED, CANCELLED, or open dispute

CANCELLED_ACTUAL_COST_SEMANTICS = NULL actual; planned remains historical
```

### Accrual (OQ-001)

```text
ACCRUAL_INCLUDES_PROPOSED = NO
ACCRUAL_INCLUDES_APPROVED = YES
ACCRUAL_INCLUDES_DISPUTED = NO
ACCRUAL_INCLUDES_REJECTED = NO
```

### Billing / invoice

```text
BILLING_REGISTER_AMOUNT_OWNER = billing-register-service
INVOICE_DOCUMENT_AMOUNT_OWNER = billing.invoices / billing.vat_invoices
PAYMENT_OBLIGATION_SOURCE = billing_registers.total_with_vat
SETTLEMENT_BILLING_DIVERGENCE_MODEL = MATCH | MISMATCH | UNLINKED
OQ_004_RESOLVED = YES
```

### Ledger

```text
LEDGER_REQUIRED = YES
LEDGER_AUTHORITY = DERIVED_EVENT_JOURNAL
LEDGER_SECOND_SSOT = NO
LEDGER_CANONICAL_FINANCIAL_WRITER = NO
LEDGER_IDEMPOTENCY_KEY = UNIQUE(tenant_id, source_event_id)
SOURCE_REVISION_MODEL = monotonic domain version per aggregate
REPLAY_DUPLICATE = DENY
NEW_REVISION_ACCEPTED = YES
OUT_OF_ORDER_REVISION_SAFE = YES
DERIVED_LEDGER_CAN_CORRECT_CANONICAL_SOURCE = NO
```

### Event / rebuild (OQ-002)

```text
SETTLEMENT_CHANGE_TRANSPORT = transactional outbox (v2.1B)
SNAPSHOT_CHANGE_TRANSPORT = internal read API (+ optional outbox)
PAYMENT_CHANGE_TRANSPORT = existing payment_outbox
REBUILD_ROOT_SOURCE = canonical domain read APIs
OQ_002_RESOLVED = YES
```

### Money / tax

```text
DECIMAL_SAFE_LEDGER_BOUNDARY = YES
FREIGHT_COST_SERVICE_ACCEPTS_FLOAT_CANONICAL_MONEY = NO
PLANNED_ACTUAL_TAX_BASIS_COMPATIBLE = YES
MIXED_CURRENCY_AGGREGATION = DENY
FX_V2_1 = OUT_OF_SCOPE
```

### Security

```text
CARRIER_BUYER_INTERNAL_ANALYTICS = DENY
CROSS_COMPANY_COST_ACCESS = DENY
CROSS_TENANT_COST_ACCESS = DENY
```

### Invariants (verified in repository)

```text
HISTORICAL_REPRICING = NO
SETTLEMENT_REPRICING = NO
BILLING_REPRICING = NO
PAYMENT_REPRICING = NO
FUEL_DOUBLE_COUNT = DENY (settlement uses snapshot total_amount)
ACCESSORIAL_DOUBLE_COUNT = DENY (requires semantic class tagging in v2.1C)
EVENT_REPLAY_DOUBLE_COUNT = DENY (with source_event_id idempotency)
UNKNOWN_AMOUNT_EQUALS_ZERO = NO
```

---

## Review gates

| Gate | Result |
|------|--------|
| OPEN_BLOCKER | 0 |
| OPEN_HIGH | 0 |
| OQ_001_RESOLVED | YES |
| OQ_002_RESOLVED | YES |
| OQ_006_RESOLVED | YES |
| OQ_004_RESOLVED | YES |
| ACTUAL_FINALITY_DEFINED | YES |
| SETTLEMENT_BILLING_DIVERGENCE_MODEL | RESOLVED |
| LEDGER_SECOND_SSOT | NO |
| LEDGER_IDEMPOTENCY_VERSION_SAFE | YES |
| REBUILD_ROOT_SOURCE_DEFINED | YES |
| DECIMAL_SAFE_LEDGER_BOUNDARY | YES |
| PLANNED_ACTUAL_TAX_BASIS_COMPATIBLE | YES |
| SOURCE_OF_TRUTH_MATRIX_UNAMBIGUOUS | YES |

---

## Remaining risks (non-blocking)

1. **OQ-005** — charge_code convention for automatic variance reasons (v2.1C).
2. **Settlement float64 domain** — ingest boundary mitigates; full decimal migration is separate tech debt.
3. **No snapshot outbox today** — rebuild relies on transport-order API; optional outbox reduces polling lag.
4. **Post-include dispute** — operational process must use register remove/re-include while register still DRAFT/CALCULATED; otherwise MISMATCH persists by design.

---

## Artifacts updated

| File | Action |
|------|--------|
| `docs/engineering/FREIGHT_COST_MANAGEMENT_v2.1_ARCHITECTURE.md` | Amended |
| `docs/architecture/FREIGHT_COST_MANAGEMENT_v2.1_FINAL_REVIEW.md` | Created |

```text
PRODUCT_RUNTIME_CHANGED = NO
DATABASE_CHANGED = NO
OPENAPI_RUNTIME_CHANGED = NO
FRONTEND_CHANGED = NO
```

---

## Next step

Merge review PR into `arch/freight-cost-management-v2.1`, then human architecture sign-off before **v2.1A implementation** (not authorized by this review).
