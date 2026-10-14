# ADR-FIN-001: Execution–document–settlement chain

## Status

Accepted — strategic architecture freeze; implementation separately authorized

```text
ADR_FIN_001=ACCEPTED
RUNTIME_IMPLEMENTATION_AUTHORIZED=NO
FINANCE_OWNS_EXECUTION_FACTS=NO
PAYMENT_OBLIGATION_IS_RECEIVABLE=NO
SHIPMENT_ID_UNIQUELY_IDENTIFIES_SETTLEMENT=NO
```

## Context

BINTRANS closes money only when physical execution, legal documents, and commercial settlement stay correctly owned. Finance-0.1B froze decimal money, commercial relationships, and tax fail-closed behavior. Finance-0.1D freezes accounting-document policy rules on top of that foundation and leaves its open legal questions unresolved. EDO ADRs froze document ownership, billing/EDO boundary, and Receivable versus PaymentObligation. Without a registry-level chain decision, NEXT work risks:

- treating shipment id as the sole settlement key
- letting billing invent legal XML truth
- letting EDO invent monetary totals
- collapsing forwarder customer-side and supply-side chains

## Decision

### 1. Ordered chain with distinct owners

```text
Execution facts (TMS / shipment)
  → legal / EDO artifacts (document-service)
  → commercial settlement (billing / finance)
  → billing register / closing documents (billing money owner)
  → payment obligation / payment rails (payment domain)
```

| Link | Owner | Must not |
| --- | --- | --- |
| Execution facts | shipment-service / TMS owners | Be rewritten by finance to “fix” money |
| Legal artifact / archive | document-service (EDO) | Become the monetary ledger |
| Settlement / commercial amounts | billing-register / finance owners | Silently default unknown tax to zero |
| PaymentObligation | payment domain | Be redefined as Receivable |

### 2. Settlement identity includes commercial relationship

```text
SETTLEMENT_IDENTITY_INCLUDES_COMMERCIAL_RELATIONSHIP=YES
SHIPMENT_ID_UNIQUELY_IDENTIFIES_SETTLEMENT=NO
MULTIPLE_SETTLEMENTS_PER_EXECUTION_ALLOWED=YES
ONE_EXECUTION_CAN_HAVE_MULTIPLE_COMMERCIAL_RELATIONSHIPS=YES
```

A forwarder move may have separate customer-side and supply-side settlements, billing chains, payment chains, and EDO chains. One register must not mix those chains.

### 3. Money and legal truth stay split

```text
MONETARY_TRUTH_OWNER=BILLING
LEGAL_XML_TRUTH_OWNER=EDO
```

Closing-document money is billing-owned. EDO owns legal artifact bytes and archive boundary per ADR-EDO-007 / related EDO freezes.

### 4. Exact money for new work

Authoritative `float64` money is forbidden for new finance work. Canonical handling is exact decimal in process, `NUMERIC` in PostgreSQL, decimal string on APIs, consistent with Finance-0.1B. Historical values are not silently recalculated on type migration.

### 5. Fail closed on tax and authority

Unknown tax blocks accounting close and tax-bearing documents. Client-authored base amounts, VAT rates, and register totals are denied. Public client tenant headers are not authority; gateway-established tenant context is.

### 6. Registry relationship

`ADR-FIN-*` is the strategic registry prefix for finance and settlement product decisions. Finance-0.1B ownership and money foundations, Finance-0.1D accounting-document rule freeze, and ADR-EDO-002 / ADR-EDO-005 remain binding detail. This ADR does not reopen those freezes, does not close the open legal questions left by Finance-0.1D, and does not authorize finance runtime.

## Consequences

- Controllers can sequence finance NEXT work without inventing a second ownership model.
- Portal and agent features that “create settlement from shipment” must still resolve commercial relationship and evidence.
- Factoring / Receivable remains a future rail distinct from PaymentObligation.

## Non-authorization

This ADR does not authorize migrations, Go/OpenAPI changes, AR/AP runtime, bank or 1C integration, staging finance enablement, or tax-rate legislative choices reserved for later research stages.

## References

- [BINTRANS_PRODUCT_ROADMAP_2026_2027_NEXT.md](../architecture/BINTRANS_PRODUCT_ROADMAP_2026_2027_NEXT.md)
- [FINANCE_0_1B_ARCHITECTURE.md](../architecture/finance-accounting/FINANCE_0_1B_ARCHITECTURE.md)
- [FINANCE_0_1B_COMMERCIAL_RELATIONSHIP.md](../architecture/finance-accounting/FINANCE_0_1B_COMMERCIAL_RELATIONSHIP.md)
- [FINANCE_0_1D_ACCOUNTING_DOCUMENT_RULES.md](../architecture/finance-accounting/FINANCE_0_1D_ACCOUNTING_DOCUMENT_RULES.md)
- [ADR-EDO-002](ADR-EDO-002-billing-edo-boundary.md)
- [ADR-EDO-005](ADR-EDO-005-receivable-vs-payment-obligation.md)
