# NLO-0.5A current-state inventory

Discovery only. Baseline `00b53de9db50cdc1e875df95b8ba7ef2daa0fe8a`. No product code.

```text
NLO_0_5A=DISCOVERY
PRODUCT_CODE_CHANGED=NO
HAVERSINE_CANONICAL_ROAD_DISTANCE=NO
```

## Routing port

`services/network-optimizer-service/internal/routing/provider.go` defines `Provider` with `Route` and `Matrix`. Traffic modes are `CURRENT` and `STATISTICAL`. Route modes are `FASTEST` and `SHORTEST`. Errors are `ROUTING_PROVIDER_UNAVAILABLE`, `ROUTE_NOT_FOUND`, `ROUTING_TIMEOUT`, and `ROUTING_INVALID_RESPONSE`.

`VehicleProfile` carries gross weight, height, width, length, axle load, and dangerous-cargo. `Complete()` is true only when every field is set. The profile hash is part of the request fingerprint. Missing fields are not filled with zero.

`RouteResult` stores provider, optional provider route id, distance metres, duration seconds, geometry, calculated time, expiry, traffic mode, route mode, and request fingerprint. `ProviderDefaultUsed` records a provider default. It is not a Haversine substitute.

## 2GIS adapter

`internal/routing/twogis/adapter.go` is the only adapter. `ProviderName()` returns `2GIS`. Configuration is `BNO_ROUTING_PROVIDER`, `BNO_2GIS_ROUTING_BASE_URL`, and `BNO_2GIS_API_KEY`. An empty URL or key returns `ROUTING_PROVIDER_UNAVAILABLE`. The HTTP client timeout is 5 seconds. The adapter posts truck routing to `/routing/7.0.0/global`. `CURRENT` maps to provider `jam`. `STATISTICAL` maps to `statistics` and sends `DepartureAt`. The API key is not written to domain records.

```text
CURRENT_ROUTING_ADAPTER=2GIS
CURRENT_PROVIDER_CONFIG=BNO_ROUTING_PROVIDER,BNO_2GIS_ROUTING_BASE_URL,BNO_2GIS_API_KEY
CURRENT_PROVIDER_PRODUCTION_READY=NO
CURRENT_PROVIDER_STAGING_ALLOWED=NOT_DOCUMENTED
CURRENT_PROVIDER_BLOCKERS=Q4_STAGING_VENDOR_APPROVAL,COMMERCIAL_TERMS_NOT_RECORDED,SLA_NOT_RECORDED
STAGING_PROVIDER_APPROVAL_REQUIRED=YES
```

Q4 in `OPEN_QUESTIONS.md` is still open. This inventory does not treat the adapter as staging approval.

## Cache and matrix

`MemoryCache` keys a `RouteResult` and rejects it when `ExpiresAt` is not after now. `CURRENT` expires after 15 minutes. Other modes expire after 24 hours. There is no Redis route cache in this service.

`SyncMatrixLimit` is 25 sources and 25 targets. `BatchMatrix` splits destinations into chunks of 25. More than 25 origins is `ROUTING_INVALID_RESPONSE`. Search code in `internal/service/search.go` calls that batch helper. It does not cap the number of batches or the search duration. NLO-0.3E's 5 second watchdog and `MAX_CANDIDATE_POOL=10` apply to same-origin consolidation, not to next-load search.

## Search modes already implemented

`NextLoadSearchPolicy` accepts `RADIUS`, `DIRECTIONAL_CORRIDOR`, and `ROUTE_ELLIPSE`. Haversine is the cheap prefilter only (`HaversineCanonicalRoad = false`).

Corridor geometry comes from the truck route release to `target_location_id`. Negative forward progress is `BACKTRACK_DIRECTION_REJECTED`. Lateral excess is `CORRIDOR_DEVIATION_EXCEEDED`. Forward excess is `FORWARD_SEARCH_EXCEEDED`. City names are not the direction test.

Road deadhead is a later truck route. Limits are `MAX_DEADHEAD_EXCEEDED` and `MAX_DEADHEAD_TIME_EXCEEDED`. A missing road fact is `ROAD_DISTANCE_UNKNOWN`. It is not stored as zero.

The implemented route-ellipse increase is:

```text
road(release, pickup) + road(pickup, delivery) + road(delivery, target) - road(release, target)
```

That is the BNO-0.1C1 insertion. A candidate over `max_route_increase_km` is `ROUTE_INCREASE_EXCEEDED`.

## What next-load search does not yet do

It does not add server-owned service duration into pickup arrival. Service duration is resolved later by the NLO-0.4 route plan. It does not create a backhaul aggregate, a roundtrip chain, or a second compatibility engine. Ranking reuses system score profiles: deadhead efficiency, capacity utilization, target proximity, waiting efficiency, pickup slack, and revenue. `MAX_CONTRIBUTION` is reserved. An unpriced revenue objective stays `UNPRICED` and is not ranked as zero.
