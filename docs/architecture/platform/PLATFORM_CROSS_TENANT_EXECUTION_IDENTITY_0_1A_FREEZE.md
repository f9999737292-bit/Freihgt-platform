# Platform cross-tenant execution identity freeze

```text
AGENT=A
STAGE=PLATFORM-CROSS-TENANT-EXECUTION-IDENTITY-0.1A-DISCOVERY
REMEDIATION_STAGE=PLATFORM-CROSS-TENANT-EXECUTION-IDENTITY-0.1A-PR280-CONTROLLER-REMEDIATION-R2
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
ROLLBACK_BOUNDARY=FUTURE_RELATIONSHIP_VERSION_ROW_ONLY
A280_001_CLOSED=YES
A280_002_CLOSED=YES
A280_003_CLOSED=YES
READY_FOR_CROSS_TENANT_IDENTITY_RUNTIME_DESIGN=YES
READY_FOR_NLO_0_9D_STATUS_INTEGRATION=NO
READY_FOR_NLO_0_9D_RUNTIME=NO
```

This freeze selects the contract. It does not create the relationship, does not change the shipment FSM, and does not authorize NLO-0.9D or runtime. Evidence is in [PLATFORM_CROSS_TENANT_EXECUTION_IDENTITY_0_1A_DISCOVERY.md](PLATFORM_CROSS_TENANT_EXECUTION_IDENTITY_0_1A_DISCOVERY.md).

## Carrier relationship

The operating execution carrier is a `core.companies` row in `operating_tenant_id`. The shipment carrier reference is a different `core.companies` row in the shipment tenant. The authority that they represent the same operating carrier is a server-generated relationship request plus privileged acceptance of that same request.

```text
CARRIER_BRIDGE_CREATION_COMMAND=CreateCarrierOperatingRelationshipRequest
CARRIER_BRIDGE_ACCEPTANCE_COMMAND=AcceptCarrierOperatingRelationshipRequest
OPERATING_CARRIER_ACCEPTANCE_EVIDENCE=ACTIVE_SELECTED_COMPANY_MEMBERSHIP+COMPANY_SCOPED_CARRIER_ADMIN_ROLE+VALID_RELATIONSHIP_REQUEST
OPERATING_ACCEPTANCE_REQUIRED_ROLE=CARRIER_ADMIN|PLATFORM_ADMIN
ORDINARY_ACTIVE_MEMBER_CAN_ACCEPT_RELATIONSHIP=NO
```

`CreateCarrierOperatingRelationshipRequest` is step 1. The source owner verifies the shipper-side actor and the source, derives the shipper-local carrier company, and sends company-service the trusted subject `source_type`, `source_id`, `source_tenant_id`, `shipper_carrier_company_id`, source version or fingerprint, `requesting_user_id`, and authorization proof. Company-service stores the request. It does not query `transport.*` or `rfx.*` as the business owner.

| source_type | Owner | Carrier column |
| --- | --- | --- |
| `SHIPMENT` | `shipment-service` | `transport.shipments.carrier_company_id` |
| `RFX_AWARD` | `rfx-service` | `rfx.rfx_awards.carrier_company_id` |
| `AWARD_TRANSPORT_ORDER_LINK` | `rfx-service` | `rfx.rfx_award_transport_orders.carrier_company_id` |

```text
SHIPMENT_SOURCE_OWNER=SHIPMENT_SERVICE
RFX_AWARD_SOURCE_OWNER=RFX_SERVICE
AWARD_LINK_SOURCE_OWNER=RFX_SERVICE
COMPANY_SERVICE_DIRECT_TRANSPORT_AUTHORITY=NO
COMPANY_SERVICE_DIRECT_RFX_AUTHORITY=NO
```

The request row is immutable except through append-only lifecycle events or successor versions:

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

`AcceptCarrierOperatingRelationshipRequest` is step 2. The body supplies the opaque request id or one-time token, and the idempotency key. The operating company is the server-verified selected company. The body does not supply it. Acceptance also requires an active membership for that user and company, and a `CARRIER_ADMIN` role on that same company and tenant. Tenant-global `PLATFORM_ADMIN` in the operating tenant may override the role check only. Several memberships and no selected company refuse. An ordinary member, a driver-only user, a role on another company, and a role in another tenant do not accept.

The relationship version is inserted only when the shipper-side request and the operating admin acceptance of that same request both exist. A raw source id, a name, a tax id, a registration number, an email, a phone, and an ordinary membership are not that correlation. The token alone is not authority. An expired, cancelled, stale, or already-accepted-by-another-company request refuses.

```text
CARRIER_BRIDGE_COMMAND_IDEMPOTENCY_SCOPE=relationship_request_id+selected_operating_tenant_id+selected_operating_company_id+idempotency_key
ACCEPTING_USER_IS_AUDIT_PROVENANCE=YES
```

The same request, same operating company, and same key replay the same relationship version. A different operating company or a different request on that key conflicts. Another authorized admin replaying the accepted request for the same company returns the same relationship. `accepted_by` is audit. A guessed request id without the token and admin context is not-found.

## Version row

```text
CARRIER_RELATIONSHIP_LIFECYCLE_MODEL=IMMUTABLE_APPEND_ONLY_VERSIONS
CARRIER_RELATIONSHIP_ENDPOINTS_IMMUTABLE=YES
```

Each version is inserted and then left unchanged. Endpoint company identities are never edited. Revocation inserts a successor with `status=REVOKED` and `supersedes_relationship_id` pointing at the previous version. The previous row remains historical proof. Audit retains the source, the accepting user, and the idempotency key.

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

One unsuperseded `ACTIVE` version is allowed for `shipper_tenant_id + shipper_carrier_company_id + operating_tenant_id`. Zero is not-found and fails closed. More than one is a data-integrity error and fails closed. There is no fallback company.

## Resolution pin

```text
CARRIER_RELATIONSHIP_RESOLUTION_PHASE=ACTIVATE_BEFORE_TMS_PROJECTION
CARRIER_RELATIONSHIP_PIN_OWNER=TRANSPORT_EXECUTION_REVISION
```

Scoring does not authorize execution. Accept selects the route plan and does not create execution identity. Activation, immediately before TMS projection commits, resolves the one unsuperseded `ACTIVE` version for the shipment owner tenant, the shipment-local carrier company, and the operating tenant. The projection transaction stores `relationship_id` and `relationship_version` on `transport.transport_execution_revisions`.

A relationship revoked after accept and before activate refuses activation and writes no execution. A relationship revoked after commit leaves the pinned revision unchanged. A later read uses the pin. It does not rewrite that pin from the current `ACTIVE` head.

Shipment-service resolves the relationship only inside a verified shipment or execution command. The read returns the operating carrier company id, the relationship id, and the relationship version. It does not return drivers, vehicles, or another shipper shipment.

## Operating driver and vehicle

```text
OPERATING_DRIVER_AUTHORITY=transport.drivers under operating tenant
OPERATING_VEHICLE_AUTHORITY=transport.vehicles under operating tenant
EXECUTION_DRIVER_NULL_IDENTITY_POLICY=NULL_IS_NO_ASSIGNMENT
EXECUTION_VEHICLE_NULL_IDENTITY_POLICY=NULL_IS_NO_ASSIGNMENT
```

A non-null execution driver id is stored only when `transport.drivers` matches the requested id, `operating_tenant_id`, the operating carrier company, `status = ACTIVE`, and `deleted_at` null. A non-null execution vehicle id uses the same five predicates on `transport.vehicles`. Any miss fails closed and stores no assignment id. A client UUID is not authority.

Null means no operating resource is claimed. The identity layer allows null. Null is not operational readiness. A later TMS stage may require an assignment before activation. This freeze does not. Shipment driver and vehicle ids are not compared with the execution columns. The shipment FSM is unchanged.

## History

Historical companies, shipments, awards, executions, drivers, vehicles, billing rows, and documents stay as stored. Automatic matching is refused. Rollback of an uncommitted command writes no version row. A committed version row is not deleted or edited.

POD remains compatible: document tenant and owner company stay in the operating tenant, and shipment tenant stays the owner. Finance continues to compare tenant-local company ids. Finance runtime change is not authorized.

## Failure matrix

XTI_01 through XTI_50, and XTI_DRIVER_01 through XTI_DRIVER_03, and XTI_VEHICLE_01 through XTI_VEHICLE_03, in the discovery note are `FUTURE_REQUIRED`. They are part of this contract and are not implemented here.
