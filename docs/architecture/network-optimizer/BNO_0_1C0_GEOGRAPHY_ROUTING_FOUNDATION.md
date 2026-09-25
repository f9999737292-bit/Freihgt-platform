# BNO-0.1C0 Geography and Routing Foundation

This release removes the geography and routing blockers found before Next Load Matching. It does not create match candidates, scores, TOP-N, recommendations, or an automatic search.

## Location ownership

`transport.locations` in transport-order-service remains the only location master. BNO stores a planning snapshot and a `location_id` reference. It does not copy address lines, contacts, or private operational notes, and it does not create `network_optimizer.locations`.

`GET /internal/v1/locations/{locationId}` returns `id`, `country_code`, `region`, `city`, `lat`, `lon`, `timezone`, `status`, and `version` for the tenant in `X-Tenant-ID`. The caller must present `X-Internal-Service-Token`. A location owned by another tenant is `NOT_FOUND`.

Source-backed loads resolve pickup and delivery through tenant-scoped planning reads:

- `GET /internal/v1/transport-orders/{id}/planning-locations`
- `GET /internal/v1/shipments/{id}/planning-locations`

Caller coordinates do not override that snapshot. A caller-supplied location id that disagrees with the source is `NOT_FOUND`.

## Capacity location identity

`PredictedCapacity.destination_location_id` is copied onto the capacity row as `location_id`. Manual capacity may send `location_id`. That id is resolved in the caller tenant before it is stored. Latitude and longitude on that row then come from the location snapshot.

## SearchGeo and DisplayGeo

SearchGeo is the stored place: `location_id`, coordinates, and coarse country, region, and city. The owner read keeps it.

DisplayGeo for `MARKETPLACE` keeps the exact place allowed by the current visibility contract.

DisplayGeo for `ANONYMIZED_MARKETPLACE` omits `location_id`, latitude, longitude, facility label, address line, and postal code. It returns `country_code`, `region`, and `city` only when those values are known. A missing city is not invented from a region, and a missing region is not invented from a country. Anonymized loads remain searchable on the server because SearchGeo is preserved on the owner record.

An anonymous result must not reveal exact position through raw coordinates, a location id, an address, or a facility label. This release does not publish an exact deadhead distance on an anonymous result. A later match release may show a rounded distance until a disclosure policy allows the exact location.

## Routing

`RoutingProvider` is provider-neutral:

- `Route` returns provider, optional provider route id, distance in metres, duration in seconds, normalized LineString geometry, calculation time, traffic mode, and route mode.
- `Matrix` returns the same distance and duration facts per cell.

The first adapter is 2GIS, selected with `BNO_ROUTING_PROVIDER=2GIS`. `BNO_2GIS_ROUTING_BASE_URL` and `BNO_2GIS_API_KEY` come from the environment. The API key is not written to logs or domain records. Network-optimizer core does not parse 2GIS JSON.

BNO traffic modes stay provider-neutral. `CURRENT` means current road conditions and does not send a planned departure. `STATISTICAL` means time-based planning and sends `DepartureAt` to the provider. The 2GIS adapter translates those modes to `jam` and `statistics`. It does not send the internal names.

A truck request stays `transport=truck` when some dimensions are unknown. Known values are mapped to the provider fields `mass`, `axle_load`, `height`, `width`, `length`, and `dangerous_cargo`. Mass and axle load are converted from kilograms to tonnes. Maximum permitted mass is not derived from gross weight. Routing API v7 places those fields under `params.truck`, asks for `output=detailed`, and sends a statistical departure as `utc`. Distance Matrix API 2.0 uses `truck_params`. Its single `type` field is `jam` for fastest current traffic, `statistics` with `start_time` for fastest statistical planning, or `shortest`. A shortest matrix request does not also claim current or statistical traffic, because the provider field cannot carry both. A statistical matrix request without `DepartureAt` fails closed. A failed matrix cell is an error and is not a zero road distance.

The synchronous 2GIS Distance Matrix accepts at most 25 sources or 25 targets in one request. A later next-load match must batch candidate destinations or use the async Matrix API. This release does not batch candidates and does not call the async API.

Vehicle height, width, length, gross weight, axle load, and dangerous-cargo state are sent only when known. Unknown stays unknown. If the provider still applies its own vehicle default, the route and matrix results record `ProviderDefaultUsed`.

Failures are `ROUTING_PROVIDER_UNAVAILABLE`, `ROUTE_NOT_FOUND`, `ROUTING_TIMEOUT`, and `ROUTING_INVALID_RESPONSE`. None of them falls back to Haversine.

The route cache key includes origin, destination, vehicle-profile hash, route mode, traffic context, and provider. Live traffic results expire. `calculated_at` and `expires_at` stay on the result so a later decision can be reproduced from provider, route mode, traffic mode, fingerprint, distance, and duration. The proprietary provider body is not the domain model.

## Distance semantics

These values stay on separate fields:

- `straight_line_distance_km` is a cheap prefilter only
- `corridor_lateral_distance_km`
- `corridor_forward_progress_km`
- `road_deadhead_km`
- `road_deadhead_minutes`
- `loaded_road_distance_km`
- `route_increase_km`

Haversine is not road distance, road duration, or deadhead. `HAVERSINE_CANONICAL_ROAD_DISTANCE=NO`.

Road deadhead is the truck road route from the release point to the pickup. A hard limit compares that road result with the effective maximum. `preferred_deadhead_km` does not reject a candidate. There is no match score in this release.

Unknown road distance is not zero and is not a straight-line value. The default `allow_unknown_road_distance` is false, and a missing route still cannot become an executable deadhead.

## Search modes

`NextLoadSearchPolicy` accepts `RADIUS`, `DIRECTIONAL_CORRIDOR`, and `ROUTE_ELLIPSE`. This release validates the configuration. It does not rank candidates.

Directional corridor, using a release in Ekaterinburg and a target in Moscow:

- `forward_search_km=500` is how far forward along the desired route a pickup may lie
- `corridor_deviation_km=50` is the maximum sideways distance from that corridor
- `max_deadhead_km=80` is the maximum road reposition from the release point to the pickup

A pickup behind the release point is `BACKTRACK_DIRECTION_REJECTED`.

Route ellipse uses road distance only:

`route_increase = road(release, pickup) + road(pickup, target) - road(release, target)`

A future candidate is inside the ellipse when `route_increase` is within `max_route_increase_km`. Straight-line geometry is not that calculation.

`target_location_id` references `transport.locations`. A direction without coordinates is `DIRECTION_TARGET_GEO_UNKNOWN`.

## Policy ownership and time

BNO owns search preferences. They are not fields on the shipment vehicle master. Levels are `CARRIER_DEFAULT`, `CAPACITY_OVERRIDE`, and `SEARCH_REQUEST`.

The effective hard maximum is the minimum of the carrier, capacity, and request hard limits. A request may tighten a limit. It does not widen it. Example: carrier 150 km, capacity 100 km, request 120 km, effective 100 km.

Pickup arrival is `predicted_available_at + road_deadhead_minutes`. Arrival after `pickup_window.end` is `PICKUP_WINDOW_MISSED`. Arrival before the window start produces waiting minutes and no score.
