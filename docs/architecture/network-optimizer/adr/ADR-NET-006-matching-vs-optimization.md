# ADR-NET-006: Matching versus optimization

## Status

Proposed — architecture freeze v0.1. Implementation not authorized.

## Context

Next load, fill, backhaul, chains, regional routes, and urban routes are different search problems. A single global solve is not justified by current data or scale measurements.

## Decision

Split candidate generation from scoring. Generation order: geo proximity, time, equipment, cargo compatibility, commercial and policy filters. Scoring is a deterministic explained formula. Local route optimization is a later stage. Objective profiles are weights, including private-fleet objectives. `NetworkValue` has a deterministic fallback of zero until evidence exists.

## Consequences

The first implementation can rank one truck against a pool without a VRP solver. Explainability is mandatory.
