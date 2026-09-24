# ADR-NET-001: Optimization bounded context

## Status

Proposed — architecture freeze v0.1. Implementation not authorized.

## Context

Transport order, shipment, tracking, RFx, contract rate, and freight cost already exist. None of them plans network fill, backhaul, or city routing. Discovery found no optimizer service.

## Decision

Introduce a bounded context `network-optimizer`. It owns plans, capacities, published opportunities, matches, and offers. It does not own execution, tenders, rates, cost, slots, or documents. No process is deployed in this freeze.

## Consequences

A future service needs a separate approval, its own schema, and gateway routes. Until then this ADR is a boundary, not a repository scaffold.
