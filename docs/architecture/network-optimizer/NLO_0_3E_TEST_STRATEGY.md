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

These checks use the server defaults from the Postgres measurement. `candidate_limit` is still not one of them (`NLO03E_I003`).

- More than 10 eligible loads in one same-origin group returns `POOL_LIMIT_EXCEEDED` and does not enumerate sets.
- Set size never exceeds 3.
- Assessed sets never exceed 1225.
- Groupage calls never exceed 2450.
- Routing calls for an NLO-0.3E set are 0.
- Wall time past 5s returns `SEARCH_BUDGET_EXCEEDED`.
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

- Same-origin set expansion makes zero routing calls. `NLO03E_ROUTING_CALLS_PER_SET=0`.
- The NLO-0.3E search does not return `ROUTING_BUDGET_EXCEEDED`.
- Existing NLO-0.3D tests still cover one-load fill: routing unavailable stays indeterminate, a stale position skips routing, and a stale ETA is not a feasible arrival. Those protections are not removed.

## Budgets and audit

- Search budget reached returns `SEARCH_BUDGET_EXCEEDED`.
- Pool budget reached returns `POOL_LIMIT_EXCEEDED`.
- `NLO03E_I002`: N-member rows are not persisted until a later migration widens `ordinal IN (1, 2)` and the generated OpenAPI member list. This discovery does not add that migration. NLO-0.3E does not assert a routing budget.
- The deterministic fingerprint covers policy, member ids and versions, and compatibility provenance.

## Execution boundary

`execution_supported=false`. The search does not mutate a shipment or order and does not create an assignment, reservation, carrier offer, slot, or driver task. It does not activate a `RoutePlan` or write a `RouteStop`.
