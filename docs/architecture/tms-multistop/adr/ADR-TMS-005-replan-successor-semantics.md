# ADR-TMS-005: Replan and successor semantics

## Status

Accepted. Architecture freeze, remediation R2. Implementation is not authorized.

## Context

ADR-NET-021 says replanning creates a successor and does not move shipment status backward. The first execution freeze said a completed stop keeps its id and is also represented on the successor while the stop's only parent was `execution_plan_id`. One row cannot be a child of two revisions. Remediation R1 uses direction A: the stop's parent is the long-lived `TransportExecution`. Revisions reference stops. They do not own them.

## Decision

A successor activation in `PENDING_EXECUTION` is projected by the trusted command. That command creates a new `TransportExecutionRevision` on the same `TransportExecution`. `EXECUTION_LINKED` is Agent D's later status, after the command returns `execution_id` and `revision_id`. In the projection transaction:

| Piece | Rule |
| --- | --- |
| Completed stop | Same id. `execution_id` unchanged. New link `INHERITED_COMPLETED` stores the successor RoutePlan stop id. The introducing link keeps the original stop id. No update of the stop row |
| Stop in `ARRIVED` or `SERVICE_STARTED` | Same id. `execution_id` unchanged. New link `INHERITED_IN_SERVICE` stores the successor stop id. Incomplete actions get new `TransportExecutionRevisionAction` links for the successor action ids. Status and timestamps unchanged |
| Remaining `PLANNED` stops | Status `CANCELLED`, reason `SUPERSEDED`, on the old revision only. New ids, still parented by the same route, linked `INTRODUCED` on the successor |
| In-service driver task | Stays on the preserved stop |
| Future driver tasks | Cancelled with the superseded planned stops. New tasks for introduced stops |
| Old revision | `SUPERSEDED` only when the successor revision commits |
| Participating shipment status | Unchanged. Owner tenant unchanged |

`IN_SERVICE_STOP_CONFLICT` compares the successor contract with the stable stop using the successor's own source ids plus the semantic fingerprint: stop role, point kind, location or anchor coordinates, and each incomplete action's type, `shipment_tenant_id`, `shipment_id`, `cargo_id`, and `cargo_version`. Successor source ids are not required to equal the introducing plan's ids. If the fingerprint does not match, projection returns `409 IN_SERVICE_STOP_CONFLICT`, writes no successor revision, and leaves the old revision `ACTIVE`. The successor activation is not `EXECUTION_LINKED`.

```text
REPLAN_SUCCESSOR_MODEL_FROZEN=YES
REPLAN_PERSISTENCE=ROUTE_OWNS_STOPS_REVISIONS_LINK_THEM
CURRENT_STOP_REPLAN_RULE=PRESERVE_IN_SERVICE_STOP
COMPLETED_STOP_ROW_REPARENTED=NO
IN_SERVICE_STOP_ROW_REPARENTED=NO
COMPLETED_HISTORY_IMMUTABLE=YES
REMAINING_STOPS_SUPERSEDED_SAFELY=YES
STABLE_STOP_SOURCE_ID_REWRITTEN=NO
STABLE_ACTION_SOURCE_ID_REWRITTEN=NO
SUCCESSOR_ROUTEPLAN_STOP_LINEAGE_PRESERVED=YES
SUCCESSOR_ROUTEPLAN_ACTION_LINEAGE_PRESERVED=YES
```

## Consequences

Two `ACTIVE` revisions of one `TransportExecution` are a failed transaction. A shipment participates in at most one active execution.
