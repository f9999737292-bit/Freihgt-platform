# ADR-NET-002: Marketplace projection and tenant isolation

## Status

Proposed — architecture freeze v0.1. Implementation not authorized.

## Context

Gateway JWT sets `X-Tenant-ID`. Shipment and order rows are tenant-owned. A matcher that selects shipments across tenants would break isolation. No load board exists.

## Decision

The engine reads only explicit `LoadOpportunity` and `Capacity` projections. A transport order is not public by default. Scopes are `PRIVATE`, `INVITED_CARRIERS`, `MARKETPLACE`, `ANONYMIZED_MARKETPLACE`, and `NETWORK_OPTIMIZATION_ONLY`. Out-of-scope reads hide existence. Cross-shipper consolidation does not reveal the other party's rate, contract, customer, internal ids, or tender.

## Consequences

Publication is a product step. Matching tests must fail if a query touches `transport.shipments` without the owner's tenant.
