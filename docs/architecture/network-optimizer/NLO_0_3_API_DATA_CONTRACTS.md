# NLO-0.3 API and data contracts

Baseline: `origin/main` `6d47cc92`. Nothing in this document is implemented. `packages/openapi` is unchanged.

## One search endpoint

`POST /v1/network/consolidation/search`

Gateway would later expose it under `/api/v1/network/consolidation/search` for the carrier roles that may already search next loads. This phase adds no route.

Request concepts:

- `current_trip_context` or an owned `capacity_id` for a future-empty pair search, not both as competing sources of residual space;
- selection scope: same owner, or cross-shipper only where both loads opted in;
- pattern: `SAME_ORIGIN_SAME_DESTINATION` or `CURRENT_TRIP_FILL`;
- policy: rehandling allowed or not;
- limit: optional cap on returned sets. Production numbers stay unset.

Response concepts:

- search id and input fingerprint;
- pattern;
- candidate sets with safe member views;
- compatibility status and condition codes;
- residual snapshot and provenance;
- route feasibility, including provider and road figures only for the carrier audience;
- `placement_check`;
- hard-reject counts by bounded reason;
- planning status and `execution_supported=false` for multi-stop or multi-order results.

No arbitrary weight payload. Profiles stay in the database when a later score exists. The first waves do not score.

## Internal ports

| Port | Owner | Existing endpoint to extend, not replace |
| --- | --- | --- |
| `ShipmentExecutionProvider` | shipment-service | prediction input is the closest read and is insufficient because it omits cargo |
| `CargoProjectionProvider` | shipment-service | tenant-scoped cargo by shipment, including nullability |
| `TrackingPositionProvider` | tracking-service | position plus `EvaluateFreshness` |
| `TrackingETAProvider` | tracking-service | existing ETA lookup |
| `CurrentTripContextProvider` | network-optimizer-service | assembles the ports above; it is not a second database |

No direct cross-service SQL. No single "optimizer data" dump endpoint.

## Events

Do not publish events in this discovery task.

Later, one lifecycle is enough to start:

- `network.consolidation_candidate.generated` — aggregate `ConsolidationCandidate`, tenant of the capacity owner, internal payload may hold member owner ids, public payload may not.
- `network.consolidation_candidate.invalidated` — same aggregate, when a pin changes.

Idempotency key is candidate id plus input fingerprint. A new fingerprint is a new candidate, not a mutation. No further event names until that lifecycle exists.

Invalidation triggers: load version or withdrawal, capacity change, shipment status, vehicle change, stale location, ETA change, catalog activation, rule-set activation, slot change, onboard cargo change.

## Audit contents

The immutable planning record stores the cargo set, versions, residual provenance, groupage fingerprint, catalog and rule versions, routing provider, road distances and times, window arithmetic, unknown constraints, and privacy scope. Human views apply the privacy document.

## Metrics

`bno_consolidation_searches_total`, `bno_consolidation_sets_evaluated_total`, `bno_consolidation_hard_reject_total{reason}`, `bno_consolidation_indeterminate_total`, `bno_consolidation_set_size_bucket`, `bno_consolidation_duration_seconds`. Reasons are a fixed code list. No identifier labels.

## Non-goals of the contract

No shipment write, order write, slot booking, driver task, reservation, assignment, offer, tender, billing, or EDO mutation.
