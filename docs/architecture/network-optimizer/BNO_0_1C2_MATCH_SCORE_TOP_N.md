# BNO-0.1C2 Match score and top N

Status: implemented on `feat/bno-match-score-topn-v0.1c2` and waiting for controller review. BNO-0.1C is not complete.

## Question this stage answers

BNO-0.1C1 decides whether a capacity can execute a load. BNO-0.1C2 orders the hard-feasible candidates and says why. A rejected candidate stays rejected. Its score and rank are null, and its score status is `NOT_APPLICABLE`. A high commercial score cannot repair a hard reject.

Eligible candidates that lack the evidence required by the selected objective stay eligible and become `UNRANKED`. They are not low-score candidates.

## Algorithm

Scoring algorithm version: `bno-score-0.1c2.1`.

The version is stored on every search run. Component formulas live in code. Profile composition and weights do not. The service loads the one `ACTIVE` `SYSTEM` profile for the requested code from `network_optimizer.score_profiles` and `network_optimizer.score_profile_components`.

Canonical component points and the final score are integers from 0 to 10000. Raw source metrics may stay floating point. Rounding is half away from zero for normalization, and half up for the integer weighted total.

Pool-relative normalization uses only hard-feasible candidates that have a value for that component:

- higher is better: `(value - min) / (max - min) * 10000`
- lower is better: `(max - value) / (max - min) * 10000`

If `min == max`, every candidate with that component receives 5000 points.

A missing required component makes the candidate `UNRANKED`. A missing optional component is dropped from the denominator. Unknown is not zero.

`score_evidence_bps = active_weight_bps / profile_weight_bps * 10000`.

The final score is the weight-normalized sum of the available component points.

## Profiles

| Code | Status | Weights |
| --- | --- | --- |
| `MIN_DEADHEAD` | executable | `DEADHEAD_EFFICIENCY` 10000 required |
| `MAX_CAPACITY_UTILIZATION` | executable | `CAPACITY_UTILIZATION` 10000 required |
| `RETURN_HOME` | executable | `TARGET_PROXIMITY` 10000 required |
| `MAX_REVENUE` | executable | `REVENUE` 10000 required |
| `MIN_RISK` | executable | `PICKUP_SLACK` 5000 required, `PREDICTION_CONFIDENCE` 3000 optional, `ETA_UNCERTAINTY` 2000 optional |
| `BALANCED` | executable | deadhead 3500 required, utilization 2500, waiting 1500, target proximity 1000, pickup slack 1000, network value 500 |
| `MAX_CONTRIBUTION` | reserved | not executable |

`MAX_CONTRIBUTION` returns `OBJECTIVE_PROFILE_NOT_EXECUTABLE` with detail `PLANNING_COST_PROVIDER_NOT_IMPLEMENTED`. It is not rewritten as `MAX_REVENUE`. BNO does not invent rub/km, fuel, driver, toll, waiting, or risk money.

There is no tenant profile API in this stage. An active system profile's configured weights sum to 10000. HTTP requests cannot submit arbitrary weights.

## Components

- `DEADHEAD_EFFICIENCY` uses canonical road deadhead kilometres from C1. Lower is better. Haversine is not deadhead.
- `CAPACITY_UTILIZATION` reuses BNO-0.1B2 `CapacityUsage`. The raw value is the maximum proven ratio among weight, volume, pallet positions, and linear metres. A missing dimension is not zero. If none is provable, the component is unavailable.
- `WAITING_EFFICIENCY` is lower-better. A feasible pickup window makes waiting known, including zero minutes. A completely absent pickup window is unknown.
- `PICKUP_SLACK` is higher-better: pickup window end minus arrival. A missing end is unavailable. Slack is not treated as infinite.
- `TARGET_PROXIMITY` is lower-better road distance from delivery to the explicit `target_location_id`. `RETURN_HOME` requires that target in every search mode, including `RADIUS`. There is no hidden home base. For `BALANCED`, a missing target only makes this component unavailable.
- `REVENUE` is higher-better and uses only the published load commercial input. It is available only when the amount is present and the currency equals `ranking_currency`. No FX conversion exists (`FX_CONVERSION_IMPLEMENTED=NO`).
- `PREDICTION_CONFIDENCE` and `ETA_UNCERTAINTY` come from the current predicted capacity. Manual capacity does not receive an invented confidence of 1.
- `NETWORK_VALUE` is the frozen deterministic fallback: raw 0, status `FALLBACK`, explanation that network value is not yet observed. Zero is not a measured neutral.

## Ranking currency

`ranking_currency` is an ordinary policy field: request, then capacity, then carrier. It is empty or exactly three uppercase ASCII letters. `MAX_REVENUE` requires it. It is not a hard maximum and it is not a rate card.

## Order and top N

Ranked candidates sort by score descending, evidence descending, road deadhead ascending, then load id ascending. Rank 1..N is assigned before truncation. There is no random or clock tie-break.

`candidate_limit` is top N after scoring:

- a positive limit returns at most that many ranked candidates and does not backfill with unranked candidates
- zero returns an empty candidate list while the counts stay complete
- an omitted limit returns every ranked candidate, then every unranked eligible candidate in load id order

The response keeps `eligible_candidate_count` and `rejection_counts_by_reason`, and adds ranked, unranked, and returned counts plus `unranked_counts_by_reason`. Ranking metadata carries the profile code, version, algorithm version, profile fingerprint, and ranking currency when set.

## Privacy

Internal scoring may use exact road distances. An anonymized public component omits exact deadhead and delivery-to-target kilometres. The explanation may name the existing deadhead bucket and must not contain coordinates, location ids, or exact anonymous geography. A commercial raw amount is returned only when the marketplace view already exposes that amount.

The per-candidate score fingerprint is audit data. It is stored and is not part of the public response. It covers the algorithm, profile fingerprint, policy fingerprint, capacity and load versions, compatibility fingerprint, raw inputs, pool bounds, and ranking currency. It does not include the current time or a random id.

## Persistence

Migration `000080_bno_match_score_topn_v0_1c2` extends the C1 search run and match candidate tables and adds the profile tables. Existing eligible C1 rows are marked `UNRANKED` with null score and rank so the new checks do not reject them. Ranked rows require rank and score. Unranked and rejected rows require both to be null. Score values outside 0..10000 are rejected. Rank is unique per search run when present.

`SaveSearch` writes the run, rejected candidates, ranked candidates, and unranked candidates in one transaction.

## Out of scope

No ML, learned weights, global solver, VRP, consolidation, chain optimizer, carrier offer, assignment, reservation, RFx, freight-cost ledger write, or FX provider. One match candidate remains one capacity plus one load. The endpoint remains `POST /v1/network/next-load/search`.
