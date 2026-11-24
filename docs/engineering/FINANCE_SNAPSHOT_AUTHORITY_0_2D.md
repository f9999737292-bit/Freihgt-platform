# FINANCE-SNAPSHOT-AUTHORITY-0.2D

Authority discovery for `POST /v1/transport-orders` and `SNAPSHOT_V1`. The `1234.56 RUB` proof inserted a snapshot row and did not exercise this path.

`COMMERCIAL_PILOT_READY=NO`

`FINAL_VERDICT=AUTHORITY_ASSESSED/REMEDIATION_REQUIRED/BLOCKED`

Base `3f306fc407aeda4bdc652d825954292f4a54fad6`. This document does not change billing, tariffs, VAT, rounding, API, or migrations, and it does not edit PR #341, #353, or #356.

## Path

Public create:

1. `POST /api/v1/transport-orders` in `services/api-gateway/internal/http/router.go` uses `transportorderrbac.Guard` with `PolicyCreate`, then the proxy.
2. The proxy rewrites the path to `POST /v1/transport-orders` on transport-order-service.
3. `PricedTransportOrderHandler.CreatePricedTransportOrder` reads the verified tenant and the actor headers.
4. `PricedTransportOrderService.CreatePricedTransportOrder` resolves a rate and calls `PricedOrderRepository.CreatePricedOrder`.
5. The repository inserts `transport.transport_orders` with `pricing_model_version=SNAPSHOT_V1`, then `transport.transport_order_rate_snapshots`, in one transaction.

Internal award conversion is a different route: `POST /internal/v1/transport-orders/from-award-scope`, behind the internal service token. It forces actor kind `BUYER` and then uses the same priced-create service.

## Identity

| Fact | Source |
| --- | --- |
| Tenant and user on the public route | JWT `tenant_id` and subject, written to `X-Tenant-ID` and `X-User-ID` when gateway auth is enabled |
| Requested company | Client `X-Company-ID`, saved before the auth middleware deletes it |
| Actor kind | Derived by the guard from the matched membership. Client `X-Actor-Kind` is deleted and replaced |
| Service tenant | `resolveVerifiedTenant` reads `X-Tenant-ID` only. A different body `tenant_id` is forbidden |
| Service actor | `X-User-ID`, `X-Company-ID`, `X-Actor-Kind` as received |

`AUTH_ENABLED` defaults to false in `services/api-gateway/internal/config/config.go`. When auth is disabled, the guard calls the proxy immediately and records `pass_dev`. It does not strip client actor headers and does not query membership.

## Membership and company

When auth is enabled, `PolicyCreate` does all of the following before the proxy:

- requires a verified user and tenant
- rejects a query or body tenant that differs from the JWT tenant
- requires `X-Company-ID` and a membership of that user in that company
- derives actor kind from the membership role and company type
- allows create for a buyer membership with `PROCUREMENT_MANAGER`, `SHIPPER_ADMIN`, `FORWARDER_MANAGER`, or `PLATFORM_ADMIN`
- a carrier kind is denied unless the caller is a platform admin who also has that company membership
- rewrites `X-Company-ID` and `X-Actor-Kind` to the matched membership

The service and repository do not query membership again.

Company checks that do happen in the service:

- shipper and consignee must exist in the same tenant
- locations and cargo must exist in the same tenant
- idempotency is unique per tenant, actor company, and key

Company check that does not happen on create:

- `shipper_company_id` in the body is stored as the order shipper and as snapshot `buyer_company_id`
- the actor company is stored only on the idempotency row
- nothing requires those two companies to be the same

A member of company A in the tenant can therefore submit company B as `shipper_company_id`, provided company B exists in that tenant. The snapshot buyer becomes company B. Later mutation allows only the shipper buyer, so company A may be unable to mutate the order it created.

Read of an order uses `CanReadTransportOrder`: shipper, consignee, or the selected carrier. A different company in the same tenant receives not found. That check is on get, list, and mutate. It is not on priced create.

## Who can create and read a snapshot

| Action | Who, when gateway auth is enabled | Limit |
| --- | --- | --- |
| Create through the public route | Buyer member with a create role, for the company named in `X-Company-ID` | Body shipper may be a different company in the tenant |
| Create through award scope | Caller that presents the internal service token. Body supplies `actor_company_id`. Kind is forced to `BUYER` | No membership lookup in transport-order-service |
| Read the order | Shipper, consignee, selected carrier, or platform admin, inside the tenant | Gateway read policy also requires a read role |
| Read the snapshot | `GET /internal/v1/transport-orders/{id}/rate-snapshot` with the internal token and `X-Tenant-ID` | Tenant filter only. No company filter |

The snapshot table rejects update and delete. There is no public snapshot read route.

## Settlement link

`SNAPSHOT_V1` settlement loads `transport.transport_order_rate_snapshots` by `transport_order_id` and `tenant_id`. It copies `total_amount`, currency, pricing source, buyer, and carrier onto the settlement and stores `rate_snapshot_id`. The shipment carrier must match the snapshot carrier. The settlement path does not repeat the gateway membership check and does not compare the original actor company with the snapshot buyer.

The `1234.56` run used an inserted snapshot. It proves amount identity for that row. It does not prove who was allowed to create the snapshot.

## Proven and assumed

Proven by code and existing tests:

- Gateway tests `TestTransportOrderCreateMissingJWTDenied`, `TestTransportOrderCreateMissingCompanyDenied`, `TestTransportOrderCompanyIsolationDenied`, `TestTransportOrderHeaderSpoofStrippedAndVerified`, `TestTransportOrderActorSpoofCannotElevate`, `TestTransportOrderTenantIsolationDenied`, `TestTransportOrderCreateBodyTenantMismatchDenied`.
- Service tests `TestPublicCreateMismatchedBodyTenantDenied`, `TestPublicCreateMissingVerifiedTenantFailClosed`, `TestAwardScopeCreateStillUsesVerifiedHeaderTenant`.
- Domain tests: a company that is not shipper, consignee, or carrier cannot read; only the shipper buyer can mutate.
- Internal snapshot tests: missing or wrong service token returns 401; missing tenant returns 401; an unpriced order returns 409.

Assumed, not proven by the money-identity run or by an end-to-end create:

- Production sets `AUTH_ENABLED=true`. The code default is false, and that default skips membership.
- Transport-order-service is unreachable except through the gateway. The service trusts actor headers.
- A foreign company cannot create a snapshot. The gateway denies a company outside the caller's membership. The service still accepts another shipper inside the tenant.
- Internal snapshot read is safe for every company because the caller holds the service token. The handler does not check company.

## Negative tests still required

| ID | Expected result |
| --- | --- |
| NT-1 | Auth enabled. User is a member of company A only. `X-Company-ID` is company B. Response is forbidden. No order and no snapshot row. |
| NT-2 | Auth enabled. Member of company A sends `shipper_company_id` of company B in the same tenant. Response is forbidden. No snapshot row. This fails on current code. |
| NT-3 | Body or query tenant differs from the JWT tenant. Forbidden. No row. Already covered at the gateway for the tenant value. |
| NT-4 | Carrier membership calls create. Forbidden. No row. Covered as `TestTransportOrderMutateCarrierDenied` for mutate and by the create policy for a carrier kind. |
| NT-5 | Tenant B calls internal snapshot read for tenant A's order. Not found. No cross-tenant body. This test is not in `rate_snapshot_internal_handler_test.go`. |
| NT-6 | Award-scope request without the internal token is unauthorized and writes nothing. |
| NT-7 | `AUTH_ENABLED` unset. Public create does not accept client-supplied actor headers as authority. Current default does not meet this. |

## Remediation contract

Do this in a later change. Do not combine it with VAT, rounding, billing math, or a migration.

| ID | Change | Acceptance |
| --- | --- | --- |
| RC-1 | Public create fails closed unless gateway auth is enabled. A disabled-auth process must not forward client `X-Company-ID` or `X-Actor-Kind` into priced create | NT-7 |
| RC-2 | Priced create rejects `shipper_company_id` that differs from the actor company before the rate call and before the insert | NT-2, and the existing tenant denials still pass |
| RC-3 | Award-scope keeps the internal token requirement. `actor_company_id` from the body is either bound to a trusted caller contract or rejected | NT-6 plus one test that a body company is not accepted as membership |
| RC-4 | Add the tenant-B snapshot read test. Leave company filtering of the internal read unchanged until a separate service-to-service decision | NT-5 |

`PAYMENT_EXECUTED=NO`

`MIGRATIONS_CREATED=NO`

`PRODUCT_BEHAVIOR_CHANGED=NO`

`STAGING_CHANGED=NO`

`PRODUCTION_CHANGED=NO`
