# TMS multi-shipment depot-start status policy discovery

```text
AGENT=C
STAGE=TMS-NLO-CHAIN-MULTISHIPMENT-STATUS-POLICY-0.1A-DISCOVERY
BASE_SHA=e114d990aae8c5d37394da6f4e07df47125f2f53
DOCS_ONLY=YES
IMPLEMENTATION_AUTHORIZED=NO
DEPOT_START_ONLY=YES
CURRENT_TRIP_EXECUTION_INCLUDED=NO
```

Evidence is the tree at `e114d990aae8c5d37394da6f4e07df47125f2f53`. The closed contract is `TMS_NLO_CHAIN_MULTISHIPMENT_STATUS_POLICY_0_1A_FREEZE.md`. NLO-0.9D-A already limits execution materialization to `DEPOT_START` and leaves shipment status eligibility to TMS. This note records the current rules that decision uses. It does not change the shipment FSM or the NLO planner.

## 1. Shipment status

`transport.shipments.status` allows `CREATED`, the assignment statuses, `PICKUP_SLOT_BOOKED`, `DELIVERY_SLOT_BOOKED`, the movement statuses from `IN_PICKUP` through `FINANCIALLY_CLOSED`, and `CANCELLED` (`000003_create_transport_tables.up.sql`).

`allowedStatusTransitions` in `services/shipment-service/internal/domain/shipment.go` is the writer path:

- `CARRIER_ASSIGNED` to `ACCEPTED_BY_CARRIER`
- `ACCEPTED_BY_CARRIER` to `VEHICLE_ASSIGNED` or `DRIVER_ASSIGNED`
- `VEHICLE_ASSIGNED` to `DRIVER_ASSIGNED`
- `DRIVER_ASSIGNED` to `PICKUP_SLOT_BOOKED`
- `PICKUP_SLOT_BOOKED` to `IN_PICKUP`
- then `LOADED`, `IN_TRANSIT`, `ARRIVED_AT_CONSIGNEE`, `UNLOADING`, `DELIVERED`, `DELIVERY_CONFIRMED`, `DOCUMENTS_COMPLETED`, `READY_FOR_BILLING`, `INCLUDED_IN_BILLING_REGISTER`, `FINANCIALLY_CLOSED`

`DELIVERY_SLOT_BOOKED` is a stored value and is counted as an occupied vehicle in `otherActiveVehicleShipmentsQuery`. It is not a target of `allowedStatusTransitions` or `manualStatusTransitions`. `TestNLO04CActivationStatusSets` rejects it for current-trip activation. `DepotStartActivationAllowed` also returns false for it because the true set is only the five statuses above. `CREATED` is the old column default. Domain create and transport-order shipment creation use `CARRIER_ASSIGNED`.

Assign-driver is allowed from `CARRIER_ASSIGNED`, `ACCEPTED_BY_CARRIER`, or `VEHICLE_ASSIGNED`. Assign-vehicle is allowed from `CARRIER_ASSIGNED`, `ACCEPTED_BY_CARRIER`, or `DRIVER_ASSIGNED`. Accept is allowed only from `CARRIER_ASSIGNED`. Each of those writes increments `version`. Generic status updates also increment `version`. Cancel is refused from `DELIVERED` onward and is otherwise a status change, not a delete.

`ValidateExecutionAssignmentReadiness` refuses `DRIVER_ASSIGNED` to `PICKUP_SLOT_BOOKED` when `driver_id` or `vehicle_id` is null. `BuildExecutionReadiness` treats `CARRIER_ASSIGNED` as not carrier-accepted and treats `ReadyToStart` as driver and vehicle present while status is still `DRIVER_ASSIGNED`.

## 2. What NLO checks today

`DepotStartActivationAllowed` returns true only for `CARRIER_ASSIGNED`, `ACCEPTED_BY_CARRIER`, `VEHICLE_ASSIGNED`, `DRIVER_ASSIGNED`, and `PICKUP_SLOT_BOOKED`. `CurrentTripActivationAllowed` returns true only for `IN_PICKUP`, `LOADED`, and `IN_TRANSIT`. `DELIVERY_SLOT_BOOKED` is explicitly excluded by `activation_policy_test.go`.

`AcceptRoutePlan` calls `ensureRoutePlanFresh` and then marks the plan accepted. It does not call `ActivationStatusAllowed` and it does not update a shipment.

`ActivateRoutePlan` calls `ensureRoutePlanFresh` again and then `ActivationStatusAllowed`. For `DEPOT_START`, freshness checks the capacity pin and returns an empty status string. `ActivationStatusAllowed("DEPOT_START", "")` is false. `TestNLO04CDepotStartActivationFailsClosedWithoutShipmentStatus` proves that refusal writes no activation row. The helper is therefore a single-status predicate with no depot-start shipment to read. It is not yet a multi-shipment rule.

`projectionCommand` requires `graph.Plan.ShipmentID` and builds one current-trip command. It copies `actor.TenantID` into `ContextShipmentTenantID` and every `ShipmentTenantID`. NLO-0.9D-A already marks that copy invalid. Depot-start projection is not implemented.

## 3. Canonical shipment and cargo

A published load stores `owner_tenant_id`, `source_type` (`SHIPMENT` or `TRANSPORT_ORDER`), and `source_id`. It does not store shipment version, cargo id, or cargo version.

For `SHIPMENT`, `source_id` is the shipment id. Execution context for that id under `owner_tenant_id` returns `tenant_id`, `shipment_version`, and nullable `cargo_id`. Cargo version comes from the cargo planning profile in that same owner tenant. A null `cargo_id` cannot be invented from load weight.

For `TRANSPORT_ORDER`, `source_id` is the order id. `uq_shipments_transport_order_active` allows one non-deleted shipment per `(tenant_id, transport_order_id)`. The index does not exclude `CANCELLED`. `GetShipmentByTransportOrderID` reads that row in the owner tenant and returns not found when it is absent. A cancelled row is still the canonical shipment and then fails the status rule. Publishing a load does not call `POST /from-transport-order`.

## 4. Execution shape

`transport.transport_executions` has `operating_tenant_id`, required `carrier_company_id`, and nullable `driver_id` and `vehicle_id`. It has no `shipment_id`.

`transport.transport_execution_participants` stores each subject's `shipment_tenant_id`, `shipment_id`, `shipment_version`, `cargo_id`, and `cargo_version`.

`transport.transport_execution_active_shipments` is unique on `(shipment_tenant_id, shipment_id)`. A second active slot for the same shipment conflicts.

`ValidateNewProjection` requires a carrier company. Depot-start must not carry a context shipment. Every action needs a shipment id, owner tenant, shipment version, cargo id, and cargo version, and that tuple must match an execution subject. Depot-start with actions and zero subjects is `EXECUTION_SUBJECT_UNMATERIALIZED`. Two cargo rows for one shipment may carry only one shipment version.

`Project` verifies every subject, then inserts the execution, revision, participants, and active slots in one transaction. `verifyMaterializedRows` reads `transport.shipments` by `id` and `tenant_id` and `transport.cargoes` by `id` and that same tenant. A version mismatch is `PLAN_STALE`. A missing row is `EXECUTION_SUBJECT_UNMATERIALIZED`. The active-slot primary key is `EXECUTION_PLAN_CONFLICT`. Any error rolls the transaction back.

TMS integration coverage keeps two shipper tenants on one operating carrier and does not rewrite `shipment_tenant_id` to the operating tenant.

## 5. Decision

The five pre-`IN_PICKUP` statuses that the domain actually writes, and that the existing depot helper already names, are the eligible set. `IN_PICKUP` and later belong to movement, which current-trip activation already treats as a different window. `CANCELLED` and `DELIVERY_SLOT_BOOKED` are not in that pre-movement writer path. Statuses do not have to be identical across shipments. Carrier, driver, and vehicle identity are separate from status because those columns are nullable and one execution has one carrier, one driver, and one vehicle. The check belongs on activation, which already re-reads freshness, and not on accept, which does not read shipment status.
