# ADR-NET-004: CargoUnit and consolidation

## Status

Proposed — architecture freeze v0.1. Implementation not authorized.

## Context

Cargo today has weight, volume, temperature, a dangerous-goods flag, and item hazard class. It has no pallets, linear metres, dimensions, or co-load rules. Shipment is one origin and one destination.

## Decision

`CargoUnit` is a projection, not a second cargo master. Missing physical attributes make the related constraint `INDETERMINATE`, not zero. The consolidation engine separates hard constraints from score. Violation is `HARD_REJECT`. Co-load of two or more shippers, including A+B+C on one vehicle, requires publication scopes that allow it. 3D packing is deferred; the placement contract is `NOT_EVALUATED`, `SEQUENCE_OK`, `SEQUENCE_CONFLICT`, or `REHANDLE_REQUIRED`.

## Consequences

Weight-and-volume fill can ship before pallet data exists, but the explanation must say what was not checked.
