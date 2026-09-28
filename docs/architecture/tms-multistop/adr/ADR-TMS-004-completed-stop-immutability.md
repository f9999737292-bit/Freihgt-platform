# ADR-TMS-004: Completed stop immutability

## Status

Accepted. Architecture freeze, remediation R2. Implementation is not authorized.

## Context

Cargo evidence is append-only. A replan that rewrites a completed pickup, or that moves the completed row onto a successor revision by changing its parent, destroys the audit of what the driver confirmed.

## Decision

A stop in `COMPLETED` cannot change its id, `execution_id`, ordinal, location, planned times, actual timestamps, or action results. The successor revision references that row through a new `TransportExecutionRevisionStop` link that stores the successor RoutePlan's `source_route_plan_stop_id`. The introducing link keeps the original source id. The link insert is not an update of the stop. Completed actions cannot change, and their stable rows do not gain a rewritten `source_route_plan_action_id`. The successor's action id is a new `TransportExecutionRevisionAction` link. Cargo evidence remains append-only. Duplicate complete commands return the original row.

```text
COMPLETED_STOP_IMMUTABLE=YES
COMPLETED_HISTORY_IMMUTABLE=YES
COMPLETED_STOP_ROW_REPARENTED=NO
IN_SERVICE_STOP_ROW_REPARENTED=NO
COMPLETED_STOP_ID_CHANGED=NO
STABLE_STOP_SOURCE_ID_REWRITTEN=NO
STABLE_ACTION_SOURCE_ID_REWRITTEN=NO
SUCCESSOR_ROUTEPLAN_STOP_LINEAGE_PRESERVED=YES
SUCCESSOR_ROUTEPLAN_ACTION_LINEAGE_PRESERVED=YES
```

## Consequences

Corrections after completion require a successor revision and new stop ids for new work, or a new evidence row for a new action. They do not update or re-parent the completed stop.
