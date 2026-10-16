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
TMS_ACTION_VERIFICATION_CONTRACT=SHIPMENT_SERVICE_AUTHORIZES_INTENT
POD_REQUIREMENT_ENDPOINT_IS_EVIDENCE_DEPENDENCY=NO
POD_UPLOAD_CALLER=shipment-service
TENANT_AUTHORITY=action.shipment_tenant_id
DRIVER_ID_AUTHORITY=transport.transport_executions.driver_id
OWNER_COMPANY_AUTHORITY=trusted command plus CompanyExists in the document tenant
DRIVER_ASSIGNMENT_AUTHORITY=shipment-service/TMS
DOCUMENT_SERVICE_VERIFIES_DRIVER_ASSIGNMENT=NO
ACTION_BIND_PIN_TIME=CREATE_UPLOAD_INTENT
POD_ACTION_BINDING_IDEMPOTENCY_SCOPE=shipment_tenant_id+execution_action_id+idempotency_key
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
| `document_tenant_id` | Equals `action.shipment_tenant_id` and `transport.shipments.tenant_id` |
| `operating_tenant_id` | TMS scope copied from the verified execution. Not the document tenant |
| `execution_id` | Verified execution |
| `execution_action_id` | Verified `DELIVERY` action. Immutable |
| `execution_stop_id` | Stop that parents the action |
| `shipment_id` | Action shipment. Matches `documents.related_entity_id` while the generic relation remains `SHIPMENT` |
| `cargo_id` | Action cargo |
| `document_id` | Immutable. At most one action |
| `document_version_id` | Version that owns the file |
| `document_file_id` | File inserted at complete |
| `idempotency_key` | With the action, identifies the intent |
| `created_at` | Insert time |

No update of `execution_action_id` or `document_id`. A correction is a new document and a new row. `RELATED_TO` is not this binding. Successor satisfaction is not decided here.

Uniqueness: one row per (`document_tenant_id`, `execution_action_id`, `idempotency_key`) when the key is present, and at most one action per `document_id`. Several rows may share one action.

Index: (`operating_tenant_id`, `execution_id`, `execution_action_id`) and (`document_tenant_id`, `document_id`). No foreign key into `transport`.

## Tenancy

```text
DOCUMENT_TENANT_SOURCE=documents.documents.tenant_id
SHIPMENT_TENANT_SOURCE=transport.shipments.tenant_id
EXECUTION_OPERATING_TENANT_SOURCE=transport.transport_executions.operating_tenant_id
```

The document tenant is the shipment tenant. The operating tenant locates the execution. Neither value is taken from a browser body or from an unverified `X-Tenant-ID` on document-service.

Fail closed, with no document id in the response, when the action's shipment tenant, the shipment row tenant, and the document tenant disagree, or when the operating tenant does not own the execution.

## Caller

shipment-service authorizes the upload intent after it has verified the action and the driver. document-service accepts that subject only from that authenticated internal caller. It does not accept an action id from the driver app or from a direct call that merely parses a body.

The existing `GET .../pod-requirement` route is not this authorization. Evidence capture does not read `pod_required`.

Driver assignment is `transport.transport_executions.driver_id` for `operating_tenant_id`, and the action's stop belongs to that execution. document-service does not compute that assignment. `shipments.driver_id` on the current gateway POD path is not action authority.

`owner_company_id` is supplied by the trusted command and must exist in the document tenant. A client company id is not authority.

## Pin and idempotency

The action, execution, and shipment are pinned at create-upload-intent. Upload content and complete refuse a different subject. The object key and the binding row stay on the pinned ids.

Scope is `shipment_tenant_id + execution_action_id + idempotency_key`. The same key and action replay one intent. The same key and a different action conflict. Completing twice yields one binding.

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
