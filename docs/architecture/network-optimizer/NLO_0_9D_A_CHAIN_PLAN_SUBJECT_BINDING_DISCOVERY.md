# NLO-0.9D-A chain plan and execution-subject binding

```text
AGENT=D
STAGE=NLO-0.9D-A-CHAIN-PLAN-SUBJECT-BINDING-DISCOVERY
BASE_SHA=028bdba38d8e925969fca96063d8a553fde8b1ea
PRODUCT_CODE_CHANGED=NO
MIGRATION_CREATED=NO
OPENAPI_CHANGED=NO
GATEWAY_CHANGED=NO
STAGING_CHANGED=NO
RUNTIME_IMPLEMENTATION_AUTHORIZED=NO
MULTI_LOAD_STATUS_POLICY=OUT_OF_SCOPE
CHAIN_ACCEPT_ACTIVATE_RUNTIME=OUT_OF_SCOPE
```

Evidence is the tree at `028bdba38d8e925969fca96063d8a553fde8b1ea`. The closed contract is `NLO_0_9D_A_CHAIN_PLAN_SUBJECT_BINDING_FREEZE.md`. This note records why that contract is determined by current code.

## 1. Baseline

```text
CHAIN_RUNTIME_PRESENT=YES
CHAIN_CANDIDATE_ROUTE_PLAN_BINDING=NULL_NOT_PINNED
CHAIN_EXECUTION_SUPPORTED=false
NLO_0_9D_DISCOVERY_IN_MAIN=YES
```

`chain_search_runs.execution_supported` is checked false in migration `000097`. `chainapi.projectSearch` forces the public flag false. `chainorch.project` sets `RoutePlanID: nil`. `NLO_0_9D_CHAIN_EXECUTION_ACCEPTANCE_DISCOVERY.md` is on this main.

## 2. How a feasible candidate is computed

`chain.Engine.planOrder` calls `routeplan.Planner.Plan` with `PreserveInsertionOrder: true` and exactly two loads, in the order under test. `scoreOrder` keeps metrics from `routeplan.Outcome` and then drops the outcome. `ProjectedCandidate` has load ids, versions, selected order, and metrics. It does not retain stops, legs, snapshots, or provider fingerprints.

`PersistCandidatesUnderOwner` inserts those rows in one `WithinChainSearch` transaction. There is no `UPDATE` of `chain_candidates` anywhere in the repository.

Generic `EvaluateRoutePlan` builds a graph with `buildRoutePlanGraph`, which sets `Version: 1` and `ExecutionSupported: false`. That path chooses its own load order unless the caller already forced it. It also calls the routing provider again. A second provider call is a new leg provenance. It is not the plan that was scored.

Re-evaluating only the selected order has the same provider problem. The stored candidate does not contain the response fingerprints needed to prove the new graph is the scored graph.

The only artifact that is the scored plan is the `routeplan.Outcome` still in memory inside that search compute, before projection discards it.

## 3. Route-plan version

`buildRoutePlanGraph` writes `version = 1`. The only `UPDATE` statements on `network_optimizer.route_plans` are:

- `MarkRoutePlanAccepted`: `status` and `accepted_at`
- `MarkRoutePlanSuperseded`: `status` and `superseded_at`

Neither statement changes `version`. No repository statement updates stops, actions, legs, snapshots, or dependencies after insert. A successor is a different plan id (`supersedes_plan_id` on the new row), not a new version of the same id.

`route_plans.version` therefore identifies the insert snapshot. The id is enough to load that snapshot. Accept already compares the stored version with the caller's version and does not increment it. A candidate must not accept a client-supplied version. It reads `plan.version` from the bound row.

Activation copies that integer into `route_plan_activations.plan_version` at insert. That copy is the activation pin. The candidate does not need a second column for the current writer.

The schema allows `version >= 1` and does not have a trigger that forbids a future update. No current writer performs one. The contract for this stage is the current writer, not a speculative version bump.

## 4. Load source types

`load_opportunities.source_type` is `TRANSPORT_ORDER` or `SHIPMENT` (migration `000075`). The row stores `owner_tenant_id`, `source_type`, and `source_id`. It does not store shipment version, cargo id, or cargo version.

`sourceverify.HTTP.Owns` proves publish-time ownership:

- `SHIPMENT`: `GET /internal/v1/shipments/{source_id}/ownership` must return that id and the same tenant.
- `TRANSPORT_ORDER`: `GET /internal/v1/transport-orders/{source_id}/ownership` must return that id and the same tenant.

The tenant on that call is the load owner, not the carrier viewing the chain.

### Shipment-backed load

`source_id` is the shipment id. `GET /internal/v1/shipments/{id}/execution-context` with `X-Tenant-ID` equal to `owner_tenant_id` returns `tenant_id`, `shipment_version`, and nullable `cargo_id`. The handler loads the shipment only in that tenant.

`cargo_id` references `transport.cargoes`. `GET /internal/v1/cargoes/{id}/planning-profile` on transport-order-service loads `transport.cargoes` with `id` and `tenant_id` and returns `version`. NLO already calls this from `sourceclient.HTTP.CargoPlanningProfile` and rejects a payload whose `tenant_id` is not the requested tenant.

There is no cargo id on the public load. Cargo id comes only from the execution-context of that shipment. A null `cargo_id` cannot be filled from load weight or cargo attributes.

### Transport-order-backed load

`source_id` is the transport order id. A shipment is a later row. `POST /from-transport-order` on shipment-service creates it. Publishing a load does not.

`uq_shipments_transport_order_active` allows one live shipment per `(tenant_id, transport_order_id)`. `GetShipmentByTransportOrderID` reads that row and returns not found when it is absent. That function is in-process on shipment-service. NLO has no HTTP client for it.

A missing shipment is not created by chain materialization.

## 5. Route action identity

`RouteActionRow` has `subject_type`, `subject_id`, `subject_version`, `source_shipment_id`, and `source_shipment_version`. It has no `source_shipment_tenant_id`, no `cargo_id`, and no `cargo_version`.

`actionFromLoad` sets `subject_type=LOAD_OPPORTUNITY` and `subject_id` to the load id. It does not set `ShipmentID`. `buildRoutePlanGraph` copies `action.ShipmentID` into `source_shipment_id` only when the planner action already has it. Current-trip onboard cargo uses `subject_type=SHIPMENT_CARGO` and `subject_id` equal to the cargo id, plus the seed shipment id and version. That path still does not store the shipment tenant.

`projectionCommand` materializes a subject only for `SHIPMENT_CARGO` with a source shipment id. It sets every `ShipmentTenantID` and `ContextShipmentTenantID` to `actor.TenantID`. TMS `verifyMaterializedRows` then requires `transport.shipments.tenant_id` and `transport.cargoes.tenant_id` to equal that value. A foreign owner fails closed. The stamp is not a successful cross-shipper projection.

TMS itself stores `operating_tenant_id` on the execution and `shipment_tenant_id` on each participant. `projection_integration_test.go` keeps two shipper tenants and does not rewrite them to the operating tenant.

## 6. Controller remediation R1

`chain.Engine.planOrder` calls `Planner.Plan` and `scoreOrder` then discards the outcome. `toPlanLoad` turns a nil weight into `0` and omits the cargo facts `cargoFromLoad` copies. `chainapi.API.groupage` uses empty equipment and does not accumulate catalog or rule refs from inside `Plan`. `planOrder` does not pass the current-trip end stop, onboard actions, or `Input.Onboard`.

The freeze requires the future execution-grade graph to pin those facts from the scoring invocation. It limits 0.9D execution materialization to `DEPOT_START`. A current-trip candidate is not given `route_plan_id`.

Migration `000098` arrived from `main` at `6f7ba7e0fe65beefde8989cf64105387e51730bd`. This remediation does not edit it.

## 7. What stays open

Shipment status eligibility is not decided here. Chain accept and activate routes are not added. `execution_supported` stays false. Current-trip chain materialization stays blocked.
