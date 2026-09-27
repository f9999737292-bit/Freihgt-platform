# NLO-0.3E test strategy

Controller review R3 freeze candidate. `TEST_STRATEGY_FREEZE_CANDIDATE=YES`. `TEST_STRATEGY_FROZEN=NO` until the controller accepts it. These checks are not implemented here. The discovery harnesses use build tag `nlo03ediscovery` and are not part of default `go test`.

## Scope

- Sets contain 2 or 3 members, all with the same canonical origin and destination. The result has no stop list.
- A fourth member is not generated.
- Generation is load-id lexicographic and the fingerprint is stable across repeats.
- Window intersection uses every member.
- The search does not accept a second additional load on `CURRENT_TRIP_FILL`. That pattern stays `MAX_ADDITIONAL_LOADS=1`.
- Multi-pick and multi-drop are not implemented.
- No `RoutePlan`, `RouteStop`, or driver multi-stop task is written.
- Route sequences explored are 0. Routing calls are 0.

## Bounds

- The global eligible pool read uses `ORDER BY pickup_location_id, delivery_location_id, id LIMIT 11`. An eleventh row returns `POOL_LIMIT_EXCEEDED` and does not enumerate sets.
- Set size never exceeds 3.
- Assessed sets never exceed 165. The precheck happens before the set is evaluated.
- Groupage calls never exceed 330. The precheck happens before the call.
- Elapsed time at or past 5 seconds does not start another set and returns `SEARCH_BUDGET_EXCEEDED`. The 5 seconds are a watchdog, not an SLO.
- A structural budget failure wins over the watchdog when both are already true.
- `candidate_limit` 0 and a large `candidate_limit` do not change pool reads, sets, groupage calls, or rows written.

## Failure atomicity

- The response is not HTTP 200 with a partial candidate list presented as complete.
- Candidate, member, and run rows of the attempt roll back.
- A completed run is not used to record the failure.

## Capacity and compatibility

Reuse `EvaluateGroupageItems` and `ResidualCapacitySnapshot`. Assert aggregation, not a second engine, for weight, volume, pallets, linear metres, temperature intersection, ADR, food grade, odor, contamination, and loading or unloading access. Unknown stays `INDETERMINATE`. Unknown is not zero.

## Invalidation

A fingerprint change is required when any of these change and the feasibility status stays the same:

- capacity id or version
- load id or version
- cargo or profile version that the evaluation consumed
- catalog version
- rule-set version
- policy version
- algorithm or budget policy version
- canonical origin, destination, or window inputs through their authoritative versions

Shipment version, tracking position, tracking freshness, ETA, routing request fingerprint, routing proof, and multi-stop rehandling are not NLO-0.3E fingerprint inputs. NLO-0.3D tests for those stay in the current-trip suite.

## Tenancy and privacy

- Same-owner sets may be feasible when every required fact is known.
- Two shippers, and three or more shippers, stay `INDETERMINATE` with `MULTI_PARTY_REFERENCE_CONTEXT_UNAVAILABLE`.
- One tenant's catalog overlay is not applied to another tenant's cargo.
- Public JSON does not contain the internal compatibility trace.

## Members

- Ordinal is unique within the candidate and is between 1 and 3.
- Load id is unique within the candidate.
- A two-member candidate still validates against the future contract.

## Execution boundary

`execution_supported=false`. The search does not mutate a shipment or order and does not create an assignment, reservation, carrier offer, slot, or driver task. It does not activate a `RoutePlan` or write a `RouteStop`.
