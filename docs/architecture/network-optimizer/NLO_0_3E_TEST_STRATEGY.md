# NLO-0.3E test strategy

These tests are frozen for a future implementation task. They are not implemented here. The discovery harness `nlo_03e_discovery_benchmark_test.go` (build tag `nlo03ediscovery`) measures the current size-2 and one-load code only.

## Bounds

- Pool larger than `MAX_CANDIDATE_POOL` returns `POOL_LIMIT_EXCEEDED` and does not enumerate pairs.
- Set size never exceeds `MAX_SET_SIZE`.
- Assessed sets never exceed `MAX_SETS_EVALUATED`.
- Route sequences per set equal 1.
- A fixture with N eligible loads in one group does not generate `2^N` sets and does not generate `N!` sequences.
- Generation order is load-id lexicographic and the fingerprint is stable across repeats.
- `candidate_limit` changes the returned page only. It does not change evaluated count, routing calls, or rows persisted.

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

- Routing unavailable yields indeterminate, not a haversine distance.
- Stale position skips routing and is not feasible.
- Stale ETA does not become a feasible arrival.
- Hard residual reject still counts as an evaluated set. After the routing budget exists, a hard reject must not be allowed to exceed `MAX_ROUTING_CALLS`.

## Budgets and audit

- Search budget reached returns `SEARCH_BUDGET_EXCEEDED`.
- Pool budget reached returns `POOL_LIMIT_EXCEEDED`.
- Routing budget reached returns `ROUTING_BUDGET_EXCEEDED`.
- Every assessed set that is stored has a run, a candidate, and one member row per load, with ordinals inside the migrated check.
- The deterministic fingerprint covers policy, member ids and versions, compatibility provenance, and routing proof.

## Execution boundary

`execution_supported=false`. The search does not mutate a shipment or order and does not create an assignment, reservation, carrier offer, slot, or driver task. It does not activate a `RoutePlan` or write an executable `RouteStop`.
