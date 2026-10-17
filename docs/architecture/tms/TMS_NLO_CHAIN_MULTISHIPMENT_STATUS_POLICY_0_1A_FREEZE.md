# TMS multi-shipment depot-start status policy freeze

```text
AGENT=C
STAGE=TMS-NLO-CHAIN-MULTISHIPMENT-STATUS-POLICY-0.1A-FREEZE
BASE_SHA=e114d990aae8c5d37394da6f4e07df47125f2f53
IMPLEMENTATION_AUTHORIZED=NO
READY_FOR_NLO_0_9D_RUNTIME=NO
READY_FOR_NLO_0_9D_STATUS_INTEGRATION=YES
DEPOT_START_ONLY=YES
CURRENT_TRIP_EXECUTION_INCLUDED=NO
NLO_0_9D_CURRENT_TRIP_EXECUTION_POLICY=DEPOT_START_ONLY
ALL_LOADS_REQUIRE_CANONICAL_SHIPMENT=YES
TRANSPORT_ORDER_WITHOUT_SHIPMENT_POLICY=REFUSE_ACTIVATION_NO_SHIPMENT_CREATED
DEPOT_START_ALLOWED_STATUSES=CARRIER_ASSIGNED,ACCEPTED_BY_CARRIER,VEHICLE_ASSIGNED,DRIVER_ASSIGNED,PICKUP_SLOT_BOOKED
ALL_SHIPMENTS_MUST_BE_ELIGIBLE=YES
MIXED_ELIGIBLE_STATUSES_ALLOWED=YES
ONE_INELIGIBLE_BLOCKS_WHOLE_ACTIVATION=YES
CANCELLED_ALWAYS_BLOCKS=YES
IN_PICKUP_OR_LATER_BLOCKS_DEPOT_START=YES
DELIVERY_SLOT_BOOKED_POLICY=INELIGIBLE_BLOCKS
CARRIER_COMPANY_MATCH_REQUIRED=YES
NULL_CARRIER_COMPANY_POLICY=REFUSE_ACTIVATION
MULTIPLE_CARRIER_COMPANIES_POLICY=REFUSE_WHOLE_ACTIVATION
DRIVER_ASSIGNMENT_CONSISTENCY_POLICY=MATCH_OR_BOTH_ABSENT
VEHICLE_ASSIGNMENT_CONSISTENCY_POLICY=MATCH_OR_BOTH_ABSENT
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
C_BLOCKER_CARRIER_IDENTITY=CLOSED
C_BLOCKER_DRIVER_ASSIGNMENT=CLOSED
C_BLOCKER_VEHICLE_ASSIGNMENT=CLOSED
C_BLOCKER_CROSS_SHIPPER_TENANCY=CLOSED
C_BLOCKER_ATOMIC_MULTI_SUBJECT_GATE=CLOSED
```

TMS owns this policy. A later NLO runtime consumes it. This document does not implement that runtime, does not change the shipment FSM, and does not authorize `CURRENT_TRIP` execution.

`DepotStartActivationAllowed` remains a single-status helper. Depot-start activation today returns no shipment status, so the helper cannot see a chain. The rule below replaces that helper for multi-shipment depot-start activation.

## Canonical shipment

```text
ALL_LOADS_REQUIRE_CANONICAL_SHIPMENT=YES
```

Every load in the selected depot-start chain must resolve to one canonical shipment and one cargo before activation may project. NLO does not create a shipment.

`SOURCE_TYPE=SHIPMENT`: `source_id` is the shipment id. The read uses `load.owner_tenant_id`. Execution context supplies `tenant_id`, `shipment_version`, and `cargo_id`. The cargo planning profile in that owner tenant supplies `cargo_version`. A null `cargo_id` is not an execution subject.

`SOURCE_TYPE=TRANSPORT_ORDER`: a canonical shipment must already exist. The predicate is `uq_shipments_transport_order_active`: one non-deleted `transport.shipments` row for `(owner_tenant_id, transport_order_id)`, read as `GetShipmentByTransportOrderID`. The index does not exclude `CANCELLED`. No row means activation refuses. A cancelled row resolves and then fails the status rule. NLO does not call `POST /from-transport-order`. After the shipment id is found, cargo resolution is the shipment-source procedure.

The shipment id, owner tenant, shipment version, cargo id, and cargo version used for the score-time pin stay the pin. Activation re-reads the live row and refuses when it differs. The re-read does not rewrite the stored pin.

## Status

Eligible depot-start statuses are only:

- `CARRIER_ASSIGNED`
- `ACCEPTED_BY_CARRIER`
- `VEHICLE_ASSIGNED`
- `DRIVER_ASSIGNED`
- `PICKUP_SLOT_BOOKED`

These are the statuses the domain writer can reach after carrier assignment and before `IN_PICKUP`. `PICKUP_SLOT_BOOKED` stays eligible because its only transition is into `IN_PICKUP`, and current-trip activation already owns `IN_PICKUP`, `LOADED`, and `IN_TRANSIT`. Booking a pickup slot is not movement.

Not eligible:

| Status | Result |
| --- | --- |
| `CREATED` | Blocks. It is not a domain-created execution status |
| `DELIVERY_SLOT_BOOKED` | Blocks. The column allows it. The transition map does not produce it, and it is not a pre-pickup status |
| `IN_PICKUP`, `LOADED`, `IN_TRANSIT`, `ARRIVED_AT_CONSIGNEE`, `UNLOADING` | Blocks. Movement has started |
| `DELIVERED`, `DELIVERY_CONFIRMED`, `DOCUMENTS_COMPLETED`, `READY_FOR_BILLING`, `INCLUDED_IN_BILLING_REGISTER`, `FINANCIALLY_CLOSED` | Blocks. The shipment is past execution start |
| `CANCELLED` | Blocks |

Every canonical shipment in the chain must be eligible. Statuses do not have to match each other. `ACCEPTED_BY_CARRIER` together with `DRIVER_ASSIGNED` is valid when both are eligible and the identity rules below pass. One ineligible shipment refuses the whole activation.

## Identity

The execution has one required `carrier_company_id` and one nullable `driver_id` and `vehicle_id`. Those execution values are the carrier caller's operating execution, not a shipment tenant.

```text
CARRIER_COMPANY_MATCH_REQUIRED=YES
```

Every shipment `carrier_company_id` must equal `transport_execution.carrier_company_id`. Null on any shipment refuses activation. Two different carrier companies refuse activation. The carrier tenant and the shipper tenant are different fields. Two shipper tenants may share one operating carrier.

```text
DRIVER_ASSIGNMENT_CONSISTENCY_POLICY=MATCH_OR_BOTH_ABSENT
VEHICLE_ASSIGNMENT_CONSISTENCY_POLICY=MATCH_OR_BOTH_ABSENT
```

For driver, and separately for vehicle:

- both null is allowed
- equal non-null ids are allowed
- unequal ids refuse the whole activation
- one null and one non-null refuses the whole activation

Status does not override the id check. A shipment before `DRIVER_ASSIGNED` or `VEHICLE_ASSIGNED` may already hold an id, and a status name may not match the columns. Activation does not write `shipment.driver_id`, `shipment.vehicle_id`, or `shipment.carrier_company_id`.

A shipment that already occupies `transport_execution_active_shipments` refuses the whole activation with the existing execution-plan conflict. No second active slot is created.

## Phase

```text
STATUS_CHECK_PHASE=ACTIVATE
ACTIVATION_REVALIDATES_STATUS=YES
ACTIVATION_REVALIDATES_SHIPMENT_VERSION=YES
```

Accept freezes the route plan. It does not read this status matrix and it does not change a shipment. Activation re-reads every canonical shipment under its owner tenant immediately before the TMS projection transaction commits. A live shipment version or cargo version that differs from the score-time pin is stale. The pin is not refreshed. A status, carrier, driver, or vehicle that fails this policy refuses activation. The stored score provenance stays as it was.

The projection transaction verifies every subject first. It then inserts one execution, its participants, and its active shipment slots, or it rolls all of them back. One failed subject does not leave an execution for the others and does not drop the failed load from the plan.

A refusal writes no execution. A later retry reads the live rows again. It does not replay a success that was not committed.

A missing or wrong-tenant shipment is the same unmaterialized-subject refusal. The read is `id` plus `tenant_id` of that shipment's owner. The operating tenant is not used as the shipment tenant, and the refusal does not describe a foreign shipment.

```text
SHIPMENT_STATUS_MUTATED_BY_NLO_ACCEPT=NO
SHIPMENT_STATUS_MUTATED_BY_NLO_ACTIVATE=NO
```

NLO does not advance `CARRIER_ASSIGNED`, `ACCEPTED_BY_CARRIER`, `VEHICLE_ASSIGNED`, `DRIVER_ASSIGNED`, `PICKUP_SLOT_BOOKED`, or any later status. Shipment, carrier, and driver workflows remain the status writers.

## Failure matrix

All rows are `FUTURE_REQUIRED` for the NLO status integration. This stage does not add the runtime.

| Id | Condition | Required result |
| --- | --- | --- |
| CHAIN_STATUS_01 | Every shipment is independently eligible and identity matches | Activation may proceed to projection |
| CHAIN_STATUS_02 | One shipment is outside the eligible set | Whole activation refused. No execution |
| CHAIN_STATUS_03 | `ACCEPTED_BY_CARRIER` and `DRIVER_ASSIGNED`, both otherwise valid | Allowed. Statuses need not be equal |
| CHAIN_STATUS_04 | One shipment is `CANCELLED` | Whole activation refused |
| CHAIN_STATUS_05 | One shipment is `IN_PICKUP` or later | Whole activation refused |
| CHAIN_STATUS_06 | A shipment carrier differs from the execution carrier | Whole activation refused |
| CHAIN_STATUS_07 | A shipment `carrier_company_id` is null | Whole activation refused |
| CHAIN_STATUS_08 | Driver ids differ, or exactly one side is null | Whole activation refused. Shipment driver is not written |
| CHAIN_STATUS_09 | Vehicle ids differ, or exactly one side is null | Whole activation refused. Shipment vehicle is not written |
| CHAIN_STATUS_10 | A `SHIPMENT` load has no canonical shipment or no cargo | Whole activation refused. No shipment is created |
| CHAIN_STATUS_11 | A `TRANSPORT_ORDER` load has no live shipment | Whole activation refused. No shipment is created |
| CHAIN_STATUS_12 | The shipment read uses a tenant other than the owner | Same unmaterialized refusal. No foreign-existence detail |
| CHAIN_STATUS_13 | Live shipment or cargo version differs from the score pin | Stale refusal. The pin is not rewritten |
| CHAIN_STATUS_14 | Status changes after accept and is no longer eligible | Activation refuses. Accept is not replayed as success |
| CHAIN_STATUS_15 | Two shipper tenants, one operating carrier, one carrier company | Allowed when every other rule passes. Owner tenants stay on the participants |
| CHAIN_STATUS_16 | Retry after a status or identity refusal | No execution exists from the refusal. The retry reads live state |
| CHAIN_STATUS_17 | Any one subject fails | No partial execution and no dropped load |
| CHAIN_STATUS_18 | Projection commits | Participant `shipment_tenant_id` values remain the owner tenants |
| CHAIN_STATUS_19 | One shipment already has an active execution slot | Whole activation refused. No second slot |
| CHAIN_STATUS_20 | One shipment is `DELIVERY_SLOT_BOOKED` | Whole activation refused |
| CHAIN_STATUS_21 | Both driver ids are null, or both vehicle ids are null | That field is consistent. The other rules still apply |
