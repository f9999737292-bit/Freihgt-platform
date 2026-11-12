# Document write authority contract 0.1B

Stage: `DOCUMENT-WRITE-AUTHORITY-CONTRACT-0.1B`

Issue: [#343](https://github.com/f9999737292-bit/Freihgt-platform/issues/343)

Discovery: [#350](https://github.com/f9999737292-bit/Freihgt-platform/pull/350), `docs/architecture/document-write-security-discovery-v0.1a.md`

Base: `origin/main` `3f306fc407aeda4bdc652d825954292f4a54fad6`

Mode: design only. This document does not change an API, a handler, a gateway route, a schema, a migration, staging, or production.

`CONTRACT_STATUS=PROPOSED`

`SECURITY_REVIEW=REQUIRED`

`APPROVED=NO`

`IMPLEMENTATION=NOT_AUTHORIZED`

`FINAL_VERDICT=CONTRACT_PROPOSED`

`GAP-B-001` stays open. This contract does not define a completeness evaluator, a requirement policy, or an external EDO exchange. An internal `SIGNED` status stays an internal document state.

`VERIFIED_INTERNET_EXPOSURE=NOT_ESTABLISHED`. No live request was sent to staging or production.

## Identities

Four identities stay separate. Possession of one does not establish the others.

| Identity | Who establishes it | What document-service may use it for |
| --- | --- | --- |
| User | api-gateway, from the verified JWT subject when `AUTH_ENABLED=true`. For driver POD, api-gateway `requireAssignedDriver` resolves the assigned driver before the hop. | Attribution after the internal caller is confirmed. `X-User-ID`, `signer_user_id`, and a body user id do not authenticate the caller. |
| Company | api-gateway. Operator document routes today pass `RequireOperatorShipmentRead`, which allows a verified tenant-global `PLATFORM_ADMIN` and strips client company headers. Driver POD uses the company on the assigned driver record. | The public-edge role or the assigned-driver company. `X-Company-ID`, `owner_company_id`, and `signer_company_id` do not authenticate the caller. `CompanyExists` remains an in-tenant existence check after the caller is confirmed. |
| Tenant | api-gateway, from the verified JWT tenant or the driver request context. | The tenant assertion on `X-Tenant-ID` only after the internal caller is confirmed. A body or query `tenant_id` that disagrees with that assertion is denied. |
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

A token from one row is a denial on every other row. An empty configured token denies the whole row.

## Caller and operation matrix

Current callers on this base, from the routers and the gateway document client:

- `POST /api/v1/documents` and `POST /api/v1/signing-sessions/{id}/signatures` reach document-service through the gateway reverse proxy. The proxy forwards the request after `RequireOperatorShipmentRead`. It does not attach `X-Internal-Service-Token`.
- Driver POD reaches document-service from `services/api-gateway/internal/document/client.go`. That client sends `X-Tenant-ID` and, for content, `X-Upload-Token`. The public routes stay `/api/v1/driver/me/shipments/{shipmentId}/pod/uploads`.
- `POST /internal/v1/pod-action-evidence/...` is already limited to `shipment-service` plus its dedicated token. This contract leaves that group unchanged.
- Shipper and carrier document routes are `GET` only on both the gateway and document-service.

| ROUTE | CALLER | AUTHENTICATION | COMPANY_AUTHORITY | TENANT_AUTHORITY | DENY_CONDITION | ACCEPTANCE_TEST |
| --- | --- | --- | --- | --- | --- | --- |
| `POST /v1/documents` | `api-gateway` after a verified user passes `RequireOperatorShipmentRead` | document-service requires `DOCUMENT_WRITE_GATEWAY_TOKEN` and `X-Internal-Service-Name=api-gateway` before `Create` | Public edge remains tenant-global `PLATFORM_ADMIN`. Body `owner_company_id` must exist in the authorized tenant and is not the caller company | `X-Tenant-ID` is the gateway tenant assertion only after caller confirmation. A disagreeing body `tenant_id` is denied | No caller credential. `X-Tenant-ID` alone. `X-User-ID` alone. `X-Company-ID` alone. Bearer alone. Wrong token. Right token with a missing or other caller name. Right name with the POD or action-evidence token. Body tenant disagrees | Real `document-service.NewRouter` and a store spy. Each denial leaves `store.calls=0` |
| `POST /v1/documents/{id}/versions` | Same caller and public edge | Same document-write credential | Same. The version stays on the document in the authorized tenant | Same header assertion. A cross-tenant document id is denied by the existing tenant predicate | Same caller denials, plus a document id outside the asserted tenant | Same spy. Cross-tenant id writes nothing |
| `POST /v1/documents/{id}/files` | Same | Same document-write credential | Same | Same | Same caller denials | Same spy |
| `POST /v1/documents/{id}/attachments` | Same | Same document-write credential | Same | Same. A query `tenant_id` that disagrees is denied | Same caller denials. A spoofed `X-User-ID` without the caller credential is denied. After confirmation, `X-User-ID` is actor metadata from the gateway subject | Same spy, including spoofed `X-User-ID` |
| `POST /v1/documents/{id}/attachments/{attachmentId}/finalize` | Same | Same document-write credential | Same | Same | Same caller denials | Same spy |
| `POST /v1/documents/{id}/attachments/{attachmentId}/signatures` | Same | Same document-write credential | Same. Signature metadata does not authenticate the caller | Same | Same caller denials. Existing content-type, size, and idempotency checks stay after the gate | Same spy |
| `POST /v1/documents/{id}/ready-for-signing` | Same | Same document-write credential | Same | Same | Same caller denials. `ValidateReadyForSigningStatus` still allows only `DRAFT` to `READY_FOR_SIGNING` | Same spy |
| `POST /v1/documents/{id}/signing-sessions` | Same. Gateway path `/api/v1/documents/*` | Same document-write credential | Same | Same | Same caller denials. `CreateSession` still requires `READY_FOR_SIGNING`, rejects an active session, and rechecks inside its transaction | Same spy, and the existing session concurrency tests stay in force |
| `POST /v1/signing-sessions/{id}/signatures` | Same. Gateway path `/api/v1/signing-sessions/*` | Same document-write credential | Signer company must exist in the authorized tenant and is not the caller company | Same. Signer user must exist in that tenant and is not the caller user | Same caller denials. A foreign-tenant session id writes nothing. Lock order stays document, then session, then `clock_timestamp()` after the session lock | Same spy, plus the existing foreign-tenant signature rejection |
| `POST /v1/documents/{id}/cancel` | Same | Same document-write credential | Same | Same | Same caller denials | Same spy |
| `POST /v1/documents/{id}/archive` | Same | Same document-write credential | Same | Same | Same caller denials | Same spy |
| `POST /v1/documents/{id}/signing-recovery` | No caller | No credential is accepted | None | Not read | Every request, including a document-write, POD-upload, or action-evidence credential. The handler returns `403` and does not call `RecoverExpiredSigning` | Existing `TestDocumentServiceRouterDeniesSigningRecovery` and `TestSigningRecoveryIsNotProxied`. Add the three future credentials to the direct-router cases. `store.calls` stays `0` |
| `POST /internal/v1/pod-uploads` | `api-gateway` `document.Client`, called from the driver POD handlers after `requireAssignedDriver` | document-service requires `POD_UPLOAD_GATEWAY_TOKEN` and `X-Internal-Service-Name=api-gateway` before `CreateIntent`. The browser does not receive this token | Company is the assigned driver company the gateway already resolved | Tenant is the driver request tenant the gateway sends. A client `X-Tenant-ID` on this route, without the POD credential, is denied | `X-Tenant-ID` alone. Body `shipmentId`, `driverId`, or `ownerCompanyId` alone. Document-write token. Action-evidence token with `shipment-service`. POD token with any other name | Real `NewRouter` spy. A tenant header alone creates no intent |
| `PUT /internal/v1/pod-uploads/{uploadId}/content` | Same gateway client | Same POD credential, then the existing `X-Upload-Token` check | Same assigned-driver company | Same gateway tenant assertion | Same caller denials. A valid `X-Upload-Token` without the POD credential is denied. A POD credential without the upload token still fails the existing upload check | Same spy. Upload-token-only writes nothing |
| `POST /internal/v1/pod-uploads/{uploadId}/complete` | Same gateway client | Same POD credential before `Complete` | Same | Same | Same caller denials. Existing driver, checksum, and repeat-complete rules stay after the gate | Same spy |
| `/internal/v1/pod-action-evidence/*` | `shipment-service` | Existing `internalAuth.Middleware` and `RequireShipmentServiceCaller` | Unchanged | Unchanged | Unchanged. This row is not a new grant and does not cover `/internal/v1/pod-uploads` or `/v1/documents` | Existing pod-action router tests stay the proof for this group |

Denial status follows the current helpers. A missing or wrong token is `401`. A token that is valid for a different route group, or a caller name outside that route's allowlist, is `403`. Signing recovery stays `403` with `signing recovery is not available` on document-service and `signing recovery has no public write authority` on the gateway.

## Direct-call tests

The implementation stage adds these tests. This document does not add them.

Use `document-service.NewRouter` with the real route table and a store spy. A hand-built chi router that mounts one handler is not the proof. Nil business services are acceptable only while the assertion is that the handler was not called. The positive cases, when they are added, use fakes and still run the caller gate first.

For create, version, file, attachment create, attachment finalize, attachment signature, ready-for-signing, signing-session, session signature, cancel, and archive, each of these requests is denied and leaves the spy at zero writes:

- no identity headers
- `X-Tenant-ID` only
- `X-User-ID` only
- `X-Company-ID` only
- `Authorization: Bearer` only
- a wrong `X-Internal-Service-Token` with `X-Internal-Service-Name=api-gateway`
- `DOCUMENT_WRITE_GATEWAY_TOKEN` with no caller name
- `DOCUMENT_WRITE_GATEWAY_TOKEN` with `shipment-service`
- `POD_UPLOAD_GATEWAY_TOKEN` or the action-evidence token with `api-gateway`
- `DOCUMENT_WRITE_GATEWAY_TOKEN` with `api-gateway` and a body or query `tenant_id` that disagrees with `X-Tenant-ID`

One positive case per group, with `DOCUMENT_WRITE_GATEWAY_TOKEN`, `X-Internal-Service-Name=api-gateway`, and a matching tenant assertion, reaches the handler. A foreign-tenant document or session id still writes nothing.

For the three `/internal/v1/pod-uploads` routes, the same spy denies tenant-only, upload-token-only, the document-write token with `api-gateway`, and the action-evidence token with `shipment-service`. The positive intent and complete cases use `POD_UPLOAD_GATEWAY_TOKEN` and `api-gateway`. Content still requires `X-Upload-Token` after that gate.

Signing recovery denies every case above, including both future gateway tokens and the action-evidence token.

Gateway tests that stay in the implementation stage: `TestSigningRecoveryIsNotProxied` keeps returning `403` without a proxy. Shipper and carrier document routes stay `GET`. With `AUTH_ENABLED=true`, a caller who lacks tenant-global `PLATFORM_ADMIN` is still denied by `RequireOperatorShipmentRead` before the proxy. With `AUTH_ENABLED=false`, the gateway attaches neither gateway token. Those gateway tests do not replace the direct `NewRouter` spy.

## Compatibility with the current gateway and EDO flow

The public route shapes stay as they are.

- Operator document and signing-session writes stay on `/api/v1/documents` and `/api/v1/signing-sessions`. `RequireOperatorShipmentRead` stays the public-edge guard. It is a shipment read check for a tenant-global `PLATFORM_ADMIN`. It is not the document-service credential.
- Shipper and carrier document routes stay `GET`.
- Driver POD stays on `/api/v1/driver/me/shipments/{shipmentId}/pod/uploads` and keeps `requireAssignedDriver` before the gateway calls document-service.
- `CreateSession` still starts only from `READY_FOR_SIGNING`, sets `SIGNING_IN_PROGRESS`, and rechecks eligibility inside the document-then-session lock.
- `AddSignature` keeps the document lock, then the session lock, then `clock_timestamp()` as its own statement after the session lock. Equality of `expires_at` and that timestamp stays overdue.
- In-process `RecoverExpiredSigning` stays available to in-process callers and stays unregistered as an authorized HTTP write. The HTTP route stays the existing `403` handler on both document-service and the gateway, registered before the document wildcard.
- Attachment signature content-type, size, and idempotency rules stay after the caller gate.
- OpenAPI for signing recovery stays `PUBLIC_SIGNING_RECOVERY=DISABLED` and `EXPECTED_PUBLIC_RESPONSE=403`. This contract does not add a public success response and does not treat `bearerAuth` as write authority.

The current hop is not yet able to present these credentials:

- The gateway reverse proxy forwards the incoming request and does not set `X-Internal-Service-Token`.
- `document.Client` sets `X-Tenant-ID` and, for content, `X-Upload-Token`.
- Local Compose sets `AUTH_ENABLED` to false and publishes `8086:8086`. The staging overlay clears the published host port and leaves `document-service:8086` on the Compose network. An unpublished port is not the caller credential.
- `tests/bintrans-e2e-transport-flow` calls `/internal/v1/pod-uploads` on document-service with the tenant header.

A later implementation pairs the service gate with the hop credential in the same change. The gateway attaches `DOCUMENT_WRITE_GATEWAY_TOKEN` only after `AUTH_ENABLED` verification and `RequireOperatorShipmentRead`. It attaches `POD_UPLOAD_GATEWAY_TOKEN` only inside `document.Client` after the driver handler has resolved the assigned driver. `AUTH_ENABLED=false` attaches neither token, so document-service denies the write. The direct e2e caller is updated in that same implementation. Doing the service gate first, without the hop credential, makes current legitimate writes fail closed. This design records that order and does not authorize the code change.

## Out of scope

Qualified signature verification, EDO operator exchange, GAP-B-001, cabinet UI, payment rules, read-route authority, live staging or production probes, secret provisioning, and production rollout are outside this contract. Read routes that still accept `X-Tenant-ID` are not closed by this document.

Independent security review is required before this contract can be treated as approved. Until that review, implementation stays unauthorized.
