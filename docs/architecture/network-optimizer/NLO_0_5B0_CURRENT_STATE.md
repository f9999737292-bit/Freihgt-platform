# NLO-0.5B0 current state

Verified on `origin/main` `4c8b22e8d447b7c80a3ca73a427a0a9630f916be`. No backhaul runtime is added here.

```text
DIRECTIONAL_CORRIDOR=IMPLEMENTED_FOR_NEXT_LOAD
ROUTE_ELLIPSE=IMPLEMENTED_FOR_NEXT_LOAD
BACKHAUL_RUNTIME=NO
```

## Search

`SearchNextLoad` loads marketplace rows with `ListMarketplaceLoads(..., 100000, 0)`. There is no discovery cap. `candidate_limit` only truncates the HTTP response. The pool is sorted by load id.

Corridor and ellipse prefilters run before road calls. Haversine is the radius prefilter only. `HaversineCanonicalRoad` is false. A corridor pickup behind the release is `BACKTRACK_DIRECTION_REJECTED`. Missing road facts become `ROAD_DISTANCE_UNKNOWN` and are not stored as zero.

## Provider calls for N survivors

Both corridor and ellipse request two routes: release to target for the corridor line, and the same pair again as the ellipse baseline. Deadhead is one origin to N pickups. Direction is up to two points per load toward the target. Ellipse also requests pickup to delivery.

`SyncMatrixLimit` is 25. `BatchMatrix` splits destinations. More than 25 origins in one call is `ROUTING_INVALID_RESPONSE`. The loaded-leg call is a square matrix: a full page is 25 by 25 cells, and only the diagonal is kept.

| N | Corridor matrix | Ellipse matrix | Routes | Ellipse cells | Ellipse timeout ceiling |
| --- | --- | --- | --- | --- | --- |
| 10 | 2 | 3 | 2 | 130 | 25s |
| 25 | 3 | 4 | 2 | 700 | 30s |
| 50 | 6 | 8 | 2 | 1400 | 50s |
| 100 | 12 | 16 | 2 | 2800 | 90s |
| 250 | 30 | 40 | 2 | 7000 | 210s |
| 500 | 60 | 80 | 2 | 14000 | 410s |
| 1000 | 120 | 160 | 2 | 28000 | 810s |

The ceiling is call count times the 5 second HTTP client timeout. It is not measured 2GIS latency.

## Adapter

`cmd/server/main.go` wires `twogis.New` when `BNO_ROUTING_PROVIDER=2GIS`. The client timeout is 5 seconds. Calls are sequential. There is no retry loop and no circuit breaker. HTTP 404 is `ROUTE_NOT_FOUND`. Status 401, 403, and 5xx are `ROUTING_PROVIDER_UNAVAILABLE`. Other 4xx, including 429, are `ROUTING_INVALID_RESPONSE`. A timeout is `ROUTING_TIMEOUT`.

`MemoryCache` exists. The search path and process wiring do not use it.

No commercial rate card, vendor SLA, or staging approval is in the repo.

```text
PROVIDER_COMMERCIAL_LIMITS=UNKNOWN
PROVIDER_STAGING_APPROVAL=NOT_CONFIRMED
EXTERNAL_PROVIDER_BENCHMARK=BLOCKED
```
