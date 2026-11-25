# BINTRANS Smart Tender — Money Policy (Spec v0.9)

**Decision D5:** DECIMAL_FOR_NEW_TENDER_PATHS  
**Preferred library:** `github.com/shopspring/decimal`  
**Main:** `892361db87985f35132aef86550a025909af648a`

---

## Legacy track (frozen in W0)

| Layer | Type | Paths |
|---|---|---|
| DB | NUMERIC(18,2) | `000038` |
| Go award/scoring/conversion | float64 | `rfx_evaluation.go`, `rfx_award.go`, `award_conversion.go` |

No behavioral change to legacy Award money in W0.

---

## New allocation track

| Concern | Contract |
|---|---|
| In-memory | `shopspring/decimal` end-to-end in allocation/sim/budget/guarantee money |
| Persistence | `NUMERIC(18,2)` for money; rate scale Controller-confirm (default 18,2 unless unit needs 18,6) |
| Currency | ISO 4217 on every snapshot; no implicit FX |
| Tax basis | NET / GROSS / UNKNOWN — never mix in one sum |
| Rounding | HALF_AWAY_FROM_ZERO to money scale for stored money |
| FX | If needed: rate version id + as-of timestamp; never silent |
| Rate comparable | unit, lane direction, equipment, load terms, currency, tax, validity — else RATE_NOT_COMPARABLE |
| Formulas | ProjectedCost = Σ x·r; TargetBudget = Σ V·target; Variance = Projected − Target; uncovered cost labeled separately |
| Guarantee volumes | Integer trip distribution must conserve total when required |

---

## Boundary

Allocation APIs must not use legacy `roundMoney(float64)` for budget outputs.  
Legacy evaluation APIs must not be rewritten to decimal without separate Controller GO.
