# ADR-NET-008: Event-driven reoptimization

## Status

Proposed — architecture freeze v0.1. Implementation not authorized.

## Context

Shipment and driver events already use an outbox and Kafka. ADR-EDO-006 fixes naming. ETA is stored in tracking and is not a bus event. Control Tower consumes shipment and driver topics.

## Decision

New facts use `network.*` past-tense names and the ADR-EDO-006 envelope. The optimizer consumes existing shipment and driver events later; it does not rename them. ETA publication, if added, is owned by tracking as `tracking.eta.updated`. Reoptimization listens for slack breaks and emits `network.chain.marked_at_risk`. It does not cancel shipments. Outbox, idempotency, and consumer DLQ follow the current platform pattern. The first next-load slice may be synchronous; events are still the contract for later loops.

## Consequences

Do not invent `shipment.eta.updated` or present-tense `capacity.available` on the bus.
