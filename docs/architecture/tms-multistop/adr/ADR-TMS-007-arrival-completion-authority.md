# ADR-TMS-007: Arrival and completion authority

## Status

Accepted. Architecture freeze. Implementation is not authorized.

## Context

Arrival and completion today are driver operational commands in `shipment-service`. Tracking stores position, freshness, and ETA to a single pickup or delivery. It does not complete the shipment. No geofence writer exists. The optimizer does not store proof of pickup or delivery.

## Decision

`shipment-service` owns arrival truth and completion truth. The assigned driver's arrive and confirm commands are authoritative. An operator in `operating_tenant_id` may override with a reason and an audit row. That override does not require the participant shipment to share the carrier tenant. Tracking may suggest approach or arrival and may store live ETA. Those signals do not set stop status, cargo evidence, or shipment status. The optimizer never completes a stop. `planned_arrival` on the stop is not replaced by live ETA.

```text
ARRIVAL_TRUTH_OWNER=shipment-service
COMPLETION_TRUTH_OWNER=shipment-service
ARRIVAL_DETECTION_MODEL=DRIVER_COMMAND_AUTHORITATIVE_TRACKING_ADVISORY
PLANNED_ARRIVAL_SEPARATE_FROM_LIVE_ETA=YES
```

Precedence when several signals exist: a recorded driver or operator command on the stop; then an audited operator override; tracking remains advisory beside those facts and does not overwrite them.

## Consequences

GPS loss degrades tracking freshness and does not block manual completion. Control Tower observes completion. It does not author it.
