# FINANCE-0.1B architecture freeze

```text
STAGE=FINANCE-0.1B
KIND=ARCHITECTURE
BASE_SHA=c86108e7159ff8322a06fb38faafa66cd00788aa
FINANCE_CORE_ARCHITECTURE_FROZEN=YES
PRODUCT_CODE_CHANGED=NO
LEGAL_COMPLIANCE_RESEARCH_REQUIRED=YES
```

FINANCE-0.1A is the evidence of the current runtime. This document freezes the kernel later finance stages must follow. It does not change Go code, schema, OpenAPI, frontend, or staging.

## Frozen decisions

```text
FINANCE_CORE_ARCHITECTURE_FROZEN=YES
AUTHORITATIVE_FLOAT64_ALLOWED=NO
CANONICAL_MONEY_TYPE=EXACT_DECIMAL
CANONICAL_DB_MONEY_TYPE=NUMERIC
CANONICAL_API_MONEY_TYPE=DECIMAL_STRING
HISTORICAL_FINANCIAL_RECALCULATION_ON_TYPE_MIGRATION=NO
ONE_EXECUTION_CAN_HAVE_MULTIPLE_COMMERCIAL_RELATIONSHIPS=YES
FINANCIAL_PARTY_ROLE_IS_TRANSACTION_SCOPED=YES
FORWARDER_COMPANY_TYPE_DOES_NOT_FIX_FINANCIAL_DIRECTION=YES
SETTLEMENT_IDENTITY_INCLUDES_COMMERCIAL_RELATIONSHIP=YES
SHIPMENT_ID_UNIQUELY_IDENTIFIES_SETTLEMENT=NO
MULTIPLE_SETTLEMENTS_PER_EXECUTION_ALLOWED=YES
RATE_SNAPSHOT_RELATIONSHIP_MODEL=RELATIONSHIP_SCOPED_COMMERCIAL_PRICE_SNAPSHOT
FORWARDER_TWO_SIDED_FINANCE_REQUIRED=YES
CUSTOMER_SETTLEMENT_SEPARATE_FROM_SUPPLY_SETTLEMENT=YES
CUSTOMER_BILLING_CHAIN_SEPARATE_FROM_SUPPLY_BILLING_CHAIN=YES
CUSTOMER_PAYMENT_CHAIN_SEPARATE_FROM_SUPPLY_PAYMENT_CHAIN=YES
CUSTOMER_EDO_CHAIN_SEPARATE_FROM_SUPPLY_EDO_CHAIN=YES
FREIGHT_BASE_HISTORICAL_REPRICING_ALLOWED=NO
REGISTER_VAT_RATE_IS_AUTHORITATIVE=NO
CLIENT_VAT_RATE_IS_AUTHORITATIVE=NO
BILLING_MAY_REPLACE_MISSING_SETTLEMENT_TAX_RATE=NO
UNKNOWN_TAX_SILENTLY_DEFAULTS_TO_ZERO=NO
UNKNOWN_TAX_USES_REGISTER_FALLBACK=NO
UNKNOWN_TAX_ACCOUNTING_CLOSE_BEHAVIOR=BLOCK_ACCOUNTING_CLOSE_AND_TAX_BEARING_DOCUMENTS
CLIENT_AUTHORED_BASE_AMOUNT=DENY
CLIENT_AUTHORED_EXTRA_CHARGES_TOTAL=DENY
CLIENT_AUTHORED_PENALTY_TOTAL=DENY
CLIENT_AUTHORED_VAT_RATE=DENY
CLIENT_AUTHORED_REGISTER_TOTAL=DENY
GENERIC_CLIENT_FINANCIAL_OVERRIDE=DENY
AUDITED_MANUAL_ADJUSTMENT_CONCEPT=YES
ONE_REGISTER_MAY_MIX_FORWARDER_CUSTOMER_AND_SUPPLY_CHAINS=NO
CLOSING_DOCUMENT_MONEY_OWNER=BILLING
EDO_LEGAL_ARTIFACT_OWNER=AGENT_B_DOCUMENT_SERVICE
MONETARY_TRUTH_OWNER=BILLING
LEGAL_XML_TRUTH_OWNER=EDO
PAYMENT_OBLIGATION_IS_RECEIVABLE=NO
FINANCE_OWNS_EXECUTION_FACTS=NO
PUBLIC_CLIENT_TENANT_HEADER_AUTHORITY=NO
DOWNSTREAM_FINANCE_SERVICE_TRUSTS_GATEWAY_TENANT_HEADER=YES
DIRECT_FINANCE_SERVICE_EXPOSURE_VERIFIED=NO
HISTORICAL_FORWARDER_RELATIONSHIPS_MAY_BE_INFERRED_WITHOUT_EVIDENCE=NO
TAX_ROUNDING_POLICY_FROZEN=NO
TAX_ROUNDING_POLICY_DEPENDS_ON_FINANCE_0_1C=YES
```

## What this freeze resolves

Exact money for new work is decimal in process, `NUMERIC` in PostgreSQL, and a decimal string on the API. Historical `NUMERIC` values are not recomputed when handling moves off `float64`.

The agreed freight principal stays on an immutable snapshot scoped to one commercial relationship. A later rate-card or RFx change does not reprice it.

Settlement identity includes that relationship. Shipment id does not uniquely identify a settlement. One execution may have one relationship or more than one. Direct shipper-to-carrier needs no forwarder. Forwarder customer-side and supply-side chains stay separate through settlement, billing, payment, and EDO.

Missing tax treatment does not become zero and does not become the register VAT rate. Accounting close and tax-bearing documents stay blocked until a server-owned TaxDecision exists. Russian tax rates and tax rounding are not chosen here.

Client-authored base amounts, extra totals, penalty totals, VAT rates, and register totals are denied. An audited manual adjustment is a separate future aggregate. It is not a generic override.

## Where the rules live

| Topic | Document |
| --- | --- |
| Decimal type, API string, scale, migration without recalc | [FINANCE_0_1B_MONEY_POLICY.md](FINANCE_0_1B_MONEY_POLICY.md) |
| Relationship, parties, settlement identity, snapshot, register, payment | [FINANCE_0_1B_COMMERCIAL_RELATIONSHIP.md](FINANCE_0_1B_COMMERCIAL_RELATIONSHIP.md) |
| TaxDecision and the unknown-tax gate | [FINANCE_0_1B_TAX_BASIS.md](FINANCE_0_1B_TAX_BASIS.md) |
| Legacy mapping, gap disposition, later slices | [FINANCE_0_1B_MIGRATION_PLAN.md](FINANCE_0_1B_MIGRATION_PLAN.md) |

## Non-goals

No schema, migration, Go change, frontend change, bank integration, 1C integration, accounts receivable or payable implementation, EDO provider, qualified signature, Russian tax-rate decision, margin recognition formula, or staging deploy.

```text
RECOMMENDED_IMPLEMENTATION_SEQUENCE=FINANCE-0.1C,FINANCE-0.2A,FINANCE-0.2B,FINANCE-0.3A,FINANCE-0.2C
```
