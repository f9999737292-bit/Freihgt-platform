# Customer Portals 0.1A — API map

Public routes are the ones registered in `services/api-gateway/internal/http/router.go`. The catch-all `r.Handle("/api/*", proxy)` forwards unmatched `/api` paths. A path listed here is a customer candidate only when it is an explicit gateway route with a role policy.

Internal service URLs, tracking-service tokens, and `/v1/driver/me/**` upstream paths are not customer API recommendations.

Classification:

| Class | Meaning |
| --- | --- |
| AVAILABLE | Explicit public route, JWT required, role policy matches the customer role, tenant comes from the JWT |
| PARTIAL | Public route exists, but role, company scope, or behavior does not match the capability |
| NOT_EXPOSED | Behavior exists on an internal or driver route and has no customer gateway route |
| NOT_IMPLEMENTED | No route and no customer-facing behavior found |
| DEFERRED | Operator or driver surface. Out of the customer portal until a later owner exposes a read model |

`AUTH_REQUIRED=YES` means the gateway auth middleware requires `Authorization: Bearer` when auth is enabled. Login itself is public.

## Shipper

Roles in the role column are seeded codes that the cited policy actually allows. `SHIPPER_LOGIST` is omitted when the policy omits it.

| Capability | Class | SOURCE_SERVICE | PUBLIC_ROUTE | AUTH_REQUIRED | ROLE_ALLOWED | TENANT_SCOPE | CURRENT_READINESS | BLOCKER |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Transport orders create | AVAILABLE | transport-order-service | `POST /api/v1/transport-orders` | YES | `SHIPPER_ADMIN` plus buyer actor | JWT tenant | Gateway policy present. No shipper UI. | Portal shell only. `SHIPPER_LOGIST` is not in `createRoles`. |
| Transport orders list and read | AVAILABLE | transport-order-service | `GET /api/v1/transport-orders`, `GET /api/v1/transport-orders/{id}` | YES | `SHIPPER_ADMIN`, `SHIPPER_LOGIST` as buyer | JWT tenant, company context on the guard | Same | No shipper UI |
| Shipment creation | AVAILABLE | shipment-service | `POST /api/v1/shipments/from-transport-order`, `POST /api/v1/shipments/from-bid` | YES | `SHIPPER_ADMIN`, `SHIPPER_LOGIST` | JWT tenant | Guarded create only. No open `POST /api/v1/shipments` policy in the explicit routes. | No shipper UI |
| Shipment list | PARTIAL | shipment-service | `GET /api/v1/shipments` via proxy | YES | No shipment role policy on this route | JWT tenant only. Optional `shipper_company_id` query is a filter, not an authorization predicate | A tenant user who can call the route can list the tenant | CP-API-001 |
| Shipment details | PARTIAL | shipment-service | `GET /api/v1/shipments/{id}` via proxy | YES | No shipment role policy | `GetByIDAndTenant` | Cross-tenant id fails. Cross-company id inside the tenant is not rejected here | CP-API-001 |
| Shipment status read | PARTIAL | shipment-service | Detail payload `status` | YES | Same as detail | Same as detail | Status field exists on the shipment resource | CP-API-001 |
| Shipment cancel | AVAILABLE | shipment-service | `POST /api/v1/shipments/{id}/cancel` | YES | `SHIPPER_ADMIN`, `SHIPPER_LOGIST` | JWT tenant | Guarded | No shipper UI. Status update route is carrier-only |
| Tender / RFx creation | PARTIAL | rfx-service | `POST /api/v1/rfx-events`, template and studio routes | YES | `SHIPPER_ADMIN` buyer-manage. Not `SHIPPER_LOGIST` | JWT tenant | Large buyer surface also used by procurement | No shipper UI. Logist is read-only |
| Bids / offers visibility | AVAILABLE | rfx-service | `GET /api/v1/freight-requests/{id}/bids`, `GET /api/v1/bids/{id}` | YES | `SHIPPER_ADMIN`, `SHIPPER_LOGIST` via combined read | JWT tenant | Policy present | No shipper UI |
| Carrier award | AVAILABLE | rfx-service | `POST /api/v1/rfx-events/{id}/award`, `POST /api/v1/bids/{id}/accept` | YES | `SHIPPER_ADMIN` buyer-manage | JWT tenant | Accept-bid uses buyer-manage | `SHIPPER_LOGIST` denied |
| Multi-stop execution visibility | NOT_EXPOSED | shipment-service execution work is not mounted here | No `/api/v1/transport-executions` route | YES for other shipment routes | None for this capability | n/a | Driver stop routes exist under `/api/v1/driver/me/stops`. Order execution routes are a different aggregate | CP-API-003. Owner C |
| Tracking | PARTIAL | tracking-service behind the gateway | `GET /api/v1/shipments/{shipmentId}/tracking` and `/tracking/locations` | YES | No role check in `tracking.Handler`. Tenant from auth context | JWT tenant passed to tracking with an internal token | Any authenticated tenant caller | CP-API-006 |
| Exceptions / problems | DEFERRED | control-tower-read-model via gateway | `/api/v1/control-tower/cases`, critical events | YES | `SHIPPER_ADMIN`, `SHIPPER_LOGIST`, `CARRIER_DISPATCHER`, `FORWARDER_MANAGER`. Not `CARRIER_ADMIN`. Not consignee | Operator workspace | This is the operator control tower, not a customer exception inbox | CP-CARRIER-003 for the carrier role split. Not a shipper portal module. |
| Delivery disposition / rejection | NOT_EXPOSED | shipment-service via driver handler | `POST /api/v1/driver/me/stops/{stopId}/actions/{actionId}/delivery-disposition` | YES | Driver flow. Gateway test rejects `CARRIER_DISPATCHER` | Driver context | Not a shipper route | CP-API-004 |
| Return / redirection visibility | NOT_IMPLEMENTED | none on the public router | Authorize-return, authorize-redirect, and hold paths return 404 in `driver_stop_gateway_test.go` | n/a | None | n/a | No customer read model | CP-API-004 |
| Documents | PARTIAL | document-service | `GET /api/v1/documents/{id}` uses trusted `X-Tenant-ID`. `GET /api/v1/documents` requires query `tenant_id` | YES at gateway | No document role matrix found on the gateway proxy | Get-by-id is header tenant. List trusts the query value | Unsafe to call list from a portal until list uses the trusted tenant | CP-API-005. Owner B |
| EDO / signatures | PARTIAL | document-service | `/api/v1/signing-sessions/{id}`, document signing routes | YES | No customer role matrix on the proxy | Mixed header and body `tenant_id` on writes | Signing exists for documents. It is not a portal flow | Owner B. Portal must not define signature facts |
| Invoices / billing | PARTIAL | billing-register-service | `GET /api/v1/billing-registers` | YES | `SHIPPER_ADMIN`, `SHIPPER_LOGIST` read. Mutate includes `SHIPPER_ADMIN` and `SHIPPER_LOGIST` | Company context enforcer | Billing registers are not a customer invoice document | No shipper UI. KPI math is not portal-owned |
| Payment status | PARTIAL | payment-service | `GET /api/v1/payment-obligations`, `GET /api/v1/payments` | YES | `SHIPPER_ADMIN`, `SHIPPER_LOGIST` read. Create excludes logist | Company context | Read of obligations exists | No shipper UI |
| Analytics / KPI | PARTIAL | gateway analytics handler | `GET /api/v1/analytics/kpis/{kpiId}` | YES | `SHIPPER_ADMIN`, `SHIPPER_LOGIST` in `analyticsrbac` | Handler enforces read roles | One KPI route. Definitions belong to Agent E | `PORTAL_MAY_DEFINE_ANALYTICS_KPI=NO` |

## Carrier

| Capability | Class | SOURCE_SERVICE | PUBLIC_ROUTE | AUTH_REQUIRED | ROLE_ALLOWED | TENANT_SCOPE | CURRENT_READINESS | BLOCKER |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Tender invitations | AVAILABLE | rfx-service | `GET /api/v1/carrier/rfx-events/{id}`, carrier-response routes | YES | `CARRIER_ADMIN`, `CARRIER_DISPATCHER` | JWT tenant | Carrier-specific routes exist. UI for them lives in web-procurement, not web-carrier | No carrier portal UI |
| Tender participation | AVAILABLE | rfx-service | `POST /api/v1/rfx-events/{id}/responses`, carrier-response start/answers/submit | YES | `CARRIER_ADMIN`, `CARRIER_DISPATCHER` | JWT tenant | Policy `PolicyCarrierRespond` | No carrier portal UI |
| Bids | AVAILABLE | rfx-service | `POST /api/v1/freight-requests/{id}/bids`, `POST /api/v1/bids/{id}/submit` | YES | `CARRIER_ADMIN`, `CARRIER_DISPATCHER` | JWT tenant | Same | No carrier portal UI |
| Awarded shipments | PARTIAL | rfx-service for award read; shipment-service for execution | `GET /api/v1/rfx-events/{id}/own-award` | YES | `CARRIER_ADMIN`, `CARRIER_DISPATCHER` | JWT tenant | Own-award is available. Awarded shipment rows are not a separate customer resource | Shipment list is not carrier-scoped (CP-API-001) |
| Assigned shipments | PARTIAL | shipment-service | `GET /api/v1/order-execution/carrier/transport-orders`, `GET /api/v1/carrier/transport-orders` | YES | `CARRIER_ADMIN`, `CARRIER_DISPATCHER` plus shipper roles on the shared read policy | JWT tenant | Execution read policy includes shipper and carrier codes | Not the same as a carrier-company shipment inbox |
| Shipment details | PARTIAL | shipment-service | `GET /api/v1/shipments/{id}` | YES | No role policy | Tenant id only | Same hole as shipper | CP-API-001 |
| Execution status | PARTIAL | shipment-service | `GET /api/v1/order-execution/transport-orders/{id}` | YES | Shipper and carrier execution read roles | JWT tenant | Single-order execution, not multi-stop revision visibility | CP-API-003 |
| Driver assignment | AVAILABLE | shipment-service | `POST /api/v1/shipments/{id}/assign-driver` | YES | `CARRIER_ADMIN`, `CARRIER_DISPATCHER` | JWT tenant | Fleet assign policy | Do not move driver self-service here |
| Vehicle assignment | AVAILABLE | shipment-service | `POST /api/v1/shipments/{id}/assign-vehicle`, `GET/POST /api/v1/vehicles` | YES | View and assign: admin and dispatcher. Create: `CARRIER_ADMIN` | JWT tenant | Fleet policies in `fleetrbac` | No carrier portal UI |
| Tracking | PARTIAL | tracking-service | Same shipment tracking routes as shipper | YES | No carrier-specific check | JWT tenant | Not limited to the assigned carrier | CP-API-006 |
| Exceptions | DEFERRED | control tower | Control tower case routes | YES | `CARRIER_DISPATCHER` only among carrier codes | Operator workspace | `CARRIER_ADMIN` is absent from `controlTowerAccessRoles` | CP-CARRIER-003 |
| Documents | PARTIAL | document-service | Same document routes | YES | No carrier role matrix on the proxy | List query tenant | Same as shipper | CP-API-005 |
| Settlement / payment | PARTIAL | billing-register-service, payment-service | `GET /api/v1/freight-settlements`, billing registers, payments | YES | `CARRIER_ADMIN`, `CARRIER_DISPATCHER` on billing read. Payment read includes `CARRIER_ADMIN` and unseeded `CARRIER_ACCOUNTANT`, not dispatcher | Company context | Dispatcher cannot read payments. Accountant role is not seeded | CP-API-003 |
| Backhaul / NLO recommendations | PARTIAL | network-optimizer-service | `POST /api/v1/network/next-load/search`, marketplace load reads, route-plan evaluate | YES | Carrier actor plus `CARRIER_ADMIN` or `CARRIER_DISPATCHER` | Company context | Carrier search exists. It is an optimizer API, not a portal recommendation contract | Owner D. Portal displays only. `PORTAL_MAY_DEFINE_BACKEND_BUSINESS_FACTS=NO` |

Driver stop arrival, POD upload, delay, and exception posts are `DEFERRED` for the carrier portal. They are implemented for `apps/driver-mobile` on `/api/v1/driver/me/**`.

## Consignee

`CONSIGNEE_OPERATOR` is the only seeded consignee code. `consignee_company_id` on a shipment or transport order does not grant this role a list.

| Capability | Class | SOURCE_SERVICE | PUBLIC_ROUTE | AUTH_REQUIRED | ROLE_ALLOWED | TENANT_SCOPE | CURRENT_READINESS | BLOCKER |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Inbound shipment list | NOT_IMPLEMENTED | shipment-service list has an optional `consignee_company_id` query | `GET /api/v1/shipments?consignee_company_id=` | YES | Query is not tied to the caller | Tenant-wide list if the query is omitted | Filter is not authorization | CP-API-001, CP-CONSIGNEE-002 |
| Inbound shipment detail | NOT_IMPLEMENTED | `GetByIDAndTenant` | `GET /api/v1/shipments/{id}` | YES | Any caller who knows the id inside the tenant | Tenant | No consignee participation check | CP-CONSIGNEE-002 |
| ETA / tracking | PARTIAL | tracking-service | `GET /api/v1/shipments/{shipmentId}/eta` and tracking routes | YES | No consignee role check | JWT tenant | Route exists for any authenticated tenant user | CP-API-006 |
| Delivery status | NOT_IMPLEMENTED | Status is a shipment field | Shipment detail | YES | Not consignee-scoped | Tenant | No consignee projection | CP-CONSIGNEE-002 |
| Delivery appointment / slot | PARTIAL | tracking-service | `GET /api/v1/shipments/{shipmentId}/slots` | YES | No consignee role check | JWT tenant | Slot read exists without participant check | CP-API-006 |
| Delivery exceptions | NOT_EXPOSED | Driver exception post; control tower cases | `/api/v1/driver/me/.../exceptions`. Control tower excludes consignee | YES | Driver, or control-tower roles | n/a for consignee | Consignee cannot see a dedicated exception list | CP-CARRIER-003 describes the carrier split. Consignee is absent from that allow list. |
| POD / documents | PARTIAL | document-service and driver POD upload | Driver POD is `/api/v1/driver/me/.../pod/uploads`. Document get is shared | YES | Driver for POD write. Document get is tenant header | List still uses query tenant | Consignee has no POD inbox | CP-API-005 |
| Delivery rejection visibility | NOT_EXPOSED | Driver delivery-disposition command | Driver route only | YES | Driver | Driver context | No consignee read | CP-API-004 |
| Return / redirection status | NOT_IMPLEMENTED | Public authorize-return and hold routes are 404 | None | n/a | None | n/a | No customer status route | CP-API-004 |
| Shipment event history | PARTIAL | gateway shipment-events handler | `GET /api/v1/shipments/{shipmentId}/events` | YES | Policy includes `CONSIGNEE_OPERATOR` and unseeded `CONSIGNEE_VIEWER` | Depends on handler tenant context | This is the only gateway policy that names the seeded consignee role | Still not a consignee inbox. CP-CONSIGNEE-003 for the viewer code |

Consignee company profile is only the generic company API. `PolicyRead` allows a member. `CONSIGNEE_OPERATOR` is not in `companyAdminRoles`, so member administration is not allowed for that code. Company list policy allows any caller that reaches the guard.

## What a portal must not call

- Tracking-service or other upstreams except through the gateway.
- `/api/v1/driver/me/**` from web-shipper, web-carrier, or web-consignee.
- Document list with a tenant id chosen in the browser, until document-service list uses the trusted header the way get-by-id already does (`trustedTenantID` in `document_handler.go`).
- Analytics KPI definitions invented in the frontend. The portal may later render `GET /api/v1/analytics/kpis/{kpiId}` for roles Agent E already allows.
