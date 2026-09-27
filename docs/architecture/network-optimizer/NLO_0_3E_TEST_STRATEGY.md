# NLO-0.3E test strategy

Controller review R1 revised this list. `TEST_STRATEGY_FROZEN=NO`. These checks are proposed for a future implementation task. They are not implemented here. The discovery harness `nlo_03e_discovery_benchmark_test.go` (build tag `nlo03ediscovery`) measures the current size-2 and one-load code only.

## Scope

- A same-origin same-destination set larger than 2 uses one pickup location and one delivery location. The result has no stop list.
- The search does not accept a second additional load on `CURRENT_TRIP_FILL`. That pattern stays `MAX_ADDITIONAL_LOADS=1`.
- The search does not implement `MULTI_PICK_ONE_DROP`, `ONE_PICK_MULTI_DROP`, or `MULTI_PICK_MULTI_DROP`.
- No `RoutePlan`, `RouteStop`, or driver multi-stop task is written.

## Sequence verdict

- NLO-0.3E explores zero alternate route sequences and does not emit a sequence-feasibility claim.
- A sequence-independent groupage failure may be `HARD_REJECT`.
- An order-dependent failure is not `HARD_REJECT` of the set. If a later wave explores stop orders, one failed order stays `INDETERMINATE` until the authorized sequence bound is exhausted.
- A fixture with N eligible loads in one group does not generate `2^N` sets and does not generate `N!` sequences.

## Bounds

These checks wait on `NLO03E_I001`. The numbers are unset, so the checks cannot be implemented yet.

- Pool larger than `MAX_CANDIDATE_POOL` returns `POOL_LIMIT_EXCEEDED` and does not enumerate pairs.
- Set size never exceeds `MAX_SET_SIZE`.
- Assessed sets never exceed `MAX_SETS_EVALUATED`.
- Generation order is load-id lexicographic and the fingerprint is stable across repeats.

`NLO03E_I003`: `candidate_limit` changes the returned page only. It does not change evaluated count, routing calls, or rows persisted. A test must show limit 0 and a large limit do the same evaluation and persistence work until a separate server budget stops the search.

## Capacity and compatibility

Reuse `EvaluateGroupageItems` and `ResidualCapacitySnapshot`. Assert aggregation, not a second engine, for weight, volume, pallets, linear metres, temperature intersection, ADR, food grade, odor, contamination, and loading or unloading access. Unknown stays `INDETERMINATE`. Unknown is not zero.

## Invalidation

A fingerprint change is required when any of these change and the feasibility status stays the same:

- rule or catalog version
- load version
- shipment version
- tracking freshness or position
- routing request fingerprint
- rehandling policy

## Tenancy and privacy

- Same-owner sets may be feasible when every required fact is known.
- Two shippers, and three or more shippers, stay `INDETERMINATE` with `MULTI_PARTY_REFERENCE_CONTEXT_UNAVAILABLE`.
- One tenant's catalog overlay is not applied to another tenant's cargo.
- Public JSON does not contain the internal compatibility trace.

## Routing and evidence

- Same-origin set expansion does not add route legs.
- Routing unavailable on the existing one-load fill yields indeterminate, not a haversine distance.
- Stale position skips routing and is not feasible.
- Stale ETA does not become a feasible arrival.
- Hard residual reject still counts as an evaluated set. After a routing budget exists, a hard reject must not be allowed to exceed `MAX_ROUTING_CALLS`.

## Budgets and audit

- Search budget reached returns `SEARCH_BUDGET_EXCEEDED`.
- Pool budget reached returns `POOL_LIMIT_EXCEEDED`.
- Routing budget reached returns `ROUTING_BUDGET_EXCEEDED`.
- `NLO03E_I002`: N-member rows are not persisted until a later migration widens `ordinal IN (1, 2)` and the generated OpenAPI member list. This discovery does not add that migration.
- The deterministic fingerprint covers policy, member ids and versions, and compatibility provenance.

## Execution boundary

`execution_supported=false`. The search does not mutate a shipment or order and does not create an assignment, reservation, carrier offer, slot, or driver task. It does not activate a `RoutePlan` or write a `RouteStop`.
