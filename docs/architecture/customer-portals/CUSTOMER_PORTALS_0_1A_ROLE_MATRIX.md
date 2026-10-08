# Customer Portals 0.1A — role matrix

Evidence is from `origin/main` `67e72d37cdf8f658de04415b0ac2fc94e0dd408d`. Roles below are codes that exist in migrations or gateway policy. Planning-only names in older docs are called out when they are not seeded.

## Canonical identity flow

```text
LOGIN_ENTRYPOINT_FOUND=YES
LOGIN_PUBLIC_ROUTE=POST /api/v1/auth/login
JWT_USAGE_FOUND=YES
SESSION_STORAGE_PATTERN=NONE_IN_CUSTOMER_PORTALS
WEB_ADMIN_SESSION_PATTERN=localStorage freight_admin_session {token,user}; localStorage freight_admin_tenant_id
TENANT_SERVER_DERIVED=YES
CLIENT_TENANT_OVERRIDE_ALLOWED=NO
```

Gateway auth (`services/api-gateway/internal/http/middleware/auth.go`) strips client `X-Tenant-ID`, `X-User-ID`, and `X-User-Email`, then writes them from the Bearer JWT (`tenant_id` claim and subject). A spoofed tenant header does not survive the gateway. `POST /api/v1/auth/login` and `POST /api/v1/users` are public. Everything else under the customer API requires a Bearer token when auth is enabled.

`X-Company-ID` is not stripped by that middleware. It is kept as `RequestedCompanyID`. Company-context enforcement, where wired, deletes the client company headers and replaces them only after membership is checked (`services/api-gateway/internal/companycontext/actor.go`). Shipment list and shipment detail do not use that enforcer.

Customer portals today have no login page, no session store, and no tenant store.

Driver identity is a separate role and a separate app. `DRIVER` is seeded in `infrastructure/migrations/000009_seed_roles.up.sql`. Operational driver calls are `/api/v1/driver/me/**`, consumed by `apps/driver-mobile`. They are not carrier-portal APIs.

## Seeded role codes

From `infrastructure/migrations/000009_seed_roles.up.sql` and `000013_seed_forwarder_manager_role.up.sql`:

| Code | Scope | Customer portal relevance |
| --- | --- | --- |
| `SHIPPER_ADMIN` | TENANT | Shipper company administrator. Buyer-manage for RFx. |
| `SHIPPER_LOGIST` | TENANT | Creates transport orders in the role description. Gateway create for transport orders and RFx manage exclude this code. |
| `CARRIER_ADMIN` | TENANT | Carrier administrator. Fleet create, billing mutate, carrier RFx. |
| `CARRIER_DISPATCHER` | TENANT | Bids, vehicles, drivers. Fleet assign. Carrier RFx. No fleet create. |
| `DRIVER` | TENANT | Driver mobile only. Not a customer-portal role. |
| `CONSIGNEE_OPERATOR` | TENANT | Seeded. Described as receiving goods and confirming delivery. Almost no gateway policies include it. |
| `PROCUREMENT_MANAGER` | TENANT | Buyer. Not a customer-portal role. |
| `FORWARDER_MANAGER` | TENANT | Buyer-like. Not a customer-portal role. |
| `FINANCE_MANAGER` | TENANT | Finance. Not a customer-portal role. |
| `PLATFORM_ADMIN` | GLOBAL | Platform operator. Not a customer-portal role. |
| `GOV_INSPECTOR` | GLOBAL | Not a customer-portal role. |

Referenced in gateway or web-admin code and **not** inserted by any migration in this tree:

| Code | Where referenced | Seeded |
| --- | --- | --- |
| `CONSIGNEE_VIEWER` | `services/api-gateway/internal/shipmentevents/rbac.go`, `apps/web-admin/composables/usePermissions.ts` | NO |
| `CARRIER_ACCOUNTANT` | billing, payment, rates, freight-cost, and analytics RBAC | NO |

`apps/web-admin/composables/usePermissions.ts` maps identity codes to product roles `shipper`, `carrier`, and `consignee`. That map is admin-shell navigation. It is not mounted in `apps/web-shipper`, `apps/web-carrier`, or `apps/web-consignee`.

## Portal access today

| PORTAL | ROLE | AUTH_SOURCE | TENANT_SOURCE | CURRENT_ACCESS | GAPS |
| --- | --- | --- | --- | --- | --- |
| Shipper | `SHIPPER_ADMIN` | None in `apps/web-shipper`. Gateway JWT if a client calls the API directly. | JWT `tenant_id` after login. | Portal UI is a welcome page. API rights exist for transport orders, RFx manage, shipment create-from-order/bid, shipment cancel, billing read, payment read, control tower, company membership admin. | No portal session. Shipment list/detail is tenant-wide, not company-scoped. |
| Shipper | `SHIPPER_LOGIST` | Same. | Same. | API read of transport orders and RFx. Cannot buyer-manage RFx or accept bids. Cannot create transport orders at the gateway. Can create shipments from an order or bid and cancel shipments. | Same portal gap. Role is narrower than the admin code. Do not collapse the two codes in a later portal. |
| Carrier | `CARRIER_ADMIN` | None in `apps/web-carrier`. | JWT `tenant_id`. | Portal UI is a welcome page. API rights exist for carrier RFx, bids, own award, fleet create, execution read/start, settlements, billing, payments, network next-load search. | No portal session. Shipment reads are not carrier-company scoped. |
| Carrier | `CARRIER_DISPATCHER` | Same. | Same. | Carrier RFx respond/read, fleet view and assign, execution read/start, shipment accept and status update. No fleet create. Control tower access includes this code and excludes `CARRIER_ADMIN`. | Same. `CARRIER_ACCOUNTANT` is not a seeded alternative for settlement UI. |
| Carrier | `DRIVER` | `apps/driver-mobile`, not web-carrier. | JWT `tenant_id` on `/api/v1/driver/me/**`. | Driver tasks, stops, POD, exceptions. | Must not be rehosted in the carrier portal. |
| Consignee | `CONSIGNEE_OPERATOR` | None in `apps/web-consignee`. | JWT `tenant_id` if a user exists. | Seeded role only. Shipment event read policy includes the code. Company actor derivation does not recognize a consignee company type. Control tower, transport orders, RFx, billing, payments, and analytics exclude it. | No consignee-scoped shipment query. `consignee_company_id` is a shipment column, not an authorization principal. |
| Consignee | `CONSIGNEE_VIEWER` | Not seeded. | Not established. | Name exists in event RBAC and web-admin nav mapping only. | Do not treat this code as a live role until identity seeds it. |

`companycontext.DeriveActorKind` returns a buyer actor for shipper, procurement, and forwarder role codes, and a carrier actor for carrier role codes. A consignee role does not match either arm. Company types `SHIPPER`, `FORWARDER`, and `LSP` become buyers. `CARRIER` becomes a carrier. There is no consignee actor kind.

## Security invariants for later portal work

- The portal must not treat a client-supplied `X-Tenant-ID` as the tenant. The gateway already replaces that header from the JWT.
- The portal must not invent `user_id`, company id, or role codes in the client.
- `X-Company-ID` may be sent only as a membership selection that the gateway revalidates. Shipment and tracking routes do not do that revalidation today.
- Cross-tenant reads fail closed at the gateway header rewrite and at shipment `GetByIDAndTenant`. They do not fail closed across companies inside one tenant for shipment list and detail.
- Driver Bearer tokens and `/api/v1/driver/me/**` stay outside customer portal clients.
