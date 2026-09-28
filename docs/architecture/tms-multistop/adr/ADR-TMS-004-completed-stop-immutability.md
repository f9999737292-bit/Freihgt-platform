# ADR-TMS-004: Completed stop immutability

## Status

Accepted. Architecture freeze, remediation R1. Implementation is not authorized.

## Context

Cargo evidence is append-only. A replan that rewrites a completed pickup, or that moves the completed row onto a successor revision by changing its parent, destroys the audit of what the driver confirmed.

## Decision

A stop in `COMPLETED` cannot change its id, `execution_id`, ordinal, location, planned times, actual timestamps, or action results. The successor revision references that row through a new `TransportExecutionRevisionStop` link. The link insert is not an update of the stop. Completed actions cannot change. Cargo evidence remains append-only. Duplicate complete commands return the original row.

```text
COMPLETED_STOP_IMMUTABLE=YES
COMPLETED_HISTORY_IMMUTABLE=YES
COMPLETED_STOP_ROW_REPARENTED=NO
COMPLETED_STOP_ID_CHANGED=NO
```

## Consequences

Corrections after completion require a successor revision and new stop ids for new work, or a new evidence row for a new action. They do not update or re-parent the completed stop.
