# NLO-0.5A roundtrip model

Docs only. A roundtrip in this release is the same one-load insertion as backhaul, bounded by a return target. It is not NLO-0.9.

```text
ROUNDTRIP_FIRST_RELEASE_SCOPE=ONE_BACKHAUL_LOAD_TOWARD_TARGET
RETURN_TARGET=policy target_location_id
ROUTE_ELLIPSE=EXISTING_FOUR_LEG_ROAD_INSERTION
UNRESTRICTED_CHAIN_SEARCH=NO
LIVE_REOPTIMIZATION_INCLUDED=NO
NLO_0_5_FIRST_RELEASE_BOUNDED=YES
```

## Shape

```text
release -> pickup -> delivery -> target
```

Release to target is the empty baseline. The load is one published opportunity whose pickup is at or near the release and whose delivery moves toward the target. Exact `delivery == target` is not required. Exact `B -> A` origin-destination equality is not required.

The implemented increase is:

```text
road(release, pickup) + road(pickup, delivery) + road(delivery, target) - road(release, target)
```

Discovery compared this with a three-term pickup detour `road(release, pickup) + road(pickup, target) - road(release, target)`. That three-term form omits the loaded delivery leg. NLO-0.5 keeps the four-term form already implemented in BNO-0.1C1. All four terms are road kilometres. Haversine is not allowed in any term.

`max_route_increase_km` is the policy hard maximum. A missing term is `ROAD_DISTANCE_UNKNOWN`, not zero, and the candidate is not executable.

## What is out

- a second or later load in the same search
- an open chain of deliveries
- live reoptimization of an active execution
- a successor revision of completed or future TMS stops

NLO-0.9 remains the chain and `AT_RISK` stage. NLO-0.6 remains regional open and closed routing.

## After acceptance

An accepted roundtrip plan reuses the NLO-0.4 route plan, accept, activate, and TMS projection handshake. NLO does not own the execution. Planning before activation is the first release. A truck already in execution can be a capacity source through current-trip context. That read does not change completed history, onboard evidence, or TMS return disposition.
