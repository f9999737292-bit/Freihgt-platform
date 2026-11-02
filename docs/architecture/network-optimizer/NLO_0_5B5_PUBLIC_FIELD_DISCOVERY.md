# NLO-0.5B5 public field discovery

Discovery of the next-load response that `network-optimizer-service` already returns. NLO-0.5B1 through NLO-0.5B4 are on main. NLO-0.5B5, public exposure beyond that response, including Control Tower, is not started. This note does not add a field, a route, a migration, or a Control Tower projection.

```text
AGENT=D
STAGE=NLO-0.5B5-PUBLIC-FIELD-DISCOVERY-0.1
BASE_MAIN_SHA=2a99b615a237b6d1f6e6038ca86a7433c567dc78
PRODUCT_MIGRATION_HEAD=000105
PRODUCT_CODE_CHANGED=NO
MIGRATIONS_CHANGED=NO
OPENAPI_CHANGED=NO
GATEWAY_CHANGED=NO
CONTROL_TOWER_CHANGED=NO
NLO_0_5B1=IMPLEMENTED
NLO_0_5B2=IMPLEMENTED
NLO_0_5B3=IMPLEMENTED
NLO_0_5B4=IMPLEMENTED
NLO_0_5B5=DEFERRED
CURRENT_NEXT_LOAD_RESPONSE=IMPLEMENTED
PUBLIC_EXPOSURE_BEYOND_NEXT_LOAD_RESPONSE=DEFERRED
CONTROL_TOWER_NEXT_LOAD_CONSUMER=NOT_FOUND
BACKHAUL_RUNTIME_IMPLEMENTED=NO
ROUNDTRIP_RUNTIME_IMPLEMENTED=NO
EXTERNAL_PROVIDER_BENCHMARK=BLOCKED
PROVIDER_SLA_PROVEN=NO
LIVE_HTTP_ROUND_TRIP=NOT_VERIFIED
IMPLEMENTATION_AUTHORIZED=NO
```

`NLO_0_5B_IMPLEMENTATION_ROADMAP.md` still lists NLO-0.5B5 as not started. This discovery does not edit that roadmap.

## Sources

Read at `2a99b615a237b6d1f6e6038ca86a7433c567dc78`.

| Source | What it fixes |
| --- | --- |
| `services/network-optimizer-service/internal/service/search.go` | `SearchResponse`, `SearchCandidateView`, `publicSearch`, `candidateView`, `responseRows` |
| `services/network-optimizer-service/internal/service/score_rank.go` | `publicComponents` |
| `services/network-optimizer-service/internal/domain/model.go` | `MarketplaceLoad`, `MarketplaceView` |
| `services/network-optimizer-service/internal/domain/geo.go` | `NextLoadSearchPolicy`, `CoarseDisplay`, hard-reject reason codes |
| `services/network-optimizer-service/internal/domain/score.go` | Score component codes and `ScoreComponentResult` |
| `services/network-optimizer-service/internal/domain/search_policy.go` | `DeadheadBucket` |
| `services/network-optimizer-service/internal/repository/search_store.go` | Stored run and candidate columns that the HTTP body does not repeat |
| `services/network-optimizer-service/internal/http/router.go` | `POST /v1/network/next-load/search` |
| `services/network-optimizer-service/internal/http/handlers/search.go` | Request decode and raw response bytes |
| `services/api-gateway/internal/http/router.go` | `POST /api/v1/network/next-load/search` |
| `services/api-gateway/internal/http/proxy.go` | Prefix `/api/v1/network` and path rewrite that drops `/api` |
| `services/api-gateway/internal/bnorbac/policies.go` | Carrier roles for `PolicySearchNextLoad` |
| `packages/openapi/network-optimizer-service.yaml` | `NextLoadSearchResponse` and related schemas |
| `docs/architecture/network-optimizer/NLO_0_5B_IMPLEMENTATION_ROADMAP.md` | B5 still not started |
| `docs/architecture/network-optimizer/adr/ADR-NET-024-bounded-backhaul-search-policy.md` | B1–B3 accepted; backhaul and roundtrip runtime absent |
| `docs/architecture/network-optimizer/BNO_0_1C2_MATCH_SCORE_TOP_N.md` | Public anonymized score-component rule |
| `infrastructure/migrations/000105_platform_postgres_role_foundation_v0_2_i1.up.sql` | Product migration head. Unchanged |

No match for `next-load`, `next_load`, or `NextLoad` in `services/control-tower-read-model-service` or `apps/`. No `GET` route for a stored next-load search. `GetSearch` is repository and test access only.

## Response that exists

One public success body. The service writes it from `publicSearch` and returns those bytes with HTTP 200. The gateway does not define a second shape.

| Item | Value | Status |
| --- | --- | --- |
| Gateway route | `POST /api/v1/network/next-load/search` | IMPLEMENTED |
| Service route | `POST /v1/network/next-load/search` | IMPLEMENTED |
| Path rewrite | `strings.TrimPrefix(path, "/api")` | IMPLEMENTED |
| OpenAPI operation | `post_search_next_load_candidates_for_an_available_capacity` | IMPLEMENTED |
| Caller | Carrier actor with `CARRIER_ADMIN` or `CARRIER_DISPATCHER`, or `PLATFORM_ADMIN` | IMPLEMENTED |
| Replay route | None | NOT_FOUND |
| Control Tower read | None | NOT_FOUND |
| Portal client | None in `apps/` | NOT_FOUND |

Rejected loads are not objects in `candidates`. `rejection_counts_by_reason` counts hard-reject codes. `unranked_counts_by_reason` counts eligible rows that stayed `UNRANKED`.

`candidate_limit` changes only the candidate array. Counts stay on the full evaluation. `responseRows` keeps rows whose hard-reject list is empty. With no limit, ranked rows come first by rank and unranked rows follow by load id. With a limit of zero, the array is empty. With a positive limit, only ranked rows are returned, cut to that limit. Unranked eligible rows are left out whenever `candidate_limit` is present. `returned_candidate_count` is the length of that array.

## Top-level fields

JSON names are the `SearchResponse` tags. The service always sets the required OpenAPI names. `effective_policy_fingerprint` is always set and is not in the OpenAPI `required` list.

| JSON field | Present on 200 | Status |
| --- | --- | --- |
| `search_id` | New run id | IMPLEMENTED |
| `capacity_id` | Requested capacity | IMPLEMENTED |
| `capacity_version` | Capacity row version | IMPLEMENTED |
| `search_mode` | Effective `RADIUS`, `DIRECTIONAL_CORRIDOR`, or `ROUTE_ELLIPSE` | IMPLEMENTED |
| `effective_policy` | Merged policy object below | IMPLEMENTED |
| `effective_policy_fingerprint` | SHA-256 of the marshalled effective policy | IMPLEMENTED |
| `eligible_candidate_count` | Ranked plus unranked, before the limit | IMPLEMENTED |
| `ranked_candidate_count` | Ranked count before the limit | IMPLEMENTED |
| `unranked_eligible_count` | Unranked eligible count before the limit | IMPLEMENTED |
| `returned_candidate_count` | Length after `responseRows` | IMPLEMENTED |
| `rejection_counts_by_reason` | Map of hard-reject code to count | IMPLEMENTED |
| `unranked_counts_by_reason` | Map of unranked reason code to count | IMPLEMENTED |
| `ranking` | Active score profile identity | IMPLEMENTED |
| `candidates` | Eligible views only | IMPLEMENTED |

`ranking` fields: `objective_profile`, `profile_version`, `algorithm_version`, `profile_fingerprint`, and `ranking_currency` when the effective policy has one. There is no weight table in this object.

## Effective policy fields

`effective_policy` is `domain.NextLoadSearchPolicy` after the carrier, capacity, and request merge. Hard maxima can only tighten. OpenAPI lists the first twelve names. `allow_unknown_road_distance` has no `omitempty` tag, so the JSON includes it. OpenAPI does not name it. The schema also does not set `additionalProperties: false`.

| JSON field | Status in service JSON | Status in OpenAPI property list |
| --- | --- | --- |
| `search_mode` | IMPLEMENTED | IMPLEMENTED |
| `target_location_id` | IMPLEMENTED, omitted when nil | IMPLEMENTED |
| `forward_search_km` | IMPLEMENTED, omitted when nil | IMPLEMENTED |
| `corridor_deviation_km` | IMPLEMENTED, omitted when nil | IMPLEMENTED |
| `max_deadhead_km` | IMPLEMENTED, omitted when nil | IMPLEMENTED |
| `preferred_deadhead_km` | IMPLEMENTED, omitted when nil | IMPLEMENTED |
| `max_deadhead_minutes` | IMPLEMENTED, omitted when nil | IMPLEMENTED |
| `min_loaded_distance_km` | IMPLEMENTED, omitted when nil | IMPLEMENTED |
| `max_route_increase_km` | IMPLEMENTED, omitted when nil | IMPLEMENTED |
| `radius_km` | IMPLEMENTED, omitted when nil | IMPLEMENTED |
| `objective_profile` | IMPLEMENTED | IMPLEMENTED |
| `ranking_currency` | IMPLEMENTED, omitted when empty | IMPLEMENTED |
| `allow_unknown_road_distance` | IMPLEMENTED, always present | NOT_FOUND |

`MAX_CONTRIBUTION` remains a non-executable profile. A request that selects it fails before a 200 body. No public field was added for backhaul chain or roundtrip mode. `search_mode` is still the three existing values. B3 feasibility does not add a candidate field. Corridor and ellipse outcomes use the existing geometry fields and the existing hard-reject codes.

## Candidate fields

`candidateView` returns only rows that `responseRows` kept. `eligibility` is the literal `ELIGIBLE`. `explanation` on that view is the one-element list `ELIGIBLE`. Rank, score, and `score_evidence_bps` are set only for `RANKED`. `unranked_reason_codes` are set only otherwise.

| JSON field | When present | Status |
| --- | --- | --- |
| `load_opportunity_id` | Always | IMPLEMENTED |
| `load_version` | Always | IMPLEMENTED |
| `eligibility` | Always `ELIGIBLE` | IMPLEMENTED |
| `load` | Marketplace view below | IMPLEMENTED |
| `deadhead_bucket` | Anonymized load with a road deadhead. Enum `0-25`, `25-50`, `50-100`, `100-200`, `200+` | IMPLEMENTED |
| `road_deadhead_km` | Non-anonymized load with a road deadhead | IMPLEMENTED |
| `road_deadhead_minutes` | Same row as `road_deadhead_km` | IMPLEMENTED |
| `forward_progress_km` | Non-anonymized corridor geometry. Omitted for anonymized loads. Not road distance | IMPLEMENTED |
| `lateral_distance_km` | Same omission rule as forward progress | IMPLEMENTED |
| `route_increase_km` | Set from one-load feasibility when applicable. Code also clears it for an anonymized load. OpenAPI describes the anonymized omission for forward and lateral only | IMPLEMENTED in Go. OpenAPI omission text is incomplete |
| `timing` | `FEASIBLE` or `UNKNOWN` on a returned row. `MISSED` is assigned together with `PICKUP_WINDOW_MISSED`, and that row is not returned | IMPLEMENTED |
| `waiting_minutes` | Pickup wait when the window check succeeds | IMPLEMENTED |
| `compatibility` | `COMPATIBLE`, `INCOMPATIBLE`, or `INDETERMINATE` | IMPLEMENTED |
| `explanation` | `["ELIGIBLE"]` on a returned row | IMPLEMENTED |
| `rank` | `RANKED` only | IMPLEMENTED |
| `score_status` | `RANKED` or `UNRANKED` | IMPLEMENTED |
| `score` | Integer 0–10000, `RANKED` only | IMPLEMENTED |
| `score_evidence_bps` | `RANKED` only | IMPLEMENTED |
| `score_components` | Public component list | IMPLEMENTED |
| `unranked_reason_codes` | `UNRANKED` only | IMPLEMENTED |

Stored and not copied onto the candidate: candidate row id, tenant id, search run id, capacity id, hard-reject codes for that load, compatibility fingerprint, policy fingerprint, score fingerprint, created time, slack, delivery-to-target kilometres as their own field, capacity usage, and loaded road kilometres. Exact road deadhead for an anonymized load stays on the stored candidate. The public field is the bucket. `BNO209_BNO210_ANONYMIZED_GEO` locks that split.

Hard-reject codes that can appear only in `rejection_counts_by_reason`: `BACKTRACK_DIRECTION_REJECTED`, `FORWARD_SEARCH_EXCEEDED`, `CORRIDOR_DEVIATION_EXCEEDED`, `MAX_DEADHEAD_EXCEEDED`, `ROAD_DISTANCE_UNKNOWN`, `PICKUP_WINDOW_MISSED`, `DIRECTION_TARGET_REQUIRED`, `DIRECTION_TARGET_GEO_UNKNOWN`, `ROUTE_INCREASE_EXCEEDED`, `DELIVERY_NOT_TOWARD_TARGET`, `MAX_DEADHEAD_TIME_EXCEEDED`, `PAYLOAD_EXCEEDED`, `VOLUME_EXCEEDED`, `OUTSIDE_RADIUS`, `CARGO_EQUIPMENT_INCOMPATIBLE`, `CARGO_EQUIPMENT_INDETERMINATE`, `CAPACITY_FACT_UNKNOWN`.

## Load object inside a candidate

`load` is `LoadOpportunity.MarketplaceView()`. OpenAPI `NetworkMarketplaceLoad` documents `id`, `visibility_scope`, and coarse `pickup` and `delivery`, with `additionalProperties: true`. The Go object is larger. That extra JSON is IMPLEMENTED. The OpenAPI property list does not name it.

Always or conditionally present from the Go struct:

| JSON field | Anonymized load | Other visible load |
| --- | --- | --- |
| `id` | Present | Present |
| `owner_tenant_id` | Omitted | Present |
| `pickup`, `delivery` | `country_code`, `region`, `city` only | `location_id`, `label`, `latitude`, `longitude`, `country_code`, `region`, `city` when set |
| `pickup_window`, `delivery_window` | `start` and `end` when set | Same |
| `weight_kg`, `volume_m3` | Present when the load has them | Same |
| `body_type`, `equipment` | Present when set | Same |
| `cargo` | Constraint fields when set | Same |
| `commercial` | Copied when `mode` or `amount` is set. Visibility scope is not a condition in `MarketplaceView` | Same |
| `visibility_scope` | Present | Present |
| `status`, `version`, `created_at`, `updated_at` | Present | Present |

Not on `MarketplaceLoad`: `source_type`, `source_id`, `invited_carrier_company_ids`, `consolidation_allowed`, `cross_shipper_consolidation_allowed`. Address lines are not on `Place`.

`BNO_0_1C2_MATCH_SCORE_TOP_N.md` says a commercial raw amount is returned only when the marketplace view already exposes that amount. `publicComponents` follows that rule for the `REVENUE` component. It does not remove `load.commercial`.

## Score components

Each public component has `code`, `status`, `weight_bps`, `reason_code`, and `explanation`. Optional: `raw_value`, `raw_unit`, `normalized_points`, `weighted_points`.

Component codes in `domain`: `DEADHEAD_EFFICIENCY`, `CAPACITY_UTILIZATION`, `WAITING_EFFICIENCY`, `TARGET_PROXIMITY`, `PICKUP_SLACK`, `REVENUE`, `PREDICTION_CONFIDENCE`, `ETA_UNCERTAINTY`, `NETWORK_VALUE`. Status values: `AVAILABLE`, `UNAVAILABLE`, `FALLBACK`.

For an anonymized load, `publicComponents` clears `raw_value` and `raw_unit` on `DEADHEAD_EFFICIENCY` and `TARGET_PROXIMITY`, and the deadhead explanation names the bucket. `REVENUE` raw value is cleared when the marketplace view has no amount. Which codes a given response contains depends on the active system profile. This discovery does not list production weights.

## Stored and not public

`SearchRun` and `StoredCandidate` hold audit that the 200 body does not repeat: tenant id, start and completion times, routing provider name, run status, candidate row id, per-load reject codes, compatibility fingerprint, and score fingerprint. Provider call counts and the search budget are not response fields. There is no public field for chain membership, roundtrip, execution support, or a Control Tower projection id. Chain search and consolidation search are different routes and are outside this response.

## Dependencies

| Dependency | State |
| --- | --- |
| NLO-0.5B1 discovery cap, NLO-0.5B2 routing budget, NLO-0.5B3 one-load feasibility, NLO-0.5B4 final-evaluation invariant | IMPLEMENTED on this main. They do not add a B5 field |
| ADR-NET-024 | Accepted for the bounded search. `BACKHAUL_RUNTIME_IMPLEMENTED=NO`. `ROUNDTRIP_RUNTIME_IMPLEMENTED=NO` |
| Gateway carrier policy | IMPLEMENTED. A future reader still needs its own authorization |
| Control Tower | No next-load consumer on this main. A later display waits on an accepted field list. This discovery does not authorize that display |
| Product migration `000105` | Head remains `000105_platform_postgres_role_foundation_v0_2_i1`. No NLO migration is assigned |
| Provider benchmark and SLA | `EXTERNAL_PROVIDER_BENCHMARK=BLOCKED`. `PROVIDER_SLA_PROVEN=NO` |

## Open questions

1. Whether any field beyond this inventory is allowed. This note does not choose one. Control Tower exposure stays `DEFERRED`.
2. Whether OpenAPI should name the Go `MarketplaceLoad` fields and `allow_unknown_road_distance`, and should state that anonymized `route_increase_km` is omitted. No schema edit is made here.
3. Whether `load.commercial` on an anonymized candidate is accepted. The view copies it when `mode` or `amount` is set. A live privacy decision is not in this note.
4. Whether a later top-N contract should keep unranked rows when `candidate_limit` is set. Today a present limit returns ranked rows only.
5. Whether a stored search needs a public replay route. None exists. `GetSearch` is not an HTTP handler.
6. Live gateway and service responses were not called. `LIVE_HTTP_ROUND_TRIP=NOT_VERIFIED`.

## Next stage

Independent review of this inventory. The following stage is a field-list decision for NLO-0.5B5, still without a new response field, a Control Tower projection, a backhaul runtime, or a roundtrip runtime, until that decision explicitly authorizes one of those. Do not continue `discovery/nlo-backhaul-bounded-benchmark-v0.5b0`.
