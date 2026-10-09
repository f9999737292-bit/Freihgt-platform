# FINANCE-0.1B money policy

```text
STAGE=FINANCE-0.1B
AUTHORITATIVE_FLOAT64_ALLOWED=NO
CANONICAL_MONEY_TYPE=EXACT_DECIMAL
CANONICAL_DB_MONEY_TYPE=NUMERIC
CANONICAL_API_MONEY_TYPE=DECIMAL_STRING
HISTORICAL_FINANCIAL_RECALCULATION_ON_TYPE_MIGRATION=NO
TAX_ROUNDING_POLICY_FROZEN=NO
TAX_ROUNDING_POLICY_DEPENDS_ON_FINANCE_0_1C=YES
MONEY_SCALE=2
AMOUNT_PRECISION=NUMERIC(18,2)
```

This policy applies to new finance work and to later migrations of billing and settlement money handling. It does not choose a Russian VAT rounding statute.

## Canonical representation

Authoritative financial calculations use exact decimal arithmetic. In new Go finance code that type is `shopspring/decimal`, the library already used by `payment-service` and `contract-rate-service`. `float32` and `float64` are not authoritative money types.

Database currency amounts are PostgreSQL `NUMERIC`. The frozen currency amount shape is `NUMERIC(18,2)`, matching current billing, settlement, payment, and freight-cost amount columns. Scale 2 is the generic currency minor-unit scale already stored by those tables. It is not a tax-law rounding rule.

API money is a decimal string. A JSON number is not the canonical finance representation. JSON numbers are binary floating-point values in common clients, including the browser. New finance request and response amounts are strings such as `"1250.50"`. A missing amount is omitted or an explicit unavailable state. It is not `"0.00"` and it is not JSON `0` unless a server-owned decision produced a zero amount.

```text
CANONICAL_MONEY_TYPE=EXACT_DECIMAL
CANONICAL_DB_MONEY_TYPE=NUMERIC
CANONICAL_API_MONEY_TYPE=DECIMAL_STRING
AUTHORITATIVE_FLOAT64_ALLOWED=NO
```

## Generic arithmetic

```text
MONEY_SCALE=2
AMOUNT_PRECISION=NUMERIC(18,2)
ROUNDING_BOUNDARIES=PERSISTENCE_ONLY_HALF_AWAY_FROM_ZERO_SCALE_2
CURRENCY_CODE_POLICY=ISO_4217_ONE_CURRENCY_PER_RELATIONSHIP_FAIL_CLOSED
```

Exact decimal arithmetic keeps full exactness until a currency amount is persisted or returned as a currency total. At that boundary the amount is rounded half away from zero to scale 2. Two services must not round the same amount with different algorithms. Mid-formula `float64` rounding is forbidden for new work.

Currency is an ISO 4217 alphabetic code of three characters. One commercial relationship, one billing register, one payment obligation, and one payment allocation each have one currency. A currency mismatch fails closed. This freeze does not add foreign-exchange conversion.

`TAX_ROUNDING_POLICY_FROZEN=NO`. Whether a tax amount is rounded per line, per rate, or per document is a FINANCE-0.1C question. Until that research is accepted, new code must not invent a second tax-rounding rule. Generic scale-2 persistence still applies to a currency amount that a server-owned decision has already produced.

## What must not be recalculated

```text
HISTORICAL_FINANCIAL_RECALCULATION_ON_TYPE_MIGRATION=NO
```

Existing `NUMERIC` values are historical evidence. A later change from `float64` handling to exact decimal must read the stored decimal text and keep that value. It must not reprice from a current contract rate, re-apply a register VAT rate, or recompute an issued total because the in-memory type changed.

## Migration waves

Each wave converts handling. It does not rewrite history.

| Wave | Aggregate | Rule |
| --- | --- | --- |
| FINANCE-0.2A | Settlement domain | Read and write principal, accessorials, and stored totals as exact decimal. Keep stored `NUMERIC` values. |
| FINANCE-0.2A | Billing register and register items | Stop scanning sums into `float64`. Stop `CalculateItemAmounts` as an authoritative calculator. |
| FINANCE-0.2A | Invoice, act, VAT invoice, UPD | Copy server decimal billing facts. Do not parse those amounts through `float64`. |

Payment and contract-rate already use exact decimal. They are not part of the float64 retirement wave. Freight-cost entries are already `NUMERIC` and append-only. They stay historical evidence.

The current authoritative `float64` paths remain those named in FINANCE-0.1A. This policy does not change that code.
