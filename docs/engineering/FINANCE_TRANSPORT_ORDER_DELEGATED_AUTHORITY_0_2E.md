# FINANCE-TRANSPORT-ORDER-DELEGATED-AUTHORITY-0.2E

Design and acceptance contract for creating a transport order and `SNAPSHOT_V1` on behalf of a грузовладелец (shipper) by another company.

`POLICY=SERVER_VERIFIED_DELEGATION`

`POLICY_APPROVED=YES`

`IMPLEMENTATION_AUTHORIZED=NO`

`COMMERCIAL_PILOT_READY=NO`

`FINAL_VERDICT=CONTRACT_READY_FOR_REVIEW`

Base `3f306fc407aeda4bdc652d825954292f4a54fad6`. Related discovery is PR #363 (`docs/engineering/FINANCE_SNAPSHOT_AUTHORITY_0_2D.md`). This document does not change transport-order-service, billing, VAT, rounding, PostgreSQL migrations, or PRs #341, #353, #356, and #363.

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
| Actor company | The company the gateway verified for this user. Client `X-Company-ID` is not authority until that membership match |
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

Required award-scope behavior:

- Keep the internal token. A missing or wrong token stays unauthorized and writes nothing (NT-6).
- Body `actor_company_id` and body `shipper_company_id` are not authority.
- Same-company award conversion is accepted only when the trusted caller proved membership of the event owner and the body actor company, body shipper, and event owner are that same company.
- A body that sets the actor company to a forwarder and the shipper to a different company is denied unless transport-order-service itself loads an `ACTIVE` `CREATE_TRANSPORT_ORDER_ON_BEHALF` grant for that tuple before the rate call.
- rfx-service must not assert a delegation it did not have the server re-check. The trusted-caller contract covers the same-company owner proof only.

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
| NT-2 | From PR #363. Member of company A sends shipper B and has no grant | Deny before the rate call |
| NT-7 | From PR #363. Auth disabled. Public create does not treat client actor headers as authority | Deny |

One allowed case is part of acceptance, not a negative test: member of forwarder F, role listed on an `ACTIVE` grant, tenant match, body shipper S equal to the grant shipper. Create succeeds. Snapshot buyer is S. Snapshot actor company is F. Snapshot grant id is that grant. The carrier relationship table is not consulted as the allow rule.

## 12. `AUTH_ENABLED=false`

`AUTH_ENABLED` defaults to false (`services/api-gateway/internal/config/config.go`). The transport-order guard then proxies immediately and records `pass_dev` (`guard.go`). It does not strip client `X-Company-ID` or `X-Actor-Kind` and it does not read membership.

Safe behavior:

- Public priced create fails closed while auth is disabled (RC-1, NT-7, NT-D9).
- Delegated create is unavailable in that mode. There is no verified actor user, actor company, or role to match against a grant.
- Client headers, a body shipper, and a body grant id are not a substitute.
- Disabling gateway auth does not widen `POST /internal/v1/transport-orders/from-award-scope`. That route still requires the internal token and the award-scope rules in section 7.

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
| RC-1 | NT-7 and NT-D9. With `AUTH_ENABLED` unset or false, `POST /api/v1/transport-orders` is denied and writes no order, snapshot, or idempotency row. Client `X-Company-ID` and `X-Actor-Kind` are not forwarded into priced create as a verified actor |
| RC-2 | Replaces the PR #363 wording that every `shipper_company_id` different from the actor company is rejected. NT-2 and NT-D1, NT-D2, NT-D3, NT-D5, NT-D6, NT-D7, and NT-D8 deny before the rate call and before insert. The allowed forwarder case in section 11 creates one order and one snapshot whose buyer is the grant shipper and whose grant id is the server row. Existing tenant denials still pass |
| RC-3 | NT-6 stays: no internal token, no row. NT-D10: a valid token plus a body actor/shipper pair is denied when transport-order-service has not loaded a matching active grant and the pair is not the same-company owner already verified by the trusted rfx caller. Award conversion that sends actor company = shipper = event owner after `requireOwnerCompanyAccess` remains the same-company path |
| RC-4 | NT-5 from PR #363. Add the tenant-B internal snapshot read test. This contract does not add a company filter on `GET /internal/v1/transport-orders/{id}/rate-snapshot`. Company filtering of that read waits for a separate service-to-service decision |

Also required on that future change, as part of RC-2 and RC-3 rather than as new product scope: the audit row from section 10, the snapshot fields from section 9, and the migration proposal from section 13. VAT, rounding, billing math, and register payer selection stay out of that change.

## What this contract does not do

- It does not implement the grant, the checks, the audit, or the migration.
- It does not edit PR #341, #353, #356, or #363.
- It does not select a VAT policy and does not declare commercial finance ready.
- It does not treat `1234.56 RUB` as evidence of create authority.
- It is ready for an independent security review. This document is the review input. Agent G does not record that review as complete.

`PAYMENT_EXECUTED=NO`

`PRODUCT_BEHAVIOR_CHANGED=NO`

`STAGING_CHANGED=NO`

`PRODUCTION_CHANGED=NO`

`MERGE_PERFORMED=NO`
