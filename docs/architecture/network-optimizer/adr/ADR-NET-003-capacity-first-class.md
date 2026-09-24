# ADR-NET-003: Capacity as a first-class concept

## Status

Proposed — architecture freeze v0.1. Implementation not authorized.

## Context

`transport.vehicles` stores plate, equipment, weight, and volume for a carrier company. It has no availability window, confidence, or remaining space over time. Trailers do not exist. Carriers will not always type empty trucks in by hand.

## Decision

`Capacity` is a time-bounded aggregate. Sources include `MANUAL`, `CURRENT_SHIPMENT_PREDICTION`, `TELEMATICS`, `DRIVER_APP`, `CARRIER_TMS`, `API`, and `FLEET_PLAN`. `PredictedCapacity` records method `RULE_BASED`, `STATISTICAL`, `ML`, or `EXTERNAL`. This freeze implements none of them. Vehicle and driver masters stay in shipment-service. Marketplace matching may omit `driver_id` and `vehicle_id` until assignment. Private fleet binds them on the plan.

## Consequences

Publishing a vehicle master does not publish capacity. Low confidence cannot auto-offer.
