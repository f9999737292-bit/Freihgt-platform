# NLO-1.0F execution and production readiness

```text
AGENT=D
STAGE=NLO-1.0F-EXECUTION-READINESS-DISCOVERY
BASE_SHA=a9ef8f40bf0e7f75cb4aee3794e7841723347e4c
PREVIOUS_PR=367
PR367_STATUS=MERGED
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
FINAL_VERDICT=READY_FOR_J_ARCHITECTURE_REVIEW
```

This note records the execution path on `a9ef8f40`. It does not enable a pilot, change NLO-0.9B, or authorize NLO-0.9D. A document is not a running adapter.

## 1. Baseline

`origin/main` is `a9ef8f40bf0e7f75cb4aee3794e7841723347e4c`, the merge of PR #367 at `2026-11-20T22:20:12Z`. Parents are `3f306fc407aeda4bdc652d825954292f4a54fad6` and `dd877e2e73e572216eb30492a08112fd5e57b91d`.

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

Shipment-service stores one revision per `operating_tenant_id` and `source_activation_id`. The same digest replays. A different digest conflicts (`transport_execution_repository.go` lines 201–217, `transport_execution.go` lines 174–186). A first projection that sets `supersedes_route_plan_id` or `supersedes_activation_id` is `ReasonExecutionPlanConflict` (`transport_execution.go` lines 184–186). `projectionCommand` does not set those fields (`execution_link.go` lines 115–132).

A temporary projection error leaves the activation pending and still returns HTTP 200 (`execution_link.go` lines 50–62). A permanent rejection marks the activation `REJECTED`.

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

Tenant controls that exist today, and what they are not:

| Check | Code | Limit |
| --- | --- | --- |
| Route-plan row | `GetRoutePlan(ctx, actor.TenantID, id)` | The caller tenant must own the plan. |
| Projection tenant | Header tenant must equal `operating_tenant_id` (`execution_projection_handler.go` lines 71–74) | A body tenant cannot override the header. |
| Carrier company | `requireCarrierTenant` (`transport_execution_repository.go` lines 161–175) | The company row must belong to that tenant. |
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
| Same-plan reapply | Protected for one activation row. A second plan of the same shipment is not automatically tied to the open execution, because supersedes on the first projection is rejected and the successor route is not called. |
| Staging and production | Not changed and not verified here. |

### 4.3 Failure modes

| Failure | Current result | Safe continuation | Forbidden action |
| --- | --- | --- | --- |
| TMS timeout or 5xx | Temporary error, activation stays `PENDING_EXECUTION`, HTTP 200 | A later activate retries the projection | Do not insert a second activation row. |
| TMS 409 or 400 | Activation becomes `REJECTED` | Stop | Do not retry the same command as a new activation id. |
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
| Tracking and ETA lookup | 1.0E CI proof | Handler contract only. |
| Recalculate after an execution deviation | No NLO consumer and no successor client | Architecture only. |
| Show an NLO recommendation in Control Tower | Control Tower consumes shipment and execution events, not NLO events | Separate read model. Not an NLO pilot surface. |

## 5. Compatibility

| Decision | What this discovery keeps |
| --- | --- |
| NLO-0.9B freeze | `execution_supported=false` through chain search. 0.9D is not authorized. Horizon, budget, and the separate chain resource stay as frozen. |
| NLO-0.9C public search | The implemented search may return candidates. It does not accept, activate, or project. |
| NLO-1.0E | The named handler proof stays the lookup contract. It is not an execution adapter. |
| TMS execution | shipment-service owns the execution. The first projection contract and the successor-revision contract stay as written. |
| Control Tower | Its consumer and projections stay outside NLO. |

## 6. Later implementation tasks

These tasks are not authorized. Each one stops rather than widening into chain execution.

1. Keep the 1.0E handler proof and the current activation idempotency unchanged.
2. Add a contract test that a second `PENDING_EXECUTION` activate retries projection once, and that `EXECUTION_LINKED` and `REJECTED` do not call TMS again. Use the existing fakes. Do not add a migration.
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
- Temporary TMS failure leaving `PENDING_EXECUTION`.
- Permanent TMS rejection leaving `REJECTED` and a second call not projecting.
- Cross-tenant plan id returning not found.
- Carrier company in another tenant returning the tenant denial.
- Producer `fresh` asserted separately from the `"FRESH"` gate, without changing the gate in the same change.
- `execution_supported=false` on the chain response after any of the above.

## 8. Stop

```text
IMPLEMENTATION_AUTHORIZED=NO
EXECUTION_ADAPTER_READY=NO
TMS_HANDOFF_READY=NO
CHAIN_EXECUTION_SUPPORTED=NO
READY_FOR_0_9D_RUNTIME=NO
PRODUCTION_CONSUMER_READINESS=BLOCKED
MERGE_PERFORMED=NO
```
