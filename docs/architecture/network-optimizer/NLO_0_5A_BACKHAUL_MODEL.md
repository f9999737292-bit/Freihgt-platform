# NLO-0.5A backhaul model

Docs only. One additional published load from a release point toward an authoritative target. No new search mode.

```text
BACKHAUL_FIRST_RELEASE_SCOPE=ONE_LOAD_TOWARD_TARGET
CAPACITY_SOURCES=MANUAL_AVAILABLE,PREDICTED_CAPACITY,CURRENT_TRIP_RESIDUAL
SUPPORTED_SEARCH_MODES=DIRECTIONAL_CORRIDOR,ROUTE_ELLIPSE
ROAD_DISTANCE_REQUIRED_FOR_EXECUTABLE_PLAN=YES
HAVERSINE_EXECUTABLE_DISTANCE_ALLOWED=NO
UNRESTRICTED_CHAIN_SEARCH=NO
```

`RADIUS` stays a next-load prefilter. It does not prove return direction, so it is not a backhaul mode.

## Geometry

Release is the capacity point: manual `available` location, predicted available location, or the current-trip position anchor already built by NLO-0.3. Target is `NextLoadSearchPolicy.target_location_id`. The carrier search policy owns that id. A request may tighten hard maxima. It may not widen them and it may not replace the location snapshot with caller coordinates.

Home is not inferred from the user, the vehicle, or a city name. If the objective is return-home, the existing rule already requires `target_location_id`.

Toward the target means the existing tests:

- corridor: forward progress from 0 through `forward_search_km`, lateral distance within `corridor_deviation_km`, and road distance from delivery to target strictly smaller than road distance from pickup to target
- ellipse: the four-leg road increase below

Pickup behind the release is `BACKTRACK_DIRECTION_REJECTED`.

## Road facts

Executable deadhead, loaded distance, and route increase come from the routing port. Unknown stays unknown.

```text
HARD_DEADHEAD_LIMIT=policy max_deadhead_km AND max_deadhead_minutes
CORRIDOR_POLICY=EXISTING_DIRECTIONAL_CORRIDOR
```

## Time

Pickup arrival is `available_at + road_deadhead_minutes`. Arrival before the window records `waiting_minutes`. Arrival after the window end is `PICKUP_WINDOW_MISSED`. A missing window stays unknown.

When the candidate is promoted to an NLO-0.4 route plan, pickup and delivery service seconds come from the active operating-tenant service-duration policy. A missing policy does not become zero and does not become an executable plan.

Roundtrip completion time, for this one load, is arrival at delivery plus delivery service, plus the road leg from delivery to the target when that leg is required by the ellipse. It is not a chain ETA.

## Capacity

| Source | First release |
| --- | --- |
| Manual `AVAILABLE` capacity | Yes |
| `PredictedCapacity` with a fresh ETA under the existing prediction age policy | Yes. Stale prediction stays rejected. |
| Current-trip residual context | Yes. Onboard evidence, completed stops, and TMS return state are read-only. |
| Fleet-wide vehicle assignment | No. That remains FLEET-1.x. |

## Ranking

Reuse the active score profile. Components already include deadhead efficiency, capacity utilization, waiting efficiency, and target proximity. Revenue stays unpriced when no rate snapshot exists. This wave does not add a freight-cost ledger.

## Pipeline

```text
visible marketplace prefilter
-> corridor or ellipse geometry
-> batched road routing
-> time, deadhead, compatibility
-> score
```

`ROAD_ROUTING_AFTER_PREFILTER=YES`. Private and unpublished foreign loads stay out of the pool. Anonymized results keep display geography. Exact coordinates, location id, address, and facility labels stay internal. Exact road kilometres on an anonymized candidate stay in the existing coarse buckets.
