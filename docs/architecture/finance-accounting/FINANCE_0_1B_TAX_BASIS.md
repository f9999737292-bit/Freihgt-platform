# FINANCE-0.1B tax basis

```text
REGISTER_VAT_RATE_IS_AUTHORITATIVE=NO
CLIENT_VAT_RATE_IS_AUTHORITATIVE=NO
BILLING_MAY_REPLACE_MISSING_SETTLEMENT_TAX_RATE=NO
UNKNOWN_TAX_SILENTLY_DEFAULTS_TO_ZERO=NO
UNKNOWN_TAX_USES_REGISTER_FALLBACK=NO
UNKNOWN_TAX_ACCOUNTING_CLOSE_BEHAVIOR=BLOCK_ACCOUNTING_CLOSE_AND_TAX_BEARING_DOCUMENTS
TAX_ROUNDING_POLICY_FROZEN=NO
TAX_ROUNDING_POLICY_DEPENDS_ON_FINANCE_0_1C=YES
LEGAL_COMPLIANCE_RESEARCH_REQUIRED=YES
```

FINANCE-0.1A showed a nil settlement VAT rate, a zero VAT amount, and a later register inclusion that can apply the client-supplied register VAT rate. That path is forbidden in the target architecture. This document does not choose VAT rates, exemptions, or statutory rounding.

## TaxDecision

Tax treatment is a server-owned snapshot. A browser value, a register field, and the latest live policy are not the treatment of a historical settlement.

A TaxDecision records:

| Provenance | Meaning |
| --- | --- |
| Policy source | Server-owned policy identity and version. Not a client rate. |
| Effective context | Tenant, relationship, parties, currency, execution reference, and the time context captured with the decision. |
| Treatment | An opaque treatment code. FINANCE-0.1C names legal treatments. `UNRESOLVED` is the only treatment this stage requires. |
| Monetary basis | The exact-decimal amount the treatment applies to. |
| Resulting tax amount | Present only after a resolved server decision. Absent while unresolved. |

`REGISTER_VAT_RATE_IS_AUTHORITATIVE=NO`. `CLIENT_VAT_RATE_IS_AUTHORITATIVE=NO`. `BILLING_MAY_REPLACE_MISSING_SETTLEMENT_TAX_RATE=NO`.

## Unknown treatment

When a commercial amount exists and no authoritative tax treatment exists, the system must not use 0%, the register VAT rate, a browser value, or whatever the current policy is today.

```text
UNKNOWN_TAX_SILENTLY_DEFAULTS_TO_ZERO=NO
UNKNOWN_TAX_USES_REGISTER_FALLBACK=NO
UNKNOWN_TAX_ACCOUNTING_CLOSE_BEHAVIOR=BLOCK_ACCOUNTING_CLOSE_AND_TAX_BEARING_DOCUMENTS
```

The target gate:

- A commercial settlement of the agreed principal may exist.
- Tax amount on that settlement stays absent. It is not stored as zero to mean "no tax".
- Register approval that publishes a tax-inclusive total is blocked.
- VAT invoice, UPD, and any closing document that states a VAT rate or VAT amount are blocked.
- Register close is blocked.
- A resolved TaxDecision is required before those commands succeed.

An act or invoice that only repeats a tax-inclusive register total is in that blocked set while treatment is unresolved, because those documents would publish a tax-inclusive figure. FINANCE-0.1C may later split commercial documents from tax documents. This stage does not.

## Immutability

Once a TaxDecision is resolved and a tax-bearing document is issued, the rate, basis, and tax amount of that document are immutable. A later policy change does not rewrite them. Correction is a future explicit successor, researched in FINANCE-0.1C, not an in-place edit.

## Closing documents and EDO

```text
CLOSING_DOCUMENT_MONEY_OWNER=BILLING
EDO_LEGAL_ARTIFACT_OWNER=AGENT_B_DOCUMENT_SERVICE
MONETARY_TRUTH_OWNER=BILLING
LEGAL_XML_TRUTH_OWNER=EDO
```

Invoice, act, VAT invoice, and UPD amounts are derived from frozen billing facts. The portal does not supply those totals. EDO does not calculate them. Legal XML, PDF, signature, and operator state stay with Agent B.
