# FINANCE-TRANSPORT-ORDER-DELEGATED-AUTHORITY-0.2E

Design and acceptance contract for creating a transport order and `SNAPSHOT_V1` on behalf of a грузовладелец (shipper) by another company.

`POLICY=SERVER_VERIFIED_DELEGATION`

`POLICY_APPROVED=YES`

`IMPLEMENTATION_AUTHORIZED=NO`

`COMMERCIAL_PILOT_READY=NO`

`FINAL_VERDICT=READY_FOR_SECURITY_R2`

Base `3f306fc407aeda4bdc652d825954292f4a54fad6`. Related discovery is PR #363 (`docs/engineering/FINANCE_SNAPSHOT_AUTHORITY_0_2D.md`). Revision 0.2E-R1 closes the security-review findings on award-scope binding, the transport-order-service actor boundary, and idempotency across shippers. Security review R2 is the next step. This revision does not record R2 as complete.

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

It is a row the server writes and the server reads back. The client does not send the permission code, the role, or the grant status. A client-supplied grant id is a locator only. The server loads the row and rejects the call when the loaded row does not match the verified actor and the body shipper.

The permission is absent from the current schema. No role constant and no snapshot column carries it.

## 4. Checks required before rate resolution and insert

Evaluate every item below inside the trusted server path, before the contract-rate call and before `CreatePricedOrder`. A failure writes no order, no snapshot, and no idempotency row.

| Check | Rule |
| --- | --- |
| Tenant | JWT tenant, grant tenant, actor company tenant, and shipper company tenant are the same value. Body `tenant_id` that differs is forbidden, as it is today |
| Actor user | Active user. Membership in the actor company is active |
| Actor company | The company transport-order-service has confirmed for this user by its own actor check (section 12). A gateway header, including a header forwarded while `AUTH_ENABLED` is false, is not that confirmation |
| Shipper company | Body `shipper_company_id` equals the shipper named on the grant. The company exists in the same tenant |
| Role | The actor's membership role is one of the create roles (`PROCUREMENT_MANAGER`, `SHIPPER_ADMIN`, `FORWARDER_MANAGER`) and is included in the grant's allowed roles. `CARRIER_ADMIN`, `CARRIER_DISPATCHER`, and `SHIPPER_LOGIST` do not receive this permission from their current read or relationship roles |
| Grant activity | Status is `ACTIVE`. `REVOKED`, expired, cancelled, and pending are denials. Re-read status at create time. A status cached at login is not enough |

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
| Trusted internal caller | The internal credential authenticates the configured award caller. A missing or wrong credential stays unauthorized and writes nothing (NT-6). That credential is only this link |
| RFx event | The service loads the event by id in the verified tenant through a server-owned read. The body is not that read. An unknown event is a denial |
| Event owner company | The owner is the owner field on the loaded event. The service does not take the owner from body `actor_company_id` or body `shipper_company_id` |
| Actor company | On the same-company path the body actor company equals the loaded event owner. On the delegated path it equals the actor company on an `ACTIVE` `CREATE_TRANSPORT_ORDER_ON_BEHALF` grant that this service loaded |
| Shipper company | The body shipper equals the loaded event owner. On the delegated path that same company is the grant shipper |
| Allowed creation | Every link above agrees. Only then the service may create one order and one `SNAPSHOT_V1` |

A valid service token together with body `actor_company_id` equal to body `shipper_company_id` is not this chain. Those two facts do not identify the event owner and do not authorize an arbitrary company in the tenant (NT-D11).

rfx-service `requireOwnerCompanyAccess` remains the caller's own check. It does not replace the load inside transport-order-service. A grant whose shipper differs from the loaded event owner does not authorize award conversion. The service loads any `CREATE_TRANSPORT_ORDER_ON_BEHALF` grant itself. A statement from rfx-service that a delegation exists is not that load.

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
- The service returns the stored result only when `shipper_company_id` is the same and the stored request hash matches this request.

A hash mismatch takes the conflict path. It does not call the replay that returns the stored order. NT-D13. This rule does not add a migration. The unique key may stay `(tenant_id, actor_company_id, idempotency_key)` because the shipper comparison is mandatory before any replay.

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
| NT-D3 | A grant exists and its status is `REVOKED`, expired, cancelled, or pending | Deny |
| NT-D4 | Grant tenant, JWT tenant, or shipper company tenant differ | Deny. No cross-tenant body |
| NT-D5 | Actor role is `CARRIER_ADMIN`, `CARRIER_DISPATCHER`, or `SHIPPER_LOGIST`, or a create role that the grant does not list | Deny |
| NT-D6 | Client sends an actor company the user is not a member of, or a grant id that belongs to another actor company | Deny. The server uses the verified membership company |
| NT-D7 | Grant names shipper S. Body `shipper_company_id` is C | Deny |
| NT-D8 | `PLATFORM_ADMIN` membership is company A. Body shipper is B. No `subject_kind=PLATFORM_ADMIN` grant names this user and shipper B | Deny |
| NT-D9 | `AUTH_ENABLED` is false or unset. Client sends `X-Company-ID`, `X-Actor-Kind`, a shipper, and a grant id | Deny. No priced-create call with those headers as authority |
| NT-D10 | Award-scope has a valid internal token. Body actor company is a forwarder and body shipper is a different company, with no grant loaded by transport-order-service | Deny |
| NT-D11 | Award-scope has a valid internal token. Body `actor_company_id` equals body `shipper_company_id`, and both are an arbitrary company C in the tenant. The service's own read of the RFx event has an owner other than C, or the event does not resolve | Deny before the rate call and before insert. No order, snapshot, or idempotency row for C |
| NT-D12 | Direct `POST /v1/transport-orders` on transport-order-service carries client `X-User-ID`, `X-Company-ID`, and `X-Actor-Kind`, and the service has not confirmed that actor. Includes the case `AUTH_ENABLED=false` and the case with no gateway on the path | Deny before the order insert and before the `SNAPSHOT_V1` insert |
| NT-D13 | A stored create used tenant T, actor company A, idempotency key K, and shipper S. The next request uses T, A, and K with shipper C, and C differs from S | Conflict denial. No second order and no second snapshot. The response does not contain the order id, snapshot id, or payload stored for S |
| NT-2 | From PR #363. Member of company A sends shipper B and has no grant | Deny before the rate call |
| NT-7 | From PR #363. Auth disabled. Public create does not treat client actor headers as authority | Deny |

One allowed case is part of acceptance, not a negative test: member of forwarder F, role listed on an `ACTIVE` grant, tenant match, body shipper S equal to the grant shipper. Create succeeds. Snapshot buyer is S. Snapshot actor company is F. Snapshot grant id is that grant. The carrier relationship table is not consulted as the allow rule.

## 12. `AUTH_ENABLED=false`

`AUTH_ENABLED` defaults to false (`services/api-gateway/internal/config/config.go`). The transport-order guard then proxies immediately and records `pass_dev` (`guard.go`). It does not strip client `X-Company-ID` or `X-Actor-Kind` and it does not read membership.

Safe behavior at the gateway:

- Public priced create fails closed while auth is disabled (RC-1, NT-7, NT-D9).
- Delegated create is unavailable in that mode. There is no verified actor user, actor company, or role to match against a grant.
- Client headers, a body shipper, and a body grant id are not a substitute.
- Disabling gateway auth does not widen `POST /internal/v1/transport-orders/from-award-scope`. That route still requires the internal token and the award-scope chain in section 7.

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

1. A company-owned grant table. Suggested name `company.transport_order_authority_grants`. Columns: id, version, tenant id, subject kind (`COMPANY_MEMBER` or `PLATFORM_ADMIN`), actor company id, grantee user id, shipper company id, buyer company id, permission code constrained to `CREATE_TRANSPORT_ORDER_ON_BEHALF`, allowed role codes, status constrained to `ACTIVE` or `REVOKED`, revoked at, created by, created at.
2. Buyer company id on the grant equals shipper company id. Actor company id differs from shipper company id on a `COMPANY_MEMBER` delegation row.
3. Do not add the permission onto `company.carrier_relationships` and do not treat relationship status as this status.
4. Add nullable `actor_company_id`, `authority_grant_id`, and `authority_grant_version` to `transport.transport_order_rate_snapshots`. A check constraint requires the grant id when `actor_company_id` is present and differs from `buyer_company_id`.
5. Add the allow-audit table from section 10. Keep the snapshot immutability trigger covering the new columns.
6. Do not backfill old snapshots with invented grants. Historical rows keep a null grant id.
7. Grant revoke is a new version or a status change on the grant. It does not update the snapshot.

`MIGRATION_REQUIRED=YES` as this proposal. `MIGRATIONS_CREATED=NO` in this PR.

PR #363 said the RC-1…RC-4 remediation should not be combined with a migration. That constraint still holds for a change that only rejects every shipper mismatch. The approved delegation policy cannot store the authority source without the columns above, so the future delegated implementation is a separate change that includes this migration after its own approval.

## Acceptance for the future RC-1…RC-4 fix

PR #363 remains unchanged. The future fix uses the acceptance below. `IMPLEMENTATION_AUTHORIZED` stays `NO` until a later stage.

| ID | Acceptance |
| --- | --- |
| RC-1 | NT-7, NT-D9, and NT-D12. With `AUTH_ENABLED` unset or false, `POST /api/v1/transport-orders` is denied and writes no order, snapshot, or idempotency row. Client `X-Company-ID` and `X-Actor-Kind` are not forwarded into priced create as a verified actor. transport-order-service also denies `POST /v1/transport-orders` when it has not confirmed the actor, before the order insert and before the `SNAPSHOT_V1` insert, including a direct call that never passed the gateway |
| RC-2 | Replaces the PR #363 wording that every `shipper_company_id` different from the actor company is rejected. NT-2 and NT-D1, NT-D2, NT-D3, NT-D5, NT-D6, NT-D7, and NT-D8 deny before the rate call and before insert. NT-D13: the same tenant, actor company, and idempotency key with a different shipper returns a conflict denial, creates no second order or snapshot, and does not return the stored result for the other shipper. The allowed forwarder case in section 11 creates one order and one snapshot whose buyer is the grant shipper and whose grant id is the server row. Existing tenant denials still pass |
| RC-3 | NT-6 stays: no internal token, no row. NT-D10: a valid token plus a forwarder actor and a different shipper is denied when transport-order-service has not loaded a matching active grant. NT-D11: a valid token plus body actor company equal to body shipper company is denied when that company is not the owner of the event the service loaded. Award conversion is allowed only after transport-order-service has confirmed the trusted caller, the loaded event, the loaded event owner, the actor company, the shipper company, and the allowed-creation rule in section 7. `requireOwnerCompanyAccess` inside rfx-service does not replace that confirmation |
| RC-4 | NT-5 from PR #363. Add the tenant-B internal snapshot read test. This contract does not add a company filter on `GET /internal/v1/transport-orders/{id}/rate-snapshot`. Company filtering of that read waits for a separate service-to-service decision |

Also required on that future change, as part of RC-2 and RC-3 rather than as new product scope: the audit row from section 10, the snapshot fields from section 9, and the migration proposal from section 13. VAT, rounding, billing math, and register payer selection stay out of that change.

## What this contract does not do

- It does not implement the grant, the checks, the audit, or the migration.
- It does not edit PR #341, #353, #356, or #363.
- It does not select a VAT policy and does not declare commercial finance ready.
- It does not treat `1234.56 RUB` as evidence of create authority.
- Revision 0.2E-R1 is the input for security review R2. Agent G does not record R2 as complete.

`PAYMENT_EXECUTED=NO`

`PRODUCT_BEHAVIOR_CHANGED=NO`

`STAGING_CHANGED=NO`

`PRODUCTION_CHANGED=NO`

`MERGE_PERFORMED=NO`
