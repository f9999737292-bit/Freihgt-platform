# BNO-0.1C1 Next-load candidate search

BNO-0.1C1 lets a carrier search published load opportunities from one of its own available capacities. The result is a deterministic set of hard-feasible match candidates. This stage does not score, rank, or return a top N.

## Candidate pipeline

1. The caller sends `capacity_id`, an optional policy override, and an optional candidate limit. A vehicle id is not search authority. `candidate_limit` below zero is a validation error. Zero returns no candidate rows and leaves the eligible count intact. The limit truncates the returned list; it is not a ranking.
2. The capacity must belong to the caller, exist, be `AVAILABLE`, and have coordinates plus an availability interval. A foreign or missing capacity is hidden with 404. `PREDICTED` and `WITHDRAWN` are not executable.
3. For `CURRENT_SHIPMENT_PREDICTION`, the feasibility clock is `PredictedCapacity.predicted_available_at`. The same current prediction supplies known equipment facts: combination type, body type, loading access, unloading access, weight, volume, temperature capability, legacy equipment type, and container size. `availability_window_start` and `availability_window_end` stay uncertainty context. Manual capacity uses `available_from` and only its own known capacity facts.
4. The effective policy is resolved, then validated.
5. The pool is published loads already visible to the caller. Visibility is checked before feasibility.
6. Mode-specific geometry prefilters run. They do not assign road distance.
7. Surviving pickups get a truck road deadhead from the routing provider. Further road legs are requested only for the active mode.
8. Hard gates run in a fixed order: deadhead limits, delivery direction or route increase, pickup window, payload and volume, then the existing cargo-equipment evaluator.
9. The search run and every evaluated candidate are stored for the searching tenant. The response returns eligible candidates only, plus aggregate rejection counts.

Eligible candidates are ordered by load id until scoring exists.

## Visibility

The pool is the marketplace projection:

- `MARKETPLACE`
- `ANONYMIZED_MARKETPLACE`
- `INVITED_CARRIERS` when the caller company is invited

`PRIVATE` and `NETWORK_OPTIMIZATION_ONLY` are not scanned into the carrier pool. The search does not read another tenant's shipments, transport orders, or raw order tables.

## Effective policy

Ordinary fields use request, then capacity override, then carrier default.

Hard maxima use the minimum of every supplied value:

- `forward_search_km`
- `corridor_deviation_km`
- `max_deadhead_km`
- `max_deadhead_minutes`
- `max_route_increase_km`
- `radius_km`

A request may tighten a hard maximum. It cannot widen one. `preferred_deadhead_km` is not a hard gate. The response includes the effective policy and its fingerprint.

## RADIUS

`radius_km` is required and must be positive. It is not `max_deadhead_km`.

The prefilter keeps a pickup whose straight-line distance from the release point is within `radius_km`. Haversine is only that prefilter. The hard deadhead limit, when set, is the later truck road distance.

## DIRECTIONAL_CORRIDOR

The provider returns the truck route from release to the policy target. Its normalized geometry is the corridor. A provider-neutral spherical projection then reports:

- `forward_progress_km`: along-track distance from the release. Negative means behind the release.
- `lateral_distance_km`: cross-track distance from the polyline.

Neither value is road distance. A pickup must have forward progress from 0 through `forward_search_km` and lateral distance within `corridor_deviation_km`. Road deadhead is calculated afterwards and remains a separate limit.

## Delivery direction

A pickup on the corridor is not enough. For this mode the road distance from delivery to the target must be strictly smaller than the road distance from pickup to the target. Otherwise the candidate is `DELIVERY_NOT_TOWARD_TARGET`. City names are not used.

## ROUTE_ELLIPSE

The insertion primitive is separate from the two-leg helper:

`next_load_route_increase_km = road(release, pickup) + road(pickup, delivery) + road(delivery, target) - road(release, target)`

All four terms are road kilometres. The candidate is eligible only when the increase is within `max_route_increase_km`.

## Deadhead and time

`road_deadhead_km` and `road_deadhead_minutes` come from the same truck route from the capacity release point to the pickup. `max_deadhead_km` and `max_deadhead_minutes` are hard limits (`MAX_DEADHEAD_EXCEEDED`, `MAX_DEADHEAD_TIME_EXCEEDED`).

Arrival at pickup is the effective available time plus `road_deadhead_minutes`. Arrival after the pickup window end is `PICKUP_WINDOW_MISSED`. Arrival before the window start records `waiting_minutes` and stays eligible. A missing window stays `UNKNOWN`; the search does not invent one.

If a required road route cannot be calculated, the candidate is `ROAD_DISTANCE_UNKNOWN`. Straight-line distance is not a substitute. A provider failure does not produce an eligible candidate.

## Weight, volume, and compatibility

Known load weight and volume must fit known capacity payload and volume (`PAYLOAD_EXCEEDED`, `VOLUME_EXCEEDED`). Unknown is not zero. A known load fact with a missing capacity fact is `CAPACITY_FACT_UNKNOWN`.

Cargo and equipment compatibility reuses the BNO-0.1B2 evaluator. Known cargo facts are passed through: cargo type, weight, volume, pallet count and type, linear metres, loaded height, stackable, fragile, packaging, food grade, temperature requirement and range, preferred setpoint, dangerous goods, hazard classes, odor, and contamination. Unknown values stay unknown. They are not stored as false or zero.

Access requirements stay separate: required and allowed loading access, and required and allowed unloading access. Access is not inferred from body type. Required body types and required equipment types come from the load.

Manual equipment uses body type, equipment type, remaining payload, and remaining volume. A catalog may fill missing capabilities for a known equipment type: unit kind, combination type, pallet positions, linear metres, dimensions, loading and unloading access, temperature, food grade, and ADR. The search does not invent a capability the catalog does not contain.

A current predicted-capacity snapshot adds the facts listed above when they are present. Container size is kept as equipment provenance. The evaluator has no container-size rule, so the fact is not turned into a compatibility decision.

`COMPATIBLE` passes. `INCOMPATIBLE` is a hard reject. `INDETERMINATE` fails closed for executable matching. Unknown temperature, loading access, ADR, pallet positions, linear metres, or height does not pass when the cargo requires that dimension.

## Privacy

Matching may use internal search geography, including exact coordinates of an anonymized load. The response uses display geography. An anonymized candidate does not return exact latitude, longitude, location id, facility label, address, postal code, exact road deadhead, or exact corridor offsets. Deadhead is one of `0-25`, `25-50`, `50-100`, `100-200`, or `200+` km. Non-anonymized visible loads may return the exact road distance.

Rejected marketplace loads are not listed. The response has `rejection_counts_by_reason` only. Stored rejection rows remain on the searching tenant.

## Routing provider provenance

`routing_provider` on a search run is a vendor name only when the active provider implements `IdentifiedProvider`. The 2GIS adapter reports `2GIS`. No configured provider is stored as `UNCONFIGURED`. A provider that does not expose a name is stored as `UNSPECIFIED`. `CONFIGURED` is configuration state and is not written as a provider identity. The routing port remains route and matrix calls.

## Matrix batching and failure

Synchronous matrix calls stay within the provider limit of 25 sources or 25 targets. Batches are deterministic, and merged cells keep the load identity. This stage does not call an asynchronous matrix API. Routing errors are counted without tenant or load labels.

## What this stage does not do

No match score, weighted ranking, revenue or contribution ranking, top N, machine learning, or consolidation solver.
