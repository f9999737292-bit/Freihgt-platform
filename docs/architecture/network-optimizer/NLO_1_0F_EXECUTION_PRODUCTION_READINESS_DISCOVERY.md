# NLO-1.0F execution and production readiness

```text
AGENT=D
STAGE=NLO-1.0F-R2
BASE_SHA=892361db87985f35132aef86550a025909af648a
PREVIOUS_HEAD=8d7e9f9c88d4f7ad140798c93f0b57bb00293880
PREVIOUS_AUDIT=REMEDIATION_REQUIRED
PREVIOUS_PR=367
PR367_STATUS=MERGED
AP_NLO10F_J_001=DOC_REMEDIATED_PENDING_J
AP_NLO10F_J_002=DOC_REMEDIATED_PENDING_J
PRODUCT_CODE_CHANGED=NO
MIGRATIONS_CHANGED=NO
STAGING_CHANGED=NO
PRODUCTION_CHANGED=NO
IMPLEMENTATION_AUTHORIZED=NO
EXECUTION_ADAPTER_READY=NO
TRACKING_INTEGRATION_READY=NO
ETA_INTEGRATION_READY=NO
TMS_HANDOFF_READY=NO
CHAIN_EXECUTION_SUPPORTED=NO
READY_FOR_0_9D_RUNTIME=NO
PRODUCTION_CONSUMER_READINESS=BLOCKED
FINAL_VERDICT=READY_FOR_J_R2_ARCHITECTURE_REVIEW
CONTROLLER_APPROVAL_REQUIRED=YES
```

This note records the execution path on `a9ef8f40`. It does not enable a pilot, change NLO-0.9B, or authorize NLO-0.9D. A document is not a running adapter.

## 1. Baseline

The discovery commit `8d7e9f9c` was based on `a9ef8f40bf0e7f75cb4aee3794e7841723347e4c`, the merge of PR #367. Current `origin/main` is `892361db87985f35132aef86550a025909af648a`. The ten commits between those SHAs are gateway redirect changes from PR #398 and PR #407. They do not change `network-optimizer-service`, `shipment-service`, `tracking-service`, `control-tower-read-model-service`, or this document's citations. This branch now contains that main history. The R2 text is a documentation correction. It does not change product code.

PR #367 is `MERGED`. Its title is `test(nlo): tracking and ETA real handler proof v1.0D`. This discovery does not edit that PR. The merge added only:

- `.github/workflows/ci.yml`
- `services/network-optimizer-service/internal/sourceclient/nlo_10d_tracking_eta_proof_test.go`
- `services/tracking-service/cmd/nlo10d-contract-harness/main.go`

The CI step runs `TestNLO10D_TrackingAndETARealHandlers` and requires the parent `PASS` plus the named subtests. The harness file says it is a test-only loopback process and is not registered in the tracking runtime.

Draft PR #399 (`c9fb34c8`) is an unmerged 1.1A document. It is not on this baseline and its decisions are not used here.

## 2. What is already implemented

### 2.1 One accepted route plan can be projected

`Service.ActivateRoutePlan` requires `Idempotency-Key` and a body whose only field is `version` (`route_plan_lifecycle.go` lines 90–98 and 196–212). For a plan with no activation row it checks freshness, accepted status, and shipment eligibility, then inserts one row in `PENDING_EXECUTION` (`routeplan/policy.go` lines 18–20).

`finishActivationHandoff` sends `executionproj.Command` to shipment-service `POST /internal/v1/transport-executions/from-route-plan-activation` (`execution_link.go` line 49, `executionproj/client.go` lines 19–23). The client times out at 5 seconds and does not follow redirects (`executionproj/client.go` lines 124–134). HTTP 200 and 201 become an acknowledgement. HTTP 400, 403, 404, and 409 are permanent. Other statuses and transport errors are temporary (`executionproj/client.go` lines 162–175).

Shipment-service accepts that route only from `X-Internal-Service-Name: network-optimizer-service` after the internal token check (`execution_projection_handler.go` lines 17–35, `router.go` lines 150–155). `AcceptRoutePlanActivation` checks that the carrier company belongs to the operating tenant (`transport_execution_repository.go` lines 142–158 and 161–175). The acknowledgement is the committed revision, not the shipment event.

NLO does not write `transport` tables. The command comment states that (`executionproj/client.go` lines 23–24). Stop coordinates on the command are the planned route-plan stops (`execution_link.go` lines 133–141).

### 2.2 Replay of the same plan

A second activate of a plan that already has an activation row returns that row (`route_plan_lifecycle.go` lines 125–131) and then calls `finishActivationHandoff`. That call projects again only while the status is `PENDING_EXECUTION`. `EXECUTION_LINKED` and `REJECTED` return the stored row (`execution_link.go` lines 42–44).

Shipment-service stores one revision per `operating_tenant_id` and `source_activation_id`. `CanonicalDigest` hashes the canonical projection, including `carrier_company_id`, `vehicle_id`, and `driver_id` (`transport_execution.go` lines 163–169 and 508–510). The same digest replays. A different digest is `ACTIVATION_BODY_CONFLICT` (`transport_execution.go` lines 178–182). A first projection that sets `supersedes_route_plan_id` or `supersedes_activation_id` is `ReasonExecutionPlanConflict` (`transport_execution.go` lines 184–186). `projectionCommand` does not set those fields (`execution_link.go` lines 115–132).

A temporary projection error leaves the activation pending and still returns HTTP 200 (`execution_link.go` lines 50–62). A permanent rejection marks the activation `REJECTED`.

The pending retry is not a replay of the first command. `finishActivationHandoff` calls `projectionCommand` again (`execution_link.go` lines 42–49). That function reads `CarrierCompanyID` and `DriverID` from a new `currentTrip.Build` (`execution_link.go` lines 119–131). `VehicleID` stays the vehicle stored on the route plan. If the first `Project` already committed in TMS and the acknowledgement was lost, a later driver or carrier change produces a different digest. TMS then returns 409. The NLO client treats 409 as permanent (`executionproj/client.go` lines 170–172) and `finishActivationHandoff` marks the activation `REJECTED` (`execution_link.go` lines 50–56). The TMS execution remains. NLO has no successor client that can cancel it.

`AcceptRoutePlanActivation` calls `requireCarrierTenant` before `Project` (`transport_execution_repository.go` lines 142–146). A carrier row with `deleted_at` set fails that check with `TENANT_DENIED` and HTTP 403 (`transport_execution_repository.go` lines 161–174, `transport_execution_commands.go` lines 217–223). The NLO client treats 403 as permanent as well. A soft-deleted carrier after a committed projection therefore follows the same split: TMS keeps the execution, and NLO can move the activation to `REJECTED`. Two overlapping activate calls can rebuild two different live commands for the same pending row. This description is the call chain. This stage did not run that race.

### 2.3 What does not come back

`network-optimizer-service` has no Kafka consumer. It does not subscribe to execution events, ETA changes, or Control Tower rows. After `EXECUTION_LINKED`, a later tracking or ETA observation does not build a new plan and does not call TMS again.

Replacement of a live execution is `POST /internal/v1/transport-executions/{executionId}/successor-revision` (`router.go` lines 157–161). That route checks the internal token and does not require the network-optimizer caller name. NLO has no client for it.

### 2.4 Tracking and ETA

`HTTP.TrackingState` and `HTTP.TrackingETA` are pull lookups. `TestNLO10D_TrackingAndETARealHandlers` proves the handler contract on an ephemeral database. It does not deploy tracking-service and does not consume Kafka.

Producer freshness is lowercase. Tracking uses `unknown`, `fresh`, `stale`, and `lost`. ETA freshness uses `unknown`, `fresh`, `stale`, and `expired`. NLO route planning compares `PositionFreshnessStatus` with `"FRESH"` (`route_plan.go` line 273) and `fresh()` is `status == "FRESH"` (`current_trip_fill.go` line 541). The producer spelling does not pass those checks. `Provider.Build` continues when the tracking or ETA call fails.

`inputFingerprint` includes position freshness, position recorded time, ETA status, ETA freshness, and ETA observed time (`currenttrip/fingerprint.go` lines 21–25). `ensureRoutePlanFresh` compares that fingerprint for `ModeCurrentTrip` (`route_plan_lifecycle.go` lines 251–260). A later activation of a plan that is not yet linked can therefore become stale when the observation changes. That check is not a subscription to execution events.

### 2.5 Chain and Control Tower

Public chain search persists and returns candidates with `ExecutionSupported: false` (`chainapi/project.go` lines 75 and 130). NLO-0.9B section K keeps that flag false through 0.9C and does not authorize 0.9D. NLO-0.9D on this tree still says `READY_FOR_0_9D_RUNTIME=NO`. Chain search does not accept, activate, or project.

Control Tower reads shipment-status events and route-execution events in its own consumer (`control-tower-read-model-service/internal/consumer/consumer.go` lines 177–195). It does not read `network.route_plan.*`. NLO emits `network.route_plan.execution_linked` inside its own transaction (`execution_link.go` lines 20 and 82–88). That event is not a Control Tower projection.

## 3. Proposed execution loop

This section is a proposal. It does not change 0.9B, 0.9C, 0.9D, the TMS projection contract, or the successor-revision contract.

```text
accepted route plan
  -> ActivateRoutePlan (existing)
  -> POST from-route-plan-activation (existing)
  -> shipment-service execution owner
  -> shipment execution events
       -> tracking consumer (existing, production blocked)
       -> Control Tower execution applier (existing, not an NLO input)
  -> a future NLO decision, only after a separate approval
       -> a new route plan
       -> successor-revision on the same execution
```

Rules that stay in force:

- TMS remains the execution owner. NLO does not write TMS tables.
- Chain results stay `execution_supported=false`. Chain search does not project.
- The first projection must not carry supersedes fields. Shipment-service rejects that shape.
- A linked activation is not projected again. A replacement is a new plan plus `successor-revision`, which NLO does not call today.
- Tracking position and ETA are not execution facts. `ended` and `completed` do not link an activation.
- Control Tower is a read model of shipment and execution events. It is not the place that applies a route plan.
- Issue #344 stays open. `applyUntil` retries every tracking apply error (`execution_event_consumer.go` lines 110–128). `PRODUCTION_CONSUMER_READINESS=BLOCKED`. `REBALANCE_DESIGN=BLOCKED`. An NLO consumer must not reuse that loop.
- A pending retry must not be described as safe while `projectionCommand` rebuilds carrier and driver. No option below is in force.

Pending-retry options, for a later controller decision:

| Option | What it would change | What it does not change |
| --- | --- | --- |
| A. Freeze the first projection command | Persist the carrier, driver, and digest with the activation row and resend that body | The current code, which rebuilds the command |
| B. Reconcile instead of reject | On 409 `ACTIVATION_BODY_CONFLICT` or 403 `TENANT_DENIED`, read whether a revision for that activation already exists before `MarkRoutePlanActivationRejected` | NLO has no such read today |
| C. Replay check before the carrier check | shipment-service would run the stored-digest replay before `requireCarrierTenant` | A digest change would still be 409 |

`RECOMMENDED_OPTION` for a later stage is B, because a lost acknowledgement plus a live TMS execution cannot be repaired by marking NLO `REJECTED`, and NLO has no successor client. A and C are shipment or activation-persistence changes. None of the three is authorized by this document. `CONTROLLER_APPROVAL_REQUIRED=YES`.

Tenant controls that exist today, and what they are not:

| Check | Code | Limit |
| --- | --- | --- |
| Route-plan row | `GetRoutePlan(ctx, actor.TenantID, id)` | The caller tenant must own the plan. |
| Projection tenant | Header tenant must equal `operating_tenant_id` (`execution_projection_handler.go` lines 71–74) | A body tenant cannot override the header. |
| Carrier company | `requireCarrierTenant` (`transport_execution_repository.go` lines 161–175) | The company row must belong to that tenant and have `deleted_at` null. This check runs before the digest replay. |
| Tracking lookup | `tenant_id = $1` | Tenant-key isolation. Not a tracking-row owner proof. |
| Successor revision | Internal token only (`router.go` lines 157–161) | Not limited to network-optimizer-service. |

## 4. Production readiness

### 4.1 Adapters required before a pilot

| Adapter | On this SHA | Required before pilot |
| --- | --- | --- |
| Route-plan activation client | Present | Keep the current idempotency and pending retry. |
| TMS projection HTTP client | Present | Live shipment-service URL, token, and tenant are not configured by this discovery. |
| Successor-revision client | Absent in NLO | Required only if a later decision allows NLO to replace a linked execution. |
| Execution-event consumer in NLO | Absent | Required before automatic recalculation. Separate from the tracking consumer. |
| Tracking and ETA pull | Present as lookups | Not sufficient. The `"FRESH"` gate does not match producer `fresh`. |
| Control Tower writer from NLO | Absent | Not required for TMS handoff. Required only for an NLO recommendation surface. |

### 4.2 Blockers

| Blocker | State |
| --- | --- |
| Chain execution | Frozen. `READY_FOR_0_9D_RUNTIME=NO`. |
| Tracking production consumer | `PRODUCTION_CONSUMER_READINESS=BLOCKED`. Issue #344. |
| Owner fence | No owner-fence symbol under `services/tracking-service`. |
| NLO execution feedback | No consumer and no successor client. |
| Freshness vocabulary | Producer lowercase `fresh` fails the NLO `"FRESH"` checks. |
| Same-plan reapply | One activation row is not inserted twice. A pending retry can still diverge from the committed TMS body. |
| Pending retry digest drift | `AP-NLO10F-J-001`. Lost acknowledgement, then a changed driver or carrier, or a soft-deleted carrier, can leave TMS active and NLO `REJECTED`. |
| Staging and production | Not changed and not verified here. |

### 4.3 Failure modes

| Failure | Current result | Safe continuation | Forbidden action |
| --- | --- | --- | --- |
| TMS timeout or 5xx before a commit | Temporary error, activation stays `PENDING_EXECUTION`, HTTP 200 | A later activate calls `projectionCommand` again | Do not treat that retry as the same digest. Do not insert a second activation row. |
| TMS timeout or 5xx after TMS has committed | NLO still sees a temporary error and stays `PENDING_EXECUTION` | The next activate rebuilds carrier and driver | Do not call the retry safe. A changed carrier or driver becomes the 409 row below. |
| TMS 409 `ACTIVATION_BODY_CONFLICT` after a committed projection | NLO marks `REJECTED`. TMS keeps the execution | Stop and reconcile outside this stage | Do not create a second activation id for the same plan. Do not claim NLO can cancel the TMS execution. |
| TMS 403 after the carrier row is soft-deleted | `requireCarrierTenant` fails before the digest replay. NLO marks `REJECTED` | Stop | Do not describe 403 as proof that TMS has no execution. |
| TMS 400, or 409 before any commit | Activation becomes `REJECTED` | Stop | Do not retry that body under a new activation id. |
| Ack tenant, activation, or plan does not match | Conflict, no link (`execution_link.go` lines 64–66) | Stop | Do not mark the plan linked. |
| Tracking or ETA error while building the command | `projectionCommand` returns `carrier company is required` when `Build` fails (`execution_link.go` lines 119–121) | The activation row can already exist | Do not describe that validation as a missing carrier until `Build` itself failed for a carrier reason. |
| Freshness changes before the first link | `ensureRoutePlanFresh` returns stale | Do not project | Do not weaken the fingerprint in this stage. |
| ETA changes after link | No NLO action | The linked execution remains the TMS revision | Do not apply the old plan again and do not invent a successor call. |
| Tracking consumer permanent error | `applyUntil` retries without classification | Control Tower and NLO stay independent | Do not declare the consumer production-ready. |
| Empty or lost position used as a chain seed | `ResolveTrip` uses coordinates without a freshness check | Chain execution stays false | Do not treat that seed as an executable plan. |

### 4.4 Pilot versus architecture

| Function | Evidence | Classification |
| --- | --- | --- |
| Evaluate, accept, and activate one route plan | Service methods and the projection client | Code present. Not a production pilot. `execution_supported` on the plan stays false. |
| Project that activation into one TMS execution | Shipment handler, repository, and integration tests in shipment-service | Code present. This discovery did not run staging. |
| Public chain search | `chainapi` and NLO-0.9C persistence | Search code present. Execution is architecture-frozen and is not a pilot. |
| Tracking and ETA lookup | PR #367, stage name v1.0D, test `TestNLO10D_TrackingAndETARealHandlers` | Handler contract only. There is no NLO-1.0E document on this tree. |
| Recalculate after an execution deviation | No NLO consumer and no successor client | Architecture only. |
| Show an NLO recommendation in Control Tower | Control Tower consumes shipment and execution events, not NLO events | Separate read model. Not an NLO pilot surface. |

## 5. Compatibility

| Decision | What this discovery keeps |
| --- | --- |
| NLO-0.9B freeze | `execution_supported=false` through chain search. 0.9D is not authorized. Horizon, budget, and the separate chain resource stay as frozen. |
| NLO-0.9C public search | The implemented search may return candidates. It does not accept, activate, or project. |
| NLO-1.0D lookup proof | `TestNLO10D_TrackingAndETARealHandlers` from PR #367 stays the lookup contract. It is not an execution adapter. The name 1.0E is not a stage on this tree. |
| TMS execution | shipment-service owns the execution. The first projection contract and the successor-revision contract stay as written. |
| Control Tower | Its consumer and projections stay outside NLO. |

## 6. Later implementation tasks

These tasks are not authorized. Each one stops rather than widening into chain execution.

1. Keep the PR #367 `TestNLO10D_TrackingAndETARealHandlers` proof and the current activation idempotency unchanged.
2. A later test must show that a second `PENDING_EXECUTION` activate rebuilds carrier and driver, that `EXECUTION_LINKED` and `REJECTED` do not call TMS again, and that a 409 or 403 after a committed projection is not a safe retry. Use fakes. Do not add a migration. Do not write that test in this stage.
3. Leave `SupersedesRoutePlanID` unset on the first projection. A change there conflicts with `RejectReplayOrCreate`.
4. Decide, in a separate review, whether NLO may call `successor-revision`. That route is not restricted to network-optimizer-service today. Restricting the caller is a shipment-service change and is out of scope until approved.
5. Design an NLO execution-event consumer as a new process. Do not extend `applyUntil`. Do not start it while Issue #344 is open for any shared tracking partition.
6. Treat an ETA or position change after link as input to a new plan. Do not reactivate the linked plan.
7. Keep chain `execution_supported=false` until a future 0.9D runtime is separately approved. This list does not approve it.
8. Do not treat tenant SQL or the carrier-company check as a tracking owner fence.

## 7. Tests for a later stage

No new test runs in this discovery. A later stage, after approval, would need:

- Replay of the same activation id and digest against shipment-service.
- A changed body for the same activation id returning conflict.
- Temporary TMS failure before commit leaving `PENDING_EXECUTION`, with the next command rebuilt from the live trip.
- Lost acknowledgement, then a changed driver or carrier, then TMS 409, with NLO `REJECTED` and the TMS execution still present.
- Soft-deleted carrier after a committed projection, then TMS 403, with the same split.
- Two overlapping activate calls for one pending row with different live carrier or driver values.
- Permanent TMS rejection leaving `REJECTED` and a second call not projecting.
- Cross-tenant plan id returning not found.
- Carrier company in another tenant returning the tenant denial.
- Producer `fresh` asserted separately from the `"FRESH"` gate, without changing the gate in the same change.
- `execution_supported=false` on the chain response after any of the above.

## 8. Findings from Agent J

These rows record the review of `8d7e9f9c`. They are not closed. Agent J has not confirmed this revision.

| Finding ID | Severity | Evidence | Proposed correction | Review status |
| --- | --- | --- | --- | --- |
| `AP-NLO10F-J-001` | MEDIUM | A pending retry rebuilds `CarrierCompanyID` and `DriverID`. `CanonicalDigest` includes both. After a lost acknowledgement, 409 or a later 403 marks NLO `REJECTED` while TMS keeps the execution. Section 4.3 previously called the retry safe. | Sections 2.2, 3, 4.2, 4.3, and 7 now record the split and three unapproved options. No product code changes. | `DOC_REMEDIATED_PENDING_J` |
| `AP-NLO10F-J-002` | LOW | The discovery called the PR #367 proof "1.0E". The merged title and test are v1.0D and `TestNLO10D_TrackingAndETARealHandlers`. | Sections 4.4, 5, and 6 now use 1.0D. | `DOC_REMEDIATED_PENDING_J` |

`STALE_BASE` from that review is addressed by merging `892361db` into this branch. The merge does not change the NLO, shipment, tracking, or Control Tower service code cited above.

## 9. Stop

```text
IMPLEMENTATION_AUTHORIZED=NO
EXECUTION_ADAPTER_READY=NO
TMS_HANDOFF_READY=NO
CHAIN_EXECUTION_SUPPORTED=NO
READY_FOR_0_9D_RUNTIME=NO
PRODUCTION_CONSUMER_READINESS=BLOCKED
PENDING_RETRY_DIGEST_DRIFT=DOCUMENTED
CONTROLLER_APPROVAL_REQUIRED=YES
MERGE_PERFORMED=NO
```
