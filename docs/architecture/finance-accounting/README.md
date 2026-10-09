# Finance accounting discovery

FINANCE-0.1A is a docs-only inventory of the BINTRANS financial chain on `origin/main` `2e408a103c3a552fc85fda06325162bf595b654a`.

| Document | Contents |
| --- | --- |
| [FINANCE_0_1A_CURRENT_STATE.md](FINANCE_0_1A_CURRENT_STATE.md) | Capability classification, payment trace, money model, VAT behavior, security, UI, readiness |
| [FINANCE_0_1A_ACCOUNTING_DOCUMENT_MAP.md](FINANCE_0_1A_ACCOUNTING_DOCUMENT_MAP.md) | Invoice, act, VAT invoice, and UPD field map, plus Billing to EDO binding |
| [FINANCE_0_1A_FORWARDER_FINANCE.md](FINANCE_0_1A_FORWARDER_FINANCE.md) | Customer-side and supply-side chain, margin, receivable versus obligation |
| [FINANCE_0_1A_GAPS.md](FINANCE_0_1A_GAPS.md) | Gap register and recommended next stages |
| [FINANCE_0_1B_ARCHITECTURE.md](FINANCE_0_1B_ARCHITECTURE.md) | Core financial architecture freeze |
| [FINANCE_0_1B_MONEY_POLICY.md](FINANCE_0_1B_MONEY_POLICY.md) | Exact decimal money policy and float64 migration rule |
| [FINANCE_0_1B_COMMERCIAL_RELATIONSHIP.md](FINANCE_0_1B_COMMERCIAL_RELATIONSHIP.md) | Commercial relationship, settlement identity, forwarder chains |
| [FINANCE_0_1B_TAX_BASIS.md](FINANCE_0_1B_TAX_BASIS.md) | Server-owned tax basis and unknown-tax gate |
| [FINANCE_0_1B_MIGRATION_PLAN.md](FINANCE_0_1B_MIGRATION_PLAN.md) | Additive legacy mapping and later implementation slices |

```text
LEGAL_COMPLIANCE_RESEARCH_REQUIRED=YES
FINANCE_CORE_ARCHITECTURE_FROZEN=YES
PRODUCT_CODE_CHANGED=NO
```

FINANCE-0.1A evidence files stay the inventory of what the code does. FINANCE-0.1B freezes the target rules. This folder does not define VAT rates, statutory XML, correction procedure, or a margin recognition policy.
