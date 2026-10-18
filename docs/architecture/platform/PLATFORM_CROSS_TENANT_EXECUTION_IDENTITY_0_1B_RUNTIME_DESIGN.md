# BINTRANS PLATFORM / IDENTITY

## CROSS-TENANT EXECUTION IDENTITY 0.1B RUNTIME DESIGN

```text
AGENT=A
STAGE=PLATFORM-CROSS-TENANT-EXECUTION-IDENTITY-0.1B-RUNTIME-DESIGN
BASE_SHA=3eaa9ef621406730e3d6335a322271abde37b269
MAIN_MIGRATION_HEAD=000099
WORKTREE=D:/Projects/freight-platform-wt/platform-cross-tenant-execution-identity-runtime-v0.1b
BRANCH=design/platform-cross-tenant-execution-identity-runtime-v0.1b
DOCS_ONLY=YES
IMPLEMENTATION_AUTHORIZED=NO
MIGRATION_AUTHORIZED=NO
STAGING_AUTHORIZED=NO
CONTRACT_CONTRADICTION_FOUND=NO
IDENTITY_BRIDGE_DECISION=HYBRID
READY_FOR_CONTROLLER_RUNTIME_IMPLEMENTATION_AUTHORIZATION=YES
READY_FOR_NLO_0_9D_STATUS_INTEGRATION=NO
READY_FOR_NLO_0_9D_RUNTIME=NO
```

This design turns the accepted 0.1A contract into an implementation-ready runtime shape. It does not authorize implementation, a migration, OpenAPI, gateway routes, or staging.

Authoritative inputs:

- `docs/architecture/platform/PLATFORM_CROSS_TENANT_EXECUTION_IDENTITY_0_1A_DISCOVERY.md`
- `docs/architecture/platform/PLATFORM_CROSS_TENANT_EXECUTION_IDENTITY_0_1A_FREEZE.md`
- `docs/architecture/tms/TMS_NLO_CHAIN_MULTISHIPMENT_STATUS_POLICY_0_1A_FREEZE.md`

`CONTRACT_CONTRADICTION_FOUND=NO`. The 0.1A freeze is not rewritten. The multi-shipment pin section below realizes the frozen pin owner as child rows of the execution revision, because one scalar pair cannot represent shipments `S1..Sn`. Owner, phase, immutability, and historical-read rules stay as frozen.

## Preserved 0.1A decisions

```text
IDENTITY_BRIDGE_DECISION=HYBRID
CANONICAL_EXECUTION_CARRIER_OWNER=OPERATING_TENANT_COMPANY
SHIPMENT_CARRIER_REFERENCE_MODEL=TENANT_LOCAL_COMPANY_REFERENCE
CARRIER_BRIDGE_AUTHORITY=SERVER_OWNED_EXPLICIT_RELATIONSHIP
CARRIER_BRIDGE_CREATION_COMMAND=CreateCarrierOperatingRelationshipRequest
CARRIER_BRIDGE_ACCEPTANCE_COMMAND=AcceptCarrierOperatingRelationshipRequest
TWO_SIDED_RELATIONSHIP_ACCEPTANCE_REQUIRED=YES
RELATIONSHIP_CREATED_ONLY_AFTER_BOTH_SIDES=YES
OPERATING_ACCEPTANCE_REQUIRED_ROLE=CARRIER_ADMIN|PLATFORM_ADMIN
ORDINARY_ACTIVE_MEMBER_CAN_ACCEPT_RELATIONSHIP=NO
ACCEPTANCE_TOKEN_REQUIRED=YES
RELATIONSHIP_REQUEST_ID_IS_ACCEPTANCE_AUTHORITY=NO
RELATIONSHIP_REQUEST_ID_BODY_POLICY=OPTIONAL_NON_AUTHORITATIVE_LOCATOR
TOKEN_AND_REQUEST_ID_MUST_MATCH=YES
TOKEN_MISSING_POLICY=REFUSE_NO_RELATIONSHIP
TOKEN_REQUEST_ID_MISMATCH_POLICY=REFUSE_NO_RELATIONSHIP
CANONICAL_EXECUTION_DRIVER_OWNER=OPERATING_EXECUTION
CANONICAL_EXECUTION_VEHICLE_OWNER=OPERATING_EXECUTION
CROSS_TENANT_DRIVER_BRIDGE_REQUIRED=NO
CROSS_TENANT_VEHICLE_BRIDGE_REQUIRED=NO
SHIPMENT_DRIVER_EQUALITY_REQUIRED_FOR_EXECUTION=NO
SHIPMENT_VEHICLE_EQUALITY_REQUIRED_FOR_EXECUTION=NO
CARRIER_RELATIONSHIP_LIFECYCLE_MODEL=IMMUTABLE_APPEND_ONLY_VERSIONS
CARRIER_RELATIONSHIP_RESOLUTION_PHASE=ACTIVATE_BEFORE_TMS_PROJECTION
CARRIER_RELATIONSHIP_PIN_OWNER=TRANSPORT_EXECUTION_REVISION
HISTORICAL_AUTO_MATCH_ALLOWED=NO
COMPANY_SERVICE_DIRECT_TRANSPORT_AUTHORITY=NO
COMPANY_SERVICE_DIRECT_RFX_AUTHORITY=NO
```

## 1. Current runtime audit

First observed at `3eaa9ef621406730e3d6335a322271abde37b269`. Reconciled with `origin/main` `e19a118c5fda7a2afe5794f33e5caf8e83b481a0` after merged PR #283. Migration head is still `000099_edo_pod_delivery_action_evidence_binding_v0_1b.up.sql`. PR #283 did not add a migration.

PR #283 is the identity authority that later stages must follow:

- Every identity user route and role route requires a verified bearer JWT. Anonymous user creation is not available. No environment flag opens those routes.
- Verified user id and verified tenant id come from that JWT. `X-Tenant-ID`, `X-User-ID`, `X-User-Roles`, `X-Platform-Admin`, query `tenant_id`, and body `tenant_id` are not authority.
- A supplied tenant that differs from the verified tenant is `TENANT_CONTEXT_CONFLICT` unless a server-verified platform admin names an existing target tenant. That override is audited.
- `LookupPrincipal` returns tenant-global roles only: `core.user_roles.company_id IS NULL`, the user row exists in the verified tenant, `users.deleted_at IS NULL`, and `users.status = ACTIVE`. A surviving role does not grant authority for a blocked or soft-deleted user.
- `LookupPrincipal` is not proof of a company-scoped role. `CARRIER_ADMIN` on a company is invisible to that lookup.
- Gateway company-context membership lookup forwards the caller bearer. The requested user id must be the authenticated subject unless the caller is a verified platform admin. A generic internal token and `X-Internal-Service-Name` are not that lookup's credential.

Company persistence is `core.companies`, one `tenant_id` per row. Membership is `core.company_memberships` with active status and `deleted_at`. Company-scoped roles are `core.user_roles.company_id`. Tenant-global `PLATFORM_ADMIN` is the row set where `company_id IS NULL`, read by `ListUserGlobalRoleCodes`. `CARRIER_ADMIN` is a seeded system role. `CompanyAdminRoles` also contains `SHIPPER_ADMIN` and `FORWARDER_MANAGER`. Those are not acceptance authority. Company-service has no relationship tables and no outbox.

Shipment-service owns `transport.shipments` (`carrier_company_id`, `version`, tenant-local `driver_id` and `vehicle_id`). `CreateExecutionProjectionFromActivation` inserts `transport.transport_executions` and `transport.transport_execution_revisions` in one transaction. `insertExecution` writes `vehicle_id` and `driver_id` from the command without the five operating-carrier predicates. Participants are one row per shipment subject. Active shipment uniqueness is `transport.transport_execution_active_shipments`. Revision insert records `source_activation_id`. A repeated activation conflicts on `transport_execution_revisions_activation_uq`. NLO is the only projection caller, and only after the internal token check.

RFx award is `rfx.rfx_awards` (`tenant_id`, `carrier_company_id`, `version`, unique per event). The award-to-order link is `rfx.rfx_award_transport_orders` (`tenant_id`, `carrier_company_id`, `version`). `AwardResponse` and `ConvertAwardToTransportOrders` both call `requireBuyerEventAccess`. That gate is a buyer actor, `HasBuyerRole`, and an active buyer membership that contains the event owner company. `HasBuyerRole` accepts `PLATFORM_ADMIN`, `PROCUREMENT_MANAGER`, `SHIPPER_ADMIN`, `SHIPPER_LOGIST`, and `FORWARDER_MANAGER`. `ListUserRoleCodes` in rfx-service is tenant-wide. It does not filter `company_id`.

Gateway public company routes are `/api/v1/companies` and the member routes. `StripUntrustedCompanyHeaders` removes client `X-Company-ID` before the gateway sets that header from a verified membership. `X-Internal-Service-Token` is stripped before downstream proxying.

`packages/shared-go/internalauth` compares one configured `X-Internal-Service-Token` in constant time. An empty expected token fails closed. The service name is not part of that credential. `X-Internal-Service-Name` is a separate header. On the execution projection route it is checked only after the shared token matches. Any holder of the shared token can present another service name. That is the current capability. It is not the future caller model.

Local and staging Compose point company-service, shipment-service, and rfx-service at one PostgreSQL database, `freight_platform`, with schemas `core`, `transport`, and `rfx`. A same-database authorization function is deployment-compatible. Company-service still must not write `transport.*` or `rfx.*`.

Idempotency already exists for priced transport orders: an advisory lock, a unique key, and a request hash, with conflict when the hash differs. Outbox rows are inserted in the same database transaction in tracking and shipment flows. Company-service does not have that table yet. A future company outbox can use the same transaction pattern.

## 2. Service ownership

```text
COMPANY_SERVICE_OWNS=relationship_request_persistence+acceptance_token_hash+relationship_persistence+relationship_versions+request_lifecycle+acceptance+revocation+relationship_audit+relationship_idempotency
SHIPMENT_SERVICE_OWNS=SHIPMENT_source_verification+shipment_tenant_and_carrier_reference+TMS_activation_orchestration+execution_projection+execution_revision+per_shipment_relationship_pins+operating_driver_and_vehicle_verification
RFX_SERVICE_OWNS=RFX_AWARD_source_verification+AWARD_TRANSPORT_ORDER_LINK_source_verification+RFX_source_version_and_provenance
COMPANY_SERVICE_DIRECT_TRANSPORT_AUTHORITY=NO
COMPANY_SERVICE_DIRECT_RFX_AUTHORITY=NO
```

Company-service reaches shipment and RFx state only through the source-owner revalidation commands in section 9. It does not query `transport.*` or `rfx.*` as business authority. Shipment-service does not insert relationship rows. It calls the company-owned authorization function inside its own transaction. That function writes only `company.*`.

## 3. Relationship request persistence

Compared:

- A. Immutable request root plus append-only events plus a constrained head.
- B. Immutable version rows plus a stable request head.
- C. A single mutable request row. Rejected. It edits historical proof.

Selected:

```text
REQUEST_STORAGE_MODEL=IMMUTABLE_REQUEST_ROOT_PLUS_APPEND_ONLY_EVENTS_PLUS_CONSTRAINED_HEAD
REQUEST_ROOT_IMMUTABLE=YES
REQUEST_EVENTS_APPEND_ONLY=YES
REQUEST_CURRENT_STATE_SOURCE=REQUEST_HEAD_ROW
REQUEST_STATUS_DATABASE_ENFORCED=YES
REQUEST_ACCEPTED_ONLY_ONCE=YES
REQUEST_EXPIRY_CLOCK_SOURCE=DATABASE_CLOCK
```

Future tables, not created in this stage:

`company.carrier_relationship_requests` is insert-only. Columns: `relationship_request_id`, `source_type`, `source_id`, `source_tenant_id`, `source_version_or_fingerprint`, `shipper_carrier_company_id`, `requesting_company_id`, `requesting_user_id`, `acceptance_token_hash`, `token_created_at`, `token_expires_at`, `created_at`, `created_by`. A trigger rejects `UPDATE` and `DELETE`.

`company.carrier_relationship_request_events` is insert-only. Columns: `event_id`, `relationship_request_id`, `request_version`, `event_type`, `request_status`, `occurred_at`, `actor_user_id`, `cancelled_at`, `cancelled_by`, `cancel_reason`, `accepted_at`, `accepted_by`, `accepted_operating_tenant_id`, `accepted_operating_carrier_company_id`, `relationship_id`, `relationship_version`. `request_version` starts at 1 and increases by 1. Unique `(relationship_request_id, request_version)`. Unique partial index on `relationship_request_id` where `event_type = ACCEPTED`. A trigger rejects `UPDATE` and `DELETE`.

`company.carrier_relationship_request_heads` has one row per request: `relationship_request_id`, `current_event_id`, `current_version`, `request_status`. Status check is `PENDING`, `ACCEPTED`, `EXPIRED`, or `CANCELLED`. The head changes only in the transaction that inserts the matching event.

The logical fields required by 0.1A are the union of the root and the latest event. Readers use the head for current status and the events for proof. Expiry uses `clock_timestamp()` inside the acceptance transaction. A later sweeper may insert `EXPIRED` for a still-`PENDING` head whose `token_expires_at` has passed. It takes the same head lock as acceptance.

## 4. Acceptance token security

```text
TOKEN_HASH_ALGORITHM=HMAC-SHA-256
TOKEN_SERVER_PEPPER_REQUIRED=YES
TOKEN_PEPPER_OWNER=COMPANY_SERVICE
TOKEN_PEPPER_STORAGE=OUTSIDE_DATABASE_SECRET_STORE
TOKEN_PEPPER_IN_AUDIT_LOGS_OUTBOX=NO
TOKEN_PEPPER_ROTATION_POLICY=INVALIDATE_UNACCEPTED_TOKENS
TOKEN_SILENT_REHASH_ALLOWED=NO
TOKEN_PRESENTATION_CHANNEL=HTTPS_JSON_BODY_ONLY
TOKEN_TTL=72H
TOKEN_ROTATION_ALLOWED=NO
TOKEN_REISSUE_POLICY=NO_REISSUE_CANCEL_AND_CREATE_NEW_REQUEST
TOKEN_INVALID_RESPONSE_POLICY=GENERIC_NOT_FOUND_NO_EXISTENCE_LEAK
TOKEN_LOGGING_POLICY=NEVER
ACCEPTANCE_TOKEN_REQUIRED=YES
RELATIONSHIP_REQUEST_ID_BODY_POLICY=OPTIONAL_NON_AUTHORITATIVE_LOCATOR
TOKEN_AND_REQUEST_ID_MUST_MATCH=YES
TOKEN_MISSING_POLICY=REFUSE_NO_RELATIONSHIP
TOKEN_REQUEST_ID_MISMATCH_POLICY=REFUSE_NO_RELATIONSHIP
```

The token is 32 bytes from a cryptographic random source, 256 bits, returned once in the create-request response to the source-owning service. Only `HMAC-SHA-256(server_pepper, token)` is stored. Comparison uses a constant-time compare of the computed HMAC and the stored HMAC. Company-service owns the pepper. The pepper lives in a secret store outside the database. It is absent from logs, traces, audit, outbox, query strings, error details, and source control. Rotation invalidates every not-yet-accepted token: those presentations fail as the generic not-found response. Stored hashes are not silently recomputed with the new pepper. A replacement for an invalidated pending request is cancel plus a new request and a new token. Replay of an already accepted request continues to use the acceptance idempotency record and does not require the old pepper to match.

Invalid attempts are counted per authenticated user and operating tenant. The response stays the generic not-found response. The counter does not lock or reveal the request. A missing token, a bad token, a mismatched locator, and an unknown request share that response.

`token_expires_at = token_created_at + 72 hours`, both taken from the database clock. There is no rotation and no reissue. A replacement is a cancelled request plus a new request and a new token.

## 5. Request creation contract

Public callers do not send raw source ids to company-service. The source owner verifies the actor and the row, then sends this trusted subject:

```text
source_type
source_id
source_tenant_id
source_version_or_fingerprint
shipper_carrier_company_id
requesting_user_id
requesting_company_id
authorization_proof_context
idempotency_key
```

`requesting_company_id` is authoritative only when the source owner proved an active membership and the company-scoped role on that company, or a tenant-global `PLATFORM_ADMIN` binding (`core.user_roles.company_id IS NULL`) in the source tenant. A body company id is not authority.

```text
SHIPMENT_REQUEST_CREATOR_AUTHORIZATION=VERIFIED_ACTIVE_USER AND SHIPMENT_TENANT_ACTOR AND ACTIVE_MEMBERSHIP_ON_SHIPMENT_SHIPPER_COMPANY AND (COMPANY_SCOPED_ROLE(SHIPPER_ADMIN OR SHIPPER_LOGIST OR FORWARDER_MANAGER) OR TENANT_GLOBAL_PLATFORM_ADMIN) AND NON_NULL_SHIPMENT_CARRIER_IN_SAME_TENANT
RFX_AWARD_REQUEST_CREATOR_AUTHORIZATION=VERIFIED_ACTIVE_USER AND BUYER_EVENT_ACCESS AND ACTIVE_MEMBERSHIP_ON_EVENT_OWNER_COMPANY AND (COMPANY_SCOPED_ROLE(PROCUREMENT_MANAGER OR SHIPPER_ADMIN OR SHIPPER_LOGIST OR FORWARDER_MANAGER) OR TENANT_GLOBAL_PLATFORM_ADMIN) AND AWARD_CARRIER_IN_SAME_TENANT
AWARD_LINK_REQUEST_CREATOR_AUTHORIZATION=VERIFIED_ACTIVE_USER AND BUYER_EVENT_ACCESS AND ACTIVE_MEMBERSHIP_ON_EVENT_OWNER_COMPANY AND (COMPANY_SCOPED_ROLE(PROCUREMENT_MANAGER OR SHIPPER_ADMIN OR SHIPPER_LOGIST OR FORWARDER_MANAGER) OR TENANT_GLOBAL_PLATFORM_ADMIN) AND LINK_CARRIER_IN_SAME_TENANT
```

Those role codes are the codes already seeded and already accepted by shipment create policy and `HasBuyerRole`. The new command binds them to the source company. No new role code is introduced. `CARRIER_ADMIN` and `CARRIER_DISPATCHER` do not create the shipper-side request.

Every create, accept, cancel, and revoke decision uses the same user predicate. The user row exists in `core.users`. Its `tenant_id` equals the verified tenant. `deleted_at IS NULL`. `status = ACTIVE`. The role row is not deleted. A company-scoped role counts only when `core.user_roles.company_id` is the selected or source company being proved. Tenant-global `PLATFORM_ADMIN` counts only when `company_id IS NULL`. `PLATFORM_ADMIN` replaces the role check only. It does not replace membership, the selected company, or the user predicate.

`rfx-service` `ListUserRoleCodes` is tenant-wide and does not filter `company_id`. That query is not company-scoped proof. Identity `LookupPrincipal` is not company-scoped proof either, because it returns only tenant-global roles. A future implementation must not call either one to decide `CARRIER_ADMIN`, `SHIPPER_ADMIN`, `SHIPPER_LOGIST`, `FORWARDER_MANAGER`, or `PROCUREMENT_MANAGER`.

```text
SOURCE_VERSION_SOURCE_BY_TYPE=SHIPMENT:transport.shipments.version|RFX_AWARD:rfx.rfx_awards.version|AWARD_TRANSPORT_ORDER_LINK:rfx.rfx_award_transport_orders.version
SOURCE_FINGERPRINT_FIELDS_BY_TYPE=SHIPMENT:tenant_id+id+version+shipper_company_id+carrier_company_id+deleted_at_null|RFX_AWARD:tenant_id+id+version+rfx_event_id+carrier_company_id|AWARD_TRANSPORT_ORDER_LINK:tenant_id+id+version+rfx_award_id+transport_order_id+carrier_company_id
SOURCE_STALE_REVALIDATION_MODEL=SOURCE_OWNER_COMPARES_STORED_FINGERPRINT_AT_ACCEPTANCE
```

## 6. Request creation idempotency

Creation is a different namespace from acceptance.

```text
REQUEST_CREATION_IDEMPOTENCY_SCOPE=caller_service+source_tenant_id+requesting_company_id+idempotency_key
REQUEST_CREATE_REPLAY_POLICY=SAME_SCOPE_SAME_REQUEST_HASH_RETURNS_SAME_REQUEST_WITHOUT_A_NEW_TOKEN
REQUEST_CREATE_CONFLICT_POLICY=SAME_SCOPE_DIFFERENT_HASH_RETURNS_IDEMPOTENCY_CONFLICT
EXISTING_ACTIVE_RELATIONSHIP_REQUEST_POLICY=ALLOW_NEW_PENDING_REQUEST_OPERATING_TENANT_UNKNOWN
EXISTING_PENDING_REQUEST_POLICY=DIFFERENT_KEY_FOR_SAME_SOURCE_AND_CARRIER_CONFLICTS
EXISTING_EXPIRED_REQUEST_POLICY=NEW_KEY_MAY_CREATE_NEW_REQUEST_SAME_KEY_REPLAYS_EXPIRED_REQUEST
EXISTING_CANCELLED_REQUEST_POLICY=NEW_KEY_MAY_CREATE_NEW_REQUEST_SAME_KEY_REPLAYS_CANCELLED_REQUEST
```

The stored hash covers `source_type`, `source_id`, `source_version_or_fingerprint`, and `shipper_carrier_company_id`. A replay does not mint a second token. The first response is the only token disclosure. A lost response is recovered by an operator path that cancels the pending request and creates a new one. The stored hash is not reversible to the token.

An `ACTIVE` relationship does not block request creation. The operating tenant is chosen only at acceptance, so company-service cannot yet know which business key the request will hit.

## 7. Internal service authentication

The current shared token plus `X-Internal-Service-Name` is not the future model. The header name is caller-controlled. Compared: one token per caller, a signed service JWT, and mTLS. The repository today implements a single shared token in `internalauth`. Dedicated tokens are the smallest compatible change. JWT and mTLS are not the current credential.

```text
INTERNAL_CALLER_AUTH_MODEL=DEDICATED_TOKEN_PER_CALLER_SERVICE
CALLER_NAME_CREDENTIAL_BOUND=YES
GENERIC_SHARED_TOKEN_SPOOF_ALLOWED=NO
SHIPMENT_SERVICE_CREDENTIAL_SCOPE=CREATE_AND_VALIDATE_SHIPMENT_REQUESTS+CALL_ACTIVATION_AUTHORIZATION_FUNCTION
RFX_SERVICE_CREDENTIAL_SCOPE=CREATE_AND_VALIDATE_RFX_AWARD_AND_AWARD_TRANSPORT_ORDER_LINK_REQUESTS
RELATIONSHIP_RESOLVER_CREDENTIAL_SCOPE=SHIPMENT_SERVICE_DATABASE_ROLE_EXECUTE_ON_AUTHORIZE_FUNCTION_ONLY
EMPTY_CREDENTIAL_POLICY=FAIL_CLOSED
INVALID_SERVICE_CREDENTIAL_POLICY=FAIL_CLOSED
DIRECT_COMPANY_SERVICE_CALL_WITHOUT_VERIFIED_CREDENTIAL=FAIL_CLOSED
COMPANY_SERVICE_REVALIDATION_CREDENTIAL_SCOPE=READ_SOURCE_FINGERPRINT_FROM_OWNING_SERVICE_ONLY
```

The presented token maps to exactly one caller. `X-Internal-Service-Name` is ignored. A shipment credential cannot create an RFx request. An RFx credential cannot resolve an activation. An empty, unknown, or invalid service credential fails closed. A direct company-service call without a verified credential fails closed. The internal create route does not accept a caller JWT or `X-Company-ID` as authority. It accepts only the dedicated caller credential, and the source owner must already have applied the user predicate before sending `requesting_user_id`.

## 8. Public acceptance contract

```text
ACCEPTANCE_PUBLIC_ROUTE=/api/v1/carrier-operating-relationship-acceptances
ACCEPTANCE_HTTP_METHOD=POST
SELECTED_COMPANY_CONTEXT_SOURCE=SERVER_VERIFIED_MEMBERSHIP_FOR_JWT_USER
PUBLIC_ACCEPTANCE_IDENTITY_SOURCE=VERIFIED_JWT
PUBLIC_ACCEPTANCE_TENANT_SOURCE=VERIFIED_JWT
MEMBERSHIP_AND_AUTHORITY_SERVER_CHECKED=YES
X_COMPANY_ID_ROLE=NON_AUTHORITATIVE_SELECTOR
X_COMPANY_ID_MISMATCH_POLICY=REFUSE
CLIENT_COMPANY_HEADER_AUTHORITY=NO
ACCEPTANCE_REQUEST_BODY=acceptance_token+idempotency_key+optional_relationship_request_id
ACCEPTANCE_SUCCESS_RESPONSE=201_relationship_id+relationship_version+request_status_ACCEPTED
ACCEPTANCE_REPLAY_RESPONSE=200_SAME_relationship_id+relationship_version
```

Public acceptance is a direct company-service operation only when the caller presents a verified credential. For this route the credential is the user bearer JWT. Company-service derives the user id and the operating tenant from that JWT. It does not derive them from `X-User-ID`, `X-Tenant-ID`, `X-User-Roles`, or `X-Platform-Admin`.

`X-Company-ID` is a non-authoritative selector. Company-service loads membership and roles from the server. The selector may be used only when it equals an active membership of the verified user in the verified tenant. A missing selector when several memberships exist, a selector for a company without that membership, or a selector that disagrees with the verified membership is a refusal. A body company id is not authority. An empty or invalid service credential on any internal company-service route is a refusal.

Operating acceptance uses one predicate, all parts required:

```text
OPERATING_ACCEPTANCE_PREDICATE=VERIFIED_ACTIVE_USER AND ACTIVE_MEMBERSHIP_IN_SELECTED_OPERATING_COMPANY AND (COMPANY_SCOPED_CARRIER_ADMIN OR OPERATING_TENANT_GLOBAL_PLATFORM_ADMIN)
```

`VERIFIED_ACTIVE_USER` is the user predicate in section 5. The membership is active on the selected operating company. The role is company-scoped `CARRIER_ADMIN` on that same company, or tenant-global `PLATFORM_ADMIN` in the operating tenant. `PLATFORM_ADMIN` replaces only the role alternative. It does not replace the active user predicate or the active membership. Ordinary active membership does not accept a relationship. The request is `PENDING`, unexpired by the database clock, not cancelled, and the source fingerprint still matches.

```text
ACCEPTANCE_ERROR_MATRIX=MISSING_OR_BAD_OR_MISMATCHED_TOKEN:IDENTICAL_404_GENERIC|FORBIDDEN_ROLE:403|EXPIRED_OR_CANCELLED_OR_STALE:409|DIFFERENT_KEY_AFTER_ACCEPT:RELATIONSHIP_REQUEST_ALREADY_ACCEPTED:409|SAME_KEY_DIFFERENT_BODY_OR_COMPANY:RELATIONSHIP_IDEMPOTENCY_CONFLICT:409|STORE_OR_SOURCE_UNAVAILABLE:503
ACCEPTED_REQUEST_CONFLICT_CODE=RETIRED_USE_RELATIONSHIP_REQUEST_ALREADY_ACCEPTED
TOKEN_MISSING_BAD_AND_MISMATCH_PUBLIC_BODY=IDENTICAL_404
```

Exact replay of the same request, selected company, and idempotency key is the 200 replay, including when the token is already consumed. The idempotency record is read before token reuse is decided. The 0.1A name `ACCEPTED_REQUEST_CONFLICT` means `RELATIONSHIP_REQUEST_ALREADY_ACCEPTED` in this design. A missing token, a token that does not verify, and a token whose optional `relationship_request_id` does not match produce one identical generic 404 body. Those three responses do not reveal whether a request exists.

This stage does not add the gateway route or the OpenAPI operation.

## 9. Source freshness revalidation

Compared: a synchronous callback, a signed proof, and an event-fed projection. A signed proof goes stale between issue and accept. An event projection can lag. Neither is allowed to accept a stale source.

```text
SOURCE_REVALIDATION_MODEL=SYNCHRONOUS_INTERNAL_CALLBACK_TO_SOURCE_OWNER
SOURCE_REVALIDATION_CALLER=COMPANY_SERVICE_WITH_REVALIDATION_CREDENTIAL
SOURCE_REVALIDATION_RESPONSE=FINGERPRINT_MATCH|NOT_FOUND|CHANGED|UNAVAILABLE
SOURCE_OWNER_UNAVAILABLE_POLICY=ERROR_NO_ACCEPTANCE
SOURCE_CHANGED_POLICY=STALE_NO_ACCEPTANCE
```

Shipment-service answers `SHIPMENT`. RFx-service answers `RFX_AWARD` and `AWARD_TRANSPORT_ORDER_LINK`. The callback checks existence, tenant, local carrier company, version, and the fingerprint fields in section 5. Company-service does not accept when the owner is unavailable. There is no stale fallback.

## 10. Acceptance transaction

```text
ACCEPTANCE_TRANSACTION_BOUNDARY=ONE_COMPANY_SCHEMA_DATABASE_TRANSACTION
ACCEPTANCE_LOCK_ORDER=IDEMPOTENCY_KEY_THEN_TOKEN_HASH_THEN_REQUEST_HEAD_THEN_RELATIONSHIP_BUSINESS_KEY
TOKEN_CONSUMPTION_ATOMIC=YES
REQUEST_ACCEPTANCE_ATOMIC=YES
RELATIONSHIP_VERSION_INSERT_ATOMIC=YES
IDEMPOTENCY_RECORD_ATOMIC=YES
OUTBOX_ATOMIC=YES
```

The transaction contains the idempotency check, token verification, pending and expiry and cancellation checks, the already-verified membership and role decision, the source callback result, request acceptance, relationship root reuse or insert, the first or reused `ACTIVE` version, the request-to-relationship link, audit, and the outbox insert. The outbox insert uses the same transaction. That is the pattern already used by shipment and tracking outboxes. No separate blocker remains.

Concurrency:

- Two admins, same request: one transaction holds the head lock. One accepts. The other, for the same company and the same idempotency key, receives the 200 replay. A different idempotency key after acceptance is `RELATIONSHIP_REQUEST_ALREADY_ACCEPTED` (409). The name `ACCEPTED_REQUEST_CONFLICT` is not a code.
- Two operating companies: the first commit wins. The second finds the request accepted for a different company and receives `RELATIONSHIP_REQUEST_ALREADY_ACCEPTED` (409) without a foreign company id.
- Accept versus cancel: the head lock serializes them. One event commits. The other sees the terminal status and refuses with `RELATIONSHIP_REQUEST_CANCELLED` (409) when the winner cancelled.
- Accept versus expiry: the database clock and the head lock decide. An expired head gets `EXPIRED` or the accept refuses with `RELATIONSHIP_REQUEST_EXPIRED` (409). It does not accept.
- Lost response: the same key replays the stored relationship as 200.
- Consumed token and a different key: `RELATIONSHIP_REQUEST_ALREADY_ACCEPTED` (409).
- Same key and a different request, body hash, or company: `RELATIONSHIP_IDEMPOTENCY_CONFLICT` (409).

## 11. Relationship version storage

Compared: immutable root plus append-only versions plus a mutable head, and a current-version flag on version rows. A flag on the version row would require updating a historical row. "No successor" is not a unique constraint.

```text
RELATIONSHIP_STORAGE_MODEL=IMMUTABLE_ROOT_PLUS_APPEND_ONLY_VERSIONS_PLUS_MUTABLE_HEAD
RELATIONSHIP_ROOT_MUTABILITY=IMMUTABLE_ENDPOINTS
RELATIONSHIP_VERSION_IMMUTABLE=YES
CURRENT_HEAD_STORAGE=company.carrier_relationship_heads
ACTIVE_UNIQUENESS_DATABASE_ENFORCED=YES
RELATIONSHIP_VERSION_INCREMENT_ATOMIC=YES
```

`company.carrier_relationships` is insert-only. Unique `(shipper_tenant_id, shipper_carrier_company_id, operating_tenant_id)`. Endpoint columns never change.

`company.carrier_relationship_versions` is insert-only. Columns are the 0.1A version fields, plus `relationship_version` as a monotonic integer and `supersedes_relationship_version_id`. Unique `(relationship_id, relationship_version)`. A trigger rejects `UPDATE` and `DELETE`.

`company.carrier_relationship_heads` has one row per relationship: `current_version`, `current_version_id`, `status`. One root per business key and one head per root is one current version. Zero heads for a lookup is not-found. Two current heads cannot be inserted. A corrupt pair that bypasses the constraints is `DATA_INTEGRITY_ERROR_FAIL_CLOSED` at read time, matching `MULTIPLE_RELATIONSHIP_POLICY`.

Revocation inserts a `REVOKED` version and moves the head. It does not edit the previous version.

## 12. Relationship reuse

The business key is `shipper_tenant_id + shipper_carrier_company_id + operating_tenant_id`. Shipment B with that same key uses the current `ACTIVE` version. The source that first created the relationship stays provenance on the request and on the version that introduced the root. Later sources do not create a second `ACTIVE` head.

```text
RELATIONSHIP_REUSE_SCOPE=SHIPPER_TENANT_ID+SHIPPER_CARRIER_COMPANY_ID+OPERATING_TENANT_ID
SOURCE_IS_PROVENANCE_NOT_RELATIONSHIP_IDENTITY=YES
NEW_SOURCE_REQUIRES_NEW_ACCEPTANCE=NO
DUPLICATE_ACCEPTED_PAIR_POLICY=LINK_REQUEST_TO_CURRENT_ACTIVE_VERSION
```

`NEW_SOURCE_REQUIRES_NEW_ACCEPTANCE=NO` applies while an unsuperseded `ACTIVE` head exists. A `REVOKED` head does not satisfy a later shipment. The next `ACTIVE` version still requires a new two-sided acceptance. That insert does not edit the revoked version.

## 13. Cancellation, expiry, and revocation

```text
PENDING_REQUEST_CANCEL_AUTHORITY=SOURCE_OWNER_CREATOR_AUTHORIZATION|SOURCE_TENANT_GLOBAL_PLATFORM_ADMIN
ACTIVE_RELATIONSHIP_REVOKE_AUTHORITY=SHIPPER_SOURCE_CREATOR_AUTHORIZATION|SHIPPER_TENANT_GLOBAL_PLATFORM_ADMIN|OPERATING_SELECTED_COMPANY_CARRIER_ADMIN|OPERATING_TENANT_GLOBAL_PLATFORM_ADMIN
REVOKE_REQUIRES_SELECTED_COMPANY=YES
REVOKE_IDEMPOTENCY_SCOPE=relationship_id+acting_tenant_id+selected_company_id+idempotency_key
REVOKE_TRANSACTION_BOUNDARY=LOCK_HEAD_INSERT_REVOKED_VERSION_MOVE_HEAD_INSERT_AUDIT_AND_OUTBOX
REACTIVATION_POLICY=NEW_ACTIVE_VERSION_AFTER_NEW_TWO_SIDED_ACCEPTANCE
REVOKED_RELATIONSHIP_REUSE_POLICY=REUSE_SAME_ROOT_INSERT_NEXT_VERSION
```

Company-scoped revoke and cancel require the server-verified selected company to be the endpoint company the actor represents. The actor still passes the user predicate in section 5. Tenant-global `PLATFORM_ADMIN` replaces only that role check, in the actor's own tenant. It does not replace membership or the selected company. `GOV_INSPECTOR` is not a mutation role. No hard delete. Expiry is a database-clock event, not an actor command.

## 14. Activation consistency

A plain HTTP read of `ACTIVE`, followed later by a TMS commit, can revoke between the two steps. That sequence is forbidden.

Compared:

- A durable lease still has a window after the HTTP response.
- Prepare then commit is atomic only if the TMS transaction locks and rechecks.
- A company-owned function inside the TMS transaction is atomic because both services use `freight_platform`.

Selected:

```text
ACTIVATION_CONSISTENCY_MODEL=SAME_DATABASE_COMPANY_OWNED_AUTHORIZATION_FUNCTION
ACTIVATION_AUTHORIZATION_POINT=INSIDE_TMS_PROJECTION_TRANSACTION_BEFORE_EXECUTION_INSERT
ACTIVE_CHECK_AND_PIN_GUARANTEE=LOCK_RELATIONSHIP_HEADS_CHECK_ACTIVE_INSERT_AUTHORIZATION_IN_SAME_TRANSACTION
ACTIVATION_AUTHORIZATION_ID=company.carrier_relationship_activation_authorizations.id
ACTIVATION_AUTHORIZATION_TTL=TRANSACTION_SCOPE
ACTIVATION_AUTHORIZATION_LOOKUP_KEY=source_activation_id+operating_tenant_id+operating_carrier_company_id
ACTIVATION_STORED_PIN_SET_FINGERPRINT=REQUIRED
ACTIVATION_RETRY_SAME_FINGERPRINT=RETURN_EXISTING_EXECUTION
ACTIVATION_RETRY_DIFFERENT_FINGERPRINT=HTTP_409_RELATIONSHIP_ACTIVATION_FINGERPRINT_CONFLICT
ACTIVATION_FINGERPRINT_CONFLICT_SECOND_EXECUTION=NO
ACTIVATION_FINGERPRINT_CONFLICT_PIN_MUTATION=NO
ACTIVATION_ATOMICITY_DATABASE=freight_platform
SPLIT_DATABASE_PLACEMENT=UNSUPPORTED_THIS_VERSION
SPLIT_DATABASE_ACTIVATION_POLICY=FAIL_CLOSED_NO_HTTP_READ_THEN_COMMIT
REVOCATION_DURING_PREPARED_ACTIVATION_POLICY=NO_DURABLE_PREPARED_ROW_REVOCATION_WAITS_ON_HEAD_LOCK
ACTIVATION_CRASH_RECOVERY_POLICY=ROLLBACK_LEAVES_NO_AUTHORIZATION_RETRY_WHOLE_ACTIVATION
ACTIVATION_RETRY_POLICY=SAME_LOOKUP_KEY_AND_SAME_FINGERPRINT_RETURNS_EXISTING_EXECUTION
HTTP_READ_THEN_COMMIT_ALLOWED=NO
```

`company.authorize_carrier_relationship_activation` is `SECURITY DEFINER` and writes only `company.*`. The shipment-service database role receives `EXECUTE` on that function and no insert privilege on relationship tables. The function locks heads in business-key order, requires one `ACTIVE` head per subject, requires one operating company across the set, and inserts the authorization row keyed by `source_activation_id` together with `pin_set_fingerprint`. Shipment-service then inserts the execution, the revision, the participants, and the pin rows. One commit covers all of it. That commit exists only because company-service and shipment-service use the same PostgreSQL database, `freight_platform`. A split-database placement is unsupported in this version. Activation then fails closed. An HTTP read of relationship state followed by a later commit is still forbidden.

There is no durable prepared row. A crash before commit rolls the authorization back, so revocation is not blocked. A committed execution keeps its pins. Revocation after commit inserts a new relationship version and does not update those pins.

Replay looks up `source_activation_id + operating_tenant_id + operating_carrier_company_id`. The same key and the same `pin_set_fingerprint` returns the existing execution and does not insert another one. The same key with a different `pin_set_fingerprint` returns `RELATIONSHIP_ACTIVATION_FINGERPRINT_CONFLICT` (409). That conflict does not create a second execution and does not change the stored pins. The fingerprint is defined in section 15.

## 15. Multi-shipment pin cardinality

0.1A stores the pin on the execution revision. One revision can cover shipments from more than one shipper tenant. A single `relationship_id` column on the revision cannot do that. The pin owner remains the revision. The physical rows are children.

```text
EXECUTION_RELATIONSHIP_PIN_CARDINALITY=ONE_ROW_PER_SHIPMENT_PARTICIPANT
PIN_OWNER=TRANSPORT_EXECUTION_REVISION
ONE_PIN_PER_SHIPMENT_PARTICIPANT=YES
ALL_PINS_MUST_RESOLVE_TO_SAME_OPERATING_TENANT=YES
ALL_PINS_MUST_RESOLVE_TO_SAME_OPERATING_COMPANY=YES
PARTIAL_RELATIONSHIP_PIN_COMMIT_ALLOWED=NO
DUPLICATE_SHIPMENT_PIN_POLICY=UNIQUE_CONFLICT_FAIL_CLOSED
PIN_SET_FINGERPRINT_REQUIRED=YES
```

Future `transport.transport_execution_carrier_relationship_pins` columns: `execution_revision_id`, `shipment_id`, `shipment_tenant_id`, `shipment_carrier_company_id`, `relationship_id`, `relationship_version`, `operating_tenant_id`, `operating_carrier_company_id`, `created_at`. Unique `(execution_revision_id, shipment_tenant_id, shipment_id)`. The revision and the activation authorization store the same `pin_set_fingerprint`. All subjects are authorized before any execution insert. One failure rolls back the batch.

`pin_set_fingerprint` is lowercase hex SHA-256 of a canonical UTF-8 JSON document with no insignificant whitespace. Top-level keys, in order, are `operating_tenant_id`, `operating_carrier_company_id`, and `subjects`. Each subject object uses keys `shipment_tenant_id`, `shipment_id`, and `shipment_carrier_company_id`, in that order. Subjects are sorted by lowercase `shipment_tenant_id`, then lowercase `shipment_id`. UUID values are lowercase. The fingerprint covers the requested participant set and the operating company. It does not include server-assigned `relationship_id` or `relationship_version`, so a retry of the same participants compares equal without learning new versions. Relationship identity stays on the pin rows.

## 16. Operating driver and vehicle validation

```text
DRIVER_VALIDATION_TRANSACTION_PHASE=SAME_TMS_TRANSACTION_AFTER_RELATIONSHIP_AUTHORIZATION_BEFORE_COMMIT
VEHICLE_VALIDATION_TRANSACTION_PHASE=SAME_TMS_TRANSACTION_AFTER_RELATIONSHIP_AUTHORIZATION_BEFORE_COMMIT
DRIVER_VALIDATION_AND_EXECUTION_COMMIT_ATOMIC=YES
VEHICLE_VALIDATION_AND_EXECUTION_COMMIT_ATOMIC=YES
SHIPMENT_DRIVER_COMPARED_TO_EXECUTION_DRIVER=NO
SHIPMENT_VEHICLE_COMPARED_TO_EXECUTION_VEHICLE=NO
NULL_DRIVER_IDENTITY_POLICY=NULL_IS_NO_ASSIGNMENT
NULL_VEHICLE_IDENTITY_POLICY=NULL_IS_NO_ASSIGNMENT
```

A non-null driver must match `id`, `tenant_id = operating_tenant_id`, `carrier_company_id` of the operating company, `status = ACTIVE`, and `deleted_at IS NULL` under a row lock in that transaction. A non-null vehicle uses the same five predicates on `transport.vehicles`. A miss fails the transaction and stores no assignment. Null stores null. Null is not operational readiness. `PICKUP_SLOT_BOOKED` stays `BLOCKED_BY_IDENTITY_MODEL` until the later TMS status stage. This design does not change the shipment FSM and does not compare shipment-local driver or vehicle ids with the execution columns.

The internal TMS row may store operating `driver_id` and `vehicle_id`. A shipper-scoped API projection must not return those identifiers. Enumeration of carrier drivers or vehicles remains forbidden. The public projection and the internal TMS model are different shapes. A shipper tenant learns that an execution exists only through its own shipment participation, without the operating driver or vehicle id.

## 17. Internal resolution API

The resolution used to commit an execution is the function in section 14, not an HTTP read.

```text
RELATIONSHIP_RESOLUTION_ROUTE=company.authorize_carrier_relationship_activation
RELATIONSHIP_RESOLUTION_REQUEST=operating_tenant_id+operating_carrier_company_id+source_activation_id+subjects[]
RELATIONSHIP_RESOLUTION_RESPONSE=per_subject_shipment_echo+relationship_id+relationship_version+operating_tenant_id+operating_carrier_company_id+activation_authorization_id
RELATIONSHIP_NOT_FOUND_STATUS=NOT_FOUND_FAIL_CLOSED
RELATIONSHIP_AMBIGUOUS_STATUS=DATA_INTEGRITY_ERROR_FAIL_CLOSED
RELATIONSHIP_STORE_FAILURE_STATUS=ERROR_NOT_NOT_FOUND
BATCH_RESOLUTION_ATOMICITY=ALL_SUBJECTS_OR_ROLLBACK
```

Each subject carries `shipment_id`, `shipment_tenant_id`, and `shipment_carrier_company_id` already verified by shipment-service. The operating company comes from the verified activation command, not from a client header. Zero relationships fail closed. More than one current head is a data-integrity error. A store error is an error. One failed subject fails the batch.

## 18. Error contract

Public acceptance responses do not include a foreign tenant id, a foreign company id, or a raw token.

| Code | HTTP | Visibility | Existence leak | Retry |
| --- | --- | --- | --- | --- |
| `RELATIONSHIP_REQUEST_NOT_FOUND` | 404 | Public generic | No | No |
| `RELATIONSHIP_REQUEST_INVALID_TOKEN` | 404 | Public generic, same body as not-found | No | No |
| `RELATIONSHIP_REQUEST_TOKEN_MISMATCH` | 404 | Public generic, same body as not-found | No | No |
| `RELATIONSHIP_REQUEST_EXPIRED` | 409 | Public after a valid token and admin context | Token already proved | No |
| `RELATIONSHIP_REQUEST_CANCELLED` | 409 | Public after a valid token and admin context | Token already proved | No |
| `RELATIONSHIP_REQUEST_ALREADY_ACCEPTED` | 409 | Public after a valid token and admin context. This is the only code for a different idempotency key after the request is `ACCEPTED`, including a second operating company. `ACCEPTED_REQUEST_CONFLICT` is not a code | No foreign company id | No |
| `RELATIONSHIP_REQUEST_STALE_SOURCE` | 409 | Public after a valid token and admin context | No | No |
| `RELATIONSHIP_ACCEPTANCE_FORBIDDEN` | 403 | Public | No request lookup before the role check | No |
| `RELATIONSHIP_IDEMPOTENCY_CONFLICT` | 409 | Public. Same idempotency key with a different request, body hash, or company. Not used for a different key after acceptance | No token | No |
| `RELATIONSHIP_NOT_FOUND` | 404 | Internal activation | No cross-shipper list | No |
| `RELATIONSHIP_AMBIGUOUS` | 409 | Internal activation | No | No |
| `RELATIONSHIP_REVOKED` | 409 | Internal activation | No | No |
| `RELATIONSHIP_AUTHORIZATION_EXPIRED` | 409 | Unused while TTL is transaction scope | No | Retry whole activation only if nothing committed |
| `RELATIONSHIP_ACTIVATION_FINGERPRINT_CONFLICT` | 409 | Internal TMS. Same activation lookup key, different `pin_set_fingerprint` | No | No. No second execution. Stored pins stay unchanged |
| `RELATIONSHIP_SOURCE_UNAVAILABLE` | 503 | Public acceptance | No | Yes |
| `RELATIONSHIP_STORE_UNAVAILABLE` | 503 | Public and internal | Must not become not-found | Yes |
| `OPERATING_DRIVER_NOT_VALID` | 409 | Internal TMS | No | No |
| `OPERATING_VEHICLE_NOT_VALID` | 409 | Internal TMS | No | No |

## 19. Outbox, audit, and observability

Future outbox events: `carrier_relationship.requested`, `accepted`, `cancelled`, `expired`, `revoked`, `activation_authorized`, `activation_consumed`, `activation_expired`.

First runtime release requires `requested`, `accepted`, `cancelled`, `expired`, and `revoked`. Activation events wait for the activation stage. `activation_expired` stays unused while authorization TTL is the transaction.

Audit retains actor, tenant, selected company, source, request, relationship, version, idempotency key reference, correlation id, reason, and timestamp. The raw token is never audited. Metrics cover create, accept success, accept refusal, invalid token, replay, conflict, revocation, resolution miss, ambiguous head, activation expiry, and driver or vehicle refusal. Token metrics count outcomes. They do not label the token.

## 20. Data retention and privacy

Request rows, events, relationship versions, acceptance idempotency records, and audit rows are retained with the relationship. They are not hard-deleted. Token hashes remain so a consumed token can still be distinguished from a replay. Creation idempotency rows follow the same lifetime as the request they created. Activation authorization rows that commit are retained with the execution revision. Rolled-back authorizations do not remain. Outbox rows may be deleted 30 days after successful publish. The audit row remains.

```text
SHIPPER_CAN_ENUMERATE_CARRIER_DRIVERS=NO
SHIPPER_CAN_ENUMERATE_CARRIER_VEHICLES=NO
CROSS_SHIPPER_PARTICIPATION_EXISTENCE_LEAK=NO
```

There is no public list of every relationship. A caller sees a relationship only when the verified selected company is one of its endpoints in the caller's tenant.

## 21. Migration and rollout design

No migration is created. No number is reserved.

Future migration classes, in order: company request tables, relationship root and version and head tables, idempotency tables, audit and outbox, the authorization function, then TMS pin rows and the revision fingerprint. Driver and vehicle checks are code in the existing TMS transaction, not a new identity table. The authorization function and the TMS insert share one PostgreSQL database, `freight_platform`. Split placement of those databases is unsupported in this version and must fail closed rather than fall back to an HTTP read followed by a commit.

```text
HISTORICAL_AUTO_MATCH_ALLOWED=NO
UNMAPPED_EXISTING_RECORD_POLICY=LEAVE_UNMAPPED_FAIL_CLOSED
FUTURE_BACKFILL_POLICY=NO_BACKFILL
ROLLBACK_POLICY=FLAG_OFF_STOPS_NEW_WRITES_COMMITTED_ROWS_REMAIN
DUAL_READ_REQUIRED=NO
FEATURE_FLAG_REQUIRED=YES
DEFAULT_FEATURE_FLAG_STATE=OFF
FIRST_RELEASE_SCOPE=REQUEST_AND_RELATIONSHIP_PERSISTENCE_PLUS_ACCEPTANCE_AND_REVOCATION
```

Historical shipments, awards, and executions stay unmapped. The flag default is off. Off means the new commands refuse and activation does not call the function. On is a later controller decision.

## 22. Implementation sequence

No stage below is authorized by this document.

| Stage | Owner | Scope | Entry | Exit | Review |
| --- | --- | --- | --- | --- | --- |
| 0.1C | backend-engineer, company-service | Request, relationship, idempotency, audit, outbox tables and the authorization function. Migration class only when a later task assigns a number. | This freeze accepted and implementation authorized | Constraints and empty-credential tests pass. No public route yet | Security review required |
| 0.1D | backend-engineer, company-service | Create, accept, cancel, expire, revoke. Public accept route and OpenAPI in that later task | 0.1C exit | Token, role, replay, and conflict tests pass. Flag default off | Security review required |
| 0.1E | backend-engineer, shipment-service | Shipment source adapter and fingerprint callback | 0.1D exit | Stale and unavailable tests pass. No direct relationship writes | Security review required |
| 0.1F | backend-engineer, rfx-service | Award and award-link adapters | 0.1D exit. Independent of 0.1E | Same freshness tests for both source types | Security review required |
| 0.1G | backend-engineer, shipment-service | In-transaction authorization and per-shipment pins | 0.1E exit. 0.1F required before RFx-sourced activation | Crash, revoke, and multi-shipment tests pass | Security review required |
| 0.1H | backend-engineer, shipment-service | Driver and vehicle predicates in that same transaction | 0.1G exit | Five-predicate failures roll back | Security review required |
| 0.1I | backend-engineer, shipment and TMS | Status integration. Separate from this identity design | 0.1H exit plus a new TMS authorization | `PICKUP_SLOT_BOOKED` policy revisited only in that stage | Independent TMS review |
| 0.1J | backend-engineer, network-optimizer | NLO-0.9D runtime | 0.1I exit plus a new NLO authorization | Not this design | Independent NLO review |

0.1E and 0.1F can proceed in parallel after 0.1D. 0.1G waits for 0.1E, and for 0.1F when the activation source is an award or an award link.

## 23. Failure and concurrency matrix

XTI_01 through XTI_54, XTI_DRIVER_01 through XTI_DRIVER_03, and XTI_VEHICLE_01 through XTI_VEHICLE_03 remain the 0.1A results. They are `FUTURE_REQUIRED` until the stage that implements them.

| Case | Result |
| --- | --- |
| XTI_RT_01 two admins accept one request | One commit. The same idempotency key receives 200. A different key is `RELATIONSHIP_REQUEST_ALREADY_ACCEPTED` (409) |
| XTI_RT_02 two operating companies accept | First company commits. The other is `RELATIONSHIP_REQUEST_ALREADY_ACCEPTED` (409) without a foreign company id |
| XTI_RT_03 accept races cancel | Head lock. One terminal event. The loser refuses |
| XTI_RT_04 accept races expiry | Database clock under the head lock. Expired does not accept |
| XTI_RT_05 source changes before accept | `STALE_NO_ACCEPTANCE` |
| XTI_RT_06 revoked between route-plan accept and activate | `REFUSE_NO_EXECUTION` |
| XTI_RT_07 revoked during the TMS transaction | Head lock. Revoke waits or activate sees `REVOKED` and rolls back |
| XTI_RT_08 crash after authorization work and before commit | Rollback. No authorization row remains. Retry the activation |
| XTI_RT_09 TMS commit and the ACK is lost | Same activation lookup key and same `pin_set_fingerprint` return the existing execution |
| XTI_RT_10 same activation id with a different `pin_set_fingerprint` | `RELATIONSHIP_ACTIVATION_FINGERPRINT_CONFLICT` (409). No second execution. Stored pins unchanged |
| XTI_RT_11 one of three relationships missing | Whole batch rolls back |
| XTI_RT_12 two relationships, two operating companies | Whole batch rolls back |
| XTI_RT_13 driver inactive before commit | Transaction rolls back. No execution |
| XTI_RT_14 vehicle inactive before commit | Transaction rolls back. No execution |
| XTI_RT_15 relationship store unavailable | Error. Not not-found |
| XTI_RT_16 source owner unavailable | `ERROR_NO_ACCEPTANCE` |
| XTI_RT_17 duplicate active heads | `DATA_INTEGRITY_ERROR_FAIL_CLOSED` |
| XTI_RT_18 historical read after revoke | Pinned revision unchanged |
| XTI_RT_19 feature flag off | New commands refuse. No activation call |
| XTI_RT_20 partial deploy rollback | Flag off. Committed rows remain. No backfill and no hard delete |
| XTI_RT_21 acceptance token missing | Generic 404. Body identical to XTI_RT_22 and XTI_RT_23 |
| XTI_RT_22 acceptance token present and not valid | Same generic 404 body as XTI_RT_21. No existence distinction |
| XTI_RT_23 token present and `relationship_request_id` does not match | Same generic 404 body as XTI_RT_21. No existence distinction |

## 24. Critical blockers

```text
A1B_BLOCKER_REQUEST_PERSISTENCE_MODEL=CLOSED
A1B_BLOCKER_TOKEN_SECURITY=CLOSED
A1B_BLOCKER_SOURCE_VERIFICATION_CONTRACT=CLOSED
A1B_BLOCKER_ACCEPTANCE_AUTHORIZATION=CLOSED
A1B_BLOCKER_RELATIONSHIP_VERSIONING=CLOSED
A1B_BLOCKER_SERVICE_AUTHENTICATION=CLOSED
A1B_BLOCKER_ACTIVATION_CONSISTENCY=CLOSED
A1B_BLOCKER_MULTI_SHIPMENT_PIN_CARDINALITY=CLOSED
A1B_BLOCKER_DRIVER_VEHICLE_VALIDATION=CLOSED
A1B_BLOCKER_REVOCATION_MODEL=CLOSED
A1B_BLOCKER_IMPLEMENTATION_SEQUENCE=CLOSED
READY_FOR_CONTROLLER_RUNTIME_IMPLEMENTATION_AUTHORIZATION=YES
IMPLEMENTATION_AUTHORIZED=NO
READY_FOR_NLO_0_9D_STATUS_INTEGRATION=NO
READY_FOR_NLO_0_9D_RUNTIME=NO
```

Controller acceptance of this design is still required before any implementation task.
