# ADR-NET-010: External routing abstraction

## Status

Proposed — architecture freeze v0.1. Implementation not authorized.

## Context

The repo stores lat/lon and a Haversine helper. It has no PostGIS, geohash, matrix, or traffic feed. Straight-line distance is not a road route.

## Decision

Ports: `RoutingProvider`, `DistanceMatrixProvider`, `GeocodingProvider`, `TrafficProvider`, `CityRulesProvider`. Geo candidate search may use proximity. Executable distance, deadhead, and travel time require a road result or an audited manual distance. If the routing provider is down, do not emit an executable plan that depends on it. No vendor SDK is chosen here.

## Consequences

A future adapter can be replaced without changing match stages. Using Haversine as kilometres in an offer is a defect.
