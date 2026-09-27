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
- Confirmed onboard cargo remains in the residual ledger. Its delivery is kept. It does not receive a synthetic pickup.

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
- The same `RouteLegKey` inside one search is one provider call. A different departure bucket is a different key.
- `UNIQUE_LOCATION_PAIRS` is not asserted as the provider-call count.
- Sequence generation and tie-break are stable across repeated runs.

## Budgets and search results

- `MAX_STOPS=8`, `MAX_ADDITIONAL_LOADS=2`, `MAX_SEQUENCE_CANDIDATES=64`, `MAX_ROUTE_LEG_EVALUATIONS=300`, `MAX_ROUTING_PROVIDER_CALLS=300`, `MAX_GROUPAGE_EVALUATIONS=600`, and the 5 second watchdog each fail closed as `SEARCH_BUDGET_EXHAUSTED`.
- `MAX_ROUTE_LOAD_SUBJECTS=4` fails as `PLAN_LOAD_LIMIT_EXCEEDED` and does not drop onboard cargo.
- A stopped search is `SEARCH_BUDGET_EXHAUSTED`.
- A finished heuristic with no plan is `NO_PLAN_FOUND_WITHIN_POLICY`.
- The public API does not return `NO_FEASIBLE_SEQUENCE` for this heuristic.
- Greedy search of 43 candidates on 4 future stops is not the 435-candidate parent-retention tree.
- Load order is lexicographic load id. Another order is not searched.

## Current trip, duration, and activation

- `GREEDY_SEARCH_DOES_NOT_CLAIM_GLOBAL_INFEASIBILITY`
- `ALTERNATIVE_FIRST_INSERTION_COULD_BE_FEASIBLE` does not become `NO_FEASIBLE_SEQUENCE`
- `LOAD_ORDER_IS_DETERMINISTIC`
- `CURRENT_TRIP_INITIAL_ONBOARD_SET_SEEDED`
- `ONBOARD_CARGO_HAS_DELIVERY_WITHOUT_SYNTHETIC_PICKUP`
- Three onboard plus one additional is allowed.
- Three onboard plus two additional is rejected.
- Four onboard plus one additional is rejected.
- `UNKNOWN_SERVICE_DURATION_INDETERMINATE`
- `ZERO_SERVICE_DURATION_NOT_ASSUMED`
- Depot-start activation allows only `CARRIER_ASSIGNED`, `ACCEPTED_BY_CARRIER`, `VEHICLE_ASSIGNED`, `DRIVER_ASSIGNED`, and `PICKUP_SLOT_BOOKED`.
- Current-trip activation allows `LOADED` and `IN_TRANSIT`, and also `IN_PICKUP`.
- `CURRENT_TRIP_ACTIVATION_DOES_NOT_RESET_SHIPMENT_STATUS`
- Route-leg evaluation budget, routing-provider call budget, and groupage evaluation budget are separate.
- The same location pair with a different departure bucket is a different cache key and may be another provider call.

## Concurrency and tenancy

- Accept with a changed shipment version is `409` `PLAN_STALE`.
- Accept twice with one idempotency key returns one accepted plan.
- Activate twice does not create two driver-task requests.
- A foreign tenant does not read the plan (`404`).
- Cross-shipper output has no foreign catalog text and no owner tenant id on the public load view.
- An accepted plan has no update path for stops. A replan inserts a new plan and marks the previous one superseded only after the insert commits.
