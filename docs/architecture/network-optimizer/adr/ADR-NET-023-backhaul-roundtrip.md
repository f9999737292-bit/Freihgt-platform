# ADR-NET-023: Backhaul and roundtrip

## Status

Proposed. NLO-0.5A architecture freeze. Implementation is not authorized.

```text
ADR_NET_023=PROPOSED
NLO_0_5A=DOCS_ONLY
IMPLEMENTATION_AUTHORIZED=NO
CONTROLLER_REVIEW_REQUIRED=YES
```

## Context

BNO-0.1C1 already searches one next load with `RADIUS`, `DIRECTIONAL_CORRIDOR`, and `ROUTE_ELLIPSE`. Road facts come from the routing port. Haversine is a prefilter. NLO-0.4 persists a route plan and hands an accepted activation to TMS. NLO-0.9 is still the chain and reoptimization stage. Q4, the staging routing vendor, is still open.

## Decision

NLO-0.5 first release is one published load from a release point toward `target_location_id`. Backhaul uses directional corridor and route ellipse. Roundtrip is that same insertion, not a chain. The route increase stays the implemented four road legs:

```text
road(release, pickup) + road(pickup, delivery) + road(delivery, target) - road(release, target)
```

The routing port and the 2GIS adapter stay as they are. Staging approval is still required. Unknown road distance does not become an executable plan. An accepted plan reuses NLO-0.4 activation. Live reoptimization is out.

```text
BACKHAUL_FIRST_RELEASE_SCOPE=ONE_LOAD_TOWARD_TARGET
ROUNDTRIP_FIRST_RELEASE_SCOPE=ONE_BACKHAUL_LOAD_TOWARD_TARGET
SUPPORTED_SEARCH_MODES=DIRECTIONAL_CORRIDOR,ROUTE_ELLIPSE
ROAD_DISTANCE_REQUIRED=YES
HAVERSINE_EXECUTABLE_DISTANCE_ALLOWED=NO
ROUTING_PROVIDER_PORT_REUSED=YES
CURRENT_ROUTING_ADAPTER=2GIS
STAGING_PROVIDER_APPROVAL_REQUIRED=YES
CAPACITY_SOURCES=MANUAL_AVAILABLE,PREDICTED_CAPACITY,CURRENT_TRIP_RESIDUAL
TARGET_LOCATION_OWNER=NEXT_LOAD_SEARCH_POLICY
MAX_CANDIDATE_BOUND=BENCHMARK_WAVE
MATRIX_BATCH_POLICY=SYNC_25
SEARCH_TIME_BUDGET_POLICY=BENCHMARK_WAVE
NLO_EXECUTION_OWNER=NO
TMS_EXECUTION_OWNER=YES
LIVE_REOPTIMIZATION_INCLUDED=NO
UNRESTRICTED_CHAIN_SEARCH=NO
```

## Consequences

Details are in `NLO_0_5A_BACKHAUL_MODEL.md`, `NLO_0_5A_ROUNDTRIP_MODEL.md`, `NLO_0_5A_ROUTING_POLICY.md`, and `NLO_0_5_IMPLEMENTATION_ROADMAP.md`. No migration and no product code accompany this ADR.
