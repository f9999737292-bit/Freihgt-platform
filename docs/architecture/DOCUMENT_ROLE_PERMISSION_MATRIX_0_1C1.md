# Document role and permission matrix 0.1C1

Stage: `DOCUMENT-ROLE-PERMISSION-MATRIX-0.1C1`

Policy: `B`

`POLICY_APPROVED=YES`

`MATRIX_APPROVED=NO`

`ROLE_GRANTS_CREATED=NO`

`MIGRATIONS_CREATED=NO`

`IMPLEMENTATION_AUTHORIZED=NO`

Base: `3f306fc407aeda4bdc652d825954292f4a54fad6`

Role policy: pull request [#369](https://github.com/f9999737292-bit/Freihgt-platform/pull/369) at `319bdae9c28eab2490e5640b88b60ddd13587ff4`

Authority contract: pull request [#360](https://github.com/f9999737292-bit/Freihgt-platform/pull/360) at `f4bbc6e3ad4f1ba27c57a6b6e9c68129f9c90544`

Issue: [#343](https://github.com/f9999737292-bit/Freihgt-platform/issues/343)

Mode: design only. This document does not change those pull requests, api-gateway, document-service, identity-service, migrations, or `core.role_permissions`.

`FINAL_VERDICT=READY_FOR_CONTROLLER_DECISION`

Policy B is the approved rule. An `ACTIVE` company membership is required and is not sufficient. The operation also needs a company-scoped permission. This matrix proposes where that permission already has a server source. A cell is not a grant.

## Cell meanings

| Cell | Meaning |
| --- | --- |
| `ALLOW_PROPOSED` | A current server guard already allows this operation for this role on one company. The cell still needs controller approval before any `role_permissions` row is inserted. |
| `DENY_PROPOSED` | No server guard grants this operation to this role. The proposal is to keep the operation denied. |
| `SEPARATE_AUTHORITY_REQUIRED` | This matrix does not decide the cell. The authority is a different contract or an unapproved exception. |

A role title is not a source. `SHIPPER_ADMIN` is not granted create because the name says admin. `FINANCE_MANAGER` is not granted document writes because its seed text mentions closing documents.

## Required check for every document operation

All of the following are required. Missing any one of them is `HTTP_DENIED=YES` and `STORE_WRITES=0`.

1. The user is the verified JWT subject.
2. The tenant is the verified JWT tenant.
3. Identity-service has an `ACTIVE` membership for that user, tenant, and company. `INVITED`, `SUSPENDED`, and `DELETED` do not pass. The lookup shape already used by the gateway is `IdentityClient.ListUserCompanies` (`GET /v1/users/{id}/companies?tenant_id={tenant}&status=ACTIVE`).
4. That same company binding carries the permission for the operation. The binding is `core.user_roles` with that `company_id`, joined to `core.role_permissions` and `core.permissions`. A `user_roles` row with `company_id` null is tenant-global and does not satisfy this step.
5. The document's `owner_company_id` is that company. When the document names a shipment, `transport.shipments` in the same tenant lists that company as `shipper_company_id`, `consignee_company_id`, `carrier_company_id`, or `forwarder_company_id`.
6. document-service has confirmed the internal caller with the dedicated token and exact service name from pull request #360.

`X-Company-ID` from the client is removed before the check. It does not select a company the membership does not contain, and it does not add a permission. With `AUTH_ENABLED=false` the gateway issues no document-write credential, so the write is denied. Signing recovery stays HTTP `403` for every role.

`PLATFORM_ADMIN` does not fill step 4 for every company. The administrative exception in pull request #360 stays unapproved.

## Permission codes

| Code | In `core.permissions` on this base | Operation |
| --- | --- | --- |
| `document.read` | Yes. `000008_seed_permissions.up.sql`. No `role_permissions` row. | Read |
| `document.create` | Yes. Same seed. No role binding. | Create |
| `document.version` | No | Version, file, attachment create, attachment finalize, attachment signature |
| `document.prepare_signing` | No | Ready for signing and create signing session |
| `document.sign` | Yes. Same seed. No role binding. | Session signature by the verified user |
| `document.cancel` | No | Cancel |
| `document.archive` | No | Archive |
| `pod.upload` | No | POD intent, content, and complete |

Gateway and document-service do not read these codes. The only `core.role_permissions` insert in this tree is the manual-spot permission, which is not a document permission.

`document.sign` in the table is the internal signature permission. It is not a legally significant qualified signature. That legal effect needs its own contract. No cell below grants it.

`billing_document.create` is a billing resource. It is not `document.create` and it is not mapped onto this matrix.

### Future migration, not created

A later implementation migration, numbered from the migration head at that time, would insert only the missing codes:

- `document.version`, resource `document`, action `version`
- `document.prepare_signing`, resource `document`, action `prepare_signing`
- `document.cancel`, resource `document`, action `cancel`
- `document.archive`, resource `document`, action `archive`
- `pod.upload`, resource `pod`, action `upload`

That migration would not insert `core.role_permissions`. It would not grant `document.read` to the four read roles below. Those grants wait until the controller approves the cells. This pull request does not add the migration file.

## Role matrix

`CARRIER_ACCOUNTANT` is not inserted by any migration in this tree. Gateway billing, payment, rates, and settlement code references it. `carrierReadRoleCodes` intentionally omits it, and `TestCarrierDocumentDeniesWrongMembership` expects `403` with no downstream call for that code. The row is included so the missing role is not treated as a document role.

| ROLE | READ | CREATE | VERSION AND ATTACHMENTS | PREPARE | SIGN | CANCEL | ARCHIVE | POD |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `SHIPPER_ADMIN` | `ALLOW_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `SEPARATE_AUTHORITY_REQUIRED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` |
| `SHIPPER_LOGIST` | `ALLOW_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `SEPARATE_AUTHORITY_REQUIRED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` |
| `CARRIER_ADMIN` | `ALLOW_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `SEPARATE_AUTHORITY_REQUIRED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` |
| `CARRIER_DISPATCHER` | `ALLOW_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `SEPARATE_AUTHORITY_REQUIRED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` |
| `CARRIER_ACCOUNTANT` | `DENY_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `SEPARATE_AUTHORITY_REQUIRED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` |
| `CONSIGNEE_OPERATOR` | `DENY_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `SEPARATE_AUTHORITY_REQUIRED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` |
| `PROCUREMENT_MANAGER` | `DENY_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `SEPARATE_AUTHORITY_REQUIRED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` |
| `FORWARDER_MANAGER` | `DENY_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `SEPARATE_AUTHORITY_REQUIRED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` |
| `FINANCE_MANAGER` | `DENY_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `SEPARATE_AUTHORITY_REQUIRED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` |
| `PLATFORM_ADMIN` | `SEPARATE_AUTHORITY_REQUIRED` | `SEPARATE_AUTHORITY_REQUIRED` | `SEPARATE_AUTHORITY_REQUIRED` | `SEPARATE_AUTHORITY_REQUIRED` | `SEPARATE_AUTHORITY_REQUIRED` | `SEPARATE_AUTHORITY_REQUIRED` | `SEPARATE_AUTHORITY_REQUIRED` | `SEPARATE_AUTHORITY_REQUIRED` |
| `DRIVER` | `DENY_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `SEPARATE_AUTHORITY_REQUIRED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `SEPARATE_AUTHORITY_REQUIRED` |
| `GOV_INSPECTOR` | `DENY_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `SEPARATE_AUTHORITY_REQUIRED` | `DENY_PROPOSED` | `DENY_PROPOSED` | `DENY_PROPOSED` |

`SIGN` is `SEPARATE_AUTHORITY_REQUIRED` on every row. Create does not imply it. The signer must be the verified JWT subject, and the signer company must be the company that holds a future `document.sign` grant. No role has that grant today. A legally significant signature is a further contract and is not decided by approving an internal permission.

`CANCEL` and `ARCHIVE` are separate codes. Create does not imply either of them. No current guard assigns them, so the proposal is deny.

## Why a cell is proposed

### Read, shipper

`ALLOW_PROPOSED` for `SHIPPER_ADMIN` and `SHIPPER_LOGIST`.

Business scenario: a shipper user opens the document list or one document for the shipper company they selected. The customer shipper document routes are `GET` only.

Server source: `RequireShipperRead` loads `ACTIVE` memberships and calls `MembershipAllowsShipperShipmentRead`. That function requires company type `SHIPPER` and one of those two role codes on that same membership. `PLATFORM_ADMIN` is not sufficient. The client `X-Company-ID` is stripped and replaced only after the membership matches.

This is the only server source for proposing `document.read` on those two roles. It does not propose create, version, prepare, sign, cancel, archive, or POD.

### Read, carrier

`ALLOW_PROPOSED` for `CARRIER_ADMIN` and `CARRIER_DISPATCHER`.

Business scenario: a carrier user opens carrier documents for the selected carrier company. The carrier document routes are `GET` only.

Server source: `RequireCarrierRead` and `MembershipAllowsCarrierRead`. The role set is `carrierReadRoleCodes`: `CARRIER_ADMIN` and `CARRIER_DISPATCHER` on a company of type `CARRIER`. The same file says `CARRIER_ACCOUNTANT` is intentionally absent. `TestCarrierDocumentDeniesWrongMembership` denies the accountant, a `DRIVER` membership, a tenant-global `PLATFORM_ADMIN`, and a role held on another company, with no downstream call.

`Test` coverage of operator routes also denies `CARRIER_ADMIN`, `CARRIER_DISPATCHER`, `CARRIER_ACCOUNTANT`, and `DRIVER` on `GET` and `POST /api/v1/documents` and on version, ready-for-signing, signing-session, cancel, archive, attachment, and session-signature routes. Those denials are why the write columns stay `DENY_PROPOSED`.

### Writes

`DENY_PROPOSED` for create, version and attachments, prepare, cancel, and archive on every company role.

document-service does not consult a role. The public write routes use `RequireOperatorShipmentRead`, which allows a tenant-global `PLATFORM_ADMIN` and does not check a company membership or a `document.*` permission. Policy B and pull request #360 both refuse to treat that guard as company authority. There is no second guard that assigns create, version, prepare, cancel, or archive to a company role. Proposing `ALLOW` from a role name would invent the grant.

### Sign

`SEPARATE_AUTHORITY_REQUIRED` for every role.

`document.sign` exists as a permission code and is bound to no role. `AddSignature` checks that the signer user and company exist in the tenant. Existence is not authority. The verified user must be the signer, and the signer company must be checked on its own. This matrix does not name the role that may sign. Qualified legal signature stays outside this document.

### Driver POD

`SEPARATE_AUTHORITY_REQUIRED` for `DRIVER` in the POD column. `DENY_PROPOSED` for every document column and for every other role in the POD column.

Business scenario: the assigned driver submits proof of delivery for that shipment through `/api/v1/driver/me/shipments/{shipmentId}/pod/uploads`.

Server source: when auth is enabled, `ensureDriverAccess` calls `CanAccessDriverRoutes`, which accepts `DRIVER` only. document-service `CreateUploadIntent` then reads `transport.shipments` for the same tenant and `driver_id`. The company stored on the POD document is the company on the server driver record. An `ACTIVE` carrier membership, `CARRIER_ADMIN`, or `document.create` does not assign the driver. `pod.upload` does not grant create, version, prepare, sign, cancel, or archive.

### Platform admin

`SEPARATE_AUTHORITY_REQUIRED` on every column.

The current operator surface lets a tenant-global `PLATFORM_ADMIN` through `RequireOperatorShipmentRead` without a company membership. The carrier document test shows that same role is forbidden on the carrier document route. Source-authority comments in shipment-service and rfx-service already say the tenant-global role does not replace membership. Policy B does not convert the operator wildcard into a company permission. The cross-company administrative exception remains unapproved. Until a controller defines that exception as its own permission, with audit and tests, the company-scoped check denies `PLATFORM_ADMIN`.

### Other roles

`CONSIGNEE_OPERATOR` is seeded and has no document route. `consignee_company_id` is a shipment column, not a document principal. `PROCUREMENT_MANAGER` and `FORWARDER_MANAGER` have no document guard. `FINANCE_MANAGER` is tied to billing registers and `billing_document.create`, not to document-service. `GOV_INSPECTOR` is a global inspection role with no document guard. `CARRIER_ACCOUNTANT` is not seeded and is denied on carrier document reads. All of those document cells are `DENY_PROPOSED`, except sign, which stays `SEPARATE_AUTHORITY_REQUIRED`.

## Company-scoped check

For a document write or a company document read, the gateway does this after it has deleted client identity and internal-authority headers:

1. Reject the write when `AUTH_ENABLED=false`. Do not attach a service token.
2. Verify the JWT. Take the user and tenant from the token.
3. Resolve the company from the document `owner_company_id` or, for the existing shipper and carrier `GET` routes, from the company query the route already requires. Ignore client `X-Company-ID`.
4. Require an `ACTIVE` membership for that user, tenant, and company.
5. Require `user_roles.company_id` for that same company, then `role_permissions` for the operation code. A missing permission row is a denial. `document.read` does not satisfy `document.create`. `document.create` does not satisfy `document.version`, `document.prepare_signing`, `document.sign`, `document.cancel`, or `document.archive`.
6. Require the shipment relationship in the same tenant when the document names a shipment.
7. Only then set the server identity headers and, for a write, add the dedicated internal token and `X-Internal-Service-Name=api-gateway` from server configuration.

document-service repeats the caller-token check before the handler. It does not accept the permission decision from a client header. A direct call with a spoofed company or permission header and no valid caller credential writes nothing.

POD uses a different sequence. The gateway requires the `DRIVER` role, the shipment in the JWT tenant, and `transport.shipments.driver_id` equal to the server driver id. It then sends `POD_UPLOAD_GATEWAY_TOKEN`, not `DOCUMENT_WRITE_GATEWAY_TOKEN`. `pod.upload` is not looked up as a substitute for steps 4 and 5 of a document write.

## Effect on existing GET routes

The four `ALLOW_PROPOSED` read cells describe the guards that already exist. They do not widen shipper or carrier `GET`.

Switching those `GET` routes onto `document.read` in `core.role_permissions` before any grant exists would deny the reads that pass today, because no role has that permission row. This proposal does not make that switch. The current `RequireShipperRead` and `RequireCarrierRead` allowlists stay until a controller approves the read cells and a later change inserts only those grants.

Operator `GET /api/v1/documents` remains the tenant-global `PLATFORM_ADMIN` surface. This matrix does not adopt it as `document.read`. It also does not remove it. Changing or keeping that surface is the unapproved administrative exception.

Shipper and carrier routes gain no `POST`. Consignee still has no document route.

## Alignment with pull request #360

These parts of the authority contract already match Policy B and this matrix. They do not need a wording change to stay in force:

- dedicated internal caller authentication on document-service
- separate tokens for document writes, POD uploads, and POD action evidence
- tenant taken from the verified JWT after the caller is confirmed
- client `X-Tenant-ID`, `X-User-ID`, and `X-Company-ID` are not authority
- header stripping independent of `AUTH_ENABLED`
- no document-write credential when `AUTH_ENABLED=false`
- signing recovery HTTP `403`
- empty and unknown tokens fail closed
- POD assignment denial with `STORE_WRITES=0`, while the assignment query may read
- one rollout of gateway, document-service, `document.Client`, credentials, and E2E tests
- no staging or production apply in the design pull request

The company section of pull request #360 still describes `ACTIVE` membership as the company grant. Under the approved Policy B that sentence is incomplete. A later edit of that pull request, not performed here, would make these changes:

1. After the membership match, require the operation permission on that same company binding. `ACTIVE` membership without the permission is a denial and `STORE_WRITES=0`.
2. State that the permissions do not imply each other. Name `document.read`, `document.create`, `document.version`, `document.prepare_signing`, `document.sign`, `document.cancel`, and `document.archive`.
3. Keep the signer rules: `signer_user_id` is the verified subject, and `signer_company_id` is the company that holds `document.sign`. State that legal signature is a separate contract.
4. Leave the `PLATFORM_ADMIN` exception unapproved. Do not add that role to the company permission grant.
5. State that `pod.upload` plus the `DRIVER` role plus `transport.shipments.driver_id` is not a `document.*` permission.
6. Replace the matrix cells that call membership alone the company grant. The grant is membership plus the operation permission plus the shipment relationship.
7. Add the negative tests T1 through T11 from this document.
8. Do not add a positive write test that succeeds on membership alone.
9. Do not change the caller credential, the header algorithm, recovery, token separation, or the rollout order.
10. Leave `COMPANY_POLICY_DECISION=REQUIRED` for the administrative exception.

Until that edit, an implementation of pull request #360 that stops at membership would allow every active member to write. That is Policy A. Policy B is the approved policy, so that implementation is not authorized by this matrix.

## Tests

These tests are specified for a later implementation. They are not added here. A denied mutation has `HTTP_DENIED=YES` and `STORE_WRITES=0`. A membership, permission, or assignment read is allowed. A repeat of a denial writes nothing. Reaching a handler is not success.

| Id | Case | Result |
| --- | --- | --- |
| T1 | `ACTIVE` membership in company C, no `document.create` | Deny create. No document row. |
| T2 | The same membership has `document.read` only | Deny create. Read of an authorized company document may still follow the existing shipper or carrier `GET` guard. |
| T3 | The membership has `document.create` and not `document.sign` | Create may proceed only if a future controller has approved create. Sign is denied. No signature row. |
| T4 | The membership has `document.create` and not `document.cancel` | Cancel is denied. The document status stays unchanged. |
| T5 | The permission is on company A and the document company is B | Deny. Company A is unchanged. |
| T6 | Tenant-global `PLATFORM_ADMIN`, no company permission on C | Deny the company-scoped write. The unapproved admin exception is not a pass. |
| T7 | `DRIVER`, valid POD credential, shipment exists, driver is not `transport.shipments.driver_id` | Deny. No upload intent, document, or object. Repeat writes nothing. |
| T8 | Client `X-Company-ID` names a company the membership does not grant | The header is removed. The operation is denied unless the server company and permission already allow it. The spoofed company gains nothing. |
| T9 | Document or shipment tenant differs from the JWT tenant | Deny. No cross-tenant row is written. |
| T10 | `AUTH_ENABLED=false`, including a client copy of a gateway token | No server write credential is attached. Deny. |
| T11 | Signing recovery, including a user who holds every document permission and an assigned driver | HTTP `403`. `STORE_WRITES=0`. |

Positive cases for the operations this matrix actually proposes:

- `SHIPPER_ADMIN` with an `ACTIVE` membership on a `SHIPPER` company reads that company's document through the shipper `GET` route. The stored tenant and owner company match. A document of another company is not returned.
- `SHIPPER_LOGIST` on that same kind of membership has the same read result.
- `CARRIER_ADMIN` with an `ACTIVE` membership on a `CARRIER` company reads that company's document through the carrier `GET` route.
- `CARRIER_DISPATCHER` on that same kind of membership has the same read result.
- An assigned `DRIVER` creates one POD intent, stores content with the issued upload token, and completes that same upload. The shipment `driver_id` and the server driver company match. A second complete returns the same document and does not create another one.

No positive test is proposed for create, version, prepare, sign, cancel, or archive. Those cells are not `ALLOW_PROPOSED`. A controller who later approves one of them adds the positive test for that operation in the same decision.

## Questions for the controller

1. Which company roles, if any, should later receive `document.create` for their own company?
2. Should version and attachments be the same set of roles as create, or a narrower set?
3. Who may prepare a document for signing?
4. Who may hold internal `document.sign`? This does not decide a legally significant signature.
5. Who may cancel, and who may archive? The answer may be two different sets.
6. Should the four `ALLOW_PROPOSED` read cells be approved, and only then written to `role_permissions`?
7. Should operator `GET /api/v1/documents` for tenant-global `PLATFORM_ADMIN` remain, be removed, or be replaced by a named company permission?
8. Is a cross-company administrative exception wanted at all? It is not approved here.
9. Should consignee or government inspection receive a future read? No server document guard exists for either.
10. Should `CARRIER_ACCOUNTANT` be seeded for billing only, with document access left denied?

`MATRIX_APPROVED=NO` until those answers exist. No grant is created by publishing this matrix.
