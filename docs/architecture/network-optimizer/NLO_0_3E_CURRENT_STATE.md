# NLO-0.3E current state

Discovery baseline: `origin/main` `6576034a4f39b451341ad72667c5fc25f6562658`. Migration head `000083_nlo_current_trip_fill_v0_3d`. NLO-0.3B, NLO-0.3C, and NLO-0.3D are IMPLEMENTED_CLOSED. This document records the code that exists. It does not change it.

`CURRENT_TRIP_FILL_PUBLIC_ENABLED=YES` for the planning search only. `MAX_ADDITIONAL_LOADS=1`. `EXECUTION_SUPPORTED=NO`.

## 3.1 SAME_ORIGIN_SAME_DESTINATION

Source: `services/network-optimizer-service/internal/service/consolidation.go` `SearchConsolidation`.

Candidate pool source: `ListPublicConsolidationPool`. The searcher does not see loads they own. A load enters the pool only when it is `PUBLISHED`, opted in (`consolidation_allowed` or `cross_shipper_consolidation_allowed`), and visible as `MARKETPLACE`, `ANONYMIZED_MARKETPLACE`, or an invited company. Postgres orders by pickup location, delivery location, then id. The query has no `LIMIT`.

Pair generation: loads with both location ids are grouped by `pickup.location_id|delivery.location_id`. Different origin-destination keys are never paired. Within one group the loads are sorted by id. Nested loops `i` from 0 and `j` from `i+1` emit each unordered pair once. A pair that is not opted in is counted as excluded and is not assessed. For one origin-destination group of N eligible loads the generated pair count is `N*(N-1)/2`. The discovery benchmark used one group and asserted that equality at 10, 50, 100, 500, and the exploratory 1000.

`TestNLO03BPerformanceFixtures` splits those sizes across several groups, so it does not measure this single-group quadratic.

Compatibility per assessed pair: `assessSameOwner` loads a catalog context for the capacity owner and a catalog context for the load owner, then `EvaluateGroupageItems` once per context when the owner scope is provable. Empty catalog contexts are provable. That is two catalog loads and two groupage evaluations per same-owner pair. Disjoint windows return before catalog and groupage. `assessCrossShipper` runs one groupage on an empty context and returns `INDETERMINATE` / `MULTI_PARTY_REFERENCE_CONTEXT_UNAVAILABLE`. It does not return `FEASIBLE`.

Persistence: every assessed pair is stored, including hard rejects and pairs omitted from the HTTP body. One run row, one candidate row, and two member rows (ordinals 1 and 2).

Sorting: feasible, then indeterminate, then hard reject; then left load id; then right load id. This is status rank plus load id. It is not a commercial score.

Privacy projection: public views do not carry the internal compatibility trace. Cross-shipper pairs stay non-feasible.

## 3.2 CURRENT_TRIP_FILL

Source: `searchCurrentTripFill` in `current_trip_fill.go`.

Pool scan: the same public pool, then `fillOptedIn`. If the load owner is the searcher, `ConsolidationAllowed` is required. Otherwise `CrossShipperConsolidationAllowed` is required. Eligible loads are sorted by load id. One `assessFill` runs per eligible load. There is no second additional load.

`CurrentTripContext.Build` reads execution, onboard evidence, cargo profiles, vehicle, tracking, and ETA once per search, not once per candidate.

Routing: `insertRoad` calls `roadLeg` four times (direct, position to pickup, pickup to delivery, delivery to destination) when the position is fresh and coordinates resolve. A hard residual reject does not skip routing or groupage. A stale position skips routing. Missing confirmed onboard returns before groupage and routing as indeterminate. Haversine is not used as road distance.

Groupage: `fillGroupage` calls `assessSameOwner`. Catalog and groupage counts follow the same two-context rule when the searcher tenant and the load owner differ.

Audit: every assessed candidate is persisted, then the response JSON is marshaled. Fill members are the one additional load (one member row per candidate in the measured harness).

Sorting: the assessed slice is ordered before the response cap. Status is part of that order. There is no MatchScore.

`candidate_limit` behavior, proven at the truncation site in both `consolidation.go` and `current_trip_fill.go`: the limit is applied to the returnable slice after every candidate has been evaluated and after the full stored set has been built. Limit 0 returns an empty candidate list and still evaluates and persists the full set. A negative limit is a validation error. It does not limit pool scan, routing, groupage, or writes.

```text
candidate_limit = returned result limit
candidate_limit does not limit evaluation
```

## 3.3 Database and query behavior

`ListPublicConsolidationPool` is one unbounded read of every published opted-in visible load. Index `load_opportunities_published_opt_in_od_idx` is partial on published opted-in `(pickup_location_id, delivery_location_id, id)`. It does not cap the row count.

Pairwise work after that read is in process: grouping is O(N), and one origin-destination group is O(N²) pairs. Each assessed pair performs catalog lookups and then three logical writes (candidate plus two members) plus the one run row. Response JSON includes every returned candidate. At a single group of 500 eligible loads the harness persisted 374251 logical rows and marshaled about 277 MB of JSON. That is measured in-memory row accounting, not Postgres latency.

Fill work after the pool read is one assessment per eligible load. Routing is four provider calls per candidate when the position is fresh. Writes are one run, one candidate, and one member per candidate. The pool read is still O(N) with no SQL limit.

Consolidation runs are indexed by tenant and capacity. Candidates are indexed by run. Neither index stops a search from inserting one row per evaluated pair.

Risks that the code structure shows, without a Postgres timing claim:

- O(N) pool read with no limit.
- O(N²) pair generation inside one origin-destination group.
- Catalog and groupage work per assessed pair, doubled when two tenants are projected.
- Write amplification of one candidate and two members per pair, including pairs the client does not receive.
- JSON amplification of the returned body.
- Four routing calls per current-trip candidate, including hard rejects, whenever the position is fresh.

## Metrics that exist

`services/network-optimizer-service/internal/platform/metrics/metrics.go` emits:

- `bno_consolidation_searches_total`
- `bno_consolidation_sets_evaluated_total`
- `bno_consolidation_outcomes_total{status}` with status `FEASIBLE`, `INDETERMINATE`, or `HARD_REJECT`
- `bno_consolidation_duration_seconds` with buckets 0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5

The code does not emit `bno_consolidation_hard_reject_total`, `bno_consolidation_indeterminate_total`, or `bno_consolidation_set_size_bucket`. Older contract notes that name those series are ahead of the emitter. No metric label carries an id.

## Persistence shape

`consolidation_search_runs` and `consolidation_candidates` can store a search of set size 2. `consolidation_candidate_members` has `CHECK (ordinal IN (1, 2))` in migration `000081`. OpenAPI `ConsolidationMember.ordinal` is enum `[1, 2]` and `members.maxItems` is 2 (`packages/openapi/network-optimizer-service.yaml`). A set with three or more loads cannot be stored without a later migration. This discovery does not add migration `000084`.

## Public API

`POST /v1/network/consolidation/search` dispatches `CURRENT_TRIP_FILL` and `SAME_ORIGIN_SAME_DESTINATION`. An unknown pattern returns `PATTERN_NOT_IMPLEMENTED`. No second search route exists.
