# NLO-0.3 implementation roadmap

Architecture baseline: `origin/main` `6d47cc92`. Current implementation main: `ef3722db92fd1c115f6539cc491df3eabd57affa`. NLO-0.3A is FROZEN_ACCEPTED and CLOSED. NLO-0.3B is IMPLEMENTED / CLOSED. NLO-0.3C is IMPLEMENTED_CLOSED. NLO-0.3D is IMPLEMENTED_CLOSED. NLO-0.3E architecture is FROZEN_ACCEPTED. ADR-NET-017 is Accepted. Implementation is not started and is not authorized. NLO-0.3 is not complete. NLO-0.4 is not started. GPS is not an NLO-0.2 dependency. Current-trip residual capacity is NLO-0.3.

## What NLO-0.3 will implement

Planning feasibility for:

- a same-origin, same-destination pair of published loads, with owner-controlled opt-in and no current-trip dependency, now IMPLEMENTED / CLOSED as NLO-0.3B;
- one confirmed onboard cargo projection plus exactly one additional published load. That wave is NLO-0.3D, IMPLEMENTED_CLOSED, planning only, `execution_supported=false`, `MAX_ADDITIONAL_LOADS=1`. It does not mutate a shipment.

Both stay `PROPOSED`. Neither mutates a shipment.

## What NLO-0.4 owns

Persistent `RoutePlan` / `RouteStop` execution, multi-stop shipment representation, shared route legs, accepted-plan activation, and driver multi-stop tasks. ADR-NET-005 stays the gate. NLO-0.3 does not pull that work forward.

## What stays NLO-0.5 and later

Backhaul and round trip, regional and urban routing with sourced city rules, chain reoptimization, network-level optimization, and any solver. Consolidation ranking weights, if ever added, are a later authorized profile, not NLO-0.3B–D.

## Opt-in rule

Same statement as ADR-NET-015, the privacy document, and the API design. Both flags are owner-controlled persisted publication facts. Default is false. A searching carrier cannot set them on the request.

- Same-owner pair: every participating load has `consolidation_allowed=true`. The cross-shipper flag does not grant or deny that pair.
- Cross-shipper pair: every participating load has `cross_shipper_consolidation_allowed=true`. `consolidation_allowed` is not required and does not substitute.

## Same O-D rule

`SAME_ORIGIN_SAME_DESTINATION` means internal canonical `pickup.location_id` equality and `delivery.location_id` equality. Labels, city, region, coarse geography, rounded coordinates, and Haversine are not proof. Missing identity is `ORIGIN_IDENTITY_UNPROVEN` or `DESTINATION_IDENTITY_UNPROVEN`. Known pickup windows must overlap. Known delivery windows must overlap. A disjoint required window is `HARD_REJECT`. An unknown required window is `INDETERMINATE`.

## Waves

The first product wave was pairwise consolidation because unit-level onboard evidence did not exist yet. A current-trip wave started first could not return a useful `FEASIBLE` result. That evidence now exists as NLO-0.3C. NLO-0.3D uses it for planning-only fill of one additional load.

### NLO-0.3B — pairwise same-origin foundation

Status: IMPLEMENTED / CLOSED. Merged in PR #174 at `8208c662f1c80475338a97dbc710268fa0fc5a07`. Migration `000081_nlo_pairwise_consolidation_v0_3b`. NLO-0.3 is not complete.

Scope: `PAIRWISE_CONSOLIDATION_ONLY`, set size 2. Owned effective capacity plus two explicitly published loads. Canonical same O-D, overlapping windows, owner-controlled opt-in, and B2 `EvaluateGroupageItems` for a same-owner pair. A same-owner `FEASIBLE` result considers capacity-owner restrictions and load-owner restrictions. One tenant's catalog overlay is not applied to the other tenant's cargo or equipment. If that ownership cannot be proven, the result is `INDETERMINATE` with `MULTI_PARTY_REFERENCE_CONTEXT_UNAVAILABLE`, not `FEASIBLE`. `CROSS_SHIPPER_OPT_IN=IMPLEMENTED`. `CROSS_SHIPPER_FULL_COMPATIBILITY_PROOF=NO`. `CROSS_SHIPPER_FEASIBLE_ALLOWED=NO`. A cross-shipper pair fails closed as `INDETERMINATE` / `MULTI_PARTY_REFERENCE_CONTEXT_UNAVAILABLE`. That fail-closed result is intentional. Planning only. No score. No shipment mutation. No solver. No current-trip residual dependency. `execution_supported=false`.

API: `POST /v1/network/consolidation/search` with pattern `SAME_ORIGIN_SAME_DESTINATION` and an owned `capacity_id`.

Security: capacity tenant ownership, status, version, and effective capability. A foreign capacity is `NOT_FOUND`. Marketplace list unchanged.

Tests: canonical id match, label mismatch excluded, unproven origin, overlapping and disjoint windows, opt-in absent, cross-shipper without `consolidation_allowed`, pair hard deny, privacy of the other shipper, version invalidation, deterministic order.

Out: current-trip context, residual occupancy, route insertion, execution, score.

### NLO-0.3C — onboard evidence, trip context, residual snapshot

Status: IMPLEMENTED_CLOSED. Merged in PR #176 at `cad3c67b93fd72329050db624ae048495ef0236f`. Feature head `d8b2b659f7fbc735bf84e53b458755a89311734d`. Migration `000082_nlo_onboard_evidence_current_trip_context_v0_3c`. `NLO_0_3_COMPLETE=NO`. NLO-0.3D is a later wave and does not change this closed status.

Scope: shipment-service owns the onboard-cargo read and the append-only unit-level `CONFIRMED_ONBOARD` evidence. A shipment-wide status does not prove that cargo is onboard. BNO consumes the provider and builds server-side `CurrentTripContext` and `ResidualCapacitySnapshot` from trusted ports. Migration `000082` is that evidence table. It does not reuse `000081`. `NEW_EXECUTION_EVIDENCE_REQUIRED=YES` before a residual result can be `FEASIBLE`. This wave does not return that public result.

API: none in this wave that treats caller residual facts as authority. Public `CURRENT_TRIP_FILL` search is NLO-0.3D, not this closed wave.

Dependencies: the execution fact is migration `000082`. Shipment status, linked cargo, and planned quantity do not qualify by themselves. Driver events in shipment-service are shipment-scoped and do not name a cargo unit. The server reads `cargo_id` from the shipment inside the status transaction.

Security: trusted tenant, owned shipment, `NOT_FOUND` for a foreign id. No browser tenant header.

Tests: `CONFIRMED_ONBOARD` counts, `PLANNED` does not, unknown dimension blocks `FEASIBLE`.

Out: load insertion, shipment writes.

### NLO-0.3D — one additional current-trip load

Status: IMPLEMENTED_CLOSED. Merged in PR #178 at `ef3722db92fd1c115f6539cc491df3eabd57affa`. Feature head `f61d64f8c34fa8cafa6bee30d44d8646c7c30655`. Migration `000083_nlo_current_trip_fill_v0_3d`. `CURRENT_TRIP_FILL_PUBLIC_ENABLED=YES` for the planning search only. `NLO_0_3E_ARCHITECTURE=FROZEN_ACCEPTED`. `NLO_0_3E_IMPLEMENTATION_STARTED=NO`. `NLO_0_4_STATUS=NOT_STARTED`. `NLO_0_3_COMPLETE=NO`. Controller findings `F001_CAPACITY_CONTEXT` and `F002_AUDIT_FINGERPRINT` are CLOSED.

Scope: context from 0.3C plus exactly one published load. Road insertion via the routing port. Location and ETA decisions follow the freshness status returned by tracking-service. Planning only. `execution_supported=false`. `MAX_ADDITIONAL_LOADS=1`. No shipment, order, assignment, reservation, offer, slot, or driver task. A pairwise audit run requires capacity id and version. A current-trip audit run leaves both absent.

API: same search endpoint with pattern `CURRENT_TRIP_FILL` and `shipment_id`. The caller does not submit residual facts or a raw context. There is no persisted `current_trip_context_id` in this wave.

Dependencies: 0.3C and a `FRESH` position when insertion needs a position. Without `CONFIRMED_ONBOARD` the result is `INDETERMINATE`.

Tests: `NLO03D_001` through `NLO03D_018`, plus the 0.3B and 0.3C suites.

Out: a second extra load, solver, slot booking, assignment, reservation, NLO-0.3E, NLO-0.4.

### NLO-0.3E — bounded expansion and optional ranking

Status: architecture FROZEN_ACCEPTED. ADR-NET-017 is Accepted. Implementation is not started and is not authorized. `ARCHITECTURE_FROZEN=YES`. `TEST_STRATEGY_FROZEN=YES`. `NLO_0_3E_IMPLEMENTATION_STARTED=NO`.

Scope: additive pattern `SAME_ORIGIN_SAME_DESTINATION_N_MEMBER` on the existing search. Legacy `SAME_ORIGIN_SAME_DESTINATION` stays pairwise with set size 2. Still no solver. A separate score, if authorized, is not MatchScore.

Out of NLO-0.3E: unrestricted `2^N`, ML, 3D packing, shipment activation.

## Traceability

| Requirement | Source | Check | Unknown | Wave | Status |
| --- | --- | --- | --- | --- | --- |
| weight | vehicle `capacity_weight`, cargo `gross_weight` | groupage in 0.3B; residual subtraction in 0.3C | `INDETERMINATE` | 0.3B / 0.3C | FROZEN_ACCEPTED |
| volume | vehicle `capacity_volume`, cargo `volume` | groupage in 0.3B; residual subtraction in 0.3C | `INDETERMINATE` | 0.3B / 0.3C | FROZEN_ACCEPTED |
| pallet count | `cargoes.pallet_count` | not coerced to zero | `INDETERMINATE` | 0.3B / 0.3C | FROZEN_ACCEPTED |
| pallet type | `pallet_type_code` plus equivalences | explicit positive factor only | `INDETERMINATE` | 0.3B | FROZEN_ACCEPTED |
| linear metres | cargo and `usable_linear_meters` | groupage in 0.3B; residual subtraction in 0.3C | `INDETERMINATE` | 0.3B / 0.3C | FROZEN_ACCEPTED |
| height | `max_loaded_height_mm`, internal height | comparison | `INDETERMINATE` | 0.3B | FROZEN_ACCEPTED |
| body and trailer type | B2 body check | groupage | `INDETERMINATE` | 0.3B | FROZEN_ACCEPTED |
| rear, side, top loading | access lists | independent | `INDETERMINATE` | 0.3B | FROZEN_ACCEPTED |
| unloading access | access lists | independent | `INDETERMINATE` | 0.3B | FROZEN_ACCEPTED |
| temperature | cargo interval, one zone | common intersection | `INDETERMINATE` | 0.3B | FROZEN_ACCEPTED |
| multi-zone | `TemperatureZoneCount` | existing indeterminate code | `INDETERMINATE` | 0.3B | FROZEN_ACCEPTED |
| food grade | B2 rules | groupage | `INDETERMINATE` | 0.3B | FROZEN_ACCEPTED |
| ADR | sourced rules and capability | groupage | `INDETERMINATE` | 0.3B | FROZEN_ACCEPTED |
| stackability | nullable flag | does not create positions | no automatic reject | 0.3B | FROZEN_ACCEPTED |
| fragility | nullable flag | rule required | no automatic reject | 0.3B | FROZEN_ACCEPTED |
| odor | B2 rules | groupage | `INDETERMINATE` | 0.3B | FROZEN_ACCEPTED |
| contamination | B2 rules | groupage | `INDETERMINATE` | 0.3B | FROZEN_ACCEPTED |
| cargo type | `cargo_type_code` | catalog rules | `INDETERMINATE` | 0.3B | FROZEN_ACCEPTED |
| same O-D | canonical `location_id` equality | pair filter plus window overlap | `ORIGIN_IDENTITY_UNPROVEN` or `DESTINATION_IDENTITY_UNPROVEN` | 0.3B | FROZEN_ACCEPTED |
| multi-pick | planning sequence | deferred | `PLAN_ONLY` | 0.4 | DEFER_TO_0_4 |
| multi-drop | planning sequence | deferred | `PLAN_ONLY` | 0.4 | DEFER_TO_0_4 |
| current-trip fill | server-built `CurrentTripContext` | one extra load | `INDETERMINATE` without onboard proof | 0.3D | FROZEN_ACCEPTED |
| cross-shipper | owner-persisted opt-in flags | cross-shipper flag on each load; independent of same-owner flag | `INDETERMINATE` `MULTI_PARTY_REFERENCE_CONTEXT_UNAVAILABLE`; never `FEASIBLE` in 0.3B | 0.3B | FROZEN_ACCEPTED |
| privacy | safe views | other shipper fields and internal location ids omitted | n/a | 0.3B | FROZEN_ACCEPTED |
| time windows | known pickup and delivery intervals | overlap required; disjoint is hard reject | `INDETERMINATE` if a required window is unknown | 0.3B | FROZEN_ACCEPTED |
| road detour | routing port | not Haversine | `SERVICE_UNAVAILABLE` | 0.3D | FROZEN_ACCEPTED |
| GPS freshness | tracking-service status | `FRESH` usable; other statuses block insertion | `INDETERMINATE` | 0.3D | FROZEN_ACCEPTED |
| ETA | tracking-service `freshnessStatus` | only `FRESH` may prove a window | `INDETERMINATE` | 0.3D | FROZEN_ACCEPTED |
| slot dependency | tracking slot state | NLO does not book | `PLAN_ONLY` | 0.3D | FROZEN_ACCEPTED |
| rehandling | placement check | forbidden conflict rejects | `NOT_EVALUATED` otherwise | 0.3D | FROZEN_ACCEPTED |
| unknown data | all of the above | required unknown is not `FEASIBLE` | `INDETERMINATE` | 0.3B | FROZEN_ACCEPTED |

## Future tests

Known and unknown residual weight. Known and unknown pallets, with unknown not equal to zero. Mixed pallet equivalence. Linear, volume, and weight exhaustion. Temperature intersection and conflict. Multi-zone indeterminate. ADR capability false, unknown, and missing rule. Food-grade, odor, and contamination conflicts. Required side and top loading. Cargo-cargo hard deny and tenant hard deny. `REQUIRE_SEPARATION` left indeterminate. Stale GPS and stale ETA. Pickup and delivery window misses. Road insertion distance. Haversine not used as road distance. Same-origin pair. One additional current-trip load. Missing cross-shipper opt-in. Cross-shipper privacy. `NETWORK_OPTIMIZATION_ONLY` absent from the human marketplace. Input version invalidation. Deterministic generation. Bounded set size. Multi-stop execution blocked.

## Scale plan, not a claim

Later benchmarks: 10, 50, 100, and 500 candidate loads. Record pairs, sets explored, groupage calls, matrix calls, duration p50 and p95, determinism, and memory. Discovery states no production performance number.

## Security

Tenant isolation stays on owner predicates and gateway headers. No IDOR lookup of another tenant's shipment. No browser tenant header. Internal token for service reads. No cross-tenant raw scan. Anonymous geography stays coarse. Source ids, commercial amounts, customer identity, and driver identity stay out of the other shipper's view.

## Acceptance

| Gate | Result |
| --- | --- |
| NLO03_ARCH_001 current execution inventory | PASS |
| NLO03_ARCH_002 current-trip semantics | PASS |
| NLO03_ARCH_003 residual capacity | PASS |
| NLO03_ARCH_004 onboard cargo evidence | PASS_WITH_FINDING: documented implementation dependency; no unit-level onboard fact exists, so residual stays unknown until shipment-service adds it |
| NLO03_ARCH_005 B2 groupage reuse | PASS |
| NLO03_ARCH_006 N-way compatibility | PASS |
| NLO03_ARCH_007 temperature | PASS |
| NLO03_ARCH_008 ADR | PASS |
| NLO03_ARCH_009 pallets and linear metres | PASS |
| NLO03_ARCH_010 load and unload access | PASS |
| NLO03_ARCH_011 rehandling | PASS |
| NLO03_ARCH_012 route insertion | PASS |
| NLO03_ARCH_013 time windows | PASS |
| NLO03_ARCH_014 tracking freshness | PASS_WITH_FINDING: tracking-service owns runtime thresholds; BNO consumes status; `BNO_PREDICTION_MAX_ETA_AGE` remains a separate NLO-0.2 gate |
| NLO03_ARCH_015 cross-shipper opt-in | PASS: `consolidation_allowed` and `cross_shipper_consolidation_allowed` are owner-controlled columns in migration `000081`. Opt-in is implemented. Full compatibility proof is not claimed, and a cross-shipper pair is not `FEASIBLE` |
| NLO03_ARCH_016 privacy | PASS |
| NLO03_ARCH_017 network-optimization-only pool | PASS |
| NLO03_ARCH_018 execution gate | PASS |
| NLO03_ARCH_019 candidate model | PASS |
| NLO03_ARCH_020 version pinning | PASS |
| NLO03_ARCH_021 invalidation | PASS |
| NLO03_ARCH_022 API plan | PASS |
| NLO03_ARCH_023 event plan | PASS |
| NLO03_ARCH_024 audit | PASS |
| NLO03_ARCH_025 failure modes | PASS |
| NLO03_ARCH_026 bounded search | PASS |
| NLO03_ARCH_027 solver deferred | PASS |
| NLO03_ARCH_028 implementation waves | PASS |
| NLO03_ARCH_029 traceability | PASS |
| NLO03_ARCH_030 security | PASS |
| NLO03_ARCH_031 trusted context server-built | PASS |
| NLO03_ARCH_032 freshness policy owner is tracking-service | PASS |
| NLO03_ARCH_033 no first-wave partial feasibility | PASS |
| NLO03_ARCH_034 onboard evidence owner is shipment-service | PASS |
| NLO03_ARCH_035 same O-D canonical identity | PASS |
| NLO03_ARCH_036 same O-D window compatibility | PASS |
| NLO03_ARCH_037 opt-in owner-controlled | PASS |
| NLO03_ARCH_038 implementation order unblocked | PASS |

## Explicit exclusions

MILP no. CP-SAT no. VRP solver no. LNS no. ML no. Genetic algorithm no. 3D bin packing no. Unbounded subset enumeration no. Shipment, order, slot, driver, reservation, assignment, offer, tender, billing, and EDO writes no.
