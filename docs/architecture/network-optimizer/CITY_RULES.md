# City rules

## Bounded context

Name: `city-rules`.

A separate microservice is not created in this freeze. The context is a provider port plus versioned data so the solver cannot hardcode Moscow or Saint Petersburg. Extraction later is allowed if rule authorship and release cadence diverge from the optimizer. See ADR-NET-009.

## Geography hierarchy

```text
Country → Region → City → Zone → Road Segment
```

A rule attaches at the most specific node that is true. Evaluation walks from the segment up to the country and applies every effective rule. More specific rules refine; they do not silently delete a stricter parent restriction unless the version says `overrides`.

## Rule model

```text
rule_id
jurisdiction
city
zone
road_segment
vehicle_conditions
cargo_conditions
permit_requirements
time_restrictions
weight_restrictions
dimension_restrictions
environmental_restrictions
effective_from
effective_to
source_reference
source_authority
version
status                 # DRAFT | ACTIVE | SUPERSEDED
```

**Current Moscow and Saint Petersburg legal numbers are not stored here.** This freeze did not verify official decrees. Putting unverified tonnes, hours, or zone polygons into the architecture pack would present them as facts. Population of real rules is an operations task with source, authority, and check date.

Checked on: 2026-09-24. Normative values imported: none.

## Profiles

`CITY_PROFILE_MOSCOW` and `CITY_PROFILE_SAINT_PETERSBURG` are named bundles. Each bundle lists the rule **types** the city must be able to express:

- restricted zones
- road classes
- vehicle mass and dimensions
- permit requirements
- time windows
- temporary restrictions
- vehicle access
- facility access

A profile id selects a rule-set id and a timezone. Example timezone labels `Europe/Moscow` already exist as a location default in transport-order code; that default is not a restriction engine.

Synthetic shape only (`EXAMPLE_NOT_NORMATIVE`):

```json
{
  "profile_id": "CITY_PROFILE_MOSCOW",
  "rule_set_id": "mos-unpopulated",
  "status": "DRAFT",
  "normative_values": "NONE",
  "note": "No legal thresholds in this document."
}
```

The same shape applies to `CITY_PROFILE_SAINT_PETERSBURG` with its own `rule_set_id`. The two profiles do not share a mutable singleton. They can share a country-level parent rule set for Russia when real data exists.

Forbidden in the future solver:

```text
if city == "Moscow"
if city == "Saint Petersburg"
```

## Temporary restrictions

Supported kinds: temporary road restriction, event, construction, weather, temporary freight access. Each has `effective_from` and `effective_to`.

Strategy:

| Mechanism | Rule |
|-----------|------|
| Snapshot | An `OptimizationDecision` stores `city_rule_version` ids used. |
| Cache | Providers cache by version. A new version is a new key. |
| Invalidation | Publish of a version, or `effective_to` passing, drops the cache entry. |
| Fallback | No silent fallback to an empty rule set. Stale or missing urban rules reject executable urban plans. |

## Slot and facility access

Facility access rules reference a location or `facility_id` when a facility master exists. Until then they reference `location_id`. They do not create slot bookings.
