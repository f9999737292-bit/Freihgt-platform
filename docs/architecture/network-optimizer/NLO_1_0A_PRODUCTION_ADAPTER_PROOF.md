# NLO-1.0A production adapter proof

```text
AGENT=D
STAGE=NLO-1.0A-PRODUCTION-ADAPTER-PROOF
BASE_SHA=1902a2b4a77e9f2dcf823dae34f85328e8903117
PR333_MERGE_SHA=3d2975a26b19d8496dbf66fe95b104b027ccff36
NLO_TREE_UNCHANGED_SINCE_PR333=YES
NLO_DIFF_EXIT=0
PR333_ANCESTOR_EXIT=0
MIGRATION_HEAD=000105_platform_postgres_role_foundation_v0_2_i1
PRODUCT_BEHAVIOR_CHANGED=NO
MIGRATIONS_CREATED=NO
```

Evidence is the tree at `1902a2b4a77e9f2dcf823dae34f85328e8903117` plus the tests added on `test/nlo-production-adapter-proof-v1.0a`. The primary checkout `D:/Projects/freight-platform` was dirty on another branch and was not used. This worktree started clean at the expected main SHA.

`git diff --exit-code 3d2975a26b19d8496dbf66fe95b104b027ccff36 1902a2b4a77e9f2dcf823dae34f85328e8903117 -- services/network-optimizer-service` exited 0. PR #333 is `MERGED` at `3d2975a26b19d8496dbf66fe95b104b027ccff36`, and that commit is an ancestor of the expected main (`git merge-base --is-ancestor` exited 0). No open PR for this stage existed at precheck. Issue #337 stays open.

## Contract map

| Route | Producer | Registration | Auth and tenant | NLO client | Use |
| --- | --- | --- | --- | --- | --- |
| `GET /internal/v1/vehicles/{id}/capability` | shipment-service `VehicleCapabilityHandler.Get` | `internal/http/router.go` | `internalauth.Middleware` requires `X-Internal-Service-Token`. `resolveVerifiedTenant` requires `X-Tenant-ID`. Missing tenant or token is HTTP 401. Unknown vehicle for that tenant is HTTP 404 via `respond.Error`. | `sourceclient.HTTP.VehicleCapability` | `currenttrip.Provider.Build` then `equipmentFromVehicle` then `compat.EvaluateCargoEquipment` and `addHeight` |
| `POST /internal/v1/tracking/states/lookup` | tracking-service `InternalHandler.LookupStates` | `internal/http/router.go` under `/internal/v1/tracking` | Same token header, compared inside the handler. Tenant from `X-Tenant-ID`. Body `shipmentIds`. Response `{"items":{id: summary}}`. | `sourceclient.HTTP.TrackingState` | Optional in `Provider.Build`. An error leaves freshness `UNKNOWN` and does not fail the build. |
| `POST /internal/v1/tracking/eta/lookup` | tracking-service `ETAInternalHandler.LookupDelivery` | same router | Same token and tenant. Body `shipmentIds` and optional `planned`. Response `{"items":{id: target}}`. | `sourceclient.HTTP.TrackingETA` | Optional in `Provider.Build`. An error leaves ETA freshness `UNKNOWN`. |

Vehicle response fields written by the handler are `id`, `tenant_id`, `version`, `capacity_weight`, `capacity_volume`, `pallet_positions`, `internal_length_mm`, `internal_width_mm`, `internal_height_mm`, `temperature_control_mode`, `temperature_capability_min_c`, `temperature_capability_max_c`, `food_grade_capability`, `adr_capability`, plus the other vehicle attributes the handler copies. Null pointers are omitted by `encoding/json`. A present `false` and a present `0` are distinct from an omitted field.

The NLO client returns `currenttrip.ErrNotFound` for HTTP 404 and 403, and `currenttrip.ErrUnavailable` for every other failure, including HTTP 401, a timeout, malformed JSON, and a 200 body whose `id` or `tenant_id` does not match the request. Empty base URL fails in `http.NewRequest` before `Client.Do`.

Other adapters, mapped and not extended:

| Adapter | Client | Fail-closed rule |
| --- | --- | --- |
| Shipment prediction | `sourceclient.HTTP.Shipment` → `/internal/v1/shipments/{id}/prediction-input` | Empty URL or identity mismatch is unavailable. |
| Execution context and onboard cargo | `ExecutionContext`, `OnboardCargo` | Required for `Provider.Build`. Not found or unavailable stops the build. |
| Cargo planning profile | `CargoPlanningProfile` on the transport-order URL | Empty URL is unavailable. Not called when onboard items are empty. |
| Execution projection | `executionproj` HTTP client set in `cmd/server/main.go` | Not part of this proof. |
| 2GIS | `twogis.New` only when `BNO_ROUTING_PROVIDER=2GIS` | Unset provider leaves `routing.Provider` nil. Road kilometres stay absent. |

## What the tests actually run

Real layers for the vehicle proof:

- `handlers.VehicleCapabilityHandler.Get`
- `handlers.ExecutionEvidenceHandler.GetExecutionContext`
- `handlers.ExecutionEvidenceHandler.GetOnboardCargo`
- `internalauth.Config.Middleware`
- `sourceclient.HTTP`
- `currenttrip.Provider.Build`
- `equipmentFromVehicle`
- `compat.EvaluateCargoEquipment`
- `addHeight`

The store under those handlers is an in-memory fixture in `services/shipment-service/cmd/nlo10a-contract-harness`. It is not a database and not a copy of the HTTP encoder. The harness prints the stored values. The test reads the capability only through the NLO client.

Manual HTTP responses are used only for mismatched id, malformed JSON, timeout, and a missing tracking item. Those are not the vehicle proof.

## Tracking blocker

`InternalHandler` stores `*service.TrackingQueryService`. That service stores `*repository.TrackingRepository`. There is no interface under the handler, so a synthetic source cannot be placed beneath the real lookup handler without a database or a product change. The existing tracking integration harness applies only a filtered migration set and is not the migration head `000105`. This stage does not add a schema and does not change the handler.

Minimal later change, not done here: give `LookupStates` and `LookupDelivery` a small query interface so the real handler and its `mapSummary` / `mapETATarget` serialization can run against an in-memory query. Until that exists, or until a test uses the real repository on the real migration head, the positive tracking and ETA contracts are unproven.

The missing-item client check in `TestNLO10A_ProtocolFailuresStayDistinct` uses a hand-written `{"items":{}}`. It shows the current client does not turn a missing element into coordinates or an arrival time. It is not a real-handler proof.

## Local commands

```text
go test -count=1 -timeout 180s -run TestNLO10A_ ./internal/service ./internal/chain
exit 0
service 5.664s PASS
chain 0.055s PASS

go test -count=1 -timeout 180s -v -run TestNLO09C_ ./internal/domain ./internal/service ./internal/chain ./internal/chainapi
exit 0
domain PASS: AnonymizedMarketplaceViewOmitsCommercial, IdentifiedMarketplaceViewKeepsPublishedCommercial, CurrencyOnlyCommercialStaysUncopied
service PASS: AnonymizedNextLoadOmitsCommercialAndKeepsLoad, AnonymizedModeWithoutAmountIsAbsent, AnonymizedCurrencyOnlyStaysAbsent, MarketplaceAndInvitedKeepPublishedCommercial, HiddenRevenueDoesNotMovePublicNormalization, AllAnonymizedRevenueStaysUnpublished, BalancedProfileDoesNotPublishHiddenRate, RevenueBearingProfileOmitsHiddenRate
chain: no TestNLO09C_ functions
chainapi PASS: ChainDiscoveryOwnAndForeignOmitsCommercial (own+own, own+foreign, anonymized foreign stays searchable, ineligible foreign stays out), ChainDiscoveryBoundKeepsNewestEligible
```

`TestNLO09C_RevenueBearingProfileOmitsHiddenRate` is the revenue-bearing proof. `TestNLO09C_HiddenRevenueDoesNotMovePublicNormalization` keeps the identified 100 and 400 scale fixed while the hidden amount changes. `TestNLO09C_BalancedProfileDoesNotPublishHiddenRate` is not that proof. Issue #337 stays open.

`TestNLO10A_ChainBoundsStayAtDiscovery` locks 2 loads, 48h, 8 first candidates, 2 second candidates, 4 pairs, 2 route evaluations, 24 provider calls, and 20s.
