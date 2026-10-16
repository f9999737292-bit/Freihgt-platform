# EDO POD delivery-action evidence binding 0.1A discovery

## Status

```text
STAGE=EDO-POD-DELIVERY-ACTION-EVIDENCE-BINDING-0.1A-DISCOVERY
BASE_SHA=6f7ba7e0fe65beefde8989cf64105387e51730bd
MAIN_MIGRATION_HEAD=000098
KIND=DOCS_ONLY_DISCOVERY
PRODUCT_CODE_CHANGED=NO
MIGRATION_CREATED=NO
OPENAPI_CHANGED=NO
GATEWAY_CHANGED=NO
STAGING_CHANGED=NO
IMPLEMENTATION_AUTHORIZED=NO
TMS_POD_REQUIREMENT_FOUNDATION_IN_MAIN=YES
POD_EVIDENCE_CANONICAL_SUBJECT=DELIVERY_ACTION
SHIPMENT_ONLY_BINDING_SUFFICIENT=NO
```

This note proves the current POD upload path and freezes how a later stage binds POD evidence to one TMS delivery action. It does not implement that binding. The normative contract is [edo-pod-delivery-action-evidence-binding-v0.1a-freeze.md](edo-pod-delivery-action-evidence-binding-v0.1a-freeze.md).

TMS requirement ownership stays on [tms/TMS_POD_REQUIREMENT_SOURCE_0_4C.md](tms/TMS_POD_REQUIREMENT_SOURCE_0_4C.md) and migration `000098`. This stage does not write a known POD decision and does not evaluate acceptance or satisfaction.

## 1. Current POD upload

`ACTUAL_RUNTIME_SOURCE` at `6f7ba7e0`.

| Fact | Location |
| --- | --- |
| Public driver route | `services/api-gateway/internal/http/router.go` routes `POST/PUT /api/v1/driver/me/shipments/{shipmentId}/pod/uploads` |
| Gateway handler | `services/api-gateway/internal/driver/pod_handler.go` `InitiatePODUpload` |
| Gateway client | `services/api-gateway/internal/document/client.go` `CreatePODUpload` posts `/internal/v1/pod-uploads` |
| document-service route | `services/document-service/internal/http/router.go` `/internal/v1/pod-uploads` |
| Handler body | `services/document-service/internal/http/handlers/pod_upload_handler.go` `CreateIntent` |
| Persist | `services/document-service/internal/service/pod_upload_service.go` `CreateUploadIntent` |
| Intent table | `infrastructure/migrations/000033_add_document_upload_intent.up.sql` `documents.document_upload_intent` |
| Document row | `documents.documents` via `DocumentRepository.CreateDocument` |
| First version | `documents.document_versions` inserted in the same create transaction, `version_number=1` |
| File row | `documents.document_files` in `CompleteUpload` through `DocumentService.AddFile` |

`CreatePODUploadInput` accepts `TenantID`, `ShipmentID`, `DriverID`, `OwnerCompanyID`, `MimeType`, `FileName`, and `IdempotencyKey`. The create path sets:

```text
document_type=POD
related_entity_type=SHIPMENT
related_entity_id=shipment_id
```

`allowedRelatedEntityTypes` in `services/document-service/internal/domain/document.go` is `SHIPMENT`, `TRANSPORT_ORDER`, `BILLING_REGISTER`, `INVOICE`, and `COMPANY`. There is no execution or delivery-action type.

```text
EXECUTION_ID_PRESENT_IN_POD_UPLOAD=NO
EXECUTION_ACTION_ID_PRESENT_IN_POD_UPLOAD=NO
DELIVERY_ACTION_BINDING_PRESENT=NO
CURRENT_POD_RELATED_ENTITY_TYPE=SHIPMENT
CURRENT_POD_RELATED_ENTITY_SCOPE=SHIPMENT_ID
```

`documents.document_attachments` and `documents.signature_verification_evidence` are not written by this upload path. `CompleteUpload` writes `documents.document_files` against `LatestVersion`.

Intent idempotency is `UNIQUE (tenant_id, driver_id, idempotency_key)`. `findIntentByIdempotency` returns that row and does not compare `shipment_id`. A replay can therefore return an intent created for a different shipment.

## 2. Current TMS delivery action

`ACTUAL_RUNTIME_SOURCE`. Migration head on this tree is `000098_tms_pod_requirement_decision_foundation_v0_4d`.

`transport.transport_execution_actions` (`000088`) stores `shipment_id`, `shipment_tenant_id`, `cargo_id`, and `action_type` of `PICKUP` or `DELIVERY`. The stop parents the action. The stop parents `transport.transport_executions`, whose tenant is `operating_tenant_id`. The execution table has no `shipment_id`. The migration comment states that shipment ownership is not moved.

`GET /internal/v1/transport-executions/{executionId}/actions/{actionId}/pod-requirement` is mounted with `internalauth` in `services/shipment-service/internal/http/router.go`. `PODRequirementRepository.loadSubject` loads the action only when `e.id`, `e.operating_tenant_id`, and `a.id` match. `GetPODRequirement` rejects a non-`DELIVERY` action with `ACTION_NOT_DELIVERY`. A missing decision row is synthetic `UNKNOWN`. It does not create a document.

The same response includes `pod_required`. That field is a requirement fact. Evidence binding must not call this route as its proof that the action exists, because a later caller could treat `pod_required=true` as a precondition for storing bytes.

`transport.transport_execution_actions.evidence_id` points at `transport.shipment_cargo_execution_evidence`. That table's states are `PLANNED`, `PICKED_UP`, `CONFIRMED_ONBOARD`, and `UNLOADED`. It is onboard execution evidence. It is not a POD document.

## 3. Selected binding

| Variant | Result |
| --- | --- |
| A. Add `DELIVERY_ACTION` to `related_entity_type` and store the action id in `related_entity_id` | Rejected. The column pair holds one subject. Replacing `SHIPMENT` drops the shipment link the current create path and `ShipmentExists` require. Keeping only the shipment id leaves the action unstored |
| B. Dedicated immutable `documents.pod_delivery_action_evidence` | Selected. The generic document can still reference the shipment. The action identity is an explicit row. One action is queryable. Tenant predicates stay on the row. The action id and document id are not updated |
| C. `action.evidence_id` or `shipment_cargo_execution_evidence` | Rejected. Those rows are TMS execution state. Writing a document id there is a cross-domain write. Their state machine is not POD file evidence |

```text
SELECTED_BINDING_MODEL=B_DEDICATED_IMMUTABLE_TABLE
REJECTED_VARIANTS=A_GENERIC_RELATED_ENTITY,C_TMS_ACTION_EVIDENCE_ID
POD_EVIDENCE_CANONICAL_SUBJECT=DELIVERY_ACTION
SHIPMENT_ONLY_BINDING_SUFFICIENT=NO
```

The shipment relation on `documents.documents` remains. It is context. It is not the canonical POD subject.

## 4. Granularity

`CreateUploadIntent` creates the document and version 1 before bytes exist. `UploadContent` stores the object and marks the intent `uploaded`. `CompleteUpload` inserts one `documents.document_files` row on the latest version and marks the intent `completed`. A second complete of the same intent returns the latest file id. It does not insert a second binding.

A document can gain another version or another file later. `documents.documents.version` is the optimistic counter, not the evidence generation. Signature and attachment rows are separate and are not this upload.

The evidence fact is therefore the file that complete pinned, on the version that owned it, on the document that owns that version.

```text
POD_EVIDENCE_BINDING_GRANULARITY=DOCUMENT_ID+DOCUMENT_VERSION_ID+DOCUMENT_FILE_ID
```

`ATTACHMENT_ID` is not part of the current POD upload path. A later attachment must not be substituted for `document_file_id`.

## 5. Cardinality

One binding row cites one delivery action and one document. A document id appears on at most one action. One action may have several binding rows, because a driver may upload more than one file and each completed file is its own document today.

```text
DOCUMENT_TO_DELIVERY_ACTION_CARDINALITY=ONE
DELIVERY_ACTION_TO_DOCUMENT_CARDINALITY=MANY
MULTISTOP_EVIDENCE_ISOLATED=YES
ONE_DOCUMENT_CANNOT_SILENTLY_SATISFY_MULTIPLE_DELIVERY_ACTIONS=YES
```

Shipment `S` with delivery actions `A1` and `A2` does not share evidence. A POD whose related entity is only `S` does not bind either action.

## 6. Tenant model

Three different sources exist.

| Name | Source | Proven by |
| --- | --- | --- |
| `DOCUMENT_TENANT_SOURCE` | `documents.documents.tenant_id` | Set from the tenant passed into `CreateUploadIntent`. `ShipmentExists` requires `transport.shipments.id` and `transport.shipments.tenant_id` to match that tenant |
| `SHIPMENT_TENANT_SOURCE` | `transport.shipments.tenant_id` | Copied to `transport.transport_execution_actions.shipment_tenant_id`. `000088` says shipment ownership is not moved |
| `EXECUTION_OPERATING_TENANT_SOURCE` | `transport.transport_executions.operating_tenant_id` | Action load predicate. It is not `shipment_tenant_id` |

```text
DOCUMENT_TENANT_SOURCE=documents.documents.tenant_id
SHIPMENT_TENANT_SOURCE=transport.shipments.tenant_id
EXECUTION_OPERATING_TENANT_SOURCE=transport.transport_executions.operating_tenant_id
```

They are not assumed equal. The document row tenant is the shipment tenant. The operating tenant is only the TMS scope used to find the execution. A caller authenticated as operating tenant `C` must not insert `documents.tenant_id=C` for a shipment owned by `S`, and must not insert `documents.tenant_id=S` while claiming the caller tenant `C` is the document owner.

Future check, all required:

- the TMS action's `shipment_id` equals the document's shipment relation;
- the action's `shipment_tenant_id` equals `transport.shipments.tenant_id`;
- `documents.tenant_id` equals that shipment tenant;
- `operating_tenant_id` matches the execution that owns the action;
- any mismatch fails before insert.

`owner_company_id` must already exist in that document tenant (`CompanyExists`). The client body does not choose the tenant or the company.

## 7. Action verification and caller

document-service has no internal-token middleware. `PODUploadHandler` parses `X-Tenant-ID`, `shipmentId`, `driverId`, and `ownerCompanyId` from the request. On that service those values are `CLIENT_CONTROLLED`.

The gateway driver route is behind `AuthWithIntegrationSupport`. When auth is enabled, `buildRequestContext` takes the tenant and user from the auth context. `requireAssignedDriver` then calls shipment-service `GET /v1/driver/me/shipments/{id}`, which uses `GetByIDAndDriver` for `shipments.driver_id` in that same tenant, and `GET /v1/driver/me` for the driver id and company id. The gateway, not the browser body, fills `driverId` and `ownerCompanyId` on the document-service call. That makes them `SERVER_VERIFIED` only on the gateway path, and only for the shipment driver pointer. The document-service handler cannot see that distinction.

`GetByIDAndDriver` is not `transport.transport_executions.driver_id` and it does not check `action_type=DELIVERY`.

```text
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
```

shipment-service already proves driver-to-execution with `execution.driver_id` and `operating_tenant_id` (`driver_stop_task_repository.go` `ListCurrentNext`). The stop task is one row per `execution_stop_id`. The action belongs to that stop. That is the assignment proof. document-service does not reimplement it.

The pod-requirement GET stays a requirement read. Capture uses a neutral action proof: the action belongs to the execution, `action_type=DELIVERY`, and the shipment, cargo, stop, and both tenants match. `pod_required` is not an input.

## 8. Pin, idempotency, and presence

The document and shipment are chosen in `CreateUploadIntent`, and the object key contains both tenant and shipment. Content and complete must not move the subject.

```text
ACTION_BIND_PIN_TIME=CREATE_UPLOAD_INTENT
POD_ACTION_BINDING_IDEMPOTENCY_SCOPE=shipment_tenant_id+execution_action_id+idempotency_key
```

Same key and same action return the same intent and document. Same key and a different action conflict. A second complete returns the one binding. It does not insert another row.

```text
BOUND_EVIDENCE_PRESENT_STATE=COMPLETED_UPLOAD_AND_PINNED_FILE
```

Intent `pending` or `uploaded`, a document row alone, a signature, or status `SIGNED` is not `EVIDENCE_PRESENT`. `EVIDENCE_PRESENT` is not acceptance and not satisfaction.

```text
DOCUMENT_EXISTS_IMPLIES_REQUIREMENT=NO
POD_UPLOAD_WRITES_TMS_REQUIREMENT=NO
TMS_REQUIREMENT_WRITES_DOCUMENT_EVIDENCE=NO
CROSS_DOMAIN_WRITE_REQUIRED=NO
POD_ACCEPTANCE_POLICY_EVALUATED_IN_THIS_STAGE=NO
POD_EVIDENCE_AUTO_INHERIT_TO_SUCCESSOR=NO
HISTORICAL_SHIPMENT_POD_AUTO_BACKFILL=NO
```

TMS 0.4C already freezes `RETURN_REDIRECT_AUTO_INHERITANCE=NO`. A return, redirect, or successor delivery action does not receive the original action's binding.

Historical rows with `related_entity_type=SHIPMENT` stay unbound. No backfill is authorized.

## 9. Future read

`GetPODActionEvidence(operating_tenant_id, execution_id, execution_action_id)` is an internal read. It returns zero, one, or many evidence identities. Many rows are returned as a list. This stage does not pick a winner and does not return `pod_satisfied`.

A verified action with no completed binding returns an empty list. That empty list is not `POD_REQUIRED=false` and not `POD_SATISFIED=false`. A store outage is an error, not an empty list. An unknown action, a foreign execution, or a foreign tenant is one not-found result and includes no document id.

## 10. Failure cases

All `FUTURE_REQUIRED`. None are implemented here.

| Id | Case | Result |
| --- | --- | --- |
| `POD_BIND_01` | Verified `DELIVERY` action, pinned complete file | One binding for that action |
| `POD_BIND_02` | `PICKUP` | Reject. No binding |
| `POD_BIND_03` | Unknown execution | Not-found. No document id |
| `POD_BIND_04` | Unknown action | Not-found. No document id |
| `POD_BIND_05` | Action from another execution | Reject. No binding |
| `POD_BIND_06` | Action shipment differs from evidence shipment | Reject. No binding |
| `POD_BIND_07` | Foreign execution tenant | Not-found. No document id |
| `POD_BIND_08` | Foreign shipment tenant | Not-found. No document id |
| `POD_BIND_09` | Driver is not `execution.driver_id` for that operating tenant | Reject. No binding |
| `POD_BIND_10` | Same idempotency key and same action | Same intent and document |
| `POD_BIND_11` | Same key and a different action | Conflict. No second binding |
| `POD_BIND_12` | Complete retry | One binding |
| `POD_BIND_13` | Two delivery actions on one shipment | Independent bindings |
| `POD_BIND_14` | Successor, return, or redirect action | Does not inherit the original binding |
| `POD_BIND_15` | Historical shipment-only POD | Remains unbound |
| `POD_BIND_16` | Evidence exists and requirement is `UNKNOWN` | Evidence present. Requirement unchanged |
| `POD_BIND_17` | Evidence exists and requirement is `KNOWN` not required | Evidence present. Requirement unchanged |
| `POD_BIND_18` | `KNOWN` required and no action evidence | No evidence row. Not a requirement write |
| `POD_BIND_19` | Binding store unavailable | Error. Not an empty evidence list |
| `POD_BIND_20` | Cross-tenant read | Not-found. No document id |

## 11. Blockers

```text
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
IMPLEMENTATION_AUTHORIZED=NO
```

`READY_FOR_POD_EVIDENCE_BINDING_RUNTIME=YES` means a later controller may authorize an implementation of this contract. This discovery does not authorize that implementation. Satisfaction still needs the TMS decision, this action evidence, and a pinned acceptance policy. Acceptance policy is not defined here.
