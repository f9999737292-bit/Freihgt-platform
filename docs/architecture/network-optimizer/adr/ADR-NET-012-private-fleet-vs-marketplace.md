# ADR-NET-012: Private fleet and marketplace modes

## Status

Proposed — architecture freeze v0.1. Implementation not authorized.

## Context

The commercial priority is marketplace network optimization. Private and dedicated fleets need assignment, depots, and different objectives. Two engines would split feasibility and city rules.

## Decision

One optimization core. Mode `MARKETPLACE` uses publication, anonymization, and carrier offers. Mode `PRIVATE_FLEET` uses the same feasibility and routing with explicit vehicle, driver, trailer, and depot resources and extra objectives (`MIN_TOTAL_COST`, `MAX_FLEET_UTILIZATION`, `MIN_NUMBER_OF_VEHICLES`, `SERVICE_LEVEL_FIRST`). Private mode does not publish another tenant's loads. Marketplace mode may match equipment before a driver or vehicle is named.

## Consequences

Fleet work is FLEET-1.x on this core, not a fork. Shared city rules and cargo constraints stay in one place.
