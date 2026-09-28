# Domain model

```text
EXECUTION_AGGREGATE_OWNER=shipment-service
EXECUTION_ROUTE_MODEL=TRANSPORT_EXECUTION
NEW_SERVICE_REQUIRED=NO
REUSE_EXISTING_SHIPMENT_AGGREGATE=NO
MULTI_SHIPMENT_EXECUTION_SUPPORTED_BY_MODEL=YES
CROSS_SHIPPER_EXECUTION_SUPPORTED_BY_MODEL=YES
PARTICIPANT_SHIPMENT_OWNERSHIP_PRESERVED=YES
SHIPMENT_REHOMED_TO_CARRIER_TENANT=NO
CROSS_TENANT_RAW_SHIPMENT_SCAN=NO
DEPOT_START_WITHOUT_PREEXISTING_SHIPMENT_SUPPORTED=YES
EXECUTION_STOP_OWNER=shipment-service
STOP_ACTION_OWNER=shipment-service
DRIVER_TASK_OWNER=shipment-service
TRACKING_OWNER=tracking-service
CONTROL_TOWER_OWNER=control-tower-read-model-service
SLOT_WINDOW_RECORD_OWNER=tracking-service
SLOT_BOOKING_AUTHORITY=NOT_IMPLEMENTED
ROUTEPLAN_OWNER=network-optimizer-service
```

`REUSE_EXISTING_SHIPMENT_AGGREGATE=NO` means the shipment row is not the execution root. Shipment status, cargo evidence, and today's single-leg commands stay in `shipment-service`. No second service is introduced. A shipment has one origin and one destination, so it cannot parent a route that visits several shipments or that starts with no shipment at all.

## Relational boundary

| Entity | Parent | Role |
| --- | --- | --- |
| Shipment | itself | Coarse status for one commercial execution |
| TransportExecution | none | Driver route. `operating_tenant_id`, carrier, vehicle, driver, current revision |
| TransportExecutionRevision | TransportExecution | One activation projection. `ACTIVE` or `SUPERSEDED` |
| TransportExecutionParticipant | TransportExecution | Materialized shipment kept in `shipment_tenant_id`, plus cargo and provenance |
| TransportExecutionStop | TransportExecution | Stable stop facts. No RoutePlan source id |
| TransportExecutionRevisionStop | revision + stop | Membership and that revision's `source_route_plan_stop_id` |
| TransportExecutionAction | TransportExecutionStop | Stable pickup or delivery. No RoutePlan source id |
| TransportExecutionRevisionAction | revision + action | That revision's `source_route_plan_action_id` |
| DriverStopTask | TransportExecutionStop | Current or next work for the route driver |
| ShipmentCargoEvidence | Shipment | Append-only proof. Already exists |

Notice-style `DriverTask` rows stay. They are not the route sequence.

`operating_tenant_id` is the carrier execution scope. It is not a shipment owner tenant. Each participant keeps `shipment_tenant_id` from the shipment's original tenant. Projection does not clone that shipment into the carrier tenant.

`DEPOT_START` leaves `TransportExecution` without an anchor shipment. Participants appear only after each cargo subject has a trusted `shipment_id`, `shipment_tenant_id`, and `cargo_id`. `CURRENT_TRIP` stores the existing trip shipment as one participant. Other opted-in shippers are further participants in their own tenants. The trip shipment does not own the stop list.

## Diagram A — activation to projection

```mermaid
flowchart LR
  subgraph planOwner [network-optimizer-service]
    RoutePlan[RoutePlan]
    Pending[Activation PENDING_EXECUTION]
    RoutePlan --> Pending
  end
  subgraph execOwner [shipment-service]
    Route[TransportExecution]
    Revision[TransportExecutionRevision]
    Pending -->|"trusted projection command"| Revision
    Route --> Revision
  end
  Pending -->|"after durable execution ids"| Linked[Activation EXECUTION_LINKED]
```

## Diagram B — execution route

```mermaid
flowchart TD
  Route[TransportExecution]
  Revision[TransportExecutionRevision]
  Link[TransportExecutionRevisionStop]
  ActionLink[TransportExecutionRevisionAction]
  Stop[TransportExecutionStop]
  Action[TransportExecutionAction]
  Participant[TransportExecutionParticipant]
  Shipment[Shipment]
  Evidence[ShipmentCargoEvidence]
  Route --> Revision
  Route --> Stop
  Route --> Participant
  Revision --> Link
  Stop --> Link
  Revision --> ActionLink
  Action --> ActionLink
  Stop --> Action
  Participant --> Shipment
  Action --> Evidence
```

The stop's parent arrow goes to the route. The revision reaches the stop only through the link.

## Identity

```text
EXECUTION_STOP_ID_ALLOCATED_BY=shipment-service
EXECUTION_STOP_PARENT=TransportExecution
EXECUTION_STOP_ID_SHIPMENT_ROW_OWNED=NO
ROUTEPLAN_STOP_REFERENCE=PER_REVISION_LINK
STABLE_STOP_SOURCE_ID_REWRITTEN=NO
STABLE_ACTION_SOURCE_ID_REWRITTEN=NO
SUCCESSOR_ROUTEPLAN_STOP_LINEAGE_PRESERVED=YES
SUCCESSOR_ROUTEPLAN_ACTION_LINEAGE_PRESERVED=YES
ONBOARD_CARGO_DELIVERY_ONLY=YES
COMPLETED_STOP_IMMUTABLE=YES
COMPLETED_STOP_ROW_REPARENTED=NO
```

A stable stop or action has no RoutePlan source id to rewrite. The revision that introduces it stores that plan's stop and action ids on `TransportExecutionRevisionStop` and `TransportExecutionRevisionAction`. A successor revision adds another link with the successor plan's ids and membership `INHERITED_COMPLETED` or `INHERITED_IN_SERVICE`. Both lineages remain. The execution id does not change.

## Onboard cargo

If cargo evidence for a participant is `CONFIRMED_ONBOARD`, the projection creates a `DELIVERY` action only. It does not create a `PICKUP`. `UNLOADED` cargo produces no further action. A new load produces pickup and delivery only when the activated plan says so and the subject is already materialized to `shipment_id` and `cargo_id`. A raw `LOAD_OPPORTUNITY` does not. See `STOP_ACTION_MODEL.md`.

## What stays on the shipment row

Origin, destination, the single planned and actual pickup and delivery timestamps, and the coarse status remain per shipment. They summarize that shipment's own first pickup and final delivery. They do not describe the whole route. Intermediate stops do not overwrite them. See `SHIPMENT_FSM_ALIGNMENT.md`.

## Single-leg compatibility

A shipment that is not a participant of an `ACTIVE` revision keeps today's driver commands and status chain. Multi-stop behavior starts only after a projection exists. Existing milestones are not removed.
