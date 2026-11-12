# Document write authority contract 0.1B

Stage: `DOCUMENT-WRITE-AUTHORITY-CONTRACT-0.1B`

Remediation: `DOCUMENT-WRITE-AUTHORITY-CONTRACT-0.1B-R1`

Issue: [#343](https://github.com/f9999737292-bit/Freihgt-platform/issues/343)

Discovery: [#350](https://github.com/f9999737292-bit/Freihgt-platform/pull/350), `docs/architecture/document-write-security-discovery-v0.1a.md`

Review findings addressed here: `J-DWA-001`, `J-DWA-002`, `J-DWA-003`

Base: `origin/main` `3f306fc407aeda4bdc652d825954292f4a54fad6`

Mode: design only. This document does not change an API, a handler, a gateway route, OpenAPI, a schema, a migration, staging, or production.

`CONTRACT_STATUS=PROPOSED`

`SECURITY_REVIEW=REQUIRED`

`REVIEW_STATE=READY_FOR_J_R2_REVIEW`

`APPROVED=NO`

`CONTRACT_APPROVED=NO`

`IMPLEMENTATION=NOT_AUTHORIZED`

`IMPLEMENTATION_AUTHORIZED=NO`

`COMPANY_POLICY_DECISION=REQUIRED`

`SIGNING_RECOVERY_HTTP_403=YES`

`TENANT_ISOLATION=REQUIRED`

`POD_ACTION_EVIDENCE_IS_SEPARATE=YES`

`EMPTY_INTERNAL_TOKEN_FAIL_CLOSED=YES`

`UNKNOWN_CALLER_DENIED=YES`

`CLIENT_AUTHORITY_HEADERS_NOT_TRUSTED=YES`

`GAP-B-001` stays open. This contract does not define a completeness evaluator, a requirement policy, or an external EDO exchange. An internal `SIGNED` status stays an internal document state.

`VERIFIED_INTERNET_EXPOSURE=NOT_ESTABLISHED`. No live request was sent to staging or production. This pull request does not apply the contract to staging or production.

## Identities

Four identities stay separate. Possession of one does not establish the others.

| Identity | Who establishes it | What document-service may use it for |
| --- | --- | --- |
| User | api-gateway, from the verified JWT subject when `AUTH_ENABLED=true`. For driver POD, api-gateway resolves the driver with `GetMe` after `ensureDriverAccess`. | The user who must match company membership or the assigned driver. `X-User-ID`, `signer_user_id`, and a body user id do not authenticate the caller. |
| Company | The server membership record or the server driver record, as defined in Company authority. A tenant-global role is a different fact. | The company for which the verified user may write. `X-Company-ID`, `owner_company_id`, and `signer_company_id` do not establish it. |
| Tenant | The `tenant_id` claim of the verified JWT, or the driver request context built from that claim. | The only tenant a confirmed caller may assert. A body or query `tenant_id` that disagrees is denied. |
| Internal service | document-service, on the write route, before the handler. | Proof that the hop came from an allowed service. The check is a dedicated token plus an exact caller name. |

document-service confirms the internal service itself. Gateway header stripping, `RequireOperatorShipmentRead`, a published or unpublished port, and Compose DNS are not that confirmation.

The confirmation uses the existing `packages/shared-go/internalauth` comparison: trim the configured token, treat an empty configured token as a denial, and compare `X-Internal-Service-Token` in constant time. The route then requires `X-Internal-Service-Name` to match the allowlist for that route. A token without the allowed name is denied. A name without the matching token is denied. `Authorization: Bearer` is not this credential. `INTERNAL_SERVICE_TOKEN` is not this credential.

Proposed credentials, not provisioned by this document:

| Route group | Configured token | Allowed `X-Internal-Service-Name` |
| --- | --- | --- |
| Document lifecycle writes below | `DOCUMENT_WRITE_GATEWAY_TOKEN` | `api-gateway` |
| `/internal/v1/pod-uploads` | `POD_UPLOAD_GATEWAY_TOKEN` | `api-gateway` |
| `/internal/v1/pod-action-evidence` | existing `POD_ACTION_EVIDENCE_SHIPMENT_SERVICE_TOKEN` | `shipment-service` |
| `POST /v1/documents/{id}/signing-recovery` | none | none |

A token from one row is a denial on every other row. An empty configured token denies the whole row. The configured value stays on the server. Responses, error bodies, and client-visible logs do not contain it. Tests use fixture values.

## Company authority

`J-DWA-001`

A tenant-global role and a company grant are different checks.

`RequireOperatorShipmentRead` and a tenant-global `PLATFORM_ADMIN` role establish neither company membership nor the right to choose `owner_company_id`. `CompanyExists` and `UserExists` establish that a row exists in the tenant. They do not establish that the verified user may act for that company or as that signer.

The default company grant for an operator document write is all of the following, taken from the server:

1. The user is the verified JWT subject.
2. The tenant is the JWT `tenant_id`.
3. Identity-service returns that subject in an `ACTIVE` company membership for that tenant. The existing gateway lookup is `IdentityClient.ListUserCompanies`, which calls `GET /v1/users/{id}/companies?tenant_id={tenant}&status=ACTIVE`.
4. The requested `owner_company_id` equals one company in that membership list.
5. When `related_entity_type` is `SHIPMENT`, `transport.shipments` has that id in the JWT tenant with `deleted_at IS NULL`, and the same `owner_company_id` is one of that row's `shipper_company_id`, `consignee_company_id`, `carrier_company_id`, or `forwarder_company_id`.
6. When another `related_entity_type` is present, the implementation confirms that the entity exists in the JWT tenant and names the same `owner_company_id`. A relationship that cannot be confirmed is a denial.
7. For a later write to an existing document, the stored document tenant and stored `owner_company_id` satisfy the same user, tenant, membership, and relationship checks.

A body `owner_company_id` is the requested company. It becomes acceptable only when it matches the server membership and the relationship checks. A client `X-Company-ID` is removed before those checks and is not a substitute.

For `POST /v1/signing-sessions/{id}/signatures` and attachment signatures, the same grant applies and these identities must match it:

- `signer_user_id` equals the verified JWT subject.
- `signer_company_id` equals the server company grant for that document.
- The session and the document belong to the JWT tenant.

A signer or company that merely exists in the tenant fails this check.

For POD, company authority is the `companyId` on the driver record returned by the server `GetMe` for the verified driver. The shipment authority is `transport.shipments.driver_id` for that shipment in the same tenant. A body `ownerCompanyId` or `driverId` that disagrees with those server values is denied.

Negative case, required: the internal caller credential is valid, the user and tenant are verified, and the company is outside the user's server membership or outside the shipment relationship. The result is `HTTP_DENIED=YES` and `STORE_WRITES=0`. A membership or relationship read is allowed. No document, version, file, attachment, signature, session, upload intent, or object is inserted or updated. A repeat of that request still writes nothing.

`PLATFORM_ADMIN` acting for every company in the tenant is not approved. If a later controller decision allows a cross-company administrative grant, that grant needs its own permission distinct from the tenant-global role, an audit record of the acting user, tenant, company, and document, and tests that a `PLATFORM_ADMIN` without the grant is denied. This contract does not create that permission.

`COMPANY_POLICY_DECISION=REQUIRED` for that administrative exception. Until a controller approves it, the implementation denies the valid caller when the company is outside the server membership.

## Gateway header handling

`J-DWA-002`

On every inbound api-gateway request, before route authorization, the reverse proxy, and `document.Client`, the gateway deletes client identity and internal-authority headers. The deletion runs when `AUTH_ENABLED=true` and when `AUTH_ENABLED=false`.

The deleted names include `X-Tenant-ID`, `X-User-ID`, `X-User-Email`, `X-User-Roles`, `X-Company-ID`, `X-Actor-Kind`, `X-Platform-Admin`, `X-Role`, `X-Internal-Service-Token`, and `X-Internal-Service-Name`, plus the headers already covered by `StripUntrustedIdentityHeaders`. `X-Internal-Service-Name` is included even though the current helper does not delete it yet.

The gateway then follows one of these paths.

When `AUTH_ENABLED=true`:

1. Verify the JWT.
2. For an operator document or signing-session write, require the verified subject and the server company grant. A tenant-global role alone does not pass.
3. For driver POD, require `ensureDriverAccess`, a shipment in the JWT tenant, and `transport.shipments.driver_id` equal to the server driver id.
4. After those checks succeed, set `X-Tenant-ID` and `X-User-ID` from the verified JWT, and set the company from the membership or the driver record.
5. After those server headers exist, and only on the outbound hop, add the dedicated token for that route group and `X-Internal-Service-Name=api-gateway`. The token value comes from gateway server configuration. It is never copied from the inbound request.

When `AUTH_ENABLED=false`:

1. Do not verify a JWT and do not create server identity headers.
2. Do not add `DOCUMENT_WRITE_GATEWAY_TOKEN` or `POD_UPLOAD_GATEWAY_TOKEN` for a public document route or a driver POD route.
3. The public write fails closed at the gateway because no hop credential is formed, and document-service denies the request again because the caller credential is absent.

`X-Upload-Token` is the upload capability returned by a successful intent. It is not an internal service credential. The gateway may send the client-presented upload token to `document.Client` only after the assigned-driver checks succeed. document-service still compares it with the stored hash. The gateway never sends `DOCUMENT_WRITE_GATEWAY_TOKEN`, `POD_UPLOAD_GATEWAY_TOKEN`, or the action-evidence token to a browser or other external client.

Required gateway and document-service denials. Each one has `HTTP_DENIED=YES` and `STORE_WRITES=0`:

- `AUTH_ENABLED=false` and a client `DOCUMENT_WRITE_GATEWAY_TOKEN`.
- `AUTH_ENABLED=false` and a client `POD_UPLOAD_GATEWAY_TOKEN`.
- `AUTH_ENABLED=false` and a client `X-Internal-Service-Name=api-gateway`.
- `AUTH_ENABLED=true`, a JWT whose user lacks the server company grant or the assigned-driver check, and a client token that matches the configured server token.
- `AUTH_ENABLED=true` and client `X-Tenant-ID`, `X-User-ID`, or `X-Company-ID` values that differ from the verified JWT and the server company grant.
- The document-service token configuration for that route group is empty.

A client token that matches the server configuration is still deleted. The gateway adds its own token only after the user, tenant, and company or driver checks succeed. An empty document-service token denies every write for that route group, including a well-formed gateway hop.

## Caller and operation matrix

Current callers on this base, from the routers and the gateway document client:

- `POST /api/v1/documents` and `POST /api/v1/signing-sessions/{id}/signatures` reach document-service through the gateway reverse proxy. The proxy forwards the request after `RequireOperatorShipmentRead`. It does not attach `X-Internal-Service-Token`. That current behavior is the gap this contract closes. It is not the approved company grant.
- Driver POD reaches document-service from `services/api-gateway/internal/document/client.go`. That client sends `X-Tenant-ID` and, for content, `X-Upload-Token`. The public routes stay `/api/v1/driver/me/shipments/{shipmentId}/pod/uploads`.
- `POST /internal/v1/pod-action-evidence/...` is already limited to `shipment-service` plus its dedicated token. This contract leaves that group unchanged.
- Shipper and carrier document routes are `GET` only on both the gateway and document-service.

| ROUTE | CALLER | AUTHENTICATION | COMPANY_AUTHORITY | TENANT_AUTHORITY | DENY_CONDITION | ACCEPTANCE_TEST |
| --- | --- | --- | --- | --- | --- | --- |
| `POST /v1/documents` | `api-gateway` after the verified JWT user passes the server company grant | document-service requires `DOCUMENT_WRITE_GATEWAY_TOKEN` and `X-Internal-Service-Name=api-gateway` before `Create` | Active membership from `ListUserCompanies` for the JWT subject and JWT tenant. `owner_company_id` equals that membership. A shipment relationship must name the same company. `PLATFORM_ADMIN` and body `owner_company_id` are not the grant | `X-Tenant-ID` is the JWT tenant only after caller confirmation and header replacement. A disagreeing body `tenant_id` is denied | No caller credential. `X-Tenant-ID` alone. `X-User-ID` alone. `X-Company-ID` alone. Bearer alone. Wrong token. Right token with a missing or other caller name. Right name with the POD or action-evidence token. Body tenant disagrees. Valid caller whose company is outside the membership or the shipment relationship | Real `document-service.NewRouter`. Credential denials leave the handler uncalled. The valid-caller foreign-company case has `HTTP_DENIED=YES` and `STORE_WRITES=0` |
| `POST /v1/documents/{id}/versions` | Same caller and company grant | Same document-write credential | The stored document company satisfies the same grant for the verified user | Same JWT tenant. A document id outside that tenant is denied | Same caller and company denials | Version row appears only for the granted company. A foreign company writes nothing |
| `POST /v1/documents/{id}/files` | Same | Same document-write credential | Same stored-document grant | Same | Same caller and company denials | Same |
| `POST /v1/documents/{id}/attachments` | Same | Same document-write credential | Same stored-document grant | Same. A query `tenant_id` that disagrees is denied | Same denials. A spoofed `X-User-ID` is removed at the gateway and is not actor authority on a direct call | Actor metadata is the verified subject after the grant. A spoofed user writes nothing |
| `POST /v1/documents/{id}/attachments/{attachmentId}/finalize` | Same | Same document-write credential | Same stored-document grant | Same | Same denials | Same |
| `POST /v1/documents/{id}/attachments/{attachmentId}/signatures` | Same | Same document-write credential | `signer_user_id` is the verified subject and `signer_company_id` is the server company grant for that document | Same | Same denials. An in-tenant signer or company that is not the verified user and grant is denied | Existing content-type, size, and idempotency checks stay after the grant. A mismatched signer writes nothing |
| `POST /v1/documents/{id}/ready-for-signing` | Same | Same document-write credential | Same stored-document grant | Same | Same denials. `ValidateReadyForSigningStatus` still allows only `DRAFT` to `READY_FOR_SIGNING` | Same |
| `POST /v1/documents/{id}/signing-sessions` | Same. Gateway path `/api/v1/documents/*` | Same document-write credential | Same stored-document grant | Same | Same denials. `CreateSession` still requires `READY_FOR_SIGNING`, rejects an active session, and rechecks inside its transaction | Existing session concurrency tests stay in force |
| `POST /v1/signing-sessions/{id}/signatures` | Same. Gateway path `/api/v1/signing-sessions/*` | Same document-write credential | `signer_user_id` equals the verified subject. `signer_company_id` equals the server company grant. Existence in the tenant is not sufficient | Same. The session belongs to that tenant | Same denials. A foreign-tenant session writes nothing. Lock order stays document, then session, then `clock_timestamp()` after the session lock | A granted signature persists. A different in-tenant signer writes nothing |
| `POST /v1/documents/{id}/cancel` | Same | Same document-write credential | Same stored-document grant | Same | Same denials | Same |
| `POST /v1/documents/{id}/archive` | Same | Same document-write credential | Same stored-document grant | Same | Same denials | Same |
| `POST /v1/documents/{id}/signing-recovery` | No caller | No credential is accepted | None | Not read | Every request, including a document-write, POD-upload, or action-evidence credential, a granted operator, and an assigned driver. The handler returns `403` and does not call `RecoverExpiredSigning` | Existing `TestDocumentServiceRouterDeniesSigningRecovery` and `TestSigningRecoveryIsNotProxied`. Add the future credentials and a granted-user case. `HTTP_DENIED=YES` and `STORE_WRITES=0` |
| `POST /internal/v1/pod-uploads` | `api-gateway` `document.Client` after the assigned-driver checks | document-service requires `POD_UPLOAD_GATEWAY_TOKEN` and `X-Internal-Service-Name=api-gateway` before `CreateIntent`. The browser does not receive this token | Company is the server driver `companyId`. The shipment row in the same tenant has `driver_id` equal to that driver. Body `ownerCompanyId` and `driverId` must match those server values | Tenant is the JWT tenant the gateway sets after stripping the client header | `X-Tenant-ID` alone. Body ids alone. Document-write token. Action-evidence token with `shipment-service`. POD token with any other name. Valid POD credential with an existing unassigned driver. Shipment tenant differs from the JWT tenant | `POD-ASSIGN-DENY` and `POD-TENANT-MISMATCH`: `HTTP_DENIED=YES`, `STORE_WRITES=0`. An assignment read is allowed. Repeat writes nothing |
| `PUT /internal/v1/pod-uploads/{uploadId}/content` | Same gateway client | Same POD credential, then the existing `X-Upload-Token` check | Same assigned driver and server driver company | Same JWT tenant | Same caller denials. A valid upload token without the POD credential, or a POD credential for an unassigned driver, writes nothing | Content is stored only for the assigned driver's intent and the issued upload token |
| `POST /internal/v1/pod-uploads/{uploadId}/complete` | Same gateway client | Same POD credential before `Complete` | Same | Same | Same caller denials. Existing checksum and same-driver rules stay after the gate | Complete persists one document and file for the assigned driver. A repeat returns that same result and does not create a second document |
| `/internal/v1/pod-action-evidence/*` | `shipment-service` | Existing `internalAuth.Middleware` and `RequireShipmentServiceCaller` | Unchanged | Unchanged | Unchanged. This row is not a new grant and does not cover `/internal/v1/pod-uploads` or `/v1/documents` | Existing pod-action router tests stay the proof for this group |

Denial status follows the current helpers. A missing or wrong token is `401`. A token that is valid for a different route group, or a caller name outside that route's allowlist, is `403`. A valid caller whose company or driver assignment fails is `403`. Signing recovery stays `403` with `signing recovery is not available` on document-service and `signing recovery has no public write authority` on the gateway.

`STORE_WRITES=0` means no insert or update of a document, version, file, attachment, signature, signing session, upload intent, or stored object. A read that decides membership, shipment party, or driver assignment may occur. For a missing caller credential, the write handler is not called.

## Direct-call and gateway tests

The implementation stage adds these tests. This document does not add them.

Use `document-service.NewRouter` with the real route table. A hand-built chi router that mounts one handler is not the proof. Credential denials use a store spy and leave it uncalled. Company, relationship, and driver-assignment denials may read and must still finish with `STORE_WRITES=0`.

For create, version, file, attachment create, attachment finalize, attachment signature, ready-for-signing, signing-session, session signature, cancel, and archive, each of these requests is denied and writes nothing:

- no identity headers
- `X-Tenant-ID` only
- `X-User-ID` only
- `X-Company-ID` only
- `Authorization: Bearer` only
- a wrong `X-Internal-Service-Token` with `X-Internal-Service-Name=api-gateway`
- `DOCUMENT_WRITE_GATEWAY_TOKEN` with no caller name
- `DOCUMENT_WRITE_GATEWAY_TOKEN` with `shipment-service`
- `POD_UPLOAD_GATEWAY_TOKEN` or the action-evidence token with `api-gateway`
- an empty configured document-write token, including a request that would otherwise be a valid gateway hop
- `DOCUMENT_WRITE_GATEWAY_TOKEN` with `api-gateway` and a body or query `tenant_id` that disagrees with the JWT tenant
- `DOCUMENT_WRITE_GATEWAY_TOKEN` with `api-gateway`, a verified user, and an `owner_company_id` outside that user's active membership
- the same valid caller when the company is in the membership and is not a party on the named shipment
- a signature whose `signer_user_id` or `signer_company_id` differs from the verified subject and the server company grant, including a signer that exists in the tenant

For the three `/internal/v1/pod-uploads` routes, the same denial set includes tenant-only, upload-token-only, the document-write token with `api-gateway`, and the action-evidence token with `shipment-service`.

`POD-ASSIGN-DENY`, for `POST /internal/v1/pod-uploads`:

1. `POD_UPLOAD_GATEWAY_TOKEN` and `X-Internal-Service-Name=api-gateway` are valid.
2. The shipment exists.
3. The driver exists and is not `transport.shipments.driver_id` for that shipment.
4. The request attempts to create an upload intent.
5. The response is a denial.
6. No upload intent is created.
7. No document or object is written.
8. `STORE_WRITES=0`. The assignment query may read the shipment.
9. The same request repeated still writes nothing and does not become an idempotent success.

`POD-TENANT-MISMATCH`: the POD credential is valid, and the shipment exists under a different tenant from the gateway tenant assertion. The result is a denial, `STORE_WRITES=0`, and a repeat still writes nothing.

Signing recovery denies every case above, including both future gateway tokens, the action-evidence token, a user with a valid company grant, and an assigned driver.

Gateway tests:

- `TestSigningRecoveryIsNotProxied` keeps returning `403` without a proxy.
- Shipper and carrier document routes stay `GET`.
- With `AUTH_ENABLED=false`, a client document-write token, a client POD token, and a client `api-gateway` name are removed and do not produce a hop credential.
- With `AUTH_ENABLED=true`, a verified user who fails the company grant or the assigned-driver check is denied before the gateway adds a server token, even when the client presented that token.
- Client tenant, user, and company headers that differ from the verified JWT are removed before the hop.
- Those gateway tests do not replace the direct `NewRouter` proof.

## Positive acceptance

A request that reaches a handler is not yet a successful business operation. Each positive case below names the persisted result.

- An operator whose JWT subject has an active membership in company C, in the JWT tenant, creates a document with `owner_company_id` C. When the document names a shipment, that shipment is in the same tenant and lists C as shipper, consignee, carrier, or forwarder. The persisted document uses that tenant and company C.
- The same user creates a new version of that document after the stored company still matches the grant. The new version is stored on that document, and the tenant and owner company stay unchanged.
- A signature succeeds only when `signer_user_id` is that verified subject, `signer_company_id` is company C for that document, and the existing session, expiry, and lock checks pass. The signature row is stored. The session completed count changes only through the existing compare-and-set on the locked row.
- An assigned driver creates a POD upload intent. `transport.shipments.driver_id` equals the server driver id, the tenant matches, and the owner company equals the server driver company. One POD document and one upload intent are stored.
- That assigned driver uploads content with the upload token issued for that intent. The bytes are stored for that intent's object key.
- Complete finishes that same upload for the same driver and a matching checksum. One document and file result. A repeat complete returns that same result and does not create a second document.
- Signing recovery over HTTP still returns `403` for this operator and this driver. `STORE_WRITES=0`.

## Rollout

A later implementation changes api-gateway, document-service, `document.Client`, the internal credential configuration, and the E2E harness together. This pull request does not make those changes and does not authorize them on staging or production.

Order:

1. Prepare the two gateway token configurations outside Git. Leave them empty until activation. An empty document-service token fails closed. Do not return the values to a client.
2. Ship one change that adds gateway stripping independent of `AUTH_ENABLED`, the user and company or driver checks, server header creation, and outbound credential attachment, together with the document-service gates and the `document.Client` hop. Update the E2E harness in that same change so a direct document-service write uses the test credential only after the same checks.
3. Activate by setting the same token values on api-gateway and document-service together. A token configured on only one side stays fail-closed: the gateway cannot form a credential it does not have, and document-service denies an empty or unexpected token.
4. Run the negative header, company, tenant, and `POD-ASSIGN-DENY` cases, then the positive acceptance cases, before any exposure beyond the local test.
5. Roll back by stopping the gateway from attaching either token and clearing the document-service token configuration. The empty token denies writes. Rollback does not resume trust in client `X-Tenant-ID`, `X-User-ID`, `X-Company-ID`, or a client internal token.

The forbidden intermediate state is a document-service that accepts a token while the gateway still forwards a client-supplied token, or attaches its token before the user, membership, and driver checks. That state would let the new credential bypass user authorization. It is not a transitional mode.

`tests/bintrans-e2e-transport-flow` currently calls `/internal/v1/pod-uploads` with a tenant header. The implementation change updates that caller. This document does not.

## Compatibility with the current gateway and EDO flow

The public route shapes and OpenAPI stay as they are.

- Operator document and signing-session paths stay `/api/v1/documents` and `/api/v1/signing-sessions`. The current `RequireOperatorShipmentRead` guard is a shipment read check for a tenant-global `PLATFORM_ADMIN`. The future company grant replaces that check as company authority. This document does not change the route registration.
- Shipper and carrier document routes stay `GET`.
- Driver POD stays on `/api/v1/driver/me/shipments/{shipmentId}/pod/uploads`.
- `CreateSession` still starts only from `READY_FOR_SIGNING`, sets `SIGNING_IN_PROGRESS`, and rechecks eligibility inside the document-then-session lock.
- `AddSignature` keeps the document lock, then the session lock, then `clock_timestamp()` as its own statement after the session lock. Equality of `expires_at` and that timestamp stays overdue.
- In-process `RecoverExpiredSigning` stays available to in-process callers and stays unregistered as an authorized HTTP write. The HTTP route stays the existing `403` handler on both document-service and the gateway, registered before the document wildcard.
- Attachment signature content-type, size, and idempotency rules stay after the caller and company checks.
- OpenAPI for signing recovery stays `PUBLIC_SIGNING_RECOVERY=DISABLED` and `EXPECTED_PUBLIC_RESPONSE=403`. This contract does not add a public success response and does not treat `bearerAuth` as write authority.
- Local Compose sets `AUTH_ENABLED` to false and publishes `8086:8086`. The staging overlay clears the published host port and leaves `document-service:8086` on the Compose network. An unpublished port is not the caller credential. With `AUTH_ENABLED=false`, the gateway forms no document write credential.

## Out of scope

Qualified signature verification, EDO operator exchange, GAP-B-001, cabinet UI, payment rules, read-route authority, live staging or production probes, secret provisioning, production rollout, and the unapproved cross-company administrative grant are outside this contract. Read routes that still accept `X-Tenant-ID` are not closed by this document.

Independent review of this remediation is required before the contract can be treated as approved. Gateway and document-service implementation requires a separate controller authorization after that review.
