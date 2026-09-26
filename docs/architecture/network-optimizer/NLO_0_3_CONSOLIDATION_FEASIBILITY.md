# NLO-0.3 consolidation feasibility

Baseline: `origin/main` `6d47cc92`. Compatibility calls `compat.EvaluateGroupage`. No second engine.

## Problem classes

| Pattern | Decision | Why |
| --- | --- | --- |
| `SAME_ORIGIN_SAME_DESTINATION` | FIRST_PRODUCT_WAVE, PLAN_ONLY | No current-trip dependency. Execution still cannot store two orders on one shipment |
| `CURRENT_TRIP_FILL` with one additional load | AFTER_ONBOARD_EVIDENCE, PLAN_ONLY | Blocked from `FEASIBLE` until shipment-service proves `CONFIRMED_ONBOARD` |
| `MULTI_PICK_ONE_DROP` | DEFER, PLAN_ONLY | Needs insertion and still cannot execute |
| `ONE_PICK_MULTI_DROP` | DEFER, PLAN_ONLY | Same |
| `MULTI_PICK_MULTI_DROP` | DEFER, and BLOCKED_BY_EXECUTION_MODEL for activation | NLO-0.4 |
| `HUB_CONSOLIDATION` | DEFER | No hub stop in the shipment schema |
| `CROSS_DOCK` | DEFER | No cross-dock execution fact |

`ROUTE_FEASIBILITY` and `EXECUTION_MODEL_SUPPORT` are separate. A same-origin pair can pass groupage and still be non-executable.

## Same origin and same destination

Two loads are the same O-D only when internal canonical identifiers match:

`load A pickup.location_id == load B pickup.location_id` and `load A delivery.location_id == load B delivery.location_id`.

Label text, city, region, coarse anonymized geography, rounded coordinates, and Haversine proximity are not equality. If a canonical id is missing, the pair is not same O-D. Exclude it with `ORIGIN_IDENTITY_UNPROVEN` or `DESTINATION_IDENTITY_UNPROVEN`. Do not guess. Internal ids may be compared inside the trust zone. Anonymized human output does not reveal those ids. Coarse display stays as it is.

Same locations are not enough. Two known pickup windows must overlap in a shared service interval. Two known delivery windows must overlap in a shared delivery interval. A proven disjoint required window is `HARD_REJECT`. A required window that is unknown is `INDETERMINATE`. Do not replace two windows with one invented timestamp.

## N-way hard feasibility

For a set, `EvaluateGroupage` already does:

- every cargo against equipment;
- every unordered cargo pair;
- aggregate weight, volume, pallets, and linear metres;
- one temperature outcome;
- ADR from sourced rules.

NLO-0.3 consumes that result. One hard reject makes the set `HARD_REJECT`. One required indeterminate condition blocks `FEASIBLE`. Incompatibilities are not averaged into a score.

`REQUIRE_SEPARATION` and `REQUIRE_CONDITION` stay explicit. Suggested condition codes when the B2 reason is one of those decisions:

- `PHYSICAL_PARTITION_REQUIRED`
- `ZONE_ALLOCATION_REQUIRED`
- `REGULATORY_CONDITION_UNPROVEN`

The first implementation has no partition allocator. Those conditions remain `INDETERMINATE`.

## Temperature

Single zone: required intervals must have a non-empty intersection. `+2..+8` with `+4..+6` yields `+4..+6`. `+2..+8` with `-25..-18` is `HARD_REJECT` when the equipment has one zone (`TEMPERATURE_RANGES_INCOMPATIBLE` / `MULTI_ZONE_REQUIRED` in `compat/eval.go`).

More than one zone: existing code returns indeterminate `MULTI_ZONE_ALLOCATION_REQUIRED` when independent control is true. NLO-0.3 does not add a zone allocator. The set is not `FEASIBLE`.

## ADR, odor, food, contamination

No new ADR matrix. `adr_capability=false` with dangerous cargo is `HARD_REJECT`. Unknown ADR capability is `INDETERMINATE`. `adr_capability=true` without an applicable sourced rule is `INDETERMINATE`. A sourced hard deny is `HARD_REJECT`. Cross-cargo segregation uses only sourced rules.

Odor, food grade, and contamination use B2 rules, including tenant rules that may tighten a result. A tenant rule cannot override a platform or regulatory hard deny. Product names are not hardcoded.

## Access, placement, windows, road

Loading and unloading access are independent. The groupage evaluator already returns `LOADING_ACCESS_UNSUPPORTED` or `LOADING_ACCESS_UNKNOWN`.

`placement_check` values:

| Value | Meaning |
| --- | --- |
| `NOT_EVALUATED` | Default in the first waves. No 3D pack |
| `SEQUENCE_OK` | Stop order alone does not block unload of earlier cargo |
| `SEQUENCE_CONFLICT` | A later delivery is physically behind an earlier one and rehandling was proven necessary |
| `REHANDLE_REQUIRED` | Sequence conflict and rehandling is the only continuation |

If rehandling is forbidden and `SEQUENCE_CONFLICT` is proven, the set is `HARD_REJECT`. If the sequence cannot be proven, the result is not `FULL_PHYSICAL_FIT`. It stays `NOT_EVALUATED` and the candidate is `INDETERMINATE` or `PLAN_ONLY`. The first waves do not claim `SEQUENCE_OK` unless the pattern has no insertion (same origin and same destination, one shared drop).

Time windows use the load pickup and delivery windows already on `LoadOpportunity`. A miss that is proven from road time is `HARD_REJECT`. A miss that depends on a stale ETA is `INDETERMINATE`.

Road distance and duration come from the routing provider. Haversine is a prefilter only.

## Candidate and versions

`ConsolidationCandidate` fields, planning only, no table in this phase:

id, current-trip or empty-capacity context, member load and cargo references, member versions, owner tenant ids for internal audit only, pattern, compatibility status, capacity aggregation, residual after the set, routing feasibility, time-window feasibility, placement check, conditions, hard-reject reasons, catalog and rule-set fingerprints, route assumptions, status, created_at.

Pins: capacity or trip version, shipment version, vehicle version, load version, cargo projection version, catalog versions, rule-set versions, routing provider and request fingerprint. A changed load version invalidates the candidate.

Statuses: `HARD_REJECT`, `INDETERMINATE`, `FEASIBLE`, `PROPOSED`, `INVALIDATED`. Not `ASSIGNED`. Not `ACTIVE`. `FEASIBLE` is not `EXECUTABLE` when the shipment model cannot store the plan.

## Search bound

N candidate loads have `2^N` subsets. That is not the default. First waves:

- `PAIRWISE_CONSOLIDATION_ONLY` is the first product wave. Set size is 2. Members are explicitly published loads with canonical same O-D and compatible windows. No current-trip residual dependency.
- `MAX_ADDITIONAL_LOADS=1` is the later current-trip wave. The set is confirmed onboard cargo plus one load. It is not the first wave, because unit-level onboard evidence does not exist yet.

Ordering is load id ascending, then candidate id. Pool and duration limits stay unset until a later measurement. They are not invented as production constants.

## Score and commerce

First waves do not score the set. Future components, not frozen as weights: incremental detour, residual utilization, waiting, new stop count, risk. `MAX_CONTRIBUTION` stays reserved. Shipper amounts are not summed. No FX. No freight-cost ledger write.

## Failure matrix

| Condition | Result |
| --- | --- |
| routing provider unavailable | `SERVICE_UNAVAILABLE` for a check that needs road distance; no Haversine substitute |
| tracking position stale or unknown | `INDETERMINATE` for current-trip insertion |
| ETA unavailable or stale | `INDETERMINATE` for windows that need it |
| onboard cargo unknown | `INDETERMINATE`; residual unknown |
| vehicle capacity unknown for a required dimension | `INDETERMINATE` |
| pallet count or linear metres unknown when that check is required | `INDETERMINATE`; not zero |
| catalog unavailable or rule set invalid | `SERVICE_UNAVAILABLE` |
| ADR coverage missing | `INDETERMINATE` |
| multi-zone allocation required | `INDETERMINATE` |
| slot required and unknown | `PLAN_ONLY` or `INDETERMINATE`; not booked |
| execution model cannot store the plan | `PLAN_ONLY` even when checks pass |

Unknown facts do not pass.

## Documents

BNO cannot prove document readiness. Document constraints are `DEFERRED`. They do not block the feasibility foundation. EDO is not a runtime dependency of NLO-0.3.
