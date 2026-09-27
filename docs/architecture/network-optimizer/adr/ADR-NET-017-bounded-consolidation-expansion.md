# ADR-NET-017: NLO-0.3E bounded consolidation expansion

Status: Proposed. Discovery and architecture freeze. Implementation is not authorized. Controller acceptance is required before any runtime change.

Baseline: `origin/main` `6576034a4f39b451341ad72667c5fc25f6562658`. Measurements: `NLO_0_3E_MEASUREMENTS.md`.

## Decision

NLO-0.3E, when later authorized, extends planning search only. `execution_supported=false`. It does not activate a route, mutate a shipment or order, or create an assignment, reservation, offer, slot, or driver task.

Algorithm: lexicographic incremental extension of load ids inside one canonical origin-destination group, from size 2 through a server-owned `MAX_SET_SIZE`, stopping at a server-owned `MAX_SETS_EVALUATED`. Unrestricted subset enumeration is forbidden. Beam search and top-K seeding are rejected because they need a score.

`MAX_SET_SIZE`, `MAX_CANDIDATE_POOL`, `MAX_SETS_EVALUATED`, `MAX_GROUPAGE_CALLS`, `MAX_ROUTING_CALLS`, and `TIME_BUDGET` are `UNSET`. Local memory measurements show that a single origin-destination group of 500 loads evaluates 124750 pairs, persists 374251 logical rows, and holds on the order of 1.7 GB. That evidence forbids unbounded enumeration. It does not set a production number. Implementation is blocked until a controller sets the numbers.

Route sequences per set: 1. Additional pickups in load-id order, then deliveries in that same order, then the existing destination. `N!` permutation is forbidden.

Cross-shipper policy: `CROSS_SHIPPER_N_WAY_FEASIBLE_ALLOWED=NO` for two shippers and for three or more. Result stays `INDETERMINATE` / `MULTI_PARTY_REFERENCE_CONTEXT_UNAVAILABLE`. One tenant's catalog overlay is not applied to another tenant's cargo.

Ranking: no. `MATCH_SCORE_REUSED=NO`. `ConsolidationPlanningScore` is not introduced. Tie break is load-id order, then the existing status rank for the returned page. Weights stay unset. `MAX_CONTRIBUTION` stays reserved.

`candidate_limit` remains a returned-result cap. Compute budgets are the server policy fields above.

Audit: each search creates a new run. Assessed sets are persisted only within the budget. Fingerprints cover members, versions, compatibility provenance, rehandling policy, and routing proof. No cache.

Invalidation: a change of rule, catalog, load, shipment, tracking, or routing fingerprint changes the candidate fingerprint even when the status does not.

Failure modes: `POOL_LIMIT_EXCEEDED`, `SEARCH_BUDGET_EXCEEDED`, `ROUTING_BUDGET_EXCEEDED`. Fail closed. Unknown is not zero and not feasible.

Problem classes in this ADR: same-origin sets larger than 2, and current-trip sets with more than one additional load, as plan-only. Multi-pick, multi-drop, and multi-pick-multi-drop wait for NLO-0.4.

Persistence: the current member ordinal check and OpenAPI `maxItems: 2` cannot store N-member sets. A migration is required later. This ADR does not add one and does not reserve `000084`.

Public API: reuse `POST /v1/network/consolidation/search`. No new route in this decision.

## Consequences

- NLO-0.3 stays incomplete until a later implementation is accepted and merged.
- NLO-0.4 is not started.
- Solver techniques remain out: VRP, MILP, CP-SAT, LNS, genetic algorithms, ML, and 3D bin packing.
- Backhaul, round trip, city rules, and network-wide optimization remain out.
