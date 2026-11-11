# Document write security discovery 0.1A

Stage: `DOCUMENT-WRITE-SECURITY-0.1A`

Issue: [#343](https://github.com/f9999737292-bit/Freihgt-platform/issues/343)

Base: `origin/main` `3f306fc407aeda4bdc652d825954292f4a54fad6`

Mode: security discovery. This document does not change an API, a handler, a schema, staging, or production.

`VERIFIED_INTERNET_EXPOSURE=NOT_ESTABLISHED`

No live request was sent to staging or production. Reachability below is what the Compose files and the HTTP routers declare. A published port in local Compose is not evidence that a public host exposes document-service.

`GAP-B-001` stays open. No document write-authority contract is defined.

## Signing recovery

Signing recovery stays fail-closed on this base.

- document-service registers `POST /v1/documents/{id}/signing-recovery` as `handlers.RejectSigningRecovery`. The handler returns `403` and does not call `RecoverExpiredSigning`.
- api-gateway registers `POST /api/v1/documents/{id}/signing-recovery` before the document wildcard as `document.RejectSigningRecovery`. The handler returns `403` and does not proxy.
- OpenAPI marks the public operation `PUBLIC_SIGNING_RECOVERY=DISABLED` and `EXPECTED_PUBLIC_RESPONSE=403`.

`RecoverExpiredSigning` remains an in-process operation. It is not an HTTP route.

## Trust model observed

document-service write handlers take the tenant from `X-Tenant-ID` through `trustedTenantID`, or, for POD upload, through a direct parse of the same header. The service does not verify a JWT, a gateway signature, or an internal service token on those routes. A body or query `tenant_id` must match the header when it is present. The header itself is the authority once the request reaches the service.

`X-User-ID` is optional for attachment actor metadata. Signing uses `signer_user_id` from the body and checks that the user exists in the header tenant. `owner_company_id` and `signer_company_id` are checked with `CompanyExists` in that tenant. They are not bound to a caller company membership.

api-gateway, when `AUTH_ENABLED` is true, strips client identity headers and sets `X-Tenant-ID` and `X-User-ID` from the verified JWT before the route guard. The document and signing-session wildcards then use `RequireOperatorShipmentRead`. With auth enabled, that guard allows only a verified user who has tenant-global `PLATFORM_ADMIN`. The guard is a shipment read check. It is not a document write-authority contract. Shipper and carrier document routes stay `GET` only.

When `AUTH_ENABLED` is false, gateway auth does not replace identity headers, and `RequireOperatorShipmentRead` forwards the request. Local `infrastructure/docker-compose/docker-compose.yml` sets `AUTH_ENABLED: "false"` and publishes `8086:8086` for document-service.

The staging overlay `docker-compose.bintrans-ct-staging.yml` clears published ports for document-service (`ports: !override []`) and is applied together with the base Compose file. Clearing the host port does not remove `document-service:8086` from the Compose network. `docker-compose.staging-shadow.yml` defaults `AUTH_ENABLED` to true for the shadow runtime. The running staging environment, its firewall, and its effective environment were not inspected.

`/internal/v1/pod-action-evidence` is a separate group. It uses `internalAuth.Middleware` and `RequireShipmentServiceCaller`. That protection does not cover `/internal/v1/pod-uploads` or `/v1/documents`.

## Route matrix

| ROUTE | CALLER | TRUST_BOUNDARY | AUTHORITY_SOURCE | TENANT_VALIDATION | DIRECT_ACCESS_RISK | REQUIRED_FIX | TEST_REQUIRED |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `POST /v1/documents` | Gateway wildcard, or any client that reaches document-service | Gateway JWT plus `PLATFORM_ADMIN` only while `AUTH_ENABLED=true`. Service has no caller check | `X-Tenant-ID`. Body `tenant_id` must match. `owner_company_id` must exist in that tenant | Header is accepted as the tenant. Company existence is not caller membership | A client that reaches the service listener can name the tenant | Later stage: require a server-authenticated caller before the handler | Direct `NewRouter` test: spoofed `X-Tenant-ID` and bearer token do not write. Gateway test: non-admin is denied when auth is on |
| `POST /v1/documents/{id}/versions` | Same | Same | Header tenant. Version is stored under that document and tenant | Header tenant | Same as create | Same caller gate | Same, plus a cross-tenant document id |
| `POST /v1/documents/{id}/files` | Same | Same | Header tenant | Header tenant | Same | Same caller gate | Same |
| `POST /v1/documents/{id}/attachments` | Same | Same | Header tenant. Optional `X-User-ID` is stored as actor when present | Header tenant. Query `tenant_id` must match | Same. Actor header is not authenticated by the service | Same caller gate. Do not treat `X-User-ID` as authority | Same, including a spoofed `X-User-ID` |
| `POST /v1/documents/{id}/attachments/{attachmentId}/finalize` | Same | Same | Header tenant | Header tenant | Same | Same caller gate | Same |
| `POST /v1/documents/{id}/attachments/{attachmentId}/signatures` | Same | Same | Header tenant and body signature metadata | Header tenant | Same | Same caller gate | Same |
| `POST /v1/documents/{id}/ready-for-signing` | Same | Same | Header tenant | Header tenant | Same | Same caller gate | Same |
| `POST /v1/documents/{id}/signing-sessions` | Gateway `/api/v1/documents/*`, or direct service | Same | Header tenant. `expires_at` is validated by the service | Header tenant | Same | Same caller gate. Do not weaken the in-transaction session checks | Same, plus the existing concurrency tests |
| `POST /v1/signing-sessions/{id}/signatures` | Gateway `/api/v1/signing-sessions/*`, or direct service | Same | Header tenant. Signer user and company must exist in that tenant | Header tenant | Same | Same caller gate | Same, plus foreign-tenant signature rejection |
| `POST /v1/documents/{id}/cancel` | Gateway document wildcard, or direct service | Same | Header tenant | Header tenant | Same | Same caller gate | Same |
| `POST /v1/documents/{id}/archive` | Gateway document wildcard, or direct service | Same | Header tenant | Header tenant | Same | Same caller gate | Same |
| `POST /v1/documents/{id}/signing-recovery` | Registered on both gateway and document-service | Fail-closed handler. No service call | None. Headers are ignored | Not used | No recovery mutation | None in this discovery. Keep the `403` | Existing `TestDocumentServiceRouterDeniesSigningRecovery` and `TestSigningRecoveryIsNotProxied` |
| `POST /internal/v1/pod-uploads` | Not registered on the gateway. Direct service or another container on the Compose network | No internal-auth middleware on this route | `X-Tenant-ID` plus body `shipmentId`, `driverId`, `ownerCompanyId` | Header parse only | A client that reaches the listener can create an upload intent for the named tenant | Later stage: caller identity distinct from the client tenant header | Direct router test that a tenant header alone is not sufficient |
| `PUT /internal/v1/pod-uploads/{uploadId}/content` | Same | Same | Header tenant plus `X-Upload-Token` | Header tenant | Token checks the upload. It does not prove who may open the listener | Same caller gate, keep the upload token | Direct router test |
| `POST /internal/v1/pod-uploads/{uploadId}/complete` | Same | Same | Header tenant, body `driverId`, checksum | Header tenant | Same | Same caller gate | Direct router test |

## Remediation plan

This discovery does not implement the fix.

1. Define a document write-authority contract. `RequireOperatorShipmentRead` and `PLATFORM_ADMIN` remain a shipment read guard until that contract exists.
2. In a later implementation stage, make document-service write routes fail closed unless the caller is authenticated by the server. A client-supplied `X-Tenant-ID`, `X-User-ID`, `X-Company-ID`, bearer token, or body tenant must not be enough on a direct connection.
3. Apply that caller rule to `/v1/documents` writes, `/v1/signing-sessions/{id}/signatures`, and `/internal/v1/pod-uploads`. Do not copy it onto shipper or carrier `GET` routes, and do not reopen signing recovery.
4. Keep `/internal/v1/pod-action-evidence` on its existing internal token and shipment-service caller check. Do not treat that check as proof that POD upload is protected.
5. Add the route tests in the matrix. Use the real `document-service.NewRouter` for direct access and the real gateway router for the public edge.
6. Decide deployment exposure separately: local Compose publishes `8086`; the staging overlay clears that publication; the Compose DNS name remains. Do not close the review by unpublishing a port alone. Do not roll the result to production before the risk is accepted.
7. Do not change the public API, role model, or schema as part of the discovery.

## Out of scope

Qualified signature verification, EDO operator exchange, GAP-B-001, UI, and live staging or production probes were not part of this pass.
