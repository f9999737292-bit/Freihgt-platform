# Tracking integration

```text
TRACKING_OWNER=tracking-service
TRACKING_INTEGRATION_REUSABLE=PARTIAL
PLANNED_ARRIVAL_SEPARATE_FROM_LIVE_ETA=YES
ARRIVAL_DETECTION_MODEL=DRIVER_COMMAND_AUTHORITATIVE_TRACKING_ADVISORY
```

## Current behavior

`tracking-service` binds a provider device to a shipment, vehicle, and driver. Location events store coordinates, speed, heading, accuracy, source, quality, and a dedup key. State is `not_configured`, `awaiting_data`, `active`, `stale`, `lost`, or `ended`. Freshness is `unknown`, `fresh`, `stale`, or `lost`.

ETA observations target `pickup` or `delivery` only (`TargetPickup`, `TargetDelivery`). Sources include provider, carrier, driver, operator, and calculated. ETA has its own freshness and quality. Slot arrival projection (`early`, `on_time`, `at_risk`, `projected_miss`, `missed`) compares that ETA to the shipment's single slot window.

Nothing in tracking names a stop ordinal. GPS ownership stays in this service. Shipment-service does not store a position stream.

## Future use

| Need | Owner | Attachment |
| --- | --- | --- |
| Current position | tracking-service | Existing shipment binding |
| ETA to the next open stop | tracking-service | New target type `execution_stop` plus `execution_stop_id`. The old `pickup` and `delivery` targets remain for shipments with no plan |
| Arrival suggestion | tracking-service | Advisory event only |
| Route deviation | tracking-service | Advisory, compared with the planned leg the execution contract copied as coordinates of the current and next stop. Raw provider geometry is not copied |
| Stop delay | shipment-service | Driver delay command, optionally annotated with tracking freshness |
| Freshness | tracking-service | Existing stale and lost transitions |

Shipment-service tells tracking which stop id is current when the current stop changes, by a trusted internal call or by the stop outbox event. Tracking does not read optimizer tables.

## ETA

```text
planned_arrival = copied onto the execution stop at projection, never overwritten
live_eta = tracking ETA observation for that stop
```

Both may be shown. Live ETA does not replace planning evidence. A driver-reported `NewETA` on the existing delay command is a third, driver-asserted value. It updates the delay record. It does not update `planned_arrival` and it does not become the tracking observation unless tracking ingests it through its existing driver ETA source.

## Arrival detection

There is no geofence writer in the repository today. A future geofence may emit `tracking.stop.approaching` or a suggested arrival. It must not set stop status, cargo evidence, or shipment status. The authoritative arrive command is the driver's `ArriveStop`. See ADR-TMS-007.

If GPS is missing, tracking freshness becomes `stale` or `lost` through the existing detector. Manual arrive still works.
