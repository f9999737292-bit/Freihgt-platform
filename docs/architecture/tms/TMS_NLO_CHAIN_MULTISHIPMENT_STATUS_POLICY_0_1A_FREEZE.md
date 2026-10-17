# TMS multi-shipment depot-start status policy freeze

```text
FREEZE_SCOPE=PARTIAL
STATUS_POLICY_FROZEN=YES
CARRIER_IDENTITY_ELIGIBILITY=UNRESOLVED
DRIVER_IDENTITY_ELIGIBILITY=UNRESOLVED
VEHICLE_IDENTITY_ELIGIBILITY=UNRESOLVED
AGENT=C
STAGE=TMS-NLO-CHAIN-MULTISHIPMENT-STATUS-POLICY-0.1A-FREEZE
BASE_SHA=e114d990aae8c5d37394da6f4e07df47125f2f53
IMPLEMENTATION_AUTHORIZED=NO
READY_FOR_NLO_0_9D_RUNTIME=NO
READY_FOR_NLO_0_9D_STATUS_INTEGRATION=NO
DEPOT_START_ONLY=YES
CURRENT_TRIP_EXECUTION_INCLUDED=NO
NLO_0_9D_CURRENT_TRIP_EXECUTION_POLICY=DEPOT_START_ONLY
ALL_LOADS_REQUIRE_CANONICAL_SHIPMENT=YES
TRANSPORT_ORDER_WITHOUT_SHIPMENT_POLICY=REFUSE_NO_SHIPMENT_CREATED
TRANSPORT_ORDER_NO_SHIPMENT_GATE_PHASE=EXECUTION_GRADE_MATERIALIZATION_AND_ACTIVATION_REVALIDATION
DEPOT_START_ALLOWED_STATUSES=CARRIER_ASSIGNED,ACCEPTED_BY_CARRIER,VEHICLE_ASSIGNED,DRIVER_ASSIGNED,PICKUP_SLOT_BOOKED
ALL_SHIPMENTS_MUST_BE_ELIGIBLE=YES
MIXED_ELIGIBLE_STATUSES_ALLOWED=YES
ONE_INELIGIBLE_BLOCKS_WHOLE_ACTIVATION=YES
CANCELLED_ALWAYS_BLOCKS=YES
IN_PICKUP_OR_LATER_BLOCKS_DEPOT_START=YES
DELIVERY_SLOT_BOOKED_POLICY=INELIGIBLE_BLOCKS
CROSS_TENANT_CARRIER_IDENTITY_SOURCE=NONE
CROSS_TENANT_DRIVER_IDENTITY_SOURCE=NONE
CROSS_TENANT_VEHICLE_IDENTITY_SOURCE=NONE
DIRECT_CARRIER_COMPANY_UUID_EQUALITY_VALID=NO
DIRECT_DRIVER_UUID_EQUALITY_VALID=NO
DIRECT_VEHICLE_UUID_EQUALITY_VALID=NO
CARRIER_COMPANY_MATCH_REQUIRED=UNPROVEN
DRIVER_ASSIGNMENT_CONSISTENCY_POLICY=UNRESOLVED
VEHICLE_ASSIGNMENT_CONSISTENCY_POLICY=UNRESOLVED
CROSS_SHIPPER_PICKUP_SLOT_BOOKED_POLICY=BLOCKED_BY_IDENTITY_MODEL
STATUS_CHECK_PHASE=ACTIVATE
ACTIVATION_REVALIDATES_STATUS=YES
ACTIVATION_REVALIDATES_SHIPMENT_VERSION=YES
PARTIAL_EXECUTION_ALLOWED=NO
SHIPMENT_STATUS_MUTATED_BY_NLO_ACCEPT=NO
SHIPMENT_STATUS_MUTATED_BY_NLO_ACTIVATE=NO
CROSS_SHIPPER_OWNER_TENANT_PRESERVED=YES
OPERATING_TENANT_USED_AS_SHIPMENT_TENANT=NO
C_BLOCKER_CANONICAL_SHIPMENT_REQUIREMENT=CLOSED
C_BLOCKER_DEPOT_STATUS_MATRIX=CLOSED
C_BLOCKER_CARRIER_IDENTITY=OPEN
C_BLOCKER_DRIVER_ASSIGNMENT=OPEN
C_BLOCKER_VEHICLE_ASSIGNMENT=OPEN
C_BLOCKER_CROSS_SHIPPER_TENANCY=OPEN
C_BLOCKER_ATOMIC_MULTI_SUBJECT_GATE=CLOSED
```

This is a partial freeze. The status set, the canonical-shipment requirement, the activation check phase, and the all-or-nothing projection gate are closed. Carrier, driver, and vehicle identity across a shipper tenant and the operating carrier tenant are not closed. NLO-0.9D status integration is not ready, and runtime is not authorized. `CURRENT_TRIP` execution stays out of scope.

`DepotStartActivationAllowed` remains a single-status helper. Depot-start activation today returns no shipment status, so the helper cannot see a chain. The status rule below replaces that helper for status only.

## Canonical shipment

```text
ALL_LOADS_REQUIRE_CANONICAL_SHIPMENT=YES
TRANSPORT_ORDER_WITHOUT_SHIPMENT_POLICY=REFUSE_NO_SHIPMENT_CREATED
TRANSPORT_ORDER_NO_SHIPMENT_GATE_PHASE=EXECUTION_GRADE_MATERIALIZATION_AND_ACTIVATION_REVALIDATION
```

Every load in the selected depot-start chain must resolve to one canonical shipment and one cargo before that chain can be an execution-grade route plan. NLO does not create a shipment.

`SOURCE_TYPE=SHIPMENT`: `source_id` is the shipment id. The read uses `load.owner_tenant_id`. Execution context supplies `tenant_id`, `shipment_version`, and `cargo_id`. The cargo planning profile in that owner tenant supplies `cargo_version`. A null `cargo_id` is not an execution subject.

`SOURCE_TYPE=TRANSPORT_ORDER`: a canonical shipment must already exist. The predicate is `uq_shipments_transport_order_active`: one non-deleted `transport.shipments` row for `(owner_tenant_id, transport_order_id)`, read as `GetShipmentByTransportOrderID`. The index does not exclude `CANCELLED`. No row means the load is not an execution subject. A cancelled row resolves and then fails the status rule. NLO does not call `POST /from-transport-order`. After the shipment id is found, cargo resolution is the shipment-source procedure.

The missing-shipment refusal is the NLO-0.9D-A execution-grade materialization gate: the lookup is stored on the score-time bundle before `Plan` returns, and a load without that shipment is not pinned as an execution subject. Activation re-reads the pin and refuses when the live shipment or cargo differs. The re-read does not rewrite the stored pin, and it is not the first time the missing shipment is discovered. An executable plan is not created with an unmaterialized subject and then repaired at projection time.

## Status

Status eligibility is separate from carrier, driver, and vehicle identity. An eligible status does not close the identity blockers.

Eligible depot-start statuses are only:

- `CARRIER_ASSIGNED`
- `ACCEPTED_BY_CARRIER`
- `VEHICLE_ASSIGNED`
- `DRIVER_ASSIGNED`
- `PICKUP_SLOT_BOOKED`

These are the statuses the domain writer can reach after carrier assignment and before `IN_PICKUP`. `PICKUP_SLOT_BOOKED` is status-eligible because its only transition is into `IN_PICKUP`, and current-trip activation already owns `IN_PICKUP`, `LOADED`, and `IN_TRANSIT`. Booking a pickup slot is not movement. `ValidateExecutionAssignmentReadiness` requires `shipment.driver_id` and `shipment.vehicle_id` before that status. Those ids are shipment-tenant resources. Their presence does not identify the operating execution's driver or vehicle.

Not eligible:

| Status | Result |
| --- | --- |
| `CREATED` | Blocks. It is not a domain-created execution status |
| `DELIVERY_SLOT_BOOKED` | Blocks. The column allows it. The transition map does not produce it, and it is not a pre-pickup status |
| `IN_PICKUP`, `LOADED`, `IN_TRANSIT`, `ARRIVED_AT_CONSIGNEE`, `UNLOADING` | Blocks. Movement has started |
| `DELIVERED`, `DELIVERY_CONFIRMED`, `DOCUMENTS_COMPLETED`, `READY_FOR_BILLING`, `INCLUDED_IN_BILLING_REGISTER`, `FINANCIALLY_CLOSED` | Blocks. The shipment is past execution start |
| `CANCELLED` | Blocks |

Every canonical shipment in the chain must be status-eligible. Statuses do not have to match each other. `ACCEPTED_BY_CARRIER` together with `DRIVER_ASSIGNED` is status-valid. One status-ineligible shipment refuses the whole activation. That refusal does not create a partial execution.

## Identity

```text
CROSS_TENANT_CARRIER_IDENTITY_SOURCE=NONE
CROSS_TENANT_DRIVER_IDENTITY_SOURCE=NONE
CROSS_TENANT_VEHICLE_IDENTITY_SOURCE=NONE
DIRECT_CARRIER_COMPANY_UUID_EQUALITY_VALID=NO
DIRECT_DRIVER_UUID_EQUALITY_VALID=NO
DIRECT_VEHICLE_UUID_EQUALITY_VALID=NO
CROSS_SHIPPER_PICKUP_SLOT_BOOKED_POLICY=BLOCKED_BY_IDENTITY_MODEL
```

`core.companies.id` is globally unique and each row has one `tenant_id`. Shipment creation proves `CompanyExists(carrier_company_id, shipment_tenant_id)`. Execution acceptance proves `requireCarrierTenant(carrier_company_id, operating_tenant_id)`. When the shipment owner tenant differs from the operating tenant, those two checks cannot be satisfied by one company row. Direct equality of `shipment.carrier_company_id` and `transport_execution.carrier_company_id` is not a proven cross-tenant identity.

`AssignDriver` and `AssignVehicle` prove `GetByIDAndTenant` against the shipment tenant. `transport.drivers.tenant_id` and `transport.vehicles.tenant_id` are single-tenant columns. Execution `driver_id` and `vehicle_id` are separate nullable columns. Direct UUID equality is not a proven relationship between those pairs.

No accepted contract makes `tax_id`, `registration_number`, company name, `drivers.user_id`, or an external reference the cross-tenant authority. This freeze does not invent that mapping. `MATCH_OR_BOTH_ABSENT` is withdrawn. It treated tenant-local UUIDs as one identity space.

Owner-tenant preservation remains proven and is unchanged: each participant keeps its canonical shipment owner, and the operating tenant is not written as that owner. Two participant tenants on one execution are a storage shape. They do not make the carrier, driver, or vehicle the same resource.

`PICKUP_SLOT_BOOKED` on shipments `S1` and `S2`, with operating tenant `C`, `S1 != C`, and `S2 != C`, is `BLOCKED_BY_IDENTITY_MODEL`. Each shipment must already hold a shipment-tenant driver id and vehicle id. The execution driver and vehicle, when present, are operating-tenant values. No bridge maps one side to the other, so this case is not supported.

Activation does not write `shipment.carrier_company_id`, `shipment.driver_id`, or `shipment.vehicle_id`. A shipment that already occupies `transport_execution_active_shipments` still refuses the whole activation with the existing execution-plan conflict.

`C_BLOCKER_CROSS_SHIPPER_TENANCY` stays open because owner-tenant preservation does not close cross-shipper execution identity. `C_BLOCKER_ATOMIC_MULTI_SUBJECT_GATE` stays closed: `Project` still verifies every materialized subject and rolls the transaction back on any error. That gate cannot be used until the open identity blockers have an accepted bridge.

## Phase

```text
STATUS_CHECK_PHASE=ACTIVATE
ACTIVATION_REVALIDATES_STATUS=YES
ACTIVATION_REVALIDATES_SHIPMENT_VERSION=YES
```

Accept freezes the route plan. It does not read this status matrix and it does not change a shipment. Activation re-reads every canonical shipment under its owner tenant immediately before the TMS projection transaction commits. A live shipment version or cargo version that differs from the score-time pin is stale. The pin is not refreshed. A status that is no longer eligible refuses activation. The stored score provenance stays as it was.

The projection transaction verifies every subject first. It then inserts one execution, its participants, and its active shipment slots, or it rolls all of them back. One failed subject does not leave an execution for the others and does not drop the failed load from the plan.

A refusal writes no execution. A later retry reads the live rows again. It does not replay a success that was not committed.

A missing or wrong-tenant shipment is the same unmaterialized-subject refusal. The read is `id` plus `tenant_id` of that shipment's owner. The operating tenant is not used as the shipment tenant, and the refusal does not describe a foreign shipment.

```text
SHIPMENT_STATUS_MUTATED_BY_NLO_ACCEPT=NO
SHIPMENT_STATUS_MUTATED_BY_NLO_ACTIVATE=NO
```

NLO does not advance `CARRIER_ASSIGNED`, `ACCEPTED_BY_CARRIER`, `VEHICLE_ASSIGNED`, `DRIVER_ASSIGNED`, `PICKUP_SLOT_BOOKED`, or any later status. Shipment, carrier, and driver workflows remain the status writers.

## Failure matrix

Status rows are the closed part of this partial freeze. Identity rows are unresolved and block integration. This stage does not add the runtime.

| Id | Condition | Required result |
| --- | --- | --- |
| CHAIN_STATUS_01 | Every shipment status is independently eligible | Status gate passes. Identity remains unresolved, so this is not an integration approval |
| CHAIN_STATUS_02 | One shipment is outside the eligible status set | Whole activation refused. No execution |
| CHAIN_STATUS_03 | `ACCEPTED_BY_CARRIER` and `DRIVER_ASSIGNED`, both status-eligible | Statuses need not be equal. Identity remains unresolved |
| CHAIN_STATUS_04 | One shipment is `CANCELLED` | Whole activation refused |
| CHAIN_STATUS_05 | One shipment is `IN_PICKUP` or later | Whole activation refused |
| CHAIN_STATUS_06 | Shipment and execution carrier company UUIDs differ across tenants | Not a valid comparison. `C_BLOCKER_CARRIER_IDENTITY` stays open |
| CHAIN_STATUS_07 | A shipment `carrier_company_id` is null | Not closed by UUID equality. Carrier identity stays open |
| CHAIN_STATUS_08 | Shipment and execution driver UUIDs differ, or one side is null | Not a valid cross-tenant comparison. Driver identity stays open |
| CHAIN_STATUS_09 | Shipment and execution vehicle UUIDs differ, or one side is null | Not a valid cross-tenant comparison. Vehicle identity stays open |
| CHAIN_STATUS_10 | A `SHIPMENT` load has no canonical shipment or no cargo | Not an execution subject. No shipment is created |
| CHAIN_STATUS_11 | A `TRANSPORT_ORDER` load has no canonical shipment | Not an execution subject at materialization, and activation revalidates. No shipment is created |
| CHAIN_STATUS_12 | The shipment read uses a tenant other than the owner | Same unmaterialized refusal. No foreign-existence detail |
| CHAIN_STATUS_13 | Live shipment or cargo version differs from the score pin | Stale refusal. The pin is not rewritten |
| CHAIN_STATUS_14 | Status changes after accept and is no longer eligible | Activation refuses. Accept is not replayed as success |
| CHAIN_STATUS_15 | Two shipper tenants and one operating carrier | Owner tenants stay on the participants. Execution identity is `BLOCKED_BY_IDENTITY_MODEL` |
| CHAIN_STATUS_16 | Retry after a status refusal | No execution exists from the refusal. The retry reads live state |
| CHAIN_STATUS_17 | Any one subject fails a closed check | No partial execution and no dropped load |
| CHAIN_STATUS_18 | Projection commits | Participant `shipment_tenant_id` values remain the owner tenants |
| CHAIN_STATUS_19 | One shipment already has an active execution slot | Whole activation refused. No second slot |
| CHAIN_STATUS_20 | One shipment is `DELIVERY_SLOT_BOOKED` | Whole activation refused |
| CHAIN_STATUS_21 | `S1` and `S2` are `PICKUP_SLOT_BOOKED`, both differ from operating tenant `C` | `BLOCKED_BY_IDENTITY_MODEL`. Shipment-tenant driver and vehicle ids are required, and no bridge maps them to the execution |
