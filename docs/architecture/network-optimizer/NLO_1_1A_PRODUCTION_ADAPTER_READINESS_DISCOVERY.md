# NLO-1.1A tracking and ETA production-adapter readiness

```text
AGENT=D
STAGE=NLO-TRACKING-ETA-PRODUCTION-ADAPTER-READINESS-1.1A
BASE_SHA=a9ef8f40bf0e7f75cb4aee3794e7841723347e4c
PREVIOUS_PR=367
PREVIOUS_STAGE=NLO-TRACKING-ETA-CI-PROOF-1.0E
PREVIOUS_AUDIT=PASS_CI_PROOF_CLOSEOUT_ONLY
J_NLO10E_001=CLOSED
PRODUCT_CODE_CHANGED=NO
MIGRATIONS_CHANGED=NO
STAGING_CHANGED=NO
PRODUCTION_CHANGED=NO
REAL_EXTERNAL_REQUESTS=NO
IMPLEMENTATION_AUTHORIZED=NO
PRODUCTION_ADAPTER_READY=NO
PRODUCTION_CONSUMER_READINESS=BLOCKED
EXECUTION_SUPPORTED=NO
FINAL_VERDICT=READY_FOR_J_ARCHITECTURE_REVIEW
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

Both internal handlers require `X-Internal-Service-Token` to equal the configured token. A mismatch returns HTTP 401 (`tracking_handler.go` lines 215–217, `eta_handler.go` lines 109–111). The token source is `TRACKING_INTERNAL_SERVICE_TOKEN`, then `INTERNAL_SERVICE_TOKEN`, then the development default (`config.go` line 89).

The tenant is the caller-supplied `X-Tenant-ID` (`tenantFromRequest`, `tracking_handler.go` lines 249–254). An empty header is HTTP 400. NLO sets that header and the service token in `HTTP.send` (`http.go` lines 151–164). An empty NLO token returns `predict.ErrUnavailable` before the request.

SQL then filters `tenant_id = $1`. The 1.0E subtests `cross_tenant_tracking` and `cross_tenant_eta` show that a caller tenant does not receive the other tenant's coordinates or arrival. That is tenant-key isolation on the lookup tables. It is not a proof that the shipment row is owned by that tenant in shipment-service. The lookup does not call shipment-service.

Invalid shipment id strings are skipped by the handler (`tracking_handler.go` lines 230–235). They do not become a 400.

Shipment execution context does check the returned `tenant_id` against the caller (`HTTP.ExecutionContext`, `current_trip.go` lines 37–38). Tracking and ETA item bodies do not repeat `tenant_id`, so the client cannot perform that same comparison on the tracking payload.

### 3.4 Freshness, null, and zero

Tracking freshness defaults are 10 minutes fresh and 30 minutes stale (`config.go` lines 66–67). ETA defaults are 15 and 60 minutes (`config.go` lines 72–73). `StateEvaluator.RefreshComputedState` recomputes freshness from `LastRecordedAt` at read time (`state_evaluator.go` line 64). `LookupTrackingStates` returns that recomputation and does not write it back. `GetTrackingSummary` may persist the refresh (`tracking_query_service.go` lines 59–65). The NLO path uses the lookup, so it does not depend on a stored freshness column staying current.

`mapTrackingSummary` emits `lastKnownPosition` only when latitude, longitude, `LastRecordedAt`, and `AgeSeconds` are all non-nil (`tracking_query_service.go` lines 135–141). Latitude `0` is non-nil and remains a coordinate. The 1.0E subtest `zero_latitude` passed. A null coordinate omits the position. NLO copies the pointers and does not substitute zero (`current_trip.go` lines 211–214).

An ETA arrival is set only when `estimatedArrivalAt` is present (`eta_handler.go` lines 220–221). A null arrival stays nil. NLO copies that pointer (`current_trip.go` lines 249–252). `HTTP.ETA` additionally requires both arrival and `sourceObservedAt` before `Present: true` (`http.go` lines 141–144).

Stale and unknown freshness are still returned to NLO. `Provider.Build` stores them. It does not reject a stale position or an unavailable ETA.

### 3.5 Errors, timeout, retry, idempotency

`HTTP.send` uses a 5 second client timeout and refuses redirects (`http.go` lines 28–35). HTTP 200 returns the body. HTTP 404 and 403 become `predict.ErrNotFound`. Any other status or transport error becomes `predict.ErrUnavailable` (`http.go` lines 177–184). `mapTripErr` maps `ErrNotFound` to `currenttrip.ErrNotFound` and everything else to `currenttrip.ErrUnavailable` (`current_trip.go` lines 256–263).

There is no retry in `sourceclient`. The execution projection client also uses a 5 second timeout (`executionproj/client.go` line 129) and is a different call.

The tracking and ETA lookups are reads. They have no idempotency key. Repeating them is safe as a read. Route-plan activation is the call that requires `Idempotency-Key` (`route_plan_lifecycle.go` lines 95–98).

### 3.6 Chain and route activation

Public chain search forces `ExecutionSupported: false` on the search and on every candidate (`chainapi/project.go` lines 75 and 130). A current-trip seed may appear as `PublicSeedCurrentTrip` (`project.go` lines 104–105). That seed does not turn the chain into an execution.

`Service.ActivateRoutePlan` accepts only a version field from the caller (`route_plan_lifecycle.go` lines 201–204) and requires an idempotency key. `finishActivationHandoff` sends `executionproj.Command` to shipment-service `POST /internal/v1/transport-executions/from-route-plan-activation` (`execution_link.go` line 49, `executionproj/client.go` lines 19–23). The command comment states that NLO does not write TMS tables. A temporary projection error leaves the activation pending and still returns HTTP 200 (`execution_link.go` lines 57–62). A permanent rejection marks the activation rejected.

This handoff does not read tracking state or ETA. Multi-stop coordinates in `executionproj.Stop` are planned route-plan coordinates (`executionproj/client.go` lines 57–68), not a live tracking fix.

NLO-0.9D remains `READY_FOR_0_9D_RUNTIME=NO` in `NLO_0_9D_CHAIN_EXECUTION_ACCEPTANCE_DISCOVERY.md`. This discovery does not change that freeze.

### 3.7 Confirmed execution facts

The cargo fact NLO treats as onboard evidence is shipment-service onboard cargo, not a tracking event. Tracking position and ETA are optional decorations on `CurrentTripContext`. TMS execution authority for an activated plan is the shipment-service projection acknowledgement. Tracking-service is not that authority.

## 4. Security gates

| Gate | Status on this SHA | Why it stays that way |
| --- | --- | --- |
| `J-NLO10E-001` | CLOSED | Indented `SKIP`/`FAIL` and all 11 subtest names are required by the CI step. Exact-head run `44821722521` passed. |
| Lookup tenant SQL | CLOSED for the 1.0E contract | `tenant_id = $1` plus the two cross-tenant subtests. |
| Tracking owner fence | OPEN | No owner-fence symbol exists under `services/tracking-service` on this SHA. |
| Kafka rebalance | OPEN, `REBALANCE_DESIGN=BLOCKED` | `docs/reports/BINTRANS_CORE_VS_FULL_DECISION_2026_11.md` leaves CORE-P0-003C blocked. PR #346 is not a consumer fix. |
| Issue #344 permanent retry | OPEN | `applyUntil` retries every `apply` error until context cancellation (`execution_event_consumer.go` lines 110–128). `PRODUCTION_CONSUMER_READINESS=BLOCKED`. |
| Historical rows without owner proof | OPEN | `LookupTrackingStates` returns the tenant's stored row. It does not require an owner-proof token. |
| Stale or unavailable ETA | OPEN for production use | The lookup returns the state. `Provider.Build` keeps it optional and does not fail closed. |
| TMS execution authority | CLOSED as a boundary, not as an adapter | Activation projects to shipment-service. Chain `execution_supported` stays false. |

Closed rows stay closed. Open rows are not given a production-ready status by this document.

## 5. Go / No-Go

`YES` is used only where this SHA has a code path and a cited check. `PARTIAL` means the path exists and a narrower test passed. `NO` means this SHA does not make the capability production-ready.

| Capability | Implemented | Tested | Production-ready | Blocker |
| --- | --- | --- | --- | --- |
| Tracking source | PARTIAL | YES | NO | Client and handler lookup exist. `Provider.Build` drops tracking errors. No production consumer. |
| ETA source | PARTIAL | YES | NO | Delivery lookup exists. NLO omits `planned`. Execution-stop ETA is not called. |
| Tenant isolation | PARTIAL | YES | NO | Lookup SQL and 1.0E cross-tenant cases pass. The internal token trusts the caller-supplied tenant header. Shipment ownership is not checked on this route. |
| Owner authority | NO | NO | NO | Owner fence is absent. Issue #344 and rebalance design stay open. |
| Freshness handling | PARTIAL | YES | NO | Evaluator recomputes age. Stale and unknown values are still accepted by `Provider.Build`. |
| Multi-shipment chain | PARTIAL | YES | NO | Chain search persists and publishes `execution_supported=false`. It does not execute a multi-shipment plan. |
| Route activation | PARTIAL | PARTIAL | NO | `ActivateRoutePlan` and the projection client exist. Post-merge job `network-optimizer-nlo04b-integration` was green. That is not a live TMS proof, and it does not consume tracking. |
| Execution linkage | PARTIAL | PARTIAL | NO | Handoff can link an acknowledgement. NLO-0.9D runtime is not authorized. Chain execution stays false. |
| Recovery | NO | NO | NO | Tracking `applyUntil` retries permanent errors. No owner-loss recovery exists on this SHA. Chain claim recovery is a different NLO search mechanism and does not recover tracking. |

`PRODUCTION_ADAPTER_READY=NO`. `PRODUCTION_CONSUMER_READINESS=BLOCKED`.

## 6. Roadmap

These stages are not authorized by this document. Each one stops rather than widening into chain execution or a production adapter.

### 6.1 NLO-TRACKING-ETA-FAIL-CLOSED-CURRENT-TRIP-1.1B

Goal: define and test the current-trip rule for a tracking error, a missing item, null coordinates, latitude `0`, an unavailable ETA, and a stale ETA. The rule must say which of those values may remain on `CurrentTripContext` and which must fail the build.

Services: `network-optimizer-service` `currenttrip` and `sourceclient` only.

Gates: keep `J-NLO10E-001` closed. Do not change tracking handlers, scoring, activation, or migrations.

Tests: extend current-trip tests so an error is distinct from `UNKNOWN`, and so null is distinct from zero. Re-run `TestNLO10D_TrackingAndETARealHandlers` unchanged.

Tenant risk: a fail-closed build must still use the caller tenant on every source call. It must not query a second tenant to fill a gap.

Acceptance: the new test names are `PASS` in exact-head CI, and `execution_supported` is still false.

Stop: if the rule needs a tracking schema change, a Kafka change, or route activation.

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
NEXT_RECOMMENDED_STAGE=NLO-TRACKING-ETA-FAIL-CLOSED-CURRENT-TRIP-1.1B
```
