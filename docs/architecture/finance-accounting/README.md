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
| [FINANCE_0_1C_RUSSIAN_ACCOUNTING_DOCUMENT_RESEARCH.md](FINANCE_0_1C_RUSSIAN_ACCOUNTING_DOCUMENT_RESEARCH.md) | Primary-source research for Russian accounting and tax documents, including the 2026 source completion |
| [FINANCE_0_1D_ACCOUNTING_DOCUMENT_RULES.md](FINANCE_0_1D_ACCOUNTING_DOCUMENT_RULES.md) | Decision contract for which accounting document is required, and which VAT class the facts support |

```text
LEGAL_COMPLIANCE_RESEARCH_REQUIRED=YES
FINANCE_CORE_ARCHITECTURE_FROZEN=YES
PRODUCT_CODE_CHANGED=NO
```

FINANCE-0.1A evidence files stay the inventory of what the code does. FINANCE-0.1B freezes the target financial kernel. FINANCE-0.1C records primary-source document research. FINANCE-0.1C-R1 adds the 2026 statutory catalog and the freight treatment conditions. FINANCE-0.1D freezes the document decision contract. It does not set a global BINTRANS VAT rate, does not treat international carriage as automatically zero-rated, and does not define statutory XML. Implementation is not authorized.
