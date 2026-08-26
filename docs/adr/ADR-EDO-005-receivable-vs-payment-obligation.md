# ADR-EDO-005: Receivable vs PaymentObligation

## Status

Accepted — architecture freeze (EDO-0.2)

## Context

Discovery v0.1 mapped receivable analogue to `billing.payment_obligations` with `source_type` currently limited to `BILLING_REGISTER`. Freight finance target includes factoring: Receivable → FactoringApplication → FactorOffer → Assignment → Financing → Settlement.

EDO-0.2 explicitly forbids redefining `payment_obligations` as the canonical Receivable aggregate.

## Decision

### Distinct aggregates

| Aggregate | Canonical owner | Purpose |
|-----------|-----------------|---------|
| **PaymentObligation** | payment-service | Accounts payable/receivable **payment execution** intent tied to a closed billing basis; status `OPEN`/`PAID`/`OVERDUE`; drives allocations and `payment_obligation.paid` outbox |
| **Receivable** (future) | payment-service or dedicated FF module (FF workstream) | **Trade receivable** as financeable asset with debtor/creditor, due date, disputed/assigned/remaining amounts, factoring eligibility |

**PaymentObligation is a related downstream/upstream object, not the canonical Receivable.**

### Receivable conceptual model (design only)

```
Receivable
  ├── billing_register_id (optional source anchor)
  ├── document_id / upd_document_id (optional EDO legal anchor)
  ├── shipment_id (optional logistics anchor)
  ├── debtor_company_id (core.companies)
  ├── creditor_company_id (core.companies)
  ├── due_date, amount, currency
  ├── disputed_amount, assigned_amount, remaining_amount
  └── status (finance lifecycle — distinct from obligation PAID)

Receivable
  └── FactoringApplication
        └── FactorOffer[]
              └── Assignment
                    └── Financing
                          └── Settlement / Payment (may create PaymentObligation or Payment)
```

### Relationship to PaymentObligation

| Scenario | Behavior |
|----------|----------|
| Register closed → pay | Existing path: billing triggers **PaymentObligation** (`source_type=BILLING_REGISTER`) — unchanged |
| Factoring assignment | **Receivable** created from billing/UPD evidence; Assignment may **reference** existing PaymentObligation or create separate financing settlement obligations |
| Payment recorded | PaymentObligation PAID event updates billing; Receivable `remaining_amount` updated via FF domain rules |
| Duplicate prevention | Receivable and PaymentObligation linked by explicit `receivable_id` / `payment_obligation_id` reference — never merged tables |

### Evaluation of existing PaymentObligation

**Keep as-is for v1.9 payment reconciliation architecture.** Extend `source_type` additively in FF phase (`RECEIVABLE`, `FACTORING_ASSIGNMENT`) — do not rename table or overload semantics.

## Consequences

### Positive

- Factoring model can evolve without breaking payment reconciliation (v1.9.2 frozen contracts)
- Clear finance boundary: obligation = payment rail; receivable = asset rail
- Resolves discovery F-008 at architecture level

### Negative

- Two finance aggregates to reconcile in reporting — requires FF read models
- Cross-workstream events needed (`ff.receivable.*`) before factoring UI

## References

- `docs/engineering/FREIGHT_PAYMENTS_RECONCILIATION_v1.9.2_ARCHITECTURE.md`
- Discovery finding F-008
- ADR-EDO-002 (billing/UPD anchor)
