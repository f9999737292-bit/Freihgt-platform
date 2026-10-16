# EDO POD delivery-action evidence binding 0.1A freeze

## Status

```text
STAGE=EDO-POD-DELIVERY-ACTION-EVIDENCE-BINDING-0.1A-FREEZE
BASE_SHA=6f7ba7e0fe65beefde8989cf64105387e51730bd
KIND=DOCS_ONLY_CONTRACT
IMPLEMENTATION_AUTHORIZED=NO
PRODUCT_CODE_CHANGED=NO
MIGRATION_CREATED=NO
OPENAPI_CHANGED=NO
GATEWAY_CHANGED=NO
STAGING_CHANGED=NO
POD_EVIDENCE_CANONICAL_SUBJECT=DELIVERY_ACTION
SHIPMENT_ONLY_BINDING_SUFFICIENT=NO
SELECTED_BINDING_MODEL=B_DEDICATED_IMMUTABLE_TABLE
POD_EVIDENCE_BINDING_GRANULARITY=DOCUMENT_ID+DOCUMENT_VERSION_ID+DOCUMENT_FILE_ID
DOCUMENT_TO_DELIVERY_ACTION_CARDINALITY=ONE
DELIVERY_ACTION_TO_DOCUMENT_CARDINALITY=MANY
MULTISTOP_EVIDENCE_ISOLATED=YES
ACTION_ID_CLIENT_AUTHORITY=NO
ACTION_TYPE_AUTHORITY=TMS
ACTION_SHIPMENT_BINDING_AUTHORITY=TMS
TMS_ACTION_VERIFICATION_CONTRACT=SHIPMENT_SERVICE_PROXIES_UPLOAD_LIFECYCLE
POD_REQUIREMENT_ENDPOINT_IS_EVIDENCE_DEPENDENCY=NO
POD_DOCUMENT_TENANT_POLICY=OPERATING_CARRIER_TENANT
POD_DOCUMENT_OWNER_COMPANY_SOURCE=transport.transport_executions.carrier_company_id
POD_EVIDENCE_AUTHOR_COMPANY_SOURCE=transport.drivers.carrier_company_id
DOCUMENT_TENANT_EQUALS_SHIPMENT_TENANT=NO
DOCUMENT_TENANT_EQUALS_OPERATING_TENANT=YES
CREATE_INTENT_CALLER=shipment-service
UPLOAD_CONTENT_CALLER=shipment-service
COMPLETE_UPLOAD_CALLER=shipment-service
CREATE_INTENT_TENANT_SOURCE=execution.operating_tenant_id
UPLOAD_CONTENT_TENANT_SOURCE=pinned document_tenant_id
COMPLETE_UPLOAD_TENANT_SOURCE=pinned document_tenant_id
CLIENT_SUPPLIED_TENANT_AUTHORITY=NO
POD_ACTION_EVIDENCE_INTERNAL_AUTH=REQUIRED
INTERNAL_CALLER_ALLOWLIST=shipment-service
DIRECT_BROWSER_TO_DOCUMENT_SERVICE_ALLOWED=NO
DRIVER_ID_AUTHORITY=transport.transport_executions.driver_id
DRIVER_ASSIGNMENT_AUTHORITY=shipment-service/TMS
DOCUMENT_SERVICE_VERIFIES_DRIVER_ASSIGNMENT=NO
ACTION_BIND_PIN_TIME=CREATE_UPLOAD_INTENT
IDEMPOTENCY_KEY_REQUIRED=YES
POD_ACTION_BINDING_IDEMPOTENCY_SCOPE=document_tenant_id+execution_action_id+idempotency_key
BOUND_EVIDENCE_PRESENT_STATE=COMPLETED_UPLOAD_AND_PINNED_FILE
DOCUMENT_EXISTS_IMPLIES_REQUIREMENT=NO
POD_UPLOAD_WRITES_TMS_REQUIREMENT=NO
TMS_REQUIREMENT_WRITES_DOCUMENT_EVIDENCE=NO
CROSS_DOMAIN_WRITE_REQUIRED=NO
POD_ACCEPTANCE_POLICY_EVALUATED_IN_THIS_STAGE=NO
POD_EVIDENCE_AUTO_INHERIT_TO_SUCCESSOR=NO
HISTORICAL_SHIPMENT_POD_AUTO_BACKFILL=NO
B_BLOCKER_DELIVERY_ACTION_EVIDENCE_IDENTITY=CLOSED
B_BLOCKER_TENANT_MODEL=CLOSED
B_BLOCKER_CALLER_AUTHORITY=CLOSED
B_BLOCKER_DRIVER_ACTION_AUTH=CLOSED
B_BLOCKER_IDEMPOTENCY=CLOSED
READY_FOR_POD_EVIDENCE_BINDING_RUNTIME=YES
TMS_REQUIREMENT_FOUNDATION_READY=YES
EDO_ACTION_EVIDENCE_CONTRACT_READY=YES
POD_ACCEPTANCE_POLICY_READY=NO
POD_SATISFACTION_RUNTIME_READY=NO
```

Proof is in [edo-pod-delivery-action-evidence-binding-v0.1a-discovery.md](edo-pod-delivery-action-evidence-binding-v0.1a-discovery.md). This freeze is the contract a later authorized stage implements. It does not authorize that stage.

## Subject

The canonical subject is one `transport.transport_execution_actions` row with `action_type=DELIVERY`, addressed by `operating_tenant_id`, `execution_id`, and `execution_action_id`.

The action must belong to that execution. TMS is the authority for action type and for the action's shipment, cargo, and stop. A shipment id is not a substitute subject.

## Binding row

Conceptual table `documents.pod_delivery_action_evidence`. No migration number is assigned.

| Column | Rule |
| --- | --- |
| `id` | Stable id |
| `document_tenant_id` | Equals `execution.operating_tenant_id`. This is the document row tenant. It is not the shipment tenant |
| `operating_tenant_id` | Same value as `document_tenant_id`, copied from the verified execution |
| `owner_company_id` | `transport.transport_executions.carrier_company_id`. Must exist in the operating tenant |
| `author_driver_id` | `transport.transport_executions.driver_id`. Actor provenance. Not the owner company |
| `author_company_id` | `transport.drivers.carrier_company_id` for that driver. Actor provenance. Not `documents.owner_company_id` |
| `execution_id` | Verified execution |
| `execution_action_id` | Verified `DELIVERY` action. Immutable |
| `execution_stop_id` | Stop that parents the action |
| `shipment_id` | Action shipment. Matches `documents.related_entity_id` while the generic relation remains `SHIPMENT` |
| `shipment_tenant_id` | `transport.shipments.tenant_id` copied from the action. Shipment ownership. Not the document tenant |
| `cargo_id` | Action cargo |
| `document_id` | Immutable. At most one action |
| `document_version_id` | Version that owns the file |
| `document_file_id` | File inserted at complete |
| `idempotency_key` | Required. With `document_tenant_id` and `execution_action_id`, identifies the intent |
| `created_at` | Insert time |

No update of `execution_action_id` or `document_id`. A correction is a new document and a new row. `RELATED_TO` is not this binding. Successor satisfaction is not decided here.

Uniqueness: one row per (`document_tenant_id`, `execution_action_id`, `idempotency_key`). The key is not null. At most one action per `document_id`. Several rows may share one action.

Index: (`operating_tenant_id`, `execution_id`, `execution_action_id`) and (`document_tenant_id`, `document_id`). No foreign key into `transport`.

## Tenancy

```text
POD_DOCUMENT_TENANT_POLICY=OPERATING_CARRIER_TENANT
DOCUMENT_TENANT_EQUALS_SHIPMENT_TENANT=NO
DOCUMENT_TENANT_EQUALS_OPERATING_TENANT=YES
POD_DOCUMENT_OWNER_COMPANY_SOURCE=transport.transport_executions.carrier_company_id
POD_EVIDENCE_AUTHOR_COMPANY_SOURCE=transport.drivers.carrier_company_id
SHIPMENT_TENANT_SOURCE=transport.shipments.tenant_id
EXECUTION_OPERATING_TENANT_SOURCE=transport.transport_executions.operating_tenant_id
```

Shipment ownership stays `shipment_tenant_id`. The document row tenant and `documents.owner_company_id` stay in the operating tenant. The driver and the driver company are actor provenance. A client body does not choose any of these.

`CompanyExists(owner_company_id, document_tenant)` must succeed for the execution carrier company. It must not be asked to accept a company from the shipment tenant, and the driver company must not be written into `owner_company_id`.

The shipment link is authorized by the TMS action proof: `action.shipment_id`, `action.shipment_tenant_id`, and `transport.shipments.tenant_id` agree. `ShipmentExists(shipment_id, document_tenant)` is not that proof when the document tenant is the operating tenant and the shipment tenant is different. Fail closed, with no document id, when those shipment facts disagree or the operating tenant does not own the execution.

## Caller and lifecycle

shipment-service proxies create, content upload, and complete after it has verified the action and the driver. document-service accepts those three calls only from `shipment-service`.

```text
CREATE_INTENT_CALLER=shipment-service
UPLOAD_CONTENT_CALLER=shipment-service
COMPLETE_UPLOAD_CALLER=shipment-service
CREATE_INTENT_TENANT_SOURCE=execution.operating_tenant_id
UPLOAD_CONTENT_TENANT_SOURCE=pinned document_tenant_id
COMPLETE_UPLOAD_TENANT_SOURCE=pinned document_tenant_id
CLIENT_SUPPLIED_TENANT_AUTHORITY=NO
POD_ACTION_EVIDENCE_INTERNAL_AUTH=REQUIRED
INTERNAL_CALLER_ALLOWLIST=shipment-service
DIRECT_BROWSER_TO_DOCUMENT_SERVICE_ALLOWED=NO
```

Authentication is `packages/shared-go/internalauth` token middleware plus `X-Internal-Service-Name=shipment-service`. A present `X-Tenant-ID` is not authentication and is not the tenant source on content or complete. The browser does not call document-service. The gateway does not call document-service for this binding.

Create stores the pin `execution_id`, `execution_action_id`, `shipment_id`, `shipment_tenant_id`, and `document_tenant_id`. Content and complete resolve the intent with the pinned document tenant and the upload id. A different tenant or a different member of that pin fails the call. The intent is not re-homed.

The existing `GET .../pod-requirement` route is not this authorization. Evidence capture does not read `pod_required`.

Driver assignment is `transport.transport_executions.driver_id` for `operating_tenant_id`, and the action's stop belongs to that execution. document-service does not compute that assignment. `shipments.driver_id` on the current gateway POD path is not action authority.

## Pin and idempotency

The action, execution, shipment, shipment tenant, and document tenant are pinned at create-upload-intent. Upload content and complete refuse a different subject. The object key and the binding row stay on the pinned ids.

```text
IDEMPOTENCY_KEY_REQUIRED=YES
POD_ACTION_BINDING_IDEMPOTENCY_SCOPE=document_tenant_id+execution_action_id+idempotency_key
```

A missing key is a validation reject and inserts no intent. The same key, same action, and same document tenant replay one intent when the shipment pin matches. The same key with a different action is `409`. The same key and action with a different `shipment_id` or `shipment_tenant_id` is `409`. Completing twice yields one binding.

## Presence

`EVIDENCE_PRESENT` requires a binding row, intent status `completed`, and the pinned `document_file_id` on a document whose `deleted_at` is null. A pending intent, an uploaded object that is not completed, a document without that file, a signature, or status `SIGNED` is not presence.

Presence does not set `POD_REQUIRED`. A missing file does not set `POD_REQUIRED=false`. Evidence may exist while the TMS decision is `UNKNOWN` or known not required. A known required decision with no binding is still a requirement decision. It is not rewritten by document-service.

Acceptance policy is not evaluated. `POD_SATISFIED` is not returned. Until the TMS decision, this action evidence, and a pinned acceptance policy are all known, satisfaction stays `UNKNOWN`.

## Read

Internal `GetPODActionEvidence(operating_tenant_id, execution_id, execution_action_id)` returns the pinned identities for that action: none, one, or many. Many rows stay a list. The read does not choose a satisfying file.

An unverified or foreign subject is not-found and contains no document id. A verified action with no completed binding is an empty list. Store unavailability is an error, not an empty list.

## Non-inheritance

A binding does not follow a return, redirect, or successor delivery action. A historical document with only `related_entity_type=SHIPMENT` is not assigned to an action. Backfill requires a separate authorization.

## Non-goals

No Go change, migration, OpenAPI change, gateway change, shipment-service change, driver-app change, acceptance policy, satisfaction runtime, historical backfill, analytics change, or staging deploy belongs to this freeze.
