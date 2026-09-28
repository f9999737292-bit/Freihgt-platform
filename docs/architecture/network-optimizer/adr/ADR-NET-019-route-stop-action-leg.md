# ADR-NET-019: Route stop, action, and leg

## Status

Accepted. NLO-0.4B persists stops, actions, and legs. Execution remains NLO-0.4D.

```text
ADR_NET_019=ACCEPTED
```

## Context

A load is not a stop. Several loads can share a warehouse. NLO-0.3E refuses to order stops. The routing port already returns road distance, duration, and fingerprints, and it can return geometry.

## Decision

`RouteStop` is `location_id` plus ordinal. Roles are `START`, `CARGO`, and `END`. Cargo work is `RouteStopAction` of type `PICKUP` or `DELIVERY`. The action points at a `RouteLoadSubject` (`LOAD_OPPORTUNITY` or `SHIPMENT_CARGO`) by id and version. It does not require a load-opportunity id. An additional load has pickup and delivery. Already-onboard shipment cargo has delivery only. Several actions may share one stop. Pickup and delivery may share a stop only when `location_id` matches, and the pickup action of a subject that has both is ordered before its delivery.

```text
MULTIPLE_ACTIONS_PER_STOP=YES
BREAK_IN_V0_4_PLANNER=NO
WAYPOINT_INSERTION_IN_V0_4_PLANNER=NO
SYNTHETIC_PICKUP_FOR_ALREADY_ONBOARD_CARGO=NO
COMPLETED_PICKUP_HISTORY_IMMUTABLE=YES
INITIAL_CAPACITY_SOURCE=CURRENT_TRIP_CONTEXT
SERVICE_DURATION_SOURCE=AUTHORITATIVE_OR_UNKNOWN
DEFAULT_ZERO=NO
```

No authoritative service duration exists. Unknown duration makes time feasibility indeterminate and blocks activation. Zero is not substituted.

`RouteLeg` connects adjacent stops and stores road `distance_m`, `duration_s`, provider, fingerprints, traffic mode, `calculated_at`, and `expires_at`. Geometry and raw provider bodies are not stored in v0.4. Unknown road distance is not zero. Reuse inside one search uses `from_location_id`, `to_location_id`, `vehicle_profile_hash`, `traffic_mode`, and `departure_bucket`.

`RouteCapacitySnapshot` is persisted as an audit after every future cargo action. For a current trip the first snapshot is the trusted residual, then each action produces the next snapshot. Onboard cargo at `START` is the confirmed current-trip set. `EvaluateGroupageItems` is reused.

```text
GROUPAGE_ENGINE_REUSED=YES
CAPACITY_CHECK_AFTER_EVERY_STOP=YES
ONBOARD_INTERVAL_COMPATIBILITY=YES
```

## Consequences

The ERD is in `NLO_0_4A_ROUTE_PLAN_MODEL.md`. This ADR does not create tables.

## Post-freeze Erratum E1

Accepted. ADR-NET-019 stays Accepted. This section corrects the start-anchor identity and the route-leg cache key. The Decision sentences that require `location_id` on every stop, and that key leg reuse only by `from_location_id` and `to_location_id`, are superseded here for `START` and for `RouteLegKey`. Cargo and `END` canonical identity in that Decision stays. Capacity snapshots and the groupage engine are unchanged.

```text
NLO04A_ERRATUM_E1=ACCEPTED
NLO04A_POST_ACCEPT_F001=CLOSED
ERRATUM_CONTROLLER_ACCEPTANCE=PASS
BLOCKING_FINDINGS=0
ADR_NET_019=ACCEPTED
```

`CurrentTripContext.CurrentPosition` (`TrackingPosition`) has latitude, longitude, `RecordedAt`, and freshness. It has no canonical `location_id`. `Capacity.LocationID` is nullable, while `Capacity.Latitude` and `Capacity.Longitude` are also nullable. A required `location_id` on every stop, including `START`, cannot be implemented against those records. A fabricated location UUID is forbidden.

`RoutePoint` is the routing identity of a stop. Canonical `location_id` remains the business identity.

```text
point_kind = CANONICAL_LOCATION or POSITION_ANCHOR

CANONICAL_LOCATION
  location_id IS NOT NULL
  latitude and longitude come from a trusted server resolution of that location

POSITION_ANCHOR
  location_id IS NULL
  latitude IS NOT NULL
  longitude IS NOT NULL
  allowed only on stop_role=START in this first release
  source = TRACKING_POSITION or CAPACITY_POSITION
```

```text
CARGO_STOP_CANONICAL_LOCATION_REQUIRED=YES
END_STOP_CANONICAL_LOCATION_REQUIRED=YES
CALLER_SUPPLIED_START_POSITION_ALLOWED=NO
CACHE_KEY_SUPPORTS_POSITION_ANCHOR=YES
TRACKING_HISTORY_PERSISTED=NO
PLAN_START_ANCHOR_PERSISTED=YES
```

`CARGO` and `END` use `CANONICAL_LOCATION`. Shared-stop equality is the same canonical `location_id`. Label, city, Haversine, and coordinate similarity do not merge cargo stops. A `POSITION_ANCHOR` never absorbs a pickup or delivery because the coordinates coincide.

`CURRENT_TRIP` `START` is the trusted `CurrentPosition` when `PositionFreshnessStatus` is `FRESH` and both coordinates are present. The persisted anchor is `POSITION_ANCHOR`, `source=TRACKING_POSITION`, with latitude, longitude, and `observed_at`. The current-trip context fingerprint stays a dependency. Missing or not-fresh position fails closed as `ROUTE_START_POSITION_UNKNOWN`.

`DEPOT_START` `START` uses `CANONICAL_LOCATION` when the trusted capacity `LocationID` is set. If that id is null and the trusted capacity has both coordinates, the start is `POSITION_ANCHOR` with `source=CAPACITY_POSITION`. If neither exists, the result is `ROUTE_START_POSITION_UNKNOWN`. City centroids are not coordinates.

`CURRENT_TRIP` `END` is required and canonical. Its `location_id` is `ShipmentExecution.DestinationLocationID`. Coordinates are the trusted resolution of that location. A tracking-coordinate-only end is not allowed. `DEPOT_START` has no separate `END` stop in this first release. The route ends at the last cargo delivery.

`routing.RouteRequest` already routes by origin and destination `Point` latitude and longitude. `routing.Fingerprint` already binds those coordinates, the vehicle profile hash, route mode, traffic mode, and the departure bucket. The plan cache key follows that shape:

```text
RoutePointFingerprint = hash(point_kind, location_id when present, exact routing coordinates, point_source, observed_at when POSITION_ANCHOR)

RouteLegKey =
  from_point_fingerprint
  + to_point_fingerprint
  + vehicle_profile_hash
  + route_mode
  + traffic_mode
  + departure_bucket
```

There is no cross-request cache in this first release. A newer trusted current-trip context, including a different fresh start position, makes a later accept or activate `PLAN_STALE`. This erratum defines no tolerance radius.
