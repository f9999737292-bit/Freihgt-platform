# ADR-NET-019: Route stop, action, and leg

## Status

Proposed. Discovery only. Not accepted. Implementation is not authorized.

## Context

A load is not a stop. Several loads can share a warehouse. NLO-0.3E refuses to order stops. The routing port already returns road distance, duration, and fingerprints, and it can return geometry.

## Decision

`RouteStop` is `location_id` plus ordinal. Roles are `START`, `CARGO`, and `END`. Cargo work is `RouteStopAction` of type `PICKUP` or `DELIVERY`. Several actions may share one stop. Pickup and delivery may share a stop only when `location_id` matches, and the pickup action of a load is ordered before its delivery.

```text
MULTIPLE_ACTIONS_PER_STOP=YES
BREAK_IN_V0_4_PLANNER=NO
WAYPOINT_INSERTION_IN_V0_4_PLANNER=NO
```

`RouteLeg` connects adjacent stops and stores road `distance_m`, `duration_s`, provider, fingerprints, traffic mode, `calculated_at`, and `expires_at`. Geometry and raw provider bodies are not stored in v0.4. Unknown road distance is not zero.

`RouteCapacitySnapshot` is persisted as an audit of the calculation after every cargo action. It is derived during planning and not mutated afterward. Onboard overlap is defined in `NLO_0_4A_MULTI_STOP_ALGORITHM.md`. `EvaluateGroupageItems` is reused.

```text
GROUPAGE_ENGINE_REUSED=YES
CAPACITY_CHECK_AFTER_EVERY_STOP=YES
ONBOARD_INTERVAL_COMPATIBILITY=YES
```

## Consequences

The proposed ERD is in `NLO_0_4A_ROUTE_PLAN_MODEL.md`. This ADR does not create tables.
