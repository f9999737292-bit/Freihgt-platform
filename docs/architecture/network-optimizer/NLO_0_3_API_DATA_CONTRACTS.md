# NLO-0.3 API and data contracts

Baseline: `origin/main` `6576034a4f39b451341ad72667c5fc25f6562658`. NLO-0.3B, NLO-0.3C, and NLO-0.3D are IMPLEMENTED_CLOSED on main. `CURRENT_TRIP_FILL` is a public planning search, `execution_supported=false`, `MAX_ADDITIONAL_LOADS=1`, and it does not mutate a shipment, order, assignment, reservation, offer, slot, or driver task. This wave does not persist `current_trip_context_id`.

```text
NLO_0_3E=ARCHITECTURE_FREEZE_CANDIDATE
IMPLEMENTATION_STARTED=NO
ADR_STATUS=PROPOSED
```

NLO-0.4 is NOT_STARTED. NLO-0.3 is not complete. This document records the proposed NLO-0.3E contract. It does not change generated OpenAPI.

## One search endpoint

`POST /v1/network/consolidation/search`

Gateway would later expose it under `/api/v1/network/consolidation/search` for the carrier roles that may already search next loads. This phase adds no route.

The caller does not upload `CurrentTripContext`. Residual payload, onboard cargo, GPS, ETA, vehicle totals, and temperature state are not request fields.

## Pattern semantics

`SAME_ORIGIN_SAME_DESTINATION` is the implemented pairwise mode. `PAIRWISE_ONLY=YES`. `SET_SIZE=2`. The response stays `PairwiseConsolidationSearchResponse` with `evaluated_pair_count` and `PairwiseConsolidationCandidate` (`members.minItems=2`, `members.maxItems=2`). NLO-0.3E does not return three-member candidates from this pattern and does not apply the N-member pool of 10 to it. `LEGACY_PAIRWISE_UNBOUNDED_BEHAVIOR=UNCHANGED_BY_0_3E`. Any later limit on this mode needs its own compatibility decision.

`CURRENT_TRIP_FILL` is the implemented planning search. `MAX_ADDITIONAL_LOADS=1`. The response stays `CurrentTripFillSearchResponse`. NLO-0.3E does not change it.

`SAME_ORIGIN_SAME_DESTINATION_N_MEMBER` is the proposed additive pattern. It is not implemented. The request sends that pattern, an owned `capacity_id`, the existing policy when it applies, and optional `candidate_limit`. The caller does not send `max_set_size`, `max_pool_size`, `max_sets`, `max_groupage_calls`, or `time_budget`. Server policy for this pattern only is pool 10, set size 3, 165 sets, 330 groupage calls, 0 routing calls, and a 5 second watchdog. `POOL_LIMIT_EXCEEDED` and `SEARCH_BUDGET_EXCEEDED` belong to this pattern. They are not retroactive pairwise errors.

The N-member response is `NMemberConsolidationSearchResponse`. It is not `PairwiseConsolidationSearchResponse`. It includes `search_id`, `capacity_id`, `capacity_version`, `pattern`, `pool_load_count`, `evaluated_set_count`, `feasible_candidate_count`, `indeterminate_candidate_count`, `hard_reject_candidate_count`, `returned_candidate_count`, and `candidates`. The counter is `evaluated_set_count`. `NMemberConsolidationCandidate` has `members.minItems=2` and `members.maxItems=3`. Ordinal is 1 through 3. `execution_supported=false`.

Proposed discriminator, not generated here:

```text
SAME_ORIGIN_SAME_DESTINATION -> PairwiseConsolidationSearchResponse
SAME_ORIGIN_SAME_DESTINATION_N_MEMBER -> NMemberConsolidationSearchResponse
CURRENT_TRIP_FILL -> CurrentTripFillSearchResponse
```

| Request | Before | After | Change |
| --- | --- | --- | --- |
| Existing `SAME_ORIGIN_SAME_DESTINATION` | pairwise | pairwise | `BREAKING=NO` |
| Existing `CURRENT_TRIP_FILL` | max additional loads 1 | max additional loads 1 | `BREAKING=NO` |
| New `SAME_ORIGIN_SAME_DESTINATION_N_MEMBER` | unsupported | bounded 2..3 member planning | `ADDITIVE=YES` |

The persisted `pattern` stores which of the two same-origin modes ran. Audit does not infer the mode from member count. An N-member fingerprint includes `pattern=SAME_ORIGIN_SAME_DESTINATION_N_MEMBER` and the algorithm and budget policy version.

`SAME_ORIGIN_SAME_DESTINATION` sends an owned `capacity_id`, pattern, policy, and optional `candidate_limit`. The server checks capacity tenant ownership, status, version, and effective capability. A foreign capacity is `NOT_FOUND`. The N-member request uses the same capacity checks.

`CURRENT_TRIP_FILL` sends `shipment_id`, pattern, policy, and optional `candidate_limit`. The server assembles the context. A foreign shipment is `NOT_FOUND`. The response may include a context summary. That summary is not trusted input. `current_trip_context_id` is not accepted in NLO-0.3D because the server does not persist that context. The pairwise response requires `capacity_id` and `capacity_version`. The current-trip response omits both. A zero UUID or a zero version is not used for an absent capacity.

Opt-in is read from persisted load publication. The search body cannot set `consolidation_allowed` or `cross_shipper_consolidation_allowed`.

Same-owner participation requires `consolidation_allowed=true` on each load. Cross-shipper participation requires `cross_shipper_consolidation_allowed=true` on each load and does not require `consolidation_allowed`.

Other request concepts:

- pattern is `SAME_ORIGIN_SAME_DESTINATION`, `CURRENT_TRIP_FILL`, or, when later implemented, `SAME_ORIGIN_SAME_DESTINATION_N_MEMBER`;
- policy is rehandling allowed or not;
- `candidate_limit` is optional. On the proposed N-member pattern it is a response cap only. It does not set the server work budgets. Legacy pairwise behavior is unchanged by NLO-0.3E.

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
| `ShipmentExecutionProvider` | shipment-service | `GET /internal/v1/shipments/{shipmentId}/execution-context` |
| `ShipmentOnboardCargoProvider` | shipment-service | `GET /internal/v1/shipments/{shipmentId}/onboard-cargo`; only `CONFIRMED_ONBOARD` evidence occupies capacity |
| `CargoProjectionProvider` | transport-order-service | `GET /internal/v1/cargoes/{id}/planning-profile`, including `version`; shipment-service does not own a second cargo projection |
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
