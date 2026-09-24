# ADR-NET-007: Freight cost and RFx ownership

## Status

Proposed — architecture freeze v0.1. Implementation not authorized.

## Context

Contract rate resolution, transport-order rate snapshots, freight-cost ledger entries, RFx awards, and billing settlements are implemented. A planning score needs revenue and cost terms.

## Decision

The optimizer may compute `expected_contribution` as a planning figure inside `OptimizationDecision`. It must not write rate cards, rate snapshots, `CostEntry` rows, awards, or settlements. `DIRECT_ACCEPT`, `FIXED_PRICE_OFFER`, and `COUNTER_OFFER` are carrier offers. `MINI_TENDER`, `RFQ`, and `AUCTION` are handed to `rfx-service`. Unpriced candidates are excluded from revenue and contribution ranking.

## Consequences

Freight cost and RFx remain the commercial systems of record. Duplicate tender or ledger logic is a defect.
