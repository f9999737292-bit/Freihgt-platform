# NLO-0.3 implementation roadmap

Baseline: `origin/main` `6d47cc92`. NLO-0.3A is a freeze. No wave below is authorized.

## What NLO-0.3 will implement

Planning feasibility for:

- a same-origin, same-destination pair of published loads;
- one confirmed onboard cargo projection plus exactly one additional published load.

Both stay `PROPOSED`. Neither mutates a shipment.

## What NLO-0.4 owns

Persistent `RoutePlan` / `RouteStop` execution, multi-stop shipment representation, shared route legs, accepted-plan activation, and driver multi-stop tasks. ADR-NET-005 stays the gate. NLO-0.3 does not pull that work forward.

## What stays NLO-0.5 and later

Backhaul and round trip, regional and urban routing with sourced city rules, chain reoptimization, network-level optimization, and any solver. Consolidation ranking weights, if ever added, are a later authorized profile, not NLO-0.3B–D.

## Waves

### NLO-0.3B — current trip context and residual capacity

Scope: internal ports for shipment cargo projection, vehicle totals, tracking position, and tracking ETA. Build `CurrentTripContext` and `ResidualCapacitySnapshot` in memory or a planning table only when an implementation task authorizes a migration. Do not number `000081` in this freeze.

API: none public, or a read of the context for the owning carrier if the implementation task says so.

Dependencies: onboard evidence. Until a unit-level loaded fact exists, residual stays `UNKNOWN` and the context says why.

Security: tenant-scoped internal token. No cross-tenant cargo scan.

Tests: known versus unknown weight, unknown pallets not treated as zero, provenance labels.

Out: search, pairing, shipment writes.

### NLO-0.3C — pairwise same-origin feasibility

Scope: two published loads, same owner or both cross-shipper flags true, same origin and destination, `EvaluateGroupage`, windows that do not need a new stop.

API: `POST /v1/network/consolidation/search` with pattern `SAME_ORIGIN_SAME_DESTINATION`.

Migration: candidate persistence only if the implementation task authorizes it.

Security: opt-in fail-closed. Marketplace list unchanged.

Tests: pair hard deny, temperature intersection and conflict, privacy of the other shipper, opt-in absent, version invalidation, deterministic order.

Out: route insertion, execution, score.

### NLO-0.3D — one additional current-trip load

Scope: context from 0.3B plus one load. Road insertion via the routing port. GPS and ETA freshness from tracking policies. `execution_supported=false`.

API: same search endpoint with pattern `CURRENT_TRIP_FILL` and `MAX_ADDITIONAL_LOADS=1`.

Dependencies: 0.3B and a fresh position. Without onboard proof the result is `INDETERMINATE`.

Tests: stale GPS, stale ETA, window miss, road distance not Haversine, one extra load, multi-stop not activated.

Out: a second extra load, solver, slot booking.

### NLO-0.3E — bounded expansion and optional ranking

Scope: only after 0.3C and 0.3D measurements. Still no solver. A separate score, if authorized, is not MatchScore.

Out of NLO-0.3E: unrestricted `2^N`, ML, 3D packing, shipment activation.

## Traceability

| Requirement | Source | Check | Unknown | Wave | Status |
| --- | --- | --- | --- | --- | --- |
| weight | vehicle `capacity_weight`, cargo `gross_weight` | residual subtraction | `INDETERMINATE` | 0.3B | FROZEN |
| volume | vehicle `capacity_volume`, cargo `volume` | residual subtraction | `INDETERMINATE` | 0.3B | FROZEN |
| pallet count | `cargoes.pallet_count` | not coerced to zero | `INDETERMINATE` | 0.3B | FROZEN |
| pallet type | `pallet_type_code` plus equivalences | explicit positive factor only | `INDETERMINATE` | 0.3C | FROZEN |
| linear metres | cargo and `usable_linear_meters` | residual subtraction | `INDETERMINATE` | 0.3B | FROZEN |
| height | `max_loaded_height_mm`, internal height | comparison | `INDETERMINATE` | 0.3C | FROZEN |
| body and trailer type | B2 body check | groupage | `INDETERMINATE` | 0.3C | FROZEN |
| rear, side, top loading | access lists | independent | `INDETERMINATE` | 0.3C | FROZEN |
| unloading access | access lists | independent | `INDETERMINATE` | 0.3C | FROZEN |
| temperature | cargo interval, one zone | common intersection | `INDETERMINATE` | 0.3C | FROZEN |
| multi-zone | `TemperatureZoneCount` | existing indeterminate code | `INDETERMINATE` | 0.3C | FROZEN |
| food grade | B2 rules | groupage | `INDETERMINATE` | 0.3C | FROZEN |
| ADR | sourced rules and capability | groupage | `INDETERMINATE` | 0.3C | FROZEN |
| stackability | nullable flag | does not create positions | no automatic reject | 0.3C | FROZEN |
| fragility | nullable flag | rule required | no automatic reject | 0.3C | FROZEN |
| odor | B2 rules | groupage | `INDETERMINATE` | 0.3C | FROZEN |
| contamination | B2 rules | groupage | `INDETERMINATE` | 0.3C | FROZEN |
| cargo type | `cargo_type_code` | catalog rules | `INDETERMINATE` | 0.3C | FROZEN |
| same O-D | load pickup and delivery identity | pair filter | not a pair | 0.3C | FROZEN |
| multi-pick | planning sequence | deferred | `PLAN_ONLY` | 0.3E or 0.4 | DEFERRED |
| multi-drop | planning sequence | deferred | `PLAN_ONLY` | 0.3E or 0.4 | DEFERRED |
| current-trip fill | `CurrentTripContext` | one extra load | `INDETERMINATE` without onboard proof | 0.3D | FROZEN |
| cross-shipper | opt-in flags | both true | excluded | 0.3C | FROZEN |
| privacy | safe views | other shipper fields omitted | n/a | 0.3C | FROZEN |
| time windows | load windows plus road time | miss is hard when proven | `INDETERMINATE` if ETA stale | 0.3D | FROZEN |
| road detour | routing port | not Haversine | `SERVICE_UNAVAILABLE` | 0.3D | FROZEN |
| GPS freshness | tracking 10/30 | stale blocks insertion | `INDETERMINATE` | 0.3D | FROZEN |
| ETA | tracking 15/60 and lookup | stale blocks windows | `INDETERMINATE` | 0.3D | FROZEN |
| slot dependency | tracking slot state | NLO does not book | `PLAN_ONLY` | 0.3D | FROZEN |
| rehandling | placement check | forbidden conflict rejects | `NOT_EVALUATED` otherwise | 0.3D | FROZEN |
| unknown data | all of the above | no silent pass | `INDETERMINATE` | 0.3B | FROZEN |

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
| NLO03_ARCH_004 onboard cargo evidence | PASS_WITH_FINDING: no unit-level onboard fact exists, so residual stays unknown |
| NLO03_ARCH_005 B2 groupage reuse | PASS |
| NLO03_ARCH_006 N-way compatibility | PASS |
| NLO03_ARCH_007 temperature | PASS |
| NLO03_ARCH_008 ADR | PASS |
| NLO03_ARCH_009 pallets and linear metres | PASS |
| NLO03_ARCH_010 load and unload access | PASS |
| NLO03_ARCH_011 rehandling | PASS |
| NLO03_ARCH_012 route insertion | PASS |
| NLO03_ARCH_013 time windows | PASS |
| NLO03_ARCH_014 tracking freshness | PASS_WITH_FINDING: tracking policies exist; BNO prediction uses a separate max-age setting |
| NLO03_ARCH_015 cross-shipper opt-in | PASS_WITH_FINDING: flags are frozen and are not columns yet |
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

## Explicit exclusions

MILP no. CP-SAT no. VRP solver no. LNS no. ML no. Genetic algorithm no. 3D bin packing no. Unbounded subset enumeration no. Shipment, order, slot, driver, reservation, assignment, offer, tender, billing, and EDO writes no.
