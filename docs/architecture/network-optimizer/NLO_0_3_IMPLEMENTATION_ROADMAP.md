# NLO-0.3 implementation roadmap

Baseline: `origin/main` `6d47cc92`. NLO-0.3A is an architecture-freeze candidate, proposed and pending controller acceptance. No wave below is authorized. GPS is not an NLO-0.2 dependency. Current-trip residual capacity is NLO-0.3.

## What NLO-0.3 will implement

Planning feasibility for:

- first, a same-origin, same-destination pair of published loads, with owner-controlled opt-in and no current-trip dependency;
- later, one confirmed onboard cargo projection plus exactly one additional published load, after unit-level onboard evidence exists.

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

The first product wave is pairwise consolidation because unit-level onboard evidence does not exist. A current-trip wave started first could not return a useful `FEASIBLE` result.

### NLO-0.3B — pairwise same-origin foundation

Scope: `PAIRWISE_CONSOLIDATION_ONLY`, set size 2. Owned effective capacity plus two explicitly published loads. Canonical same O-D, overlapping windows, owner-controlled opt-in, `EvaluateGroupage`. Planning only. No score. No shipment mutation. No solver. No current-trip residual dependency.

API: `POST /v1/network/consolidation/search` with pattern `SAME_ORIGIN_SAME_DESTINATION` and an owned `capacity_id`.

Security: capacity tenant ownership, status, version, and effective capability. A foreign capacity is `NOT_FOUND`. Marketplace list unchanged.

Tests: canonical id match, label mismatch excluded, unproven origin, overlapping and disjoint windows, opt-in absent, cross-shipper without `consolidation_allowed`, pair hard deny, privacy of the other shipper, version invalidation, deterministic order.

Out: current-trip context, residual occupancy, route insertion, execution, score.

### NLO-0.3C — onboard evidence, trip context, residual snapshot

Scope: shipment-service owns the future `ShipmentOnboardCargoProvider`. BNO consumes it. Build server-side `CurrentTripContext` and `ResidualCapacitySnapshot` from trusted ports. Do not number `000081` in this candidate. `NEW_EXECUTION_EVIDENCE_REQUIRED=YES` before a residual result can be `FEASIBLE`.

API: none that treats caller residual facts as authority.

Dependencies: a new execution fact. Shipment status, linked cargo, and planned quantity do not qualify. Driver events in shipment-service are shipment-scoped and do not name a cargo unit.

Security: trusted tenant, owned shipment, `NOT_FOUND` for a foreign id. No browser tenant header.

Tests: `CONFIRMED_ONBOARD` counts, `PLANNED` does not, unknown dimension blocks `FEASIBLE`.

Out: load insertion, shipment writes.

### NLO-0.3D — one additional current-trip load

Scope: context from 0.3C plus one published load. Road insertion via the routing port. Location and ETA decisions follow the freshness status returned by tracking-service. `execution_supported=false`.

API: same search endpoint with pattern `CURRENT_TRIP_FILL`, `shipment_id` or a server-created `current_trip_context_id`, and `MAX_ADDITIONAL_LOADS=1`.

Dependencies: 0.3C and a `FRESH` position when insertion needs a position. Without `CONFIRMED_ONBOARD` the result is `INDETERMINATE`.

Tests: stale, lost, and unknown location status, non-fresh ETA, window miss, road distance not Haversine, one extra load, caller residual facts ignored, multi-stop not activated.

Out: a second extra load, solver, slot booking.

### NLO-0.3E — bounded expansion and optional ranking

Scope: only after 0.3B, 0.3C, and 0.3D measurements. Still no solver. A separate score, if authorized, is not MatchScore.

Out of NLO-0.3E: unrestricted `2^N`, ML, 3D packing, shipment activation.

## Traceability

| Requirement | Source | Check | Unknown | Wave | Status |
| --- | --- | --- | --- | --- | --- |
| weight | vehicle `capacity_weight`, cargo `gross_weight` | groupage in 0.3B; residual subtraction in 0.3C | `INDETERMINATE` | 0.3B / 0.3C | PROPOSED |
| volume | vehicle `capacity_volume`, cargo `volume` | groupage in 0.3B; residual subtraction in 0.3C | `INDETERMINATE` | 0.3B / 0.3C | PROPOSED |
| pallet count | `cargoes.pallet_count` | not coerced to zero | `INDETERMINATE` | 0.3B / 0.3C | PROPOSED |
| pallet type | `pallet_type_code` plus equivalences | explicit positive factor only | `INDETERMINATE` | 0.3B | PROPOSED |
| linear metres | cargo and `usable_linear_meters` | groupage in 0.3B; residual subtraction in 0.3C | `INDETERMINATE` | 0.3B / 0.3C | PROPOSED |
| height | `max_loaded_height_mm`, internal height | comparison | `INDETERMINATE` | 0.3B | PROPOSED |
| body and trailer type | B2 body check | groupage | `INDETERMINATE` | 0.3B | PROPOSED |
| rear, side, top loading | access lists | independent | `INDETERMINATE` | 0.3B | PROPOSED |
| unloading access | access lists | independent | `INDETERMINATE` | 0.3B | PROPOSED |
| temperature | cargo interval, one zone | common intersection | `INDETERMINATE` | 0.3B | PROPOSED |
| multi-zone | `TemperatureZoneCount` | existing indeterminate code | `INDETERMINATE` | 0.3B | PROPOSED |
| food grade | B2 rules | groupage | `INDETERMINATE` | 0.3B | PROPOSED |
| ADR | sourced rules and capability | groupage | `INDETERMINATE` | 0.3B | PROPOSED |
| stackability | nullable flag | does not create positions | no automatic reject | 0.3B | PROPOSED |
| fragility | nullable flag | rule required | no automatic reject | 0.3B | PROPOSED |
| odor | B2 rules | groupage | `INDETERMINATE` | 0.3B | PROPOSED |
| contamination | B2 rules | groupage | `INDETERMINATE` | 0.3B | PROPOSED |
| cargo type | `cargo_type_code` | catalog rules | `INDETERMINATE` | 0.3B | PROPOSED |
| same O-D | canonical `location_id` equality | pair filter plus window overlap | `ORIGIN_IDENTITY_UNPROVEN` or `DESTINATION_IDENTITY_UNPROVEN` | 0.3B | PROPOSED |
| multi-pick | planning sequence | deferred | `PLAN_ONLY` | 0.3E or 0.4 | DEFERRED |
| multi-drop | planning sequence | deferred | `PLAN_ONLY` | 0.3E or 0.4 | DEFERRED |
| current-trip fill | server-built `CurrentTripContext` | one extra load | `INDETERMINATE` without onboard proof | 0.3D | PROPOSED |
| cross-shipper | owner-persisted opt-in flags | cross-shipper flag on each load; independent of same-owner flag | excluded | 0.3B | PROPOSED |
| privacy | safe views | other shipper fields and internal location ids omitted | n/a | 0.3B | PROPOSED |
| time windows | known pickup and delivery intervals | overlap required; disjoint is hard reject | `INDETERMINATE` if a required window is unknown | 0.3B | PROPOSED |
| road detour | routing port | not Haversine | `SERVICE_UNAVAILABLE` | 0.3D | PROPOSED |
| GPS freshness | tracking-service status | `FRESH` usable; other statuses block insertion | `INDETERMINATE` | 0.3D | PROPOSED |
| ETA | tracking-service `freshnessStatus` | only `FRESH` may prove a window | `INDETERMINATE` | 0.3D | PROPOSED |
| slot dependency | tracking slot state | NLO does not book | `PLAN_ONLY` | 0.3D | PROPOSED |
| rehandling | placement check | forbidden conflict rejects | `NOT_EVALUATED` otherwise | 0.3D | PROPOSED |
| unknown data | all of the above | required unknown is not `FEASIBLE` | `INDETERMINATE` | 0.3B | PROPOSED |

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
| NLO03_ARCH_015 cross-shipper opt-in | PASS_WITH_FINDING: flags are proposed owner-controlled facts and are not columns yet |
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
