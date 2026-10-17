# Platform cross-tenant execution identity 0.1A discovery

```text
AGENT=A
STAGE=PLATFORM-CROSS-TENANT-EXECUTION-IDENTITY-0.1A-DISCOVERY
CONTINUATION_STAGE=PLATFORM-CROSS-TENANT-EXECUTION-IDENTITY-0.1A-DISCOVERY-CONTINUATION
BASE_SHA=655556b3d1ebf02fda8241a5a652012d5abc0a2e
MAIN_MIGRATION_HEAD=000099
KIND=DOCS_ONLY_DISCOVERY
PRODUCT_CODE_CHANGED=NO
MIGRATION_CREATED=NO
OPENAPI_CHANGED=NO
GATEWAY_CHANGED=NO
STAGING_CHANGED=NO
IMPLEMENTATION_AUTHORIZED=NO
CURRENT_FINDING_ACCEPTED=YES
BRIDGE_PRESENT=NO
LEGAL_ENTITY_GLOBAL_ID_PRESENT=NO
CANONICAL_CROSS_TENANT_COMPANY_BRIDGE_PRESENT=NO
IDENTITY_BRIDGE_DECISION=HYBRID
CANONICAL_EXECUTION_CARRIER_OWNER=OPERATING_TENANT_COMPANY
SHIPMENT_CARRIER_REFERENCE_MODEL=TENANT_LOCAL_COMPANY_REFERENCE
CARRIER_BRIDGE_AUTHORITY=SERVER_OWNED_EXPLICIT_RELATIONSHIP
CARRIER_BRIDGE_CREATED_BY=SERVER_COMMAND_AFTER_OPERATING_CARRIER_ACCEPTANCE
CARRIER_BRIDGE_VERIFIED_BY=OPERATING_TENANT_MEMBERSHIP
CARRIER_BRIDGE_LIFECYCLE_OWNER=COMPANY_SERVICE
BRIDGE_RESOLUTION_OWNER_SERVICE=SHIPMENT_SERVICE
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
RFX_TO_SHIPMENT_CARRIER_PROVENANCE=SAME_TENANT_UUID_COPY_NO_CROSS_TENANT_BRIDGE
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
A_BLOCKER_CARRIER_GLOBAL_IDENTITY=CLOSED
A_BLOCKER_SHIPMENT_TO_OPERATING_CARRIER_BRIDGE=CLOSED
A_BLOCKER_DRIVER_IDENTITY_BOUNDARY=CLOSED
A_BLOCKER_VEHICLE_IDENTITY_BOUNDARY=CLOSED
A_BLOCKER_RFX_TO_SHIPMENT_PROVENANCE=CLOSED
A_BLOCKER_TENANT_AUTHORIZATION=CLOSED
A_BLOCKER_MIGRATION_STRATEGY=CLOSED
READY_FOR_CROSS_TENANT_IDENTITY_RUNTIME_DESIGN=YES
READY_FOR_NLO_0_9D_STATUS_INTEGRATION=NO
READY_FOR_NLO_0_9D_RUNTIME=NO
```

Evidence is the tree at `655556b3d1ebf02fda8241a5a652012d5abc0a2e`. Migration head is `000099_edo_pod_delivery_action_evidence_binding_v0_1b`. The accepted finding stands: no cross-tenant carrier, driver, or vehicle bridge exists. This continuation selects the future contract. It does not create schema, change product code, or authorize runtime.

The normative contract is [PLATFORM_CROSS_TENANT_EXECUTION_IDENTITY_0_1A_FREEZE.md](PLATFORM_CROSS_TENANT_EXECUTION_IDENTITY_0_1A_FREEZE.md). The TMS status freeze in [TMS_NLO_CHAIN_MULTISHIPMENT_STATUS_POLICY_0_1A_FREEZE.md](../tms/TMS_NLO_CHAIN_MULTISHIPMENT_STATUS_POLICY_0_1A_FREEZE.md) remains the status policy. NLO-0.9D status integration stays unauthorized.

## 1. Accepted current identity

`core.companies.id` is a global primary key with one `tenant_id`. `ShipmentRepository.CompanyExists` and `TransportExecutionRepository.requireCarrierTenant` both require `id` and `tenant_id`. One company row cannot satisfy a shipper tenant and a different operating tenant.

`transport.drivers` and `transport.vehicles` each have one `tenant_id`. `AssignDriver` and `AssignVehicle` load those rows with `GetByIDAndTenant` in the shipment tenant. `transport.transport_executions.driver_id` and `vehicle_id` are nullable, have no foreign key to those tables, and `insertExecution` writes them from the projection command. `ValidateNewProjection` does not compare them with shipment columns.

`tax_id`, `registration_number`, legal name, `license_number`, `plate_number`, `users.email`, `company_memberships`, RFX participant company id, and participant `shipment_tenant_id` are not a cross-tenant authority. `LEGAL_ENTITY_GLOBAL_ID_PRESENT=NO`.

## 2. Option matrix

### A. GLOBAL_PARTY

A new legal-entity root would sit above `core.companies`, and each tenant company would become a representation of that party.

```text
TENANT_ISOLATION=WEAKENED
RFx_IMPACT=PARTICIPANT_AND_AWARD_IDS_CHANGE_MEANING
SHIPMENT_CREATION_IMPACT=COMPANY_EXISTS_PREDICATE_CHANGES
CARRIER_PORTAL_IMPACT=GLOBAL_MEMBERSHIP_REQUIRED
DRIVER_APP_IMPACT=GLOBAL_PERSON_TEMPTATION
TMS_IMPACT=EXECUTION_CARRIER_LEAVES_TENANT_COMPANY
POD_IMPACT=OWNER_COMPANY_MEANING_CHANGES
FINANCE_IMPACT=HISTORICAL_COMPANY_ID_EQUALITY_BREAKS
MIGRATION_COMPLEXITY=HIGH
SECURITY_RISK=HIGH
RECOMMENDATION=REJECTED
```

The schema has no party root. Every company, RFX, shipment, contract, snapshot, and settlement lookup pairs a company id with a tenant. Settlement refuses a shipment whose `carrier_company_id` differs from the snapshot or award-link carrier in that same tenant. A global root is a new identity space. It is not required to name two existing company rows.

### B. EXPLICIT_COMPANY_RELATIONSHIP

Companies stay tenant-local. A server-owned mapping connects the shipper-local carrier reference to the operating carrier company.

```text
TENANT_ISOLATION=PRESERVED
RFx_IMPACT=PARTICIPANT_ID_UNCHANGED
SHIPMENT_CREATION_IMPACT=LOCAL_REFERENCE_UNCHANGED
CARRIER_PORTAL_IMPACT=ACCEPT_LINK_ONLY
DRIVER_APP_IMPACT=UNSTATED
TMS_IMPACT=RESOLVE_OPERATING_COMPANY_BEFORE_PROJECT
POD_IMPACT=COMPATIBLE_WITH_OPERATING_OWNER_COMPANY
FINANCE_IMPACT=HISTORICAL_IDS_UNCHANGED
MIGRATION_COMPLEXITY=MEDIUM
SECURITY_RISK=MEDIUM_UNTIL_CLIENT_ASSERTION_IS_REFUSED
RECOMMENDATION=CARRIER_HALF_ONLY
```

This matches the company evidence. It does not decide driver or vehicle assignment.

### C. EXECUTION_OWNED_ASSIGNMENT_NO_DRIVER_VEHICLE_BRIDGE

`transport_executions.driver_id` and `vehicle_id` are the operational assignment. Shipment driver and vehicle ids are tenant-local and are not required to equal the execution assignment.

```text
TENANT_ISOLATION=PRESERVED
RFx_IMPACT=NONE
SHIPMENT_CREATION_IMPACT=NONE
CARRIER_PORTAL_IMPACT=ASSIGNS_INSIDE_OPERATING_TENANT
DRIVER_APP_IMPACT=RESOLVES_EXECUTION_DRIVER
TMS_IMPACT=NO_CROSS_TENANT_UUID_EQUALITY
POD_IMPACT=ALREADY_USES_EXECUTION_DRIVER
FINANCE_IMPACT=NONE
MIGRATION_COMPLEXITY=LOW
SECURITY_RISK=LOW
RECOMMENDATION=DRIVER_AND_VEHICLE_HALF_ONLY
```

This matches execution storage, driver-stop resolution, and the POD driver rule. It leaves the two carrier companies unconnected.

### D. HYBRID

Carrier identity is the explicit company relationship. Driver and vehicle identity stay on the operating execution. Shipment driver and vehicle UUIDs are not compared with the execution columns.

```text
TENANT_ISOLATION=PRESERVED
RFx_IMPACT=PARTICIPANT_ID_UNCHANGED
SHIPMENT_CREATION_IMPACT=LOCAL_REFERENCE_UNCHANGED
CARRIER_PORTAL_IMPACT=ACCEPT_LINK_AND_ASSIGN_IN_OPERATING_TENANT
DRIVER_APP_IMPACT=RESOLVES_EXECUTION_DRIVER
TMS_IMPACT=RESOLVE_CARRIER_THEN_STORE_OPERATING_ASSIGNMENT
POD_IMPACT=COMPATIBLE
FINANCE_IMPACT=HISTORICAL_IDS_UNCHANGED
MIGRATION_COMPLEXITY=MEDIUM
SECURITY_RISK=LOW_WHEN_RESOLUTION_IS_SERVER_SCOPED
RECOMMENDATION=SELECTED
```

### E. BLOCKED

```text
TENANT_ISOLATION=NOT_APPLICABLE
RFx_IMPACT=NOT_APPLICABLE
SHIPMENT_CREATION_IMPACT=NOT_APPLICABLE
CARRIER_PORTAL_IMPACT=NOT_APPLICABLE
DRIVER_APP_IMPACT=NOT_APPLICABLE
TMS_IMPACT=NOT_APPLICABLE
POD_IMPACT=NOT_APPLICABLE
FINANCE_IMPACT=NOT_APPLICABLE
MIGRATION_COMPLEXITY=NOT_APPLICABLE
SECURITY_RISK=NOT_APPLICABLE
RECOMMENDATION=NOT_REQUIRED
```

The company, driver, vehicle, RFX, POD, and settlement evidence is sufficient to eliminate A and to combine B and C. E is not the result.

## 3. Carrier decision

The question "these two tenant-local company records represent the same operating carrier" is answered by a server-owned relationship between two company rows. It is not answered by one global party id, by UUID equality, or by company name, tax id, registration number, email, phone, or external reference.

```text
CANONICAL_EXECUTION_CARRIER_OWNER=OPERATING_TENANT_COMPANY
SHIPMENT_CARRIER_REFERENCE_MODEL=TENANT_LOCAL_COMPANY_REFERENCE
CARRIER_BRIDGE_AUTHORITY=SERVER_OWNED_EXPLICIT_RELATIONSHIP
CARRIER_BRIDGE_CREATED_BY=SERVER_COMMAND_AFTER_OPERATING_CARRIER_ACCEPTANCE
CARRIER_BRIDGE_VERIFIED_BY=OPERATING_TENANT_MEMBERSHIP
CARRIER_BRIDGE_LIFECYCLE_OWNER=COMPANY_SERVICE
```

`requireCarrierTenant` already requires the execution carrier company to exist in `operating_tenant_id`. `CreateFromTransportOrder` already requires `shipment.carrier_company_id` to exist in the shipment tenant. The future relationship names that pair. The shipper company id remains the shipment reference. The operating company id is what execution and POD store.

Future contract shape, with no table in this stage:

```text
relationship_id=server_generated
shipper_tenant_id
shipper_carrier_company_id
operating_tenant_id
operating_carrier_company_id
status=ACTIVE|REVOKED
```

Both company ids are proved with the existing `id AND tenant_id` predicate before insert. One `ACTIVE` row is allowed for `(shipper_tenant_id, shipper_carrier_company_id, operating_tenant_id)`. A second operating company for that triple is ambiguous and inserts nothing. Attribute equality inserts nothing. A client-supplied pair inserts nothing.

Lifecycle storage belongs with company identity because both ends are `core.companies` rows. Execution projection does not become the owner of the relationship.

## 4. Driver boundary

A shipper can send a shipment through carrier `C` without a directory of `C`'s drivers. The operating tenant assigns `transport_executions.driver_id`. Driver stop resolution and POD authority already use that execution column with `operating_tenant_id`. `GetByUserIDAndTenant` stays inside one tenant. A cross-tenant driver map would be an enumeration path.

```text
CANONICAL_EXECUTION_DRIVER_OWNER=OPERATING_EXECUTION
SHIPMENT_DRIVER_ROLE=TENANT_LOCAL_ASSIGNMENT
CROSS_TENANT_DRIVER_BRIDGE_REQUIRED=NO
SHIPMENT_DRIVER_EQUALITY_REQUIRED_FOR_EXECUTION=NO
DIRECT_DRIVER_UUID_EQUALITY_VALID=NO
```

`ValidateExecutionAssignmentReadiness` remains a shipment-local rule: `DRIVER_ASSIGNED` to `PICKUP_SLOT_BOOKED` requires `shipment.driver_id` and `shipment.vehicle_id` to be non-null. That function does not read `transport_executions`. The shipment FSM stays as it is. Cross-tenant execution activation must not treat those non-null shipment ids as `execution.driver_id`. Readiness for the operating assignment is the execution row in the operating tenant. `PICKUP_SLOT_BOOKED` stays `BLOCKED_BY_IDENTITY_MODEL` as proof of the operating driver, which is the TMS freeze rule.

A global driver or person id is not introduced. `users.email` is unique per tenant. `drivers.user_id` has no foreign key to `core.users`.

## 5. Vehicle boundary

The same boundary applies to vehicles. Plate uniqueness is `(tenant_id, plate_number)`. Assign-vehicle proves the vehicle in the shipment tenant. The execution column is a separate nullable id. A shipper tenant does not receive the carrier fleet.

```text
CANONICAL_EXECUTION_VEHICLE_OWNER=OPERATING_EXECUTION
SHIPMENT_VEHICLE_ROLE=TENANT_LOCAL_ASSIGNMENT
CROSS_TENANT_VEHICLE_BRIDGE_REQUIRED=NO
SHIPMENT_VEHICLE_EQUALITY_REQUIRED_FOR_EXECUTION=NO
DIRECT_VEHICLE_UUID_EQUALITY_VALID=NO
```

Shipment-local vehicle presence for `PICKUP_SLOT_BOOKED` stays inside the shipment tenant. It is not compared with `transport_executions.vehicle_id`.

## 6. RFX to shipment provenance

```text
RFX_CARRIER_IDENTITY_SOURCE=RFX_TENANT_PARTICIPANT_COMPANY_ID
AWARD_CARRIER_IDENTITY_SOURCE=RFX_TENANT_RESPONSE_PARTICIPANT_COMPANY_ID
TRANSPORT_ORDER_CARRIER_IDENTITY_SOURCE=SAME_TENANT_SNAPSHOT_AND_AWARD_LINK
SHIPMENT_CARRIER_IDENTITY_SOURCE=SHIPMENT_TENANT_COMPANY_ID
EXECUTION_CARRIER_IDENTITY_SOURCE=OPERATING_TENANT_COMPANY_ID
RFX_TO_SHIPMENT_CARRIER_PROVENANCE=SAME_TENANT_UUID_COPY_NO_CROSS_TENANT_BRIDGE
```

`AddParticipant` sets `tenant_id` from the actor and calls `CompanyExists(company_id, tenant_id)` before insert. The participant row stores that `company_id`.

`ValidateAwardConversionResponse` requires `response.participant_company_id` to equal `award.carrier_company_id`. `CreateAwardTransactional` inserts that carrier id on `rfx.rfx_awards` in the event tenant and marks the participant with the same `company_id` and `tenant_id`.

`ConvertAwardToTransportOrders` passes `award.CarrierCompanyID` and `event.TenantID` to `CreateFromAwardScope`. `ResolveCarrierID` keeps that pricing-context carrier id. The transport order table has no `carrier_company_id` column. The id is stored on `transport.transport_order_rate_snapshots` and on `rfx.rfx_award_transport_orders`, both with the RFX tenant id.

`CreateFromTransportOrder` proves `CompanyExists(carrier_company_id, shipment_tenant)` and writes that caller-supplied id onto the shipment. `CreateFromBid` copies `bid.carrier_company_id` from the same tenant. `LoadShipmentContext` then requires the shipment carrier id to equal the snapshot or award-link carrier id in that tenant.

`projectionCommand` in `execution_link.go` copies `actor.TenantID` into `OperatingTenantID` and `ShipmentTenantID`, and copies the current-trip shipment carrier id onto the execution. That is a same-tenant copy. The TMS freeze already marks that copy invalid for a depot-start chain whose owner tenant differs from the operating tenant.

Current behavior:

```text
COPIES_OPERATING_CARRIER_COMPANY_INTO_SHIPPER_TENANT=NO
EXPECTS_PREEXISTING_SHIPPER_LOCAL_CARRIER_COMPANY=YES
CROSS_TENANT_CARRIER_PROVENANCE=ABSENT
DIFFERENT_COMPANY_IDS_WITHOUT_EXPLICIT_BRIDGE=YES
```

Inside one tenant the same company UUID is copied from participant to response, award, snapshot, award link, and shipment. When the operating tenant differs, that UUID is only the shipper-local reference. The operating company id is a different row, and nothing records the pair.

## 7. Authorization contract

```text
CLIENT_SUPPLIED_BRIDGE_AUTHORITY=NO
SHIPPER_CAN_ENUMERATE_CARRIER_DRIVERS=NO
SHIPPER_CAN_ENUMERATE_CARRIER_VEHICLES=NO
CARRIER_CAN_MUTATE_SHIPMENT_OWNER_IDENTITY=NO
CROSS_SHIPPER_PARTICIPATION_EXISTENCE_LEAK=NO
BRIDGE_RESOLUTION_OWNER_SERVICE=SHIPMENT_SERVICE
BRIDGE_READ_AUTHORIZATION_SCOPE=VERIFIED_SHIPMENT_OR_EXECUTION_COMMAND
BRIDGE_WRITE_AUTHORIZATION_SCOPE=SERVER_COMMAND_BOTH_COMPANIES_PROVEN
```

A read is allowed only inside a server command that already holds a verified shipment in its owner tenant or a verified execution in its operating tenant. The read returns the operating carrier company id for that pair, or not-found. It does not return drivers, vehicles, or other shipments.

A write is allowed only after both companies have been proved in their own tenants and the operating carrier membership has accepted the link. The acceptance is a server-recorded membership check. A request body that names the foreign company is not acceptance. The carrier command does not write `shipments.tenant_id`, shipper company, consignee company, or `shipment.carrier_company_id`.

Resolution for one shipment does not report whether another shipper tenant has a relationship.

## 8. POD and finance

The merged POD contract pins `document_tenant_id` to `execution.operating_tenant_id`. `owner_company_id` is the execution carrier company and must exist in that document tenant. `shipment_tenant_id` stays the shipment owner. Driver authority is `execution.driver_id` for the operating tenant. `shipments.driver_id` is not action authority.

```text
POD_IDENTITY_MODEL_COMPATIBLE=YES
```

HYBRID uses that same operating company as the execution carrier and does not move shipment ownership. POD does not need a driver or vehicle bridge.

Finance reads `shipment.carrier_company_id` in the shipment tenant. For `SNAPSHOT_V1` it loads `transport.transport_order_rate_snapshots.carrier_company_id` in that tenant. Otherwise it loads `rfx.rfx_award_transport_orders.carrier_company_id` in that tenant. The two ids must be equal or settlement is refused. `billing.freight_settlements.carrier_company_id` and billing-register item carrier ids are those tenant-local ids. Contract rates also prove buyer and carrier with `CompanyExistsInTenant`.

```text
FINANCE_IDENTITY_IMPACT=HISTORICAL_TENANT_LOCAL_COMPANY_ID_UNCHANGED
FINANCE_RUNTIME_CHANGE_AUTHORIZED=NO
```

A later mapping may be referenced by new finance work. It does not replace historical settlement, register, snapshot, or award-link company ids.

## 9. Migration classes

No migration is created.

| Class | Future treatment |
| --- | --- |
| Existing company records | Remain tenant-local. No party root is backfilled |
| Existing shipments | Keep `carrier_company_id`, `driver_id`, and `vehicle_id` as stored |
| Existing awards | Keep the RFX-tenant carrier id on the award, award link, and snapshot |
| Existing executions | Keep `operating_tenant_id`, `carrier_company_id`, `driver_id`, and `vehicle_id` as stored |
| Existing drivers | Remain shipment-tenant or operating-tenant rows. No cross-tenant link is added |
| Existing vehicles | Same as drivers |
| Historical billing and document rows | Keep the company id and document tenant already stored |

```text
HISTORICAL_AUTO_MATCH_ALLOWED=NO
UNMAPPED_EXISTING_RECORD_POLICY=LEAVE_UNMAPPED_FAIL_CLOSED
AMBIGUOUS_MATCH_POLICY=REFUSE_NO_ROW
ROLLBACK_BOUNDARY=FUTURE_RELATIONSHIP_ROW_ONLY
```

An unmapped historical shipment stays unmapped. A later cross-tenant activation of that shipment fails closed. An ambiguous attribute match creates no row. Rollback of a future relationship insert removes or revokes that relationship only. It does not rewrite companies, shipments, awards, executions, drivers, vehicles, billing rows, or documents.

## 10. Decision

```text
IDENTITY_BRIDGE_DECISION=HYBRID
```

A is rejected because the repository identity is the tenant-local company row and finance equality depends on that id. B alone leaves driver and vehicle equality unresolved. C alone leaves the two carrier companies unresolved. The evidence supports both halves together: an explicit carrier relationship, execution-owned driver and vehicle assignment, and no direct shipment-to-execution driver or vehicle UUID equality.

```text
READY_FOR_CROSS_TENANT_IDENTITY_RUNTIME_DESIGN=YES
IMPLEMENTATION_AUTHORIZED=NO
READY_FOR_NLO_0_9D_STATUS_INTEGRATION=NO
READY_FOR_NLO_0_9D_RUNTIME=NO
```

`READY_FOR_CROSS_TENANT_IDENTITY_RUNTIME_DESIGN=YES` means a later controller may authorize a design of this contract. This discovery does not authorize that implementation. The TMS carrier, driver, vehicle, and cross-shipper runtime blockers stay open until that implementation exists.

## 11. Failure and security matrix

Every row is `FUTURE_REQUIRED`. None of these checks exist as a bridge runtime in this tree.

| Id | Condition | Required result |
| --- | --- | --- |
| XTI_01 | Shipper-tenant company UUID is submitted as the operating carrier when the tenants differ | Refuse. No execution |
| XTI_02 | Client body supplies the relationship or the foreign company as authority | Refuse. No row |
| XTI_03 | Name, tax id, registration number, email, phone, or external reference is the match key | No row |
| XTI_04 | Shipper asks for the operating carrier driver directory | No directory |
| XTI_05 | Shipper asks for the operating carrier fleet | No fleet |
| XTI_06 | Carrier command writes shipment owner tenant or owner company | Refuse. Shipment identity unchanged |
| XTI_07 | Resolution for one shipment reports another shipper shipment | No foreign existence |
| XTI_08 | Shipper-local carrier company is missing in its tenant | Not-found. No row |
| XTI_09 | Operating carrier company is missing in its tenant | Not-found. No row |
| XTI_10 | A second ACTIVE operating company is proposed for the same shipper reference and operating tenant | Refuse. No second row |
| XTI_11 | Cross-tenant activation has no ACTIVE relationship | Fail closed. No execution |
| XTI_12 | Shipment and execution driver UUIDs are compared across tenants | Comparison is not identity |
| XTI_13 | Shipment and execution vehicle UUIDs are compared across tenants | Comparison is not identity |
| XTI_14 | POD document tenant is set to the shipment tenant | Refuse. Document tenant stays the operating tenant |
| XTI_15 | POD owner company is the shipper-local carrier company | Refuse. Owner stays the operating carrier company |
| XTI_16 | Settlement rewrites a historical carrier company id to another tenant's company | Refuse. Historical id unchanged |
| XTI_17 | A job creates relationships by attribute similarity | No row |
| XTI_18 | Resolution runs without a verified shipment or execution command | Refuse |
| XTI_19 | Two operating companies match one shipper reference | Ambiguous. No row |
| XTI_20 | Relationship is revoked after award | Award, snapshot, and settlement ids stay. Later activation fails closed |
| XTI_21 | Operating assignment changes after activation | Committed execution assignment stays on that revision. Shipment columns are not rewritten to match |
| XTI_22 | Shipment-local driver id differs from execution driver id | Allowed. Not a conflict |
| XTI_23 | Shipment-local vehicle id differs from execution vehicle id | Allowed. Not a conflict |
| XTI_24 | Historical shipment has no relationship | Stays unmapped. Cross-tenant activation fails closed |
| XTI_25 | Relationship store is unavailable | Error. No guessed company |
