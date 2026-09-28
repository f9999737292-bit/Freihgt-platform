# ADR-TMS-001: Execution aggregate ownership

## Status

Accepted. Architecture freeze, remediation R1. Implementation is not authorized.

## Context

`shipment-service` owns shipment status, assignment, driver operations, cargo execution evidence, and `transport.shipment_event_outbox`. `network-optimizer-service` owns RoutePlan. ADR-NET-019 allows a route whose actions reference `LOAD_OPPORTUNITY` or `SHIPMENT_CARGO`, including several cargo subjects. ADR-NET-021 allows `DEPOT_START`, where `RoutePlan.shipment_id` may be null, and `CURRENT_TRIP`, where an existing shipment is context rather than the only cargo on the route.

Remediation R1 closes the first freeze's assumption that one `Shipment` row is the parent of the execution plan. That model cannot represent a depot-start route or more than one shipment on the same driver sequence. A new microservice is not the remedy. The shipment row also cannot be the parent, because coarse status, origin, and destination stay per shipment.

## Decision

`shipment-service` owns execution. No new service is created. The long-lived aggregate is `TransportExecution`, not a shipment.

`TransportExecution` references the carrier, vehicle, driver, and the activation of its current revision. It does not require a pre-existing anchor shipment. `DEPOT_START` may create it with no `shipment_id`. `CURRENT_TRIP` records the existing trip shipment as one participant, not as the aggregate root.

Children and associations:

| Entity | Parent | Role |
| --- | --- | --- |
| `TransportExecution` | none inside shipment | Driver route aggregate |
| `TransportExecutionRevision` | `TransportExecution` | One activation projection |
| `TransportExecutionParticipant` | `TransportExecution` | Materialized `shipment_id` + `cargo_id` |
| `TransportExecutionStop` | `TransportExecution` | Stable stop row |
| `TransportExecutionRevisionStop` | revision and stop | Membership only. Does not own the stop |
| `TransportExecutionAction` | `TransportExecutionStop` | Pickup or delivery of one materialized cargo |

Shipment coarse status stays on each participating shipment. The driver sequence is the active revision of `TransportExecution`.

```text
EXECUTION_AGGREGATE_OWNER=shipment-service
EXECUTION_ROUTE_MODEL=TRANSPORT_EXECUTION
NEW_SERVICE_REQUIRED=NO
REUSE_EXISTING_SHIPMENT_AGGREGATE=NO
MULTI_SHIPMENT_EXECUTION_SUPPORTED_BY_MODEL=YES
DEPOT_START_WITHOUT_PREEXISTING_SHIPMENT_SUPPORTED=YES
```

`REUSE_EXISTING_SHIPMENT_AGGREGATE=NO` means the shipment row is not the execution root. Shipment status, evidence, and single-leg commands stay where they are.

## Consequences

NLO-0.4D, when later authorized, writes these rows from an activated plan whose cargo actions are already materialized. It does not move stop completion into the optimizer. This ADR does not weaken the NLO rule that production activation stays blocked until an authoritative service-duration source exists.
