# NLO-0.5B5 anonymized commercial exposure contract

Contract for a later Go fix. This document does not change Go, OpenAPI, migrations, or runtime. It does not add a response field, a replay route, or a Control Tower projection. Backhaul runtime and roundtrip runtime stay unauthorized.

```text
AGENT=D
STAGE=NLO-0.5B5-ANON-COMMERCIAL-CONTRACT-0.1
SOURCE_PR=323
BASE_MAIN_SHA=2a99b615a237b6d1f6e6038ca86a7433c567dc78
DISCOVERY_HEAD=2de55fb28d37480395fcbfbbe1af2edcfce14b9c
PRODUCT_CODE_CHANGED=NO
OPENAPI_CHANGED=NO
MIGRATIONS_CHANGED=NO
IMPLEMENTATION_AUTHORIZED=NO
LIVE_RUNTIME_EXPOSURE=NOT_VERIFIED
ANONYMIZED_COMMERCIAL_PUBLIC=LEAK_ON_CURRENT_MAIN
ROUTE_PLAN_COMMERCIAL_PUBLIC=OMITTED
CHAIN_CANDIDATE_COMMERCIAL_PUBLIC=OMITTED
CARGO_CONSTRAINTS_DECISION_REQUIRED=YES
IDENTIFIED_MARKETPLACE_COMPATIBILITY=PRESERVE
```

Evidence is the code at `2a99b615a237b6d1f6e6038ca86a7433c567dc78`. No live request with commercial data was sent.

## Serialization sites

| Site | File | Role |
| --- | --- | --- |
| `LoadOpportunity.MarketplaceView` | `services/network-optimizer-service/internal/domain/model.go` | Builds the public load object. This is the commercial copy. |
| `candidateView` | `services/network-optimizer-service/internal/service/search.go` | Puts `MarketplaceView()` on `candidates[].load` for next-load. |
| `publicComponents` | `services/network-optimizer-service/internal/service/score_rank.go` | Decides whether a `REVENUE` component keeps `raw_value` and `raw_unit`. |
| `publicSearch` | `services/network-optimizer-service/internal/service/search.go` | Marshals `SearchResponse`. It does not reshape `commercial`. |

`ListMarketplaceLoads` selects `commercial_mode`, `commercial_amount`, and `commercial_currency` onto the internal `LoadOpportunity`. That read is not the public body. The public copy happens only in `MarketplaceView`.

## Next-load commercial path

`MarketplaceView` copies `Commercial` when `mode` is non-empty or `amount` is non-nil. The condition does not read `VisibilityScope`. `ANONYMIZED_MARKETPLACE` gets the same object as `MARKETPLACE` and `INVITED_CARRIERS`.

| Stored commercial | Public `load.commercial` on an anonymized candidate |
| --- | --- |
| `amount` set | `mode`, `amount`, and `currency` are serialized. `amount` makes the copy. |
| `mode` set, `amount` nil | `mode` and `currency` are serialized. `amount` is omitted by `omitempty`. |
| `currency` only | Not copied. Both `mode` and `amount` are empty. |
| all empty | `commercial` is omitted. |

`candidateView` embeds that view for every returned eligible row. There is no second next-load encoder.

## Next-load raw REVENUE path

Internal scoring is not the HTTP body. The public step is `publicComponents`.

1. `scoreFacts` copies `Commercial.Amount` to `ScoreFacts.Revenue` and `Commercial.Currency` to `ScoreFacts.RevenueCurrency`.
2. `revenueObs` sets `raw_value` to that amount and `raw_unit` to the ranking currency when the currencies match. The explanation is `Published load commercial amount in the ranking currency`. It does not embed the number. The number is `raw_value`.
3. `publicComponents` sets `showAmount` from `MarketplaceView().Commercial.Amount != nil`. For `REVENUE`, it clears `raw_value` and `raw_unit` only when `showAmount` is false. An anonymized load with an amount therefore keeps the raw amount and the currency unit.
4. `persistSearch` stores the components from before `publicComponents`. No next-load GET returns that row.

`normalized_points`, `weighted_points`, `score`, and `rank` can still reflect revenue. `TestBNO304_ANONYMIZED_SCORE_ALLOWED` allows an anonymized score. This contract does not remove rank or score. It removes the commercial object and the raw revenue observation from the anonymized body.

## Projection comparison

| Projection | Commercial object | Raw revenue | Cargo constraints |
| --- | --- | --- | --- |
| Next-load `candidates[].load` | Copied for every scope when `mode` or `amount` is set | Kept for `REVENUE` when `amount` is set, including anonymized | Full `CargoConstraints` copied for every scope |
| Route-plan `marshalRoutePlanPublicLoadSnapshot` | Not copied. Comment and `TestNLO04B_R2_OwnTenantEvaluateUsesSafeProjection` require the terms to be absent | Not a route-plan field | Included when the cargo JSON is not `{}`, including anonymized |
| Chain `projectCandidates` | No commercial field on `PublicCandidate` | `RevenueAmount` and `RevenueCurrency` exist on the stored row and are not copied. `TestI2C_PRIV_public_projection_omits_durable_revenue` locks that | Not on `PublicCandidate` |
| Chain `VisibleLoad` | Not assigned in `MarketplacePool.Loads` | Not on that struct | Not on that struct |

`MarketplaceView` is also used for consolidation members, N-member members, and current-trip fill members. Those routes are not next-load. A fix inside `MarketplaceView` omits anonymized commercial there too. Route-plan already ignores `view.Commercial`. Chain public JSON does not call `MarketplaceView`.

## Anonymized allowlist

For `visibility_scope=ANONYMIZED_MARKETPLACE`, the later next-load candidate may keep only these load fields:

| Field | Rule |
| --- | --- |
| `id`, `version`, `status`, `visibility_scope`, `created_at`, `updated_at` | Already returned |
| `pickup`, `delivery` | `country_code`, `region`, `city` only. `CoarseDisplay` already drops `location_id`, `label`, `latitude`, and `longitude` |
| `pickup_window`, `delivery_window` | Already returned |
| `weight_kg`, `volume_m3`, `body_type`, `equipment` | Already returned when set |

Forbidden on that load object:

| Field | Required later state |
| --- | --- |
| `commercial` | Absent. Covers `mode`, `amount`, and `currency` |
| `owner_tenant_id` | Already absent |
| `source_type`, `source_id`, `invited_carrier_company_ids` | Already absent from `MarketplaceLoad` |
| `consolidation_allowed`, `cross_shipper_consolidation_allowed` | Already absent from `MarketplaceLoad` |
| exact pickup or delivery coordinates, location id, label | Already absent |

Candidate fields that stay, because they are already the anonymized contract: `deadhead_bucket` instead of `road_deadhead_km` and `road_deadhead_minutes`; no `forward_progress_km`, `lateral_distance_km`, or `route_increase_km`; `timing`, `waiting_minutes`, `compatibility`, `explanation`, `rank`, `score`, `score_evidence_bps`, `score_status`, and `unranked_reason_codes`. `REVENUE.raw_value` and `REVENUE.raw_unit` must be empty for an anonymized candidate even when the stored amount is present. The existing unavailable-revenue explanation may stay. It must not contain the amount or the currency code.

`cargo` is not on this allowlist. See the next section.

## Cargo constraints

`MarketplaceView` copies `Cargo` for every scope. The struct includes `dangerous`, `hazard_classes`, temperature bounds, body and access constraints, pallet and packaging codes, odor, contamination, and food-grade flags. Route-plan public snapshots include the same JSON whenever it is not empty. Chain public candidates do not.

```text
CARGO_CONSTRAINTS_DECISION_REQUIRED=YES
HAZARD_CLASSES_ON_ANONYMIZED_ALLOWLIST=UNDECIDED
CARGO_FIX_IN_THIS_CONTRACT=NO
```

The commercial omission does not depend on a cargo decision and must not add or remove cargo fields. A later controller decision chooses whether any cargo field, including `hazard_classes`, may remain on an anonymized next-load load. Until that decision, tests must not lock the current cargo JSON as required.

## Identified loads

`MARKETPLACE` and `INVITED_CARRIERS` keep the current `MarketplaceView` commercial copy and the current `showAmount` rule. A published amount stays on `load.commercial` and on `REVENUE.raw_value` / `raw_unit` when the ranking currency matches. This contract does not change their fields.

`NETWORK_OPTIMIZATION_ONLY` is not a next-load visibility scope in `LoadVisible`. No new exposure is defined for it.

## Required later Go change

No code change is authorized by this document. The later patch, after approval, is:

1. In `MarketplaceView`, do not set `Commercial` when `VisibilityScope` is `ANONYMIZED_MARKETPLACE`.
2. In `publicComponents`, clear `REVENUE` `raw_value` and `raw_unit` whenever the load is anonymized. Do not use `showAmount` to keep them.
3. Do not add a response field, a search replay route, a Control Tower projection, a migration, or a backhaul or roundtrip mode.
4. Do not change rank, score, or score profile weights.

OpenAPI `NetworkMarketplaceLoad` does not list `commercial` and does not set it required. `NextLoadScoreComponent.raw_value` is already optional. The omission needs no OpenAPI edit. A description edit is not part of the Go fix.

## Negative tests for that later patch

Add these beside the existing search and score tests. Use fixture amounts only. Do not call a live service.

| Test | Assert |
| --- | --- |
| Anonymized amount, mode, and currency | Marshalled next-load candidate has no `commercial`, and the fixture amount, mode, and currency strings are absent |
| Anonymized mode with nil amount | `mode` and `currency` are absent |
| Anonymized currency only | Still absent. This is the current behavior and must not regress |
| Anonymized `REVENUE` | `raw_value` is nil and `raw_unit` is empty while the stored amount is in the ranking currency. Rank and score may remain |
| `MARKETPLACE` priced load | `commercial.amount`, `commercial.mode`, and `commercial.currency` remain, and `REVENUE.raw_value` remains when the profile uses revenue |
| `INVITED_CARRIERS` for the invited company | Same preservation as `MARKETPLACE` |
| Route-plan snapshot | Existing R2 commercial absence stays |
| Chain `projectCandidates` | Existing durable revenue omission stays |

Do not add a test that requires `hazard_classes` or any other cargo field on an anonymized candidate.

## Open questions

1. Cargo and `hazard_classes` on anonymized next-load and route-plan snapshots remain `CARGO_CONSTRAINTS_DECISION_REQUIRED`.
2. Whether anonymized `score` under `MAX_REVENUE` should later stop including revenue is outside this omission. Current tests allow the score.
3. `LIVE_RUNTIME_EXPOSURE=NOT_VERIFIED`.

## Next stage

Independent approval of this omission. The following implementation is the two-site Go change and the negative tests above. It stays inside the current next-load response.
