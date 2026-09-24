# ADR-NET-005: Transport leg and route model

## Status

Proposed — architecture freeze v0.1. Implementation not authorized.

## Context

`TransportOrder` and `Shipment` each store one origin location and one destination location. There is no leg or stop table. Several orders cannot share a movement without a new model. Existing shipment APIs must keep working.

## Decision

Planning uses `RoutePlan`, `RouteLeg`, and `RouteStop` (`PICKUP`, `DELIVERY`, `HUB`, `DEPOT`, `BREAK`). Many cargo units may reference one leg. The plan is not a shipment. Single-leg acceptance may later call the existing create-shipment path. Multi-stop and multi-shipper plans cannot become `ACTIVE` until an authorized evolution adds an optional `route_leg_id` on shipment or an equivalent compatible execution model. This freeze does not alter `transport.shipments`.

## Consequences

UI and offers must not promise an executable multi-stop shipment before that evolution.
