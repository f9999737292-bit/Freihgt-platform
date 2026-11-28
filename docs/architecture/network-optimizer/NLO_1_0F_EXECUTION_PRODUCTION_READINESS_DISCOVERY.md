# NLO-1.0F execution and production readiness

```text
AGENT=D
STAGE=NLO-1.0F-EXECUTION-READINESS-DOC-REMEDIATION-R2
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
PENDING_RETRY_DIGEST_DRIFT=PRODUCTION_BLOCKER
RECOMMENDED_OPTION=A_PLUS_B_PLUS_C
FINAL_VERDICT=READY_FOR_J_R2_ARCHITECTURE_REVIEW
CONTROLLER_APPROVAL_REQUIRED=YES
```

This note records the execution path on current `origin/main` `892361db`. It does not enable a pilot, change NLO-0.9B, or authorize NLO-0.9D. A document is not a running adapter. The cited NLO, shipment, tracking, and Control Tower lines are unchanged by the gateway commits that moved main from `a9ef8f40` to `892361db`.

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

`PENDING_RETRY_DIGEST_DRIFT=PRODUCTION_BLOCKER`. A later activate is not unconditionally safe.

`finishActivationHandoff` calls `projectionCommand` again while the row is `PENDING_EXECUTION` (`execution_link.go` lines 42–49). That rebuild reads `CarrierCompanyID` and `DriverID` from a new `currentTrip.Build` (`execution_link.go` lines 119–131). `VehicleID` is `graph.Plan.VehicleID` (`execution_link.go` line 131). A change of the live vehicle does not enter the retried command. The digest field `vehicle_id` changes only when the stored route-plan vehicle changes, which this retry does not do.

The lost-acknowledgement sequence is:

1. NLO inserts one route-plan activation in `PENDING_EXECUTION`.
2. TMS commits the execution for that activation id.
3. The acknowledgement is lost. The NLO client treats a timeout, an acknowledgement decode error, and a 5xx as temporary (`executionproj/client.go` lines 154–175).
4. NLO leaves the activation in `PENDING_EXECUTION` and returns HTTP 200 (`execution_link.go` lines 50–62).
5. Before the next attempt the live driver or the live carrier changes. A live vehicle change is not copied into the command.
6. The next `PENDING_EXECUTION` attempt builds a new command from `currentTrip.Build`.
7. `CanonicalDigest` of that command differs because `driver_id` or `carrier_company_id` differs.
8. TMS returns 409 `ACTIVATION_BODY_CONFLICT` (`transport_execution.go` lines 178–182).
9. The NLO client treats 409 as permanent, and `finishActivationHandoff` marks the activation `REJECTED` (`execution_link.go` lines 50–56). The TMS execution remains. NLO has no successor client that can cancel it.

A crash after the TMS commit and before `EXECUTION_LINKED` is the same sequence. The row stays `PENDING_EXECUTION`, and the next process runs the rebuild above.

Soft-delete is a separate path. `AcceptRoutePlanActivation` calls `requireCarrierTenant` before `Project` (`transport_execution_repository.go` lines 142–146). The company must match the operating tenant and have `deleted_at` null (`transport_execution_repository.go` lines 161–174). Failure is `TENANT_DENIED` and HTTP 403 (`transport_execution_commands.go` lines 217–223). The stored-digest replay inside `Project` does not run. The NLO client treats 403 as permanent and can mark `REJECTED` while the committed execution remains.

Two overlapping activate calls can both observe `PENDING_EXECUTION` and build two different live commands. The first commit wins. The second can receive 409 and mark the shared row `REJECTED`. This is the call chain. This stage did not execute it.

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

Pending-retry options, for a later controller decision. None is in force. `CONTROLLER_APPROVAL_REQUIRED=YES`.

A. Immutable projection snapshot. At the activation insert, store the projection command and every field that `CanonicalDigest` hashes. Every later `PENDING_EXECUTION` send uses that stored command. The live trip is not read again for carrier, driver, or vehicle.

B. Reconciliation state. After an uncertain TMS result, or on 409 `ACTIVATION_BODY_CONFLICT` or 403 `TENANT_DENIED`, NLO does not call `MarkRoutePlanActivationRejected`. It keeps the activation out of `REJECTED` until a tenant-scoped read shows whether a revision for that activation id exists.

C. Replay lookup. Before a create-or-reject decision, resolve the existing revision by operating tenant and activation id. The lookup must keep the internal-token check, the network-optimizer caller check, and the tenant predicate. A soft-deleted carrier must not hide a revision that is already stored. The current `requireCarrierTenant` call runs before that lookup (`transport_execution_repository.go` lines 142–146).

| Criterion | A. Immutable snapshot | B. Reconciliation state | C. Replay lookup |
| --- | --- | --- | --- |
| Idempotency | The resent body matches the first digest, so TMS replay returns the stored revision. | Does not by itself make two bodies identical. It stops a mismatched body from being recorded as `REJECTED`. | Same-digest replay already exists inside `Project`. C makes that path reachable when the carrier row is no longer active. |
| Tenant isolation | The snapshot is stored and loaded with the activation row, which is already keyed by `actor.TenantID`. | The reconcile read must use the caller tenant and the activation id. A cross-tenant id stays not found. | The existing lookup is `operating_tenant_id` plus `source_activation_id` (`transport_execution_repository.go` lines 224–229). |
| Lost acknowledgement | The retry sends the committed body. | The uncertain result stays unresolved until the read, including a crash after the TMS commit. | An identical body can replay. A rebuilt body still conflicts. |
| Concurrent activation | Both callers send the body committed with the one activation row. | Two different in-flight bodies can still race. B only stops the loser from forcing `REJECTED` without a read. | Two different bodies still produce one commit and one 409. |
| Soft-delete carrier | An identical retry still reaches `requireCarrierTenant` first and can return 403. | 403 does not become `REJECTED` until the read shows the revision. | The lookup sees the stored revision before the carrier check. A different body remains 409. |
| Crash recovery | After restart the pending row still has the original command. | Restart does not invent `REJECTED`. The next attempt reads TMS before a terminal mark. | Restart can replay only when the lookup runs first and the body matches. |
| Digest consistency | The stored command is the digest. A live driver, carrier, or vehicle change is ignored on retry. | The live rebuild can still change the digest. B does not hide that conflict. | C does not freeze the command. |
| TMS compatibility | The existing projection route accepts the same body. No TMS write-contract change. | NLO needs a read of the stored revision. That read is not an NLO client today. The write route stays as it is. | Reordering the carrier check and the replay lookup is a shipment-service change. The public command shape stays as it is. |
| Migrations | A later migration must store the snapshot or its digest inputs. This document does not add it. | A new persisted state is a later migration. Reusing `PENDING_EXECUTION` until the read finishes does not require one. | No new table is required for the reorder. This document does not change shipment-service. |
| Tests | Resend after a driver change and assert the stored command, not the live trip. | 409 and 403 leave the row unrejected until the read. | Soft-deleted carrier plus the original body returns the stored revision. A different body still returns 409. |
| Implementation risk | A snapshot taken before TMS commit can preserve a command TMS later rejects. The snapshot must be the body that was sent, and a rejected first attempt must not be retried as success. | A reconcile state without a read leaves the activation pending forever. | Doing C without A still rejects a changed driver. Doing A without C still 403s after soft-delete. |

`RECOMMENDED_OPTION=A_PLUS_B_PLUS_C`. A alone still 403s after a soft-delete. B alone still rebuilds a different command. C alone still 409s when the live driver or carrier changed. The three together cover the lost acknowledgement, the concurrent rebuild, and the soft-delete. This recommendation is not a controller decision and does not authorize the migration, the new read, or the shipment-service reorder. Chain execution stays false.

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
| Pending retry digest drift | `PRODUCTION_BLOCKER`. Lost acknowledgement, then a changed driver or carrier, can leave TMS active and NLO `REJECTED`. A live vehicle change does not enter the retried command. Soft-delete of the carrier is the 403 path. |
| Staging and production | Not changed and not verified here. |

### 4.3 Failure modes

| Failure | Current result | Safe continuation | Forbidden action |
| --- | --- | --- | --- |
| TMS timeout or 5xx before a commit | Temporary error, activation stays `PENDING_EXECUTION`, HTTP 200 | A later activate calls `projectionCommand` again | Do not treat that retry as the same digest. Do not insert a second activation row. |
| TMS timeout, acknowledgement decode error, or 5xx after TMS has committed | NLO still sees a temporary error and stays `PENDING_EXECUTION` | The next activate rebuilds carrier and driver. It does not copy a new live vehicle | Do not call the retry safe. A changed carrier or driver becomes the 409 row below. |
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

No test in this table was run in this stage. `PASS` is not claimed.

| Future test | Expected observation | Status |
| --- | --- | --- |
| Lost acknowledgement after a successful TMS commit | NLO stays `PENDING_EXECUTION` while the TMS revision exists | `NOT_RUN` |
| Driver change between attempts | The retried command carries the new driver and the digest differs | `NOT_RUN` |
| Carrier change between attempts | The retried command carries the new carrier and the digest differs | `NOT_RUN` |
| Vehicle change on the live trip | The retried command keeps `graph.Plan.VehicleID` | `NOT_RUN` |
| Soft-delete of the carrier after the TMS commit | 403 from `requireCarrierTenant` before digest replay | `NOT_RUN` |
| Concurrent activate | Two live commands for one pending row; the loser can mark `REJECTED` | `NOT_RUN` |
| Digest mismatch | TMS 409 `ACTIVATION_BODY_CONFLICT` | `NOT_RUN` |
| TMS execution still present when NLO is `REJECTED` | The revision remains after the 409 or 403 rejection | `NOT_RUN` |
| Crash and retry | A process restart after the TMS commit follows the pending rebuild | `NOT_RUN` |
| Cross-tenant replay denial | A different operating tenant does not read or replay the stored revision | `NOT_RUN` |
| Same activation id and same digest | TMS replay, `Created=false` | `NOT_RUN` |
| Carrier company in another tenant | `TENANT_DENIED` | `NOT_RUN` |
| Producer `fresh` versus the `"FRESH"` gate | The gate does not treat producer `fresh` as fresh | `NOT_RUN` |
| Chain response after any of the above | `execution_supported=false` | `NOT_RUN` |

## 8. Findings from Agent J

These rows record the review of `8d7e9f9c`. They are not closed. Agent J has not confirmed this revision.

| Finding ID | Severity | Evidence | Proposed correction | Review status |
| --- | --- | --- | --- | --- |
| `AP-NLO10F-J-001` | MEDIUM | A pending retry rebuilds `CarrierCompanyID` and `DriverID`. `CanonicalDigest` includes both. After a lost acknowledgement, 409 or a later 403 marks NLO `REJECTED` while TMS keeps the execution. Section 4.3 previously called the retry safe. | Sections 2.2 and 4.3 now give the nine-step sequence, the soft-delete path, and `PRODUCTION_BLOCKER`. Section 3 compares A, B, and C. The recommendation `A_PLUS_B_PLUS_C` is not approved. | `DOC_REMEDIATED_PENDING_J` |
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
PENDING_RETRY_DIGEST_DRIFT=PRODUCTION_BLOCKER
CONTROLLER_APPROVAL_REQUIRED=YES
MERGE_PERFORMED=NO
```
