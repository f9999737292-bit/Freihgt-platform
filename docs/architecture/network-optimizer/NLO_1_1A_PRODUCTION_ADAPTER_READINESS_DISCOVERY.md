# NLO-1.1A tracking and ETA production-adapter readiness

```text
AGENT=D
STAGE=NLO-TRACKING-ETA-PRODUCTION-ADAPTER-READINESS-1.1A
BASE_SHA=a9ef8f40bf0e7f75cb4aee3794e7841723347e4c
PREVIOUS_PR=367
PREVIOUS_STAGE=NLO-TRACKING-ETA-CI-PROOF-1.0E
PREVIOUS_AUDIT=REMEDIATION_REQUIRED
J_NLO10E_001=CLOSED
J_NLO11A_001_TO_006=DOC_REMEDIATED_PENDING_J
REMEDIATION=1.1A-R2
PRODUCT_CODE_CHANGED=NO
MIGRATIONS_CHANGED=NO
STAGING_CHANGED=NO
PRODUCTION_CHANGED=NO
REAL_EXTERNAL_REQUESTS=NO
IMPLEMENTATION_AUTHORIZED=NO
PRODUCTION_ADAPTER_READY=NO
PRODUCTION_CONSUMER_READINESS=BLOCKED
EXECUTION_SUPPORTED=NO
FINAL_VERDICT=READY_FOR_J_R2_ARCHITECTURE_REVIEW
CONTROLLER_APPROVAL_REQUIRED=YES
DECISIONS_IN_FORCE=NO
```

This note records what the code on `a9ef8f40` does. It does not add an adapter, a chain runtime, or a production consumer. A document is not a running mechanism. Closed gates stay closed.

## 1. Baseline

`origin/main` at discovery is `a9ef8f40bf0e7f75cb4aee3794e7841723347e4c`, the merge commit of PR #367. Its parents are `3f306fc407aeda4bdc652d825954292f4a54fad6` and `dd877e2e73e572216eb30492a08112fd5e57b91d`. Exact-head proof CI `44821722521` is `success` on `dd877e2e`. Post-merge CI `44828104256` is `success` on `a9ef8f40`, 75 jobs.

PR #367 added three paths and nothing else:

- `.github/workflows/ci.yml`
- `services/network-optimizer-service/internal/sourceclient/nlo_10d_tracking_eta_proof_test.go`
- `services/tracking-service/cmd/nlo10d-contract-harness/main.go`

The harness file states that it is a test-only loopback process and is not registered in the tracking runtime (`cmd/nlo10d-contract-harness/main.go` lines 1–3).

## 2. What 1.0E proved

`TestNLO10D_TrackingAndETARealHandlers` calls `sourceclient.HTTP.TrackingState` and `TrackingETA` against the real tracking constructors, repositories, evaluators, and handlers on an ephemeral PostgreSQL 16 database loaded only with migrations `000026` and `000027`. The exact-head log contains the parent line `--- PASS: TestNLO10D_TrackingAndETARealHandlers` and these subtests: `fresh_coordinates`, `zero_latitude`, `null_coordinates`, `missing_shipment`, `eta_arrival`, `eta_null_arrival`, `eta_missing_unavailable`, `cross_tenant_tracking`, `cross_tenant_eta`, `wrong_token`, `missing_tenant`.

That proof is a handler contract. It does not deploy tracking-service, does not consume Kafka, and does not authorize route activation or chain execution.

## 3. Inventory

### 3.1 Tracking source used by NLO

| Item | Evidence |
| --- | --- |
| Caller | `sourceclient.HTTP.TrackingState` (`current_trip.go` lines 180–219) |
| Route | `POST {trackingURL}/internal/v1/tracking/states/lookup` |
| Request DTO | JSON `{"shipmentIds":[one id]}`. The handler type is `lookupRequest` (`tracking_handler.go` lines 210–212). NLO sends one id. |
| Response DTO read by NLO | `items[shipmentId].trackingStatus`, `freshness.status`, `freshness.ageSeconds`, `lastKnownPosition.latitude`, `longitude`, `recordedAt` |
| Producer | `InternalHandler.LookupStates` (`tracking_handler.go` lines 214–246) → `TrackingQueryService.LookupTrackingStates` (`tracking_query_service.go` lines 84–114) → `TrackingRepository.LookupTrackingStates` and `BatchActiveBindings` |
| Data source | Tables `tracking.shipment_tracking_state` and `tracking.shipment_tracking_binding`. SQL predicates are `tenant_id = $1 AND shipment_id = ANY($2)` and, for bindings, `status = 'active'` (`tracking_repository.go` lines 347–352 and 326). |
| Confirmed cargo state | Not this route. Onboard evidence comes from shipment-service `GET /internal/v1/shipments/{id}/onboard-cargo` via `HTTP.OnboardCargo` (`current_trip.go` lines 50–79). `Provider.Build` counts `EvidenceConfirmedOnboard` (`provider.go` lines 86–95). |

Missing lookup key becomes `Freshness: "UNKNOWN"`, `TrackingStatus: "UNKNOWN"`, with a nil error (`current_trip.go` lines 206–208). The real handler still emits an item for a parsed id with `not_configured` or `awaiting_data` (`tracking_query_service.go` lines 98–108). The `UNKNOWN` branch is the client behavior when the key is absent.

`Provider.Build` treats a tracking error as optional. On `trackErr != nil` it keeps `positionFreshness = "UNKNOWN"` and continues (`provider.go` lines 113–122). A vehicle capability error still aborts the build (`provider.go` lines 104–108).

### 3.2 ETA source used by NLO

| Item | Evidence |
| --- | --- |
| Current-trip caller | `sourceclient.HTTP.TrackingETA` (`current_trip.go` lines 222–253) |
| Prediction caller | `sourceclient.HTTP.ETA` (`http.go` lines 119–144). A missing item, or a nil arrival or observation, returns `predict.ETAFact{}` and a nil error. |
| Route | `POST {trackingURL}/internal/v1/tracking/eta/lookup` |
| Request DTO | JSON `{"shipmentIds":[one id]}`. NLO does not send `planned`. |
| Handler | `ETAInternalHandler.LookupDelivery` (`eta_handler.go` lines 108–144). It accepts optional `planned` and passes it to `ETAQueryService.LookupDeliveryETA`. |
| SQL | `tracking.shipment_eta_state` where `tenant_id = $1 AND shipment_id = ANY($2) AND target_type = $3` (`eta_repository.go` lines 153–158). The service asks for `domain.TargetDelivery` (`eta_query_service.go` line 101). |
| Missing row | Status `unavailable`, freshness `unknown`, arrival projection `unknown` (`eta_query_service.go` lines 111–122). If the caller had sent a delivered status or `actualDeliveryAt`, the status becomes `completed`. NLO does not send those fields, so that branch is not exercised by the NLO client. |
| Client miss | Absent key becomes `Status: "UNKNOWN"`, `FreshnessStatus: "UNKNOWN"`, nil error (`current_trip.go` lines 246–247). |

`Provider.Build` also treats an ETA error as optional (`provider.go` lines 124–135).

A different ETA route exists: `ETAQueryService.GetExecutionStopETA` (`eta_query_service.go` lines 179–190). `sourceclient` does not call it. Multi-stop execution ETA is not the NLO lookup.

### 3.3 Tenant authorization and shipment ownership

These checks are different. None of them is a tracking-row owner proof.

| Check | What the code does | What it is not |
| --- | --- | --- |
| Tenant membership precheck | `ExecutionEvidenceService.load` calls `GetByIDAndTenant` after `ValidateVerifiedTenant` (`execution_evidence_service.go` lines 45–52). `Provider.Build` calls `ExecutionContext` before tracking (`provider.go` lines 75–79). The prediction path loads the shipment and rejects a mismatched `tenant_id` (`prediction.go` lines 282–288). | A proof that a tracking row was written by the current owner. |
| Tenant-key SQL isolation | Lookup SQL uses `tenant_id = $1`. Binding lookup also requires `status = 'active'`. ETA lookup also requires `target_type`. The 1.0E subtests `cross_tenant_tracking` and `cross_tenant_eta` passed. | Shipment ownership, and not an owner fence. |
| Shipment ownership proof | The shipment read above is tenant-scoped. `HTTP.ExecutionContext` also rejects a body whose `tenant_id` or shipment id does not match (`current_trip.go` lines 37–38). | A check inside `LookupStates` or `LookupDelivery`. Those handlers do not call shipment-service. |
| Historical row owner proof | `LookupTrackingStates` returns the stored row for that tenant and shipment (`tracking_repository.go` lines 347–352). | An owner-proof token. No such column is consulted. |
| Tracking owner fence | No owner-fence symbol exists under `services/tracking-service` on this SHA. | Closed by the tenant SQL predicate. |
| TMS execution authority | Activation acknowledgement comes from shipment-service. Chain publication forces `execution_supported=false`. | Permission for NLO to treat tracking `ended` or ETA `completed` as execution facts. |

Both internal handlers compare `X-Internal-Service-Token` with `!=` (`tracking_handler.go` lines 215–217, `eta_handler.go` lines 109–111). They do not use `internalauth.Config.Authorize`, which rejects an empty token and compares in constant time (`packages/shared-go/internalauth/auth.go` lines 16–26). The tracking token falls through to `dev_internal_tracking_token` with no environment guard (`config.go` line 89). Any holder of the accepted token can supply another tenant in `X-Tenant-ID`. That is `CLOSED_FOR_PRODUCTION` for internal-auth hardening. It does not reopen the 1.0E tenant-key result.

The tenant on the NLO call is the caller-supplied header set in `HTTP.send` (`http.go` lines 151–164). An empty NLO service token returns `predict.ErrUnavailable` before the request. An empty tracking tenant header is HTTP 400 (`tracking_handler.go` lines 249–254). Invalid shipment id strings are skipped and do not become a 400 (`tracking_handler.go` lines 230–235). Tracking and ETA items do not echo `tenant_id`, so the client cannot repeat the execution-context comparison on those payloads.

### 3.4 Freshness vocabulary

Tracking and ETA do not share one freshness enum. NLO does not normalize either contract.

Tracking producer values are lowercase `unknown`, `fresh`, `stale`, and `lost` (`telemetry.go` lines 17–20). `EvaluateFreshness` returns `unknown` when `lastRecordedAt` is nil, `fresh` at or below 10 minutes, `stale` at or below 30 minutes, and `lost` after that (`freshness.go` lines 24–39, `config.go` lines 66–67). A negative age is clamped to 0 and therefore classifies as `fresh` (`freshness.go` lines 29–31). Ingest validation of a future timestamp was not re-checked in this remediation.

ETA freshness values are lowercase `unknown`, `fresh`, `stale`, and `expired` (`eta.go` lines 26–29). `EvaluateETAFreshness` uses 15 and 60 minutes and returns `expired` past the stale threshold (`eta_freshness.go` lines 22–37, `config.go` lines 72–73). ETA status is a second field: `available`, `stale`, `expired`, `unavailable`, or `completed` (`eta_freshness.go` lines 41–57). There is no tracking freshness value `expired`, and no ETA freshness value `lost`.

`sourceclient` copies the producer string. It does not map case. If the tracking freshness string is empty, the client substitutes `"UNKNOWN"` (`current_trip.go` lines 216–218). A missing tracking item is also `"UNKNOWN"` (`current_trip.go` lines 206–208). Those two client strings are not the producer value `unknown`.

`Provider.Build` starts from `"UNKNOWN"` and, on success, stores the copied producer string (`provider.go` lines 113–122 and 124–135). `service.fresh` returns true only for `"FRESH"` (`current_trip_fill.go` line 541). `route_plan.go` line 273 requires `PositionFreshnessStatus != "FRESH"` to fail. Producer `fresh` therefore fails both checks. NLO tests that supply `"FRESH"` (`sourceclient/nlo_03c_test.go` lines 48 and 107) do not exercise the producer spelling. The 1.0E proof asserts the producer spelling `fresh` on the client result. It does not assert the NLO `"FRESH"` gate.

`mapTrackingSummary` emits `lastKnownPosition` only when latitude, longitude, `LastRecordedAt`, and `AgeSeconds` are all non-nil (`tracking_query_service.go` lines 135–141). Latitude `0` remains a coordinate. The 1.0E subtest `zero_latitude` passed. A null coordinate omits the position. NLO copies the pointers (`current_trip.go` lines 211–214). An ETA arrival is set only when `estimatedArrivalAt` is present (`eta_handler.go` lines 220–221). `HTTP.ETA` also requires `sourceObservedAt` before `Present: true` (`http.go` lines 141–144).

| Producer value | CurrentTrip value | Normalized proposal | Risk |
| --- | --- | --- | --- |
| tracking `fresh` | copied `fresh` | keep the producer spelling; do not alias it to `"FRESH"` | `route_plan.go:273` and `fresh()` reject it |
| tracking `stale` | copied `stale` | distinct from `lost` and from `"UNKNOWN"` | stored; not treated as fresh by the uppercase check |
| tracking `lost` | copied `lost` | tracking-only; ETA has no `lost` | stored; chain seed can still use the coordinates |
| tracking `unknown` | copied `unknown` | not the same string as client `"UNKNOWN"` | stored as a present position with producer unknown |
| tracking empty freshness | client `"UNKNOWN"` | synthetic client value | only the empty-string branch in `TrackingState` |
| tracking fetch error | position nil, context freshness stays initial `"UNKNOWN"` | not a producer value | indistinguishable today from a successful unknown context |
| tracking value outside the four constants | copied unchanged | no allow-list exists | fails `"FRESH"` and is still stored |
| ETA freshness `fresh` / `stale` / `expired` / `unknown` | copied onto `ETA.FreshnessStatus` | do not reuse the tracking set; `expired` is ETA-only | `fresh()` is false for producer `fresh` |
| ETA status `unavailable` | copied onto `ETA.Status` when the item exists | not the same as a missing key | item remains non-nil |
| ETA freshness empty on a present item | `ETA.FreshnessStatus` stays empty; context freshness stays initial `"UNKNOWN"` (`provider.go` lines 131–133) | do not apply the tracking empty-string rewrite | context `"UNKNOWN"` hides the empty ETA field |
| ETA missing key | client `Status` and `FreshnessStatus` `"UNKNOWN"` | client sentinel, not producer `unknown` | nil arrival on a non-nil ETA object |
| ETA fetch error | ETA pointer nil, context freshness stays initial `"UNKNOWN"` | not an `unavailable` item | `forecast` fails this case; `Build` does not |

### 3.5 Error taxonomy

`HTTP.send` uses a 5 second timeout and refuses redirects (`http.go` lines 28–35). There is no retry in `sourceclient`. HTTP 200 returns the body. HTTP 403 and 404 become `predict.ErrNotFound`. Every other status, including 401 and 500, and every transport error, becomes `predict.ErrUnavailable` (`http.go` lines 168–184). `mapTripErr` maps `ErrNotFound` to `currenttrip.ErrNotFound` and every other error to `currenttrip.ErrUnavailable` (`current_trip.go` lines 256–263). The status code and response body are discarded.

`Provider.Build` ignores a tracking or ETA error and continues (`provider.go` lines 116–135). The context field stays the initial `"UNKNOWN"`, and the position or ETA pointer stays nil. That is not the same object as a successful item whose producer freshness is `unknown`.

`forecast` does not follow that rule. An ETA error other than `predict.ErrNotFound` fails the forecast. `ErrNotFound` becomes an empty `ETAFact` and continues (`prediction.go` lines 303–309).

The lookups are reads and have no idempotency key. Route-plan activation requires `Idempotency-Key` (`route_plan_lifecycle.go` lines 95–98).

| Case | Current result | Information lost | Safe to keep planning | Forbidden use | 1.1B proposal, not in force |
| --- | --- | --- | --- | --- | --- |
| HTTP 401 | `ErrUnavailable`, then `Build` succeeds with a nil position or ETA and context `"UNKNOWN"` | status, body, and the fact that the token was rejected | shipment and cargo facts already loaded may remain | do not treat the context string as producer `unknown`, and do not anchor a route on it | a source-unavailable state, distinct from producer `unknown` |
| HTTP 403 | `ErrNotFound`, then the same swallowed `Build` result | 403 is indistinguishable from 404 after `mapTripErr` | same as 401 for `Build` | do not report "shipment not found" from a tracking 403; `Build` itself does not | same source-unavailable state |
| HTTP 404 | same mapping as 403 | the lookup handlers do not use 404 for a missing shipment row | same | do not invent a shipment miss from a lookup 404 | same source-unavailable state |
| HTTP 500 | `ErrUnavailable`, swallowed | status and body | same | do not anchor | same source-unavailable state |
| Timeout | `client.Do` error becomes `ErrUnavailable`, swallowed | the 5 second deadline is not visible on the context | same | do not anchor | same source-unavailable state |
| Transport failure | same as timeout | the transport error text | same | do not anchor | same source-unavailable state |
| HTTP 200, item key absent | nil error; tracking position is non-nil with nil coordinates and client `"UNKNOWN"` (`current_trip.go` lines 206–208) | nothing from a body that had no key | yes, as uncertainty | chain seed already rejects nil coordinates (`resolve.go` lines 162–164) | retain as uncertainty, not as source-unavailable |
| Coordinates absent on a present item | position object kept, latitude and longitude nil | no coordinate is invented | yes | do not substitute `0` | retain; `0` stays a real latitude when the pointer is non-nil |
| Producer freshness `unknown` | copied `unknown` on a present object | none | yes, as uncertainty | do not pass `fresh()` or the `"FRESH"` gate | retain the lowercase producer value |
| ETA status `unavailable` on a present item | copied onto `ETA.Status` | none | yes, as uncertainty | do not treat it as a fetch failure | retain; do not require `Build` to fail |
| Tracking freshness empty | client substitutes `"UNKNOWN"` (`current_trip.go` lines 216–218) | the empty producer field | yes, as a client sentinel | do not equate it with producer `unknown` | name the sentinel explicitly |
| ETA freshness empty | the ETA field stays empty; context freshness stays initial `"UNKNOWN"` (`provider.go` lines 131–133) | tracking and ETA empty fields are handled differently | yes, as uncertainty | do not use one rewrite for both contracts | keep the two results distinct |
| Value outside the producer constants | copied if non-empty | no allow-list rejects it | the uppercase fresh gate rejects it | do not treat an unknown token as `fresh` | keep it out of the fresh gate until a controller names an allow-list |
| Malformed JSON after HTTP 200 | `ErrUnavailable` from `json.Unmarshal` (`current_trip.go` lines 203–204), then swallowed by `Build` | the parse error | same as a transport failure for `Build` | do not anchor | source-unavailable, not producer `unknown` |

### 3.6 Chain and route activation

Public chain search forces `ExecutionSupported: false` on the search and on every candidate (`chainapi/project.go` lines 75 and 130). A current-trip seed may appear as `PublicSeedCurrentTrip` (`project.go` lines 104–105). That seed does not turn the chain into an execution.

`Service.ActivateRoutePlan` accepts only a version field from the caller (`route_plan_lifecycle.go` lines 201–204) and requires an idempotency key. For `ModeCurrentTrip`, `ensureRoutePlanFresh` calls `currentTrip.Build` and compares `trip.InputFingerprint` with the stored `CURRENT_TRIP_CONTEXT` dependency (`route_plan_lifecycle.go` lines 251–260). `inputFingerprint` includes `PositionFreshness`, `PositionRecordedAt`, `ETAStatus`, `ETAFreshness`, and `ETAObservedAt` (`fingerprint.go` lines 21–25, 47–48, and 56–57). It does not include latitude, longitude, or `EstimatedArrivalAt`.

`projectionCommand` calls `Build` again. Any error from that call, and a nil carrier, becomes the validation error `carrier company is required` (`execution_link.go` lines 119–121). The command then sent to `POST /internal/v1/transport-executions/from-route-plan-activation` (`execution_link.go` line 49, `executionproj/client.go` lines 19–23) carries the planned stop coordinates (`executionproj/client.go` lines 57–68). Those stop fields are not a live tracking fix. The activation decision still depends on the fingerprint, so tracking and ETA observations can make the plan stale before that command is sent.

A changed freshness string changes the fingerprint even when the coordinates are unchanged. A changed ETA status, ETA freshness, or `SourceObservedAt` changes it even when `EstimatedArrivalAt` is unchanged. A tracking or ETA error after the plan was built replaces the stored observation with nil pointers and the initial `"UNKNOWN"`, so the fingerprint differs and `ensureRoutePlanFresh` returns `planStale`. A later activation request that finds an existing activation row returns that row without another fingerprint comparison (`route_plan_lifecycle.go` lines 125–131). The first request that does not find a row does compare.

`StoreSeedResolver.ResolveTrip` uses `CurrentPosition` latitude and longitude as `routeplan.SourceTracking` and does not read `PositionFreshnessStatus` (`chainapi/resolve.go` lines 162–183). `current_trip_fill.go` lines 296–303 add `ReasonPositionNotFresh` or `ReasonETANotFresh` unless `fresh()` is true, and `insertRoad` still requires non-nil coordinates and an arrival (`current_trip_fill.go` lines 445–447).

A temporary projection error leaves the activation pending and still returns HTTP 200 (`execution_link.go` lines 57–62). A permanent rejection marks the activation rejected. NLO does not write TMS tables.

NLO-0.9D remains `READY_FOR_0_9D_RUNTIME=NO` in `NLO_0_9D_CHAIN_EXECUTION_ACCEPTANCE_DISCOVERY.md`. This discovery does not change that freeze.

### 3.7 Confirmed execution facts

The cargo fact NLO treats as onboard evidence is shipment-service onboard cargo, not a tracking event. Tracking and ETA are optional only for the success of `Provider.Build`. They are not optional for the current-trip activation fingerprint, the `"FRESH"` route gate, the fill freshness reasons, or the chain-seed coordinates. TMS execution authority for an activated plan remains the shipment-service acknowledgement. Tracking `ended` and ETA `completed` are not that acknowledgement.

## 4. Security gates

Status words in this section:

- `RESOLVED_FOR_SCOPE` means the cited check passed for the named scope and stays closed as a proof.
- `CLOSED_FOR_PRODUCTION` means the barrier is still in force. The gate is not passable.
- `BLOCKER_OPEN` means the production use is not authorized and the barrier is not a completed proof.

| Gate | Status on this SHA | Why it stays that way |
| --- | --- | --- |
| `J-NLO10E-001` | `RESOLVED_FOR_SCOPE` | Indented `SKIP`/`FAIL` and all 11 subtest names are required by the CI step. Exact-head run `44821722521` passed. The proof is a handler contract. |
| Lookup tenant SQL | `RESOLVED_FOR_SCOPE` | `tenant_id = $1` plus the two cross-tenant subtests. This is key isolation, not owner proof. |
| Tracking internal auth hardening | `CLOSED_FOR_PRODUCTION` | Handlers use `!=` and the development token has no environment guard (`config.go` line 89). Shared `internalauth` constant-time compare is not used. |
| Tracking owner fence | `CLOSED_FOR_PRODUCTION` | No owner-fence symbol exists under `services/tracking-service` on this SHA. Tenant SQL does not close this gate. |
| Kafka rebalance | `CLOSED_FOR_PRODUCTION` | `docs/reports/BINTRANS_CORE_VS_FULL_DECISION_2026_11.md` leaves CORE-P0-003C blocked. `REBALANCE_DESIGN=BLOCKED`. |
| Issue #344 permanent retry | `CLOSED_FOR_PRODUCTION` | `applyUntil` retries every `apply` error until context cancellation (`execution_event_consumer.go` lines 110–128). `PRODUCTION_CONSUMER_READINESS=BLOCKED`. |
| Historical rows without owner proof | `CLOSED_FOR_PRODUCTION` | `LookupTrackingStates` returns the tenant's stored row. It does not require an owner-proof token. |
| Stale or unavailable ETA as a route anchor | `CLOSED_FOR_PRODUCTION` | The lookup returns the state. `Provider.Build` stores it. Chain seed can use non-fresh coordinates. The `"FRESH"` gate does not accept producer `fresh`. |
| TMS execution authority | `CLOSED_FOR_PRODUCTION` | Activation projects to shipment-service. Chain `execution_supported` stays false. Tracking and ETA observations are not execution facts. |

`CLOSED_FOR_PRODUCTION` rows stay closed. This document does not mark any of them passable.

## 5. Go / No-Go

`YES` is used only where this SHA has a code path and a cited check. `PARTIAL` means the path exists and a narrower test passed. `NO` means this SHA does not make the capability production-ready.

| Capability | Implemented | Tested | Production-ready | Blocker |
| --- | --- | --- | --- | --- |
| Tracking source | PARTIAL | PARTIAL | NO | Client and handler lookup exist and 1.0E passed. `Provider.Build` drops tracking errors. Producer `fresh` does not satisfy the NLO `"FRESH"` consumers. |
| ETA source | PARTIAL | PARTIAL | NO | Delivery lookup exists and 1.0E passed. NLO omits `planned`. ETA freshness uses `expired`, not tracking `lost`. `forecast` and `Build` handle ETA errors differently. |
| Tenant isolation | PARTIAL | PARTIAL | NO | Lookup SQL and 1.0E cross-tenant cases pass. That is tenant-key isolation. It is not shipment ownership and not a tracking-row owner proof. |
| Owner authority | NO | NO | NO | Owner fence is `CLOSED_FOR_PRODUCTION`. Issue #344 and rebalance design stay `CLOSED_FOR_PRODUCTION`. |
| Freshness handling | PARTIAL | PARTIAL | NO | Evaluators recompute age. The 1.0E proof covers producer spelling only. It does not cover the uppercase NLO gate, `lost`, or ETA `expired`. |
| Multi-shipment chain | PARTIAL | PARTIAL | NO | Chain search persists and publishes `execution_supported=false`. `ResolveTrip` can anchor on coordinates without a freshness check. It does not execute a multi-shipment plan. |
| Route activation | PARTIAL | PARTIAL | NO | `ActivateRoutePlan` rebuilds the trip and compares a fingerprint that includes tracking and ETA observations. The projection stop coordinates remain planned points. |
| Execution linkage | PARTIAL | PARTIAL | NO | Handoff can link an acknowledgement. NLO-0.9D runtime is not authorized. Chain execution stays false. |
| Recovery | NO | NO | NO | Tracking `applyUntil` retries permanent errors. No owner-loss recovery exists on this SHA. Chain claim recovery is a different NLO search mechanism and does not recover tracking. |

`PRODUCTION_ADAPTER_READY=NO`. `PRODUCTION_CONSUMER_READINESS=BLOCKED`.

## 6. Roadmap

These stages are not authorized by this document. Each one stops rather than widening into chain execution or a production adapter.

### 6.1 Future NLO tracking and ETA policy

This is not an authorization to implement 1.1B. The earlier proposal that any tracking or ETA error fails `Provider.Build` is withdrawn. `Build` already continues after those errors (`provider.go` lines 116–135). Failing the whole build would also change activation, because `ensureRoutePlanFresh` and `projectionCommand` call `Build`. A `Build` error in `projectionCommand` is reported as `carrier company is required` (`execution_link.go` lines 119–121).

Separate policies, none of them in force:

| Action | Current code | Proposal until a controller approves it |
| --- | --- | --- |
| Build `CurrentTripContext` | Tracking and ETA errors are swallowed. Shipment, vehicle, and onboard errors still abort. | Keep the build. Record source-unavailable separately from producer `unknown` and from a present `unavailable` ETA. |
| Route planning | `route_plan.go` line 273 and `fresh()` require `"FRESH"`. Producer `fresh` fails that check. | Do not turn the gate on by renaming case until the controller accepts that behavior change. |
| Current-trip chain seed | `ResolveTrip` uses coordinates and ignores freshness (`resolve.go` lines 162–183). | Do not use a non-fresh or source-unavailable position as `SourceTracking`. |
| Route-plan fingerprint | Freshness and observation timestamps are inside `InputFingerprint`. Coordinates and `EstimatedArrivalAt` are not. | Leave the hash unchanged in 1.1B. A change here is an activation change. |
| Activation | `ModeCurrentTrip` rebuilds the trip and returns `planStale` on a fingerprint mismatch. An existing activation row is returned without a second comparison (`route_plan_lifecycle.go` lines 125–131). | Preserve that dependency. Do not hide a source error inside the stored fingerprint. |
| Showing incomplete data | Fill adds `ReasonPositionNotFresh` or `ReasonETANotFresh` when the uppercase check fails (`current_trip_fill.go` lines 296–303). | A user-facing distinction between uncertainty and source-unavailable needs a controller decision. |

These actions stay forbidden whether or not a fresh tracking or ETA observation exists:

- Chain `execution_supported` stays false (`chainapi/project.go` lines 75 and 130).
- NLO does not write TMS tables.
- Tracking `ended` and ETA `completed` are not TMS execution facts.
- Owner fence, Issue #344, and Kafka rebalance stay `CLOSED_FOR_PRODUCTION`.
- Latitude `0` is a coordinate when the pointer is non-nil. It is not a missing coordinate.
- This document does not authorize a code change.

### 6.2 Tracking owner and consumer gate

Goal: leave Issue #344, owner fence, and rebalance design as tracking-owned gates. NLO does not implement them.

Services: `tracking-service` consumer, only under a later tracking assignment.

Gates: `PRODUCTION_CONSUMER_READINESS=BLOCKED` until that assignment closes #344 with code, not a document.

Tests: permanent-error classification, quarantine, and loss of partition ownership. Those tests do not exist on this SHA.

Tenant risk: a retried or rebalanced consumer can apply a stop event after ownership changes. That risk blocks a production adapter even if the lookup SQL is tenant-scoped.

Acceptance: not defined here. This discovery does not estimate it.

Stop: any NLO stage that would mark the consumer production-ready, or that would read historical tracking rows as owner-proven.

### 6.3 Later, still unauthorized

A batch lookup, execution-stop ETA, and chain execution materialization wait on 6.1 and 6.2. NLO-0.9D runtime stays unauthorized. This discovery does not open it.

## 7. Stop

```text
IMPLEMENTATION_AUTHORIZED=NO
PRODUCTION_ADAPTER_READY=NO
EXECUTION_SUPPORTED=NO
PRODUCTION_CONSUMER_READINESS=BLOCKED
CONTROLLER_APPROVAL_REQUIRED=YES
DECISIONS_IN_FORCE=NO
NEXT_RECOMMENDED_STAGE=CONTROLLER_REVIEW_OF_PREIMPLEMENTATION_DECISIONS
```

## 8. Findings from Agent J

The six findings below are taken from the Agent J review of commit `4eee635d7108fa6ad2f18d4c48cebe26d0884fd5`. GitHub PR #399 had no review comments. This table does not mark any finding closed. Agent J has not confirmed the remediation.

| Finding ID | Severity | Evidence | Proposed correction | Review status |
| --- | --- | --- | --- | --- |
| `J-NLO11A-001` | MEDIUM | Producer freshness is lowercase `fresh`. `route_plan.go` line 273 and `current_trip_fill.go` line 541 compare `"FRESH"`. `sourceclient` does not normalize case. `nlo_03c_test.go` lines 48 and 107 supply the uppercase values. | Sections 3.4 and 5 now record the mismatch. Tracking source and freshness are `Tested=PARTIAL`. No shared normalization is assigned to tracking and ETA. | `DOC_REMEDIATED_PENDING_J` |
| `J-NLO11A-002` | MEDIUM | `ensureRoutePlanFresh` calls `Build` and compares `InputFingerprint`, which includes position freshness, `PositionRecordedAt`, ETA status, ETA freshness, and `ETAObservedAt` (`fingerprint.go` lines 21–25, 47–48, 56–57; `route_plan_lifecycle.go` lines 251–260). | Section 3.6 withdraws the claim that activation does not read tracking or ETA. The fingerprint is not changed in this stage. | `DOC_REMEDIATED_PENDING_J` |
| `J-NLO11A-003` | MEDIUM | `ResolveTrip` copies coordinates to `SourceTracking` without reading freshness (`resolve.go` lines 162–183). Consumers also include `route_plan.go` line 273 and `current_trip_fill.go` lines 296–303 and 445–447. | Section 6.1 names those consumers. 1.1B is not limited to `currenttrip` and `sourceclient`, and it is not authorized here. | `DOC_REMEDIATED_PENDING_J` |
| `J-NLO11A-004` | LOW | Lookup handlers compare the service token with `!=`. `internalauth.Authorize` uses constant time and rejects an empty token (`auth.go` lines 16–26). `config.go` line 89 supplies `dev_internal_tracking_token` with no environment guard. | Section 4 adds `Tracking internal auth hardening` as `CLOSED_FOR_PRODUCTION`. | `DOC_REMEDIATED_PENDING_J` |
| `J-NLO11A-005` | LOW | `Provider.Build` loads a tenant-scoped shipment before tracking (`provider.go` lines 75–79; `execution_evidence_service.go` lines 45–52). `forecast` fails on a non-404 ETA error (`prediction.go` lines 303–309). `Build` swallows that error. | Section 3.3 separates membership, SQL isolation, and owner proof. Section 3.5 records the `forecast` / `Build` divergence. | `DOC_REMEDIATED_PENDING_J` |
| `J-NLO11A-006` | LOW | The previous gate table used `OPEN` for barriers that are not passable, and section 3.4 omitted tracking `lost` and ETA `expired`. | Section 4 uses `RESOLVED_FOR_SCOPE`, `CLOSED_FOR_PRODUCTION`, and `BLOCKER_OPEN`. Section 3.4 lists both freshness sets. | `DOC_REMEDIATED_PENDING_J` |

## 9. PREIMPLEMENTATION_DECISIONS

None of these options takes effect in this commit. Each one has `CONTROLLER_APPROVAL_REQUIRED=YES`.

### 9.1 Vocabulary and case

`CURRENT_BEHAVIOR`: Tracking emits `unknown`, `fresh`, `stale`, `lost`. ETA freshness emits `unknown`, `fresh`, `stale`, `expired`. NLO stores the producer string and compares it with `"FRESH"`.

`OPTION_A`: Keep the producer spellings as the canonical values. Change the NLO comparisons to those spellings, separately for tracking and ETA, only after a later approved stage.

`OPTION_B`: Translate both contracts to uppercase `"FRESH"` inside `sourceclient`. That would make the current gate true for producer `fresh` and is a behavior change, not a documentation fix.

`RECOMMENDED_OPTION`: `OPTION_A`. The 1.0E proof already asserts lowercase `fresh` from the handler. One uppercase alias would hide the two different vocabularies.

`SECURITY_IMPACT`: `OPTION_B` would let a real lowercase `fresh` pass a gate that currently rejects it, including chain-adjacent planning. `OPTION_A` leaves that gate closed until the controller approves the comparison change.

`TEST_REQUIREMENTS`: A handler-shaped JSON fixture with lowercase `fresh`, `stale`, `lost`, `unknown`, and ETA `expired`, asserted at `route_plan`, fill, and `ResolveTrip`. Re-run `TestNLO10D_TrackingAndETARealHandlers` unchanged.

`CONTROLLER_APPROVAL_REQUIRED=YES`

### 9.2 Scope of a later 1.1B

`CURRENT_BEHAVIOR`: The same observation is read by `Provider.Build`, `route_plan.go`, `current_trip_fill.go`, `chainapi/resolve.go`, and the activation fingerprint.

`OPTION_A`: A later implementation stage includes those consumers, or explicitly freezes each one it does not change.

`OPTION_B`: Limit the later stage to `currenttrip` and `sourceclient`. The `"FRESH"` gate, chain seed, and activation fingerprint would stay as they are.

`RECOMMENDED_OPTION`: `OPTION_A`. A client-only change cannot correct `route_plan.go` line 273, `fresh()`, or `ResolveTrip`.

`SECURITY_IMPACT`: `OPTION_B` leaves chain seed able to anchor on stale, lost, or unknown coordinates.

`TEST_REQUIREMENTS`: One test per consumer for producer `fresh`, producer `stale`, a nil coordinate, and a swallowed source error.

`CONTROLLER_APPROVAL_REQUIRED=YES`

### 9.3 Error taxonomy

`CURRENT_BEHAVIOR`: 403 and 404 collapse to `ErrNotFound`. 401, 500, timeout, transport errors, and malformed JSON collapse to `ErrUnavailable`. `Build` then stores initial `"UNKNOWN"` and a nil pointer. `forecast` fails on every ETA error except `ErrNotFound`.

`OPTION_A`: Keep `Build` successful. Represent source-unavailable separately from producer `unknown`, a missing item, null coordinates, and ETA `unavailable`. Leave `forecast` unchanged until its own decision.

`OPTION_B`: Fail `Build` on every tracking or ETA error.

`RECOMMENDED_OPTION`: `OPTION_A`. `OPTION_B` would turn a tracking 401 into an activation failure and, inside `projectionCommand`, into `carrier company is required`.

`SECURITY_IMPACT`: Source-unavailable must not be usable as `SourceTracking` or as a fresh route start. Acceptable uncertainty may remain on the context.

`TEST_REQUIREMENTS`: Separate assertions for 401, 403, 404, 500, timeout, transport error, missing key, null coordinates, producer `unknown`, and ETA `unavailable`. Latitude `0` stays present.

`CONTROLLER_APPROVAL_REQUIRED=YES`

### 9.4 Activation fingerprint

`CURRENT_BEHAVIOR`: `PositionFreshness`, `PositionRecordedAt`, `ETAStatus`, `ETAFreshness`, and `ETAObservedAt` are in the hash. A freshness change with unchanged coordinates changes the hash. A source error after the plan was stored changes it to nil observation plus `"UNKNOWN"` and `ensureRoutePlanFresh` returns `planStale`.

`OPTION_A`: Preserve the current fingerprint fields. Do not change the hash in the next implementation stage.

`OPTION_B`: Remove tracking and ETA fields from the activation hash.

`RECOMMENDED_OPTION`: `OPTION_A`. Removing fields would let a current-trip plan activate after its observation changed. That is an activation change and is outside this document.

`SECURITY_IMPACT`: The current hash can reject a previously accepted plan when freshness moves from `fresh` to `stale`, or when a later read fails. That rejection is visible as `planStale`. A replay after an activation row exists does not compare the hash again.

`TEST_REQUIREMENTS`: Fingerprint equality for unchanged coordinates with a freshness change; a tracking error after a stored plan; a missing ETA after a stored ETA observation; a second activation with the same idempotency key after the row exists.

`CONTROLLER_APPROVAL_REQUIRED=YES`

### 9.5 Chain-seed freshness

`CURRENT_BEHAVIOR`: Any non-nil latitude and longitude can become `SourceTracking`. Freshness is not read. Missing coordinates return `ErrSeedIneligible`.

`OPTION_A`: Require the tracking producer value `fresh` and non-nil coordinates before `SourceTracking`. Do not invent a new minute threshold. The producer thresholds are 10 and 30 minutes for tracking (`freshness.go` lines 7–9).

`OPTION_B`: Keep any coordinates eligible.

`RECOMMENDED_OPTION`: `OPTION_A`, as a proposal only. The producer `fresh` check is not the same rule as the uppercase `"FRESH"` gate.

`SECURITY_IMPACT`: `OPTION_B` keeps a lost or unknown position eligible as a chain anchor. `OPTION_A` rejects that anchor and does not by itself authorize chain execution.

`TEST_REQUIREMENTS`: Seeds for producer `fresh`, `stale`, `lost`, `unknown`, nil coordinates, and latitude `0`. `execution_supported` remains false.

`CONTROLLER_APPROVAL_REQUIRED=YES`

### 9.6 Owner proof and allowed paths

`CURRENT_BEHAVIOR`: Shipment membership is tenant-scoped. Tracking SQL filters `tenant_id`. Neither check is a tracking-row owner proof. The owner fence is absent.

`OPTION_A`: A later NLO stage may edit only `sourceclient`, `currenttrip`, `service/route_plan.go`, `service/current_trip_fill.go`, `chainapi/resolve.go`, and their tests, and only after decision 9.2 is approved. It may not edit tracking handlers, Kafka, migrations, scoring, or the fingerprint function.

`OPTION_B`: Include tracking-service auth and the owner fence in that same NLO stage.

`RECOMMENDED_OPTION`: `OPTION_A`. Owner proof and internal-auth hardening stay tracking gates and remain `CLOSED_FOR_PRODUCTION`.

`SECURITY_IMPACT`: `OPTION_B` would mix an unapproved tracking security change into an NLO vocabulary change. Tenant membership must continue to be checked. It still must not be described as owner proof.

`TEST_REQUIREMENTS`: The existing cross-tenant 1.0E subtests stay unchanged. New tests must not claim owner proof from a `tenant_id` predicate.

`CONTROLLER_APPROVAL_REQUIRED=YES`

### 9.7 Metrics

`CURRENT_BEHAVIOR`: `Provider.Build` records a result string `built` after a swallowed tracking or ETA error. The caller cannot see 401, timeout, producer `stale`, or producer `unknown` from that string.

`OPTION_A`: Add aggregate counts for source-unavailable, producer `stale`, tracking `lost`, ETA `expired`, and producer `unknown`. Labels must not contain tenant id, shipment id, coordinates, or token text.

`OPTION_B`: Log the shipment id and tenant id with each swallowed error.

`RECOMMENDED_OPTION`: `OPTION_A`.

`SECURITY_IMPACT`: `OPTION_B` would put tenant data into operational logs. Counts without identifiers do not.

`TEST_REQUIREMENTS`: A unit test that a swallowed 401 and a producer `unknown` increment different counters, and that the metric name or labels contain neither a tenant id nor a shipment id.

`CONTROLLER_APPROVAL_REQUIRED=YES`
