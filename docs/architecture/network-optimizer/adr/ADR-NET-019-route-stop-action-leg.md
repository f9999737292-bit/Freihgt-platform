# ADR-NET-019: Route stop, action, and leg

## Status

Accepted. Architecture freeze. Implementation is not started.

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

The proposed ERD is in `NLO_0_4A_ROUTE_PLAN_MODEL.md`. This ADR does not create tables.
