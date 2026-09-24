# ADR-NET-009: City rules architecture

## Status

Proposed — architecture freeze v0.1. Implementation not authorized.

## Context

Urban restrictions differ by city and change on effective dates. The solver must serve Moscow and Saint Petersburg first and more cities later. Hardcoding city names would fork the core. Official numeric rules were not verified in this freeze.

## Decision

Bounded context `city-rules`, accessed through `CityRulesProvider`. Rules are versioned, effective-dated, and source-backed, on a hierarchy Country → Region → City → Zone → Road segment. Profiles `CITY_PROFILE_MOSCOW` and `CITY_PROFILE_SAINT_PETERSBURG` select rule sets. They contain no normative thresholds until a sourced load. Stale or missing rules reject executable urban plans. Cache key is the version. Decisions snapshot the version ids. A standalone city-rules process is not part of this freeze.

## Consequences

NLO-0.7 and NLO-0.8 are data loads, not solver forks. Unverified legal numbers must not be copied into seed data from this pack.
