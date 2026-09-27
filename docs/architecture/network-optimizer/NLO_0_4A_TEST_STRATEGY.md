# NLO-0.4A test strategy

Status: proposed candidate. These tests are not implemented. The only executable check in this discovery is the combinatorial cost model.

```text
TEST_STRATEGY_FROZEN=NO
TEST_STRATEGY_FREEZE_CANDIDATE=YES
PRODUCTION_PLANNER_TESTS=NO
```

Later implementation must include:

## Compatibility with NLO-0.3

- One additional load on a current trip still matches NLO-0.3D feasibility when the insertion chosen is position, pickup, delivery, destination. `CURRENT_TRIP_FILL` itself stays capped at 1 until that implementation wave explicitly switches callers.
- Pairwise same-origin search and N-member same-origin search stay unchanged.

## Sequencing

- Two additional loads.
- Pickup ordinal is never after delivery ordinal.
- Two loads with the same pickup `location_id` share one stop and keep two pickup actions.
- Two loads with the same delivery `location_id` share one stop.
- A stop may contain a delivery and a later load's pickup when the location matches. Action order is pickup-before-delivery for the same load.
- A completed stop is not an insertion gap and does not change ordinal relative to other completed stops.
- Confirmed onboard cargo remains in the residual ledger. Its delivery is not dropped.

## Capacity and compatibility

- Weight, volume, pallets, and linear metres are checked after every action.
- Two cargos are compatibility-checked only on legs where both are onboard.
- A temperature conflict on one overlapping leg rejects the sequence even if earlier legs were fine.
- ADR, food-grade, and odor use the same per-leg onboard set.
- Unknown required data is `INDETERMINATE`, not feasible.
- `EvaluateGroupageItems` is the only compatibility engine. A test fails if a second rule engine is introduced.

## Time and routing

- Arrival before the window waits.
- Arrival after the window is `STOP_WINDOW_VIOLATION`.
- Provider unknown, timeout, and unavailable are `ROUTING_UNAVAILABLE` and do not store distance 0.
- Haversine is not written into `distance_m`.
- The same `RouteLegKey` inside one search is one provider call.
- Sequence generation and tie-break are stable across repeated runs.

## Budgets

- `MAX_STOPS=8`, `MAX_ADDITIONAL_LOADS=2`, `MAX_SEQUENCE_CANDIDATES=64`, `MAX_ROUTING_CALLS=300`, and the 5 second watchdog each fail closed as `SEARCH_BUDGET_EXCEEDED` with the matching `budget` value.
- A stopped search is `SEARCH_BUDGET_EXHAUSTED`, never `NO_FEASIBLE_SEQUENCE`.
- `NO_FEASIBLE_SEQUENCE` appears only when the bounded pairs were all evaluated.

## Concurrency and tenancy

- Accept with a changed shipment version is `409` `PLAN_STALE`.
- Accept twice with one idempotency key returns one accepted plan.
- Activate twice does not create two driver-task requests.
- A foreign tenant does not read the plan (`404`).
- Cross-shipper output has no foreign catalog text and no owner tenant id on the public load view.
- An accepted plan has no update path for stops. A replan inserts a new plan and marks the previous one superseded only after the insert commits.
