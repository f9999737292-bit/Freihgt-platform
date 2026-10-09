# FINANCE-0.1B migration plan

```text
HISTORICAL_FINANCIAL_RECALCULATION_ON_TYPE_MIGRATION=NO
HISTORICAL_FORWARDER_RELATIONSHIPS_MAY_BE_INFERRED_WITHOUT_EVIDENCE=NO
SCHEMA_IN_THIS_STAGE=NO
```

Current rows keep `buyer_company_id`, `carrier_company_id`, and `shipment_id`, with one settlement per shipment. Those values stay as recorded. This stage does not migrate them.

## Legacy direct mapping

An existing settlement may later be linked to a `DIRECT` relationship only when every one of these is already stored:

- one buyer company and one carrier company on that settlement
- the buyer is taken as the customer party and the carrier as the provider party because those columns say so, not because of company type
- the execution has no second commercial amount in finance data
- the principal is the stored settlement amount, which itself came from the snapshot or the award row

Company type `FORWARDER` on either party blocks that mapping. The relationship side stays `UNKNOWN`. Customer-side versus supply-side is not inferred.

```text
HISTORICAL_FORWARDER_RELATIONSHIPS_MAY_BE_INFERRED_WITHOUT_EVIDENCE=NO
```

These stay unknown unless a stored fact already says them: relationship side when a forwarder is present, tax treatment, and which commercial role a forwarder held. They are not defaulted.

## When the shipment unique constraint can be retired

The unique `(tenant_id, shipment_id)` settlement constraint is retired only after all of the following exist in an implementation stage:

- uniqueness of one non-cancelled settlement lineage per relationship and execution reference
- every historical row classified as proven `DIRECT` or left `UNKNOWN`
- the shipment-scoped create API no longer inserts a shipment-unique settlement except through the compatibility rule below

## Old API coexistence

The current shipment-scoped settlement create remains a temporary adapter. It may create or return the settlement only for a proven `DIRECT` mapping. If the mapping is not proven, the adapter fails closed. It must not create a second relationship and it must not label a forwarder as the buyer.

Register item create that accepts client amounts is not a compatibility path for new work. It is denied by the target policy. Removal of the route is an implementation change, not this stage.

## Historical immutability

These stay immutable once frozen:

- commercial price snapshot
- final settlement principal and stored totals
- approved billing basis
- issued accounting-document amounts

Type migration copies stored `NUMERIC` text into exact decimal. It does not recompute those facts. Corrections, when legal research allows them, are new records.

## 0.1A gap disposition

`FINANCE_0_1A_GAPS.md` contains 19 gap headings. Its header records `TOTAL_GAPS=19` and `BLOCKING_GAPS=12`. This plan classifies those 19 headings. Individual 0.1A findings are unchanged.

`ARCHITECTURE_RESOLVED` means the target rule is frozen here. `PRODUCT_DEFECT_CLOSED=NO` on every row. The current runtime is unchanged.

| Gap | Disposition | Product defect |
| --- | --- | --- |
| FIN-SET-001 | ARCHITECTURE_RESOLVED | NO. Implement in FINANCE-0.3A. |
| FIN-SET-002 | ARCHITECTURE_RESOLVED | NO. Implement in FINANCE-0.2A. |
| FIN-SET-003 | ARCHITECTURE_RESOLVED | NO. Fail-closed gate in FINANCE-0.2B. Legal treatments stay FIN-TAX-001. |
| FIN-BIL-001 | ARCHITECTURE_RESOLVED | NO. Deny client line totals in FINANCE-0.2A. |
| FIN-BIL-002 | ARCHITECTURE_RESOLVED | NO. Remove register VAT fallback in FINANCE-0.2B. |
| FIN-BIL-003 | ARCHITECTURE_RESOLVED | NO. Stop hard-deleting billed lines in the immutability slice of FINANCE-0.2A. |
| FIN-DOC-001 | DEFERRED_TO_LATER_STAGE | NO. FINANCE-0.3B with Agent B. |
| FIN-DOC-002 | DEFERRED_TO_LATER_STAGE | NO. Depends on FINANCE-0.3B. |
| FIN-DOC-003 | DEFERRED_TO_LATER_STAGE | NO. Agent B. |
| FIN-DOC-004 | DEFERRED_TO_0_1C | NO. Field set needs legal research. |
| FIN-DOC-005 | DEFERRED_TO_LATER_STAGE | NO. Agent A gateway routes. |
| FIN-TAX-001 | DEFERRED_TO_0_1C | NO. |
| FIN-PAY-001 | DEFERRED_TO_LATER_STAGE | NO. Receivable is not designed as a table here. |
| FIN-PAY-002 | DEFERRED_TO_LATER_STAGE | NO. Overdue follows AR/AP. |
| FIN-BANK-001 | DEFERRED_TO_LATER_STAGE | NO. |
| FIN-1C-001 | DEFERRED_TO_LATER_STAGE | NO. |
| FIN-FWD-001 | ARCHITECTURE_RESOLVED | NO. Implement in FINANCE-0.3A. |
| FIN-FWD-002 | DEFERRED_TO_LATER_STAGE | NO. No margin formula. |
| FIN-SEC-001 | DEFERRED_TO_LATER_STAGE | NO. Agent A. |

```text
GAPS_ARCHITECTURE_RESOLVED=7
GAPS_UNDISPOSITIONED=0
GAPS_DEFERRED_TO_0_1C=2
GAPS_DEFERRED_LATER=10
ARCHITECTURE_RESOLVED_RUNTIME_FIXES_PENDING=7
PRODUCT_DEFECTS_CLOSED_IN_0_1B=0
```

`ARCHITECTURE_RESOLVED` means the target policy or design is frozen. It does not mean the current product or runtime behavior is remediated. `GAPS_UNDISPOSITIONED=0` means every heading has a disposition. `ARCHITECTURE_RESOLVED_RUNTIME_FIXES_PENDING=7` and `PRODUCT_DEFECTS_CLOSED_IN_0_1B=0` record that those seven rows still need an implementation slice. The counts add as 7 + 0 + 2 + 10 = 19.

## Implementation sequence

Dependencies force this order. Nothing in the list is authorized by this stage.

1. FINANCE-0.1C. Russian legal and accounting research for tax treatments, document fields, and correction shape. Docs only until a later stage is authorized.
2. FINANCE-0.2A. Exact-decimal settlement and billing handling, denial of client-authored totals, and an end to hard-deleting billed lines. This wave must not recalculate historical amounts. It can be specified beside 0.1C because it does not choose tax law.
3. FINANCE-0.2B. Server-owned TaxDecision and the unknown-tax gate. It consumes 0.1C treatment codes. It must not default unknown tax to zero or to a register rate.
4. FINANCE-0.3A. Commercial relationship and forwarder dual-chain persistence, including legacy `DIRECT` versus `UNKNOWN` mapping. New rows use the decimal and tax gates from 0.2A and 0.2B.
5. FINANCE-0.3B. Billing `document_id` contract with Agent B, one EDO chain per relationship. This follows FINANCE-0.3A so the two forwarder chains are not bound to one package. The stage id is 0.3B so the sequence stays monotonic.

Later, and not sequenced in detail here: receivable and payable aggregates, overdue, audited manual-adjustment implementation, bank statement import, and 1C finance exchange. Margin recognition stays undefined.

```text
RECOMMENDED_IMPLEMENTATION_SEQUENCE=FINANCE-0.1C,FINANCE-0.2A,FINANCE-0.2B,FINANCE-0.3A,FINANCE-0.3B
```

## Cross-agent work

| Agent | Later work | Not this stage |
| --- | --- | --- |
| A | Keep public client tenant headers non-authoritative. Keep finance service ports off the public edge. Add closing-document gateway routes only after G publishes the contract. | FIN-SEC-001 and FIN-DOC-005 |
| B | Legal artifact, signature, and operator state. Accept a billing `document_id` per relationship. Do not calculate money or VAT. | FIN-DOC-003 and FINANCE-0.3B |
| C | Keep shipment, transport order, and POD as execution facts. If an execution is not one shipment, publish that execution id before FINANCE-0.3A stores it. Do not own settlement identity. | Execution-id confirmation |
| F | Present server facts after the contracts exist. Do not sum register totals into margin or revenue truth. | Portal work |
| E | Read published financial facts. Do not define them. | Analytics semantics |

Agent G owns the financial rules in this folder. G does not implement them in FINANCE-0.1B.
