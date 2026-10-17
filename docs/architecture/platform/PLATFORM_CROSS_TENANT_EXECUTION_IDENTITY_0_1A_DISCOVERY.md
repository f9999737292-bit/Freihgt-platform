# Platform cross-tenant execution identity 0.1A discovery

```text
AGENT=A
STAGE=PLATFORM-CROSS-TENANT-EXECUTION-IDENTITY-0.1A-DISCOVERY
CONTINUATION_STAGE=PLATFORM-CROSS-TENANT-EXECUTION-IDENTITY-0.1A-DISCOVERY-CONTINUATION
REMEDIATION_STAGE=PLATFORM-CROSS-TENANT-EXECUTION-IDENTITY-0.1A-PR280-CONTROLLER-REMEDIATION-R2
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
CARRIER_BRIDGE_CREATION_COMMAND=CreateCarrierOperatingRelationshipRequest
CARRIER_BRIDGE_ACCEPTANCE_COMMAND=AcceptCarrierOperatingRelationshipRequest
SHIPPER_SIDE_IDENTITY_SOURCE=SHIPMENT|RFX_AWARD|AWARD_TRANSPORT_ORDER_LINK
OPERATING_SIDE_IDENTITY_SOURCE=AUTHENTICATED_OPERATING_TENANT_MEMBERSHIP
OPERATING_CARRIER_ACCEPTANCE_EVIDENCE=ACTIVE_SELECTED_COMPANY_MEMBERSHIP+COMPANY_SCOPED_CARRIER_ADMIN_ROLE+VALID_RELATIONSHIP_REQUEST
OPERATING_SELECTED_COMPANY_SOURCE=SERVER_VERIFIED_SELECTED_COMPANY_CONTEXT
OPERATING_ACCEPTANCE_REQUIRED_ROLE=CARRIER_ADMIN|PLATFORM_ADMIN
ORDINARY_ACTIVE_MEMBER_CAN_ACCEPT_RELATIONSHIP=NO
MULTIPLE_MEMBERSHIPS_WITHOUT_SELECTED_COMPANY_POLICY=REFUSE
CLIENT_SUPPLIED_OPERATING_COMPANY_AUTHORITY=NO
CARRIER_BRIDGE_COMMAND_OWNER_SERVICE=COMPANY_SERVICE
CARRIER_BRIDGE_WRITE_SERVICE=COMPANY_SERVICE
CARRIER_BRIDGE_COMMAND_IDEMPOTENCY_SCOPE=relationship_request_id+selected_operating_tenant_id+selected_operating_company_id+idempotency_key
ACCEPTING_USER_IS_AUDIT_PROVENANCE=YES
CARRIER_RELATIONSHIP_REQUEST_OWNER=COMPANY_SERVICE
SHIPPER_SOURCE_VERIFICATION_OWNER=SOURCE_OWNING_SERVICE
RELATIONSHIP_REQUEST_SERVER_GENERATED=YES
RAW_SOURCE_TYPE_ID_ACCEPTANCE_ALLOWED=NO
OPAQUE_REQUEST_TOKEN_REQUIRED=YES
REQUEST_TOKEN_ALONE_IS_AUTHORITY=NO
OPERATING_ADMIN_CONTEXT_REQUIRED=YES
SOURCE_OWNER_AUTHORIZATION_REQUIRED=YES
CLIENT_SUPPLIED_COMPANY_PAIR_AUTHORITY=NO
TWO_SIDED_RELATIONSHIP_ACCEPTANCE_REQUIRED=YES
SHIPPER_SIDE_CONSENT_PINNED_BY=RELATIONSHIP_REQUEST_ID_AND_SOURCE_PROOF
OPERATING_SIDE_CONSENT_PINNED_BY=RELATIONSHIP_REQUEST_ID_AND_ADMIN_CONTEXT
RELATIONSHIP_CREATED_ONLY_AFTER_BOTH_SIDES=YES
SHIPMENT_SOURCE_OWNER=SHIPMENT_SERVICE
RFX_AWARD_SOURCE_OWNER=RFX_SERVICE
AWARD_LINK_SOURCE_OWNER=RFX_SERVICE
COMPANY_SERVICE_DIRECT_TRANSPORT_AUTHORITY=NO
COMPANY_SERVICE_DIRECT_RFX_AUTHORITY=NO
CARRIER_BRIDGE_LIFECYCLE_OWNER=COMPANY_SERVICE
CARRIER_RELATIONSHIP_LIFECYCLE_MODEL=IMMUTABLE_APPEND_ONLY_VERSIONS
CARRIER_RELATIONSHIP_ENDPOINTS_IMMUTABLE=YES
CARRIER_RELATIONSHIP_VERSION_REQUIRED=YES
CARRIER_RELATIONSHIP_AUDIT_REQUIRED=YES
CARRIER_RELATIONSHIP_RESOLUTION_PHASE=ACTIVATE_BEFORE_TMS_PROJECTION
CARRIER_RELATIONSHIP_PIN_OWNER=TRANSPORT_EXECUTION_REVISION
ACTIVATION_REVALIDATES_RELATIONSHIP_ACTIVE=YES
EXECUTION_AUDIT_PRESERVES_RELATIONSHIP_PIN=YES
RELATIONSHIP_REVOKED_BEFORE_ACTIVATE_POLICY=REFUSE_NO_EXECUTION
RELATIONSHIP_REVOKED_AFTER_EXECUTION_POLICY=HISTORICAL_EXECUTION_UNCHANGED
ACTIVE_RELATIONSHIP_UNIQUENESS=ONE_UNSUPERSEDED_ACTIVE_FOR_SHIPPER_CARRIER_AND_OPERATING_TENANT
ZERO_RELATIONSHIP_POLICY=NOT_FOUND_FAIL_CLOSED
MULTIPLE_RELATIONSHIP_POLICY=DATA_INTEGRITY_ERROR_FAIL_CLOSED
OPERATING_DRIVER_AUTHORITY=transport.drivers under operating tenant
OPERATING_VEHICLE_AUTHORITY=transport.vehicles under operating tenant
EXECUTION_DRIVER_SERVER_VERIFIED=YES
EXECUTION_VEHICLE_SERVER_VERIFIED=YES
EXECUTION_DRIVER_MUST_MATCH_OPERATING_CARRIER=YES
EXECUTION_VEHICLE_MUST_MATCH_OPERATING_CARRIER=YES
EXECUTION_DRIVER_NULL_IDENTITY_POLICY=NULL_IS_NO_ASSIGNMENT
EXECUTION_VEHICLE_NULL_IDENTITY_POLICY=NULL_IS_NO_ASSIGNMENT
SHIPMENT_LOCAL_DRIVER_COMPARED_TO_EXECUTION_DRIVER=NO
SHIPMENT_LOCAL_VEHICLE_COMPARED_TO_EXECUTION_VEHICLE=NO
A280_001_CLOSED=YES
A280_002_CLOSED=YES
A280_003_CLOSED=YES
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
ROLLBACK_BOUNDARY=FUTURE_RELATIONSHIP_VERSION_ROW_ONLY
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
CARRIER_BRIDGE_CREATION_COMMAND=CreateCarrierOperatingRelationshipRequest
CARRIER_BRIDGE_ACCEPTANCE_COMMAND=AcceptCarrierOperatingRelationshipRequest
CARRIER_BRIDGE_LIFECYCLE_OWNER=COMPANY_SERVICE
```

`requireCarrierTenant` already requires the execution carrier company to exist in `operating_tenant_id`. `CreateFromTransportOrder` already requires `shipment.carrier_company_id` to exist in the shipment tenant. The future relationship names that pair. The shipper company id remains the shipment reference. The operating company id is what execution and POD store.

The request, the acceptance command, the version row, and the activation pin are defined in section 14. Lifecycle storage belongs with company identity because both ends are `core.companies` rows. Execution projection stores the pin. It does not own the relationship.

## 4. Driver boundary

A shipper can send a shipment through carrier `C` without a directory of `C`'s drivers. The operating tenant assigns `transport_executions.driver_id`. Driver stop resolution and POD authority already use that execution column with `operating_tenant_id`. `GetByUserIDAndTenant` stays inside one tenant. A cross-tenant driver map would be an enumeration path.

```text
CANONICAL_EXECUTION_DRIVER_OWNER=OPERATING_EXECUTION
SHIPMENT_DRIVER_ROLE=TENANT_LOCAL_ASSIGNMENT
CROSS_TENANT_DRIVER_BRIDGE_REQUIRED=NO
SHIPMENT_DRIVER_EQUALITY_REQUIRED_FOR_EXECUTION=NO
DIRECT_DRIVER_UUID_EQUALITY_VALID=NO
```

`ValidateExecutionAssignmentReadiness` remains a shipment-local rule: `DRIVER_ASSIGNED` to `PICKUP_SLOT_BOOKED` requires `shipment.driver_id` and `shipment.vehicle_id` to be non-null. That function does not read `transport_executions`. The shipment FSM stays as it is. Cross-tenant execution activation must not treat those non-null shipment ids as `execution.driver_id`. `PICKUP_SLOT_BOOKED` stays `BLOCKED_BY_IDENTITY_MODEL` as proof of the operating driver, which is the TMS freeze rule.

A non-null `transport_executions.driver_id` is stored only after the server reads `transport.drivers` and finds `id` equal to the requested id, `tenant_id` equal to `operating_tenant_id`, `carrier_company_id` equal to the operating carrier company, `status` equal to `ACTIVE`, and `deleted_at` null. The client UUID is not that proof. `insertExecution` does not perform this read today. The rule is future contract.

```text
OPERATING_DRIVER_AUTHORITY=transport.drivers under operating tenant
EXECUTION_DRIVER_SERVER_VERIFIED=YES
EXECUTION_DRIVER_MUST_MATCH_OPERATING_CARRIER=YES
SHIPMENT_LOCAL_DRIVER_COMPARED_TO_EXECUTION_DRIVER=NO
EXECUTION_DRIVER_NULL_IDENTITY_POLICY=NULL_IS_NO_ASSIGNMENT
```

Null means no operating driver is claimed. The identity layer accepts null and does not invent a driver row. Null is not operational readiness. A later TMS status stage decides whether activation requires a driver. This stage does not.

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

A non-null `transport_executions.vehicle_id` is stored only after the server reads `transport.vehicles` and finds `id` equal to the requested id, `tenant_id` equal to `operating_tenant_id`, `carrier_company_id` equal to the operating carrier company, `status` equal to `ACTIVE`, and `deleted_at` null. The client UUID is not that proof.

```text
OPERATING_VEHICLE_AUTHORITY=transport.vehicles under operating tenant
EXECUTION_VEHICLE_SERVER_VERIFIED=YES
EXECUTION_VEHICLE_MUST_MATCH_OPERATING_CARRIER=YES
SHIPMENT_LOCAL_VEHICLE_COMPARED_TO_EXECUTION_VEHICLE=NO
EXECUTION_VEHICLE_NULL_IDENTITY_POLICY=NULL_IS_NO_ASSIGNMENT
```

Null means no operating vehicle is claimed. The identity layer accepts null. Null is not operational readiness. A later TMS status stage decides whether activation requires a vehicle.

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

A write is the two-step handshake in section 14. A request body that names either company is not acceptance. The carrier command does not write `shipments.tenant_id`, shipper company, consignee company, or `shipment.carrier_company_id`.

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
ROLLBACK_BOUNDARY=FUTURE_RELATIONSHIP_VERSION_ROW_ONLY
```

An unmapped historical shipment stays unmapped. A later cross-tenant activation of that shipment fails closed. An ambiguous attribute match creates no row. Rollback of an uncommitted command writes no version row. A committed version row is not deleted or edited. Revocation appends a successor. Companies, shipments, awards, executions, drivers, vehicles, billing rows, and documents stay as stored.

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

## 12. Controller remediation R1

```text
A280_001_CLOSED=YES
A280_002_CLOSED=REOPENED
A280_003_CLOSED=YES
```

### Operating assignment

`transport.transport_executions.driver_id` and `vehicle_id` stay nullable. `insertExecution` currently writes the projection values with no read of `transport.drivers` or `transport.vehicles`. The future identity rule is the opposite of that write.

When `driver_id` is non-null, the server reads one `transport.drivers` row and requires all of these: `id` equals the requested id, `tenant_id` equals `operating_tenant_id`, `carrier_company_id` equals the operating carrier company on the execution, `status` equals `ACTIVE`, and `deleted_at` is null. A miss fails closed and stores no driver id.

When `vehicle_id` is non-null, the server reads one `transport.vehicles` row with the same five predicates. A miss fails closed and stores no vehicle id.

```text
EXECUTION_DRIVER_NULL_IDENTITY_POLICY=NULL_IS_NO_ASSIGNMENT
EXECUTION_VEHICLE_NULL_IDENTITY_POLICY=NULL_IS_NO_ASSIGNMENT
```

Null claims no resource. The identity layer allows it. A later TMS stage may require a non-null assignment before activation. This remediation does not make that operational decision, and it does not compare the shipment-local ids with the execution ids.

### Relationship command

R1 closed A280-002 with one command and `ACTIVE_COMPANY_MEMBERSHIP`. That closure is reopened. An active membership is read scope. It is not authority to create the relationship. The replacement contract is section 14.

### Version, pin, and uniqueness

```text
CARRIER_RELATIONSHIP_LIFECYCLE_MODEL=IMMUTABLE_APPEND_ONLY_VERSIONS
```

Each version is a new row. After insert, the row is not edited. Endpoint company identities are copied onto a successor and are never updated in place. Revocation inserts a successor with `status=REVOKED` and `supersedes_relationship_id` set to the previous version. The previous row remains the historical proof. Audit records the command correlation: source, accepting user, and idempotency key.

```text
relationship_id
relationship_version
shipper_tenant_id
shipper_carrier_company_id
operating_tenant_id
operating_carrier_company_id
status=ACTIVE|REVOKED
source_type
source_id
created_at
created_by
accepted_at
accepted_by
revoked_at
revoked_by
revoke_reason
supersedes_relationship_id
```

Resolution is activation, immediately before the TMS projection transaction. Route-plan scoring does not authorize execution. Accept selects the route plan and, in the TMS freeze, does not create execution identity and does not read this relationship. Activation resolves one unsuperseded `ACTIVE` version for the shipment owner tenant, the shipment-local carrier company, and the operating tenant. It then pins `relationship_id` and `relationship_version` on the `transport.transport_execution_revisions` row written in that same transaction. A later read of the committed execution uses that pin. It does not replace the pin with the current `ACTIVE` head.

```text
CARRIER_RELATIONSHIP_RESOLUTION_PHASE=ACTIVATE_BEFORE_TMS_PROJECTION
CARRIER_RELATIONSHIP_PIN_OWNER=TRANSPORT_EXECUTION_REVISION
ACTIVATION_REVALIDATES_RELATIONSHIP_ACTIVE=YES
EXECUTION_AUDIT_PRESERVES_RELATIONSHIP_PIN=YES
RELATIONSHIP_REVOKED_BEFORE_ACTIVATE_POLICY=REFUSE_NO_EXECUTION
RELATIONSHIP_REVOKED_AFTER_EXECUTION_POLICY=HISTORICAL_EXECUTION_UNCHANGED
ACTIVE_RELATIONSHIP_UNIQUENESS=ONE_UNSUPERSEDED_ACTIVE_FOR_SHIPPER_CARRIER_AND_OPERATING_TENANT
ZERO_RELATIONSHIP_POLICY=NOT_FOUND_FAIL_CLOSED
MULTIPLE_RELATIONSHIP_POLICY=DATA_INTEGRITY_ERROR_FAIL_CLOSED
```

The active key is `shipper_tenant_id`, `shipper_carrier_company_id`, and `operating_tenant_id` among version rows with `status=ACTIVE` and no successor. Zero such rows is not-found. More than one is a data-integrity error. Neither result creates an execution, and neither result picks a fallback company.

## 13. Failure and security matrix

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
| XTI_26 | Operating driver tenant differs from `operating_tenant_id` | Fail closed. No driver id stored |
| XTI_27 | Operating driver carrier company differs from the operating carrier | Fail closed. No driver id stored |
| XTI_28 | Operating vehicle tenant differs from `operating_tenant_id` | Fail closed. No vehicle id stored |
| XTI_29 | Operating vehicle carrier company differs from the operating carrier | Fail closed. No vehicle id stored |
| XTI_30 | Relationship command has no verified shipper-side source | Refuse. No row |
| XTI_31 | Relationship command has no active operating-carrier membership | Refuse. No row |
| XTI_32 | Same idempotency key resolves to a different company pair | Conflict. No second row |
| XTI_33 | Relationship is revoked after accept and before activate | Refuse. No execution |
| XTI_34 | Store returns more than one unsuperseded ACTIVE row | Data-integrity error. No execution |
| XTI_35 | Committed execution is read after revocation | Historical pin unchanged |
| XTI_DRIVER_01 | Driver belongs to another tenant | Fail closed. No driver id stored |
| XTI_DRIVER_02 | Driver belongs to another carrier company | Fail closed. No driver id stored |
| XTI_DRIVER_03 | Driver is inactive or deleted | Fail closed. No driver id stored |
| XTI_VEHICLE_01 | Vehicle belongs to another tenant | Fail closed. No vehicle id stored |
| XTI_VEHICLE_02 | Vehicle belongs to another carrier company | Fail closed. No vehicle id stored |
| XTI_VEHICLE_03 | Vehicle inactive or deleted | Fail closed. No vehicle id stored |
| XTI_36 | Ordinary active member attempts acceptance | Forbidden. No relationship |
| XTI_37 | User has multiple companies and no verified selected company | Refuse. Ambiguous context |
| XTI_38 | `CARRIER_ADMIN` role belongs to another company | Forbidden. No relationship |
| XTI_39 | `CARRIER_ADMIN` role belongs to another tenant | Forbidden. No relationship |
| XTI_40 | Raw shipment id is submitted without a relationship request | Refuse |
| XTI_41 | Raw RFX award id is submitted without a relationship request | Refuse |
| XTI_42 | Raw award-link id is submitted without a relationship request | Refuse |
| XTI_43 | Request token is valid and the actor is not `CARRIER_ADMIN` or operating-tenant `PLATFORM_ADMIN` | Forbidden |
| XTI_44 | `CARRIER_ADMIN` context is valid and the request token is missing | Refuse |
| XTI_45 | Request expired before acceptance | Refuse |
| XTI_46 | Request cancelled before acceptance | Refuse |
| XTI_47 | Source fingerprint changed after the request was created | Stale. Refuse |
| XTI_48 | Request is accepted for a different operating company | Conflict |
| XTI_49 | A second authorized admin replays an accepted request for the same operating company | Same relationship |
| XTI_50 | Opaque request id is presented without a valid token and admin context | Not-found. No existence leak |

## 14. Controller remediation R2

```text
A280_002_REOPENED=YES
A280_002_CLOSED=YES
A280_002A=CLOSED
A280_002B=CLOSED
```

R1 treated `ActiveMembershipExists` as acceptance. That predicate is `core.company_memberships` for `tenant_id`, `user_id`, `company_id`, `status = ACTIVE`, and `deleted_at` null. It proves the user can be read in that company. It does not prove the user may create a cross-tenant company relationship. `CARRIER_ADMIN` is a seeded role code. `core.user_roles` binds that role to one `tenant_id`, `user_id`, and `company_id`. A role on another company or another tenant is a different binding. `PLATFORM_ADMIN` is the existing tenant-global override. A company-scoped `PLATFORM_ADMIN` row is not that override. Gateway company selection strips untrusted company headers and matches the selected company to a membership. A role on another company does not authorize the selected company. This stage does not add a new trust of a client company header.

### Privileged acceptance

```text
OPERATING_CARRIER_ACCEPTANCE_EVIDENCE=ACTIVE_SELECTED_COMPANY_MEMBERSHIP+COMPANY_SCOPED_CARRIER_ADMIN_ROLE+VALID_RELATIONSHIP_REQUEST
OPERATING_SELECTED_COMPANY_SOURCE=SERVER_VERIFIED_SELECTED_COMPANY_CONTEXT
OPERATING_ACCEPTANCE_REQUIRED_ROLE=CARRIER_ADMIN|PLATFORM_ADMIN
ORDINARY_ACTIVE_MEMBER_CAN_ACCEPT_RELATIONSHIP=NO
MULTIPLE_MEMBERSHIPS_WITHOUT_SELECTED_COMPANY_POLICY=REFUSE
CLIENT_SUPPLIED_OPERATING_COMPANY_AUTHORITY=NO
```

Acceptance requires an authenticated operating tenant, a server-verified selected company that exists in that tenant, an active membership for the authenticated user and that company, and a `CARRIER_ADMIN` role on that same company and tenant. A verified tenant-global `PLATFORM_ADMIN` in the operating tenant may override the `CARRIER_ADMIN` check. It does not override the selected company, the active membership, or the relationship request. Several memberships and no verified selected company refuse. The server does not choose the first membership. A company id in the acceptance body is ignored.

The following do not accept: an ordinary active member, a driver-only user, a membership without the company admin role, a `CARRIER_ADMIN` role on another company, and a `CARRIER_ADMIN` role in another tenant.

### Two-sided request

A shipment, RFX award, or award-to-order link names only the shipper-local carrier company. It does not name the operating company. Knowing those ids is not correlation. Name, tax id, registration number, email, and phone are not correlation.

```text
CARRIER_BRIDGE_CREATION_COMMAND=CreateCarrierOperatingRelationshipRequest
CARRIER_BRIDGE_ACCEPTANCE_COMMAND=AcceptCarrierOperatingRelationshipRequest
TWO_SIDED_RELATIONSHIP_ACCEPTANCE_REQUIRED=YES
RELATIONSHIP_CREATED_ONLY_AFTER_BOTH_SIDES=YES
SHIPPER_SIDE_CONSENT_PINNED_BY=RELATIONSHIP_REQUEST_ID_AND_SOURCE_PROOF
OPERATING_SIDE_CONSENT_PINNED_BY=RELATIONSHIP_REQUEST_ID_AND_ADMIN_CONTEXT
```

Step 1 is `CreateCarrierOperatingRelationshipRequest`. The source-owning service verifies the authenticated shipper-side actor, that the actor may operate the source, that `source_type` is one of the three allowed values, and that `source_id` belongs to the source tenant. It derives the tenant-local carrier company from the source and proves that company exists in that tenant. It sends company-service a trusted internal subject:

```text
source_type
source_id
source_tenant_id
shipper_carrier_company_id
source_version or immutable source fingerprint
requesting_user_id
authorization_proof_context
```

```text
SHIPMENT_SOURCE_OWNER=SHIPMENT_SERVICE
RFX_AWARD_SOURCE_OWNER=RFX_SERVICE
AWARD_LINK_SOURCE_OWNER=RFX_SERVICE
COMPANY_SERVICE_DIRECT_TRANSPORT_AUTHORITY=NO
COMPANY_SERVICE_DIRECT_RFX_AUTHORITY=NO
```

`shipment-service` owns `transport.shipments`. `rfx-service` owns `rfx.rfx_awards` and `rfx.rfx_award_transport_orders`. Company-service does not read those tables as its own business authority. This stage does not add the internal API.

Company-service creates the request. The request is server-generated. Raw `source_type` and `source_id` in an acceptance body do not accept it.

```text
CARRIER_RELATIONSHIP_REQUEST_OWNER=COMPANY_SERVICE
SHIPPER_SOURCE_VERIFICATION_OWNER=SOURCE_OWNING_SERVICE
RELATIONSHIP_REQUEST_SERVER_GENERATED=YES
RAW_SOURCE_TYPE_ID_ACCEPTANCE_ALLOWED=NO
OPAQUE_REQUEST_TOKEN_REQUIRED=YES
REQUEST_TOKEN_ALONE_IS_AUTHORITY=NO
OPERATING_ADMIN_CONTEXT_REQUIRED=YES
SOURCE_OWNER_AUTHORIZATION_REQUIRED=YES
CLIENT_SUPPLIED_COMPANY_PAIR_AUTHORITY=NO
```

Request shape:

```text
relationship_request_id
request_version
source_type
source_id
shipper_tenant_id
shipper_carrier_company_id
request_status=PENDING|ACCEPTED|EXPIRED|CANCELLED
opaque_acceptance_token_hash
created_at
created_by
expires_at
cancelled_at
cancelled_by
cancel_reason
accepted_at
accepted_by
accepted_operating_tenant_id
accepted_operating_carrier_company_id
relationship_id
relationship_version
```

The request is immutable except through append-only lifecycle events or successor versions. The stored token is a hash. The token alone is not authority.

Step 2 is `AcceptCarrierOperatingRelationshipRequest`. The body supplies the opaque `relationship_request_id` or one-time acceptance token, and the idempotency key. The operating company is the server-verified selected company. Both companies are proved with `id` and `tenant_id` before the relationship version is inserted. The relationship version is inserted only after the request exists and that same request is accepted. An expired request, a cancelled request, and a request already accepted by another operating company refuse. A source fingerprint that no longer matches the request is stale and refuses.

```text
CARRIER_BRIDGE_COMMAND_IDEMPOTENCY_SCOPE=relationship_request_id+selected_operating_tenant_id+selected_operating_company_id+idempotency_key
ACCEPTING_USER_IS_AUDIT_PROVENANCE=YES
```

The same request, same operating company, and same key replay the same relationship version. The same request and a different operating company conflict. The same key and a different request conflict inside that namespace. `accepted_by` records the admin and does not split the relationship when another authorized admin retries the same accepted request for the same operating company. A guessed request id without the token and the admin context is not-found and does not reveal that a request exists.
