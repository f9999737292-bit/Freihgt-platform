# Platform cross-tenant execution identity freeze

```text
AGENT=A
STAGE=PLATFORM-CROSS-TENANT-EXECUTION-IDENTITY-0.1A-DISCOVERY
BASE_SHA=655556b3d1ebf02fda8241a5a652012d5abc0a2e
MAIN_MIGRATION_HEAD=000099
IDENTITY_BRIDGE_DECISION=HYBRID
FREEZE_SCOPE=CONTRACT
IMPLEMENTATION_AUTHORIZED=NO
SCHEMA_CREATED=NO
BRIDGE_PRESENT=NO
LEGAL_ENTITY_GLOBAL_ID_PRESENT=NO
CANONICAL_CROSS_TENANT_COMPANY_BRIDGE_PRESENT=NO
CANONICAL_EXECUTION_CARRIER_OWNER=OPERATING_TENANT_COMPANY
SHIPMENT_CARRIER_REFERENCE_MODEL=TENANT_LOCAL_COMPANY_REFERENCE
CARRIER_BRIDGE_AUTHORITY=SERVER_OWNED_EXPLICIT_RELATIONSHIP
CARRIER_BRIDGE_CREATED_BY=SERVER_COMMAND_AFTER_OPERATING_CARRIER_ACCEPTANCE
CARRIER_BRIDGE_VERIFIED_BY=OPERATING_TENANT_MEMBERSHIP
CARRIER_BRIDGE_LIFECYCLE_OWNER=COMPANY_SERVICE
BRIDGE_RESOLUTION_OWNER_SERVICE=SHIPMENT_SERVICE
BRIDGE_READ_AUTHORIZATION_SCOPE=VERIFIED_SHIPMENT_OR_EXECUTION_COMMAND
BRIDGE_WRITE_AUTHORIZATION_SCOPE=SERVER_COMMAND_BOTH_COMPANIES_PROVEN
CANONICAL_EXECUTION_DRIVER_OWNER=OPERATING_EXECUTION
SHIPMENT_DRIVER_ROLE=TENANT_LOCAL_ASSIGNMENT
CROSS_TENANT_DRIVER_BRIDGE_REQUIRED=NO
SHIPMENT_DRIVER_EQUALITY_REQUIRED_FOR_EXECUTION=NO
CANONICAL_EXECUTION_VEHICLE_OWNER=OPERATING_EXECUTION
SHIPMENT_VEHICLE_ROLE=TENANT_LOCAL_ASSIGNMENT
CROSS_TENANT_VEHICLE_BRIDGE_REQUIRED=NO
SHIPMENT_VEHICLE_EQUALITY_REQUIRED_FOR_EXECUTION=NO
DIRECT_CARRIER_COMPANY_UUID_EQUALITY_VALID=NO
DIRECT_DRIVER_UUID_EQUALITY_VALID=NO
DIRECT_VEHICLE_UUID_EQUALITY_VALID=NO
CLIENT_SUPPLIED_BRIDGE_AUTHORITY=NO
SHIPPER_CAN_ENUMERATE_CARRIER_DRIVERS=NO
SHIPPER_CAN_ENUMERATE_CARRIER_VEHICLES=NO
CARRIER_CAN_MUTATE_SHIPMENT_OWNER_IDENTITY=NO
CROSS_SHIPPER_PARTICIPATION_EXISTENCE_LEAK=NO
POD_IDENTITY_MODEL_COMPATIBLE=YES
FINANCE_IDENTITY_IMPACT=HISTORICAL_TENANT_LOCAL_COMPANY_ID_UNCHANGED
FINANCE_RUNTIME_CHANGE_AUTHORIZED=NO
HISTORICAL_AUTO_MATCH_ALLOWED=NO
UNMAPPED_EXISTING_RECORD_POLICY=LEAVE_UNMAPPED_FAIL_CLOSED
AMBIGUOUS_MATCH_POLICY=REFUSE_NO_ROW
ROLLBACK_BOUNDARY=FUTURE_RELATIONSHIP_ROW_ONLY
READY_FOR_CROSS_TENANT_IDENTITY_RUNTIME_DESIGN=YES
READY_FOR_NLO_0_9D_STATUS_INTEGRATION=NO
READY_FOR_NLO_0_9D_RUNTIME=NO
```

This freeze selects the contract. It does not create the relationship, does not change the shipment FSM, and does not authorize NLO-0.9D. Evidence and the option matrix are in [PLATFORM_CROSS_TENANT_EXECUTION_IDENTITY_0_1A_DISCOVERY.md](PLATFORM_CROSS_TENANT_EXECUTION_IDENTITY_0_1A_DISCOVERY.md).

## Carrier

The operating execution carrier is a `core.companies` row in `operating_tenant_id`. The shipment carrier reference is a different `core.companies` row in the shipment tenant. The authority that they represent the same operating carrier is one server-owned relationship:

```text
shipper_tenant_id
shipper_carrier_company_id
operating_tenant_id
operating_carrier_company_id
status=ACTIVE|REVOKED
```

Both companies are proved with `id` and `tenant_id` before the row exists. One `ACTIVE` row is allowed for `(shipper_tenant_id, shipper_carrier_company_id, operating_tenant_id)`. Name, tax id, registration number, email, phone, and external reference are not the key. A client body is not the authority.

Company-service owns the relationship lifecycle. Shipment-service resolves it only inside a verified shipment or execution command. The shipper does not receive the carrier driver directory or fleet. The carrier does not write shipment owner identity. Resolution for one shipment does not reveal another shipper shipment.

## Driver and vehicle

`transport_executions.driver_id` and `vehicle_id` are the operating assignment. `shipment.driver_id` and `shipment.vehicle_id` remain tenant-local shipment assignment. Equality of those UUIDs is not cross-tenant identity. No driver bridge and no vehicle bridge are required.

`ValidateExecutionAssignmentReadiness` stays a shipment-local non-null check on the way to `PICKUP_SLOT_BOOKED`. That check is not proof of the execution assignment. The shipment FSM is unchanged.

## Provenance and history

RFX participant, award, rate snapshot, award link, and shipment carrier id are one tenant-local company UUID copied inside that tenant. That copy is not the operating carrier when the operating tenant differs.

Historical companies, shipments, awards, executions, drivers, vehicles, billing rows, and documents stay as stored. Automatic matching is refused. An unmapped record stays unmapped, and a later cross-tenant activation fails closed. Rollback affects only a future relationship row.

POD remains compatible: document tenant and owner company stay in the operating tenant, and shipment tenant stays the owner. Finance continues to compare tenant-local company ids. Finance runtime change is not authorized.

## Failure matrix

XTI_01 through XTI_25 in the discovery note are `FUTURE_REQUIRED`. They are part of this contract and are not implemented here.
