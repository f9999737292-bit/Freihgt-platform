# FINANCE-TRANSPORT-ORDER-DELEGATED-AUTHORITY-0.2E

Design and acceptance contract for creating a transport order and `SNAPSHOT_V1` on behalf of a грузовладелец (shipper) by another company.

`POLICY=SERVER_VERIFIED_DELEGATION`

`POLICY_APPROVED=YES`

`IMPLEMENTATION_AUTHORIZED=NO`

`COMMERCIAL_PILOT_READY=NO`

`FINAL_VERDICT=READY_FOR_J_R3D_DELTA_REVIEW`

Base at authoring was `3f306fc407aeda4bdc652d825954292f4a54fad6`. Related discovery is PR #363 (`docs/engineering/FINANCE_SNAPSHOT_AUTHORITY_0_2D.md`). Revision 0.2E-R1 closed the first security-review findings on award-scope binding, the transport-order-service actor boundary, and idempotency across shippers. Revision 0.2E-R2 closed Agent J findings `J-FIN02E-R2-001`, `J-FIN02E-R2-002`, and `J-FIN02E-R2-003`. Those three remain closed. Revision 0.2E-R3 clarified grant replay, grant lifecycle, and the negative-test matrix after `PASS_ARCHITECTURE_ONLY`. Those R3 findings remain closed. Revision 0.2E-R3D closes `J-FIN02E-R3D-001` and `J-FIN02E-R3D-002`. Agent G does not record the R3D delta review as complete.

This document does not change transport-order-service, billing, VAT, rounding, PostgreSQL migrations, or PRs #341, #353, #356, and #363.

An existing company relationship is not proof of the right to create a transport order.

## 1. Company relationship model that exists

Company relationships live in company-service, migration `000100_company_carrier_relationship_foundation_v0_1c.up.sql`, with revoke, cancel, and execution-pin follow-ups in `000101`, `000102`, and `000103`.

| Object | What it stores |
| --- | --- |
| `company.carrier_relationship_requests` | `source_type` in `SHIPMENT`, `RFX_AWARD`, `AWARD_TRANSPORT_ORDER_LINK`; `source_tenant_id`; `shipper_carrier_company_id`; `requesting_company_id`; `requesting_user_id` |
| Request head status | `PENDING`, `ACCEPTED`, `EXPIRED`, `CANCELLED` |
| `company.carrier_relationships` | `shipper_tenant_id`, `shipper_carrier_company_id`, `operating_tenant_id`, `operating_carrier_company_id` |
| Relationship version and head | `ACTIVE` or `REVOKED` |
| `company.carrier_relationship_audit` | Actor, tenant, selected company, request or relationship id, action. This is the relationship lifecycle audit |
| `company.carrier_relationship_activation_authorizations` and `transport.transport_execution_carrier_relationship_pins` (`000103`) | A locked relationship version pinned to a shipment execution |

Server checks that exist, in `services/company-service/internal/domain/relationship.go`:

- Create a request only when the actor is active, the membership is active, and the role matches the source. Shipment creators: `SHIPPER_ADMIN`, `SHIPPER_LOGIST`, `FORWARDER_MANAGER`. RFx award and award-link creators add `PROCUREMENT_MANAGER`.
- Accept and operating-side revoke require `CARRIER_ADMIN` plus an active membership.
- `TenantGlobalPlatformAdmin` returns true from `CanCreateRelationshipRequest`, `CanAcceptRelationship`, and `CanReadRelationshipRequest` without a company role.
- Source create from rfx-service and shipment-service uses a dedicated credential. An empty token, a shared internal token, or a lost transport call fails closed. That check confirms the source subject. It does not grant order creation.
- The only gateway path that restores a client `X-Company-ID` after the identity strip is `POST /api/v1/carrier-operating-relationship-acceptances`. That route accepts a carrier relationship. It is not order-create authority.

`services/transport-order-service` and `services/api-gateway/internal/transportorderrbac` do not reference `carrier_relationship`. Priced create does not call company-service.

EDO `documents.document_relationships.relationship_type` (`CORRECTS`, `REPLACES`, `RELATED_TO` in migration `000087`) is a document link. It is not this model.

## 2. Which relationship types allow a forwarder to create an order

None.

`SHIPMENT`, `RFX_AWARD`, and `AWARD_TRANSPORT_ORDER_LINK` are source types of a carrier operating relationship. `ACTIVE` means the operating carrier relationship version is current. `000103` pins that version to execution. `FORWARDER_MANAGER` may request that relationship. None of those facts is `CREATE_TRANSPORT_ORDER_ON_BEHALF`.

A future implementation must deny order create when the only positive evidence is an `ACTIVE` carrier relationship, a pending request, an acceptance token, or an execution pin.

## 3. Server-owned permission

The permission code is `CREATE_TRANSPORT_ORDER_ON_BEHALF`.

It is a row the server writes and the server reads back. The client does not send the permission code, the role, or the grant status. A client-supplied grant id is a locator only. The server does not use that locator as the lookup key.

The lookup key is the authority tuple: verified tenant, verified actor company, verified grantee user, requested shipper company, and permission code `CREATE_TRANSPORT_ORDER_ON_BEHALF`. The server loads the `ACTIVE` row for that tuple. At most one `ACTIVE` row may exist for it. Zero rows is a denial on the delegated path. More than one `ACTIVE` row is a fail-closed denial. When the client sends a locator, it must equal the `grant_id` of that one row. A locator that names any other row, including a `REVOKED` row, is a denial. The server does not switch to the row named by the locator.

The same load runs on a first create and on an idempotency replay. An idempotency key does not carry authority. The server re-reads membership, role, and the current grant for the actor and shipper named in the request before it uses the key. It does not read the stored order to decide which grant to load.

The permission is absent from the current schema. No role constant and no snapshot column carries it.

## 4. Checks required before rate resolution and insert

Evaluate the following sequence inside transport-order-service, before the contract-rate call and before `CreatePricedOrder`. A failure writes no order, no snapshot, and no idempotency row. A failure also returns no stored transport order id and no stored snapshot id.

The sequence applies to the first create and to every replay. Replay does not waive it.

1. Establish the trusted actor identity (section 12). Client headers are not that identity.
2. Check the verified tenant and the verified actor company. A body tenant or a body actor company that differs is a denial.
3. Re-read current membership and require a create role. This step does not open the idempotency table and does not use a stored `authority_grant_id`.
4. Authorize the actor and shipper named in this request. When they are the same company, same-company authority is enough. When they differ, load the single `ACTIVE` `CREATE_TRANSPORT_ORDER_ON_BEHALF` grant for the authority tuple in section 3 and require the lifecycle result in section 13. The request body selects this path. A stored order does not.
5. Look up `idempotency_key` only after steps 1–4 succeed.
6. After that lookup, compare the stored `authority_grant_id` and `authority_grant_version` with the authority from step 4. A null stored grant id is acceptable only for a same-company stored row whose shipper is the actor company. A non-null stored grant id must be the same grant id as step 4, and the fresh version must equal the stored version. A different grant id or a different version is a denial. Replay does not adopt the new id or the new version.
7. Immediately before the response returns the stored order and snapshot, re-read that authority and compare tenant, actor company, shipper, and request hash. A revoke, a version mismatch, a failed read, or a different identity is a denial.

No step before step 5 reads the stored order to classify the grant. There is no automatic succession from an older grant to a newer grant. A controller would have to decide any transfer of authority in a separate policy. This contract does not make that decision. The mismatch path is fail-closed (NT-D36).

A denial at any step writes no order, no snapshot, and no idempotency row. The response omits the stored transport order id, the stored snapshot id, and commercial fields such as amount, currency, and carrier price.

| Check | Rule |
| --- | --- |
| Tenant | JWT tenant, grant tenant, actor company tenant, and shipper company tenant are the same value. Body `tenant_id` that differs is forbidden, as it is today |
| Actor user | Active user. Membership in the actor company is active |
| Actor company | The company transport-order-service has confirmed for this user by its own actor check (section 12). A gateway header, including a header forwarded while `AUTH_ENABLED` is false, is not that confirmation |
| Shipper company | Body `shipper_company_id` equals the shipper named on the grant. The company exists in the same tenant |
| Role | The actor's membership role is one of the create roles (`PROCUREMENT_MANAGER`, `SHIPPER_ADMIN`, `FORWARDER_MANAGER`) and is included in the grant's allowed roles. `CARRIER_ADMIN`, `CARRIER_DISPATCHER`, and `SHIPPER_LOGIST` do not receive this permission from their current read or relationship roles |
| Grant activity | Section 13. The only usable result is a fresh `ACTIVE` read of the current grant. `REVOKED`, any other observed status, and an unreadable grant are denials. A status cached at login is not this check |

Same-company create stays a separate path. It applies only when body `shipper_company_id` equals the verified actor company. That path does not require a delegation grant and does not use a carrier relationship.

## 5. Arbitrary `shipper_company_id`

Forbidden.

Today `BuildSnapshotFromResolve` sets snapshot `buyer_company_id` from body `shipper_company_id` (`services/transport-order-service/internal/client/contractrate/client.go`). The repository stores that buyer. The actor company is stored only on `transport.transport_order_create_idempotency`. Nothing compares the two.

Required behavior:

- No grant for the exact tuple `(tenant, actor user, actor company, shipper company, role)` means deny.
- A grant for shipper S does not authorize body shipper C.
- The server does not search for some other active relationship and substitute it.
- Omitted `shipper_company_id` does not default to a company taken from the client.

## 6. `PLATFORM_ADMIN`

Current `policyAllows` in `services/api-gateway/internal/transportorderrbac/policies.go` returns true for `PolicyCreate` when `isPlatformAdmin` is true, including when actor kind is not `BUYER`, after a membership match. `CanCreateRelationshipRequest` has the same platform-admin short-circuit. Neither short-circuit may authorize on-behalf order create.

`PLATFORM_ADMIN` needs its own grant:

- `subject_kind=PLATFORM_ADMIN`
- `grantee_user_id` equals the verified admin user
- tenant, shipper company, and buyer company are named on that row
- status is `ACTIVE`
- the row's permission code is `CREATE_TRANSPORT_ORDER_ON_BEHALF`

The admin role does not skip company checks. An admin who is a member of the shipper company uses the same-company path. That membership is not a delegation. An admin who is a member only of company A, with shipper B and no platform grant for B, is denied.

## 7. Internal RFx award to transport order

Public award conversion and internal priced create are different checks.

RFx `ConvertAwardToTransportOrders` (`services/rfx-service/internal/service/award_conversion_service.go`):

1. `requireBuyerEventAccess` loads the event in the actor tenant and calls `requireOwnerCompanyAccess`.
2. `requireOwnerCompanyAccess` requires a buyer actor whose buyer-company list contains `event.OwnerCompanyID`. A non-member receives not found.
3. The transport-order call then sets `ShipperCompanyID` and `ConsigneeCompanyID` to `event.OwnerCompanyID`, and `ActorCompanyID` to that same owner company.

That RFx path is same-company membership. It is not a forwarder creating an order for a different shipper. `AWARD_TRANSPORT_ORDER_LINK` remains a carrier-relationship source type.

Transport-order `CreateFromAwardScope` (`priced_transport_order_service.go`) then forces actor kind `BUYER` and continues into priced create. The handler reads `actor_company_id` and `shipper_company_id` from the body. It does not load membership and it does not load a grant. The internal service token is the route gate. The token is not a company grant.

Required award-scope behavior. transport-order-service confirms the whole chain itself, before the rate call and before insert. A failed link writes no order, no snapshot, and no idempotency row.

| Link | What transport-order-service confirms |
| --- | --- |
| Trusted internal caller | The inbound credential is the dedicated `rfx-service` award-caller credential defined below. A missing credential, a shared multi-service token, or a token issued to another service stays unauthorized and writes nothing (NT-6, NT-D21). That credential is only this link |
| RFx event | transport-order-service loads the award-conversion subject from rfx-service. The body is not that read. An unknown event is a denial |
| Event owner company | The owner is `owner_company_id` on the loaded subject. The service does not take the owner from body `actor_company_id` or body `shipper_company_id` |
| Lot | When the body names `rfx_lot_id`, the loaded subject says that lot exists on that event. A missing lot is a denial. An event-level conversion, with no lot id, is allowed only when the loaded subject says the award has no lot scope |
| Award status | The loaded event status is `AWARDED`, the status `ValidateAwardConversionEventStatus` already requires. Any other status is a denial |
| Winning carrier | The loaded subject names `winning_carrier_company_id`. Body `carrier_company_id` equals that company. A different carrier is a denial |
| Actor company | On the same-company path the body actor company equals the loaded event owner. On the delegated path it equals the actor company on an `ACTIVE` `CREATE_TRANSPORT_ORDER_ON_BEHALF` grant that this service loaded, including grant version |
| Shipper company | The body shipper equals the loaded event owner. On the delegated path that same company is the grant shipper |
| Tenant | Request tenant, loaded event tenant, grant tenant, actor company tenant, and shipper company tenant are one value |
| Allowed creation | Every link above agrees, and the idempotency sequence in section 4 has already authorized the actor. Only then the service may create one order and one `SNAPSHOT_V1`, or return the stored pair for the same authorized request |

A valid service token together with body `actor_company_id` equal to body `shipper_company_id` is not this chain. Those two facts do not identify the event owner and do not authorize an arbitrary company in the tenant (NT-D11).

rfx-service `requireOwnerCompanyAccess` remains the caller's own check. It does not replace the load inside transport-order-service. A grant whose shipper differs from the loaded event owner does not authorize award conversion. The service loads any `CREATE_TRANSPORT_ORDER_ON_BEHALF` grant itself. A statement from rfx-service that a delegation exists is not that load.

`POST /internal/v1/rates/resolve` prices a snapshot. It does not prove that the lot exists, that the event is `AWARDED`, that the carrier won, or that the event owner is the company in the body. A successful resolve is not a substitute for the award-conversion subject or the grant read (NT-D28).

### Trusted RFx → TMS channel

The future implementation uses three dedicated credentials. This PR does not add them.

| Call | Caller identity | Acceptable credential | Rejected credential |
| --- | --- | --- | --- |
| rfx-service calls `POST /internal/v1/transport-orders/from-award-scope` | `rfx-service` | Dedicated award-caller credential issued only to rfx-service | A shared multi-service token, or a token issued to shipment-service, billing-register-service, company-service, document-service, or contract-rate-service |
| transport-order-service reads the award subject | `transport-order-service` | Dedicated RFx read credential issued only to transport-order-service | The inbound award-caller credential, a shared token, or another service's token |
| transport-order-service reads the grant | `transport-order-service` | Dedicated company-grant read credential issued only to transport-order-service | A shared token, or a token whose service identity is not transport-order-service |

A shared internal token proves only that some caller knows a platform secret. It does not prove that the caller is rfx-service.

Server-owned event read, required before rate resolution and before insert:

`GET /internal/v1/rfx-events/{eventId}/award-conversion-subject`

- Caller: transport-order-service, with the dedicated RFx read credential.
- Query: `tenant_id`, and `lot_id` when the award scope names a lot.
- Response fields: `event_id`, `tenant_id`, `owner_company_id`, `event_status`, `award_id`, `winning_carrier_company_id`, `lot_id`, `lot_exists`, `lot_scoped`, `source_version`.
- Allow only when `tenant_id` equals the verified tenant, `event_status` is `AWARDED`, `owner_company_id` equals the body shipper, and `winning_carrier_company_id` equals body `carrier_company_id`.
- When `lot_id` is present, `lot_exists` is true and the lot belongs to `event_id`. When `lot_id` is absent, `lot_scoped` is false.
- The stored award link records `event_id`, `award_id`, `lot_id` when present, and `source_version` so the order points at the subject that authorized it.

Server-owned grant read, required on the delegated path before idempotency lookup:

`GET /internal/v1/transport-order-authority-grants/active`

- Caller: transport-order-service, with the dedicated company-grant read credential.
- Query: `tenant_id`, `actor_company_id`, `actor_user_id`, `shipper_company_id`, `permission_code=CREATE_TRANSPORT_ORDER_ON_BEHALF`.
- Response fields: `grant_id`, `grant_version`, `status`, `tenant_id`, `subject_kind`, `actor_company_id`, `grantee_user_id`, `shipper_company_id`, `buyer_company_id`, `permission_code`, `allowed_role_codes`.
- The service queries by the authority tuple in section 3. It uses the returned `grant_id` and `grant_version` only when the response contains exactly one `ACTIVE` row.
- A client grant id is the locator from section 3. It must equal that row. It is not a second lookup key.
- `status` must be `ACTIVE`. Any other status, including `REVOKED`, is a denial. Expiry is not an additional status unless a controller later selects option E1.

Fail closed:

- The RFx subject read times out, returns a transport error, or returns an unavailable status. Deny. Write no order, no snapshot, and no idempotency row.
- The company-service grant read times out, returns a transport error, or returns an unavailable status on a delegated create or a delegated replay. Deny. Write nothing, and return no stored order id or snapshot id.
- The service does not fill the event owner, the award status, the winning carrier, or the grant from the request body when a read fails.

### J-FIN02E-R3-INFO-01 Award source freshness

`source_version` on the award-conversion subject can change after transport-order-service has read it and before the order insert commits. The future implementation proposes this check and does not contain it today:

- Hold the `source_version` returned by the award-subject read.
- Immediately before the atomic insert of the order, snapshot, and idempotency row, compare that version with a new server-owned read, or make the insert conditional on the same version inside the writing transaction.
- When the version differs, or the freshness read fails, roll the transaction back. Write no order, no snapshot, and no idempotency row.
- An equivalent server guarantee is a single rfx-service operation that reserves that `source_version` for this conversion and fails closed when the version has moved. This contract does not choose the transport. It requires one of those two guarantees before an implementation can treat the award subject as stable.

This check is a proposal. It is not implemented on main.

## 8. Four parties

| Party | Meaning | Today | Required on a delegated create |
| --- | --- | --- | --- |
| Actor company | Company the user is acting as | Idempotency row only | Verified membership company. Stored on the audit row and on the snapshot as the actor, not as the buyer |
| Грузовладелец (shipper) | Cargo owner on `transport.transport_orders.shipper_company_id` | Body value, existence check in the tenant | Equals the grant's shipper company |
| Buyer | Commercial buyer on `transport.transport_order_rate_snapshots.buyer_company_id` | Copied from body `shipper_company_id` | Equals the grant's buyer. Under this policy the grant buyer is the shipper company, the represented грузовладелец. The actor company is not the buyer |
| Payer | Party that pays freight | Not a column of priced create or of the snapshot. Later billing uses the register customer, and closing documents copy that customer into the document buyer | Delegated create does not write a payer and does not copy the actor company into a payer field. Buyer and payer stay different fields |

Consignee remains the order consignee. This policy does not treat consignee as buyer, payer, or grantor.

Settlement that later copies snapshot buyer onto the settlement is unchanged by this contract. That copy must not be read as a payer decision.

## 9. `SNAPSHOT_V1` buyer and authority source

On an allowed delegated create the snapshot must persist, in the same insert as the order:

- `buyer_company_id` = grant buyer = grant shipper
- `actor_company_id` = verified actor company, different from the buyer on this path
- `authority_grant_id` and `authority_grant_version` = the row that was `ACTIVE` at insert time

The snapshot mutation trigger already rejects update and delete. The grant reference is written once. A later revoke does not rewrite the snapshot. Revoke stops a later create. It does not rewrite history.

A delegated insert with a null grant id is a failed create.

Same-company create stores `buyer_company_id` = actor company = shipper, and a null `authority_grant_id`.

Current `000051` columns include `buyer_company_id` and have no actor company and no authority grant. The idempotency table is `(tenant_id, actor_company_id, idempotency_key)` plus hashes and ids. It is not the authority record.

### Idempotency across shippers

`ComputeCreateRequestHash` includes `shipper_company_id`. The future implementation keeps the shipper inside that hash.

When a later request repeats `tenant_id`, `actor_company_id`, and `idempotency_key` with a different `shipper_company_id`:

- The service returns a conflict denial.
- The denial does not insert a second order and does not insert a second snapshot.
- The denial body does not contain the stored transport order id, the stored snapshot id, or the payload of the operation that belongs to the other грузовладелец.
- The service returns the stored result only when section 4 has authorized the caller, `shipper_company_id` is the same, and the stored request hash matches this request.

A hash mismatch takes the conflict path. It does not call the replay that returns the stored order. NT-D13. This rule does not add a migration. The unique key may stay `(tenant_id, actor_company_id, idempotency_key)` because the shipper comparison is mandatory before any replay.

Authorization stays in front of that lookup. A revoked grant, a missing create role, a missing grant, or a foreign tenant is denied at steps 1–4 of section 4. A stored grant that differs from the current grant, or a grant that is no longer acceptable, is denied at steps 6–7. Those denials do not return the stored order, the stored snapshot, or commercial fields. NT-D14 through NT-D18 and NT-D31 through NT-D36.

## 10. Audit

Write one allow-audit row in the same transaction as the order and the snapshot:

- actor user id, actor company id, shipper company id, buyer company id, tenant id
- permission code `CREATE_TRANSPORT_ORDER_ON_BEHALF` for the delegated path, and an explicit same-company action code for the non-delegated path
- `authority_grant_id` and version, null only on the same-company path
- idempotency key, transport order id, rate snapshot id, correlation id, `occurred_at`

A denial writes no order and no snapshot. Denial audit must not be implemented by inserting those rows.

`company.carrier_relationship_audit` stays the relationship lifecycle log. `transport.transport_order_create_idempotency` stays the replay key. Neither row satisfies this audit.

## 11. Negative tests

These tests are specified here and are not implemented in this change. Each denial asserts that the order table, the snapshot table, and the create-idempotency table gained no row.

| ID | Case | Result |
| --- | --- | --- |
| NT-D1 | Actor company and shipper company have no relationship and no grant | Deny |
| NT-D2 | An `ACTIVE` carrier relationship exists between the companies, and no `CREATE_TRANSPORT_ORDER_ON_BEHALF` grant exists | Deny |
| NT-D3 | The grant read for the requested authority tuple returns `REVOKED` or any status other than `ACTIVE` | Deny. This case does not use expiry. An expiry denial exists only as conditional NT-D33 if a controller later selects E1 |
| NT-D4 | Grant tenant, JWT tenant, or shipper company tenant differ | Deny. No cross-tenant body |
| NT-D5 | Actor role is `CARRIER_ADMIN`, `CARRIER_DISPATCHER`, or `SHIPPER_LOGIST`, or a create role that the grant does not list | Deny |
| NT-D6 | Client sends an actor company the user is not a member of, or a grant id that belongs to another actor company | Deny. The server uses the verified membership company |
| NT-D7 | Grant names shipper S. Body `shipper_company_id` is C | Deny |
| NT-D8 | `PLATFORM_ADMIN` membership is company A. Body shipper is B. No `subject_kind=PLATFORM_ADMIN` grant names this user and shipper B | Deny |
| NT-D9 | `AUTH_ENABLED` is false or unset. Client sends `X-Company-ID`, `X-Actor-Kind`, a shipper, and a grant id | Deny. No priced-create call with those headers as authority |
| NT-D10 | Award-scope has a valid internal token. Body actor company is a forwarder and body shipper is a different company, with no grant loaded by transport-order-service | Deny |
| NT-D11 | Award-scope has a valid internal token. Body `actor_company_id` equals body `shipper_company_id`, and both are an arbitrary company C in the tenant. The service's own read of the RFx event has an owner other than C, or the event does not resolve | Deny before the rate call and before insert. No order, snapshot, or idempotency row for C |
| NT-D12 | Direct `POST /v1/transport-orders` on transport-order-service carries client `X-User-ID`, `X-Company-ID`, and `X-Actor-Kind`, and the service has not confirmed that actor. Includes the case `AUTH_ENABLED=false` and the case with no gateway on the path | Deny before the order insert and before the `SNAPSHOT_V1` insert |
| NT-D13 | A stored create used tenant T, actor company A, idempotency key K, and shipper S. The next request uses T, A, and K with shipper C, and C differs from S. Section 4 has already authorized the caller | Conflict denial. No second order and no second snapshot. The response does not contain the order id, snapshot id, or payload stored for S |
| NT-D14 | A delegated create stored an order under an `ACTIVE` grant. The grant is then `REVOKED`. The same actor replays the same idempotency key | Deny at the grant check. No idempotency lookup result is returned. The response has no stored order id and no stored snapshot id |
| NT-D15 | The stored order belongs to actor company A. A second user of company A has no create role and replays the key | Deny at the role check. The response has no stored order id and no stored snapshot id |
| NT-D16 | The stored order was delegated. The replay caller has a create role and has no `CREATE_TRANSPORT_ORDER_ON_BEHALF` grant for that shipper | Deny at the grant check. The response has no stored order id and no stored snapshot id |
| NT-D17 | The stored order is in tenant A. The caller is verified in tenant B and presents tenant A's key or tenant A's body tenant | Deny at the tenant check. The response has no stored order id and no stored snapshot id |
| NT-D18 | Any denial in NT-D13 through NT-D17 | The response body omits the stored transport order id, the stored snapshot id, and the stored order payload |
| NT-D19 | Award-scope credential is the dedicated rfx-service credential. The RFx award-subject read times out or is unavailable | Deny. No order, snapshot, or idempotency row |
| NT-D20 | Delegated award-scope. The company-service grant read times out or is unavailable | Deny. No order, snapshot, or idempotency row, and no stored ids in the response |
| NT-D21 | Award-scope presents a valid token issued to shipment-service, billing-register-service, or a shared multi-service token | Unauthorized. No order, snapshot, or idempotency row |
| NT-D22 | The verified tenant and the tenant on the loaded award subject differ, or the body tenant differs from the verified tenant | Deny. No row |
| NT-D23 | Body shipper or body actor company is company C. The loaded award subject owner is a different company | Deny. No row |
| NT-D24 | Award-scope grant read returns `REVOKED` | Deny before idempotency lookup. No stored ids in the response |
| NT-D25 | Body names `rfx_lot_id`. The loaded subject says the lot does not exist on the event | Deny. No row |
| NT-D26 | The loaded event status is not `AWARDED` | Deny. No row |
| NT-D27 | Body `carrier_company_id` differs from the loaded `winning_carrier_company_id` | Deny. No row |
| NT-D28 | Contract-rate resolve would succeed, and the award-subject read was not completed | Deny. Resolve is not the award check |
| NT-D29 | The loaded award subject has `lot_scoped=true`. The body omits `rfx_lot_id` | Deny. The service does not choose a lot |
| NT-D30 | The inbound rfx-service award-caller credential is presented to `GET /internal/v1/rfx-events/{eventId}/award-conversion-subject` | The RFx read rejects it. transport-order-service denies. No order, snapshot, or idempotency row |
| NT-D31 | Replay. The membership read times out or is unavailable | Deny at step 3, before idempotency lookup. No stored order id, snapshot id, or commercial fields |
| NT-D32 | Delegated replay. The current grant read times out or is unavailable | Deny at step 4, before idempotency lookup. No stored order id, snapshot id, or commercial fields |
| NT-D33 | Conditional on lifecycle option E1 in section 13. A grant with `expires_at` in the past is replayed | Deny before the stored order is returned. No stored ids and no commercial fields. This test is not a current rule. Option E1 is not selected |
| NT-D34 | Replay. The grant read returns `REVOKED`, `pending`, `cancelled`, empty, or any status other than `ACTIVE` | Deny. No stored order id, snapshot id, or commercial fields |
| NT-D35 | The loaded award subject names more than one winning carrier, or a lot winner that differs from `winning_carrier_company_id` | Deny. One body carrier does not select a winner |
| NT-D36 | The create stored `authority_grant_id` G1 at version V1. Replay authorizes grant G2, or the same G1 at a different version | Deny at step 6. No transfer and no adoption of the new version. No stored order id, snapshot id, or commercial fields |
| NT-D37 | The authority tuple has two `ACTIVE` rows, or the read cannot prove that there is exactly one | Deny before idempotency lookup. No stored order id, snapshot id, or commercial fields |
| NT-D18a | Any denial in NT-D18 and NT-D29 through NT-D37 | The response omits the stored transport order id, the stored snapshot id, amount, currency, and carrier price |
| NT-2 | From PR #363. Member of company A sends shipper B and has no grant | Deny before the rate call |
| NT-7 | From PR #363. Auth disabled. Public create does not treat client actor headers as authority | Deny |

One allowed case is part of acceptance, not a negative test: member of forwarder F, role listed on an `ACTIVE` grant, tenant match, body shipper S equal to the grant shipper. Create succeeds. Snapshot buyer is S. Snapshot actor company is F. Snapshot grant id is that grant. The carrier relationship table is not consulted as the allow rule.

## 12. `AUTH_ENABLED=false`

`AUTH_ENABLED` defaults to false (`services/api-gateway/internal/config/config.go`). The transport-order guard then proxies immediately and records `pass_dev` (`guard.go`). It does not strip client `X-Company-ID` or `X-Actor-Kind` and it does not read membership.

Safe behavior at the gateway:

- Public priced create fails closed while auth is disabled (RC-1, NT-7, NT-D9).
- Delegated create is unavailable in that mode. There is no verified actor user, actor company, or role to match against a grant.
- Client headers, a body shipper, and a body grant id are not a substitute.
- Disabling gateway auth does not widen `POST /internal/v1/transport-orders/from-award-scope`. That route still requires the dedicated rfx-service award-caller credential and the award-scope chain in section 7.

The gateway flag is not the actor check. transport-order-service confirms the actor on its own boundary for `POST /v1/transport-orders`, before the rate call and before `CreatePricedOrder`.

The service confirms all of the following from a server-owned read, not from the arrival of headers:

- the actor user is active
- the actor company membership for that user is active
- the tenant of that membership is the verified tenant
- the membership role is allowed to create

Client `X-User-ID`, `X-Company-ID`, and `X-Actor-Kind` do not establish those facts. When the service cannot confirm them, or the membership read is unavailable, the call is denied. The denial happens before the order row and before the `SNAPSHOT_V1` row. No idempotency row is written (NT-D12).

This service check still runs when `AUTH_ENABLED` is false, when the gateway forwards the request, and when a client reaches transport-order-service directly. Award-scope uses the chain in section 7 on its own route. It does not skip that chain because the public actor check is a different boundary.

## 13. Schema and migration proposal

The current schema is not sufficient. A later implementation needs a migration. This PR does not add that migration.

Proposal only, for a later approved change:

1. A company-owned grant table. Suggested name `company.transport_order_authority_grants`. Columns: id, version, tenant id, subject kind (`COMPANY_MEMBER` or `PLATFORM_ADMIN`), actor company id, grantee user id, shipper company id, buyer company id, permission code constrained to `CREATE_TRANSPORT_ORDER_ON_BEHALF`, allowed role codes, status constrained to `ACTIVE` or `REVOKED`, revoked at, created by, created at. `expires_at` is not a column of this proposal.
2. Buyer company id on the grant equals shipper company id. Actor company id differs from shipper company id on a `COMPANY_MEMBER` delegation row.
3. Do not add the permission onto `company.carrier_relationships` and do not treat relationship status as this status.
4. Add nullable `actor_company_id`, `authority_grant_id`, and `authority_grant_version` to `transport.transport_order_rate_snapshots`. A check constraint requires the grant id when `actor_company_id` is present and differs from `buyer_company_id`.
5. Add the allow-audit table from section 10. Keep the snapshot immutability trigger covering the new columns.
6. Do not backfill old snapshots with invented grants. Historical rows keep a null grant id.
7. Grant revoke writes a new version with status `REVOKED`. It does not update the snapshot. `REVOKED` is terminal for that grant id. The same id does not return to `ACTIVE`.
8. Proposed partial unique index, not a migration in this PR: one `ACTIVE` row for `(tenant_id, actor_company_id, grantee_user_id, shipper_company_id, permission_code)`. A future implementation that sees more than one `ACTIVE` row for that tuple fails closed. This index is a schema proposal. It is not created here and it is not a live rule.

### Grant lifecycle

This model is a proposal. Main has no `CREATE_TRANSPORT_ORDER_ON_BEHALF` grant, no grant status, and no `expires_at`. Section 4 uses this model for every current-authority check and for the post-lookup check.

| Observed result | Meaning |
| --- | --- |
| `ACTIVE` | The only result that can pass step 4 or step 6. The read must be fresh |
| `REVOKED` | Denial. Terminal for that grant id. The snapshot keeps the old grant id. Replay does not return the stored order. A later permission is a new grant id |
| `pending`, `cancelled`, empty, or any other value | Not a stored status of this table. If a read returns one, the call fails closed. The service does not map it to `ACTIVE` |
| Read timeout, transport error, or unavailable | Fail closed. The service does not reuse a cached status |

Versioning: every change to a grant row increases `grant_version`. The snapshot stores the version that was `ACTIVE` at insert. Step 6 accepts the stored row only when the fresh read of that same grant id is `ACTIVE` and the live version equals the stored `authority_grant_version`. A new version of the same id, or a different grant id for the same authority tuple, does not inherit the old snapshot. Giving the permission again after `REVOKED` creates a new grant id. It does not reopen the revoked id.

A status change between step 4 and step 7 is decided by the step 7 re-read. When that re-read is not a matching `ACTIVE` version, the response is a denial and contains no stored order, snapshot, or commercial fields.

`POLICY_DECISION_REQUIRED` for expiry. This contract does not select an option.

| Option | Rule | Status |
| --- | --- | --- |
| E0 | No `expires_at`. A grant ends only by a new `REVOKED` version. Usable only while a fresh read says `ACTIVE` | Not selected. The column list omits `expires_at` so this document can be written without choosing. That omission is not an approval of E0 |
| E1 | Add `expires_at`. Usable only when status is `ACTIVE` and the check time is earlier than `expires_at`, both at step 4 and at step 7. If the clock crosses `expires_at` between those reads, deny | Not selected. NT-D33 applies only after a controller selects E1 |

`MIGRATION_REQUIRED=YES` as this proposal. `MIGRATIONS_CREATED=NO` in this PR.

PR #363 said the RC-1…RC-4 remediation should not be combined with a migration. That constraint still holds for a change that only rejects every shipper mismatch. The approved delegation policy cannot store the authority source without the columns above, so the future delegated implementation is a separate change that includes this migration after its own approval.

## Acceptance for the future RC-1…RC-4 fix

PR #363 remains unchanged. The future fix uses the acceptance below. `IMPLEMENTATION_AUTHORIZED` stays `NO` until a later stage.

| ID | Acceptance |
| --- | --- |
| RC-1 | Runtime-open. NT-7, NT-D9, and NT-D12. With `AUTH_ENABLED` unset or false, `POST /api/v1/transport-orders` is denied and writes no order, snapshot, or idempotency row. Client `X-Company-ID` and `X-Actor-Kind` are not forwarded into priced create as a verified actor. transport-order-service also denies `POST /v1/transport-orders` when it has not confirmed the actor, before the order insert and before the `SNAPSHOT_V1` insert, including a direct call that never passed the gateway |
| RC-2 | Runtime-open. Replaces the PR #363 wording that every `shipper_company_id` different from the actor company is rejected. NT-2 and NT-D1, NT-D2, NT-D3, NT-D5, NT-D6, NT-D7, and NT-D8 deny before the rate call and before insert. Steps 1–4 run before idempotency lookup: NT-D14, NT-D15, NT-D16, NT-D17, NT-D18, NT-D31, NT-D32, and NT-D34 deny without returning a stored order id, snapshot id, or commercial fields. Steps 6–7 deny NT-D36 when the current grant id differs from the stored grant id. NT-D13: the same tenant, actor company, and idempotency key with a different shipper returns a conflict denial and does not return the stored result for the other shipper. The allowed forwarder case in section 11 creates one order and one snapshot whose buyer is the grant shipper and whose grant id and grant version are the server row. Existing tenant denials still pass |
| RC-3 | Runtime-open. NT-6 stays: no dedicated rfx-service credential, no row. NT-D10: a dedicated credential plus a forwarder actor and a different shipper is denied when transport-order-service has not loaded a matching active grant. NT-D11 and NT-D23: body companies that are not the loaded event owner are denied. NT-D19 and NT-D20 fail closed when the RFx subject or the grant read is unavailable. NT-D21 rejects another service's token and a shared token. NT-D30 rejects the inbound award credential on the RFx read. NT-D22 denies a substituted tenant. NT-D24 denies a revoked grant before replay. NT-D25, NT-D26, NT-D27, NT-D29, and NT-D35 deny a missing lot, a lot-scoped award without a lot id, a non-`AWARDED` event, a carrier that did not win, and a split award. NT-D28 denies when resolve is the only successful check. `requireOwnerCompanyAccess` inside rfx-service does not replace that confirmation |
| RC-4 | Deferred. NT-5 from PR #363. Add the tenant-B internal snapshot read test. This contract does not add a company filter on `GET /internal/v1/transport-orders/{id}/rate-snapshot`. Company filtering of that read waits for a separate service-to-service decision |

Also required on that future change, as part of RC-2 and RC-3 rather than as new product scope: the audit row from section 10, the snapshot fields from section 9, and the migration proposal from section 13. VAT, rounding, billing math, and register payer selection stay out of that change.

## What this contract does not do

- It does not implement the grant, the checks, the audit, or the migration.
- It does not edit PR #341, #353, #356, or #363.
- It does not select a VAT basis. NET, GROSS, and VAT-not-applicable stay unselected.
- It does not approve a rounding rule.
- It does not introduce a payer identity. Buyer and payer stay different parties.
- It does not confirm billing readiness.
- It does not make legal EDO ready.
- RC-1, RC-2, and RC-3 stay runtime-open. RC-4 stays deferred.
- It does not treat `1234.56 RUB` as evidence of create authority.
- `J-FIN02E-R2-001`, `J-FIN02E-R2-002`, and `J-FIN02E-R2-003` stay closed. The R3 findings stay closed.
- Revision 0.2E-R3D is the input for Agent J's short delta review. Agent G does not record that review as complete.
- `expires_at` stays `POLICY_DECISION_REQUIRED`. Options E0 and E1 are not selected.
- This revision creates no grant rows and no migration. The partial unique index in section 13 is a schema proposal only.

`PAYMENT_EXECUTED=NO`

`PRODUCT_BEHAVIOR_CHANGED=NO`

`STAGING_CHANGED=NO`

`PRODUCTION_CHANGED=NO`

`MERGE_PERFORMED=NO`
