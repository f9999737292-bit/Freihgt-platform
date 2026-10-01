# NLO-0.5A routing policy

The routing port stays vendor-neutral. The first adapter remains 2GIS. Staging use of that adapter is not approved.

```text
ROUTING_PROVIDER_PORT_REUSED=YES
CURRENT_ROUTING_ADAPTER=2GIS
STAGING_PROVIDER_APPROVAL_REQUIRED=YES
HAVERSINE_EXECUTABLE_DISTANCE_ALLOWED=NO
HAVERSINE_ROUTE_ELLIPSE_ALLOWED=NO
UNKNOWN_ROAD_DISTANCE_FAIL_CLOSED=YES
ROUTING_PROVIDER_FAILURE_POLICY=ROAD_DISTANCE_UNKNOWN
```

Provider unavailable, timeout, invalid response, and route-not-found all become `ROAD_DISTANCE_UNKNOWN` for that candidate. They do not fall back to straight-line distance. `AllowUnknownRoadDistance` must not make an executable plan. An unknown required road fact blocks activation the same way an unknown service duration does.

## Batch policy

Synchronous matrix batches stay at the implemented limit of 25. Origins above 25 are invalid for one call. Destination batches are deterministic.

These next-load limits are not in the current code. They are not assigned a production number here:

```text
MAX_CANDIDATES_BEFORE_ROAD_ROUTING=BENCHMARK_WAVE
MATRIX_BATCH_SIZE=25
MAX_MATRIX_CALLS_PER_SEARCH=BENCHMARK_WAVE
SEARCH_TIME_BUDGET_POLICY=BENCHMARK_WAVE
```

NLO-0.3E's pool of 10 and 5 second watchdog are consolidation bounds. They are not copied onto backhaul as if they had been measured for road search.

## Cache

A cached route is usable only before `ExpiresAt`. `CURRENT` is 15 minutes. Statistical and other modes are 24 hours. A stale entry is a miss. The miss calls the provider again. The cache key is the route fingerprint: provider, endpoints, vehicle-profile hash, route mode, traffic mode, and departure bucket. Provider secrets are not stored.

## Audit fields

A reproducible decision keeps provider name, provider route id when the adapter returns one, route mode, traffic mode, calculated time, expiry, road distance, road duration, search-policy fingerprint, capacity version, load version, service-duration policy version when a plan is built, compatibility and rule fingerprints, and the score explanation. It does not keep the API key.

## Failure

| Condition | Result |
| --- | --- |
| Provider down, timeout, invalid body, route not found | `ROAD_DISTANCE_UNKNOWN` |
| Stale cache | miss, then the same failure policy |
| Unknown capacity fact | `CAPACITY_FACT_UNKNOWN` |
| Stale predicted ETA | existing prediction rejection |
| Missing ETA where prediction requires it | prediction fails closed |
| Missing location coordinates | `DIRECTION_TARGET_GEO_UNKNOWN` or no release point |
| Missing service duration on an executable plan | `SERVICE_DURATION_UNKNOWN` |
| No rate | `UNPRICED`, not a fake contribution |

No executable plan is emitted while a required road fact is unknown.
