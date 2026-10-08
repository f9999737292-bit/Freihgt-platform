# NLO-0.5B4 final evaluation cap

The discovery conclusions stand. The invariant test locks them. Search still does not consult `CandidateFinalEvaluationCap`. There is no second runtime prune.

```text
NLO_0_5B4=IMPLEMENTED
FINAL_EVALUATION_CAP_INVARIANT_PROVEN=YES
FINAL_EVALUATION_CAP_RUNTIME_ENFORCED=NO
FINAL_EVALUATION_BOUND_SOURCE=ROUTING_SLICE_PLUS_UNKNOWN_ROAD_HARD_REJECT
FINAL_EVALUATION_DEFINITION=empty hard-reject list passed to ScorePool, producing RANKED or UNRANKED
FINAL_CAP_DECISION=ROUTING_BOUND_ALREADY_PROVES_FINAL_BOUND
SEPARATE_FINAL_CAP_CODE_REQUIRED=NO
FINAL_CAP_INVARIANT_TEST_REQUIRED=YES
SCORE_PERSISTENCE_DECISION=EXISTING_PERSISTENCE_SUFFICIENT
MIGRATION_REQUIRED_FOR_B4=NO
B4_IMPLEMENTATION_TYPE=INVARIANT_ONLY
```

Base of this reading: `67e72d37cdf8f658de04415b0ac2fc94e0dd408d`.

Frozen policy, unchanged:

```text
CANDIDATE_DISCOVERY_CAP=1000
CANDIDATE_ROUTING_CAP=25
CANDIDATE_FINAL_EVALUATION_CAP=25
MATRIX_CALL_CAP=4
ROUTE_CALL_CAP=2
PROVIDER_CALL_CAP=6
PROVIDER_REQUEST_TIMEOUT=5s
SEARCH_HARD_BUDGET=30s
RETRY_COUNT=0
```

`CandidateFinalEvaluationCap` is declared in `services/network-optimizer-service/internal/domain/routing_bound.go` and is read only by tests. Search does not consult it. ADR-NET-024 keeps `FINAL_EVALUATION_CAP_RUNTIME_ENFORCED=NO`. This discovery does not flip that marker.

## Final evaluation definition

The phrase is policy language in ADR-NET-024: the cap is frozen for a later scoring wave. The matching runtime stage is the set of next-load rows with an empty hard-reject list. `scoreEligible` sends only that set to `domain.ScorePool`. Each member then receives `RANKED` or `UNRANKED`.

| Alternative | Meaning | Matches this architecture |
| --- | --- | --- |
| A | Candidate sent to scoring | Yes, when scoring means `ScorePool` input. The full `evaluateLoads` slice is not that input. |
| B | Candidate surviving feasibility and compatibility | Same set as A and C. Empty reasons are the survival test. |
| C | Candidate producing `RANKED` or `UNRANKED` | Yes. This is the frozen definition. |
| D | Candidate persisted with any score-status column | No. `persistSearch` writes every evaluated row, including `REJECTED` / `NOT_APPLICABLE`. Those rows are audit, not final evaluation. |

```text
FINAL_EVALUATION_DEFINITION=empty hard-reject list passed to ScorePool, producing RANKED or UNRANKED
```

No new business stage is required.

## Pipeline bounds

`SearchNextLoad` order is `visibleLoads`, load-id sort, `evaluateLoads`, `scoreEligible`, `persistSearch`, `publicSearch` / `responseRows`. `candidate_limit` only shortens the HTTP candidate list. It does not add rows.

| Stage | Input max | Output max | Bound source | Ordering | Can fan out | Can reintroduce pruned candidates |
| --- | --- | --- | --- | --- | --- | --- |
| `visibleLoads` | Marketplace count | 1000 | `ListMarketplaceLoads` limit `CandidateDiscoveryCap`, then `boundDiscoveryCandidates` | `created_at DESC, id`; duplicate load ids dropped | NO | NO |
| Load-id sort | 1000 | 1000 | Same slice | Load id ascending | NO | NO |
| Cheap prefilter | 1000 | 1000 | First loop of `evaluateLoads` | Load id ascending | NO | NO |
| Routing selection | 1000 prefilter survivors | 25 | `prefiltered[:CandidateRoutingCap]` | First 25 in load-id order | NO | NO |
| `roadFacts` | 25 | 25 map entries | Same slice; function slices again if given more | Destination index of that slice | NO | NO |
| Feasibility on the full discovery set | 1000 | 1000 rows | Second loop walks every discovered load | Load id ascending | NO | NO |
| Compatibility | 1000 | 1000 | Called once per discovered load | Same row | NO | NO |
| `scoreEligible` / `ScorePool` | 25 reason-free rows | 25 scores | Rows with `len(reasons)==0` only | Rank: total desc, evidence bps desc, road km asc, load id asc | NO | NO |
| `responseRows` | 25 eligible | 25, or fewer when `candidate_limit` is set | Omits any row with reasons | Ranked by rank; unranked by load id | NO | NO |
| `persistSearch` | 1000 | 1000 stored rows | Every evaluated row | Evaluated order | NO | NO |

`MAX_PERSISTED_CANDIDATE_ROWS` is the discovery cap, not the final-evaluation cap. The extra rows are rejected audit rows.

```text
MAX_ROUTED_CANDIDATES=25
MAX_FINAL_EVALUATED_CANDIDATES=25
MAX_SCORED_CANDIDATES=25
MAX_RANKED_CANDIDATES=25
MAX_PERSISTED_CANDIDATE_ROWS=1000
```

## Why non-routed rows cannot be final-evaluated

Routing selection is not, by itself, the proof. The proof is the next gate.

1. `evaluateLoads` requests road facts only for `routed`.
2. The second loop still visits every discovered load, including prefilter survivors past the routing slice.
3. A prefilter survivor that is absent from the road-fact map has `deadheadKm == nil`. The loop appends `ROAD_DISTANCE_UNKNOWN`.
4. Early rejects (missing pickup coordinates, outside radius, corridor geometry) already have reasons and leave before that road lookup.
5. `scoreEligible` assigns `NOT_APPLICABLE` and skips `ScorePool` when `len(reasons) > 0`.
6. `responseRows` omits every row that still has reasons.
7. `AllowUnknownRoadDistance` is not read on this path. A nil road distance stays a hard reason.
8. Duplicate load ids are dropped in `boundDiscoveryCandidates`, so a pruned id cannot reappear under a second row.

A non-routed candidate therefore cannot remain eligible, receive a score, receive a rank, or appear as an eligible response candidate. It can still receive compatibility evaluation and a persisted `REJECTED` row. Those are not final evaluation.

`ScorePool` scores only the facts it is given. It does not load more candidates.

## Mode by mode

All three modes share the routing slice and the unknown-road reject. They do not share the cheap prefilter.

| Mode | Routing required | Road distance required | Final evaluation can exceed 25 | Scoring can exceed 25 |
| --- | --- | --- | --- | --- |
| `RADIUS` | YES | YES | NO | NO |
| `DIRECTIONAL_CORRIDOR` | YES | YES | NO | NO |
| `ROUTE_ELLIPSE` | YES | YES | NO | NO |

`RADIUS` rejects outside-radius loads before routing. Survivors still need a measured deadhead.

`DIRECTIONAL_CORRIDOR` rejects a missing route line, a failed projection, backtrack, excess lateral distance, and excess forward distance before routing. Survivors still need a measured deadhead. Corridor backhaul reasons, when applicable, are further hard rejects.

`ROUTE_ELLIPSE` has no cheap geometry prefilter. Every load with pickup coordinates enters `prefiltered`. Routing still keeps the first 25 by load id. The rest receive `ROAD_DISTANCE_UNKNOWN` and are not scored. Ellipse increase is evaluated only when the road facts for that routed load exist.

## Fan-out and reintroduction

`SearchNextLoad` does not call consolidation, current-trip fill, groupage, or a second-load composer. `roadFacts` does not append loads. A partial matrix keeps successful cells and leaves failed cells unknown; it does not add the pruned tail. `ProviderRetryCount` is 0 and this search has no retry loop. There is no Haversine fallback into eligibility. `HaversineCanonicalRoad` stays false.

Pruned prefilter survivors stay in the evaluated slice as rejected rows. They are not removed and then added back into the routed set, the score pool, or the eligible response.

```text
POST_ROUTING_FAN_OUT_FOUND=NO
PRUNED_CANDIDATE_REINTRODUCTION_FOUND=NO
```

## Cap decision

```text
FINAL_CAP_DECISION=ROUTING_BOUND_ALREADY_PROVES_FINAL_BOUND
SEPARATE_FINAL_CAP_CODE_REQUIRED=NO
FINAL_CAP_INVARIANT_TEST_REQUIRED=YES
```

The invariant is: a discovered load outside the routed slice always has a hard reason before `scoreEligible`, so `ScorePool` input, ranked rows, and eligible response rows cannot exceed 25 on `RADIUS`, `DIRECTIONAL_CORRIDOR`, or `ROUTE_ELLIPSE`.

The constant `CandidateFinalEvaluationCap` is still unused by search. A later edit that stopped appending `ROAD_DISTANCE_UNKNOWN` for a nil deadhead would let unscored routing survivors into `ScorePool`. The next implementation wave should lock the current bound with a test. It should not add a second prune.

```text
FINAL_CAP_APPLIED_BEFORE=not introduced
FINAL_CAP_APPLIED_AFTER=not introduced
FINAL_CAP_ORDER=existing routing order: load id ascending, first 25 cheap-prefilter survivors
```

If a future change made unknown road distance eligible, the only deterministic cut that preserves B1 and B2 order is the same load-id prefix already used for routing, applied before `ScorePool`. That cut is not part of this decision.

## Score persistence

Schema: `000079_bno_next_load_candidate_search_v0_1c1` creates `next_load_search_runs` and `match_candidates`. `000080_bno_match_score_topn_v0_1c2` adds score columns and the eligibility/score shape check. The repository types are `SearchRun` and `StoredCandidate` in `internal/repository/search_store.go`. `persistSearch` fills them.

| Fact | Persisted | Required for B4 | Where |
| --- | --- | --- | --- |
| Eligibility | YES | YES | `match_candidates.eligibility` |
| Reject reasons | YES | YES | `match_candidates.reject_reasons` |
| Road deadhead km and minutes | YES | YES | `road_deadhead_km`, `road_deadhead_minutes` |
| Waiting minutes | YES | YES | `waiting_minutes` |
| Compatibility status | YES | YES | `compatibility_status` |
| Compatibility fingerprint | YES | YES | `compatibility_fingerprint` |
| Policy fingerprint | YES | YES | Candidate `policy_fingerprint` and run `effective_policy_fingerprint` |
| Score status | YES | YES | `score_status` (`NOT_APPLICABLE`, `RANKED`, `UNRANKED`) |
| Score components | YES | YES | `score_components` |
| Unranked reason codes | YES | YES | `unranked_reason_codes` |
| Score fingerprint | YES | YES | `score_fingerprint` |
| Rank | YES | YES | `rank`, only when `RANKED` |
| Total score | YES | YES | `score_total`, only when `RANKED` |
| Evidence bps | YES | YES | `score_evidence_bps`, only when `RANKED` |
| Load version | YES | YES | `match_candidates.load_version` |
| Capacity version | YES | YES | `next_load_search_runs.capacity_version` |
| Profile code, version, fingerprint | YES | YES | Run columns from `000080` |
| Scoring algorithm version | YES | YES | `scoring_algorithm_version` |
| Ranking currency | YES | YES | `ranking_currency` |

The `000080` check requires rejected rows to stay `NOT_APPLICABLE` with null rank and null total. Eligible ranked rows must store rank, total, and evidence. Eligible unranked rows store status and reason codes with null rank and null total. That is the required shape.

Forward progress, lateral distance, and route increase are response fields. They are not required B4 score facts and are not added here. Prometheus counters for provider calls, routing selection, and pruned counts stay observability. They are not business KPI rows. Planning facts stay with Agent D. Analytics KPIs stay with Agent E.

```text
SCORE_PERSISTENCE_DECISION=EXISTING_PERSISTENCE_SUFFICIENT
MIGRATION_REQUIRED_FOR_B4=NO
```

## Local work for 1,000 discovered rows

```text
COMPATIBILITY_EVALUATIONS_MAX=1000
SCORE_INPUT_ROWS_MAX=25
PERSISTED_CANDIDATE_ROWS_MAX=1000
```

Compatibility runs once per discovered load, including early rejects and the unrouted tail. Scoring already stops at the reason-free set. Persistence writes the full evaluated slice.

The recommended invariant-only wave does not change that work.

```text
CPU_IMPACT=NONE
MEMORY_IMPACT=NONE
DB_WRITE_IMPACT=NONE
```

A separate persistence cap that dropped rejected audit rows would reduce candidate inserts from up to 1000 to 25 and would skip compatibility on the unrouted tail. That would be a product change to the audit record. It is not required to keep final evaluation at 25, and it is not recommended.

## Security and privacy

Discovery uses `marketplaceVisibilitySQL`: published loads, `owner_tenant_id` not equal to the viewer tenant, and visibility `MARKETPLACE`, `ANONYMIZED_MARKETPLACE`, or `INVITED_CARRIERS` for the viewer company. This discovery does not change own-load exclusion, private-load exclusion, tenant isolation, or anonymized geo handling in `candidateView` / `publicComponents`.

```text
SECURITY_MODEL_CHANGE_REQUIRED=NO
PRIVACY_MODEL_CHANGE_REQUIRED=NO
```

## Invariant test

`TestNLO05B4FinalEvaluationCapInvariant` locks the bound for `RADIUS`, `DIRECTIONAL_CORRIDOR`, and `ROUTE_ELLIPSE`. Each subtest discovers 1,000 marketplace loads, leaves more than 25 cheap-prefilter survivors, and gives road facts only to the loads the provider is asked for.

The test proves the downstream consequence: eligible, ranked, unranked, and returned candidates stay at 25; the unrouted tail stays `ROAD_DISTANCE_UNKNOWN`, `REJECTED`, and `NOT_APPLICABLE`; a caller `candidate_limit` of 1000 does not add candidates; repeating the search returns the same load-id prefix. Persisted rows stay at 1,000 because rejected audit rows are still stored.

```text
FINAL_EVALUATION_BOUND_SOURCE=ROUTING_SLICE_PLUS_UNKNOWN_ROAD_HARD_REJECT
FINAL_EVALUATION_CAP_RUNTIME_ENFORCED=NO
SCORE_POOL_BOUND_PROOF=STRUCTURAL
```

`domain.ScorePool` has no production counter. The test counts persisted `RANKED` and `UNRANKED` rows, which are the only rows `scoreEligible` can produce, and checks that every unrouted tail row has a hard reject. `CandidateFinalEvaluationCap` remains unused by search. No migration and no public field were added.
