# Current-state inventory

Baseline `0fc6a7979ca5ea0cbbbd50ff5770bbbf22304a5c`. Inventory is from repository code and migrations. Highest applied migration number in tree is `000085`. No `000086` file exists. This discovery creates none.

```text
SHIPMENT_MULTI_STOP_EXISTS=NO
SHIPMENT_STOP_MODEL_EXISTS=NO
DRIVER_STOP_MODEL_EXISTS=NO
DRIVER_TASK_MODEL_EXISTS=PARTIAL
TRACKING_STOP_AWARE=PARTIAL
CONTROL_TOWER_STOP_AWARE=NO
SLOT_BOOKING_REUSABLE=PARTIAL
```

## Shipment aggregate

`transport.shipments` and `domain.Shipment` are a single origin and a single destination.

| Field | Meaning today |
| --- | --- |
| `OriginLocationID`, `DestinationLocationID` | One pickup site and one delivery site |
| `CargoID` | One optional cargo pointer on the shipment row |
| `PlannedPickupAt`, `PlannedDeliveryAt` | One pair of plan times |
| `ActualPickupAt`, `ActualDeliveryAt` | One pair of actual times |
| `DriverID`, `VehicleID` | One assigned driver and one vehicle |
| `Version` | Optimistic concurrency for the shipment row |

`allowedStatusTransitions` in `services/shipment-service/internal/domain/shipment.go` is a single chain:

```text
CARRIER_ASSIGNED
→ ACCEPTED_BY_CARRIER
→ VEHICLE_ASSIGNED
→ DRIVER_ASSIGNED
→ PICKUP_SLOT_BOOKED
→ IN_PICKUP
→ LOADED
→ IN_TRANSIT
→ ARRIVED_AT_CONSIGNEE
→ UNLOADING
→ DELIVERED
→ DELIVERY_CONFIRMED
→ DOCUMENTS_COMPLETED
→ READY_FOR_BILLING
→ INCLUDED_IN_BILLING_REGISTER
→ FINANCIALLY_CLOSED
```

`DELIVERY_SLOT_BOOKED` is a named constant. It is not a target in `allowedStatusTransitions` or `manualStatusTransitions`. Assignment targets (`ACCEPTED_BY_CARRIER`, `VEHICLE_ASSIGNED`, `DRIVER_ASSIGNED`) are not reachable through generic status patch. Cancel is forbidden from `DELIVERED` onward.

There is no `shipment_stop` table, no ordinal, and no stop repository.

`SHIPMENT_MULTI_STOP_EXISTS=NO`. `SHIPMENT_STOP_MODEL_EXISTS=NO`.

## Cargo execution evidence

`transport.shipment_cargo_execution_evidence` (migration `000082`) is append-only. Update and delete triggers reject mutation. States are `PLANNED`, `PICKED_UP`, `CONFIRMED_ONBOARD`, `UNLOADED`.

`CargoEvidenceIntentForDriverEvent` writes `CONFIRMED_ONBOARD` on `PICKUP_COMPLETED` and `UNLOADED` on `DELIVERY_COMPLETED`. `ResolveOnboardCargo` keeps the latest row per cargo. Shipment status is not onboard proof.

This is shipment-scoped evidence. It is not a stop or an action row. It is the evidence model multi-stop actions must keep writing. Already-onboard cargo must not gain a synthetic pickup.

## Transport order execution

`transport-order-service` remains the commercial order. `shipment-service` `order_execution.go` links an awarded order to one shipment and exposes readiness (`carrier accepted`, `driver assigned`, `vehicle assigned`). Execute copies one planned pickup and one planned delivery. It does not create stops.

## Driver operations

`driver_operations.go` maps one event to one shipment status:

| Driver command | Shipment status |
| --- | --- |
| `ARRIVED_AT_PICKUP` | `IN_PICKUP` |
| `LOADING_STARTED` | informational; status unchanged |
| `PICKUP_COMPLETED` | `LOADED` |
| `DEPARTED_PICKUP` | `IN_TRANSIT` |
| `ARRIVED_AT_DELIVERY` | `ARRIVED_AT_CONSIGNEE` |
| `UNLOADING_STARTED` | `UNLOADING` |
| `DELIVERY_COMPLETED` | `DELIVERED` |

`AllowedDriverMilestoneActions` exposes the next command from the current shipment status. The driver mobile app (`apps/driver-mobile/src/utils/milestones.ts`) mirrors that map. The detail view shows origin, destination, and the two planned times.

Idempotency is `transport.driver_operation_idempotency`: tenant, driver, operation type, key, stored response. The mobile client generates an operation id and refuses submit while offline (`OfflineBanner`, pages set an offline error). There is no durable offline queue and no stop ordinal.

Exceptions and delays are shipment-scoped (`driver_reported_exception`, `driver_reported_delay`) with reason codes already used by operations (`TRAFFIC`, `VEHICLE_BREAKDOWN`, `LOADING_DELAY`, `UNLOADING_DELAY`, `ROUTE_BLOCKED`, `CUSTOMER_UNAVAILABLE` / `CUSTOMER_DELAY`, `CARGO_ISSUE`, `DOCUMENT_ISSUE`, `ACCIDENT`, `OTHER`).

`DRIVER_STOP_MODEL_EXISTS=NO`.

## Driver task inbox

`transport.driver_task` (migration `000034`) is an inbox, not a route.

| Exists | Missing for stop execution |
| --- | --- |
| Types `REQUEST_DELAY_REASON`, `REQUEST_STATUS_CONFIRMATION`, `REQUEST_ARRIVAL_CONFIRMATION`, `REQUEST_DOCUMENT_ACTION`, `GENERAL_OPERATIONAL_NOTICE` | No stop, ordinal, location, or cargo action |
| Status `PENDING → DELIVERED → READ → ACKNOWLEDGED → COMPLETED`, plus `EXPIRED`, `CANCELLED` | That lifecycle is acknowledgement, not arrive/service/complete |
| Idempotency key and source-event unique indexes | Keys identify a notice, not an activation |
| Outbox `driver.task_created`, `driver.task_completed`, `driver.task_expired`, `driver.task_cancelled` in the same transaction as the task row | No stop progress event |
| Sources `SYSTEM`, `CONTROL_TOWER`, `OPERATOR` | No RoutePlan source |

`DRIVER_TASK_MODEL_EXISTS=PARTIAL`. The inbox, device registry, idempotency, and outbox transaction are reusable. The task type and status machine are not a stop task.

## Tracking

`tracking-service` owns position and ETA. Tables from migrations `000026`, `000027`, `000028`:

| Table | Grain |
| --- | --- |
| `tracking.shipment_tracking_binding` | One shipment, vehicle, driver, provider device |
| `tracking.shipment_tracking_state` | Freshness `unknown/fresh/stale/lost`, quality, status `not_configured` through `ended` |
| Location events | `shipment_id`, lat/lon, `recorded_at`, dedup key, source `vehicle_telematics`, `driver_mobile`, `carrier_api`, `manual`, `system_import` |
| `tracking.shipment_eta_state` | Target `pickup` or `delivery` only |
| `tracking.shipment_slot_revision` / `shipment_slot_state` | Slot type `pickup` or `delivery` |

No `stop_id`, no ordinal, no geofence writer that completes a shipment. `TRACKING_STOP_AWARE=PARTIAL`: shipment binding, freshness, dedup, and a two-target ETA exist. Stop sequence does not.

## Control Tower

`control-tower-read-model-service` consumes `shipment.created`, `shipment.status.changed`, `shipment.cancelled`, `shipment.ready_for_billing`, `shipment.documents_completed`, and `shipment.financially_closed`. The projection stores one status and the four pickup/delivery timestamps. Version gaps are detected on the shipment aggregate version.

Driver ingestion accepts `driver.arrived_at_pickup`, `driver.arrived_at_delivery`, `driver.delivery.completed`, `driver.delay.reported`, `driver.problem.reported`, and maps legacy `driver.exception_reported` to `driver.problem.reported`. Cases, risk, and automation hang off shipment status and driver problems.

`CONTROL_TOWER_STOP_AWARE=NO`. Consumers and case workflow are reusable. Nothing projects an ordered stop.

## Slot booking

No slot-booking service and no reservation table. `PICKUP_SLOT_BOOKED` is a manual shipment transition from `DRIVER_ASSIGNED`. Tracking stores observed windows (`proposed`, `booked`, `confirmed`, `cancelled`, `completed`, `missed`) for `pickup` and `delivery` on one shipment. NLO-0.4A states the optimizer does not reserve a slot. `SLOT_BOOKING_REUSABLE=PARTIAL`: the tracking slot revision is reusable as a window record for a future stop reference. It cannot represent a sequence of stops, and it is not a booking authority that execution must obey blindly.

## Outbox

`transport.shipment_event_outbox` is written in the same database transaction as the shipment mutation (`insertOutboxRow` on the driver-operation and driver-task transactions). Aggregate type is `SHIPMENT`. Status events use `shipment.created` and `shipment.status.changed`. Publish is asynchronous after commit. Replay exists.

## RoutePlan as it exists today

`network-optimizer-service` persists `RoutePlan`, `RouteStop`, `RouteAction`, legs, capacity snapshots, and dependencies (migration `000085`). `execution_supported` stays false. `route_plan_activations` does not exist. Accept and activate are NLO-0.4C and are not implemented. This inventory does not change those tables.

Planning facts already stored, and therefore available to a future contract, include `route_plan_id`, `version`, `shipment_id`, `shipment_version`, `evaluation_fingerprint`, `context_fingerprint`, `planning_mode`, `supersedes_plan_id`, stop `ordinal`, `stop_role`, `point_kind`, `location_id`, coordinates, `planned_arrival`, `planned_departure`, `service_duration_seconds`, and action `action_type`, `action_ordinal`, subject id and version, `source_shipment_id`, and `evidence_state`.
