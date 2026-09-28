# ADR-TMS-004: Completed stop immutability

## Status

Accepted. Architecture freeze. Implementation is not authorized.

## Context

Cargo evidence is already append-only. Driver idempotency stores the first response. A replan that rewrites a completed pickup would destroy the audit of what the driver confirmed and when.

## Decision

A stop in `COMPLETED` cannot change its ordinal, location, planned times, actual timestamps, or action results. Completed actions cannot change. Cargo evidence remains append-only. Duplicate complete commands return the original row.

```text
COMPLETED_STOP_IMMUTABLE=YES
```

## Consequences

Corrections after completion require a successor plan or a new evidence row for a new action. They do not update the completed stop.
