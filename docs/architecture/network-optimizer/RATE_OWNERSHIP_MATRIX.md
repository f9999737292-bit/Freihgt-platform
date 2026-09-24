# Rate ownership matrix

The optimizer is not a third rate engine and not a second freight-cost service.

| Concept | Owner | Optimizer use | Optimizer must not |
|---------|-------|---------------|--------------------|
| Contract rate | `contract-rate-service` | Read resolution for a lane, equipment, and date | Store its own rate cards or change versions |
| Market rate | Not found as a master | Optional future external input, snapshotted | Invent a market index |
| Offered rate | Load opportunity / carrier offer | The price the publisher puts on an opportunity or offer | Treat it as the contract SSOT |
| Carrier counter rate | `CarrierOffer` in `COUNTERED` | Record the counter as an offer revision | Write it into the contract |
| Rate snapshot | `transport-order-service` | Read the frozen order price as a commercial input | Mutate `transport_order_rate_snapshots` |
| Expected operating cost | Planning estimate inside the decision snapshot | Compute a **planning** cost for ranking | Insert freight-cost ledger rows |
| Actual freight cost | `freight-cost-service` | Ignore for ranking except as historical read if a later API allows | Recalculate variance or accruals |
| Settlement | `billing-register-service` | None at match time | Create settlements from a recommendation |
| Award / bid price | `rfx-service` | If the shipper chose `MINI_TENDER`, `RFQ`, or `AUCTION`, hand off | Run a parallel tender state machine |

## Planning formula

Canonical **planning** contribution. Each term is either a referenced snapshot or an explicit assumption with a version:

```text
expected_revenue
- loaded_operating_cost
- deadhead_cost
- waiting_cost
- tolls
- fuel_cost
- driver_cost
- stop_cost
- expected_delay_cost
- risk_adjustment
= expected_contribution
```

If revenue or a required cost input is missing, `commercial_status = UNPRICED`. Unpriced candidates are not ranked on `MAX_REVENUE` or `MAX_CONTRIBUTION`. They may still appear in a feasibility list for the owning carrier.

`expected_contribution` is not a `PLANNED_COST_SNAPSHOT`. Freight cost remains the only writer of planned, accrual, billed, and paid cost entries.

## Offer versus RFx

| Mode | Where it lives |
|------|----------------|
| `DIRECT_ACCEPT` | Carrier offer → assignment, still subject to reservation races |
| `FIXED_PRICE_OFFER` | Carrier offer at the published rate |
| `COUNTER_OFFER` | Carrier offer revision |
| `MINI_TENDER` | Create/use freight request in `rfx-service` |
| `RFQ` | RFx event |
| `AUCTION` | RFx event |

Flow: optimizer selects candidate carriers, then either opens an offer in this context or asks RFx to open a competitive procedure. Award stays in RFx. A won award returns through the existing award-to-order path, not through a private optimizer price table.
