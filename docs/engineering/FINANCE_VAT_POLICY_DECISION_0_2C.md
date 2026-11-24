# FINANCE-VAT-POLICY-DECISION-0.2C

Decision package for the SNAPSHOT_V1 money basis and VAT. No option is selected.

`COMMERCIAL_PILOT_READY=NO`

`FINAL_VERDICT=DECISION_PACKAGE_READY/BLOCKED`

This document does not change calculation, billing handlers, API contracts, or migrations. It does not edit PR #341 or PR #353.

| Related record | State |
| --- | --- |
| PR #341 | Draft, open, head `5444800f2b9e1df8a2bdedf33ca28c15eebeedb8` |
| PR #353 | Draft, open, head `0918fee4e833cb71eb41dabee166b7b92ab83ff4` |
| This branch base | `origin/main` `3f306fc407aeda4bdc652d825954292f4a54fad6` |

## 1. Proven chain

One synthetic shipment, one currency, no extras, penalties, withholdings, or disputes. The isolated run in PR #341 passed with exit code 0. Billing code on current `main` is unchanged since that proof.

| Object | ID | Stored text |
| --- | --- | --- |
| Snapshot `total_amount` | `b1c0ffc6-56b0-48fe-bea0-e6ca2c501b2c` | `1234.56` RUB |
| Settlement `base_freight_amount` | `309e9444-e930-42aa-b904-b3f6819e1016` | `1234.56` RUB |
| Register item `base_amount` | `b518f94e-527a-44f0-bd06-55721d5a8afa` | `1234.56` |
| Register `total_without_vat` | `e97bb29b-345b-4b7a-9c30-10e2f6670c10` | `1234.56` RUB |
| VAT invoice `amount_without_vat` | `20400da8-f08f-4bb4-a76f-4591d00aaf7e` | `1234.56` |
| Act `total_amount` | `4aa6de4e-664e-4164-be3c-edc1bd398773` | `1234.56` RUB |

`pricing_model_version=SNAPSHOT_V1`. `pricing_source=RFQ_AWARD`. The award row stayed `100000` and was not the price source.

`SNAPSHOT_AMOUNT_BASIS=UNKNOWN`

The act has only `total_amount`. The current insert copies register `total_with_vat`. In this run that gross text is also `1234.56` because `vat_rate` is null and `vat_amount` is `0.00`. That match is an observation under a missing rate. It does not establish a gross basis.

## 2. Null `vat_rate`

A null `vat_rate` records that no rate was supplied. The snapshot table has no VAT column. The SNAPSHOT_V1 loader does not set `VATRate`. `CalculateSettlementTotalsDecimal` and `CalculateItemAmounts` then store VAT `0`.

`OBSERVED_RUNTIME_VAT_ZERO=YES`

`VAT_POLICY_CONFIRMED=NO`

`VAT_RATE_SOURCE=ABSENT`

This null is not option Z, not a zero rate, and not a tax exemption. An exemption exists only after the owner records a legal basis and the system stores that decision in an explicit field. Until then `COMMERCIAL_VAT_GATE=BLOCKED`.

## 3. Unselected basis options

The owner picks one of N, G, or Z, or leaves U in force. This package does not pick. Rate `R` below is a placeholder. No percentage is proposed.

### N — snapshot amount is net

`total_amount` is the amount before VAT.

| Store | Consequence after a later approved rate R |
| --- | --- |
| Settlement `base_freight_amount` | Stays `1234.56` |
| Settlement `total_without_vat` | Stays `1234.56` |
| Settlement `vat_amount` | Computed from `1234.56` and R, then rounded by the approved rule |
| Settlement `total_with_vat` | Net plus that VAT |
| Register line and register net | Stay `1234.56` when extras and penalties are zero |
| Register gross | Follows the settlement gross |
| Act `total_amount` | Becomes the gross, because the act copies `total_with_vat` |
| VAT invoice | Net `1234.56`, rate R, VAT and gross from the same calculation |

Until that decision, the live path stores gross equal to `1234.56` and VAT `0.00`.

### G — snapshot amount is gross

`total_amount` already includes VAT. A later implementation has to extract net and VAT from `1234.56`. The current loader does not extract. If a rate pointer were set on today's code, VAT would be added on top of `1234.56`.

| Store | Consequence only after an approved extraction rule |
| --- | --- |
| Settlement `base_freight_amount` | Needs an explicit rule: keep the gross `1234.56`, or replace the base with the extracted net |
| Settlement net, VAT, gross | Must reconstruct to gross `1234.56` after rounding |
| Register line | Same rule as the settlement base |
| Act `total_amount` | Stays `1234.56` if the act continues to copy the gross |
| VAT invoice | Net becomes the extracted amount. Gross stays `1234.56`. Rate is the approved R |

The extraction formula is itself an owner decision. Two different formulas can give different kopecks. This package does not name one.

### Z — VAT does not apply

The operation has a documented reason that VAT is not charged.

| Store | Consequence after that document is accepted |
| --- | --- |
| Settlement base, net, and gross | All `1234.56` |
| VAT amount | `0.00` as an explicit result |
| `vat_rate` | An explicit marker defined by the owner. A null column does not satisfy Z |
| Act `total_amount` | `1234.56` |
| VAT invoice | Net and gross `1234.56`, VAT `0.00`, and the same explicit marker |

### U — leave unresolved

Principal identity of `1234.56` stays assessed. No VAT math change. Commercial pilot stays closed.

## 4. Questions

### Product owner

1. Which option applies to SNAPSHOT_V1 freight: N, G, or Z?
2. Is that choice the same for RFQ award, spot bid, contract rate, and manual spot?
3. May the commercial pilot show closing documents while the basis stays U?
4. The act stores one amount, the register gross. Is that the amount the pilot may display?
5. Who may later approve a change to VAT storage: product owner, finance owner, or both?

### Accounting and tax specialist

1. What tax category applies to this freight operation?
2. What rate applies, or what document states that VAT is not charged?
3. Is the snapshot `total_amount` the tax base, the VAT-inclusive price, or neither?
4. Which extraction formula applies if the answer is G?
5. Which rounding rule applies to the tax and to the document total?
6. May a VAT invoice be stored while `vat_rate` is null?
7. Is a stored VAT of `0.00` with a null rate acceptable on a commercial document?

## 5. Rounding choices

Current comment in `money_policy.go`: half-away-from-zero to 2 decimal places, then the register sums the rounded lines. One ISO currency per register. Storage is `NUMERIC(18,2)` for amounts and `NUMERIC(5,2)` for rates.

These choices stay open:

| ID | Rule | Relation to today's code |
| --- | --- | --- |
| RD-1 | Half away from zero at 2 decimal places, on decimal values, line then sum | Matches the written policy. Removes the `float64` step before rounding |
| RD-2 | Keep `round2` on `float64` for register lines | Matches the register helper. Keeps the `1.255` → `1.25` behavior |
| RD-3 | Half to even at 2 decimal places | Differs from the written policy and from `decimal.Round(2)` |

No rounding rule is selected here. A tax specialist names the rule. An engineer then maps it to RD-1, RD-2, or RD-3.

## 6. Float64 and decimal

Two calculations exist.

| Path | Type | Where |
| --- | --- | --- |
| Snapshot load and settlement insert | `decimal`, `total_amount::text`, then `StringFixed(2)` | `FreightSettlementRepository` for `SNAPSHOT_V1` |
| Settlement totals when a rate exists | `decimal`, with the rate taken through `decimal.NewFromFloat` | `CalculateSettlementTotalsDecimal` |
| Register line | `float64` `base + extra - penalties`, then `round2` | `CalculateItemAmounts` |
| Domain field `BaseFreightAmount` | `float64` | `FreightSettlement` |

The proven `1234.56` survives the float path. A three-decimal input does not:

| Input text | `decimal.Round(2)` | `round2` after `Float64()` |
| --- | --- | --- |
| `1.255` | `1.26` | `1.25` |

`NUMERIC(18,2)` cannot store `1.255`. The divergence matters when a non-boundary value reaches `CalculateItemAmounts` or `CalculateSettlementTotals` before it is written.

## 7. Tests to keep and tests to add later

Already in PR #341, and still the regression bar:

| ID | Assertion |
| --- | --- |
| T-KEEP-1 | Texts `1234.56`, `0.01`, `0.10`, `10.01`, `8.10`, `0.29`, `100000.10` survive `round2` |
| T-KEEP-2 | Text `1.255` is `1.26` in decimal and `1.25` in `round2`. The test stays in place until a separate remediation |
| T-KEEP-3 | Nil rate on `1234.56` stores net `1234.56`, VAT `0`, gross `1234.56` |

Add only after the owner decision, with expected texts taken from that decision:

| ID | Case |
| --- | --- |
| T-NEXT-1 | Boundaries `0.00`, `0.01`, `0.99`, and `9999999999999999.99` (`NUMERIC(18,2)` upper bound), one currency, exact decimal text |
| T-NEXT-2 | `1.255` through the approved rounding path, expected text named by RD-1, RD-2, or RD-3 |
| T-NEXT-3 | One shipment at the approved basis. Snapshot `1234.56` compared with settlement, register, act, and VAT invoice using the approved rate or the explicit Z marker |
| T-NEXT-4 | A null `vat_rate` remains an absent rate. The test must fail if a null is labeled as option Z |

This PR adds none of those tests and does not change the existing ones.

## 8. Changes allowed only after the tax decision

Blocked until N, G, or Z is approved, together with a rate or a documented non-application and a rounding rule:

- writing a VAT rate into the SNAPSHOT_V1 settlement path
- adding VAT on top of `total_amount` or extracting VAT from it
- treating null `vat_rate` as exemption or as a zero rate
- a new column for basis, rate, or exemption
- a migration or API change for those fields
- changing `round2`, `CalculateItemAmounts`, or `CalculateSettlementTotalsDecimal`
- changing the act so that it stores a net amount
- declaring `COMMERCIAL_PILOT_READY=YES`

Allowed while the decision is open: documentation, the existing proof in PR #341, and the gap record in PR #353.

`PAYMENT_EXECUTED=NO`

`BANK_CONNECTED=NO`

`EDO_OPERATOR_CONNECTED=NO`

`MIGRATIONS_CREATED=NO`

`PRODUCT_BEHAVIOR_CHANGED=NO`

`STAGING_CHANGED=NO`

`PRODUCTION_CHANGED=NO`
