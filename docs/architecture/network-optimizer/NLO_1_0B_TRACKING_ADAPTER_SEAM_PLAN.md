# NLO-1.0B tracking adapter seam plan

```text
AGENT=D
STAGE=NLO-1.0B-PRODUCTION-ADAPTER-PROOF
BASE=3f306fc407aeda4bdc652d825954292f4a54fad6
RELATED_PR=340
RELATED_ISSUE=337
PRODUCT_BEHAVIOR_CHANGED=NO
MIGRATIONS_CREATED=NO
SCORING_POLICY_CHANGED=NO
PUBLICATION_RULES_CHANGED=NO
FINAL_VERDICT=BLOCKED_WITH_GAPS
```

This document is the bounded plan. It does not add the seam, a harness, or a migration. The vehicle proof on Draft PR #340 stays where it is.

## PR #340

| Field | Value |
| --- | --- |
| State | OPEN, draft |
| Head | `e8dd45364802ca5849d53fadaf3dfc50cb71d67c` on `test/nlo-production-adapter-proof-v1.0a` |
| Base recorded by GitHub | `1902a2b4a77e9f2dcf823dae34f85328e8903117` |
| Merge state | `BEHIND`, `MERGEABLE` |
| Reviews | none |
| CI run | `42624482682` |
| NLO job | `backend-go-check (services/network-optimizer-service)` SUCCESS |
| Blocking job | `frontend-web-carrier-check` CANCELLED at 2026-11-09T19:25:12Z |

The run is not a green exact-head result. `origin/main` is now `3f306fc407aeda4bdc652d825954292f4a54fad6`. The commits after `1902a2b4` do not touch `services/network-optimizer-service`. They do add `services/tracking-service/executioncontract/apply.go` and the shipment stop-event contract test. `git diff --exit-code 1902a2b4 origin/main -- services/tracking-service/internal/http services/tracking-service/internal/service services/tracking-service/internal/repository` exited 0, so the lookup handlers are unchanged.

PR #340 files, and only these, are the vehicle proof:

- `services/shipment-service/cmd/nlo10a-contract-harness/main.go`
- `services/network-optimizer-service/internal/service/nlo_10a_adapter_proof_test.go`
- `services/network-optimizer-service/internal/chain/nlo_10a_bounds_test.go`
- `docs/architecture/network-optimizer/NLO_1_0A_PRODUCTION_ADAPTER_PROOF.md`

This plan does not modify them. The next implementation branch starts at current `origin/main` and brings those four files across unchanged, because they are new files and do not overlap this document. Do not merge #340, do not rebase it, and do not retarget it from this stage.

`executioncontract.ApplyPayload` is a stop-event consumer entry used by the shipment contract test. The tracking server does not import that package. It is not a lookup seam and must not be used as the NLO tracking proof.

## Vehicle chain to keep

The proven path remains:

`VehicleCapabilityHandler.Get` → real JSON → `sourceclient.HTTP.VehicleCapability` → `currenttrip.Provider.Build` → `equipmentFromVehicle` → `compat.EvaluateCargoEquipment` and `addHeight`.

The store under the shipment handlers is in-memory. That proof is not a database proof. Do not replace it with a copied DTO or a hand-written mux.

## Minimal tracking-state seam

`InternalHandler.LookupStates` is the real handler. It checks `X-Internal-Service-Token` itself, reads `X-Tenant-ID` through `tenantFromRequest`, parses `shipmentIds`, calls `TrackingQueryService.LookupTrackingStates`, and serializes with `mapSummary`.

`TrackingQueryService.repo` is `*repository.TrackingRepository`. `NewInternalHandler` accepts only `*service.TrackingQueryService`. A test cannot place a store under the real lookup without a type seam. A fake `LookupTrackingStates` on the handler would skip `RefreshComputedState`, `BatchActiveBindings`, and `mapTrackingSummary`. That is a mock adapter and is not the proof.

The smallest seam that keeps the handler path:

1. In `tracking_query_service.go`, add an interface with the two methods `LookupTrackingStates` already calls:
   - `LookupTrackingStates(ctx, tenantID, shipmentIDs) (map[uuid.UUID]domain.ShipmentTrackingState, error)`
   - `BatchActiveBindings(ctx, tenantID, shipmentIDs) (map[uuid.UUID]bool, error)`
2. Store that interface on `TrackingQueryService` and call it from `LookupTrackingStates` only.
3. `NewTrackingQueryService` keeps `*repository.TrackingRepository` and assigns the same value to the interface. `cmd/server/main.go` stays on that constructor.
4. Add one constructor that accepts the interface plus `*StateEvaluator` and `domain.FreshnessPolicy`, for the lookup path only. `GetTrackingSummary` and history stay on the concrete repository constructor.

`*TrackingRepository` already has both methods, so production behavior stays the same. No route, status code, JSON field, or freshness rule changes.

The harness is a test-only `cmd` in `tracking-service`, listening on `127.0.0.1:0`. It mounts the real `LookupStates` on `POST /internal/v1/tracking/states/lookup` and uses the real evaluator. The store is in-memory and must apply `tenant_id` the same way the SQL does: `WHERE tenant_id = $1 AND shipment_id = ANY($2)`. The NLO test calls `sourceclient.HTTP.TrackingState` and then `currenttrip.Provider.Build`. It does not decode a private copy of the payload.

## ETA data and schema

`ETAInternalHandler.LookupDelivery` is the real handler. It uses the same token and tenant checks, body fields `shipmentIds` and optional `planned`, then `ETAQueryService.LookupDeliveryETA`, then `mapETATarget`.

The service method calls `ETARepository.LookupETAStates` for `target_type = delivery`, then `ETAStateEvaluator.RefreshComputedState` and `mapETATargetSummary`. The matching seam is an interface with only `LookupETAStates`. `NewETAQueryService` keeps `*repository.ETARepository`. A second constructor accepts the interface for the lookup path. `cmd/server/main.go` does not change.

Tables, already migrated, not to be recreated:

| Table | Migration | Lookup key |
| --- | --- | --- |
| `tracking.shipment_tracking_binding` | `000026_add_shipment_telemetry_foundation_v0.7.0` | `tenant_id`, `shipment_id`, `status = 'active'` |
| `tracking.shipment_tracking_state` | same file | primary key `(tenant_id, shipment_id)` |
| `tracking.shipment_eta_state` | `000027_add_shipment_eta_intelligence_v0.7.1` | primary key `(tenant_id, shipment_id, target_type)` |

State columns the lookup reads: `tracking_status`, `provider_code`, `last_latitude`, `last_longitude`, `last_recorded_at`, `last_received_at`, `last_speed_kph`, `last_heading_degrees`, `freshness_status`, `quality_status`, `age_seconds`, `delivery_delay_seconds`.

ETA columns the delivery lookup reads: `status`, `estimated_arrival_at`, `source_type`, `provider_code`, `source_observed_at`, `received_at`, `freshness_status`, `quality_status`, `age_seconds`, `delivery_lag_seconds`, `version`, `updated_at`. `target_type` for this route is `delivery`.

`mapTrackingSummary` emits `lastKnownPosition` only when latitude, longitude, `last_recorded_at`, and `age_seconds` are all non-nil. `RefreshComputedState` always stores a pointer from `EvaluateFreshness`. A nil `last_recorded_at` becomes freshness `unknown` and age `0`, and still does not emit coordinates. A present latitude of `0` with a real longitude and timestamp is a coordinate.

`LookupDeliveryETA` returns an item for every parsed shipment id. A missing row is `status=unavailable`, `freshnessStatus=unknown`, and no `estimatedArrivalAt`. That is not an omitted JSON key and not a zero time. The NLO client turns a missing key into `Status=UNKNOWN` and a nil arrival. The real handler does not use that missing-key path for a parsed id. Invalid id strings are skipped and produce no item.

The existing `applyMigrations` in `location_dedup_integration_test.go` keeps only migration numbers `<= 14` or `26`. It loads `000026` and skips `000027`, so it cannot prove ETA. It is not migration head `000105`. This stage does not add a schema or point that harness at the lookup proof.

An in-memory store proves the handler, evaluator, JSON, and NLO decoder. It does not prove the SQL text. A later repository proof, only if separately authorized, uses the existing tables on an isolated database at the real migration head. No new migration.

## Checks the implementation must run

Positive, with non-default values:

- latitude `55.751244`, longitude `37.618423`, `last_recorded_at` two minutes before the evaluator clock, active binding present. Expect `fresh`, non-nil coordinates, and the same values after `sourceclient.HTTP.TrackingState`.
- delivery ETA `estimated_arrival_at` at a distinct RFC3339 instant and `source_observed_at` inside the 15-minute fresh window. Expect `available`, a non-nil arrival, and the same instant after `TrackingETA`.
- `Provider.Build` keeps those values on the current-trip context. A tracking or ETA error still leaves the build successful. A vehicle capability error still aborts it.

Negative, each with its existing result:

- no coordinates: `lastKnownPosition` absent, NLO latitude and longitude nil.
- latitude `0` with a non-zero longitude and a timestamp: coordinate kept.
- nil `estimated_arrival_at`: field absent, NLO arrival nil.
- other tenant: store or SQL filters `tenant_id`; caller receives `not_configured` / `unknown` and no foreign coordinates.
- missing or wrong token: HTTP 401 `UNAUTHORIZED`.
- missing `X-Tenant-ID`: HTTP 400 `VALIDATION_ERROR`.
- empty NLO base URL: `ErrUnavailable` and no outbound request.
- malformed JSON and timeout: `ErrUnavailable`.
- HTTP 200 whose item id does not match: client returns the missing-item `UNKNOWN` value and does not copy the other shipment.

Do not collapse 400 and 401 into one new code. Do not treat age `0` from a nil timestamp as a position or an arrival.

## Tenant, publication, and hidden rate

Checked on `3f306fc4`. NLO product code is unchanged since `1902a2b4`, which already contained the PR #333 merge.

- Marketplace search uses `marketplaceVisibilitySQL`: `status='PUBLISHED'`, `owner_tenant_id <> viewer`, and `MARKETPLACE`, `ANONYMIZED_MARKETPLACE`, or invited-company match.
- Chain search uses `chainDiscoverySQL`: own published rows need `consolidation_allowed`; foreign published rows need `cross_shipper_consolidation_allowed` and the same visibility predicate.
- `LoadOpportunity.MarketplaceView` copies commercial terms only when visibility is not `ANONYMIZED_MARKETPLACE`.
- `TestNLO09C_MarketplaceAndInvitedKeepPublishedCommercial` covers identified `MARKETPLACE` and `INVITED_CARRIERS`.
- `TestNLO09C_RevenueBearingProfileOmitsHiddenRate` and `TestNLO09C_HiddenRevenueDoesNotMovePublicNormalization` cover hidden-rate isolation on `MAX_REVENUE`. The identified amounts stay 100 and 400 while the hidden amount changes.
- Tracking SQL filters `tenant_id = $1` in `LookupTrackingStates`, `BatchActiveBindings`, and `LookupETAStates`.

No scoring weight, publication predicate, or activation flag changes in this stage.

## Issue #337

The issue stays open. `profilefixture.V1` gives `BALANCED` no `REVENUE` component. `TestNLO09C_BalancedProfileDoesNotPublishHiddenRate` only asserts that two hidden amounts produce the same public score. That assertion is true on a build that still lets hidden revenue into a revenue component, because this profile has no such component. It is not the revenue-isolation proof. Do not close the issue and do not edit the scoring profile to make the BALANCED test fail.

## Implementation bounds

Allowed in the next implementation PR:

- the two lookup interfaces and the two lookup constructors
- a loopback harness under `services/tracking-service/cmd`
- NLO tests that use `sourceclient.HTTP`, `Provider.Build`, and the existing vehicle harness files from PR #340
- no change to `cmd/server/main.go` beyond the constructors it already calls

Dependencies:

- PR #340 vehicle harness and its tests, copied forward without assertion edits
- real handlers, real `mapSummary` / `mapETATarget`, real evaluators, real NLO client
- migrations `000026` and `000027` as the schema description; head remains `000105`
- no new module, no `go.work` or lockfile edit, no CI permission or gate change
- new tests live in packages already run by `go test ./...`

Out of scope:

- scoring weights, `MAX_CONTRIBUTION`, contract rates, marketplace publication
- chain accept, activation, `execution_supported`, `route_plan_id`
- a new migration or a partial schema substituted for `000026` and `000027`
- live 2GIS, deployment, and merging either draft

## Verdict

Vehicle capability remains proven on PR #340. Tracking state and ETA are specified, and the seam above is small enough to implement without a behavior change. They are not proven yet. `FINAL_VERDICT=BLOCKED_WITH_GAPS`.
