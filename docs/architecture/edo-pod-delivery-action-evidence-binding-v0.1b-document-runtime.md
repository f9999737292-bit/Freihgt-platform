# EDO POD delivery-action evidence binding 0.1B document runtime

Document-service foundation for the frozen contract in [edo-pod-delivery-action-evidence-binding-v0.1a-freeze.md](edo-pod-delivery-action-evidence-binding-v0.1a-freeze.md).

This stage does not implement shipment-service proxying, gateway routing, the driver app, POD acceptance, or POD satisfaction.

```text
POD_EVIDENCE_CANONICAL_SUBJECT=DELIVERY_ACTION
POD_DOCUMENT_TENANT_POLICY=OPERATING_CARRIER_TENANT
POD_DOCUMENT_OWNER_COMPANY_SOURCE=transport.transport_executions.carrier_company_id
POD_EVIDENCE_AUTHOR_COMPANY_SOURCE=transport.drivers.carrier_company_id
DOCUMENT_TENANT_EQUALS_SHIPMENT_TENANT=NO
DOCUMENT_TENANT_EQUALS_OPERATING_TENANT=YES
POD_ACTION_EVIDENCE_INTERNAL_AUTH=REQUIRED
POD_ACTION_CALLER_CREDENTIAL_SCOPE=SHIPMENT_SERVICE_ONLY
DOCUMENT_SERVICE_DIRECT_TRANSPORT_SQL=NO
TMS_SUBJECT_AUTHORITY=shipment-service
DOCUMENT_SERVICE_TRUSTED_SUBJECT_ONLY=YES
DIRECT_BROWSER_TO_DOCUMENT_SERVICE_ALLOWED=NO
CLIENT_SUPPLIED_TENANT_AUTHORITY=NO
IDEMPOTENCY_KEY_REQUIRED=YES
POD_ACTION_BINDING_IDEMPOTENCY_SCOPE=document_tenant_id+execution_action_id+idempotency_key
COMPLETE_BINDING_ATOMIC=YES
DOCUMENT_EXISTS_IMPLIES_REQUIREMENT=NO
POD_UPLOAD_WRITES_TMS_REQUIREMENT=NO
TMS_REQUIREMENT_WRITES_DOCUMENT_EVIDENCE=NO
POD_ACCEPTANCE_POLICY_EVALUATED=NO
POD_SATISFACTION_RUNTIME=NO
POD_EVIDENCE_AUTO_INHERIT_TO_SUCCESSOR=NO
HISTORICAL_SHIPMENT_POD_AUTO_BACKFILL=NO
END_TO_END_POD_ACTION_RUNTIME_READY=NO
```

## Persistence

Migration `000099_edo_pod_delivery_action_evidence_binding_v0_1b`.

| Table | Role |
| --- | --- |
| `documents.pod_delivery_action_upload_intent` | Action-bound upload intent. Pins are immutable after insert |
| `documents.pod_delivery_action_evidence` | Immutable binding written only when complete commits |

`document_tenant_id` equals `operating_tenant_id`. `shipment_tenant_id` stays the canonical shipment owner and may differ. `owner_company_id` is the execution carrier company. `author_driver_id` and `author_company_id` are actor provenance.

One document binds to at most one delivery action. One delivery action may have many POD documents. There is no foreign key from `documents` into `transport`. Historical shipment-only POD rows are not backfilled.

`documents.pod_delivery_action_evidence` has no mutable lifecycle columns. A database trigger rejects every `UPDATE` of an evidence row. A correction is a new POD document and a new evidence row.

The file row written at complete stores the server-configured object placement. `storage_provider` is `local` for the local object store and `s3` for the S3 object store. `bucket_name` is the configured S3 bucket, and null for the local store. The provider is not inferred from the object key.

The legacy `documents.document_upload_intent` idempotency scope (`tenant_id + driver_id + idempotency_key`) is unchanged.

Idempotency key is required. The runtime unique key is `(document_tenant_id, idempotency_key)`, which also enforces the freeze triple `(document_tenant_id, execution_action_id, idempotency_key)`. The same key with a different action, or the same action with a different execution, stop, shipment, shipment tenant, cargo, owner company, or author pin, is a conflict and does not insert another intent.

## Internal API

Authentication is `packages/shared-go/internalauth` with the dedicated server token `POD_ACTION_EVIDENCE_SHIPMENT_SERVICE_TOKEN`, plus `X-Internal-Service-Name=shipment-service`. An empty dedicated token fails closed. The shared `INTERNAL_SERVICE_TOKEN` does not authorize these routes. A dedicated token presented with another caller name is rejected. `X-Tenant-ID` is not authentication and is not the tenant source.

| Method | Route |
| --- | --- |
| POST | `/internal/v1/pod-action-evidence/upload-intents` |
| PUT | `/internal/v1/pod-action-evidence/upload-intents/{uploadId}/content` |
| POST | `/internal/v1/pod-action-evidence/upload-intents/{uploadId}/complete` |
| GET | `/internal/v1/pod-action-evidence/{operatingTenantId}/{executionId}/{executionActionId}` |

Create accepts the authenticated shipment-service subject command and pins it. document-service does not read `transport.*` to rediscover execution, action, stop, shipment, shipment tenant, cargo, carrier, driver, or actor provenance. It validates that every required id is present, `actionType` is `DELIVERY`, and `documentTenantId` equals `operatingTenantId`. It checks `CompanyExists(owner_company_id, document_tenant_id)` and the idempotency pin. It does not call `ShipmentExists(shipment_id, document_tenant_id)` and does not change `DocumentService.Create`.

Content and complete resolve the intent by upload id and keep that pin. A subject value supplied again is checked against the pin and cannot re-home the intent. The object key stays the server-derived pin. Content upload does not overwrite an existing object. A retry after a successful upload returns success when that object is present. If the object was stored and the intent is still pending, the same intent is marked uploaded from the stored object size.

A replay of an intent that is not completed returns the same upload and document ids with a fresh upload token. Only `upload_token_hash` changes. A completed replay returns the completed identity and does not create a second document or a new upload token.

The read returns evidence for the scope shipment-service already verified: none, one, or many. A verified action with no evidence is an empty list. Unknown or foreign actions are not found by shipment-service before it calls document-service. A repository error is an error, not an empty list.

## Evidence presence

Present evidence requires a binding row, intent status `completed`, the pinned `document_file_id`, and a document whose `deleted_at` is null. A document row, a pending intent, an uploaded object that is not completed, and status `SIGNED` are not presence. This service does not read or write `transport.transport_execution_pod_requirement_decisions`.

## Transaction boundary

`COMPLETE_BINDING_ATOMIC=YES` for the database boundary inside `PODActionEvidenceRepository.CompleteIntent`.

One transaction locks the intent and then inserts `documents.document_files`, inserts `documents.pod_delivery_action_evidence`, and sets the intent to `completed`. A commit failure rolls those three writes back together. Completing an already completed intent returns the existing evidence id and does not insert a second binding.

The object bytes are stored earlier, at content upload. That object is not evidence until the completion transaction commits. This stage does not compensate a failed commit by deleting the object or by writing a second binding.

## Remaining boundary

shipment-service verifies the operating tenant, execution, delivery action, stop, shipment, shipment tenant, cargo, carrier company, driver assignment, and actor provenance, then calls these routes with that trusted subject. The gateway and the driver app do not call document-service for this binding. POD acceptance and POD satisfaction stay unimplemented.
