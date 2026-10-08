# Customer Portals 0.1A — gaps

Blocking means a customer cannot safely use that portal capability until the gap is closed. Recommended stage is the next planning slice, not an authorization to implement it in this branch.

```text
TOTAL_GAPS=23
BLOCKING_GAPS=13
```

## Auth

### CP-AUTH-001

| Field | Value |
| --- | --- |
| PORTAL | Shipper, Carrier, Consignee |
| DESCRIPTION | None of the three apps has a login page, session store, auth middleware, or logout. |
| BLOCKING | YES |
| OWNER_AGENT | F |
| SOURCE_EVIDENCE | `apps/web-shipper/pages/index.vue` and the matching carrier and consignee pages. No `middleware/` directory. Contrast `apps/web-admin/pages/login.vue` and `stores/auth.ts`. |
| RECOMMENDED_STAGE | CUSTOMER-PORTALS-0.1C after 0.1B freezes the auth flow |

### CP-AUTH-002

| Field | Value |
| --- | --- |
| PORTAL | Shipper, Carrier, Consignee |
| DESCRIPTION | The working admin client stores the JWT in `localStorage` and also sends `X-Tenant-ID` from a client tenant store. The gateway replaces that header from the JWT. A portal must not treat the client header as authority, and must not copy document-list query `tenant_id`. |
| BLOCKING | YES |
| OWNER_AGENT | F |
| SOURCE_EVIDENCE | `apps/web-admin/stores/auth.ts`, `apps/web-admin/utils/buildApiRequestHeaders.ts`, `services/api-gateway/internal/http/middleware/auth.go` |
| RECOMMENDED_STAGE | CUSTOMER-PORTALS-0.1B freeze, then 0.1C implementation |

### CP-AUTH-003

| Field | Value |
| --- | --- |
| PORTAL | Shipper, Carrier, Consignee |
| DESCRIPTION | `X-Company-ID` is accepted from the client and only revalidated on routes that use the company-context enforcer. Shipment list, shipment detail, and tracking do not. |
| BLOCKING | YES |
| OWNER_AGENT | A |
| SOURCE_EVIDENCE | `services/api-gateway/internal/http/middleware/auth.go` keeps `RequestedCompanyID`. `services/api-gateway/internal/companycontext/actor.go` strips company headers only inside that enforcer. |
| RECOMMENDED_STAGE | Gateway hardening coordinated with Agent C shipment scope. Portal 0.1B must not invent a client company override. |

## Shipper

### CP-SHIPPER-001

| Field | Value |
| --- | --- |
| PORTAL | Shipper |
| DESCRIPTION | Shipper portal has no transport-order, shipment, tender, tracking, document, or billing screens. |
| BLOCKING | YES |
| OWNER_AGENT | F |
| SOURCE_EVIDENCE | `apps/web-shipper/pages/index.vue` |
| RECOMMENDED_STAGE | First shipper implementation wave after 0.1B |

### CP-SHIPPER-002

| Field | Value |
| --- | --- |
| PORTAL | Shipper |
| DESCRIPTION | `SHIPPER_ADMIN` and `SHIPPER_LOGIST` are different gateway policies. Logist cannot create transport orders, manage RFx, or accept bids. A portal that shows one "shipper" capability set will over-promise. |
| BLOCKING | NO |
| OWNER_AGENT | F |
| SOURCE_EVIDENCE | `services/api-gateway/internal/transportorderrbac/policies.go`, `services/api-gateway/internal/rfxrbac/policies.go` |
| RECOMMENDED_STAGE | CUSTOMER-PORTALS-0.1B role matrix |

## Carrier

### CP-CARRIER-001

| Field | Value |
| --- | --- |
| PORTAL | Carrier |
| DESCRIPTION | Carrier portal has no tender inbox, bid, award, fleet, execution, or settlement screens. |
| BLOCKING | YES |
| OWNER_AGENT | F |
| SOURCE_EVIDENCE | `apps/web-carrier/pages/index.vue` |
| RECOMMENDED_STAGE | First carrier implementation wave after 0.1B |

### CP-CARRIER-002

| Field | Value |
| --- | --- |
| PORTAL | Carrier |
| DESCRIPTION | Driver operational actions already have an app and an API. They must not move into web-carrier. |
| BLOCKING | NO |
| OWNER_AGENT | F |
| SOURCE_EVIDENCE | `apps/driver-mobile/**`, `services/api-gateway/internal/http/router.go` `/api/v1/driver/me/**`. Role `DRIVER` in `000009_seed_roles.up.sql`. |
| RECOMMENDED_STAGE | CUSTOMER-PORTALS-0.1B boundary. Owner C keeps the driver API. |

### CP-CARRIER-003

| Field | Value |
| --- | --- |
| PORTAL | Carrier |
| DESCRIPTION | Control tower allows `CARRIER_DISPATCHER` and not `CARRIER_ADMIN`. A carrier exception screen cannot assume both carrier codes see the same operator API. |
| BLOCKING | NO |
| OWNER_AGENT | C |
| SOURCE_EVIDENCE | `services/api-gateway/internal/controltower/rbac.go` |
| RECOMMENDED_STAGE | Later carrier exception slice. Not the first portal wave. |

## Consignee

### CP-CONSIGNEE-001

| Field | Value |
| --- | --- |
| PORTAL | Consignee |
| DESCRIPTION | Consignee portal has no inbound shipment, ETA, slot, document, or profile screens. |
| BLOCKING | YES |
| OWNER_AGENT | F |
| SOURCE_EVIDENCE | `apps/web-consignee/pages/index.vue` |
| RECOMMENDED_STAGE | After a consignee read model exists |

### CP-CONSIGNEE-002

| Field | Value |
| --- | --- |
| PORTAL | Consignee |
| DESCRIPTION | `consignee_company_id` is stored on shipments and transport orders. It is not an authorization principal. Company actor derivation has no consignee kind. |
| BLOCKING | YES |
| OWNER_AGENT | C |
| SOURCE_EVIDENCE | `services/shipment-service/internal/http/handlers/shipment_handler.go` optional query filter. `services/api-gateway/internal/companycontext/actor.go` buyer and carrier arms only. |
| RECOMMENDED_STAGE | TMS consignee visibility, before any consignee portal build |

### CP-CONSIGNEE-003

| Field | Value |
| --- | --- |
| PORTAL | Consignee |
| DESCRIPTION | `CONSIGNEE_VIEWER` is named in shipment-event RBAC and web-admin permission mapping and is not seeded. |
| BLOCKING | YES |
| OWNER_AGENT | A |
| SOURCE_EVIDENCE | No `CONSIGNEE_VIEWER` insert under `infrastructure/migrations`. Reference in `services/api-gateway/internal/shipmentevents/rbac.go`. |
| RECOMMENDED_STAGE | Identity role seed, or remove the code from policies. Do not build a portal against it until one of those happens. |

## Shared frontend

### CP-SHARED-001

| Field | Value |
| --- | --- |
| PORTAL | Shared |
| DESCRIPTION | Production form, table, badge, modal, and empty-state components live in `apps/web-admin/components/ui`. `packages/ui` exports two components. |
| BLOCKING | NO |
| OWNER_AGENT | F |
| SOURCE_EVIDENCE | `packages/ui/src/index.ts`, `apps/web-admin/components/ui/*.vue` |
| RECOMMENDED_STAGE | CUSTOMER-PORTALS-0.1B design-system policy |

### CP-SHARED-002

| Field | Value |
| --- | --- |
| PORTAL | Shared |
| DESCRIPTION | Auth store, API client, and role guards are app-local. `packages/shared-ts` has no API client. |
| BLOCKING | NO |
| OWNER_AGENT | F |
| SOURCE_EVIDENCE | `packages/shared-ts/src/types/index.ts`, `apps/web-admin/composables/useApi.ts`, `apps/web-procurement/middleware/auth.ts` |
| RECOMMENDED_STAGE | CUSTOMER-PORTALS-0.1B, extraction only if topology B or A needs it |

### CP-SHARED-003

| Field | Value |
| --- | --- |
| PORTAL | Shared |
| DESCRIPTION | Shared i18n catalogs have four common keys. Product copy in the skeleton hint is hardcoded English. Admin product strings are in the admin app. |
| BLOCKING | NO |
| OWNER_AGENT | F |
| SOURCE_EVIDENCE | `packages/i18n/src/locales/ru-RU.json`, `en-US.json`. Hint in each portal `pages/index.vue`. |
| RECOMMENDED_STAGE | CUSTOMER-PORTALS-0.1B i18n policy |

## API

### CP-API-001

| Field | Value |
| --- | --- |
| PORTAL | Shipper, Carrier, Consignee |
| DESCRIPTION | Shipment list and shipment detail are scoped by trusted tenant id only. Company query parameters do not prove the caller belongs to that company. |
| BLOCKING | YES |
| OWNER_AGENT | C |
| SOURCE_EVIDENCE | `services/shipment-service/internal/http/handlers/shipment_handler.go` `List` and `GetByID`. Explicit shipment RBAC in the gateway covers create, accept, status, and cancel, not list or get. |
| RECOMMENDED_STAGE | Before any portal shipment inbox |

### CP-API-002

| Field | Value |
| --- | --- |
| PORTAL | Carrier |
| DESCRIPTION | `CARRIER_ACCOUNTANT` is allowed on billing, payment, rates, and analytics policies and is not seeded. |
| BLOCKING | NO |
| OWNER_AGENT | A |
| SOURCE_EVIDENCE | `services/api-gateway/internal/paymentrbac/guard.go`. No migration insert for the code. |
| RECOMMENDED_STAGE | Identity seed if settlement UI needs that role. Carrier admin remains the seeded payment reader. |

### CP-API-003

| Field | Value |
| --- | --- |
| PORTAL | Shipper, Carrier, Consignee |
| DESCRIPTION | No public customer route exposes multi-stop transport execution. Driver stop commands are not a substitute. |
| BLOCKING | NO |
| OWNER_AGENT | C |
| SOURCE_EVIDENCE | No `transport-executions` route in `services/api-gateway/internal/http/router.go`. Driver routes stop at `/api/v1/driver/me/**`. |
| RECOMMENDED_STAGE | After Agent C publishes a customer read model |

### CP-API-004

| Field | Value |
| --- | --- |
| PORTAL | Shipper, Consignee |
| DESCRIPTION | Delivery disposition is a driver command. Return authorization, redirect authorization, and hold are not registered and respond 404 in the driver gateway test. |
| BLOCKING | NO |
| OWNER_AGENT | C |
| SOURCE_EVIDENCE | `services/api-gateway/internal/http/router.go` delivery-disposition route. `services/api-gateway/internal/http/driver_stop_gateway_test.go` blocked paths. |
| RECOMMENDED_STAGE | TMS disposition visibility, then portal rendering |

### CP-API-005

| Field | Value |
| --- | --- |
| PORTAL | Shipper, Carrier, Consignee |
| DESCRIPTION | Document get-by-id uses the trusted tenant header. Document list requires `tenant_id` in the query. Writes parse `tenant_id` from the body. |
| BLOCKING | YES |
| OWNER_AGENT | B |
| SOURCE_EVIDENCE | `services/document-service/internal/http/handlers/document_handler.go` `List`, `trustedTenantID`, `parseCreateDocumentRequest` |
| RECOMMENDED_STAGE | EDO/document read path before a portal document screen |

### CP-API-006

| Field | Value |
| --- | --- |
| PORTAL | Shipper, Carrier, Consignee |
| DESCRIPTION | Tracking, ETA, and slot reads require a JWT tenant and do not check that the caller participates in the shipment. |
| BLOCKING | YES |
| OWNER_AGENT | C |
| SOURCE_EVIDENCE | `services/api-gateway/internal/tracking/handler.go` `buildRequestContext` |
| RECOMMENDED_STAGE | Before any portal tracking or ETA page |

### CP-API-007

| Field | Value |
| --- | --- |
| PORTAL | Shipper, Carrier |
| DESCRIPTION | Analytics exposes one KPI read route to shipper and carrier role codes. The portal must not define KPI meaning. Consignee is excluded. |
| BLOCKING | NO |
| OWNER_AGENT | E |
| SOURCE_EVIDENCE | `services/api-gateway/internal/analyticsrbac/policy.go`, `GET /api/v1/analytics/kpis/{kpiId}` |
| RECOMMENDED_STAGE | Analytics contract, then optional portal rendering |

## Test and deploy

### CP-TEST-001

| Field | Value |
| --- | --- |
| PORTAL | Shipper, Carrier, Consignee |
| DESCRIPTION | No unit tests and no e2e tests exist for the three apps. |
| BLOCKING | YES |
| OWNER_AGENT | F |
| SOURCE_EVIDENCE | App trees contain no `tests/` or `e2e/` directories. `package.json` has no `test` script. web-admin and web-procurement do. |
| RECOMMENDED_STAGE | CUSTOMER-PORTALS-0.1B test policy, applied on the first implementation wave |

### CP-DEPLOY-001

| Field | Value |
| --- | --- |
| PORTAL | Shipper, Carrier, Consignee |
| DESCRIPTION | No Dockerfile, CI workflow, or staging compose service exists for the three apps. |
| BLOCKING | YES |
| OWNER_AGENT | A |
| SOURCE_EVIDENCE | `.github/workflows/ci.yml` frontend jobs name web-admin and web-procurement only. `apps/web-admin/Dockerfile` is the only app Dockerfile. Staging compose names `web-admin`. |
| RECOMMENDED_STAGE | Packaging after 0.1B deployment boundary. Agent F does not deploy staging in 0.1A. |

## Count check

Blocking: CP-AUTH-001, CP-AUTH-002, CP-AUTH-003, CP-SHIPPER-001, CP-CARRIER-001, CP-CONSIGNEE-001, CP-CONSIGNEE-002, CP-CONSIGNEE-003, CP-API-001, CP-API-005, CP-API-006, CP-TEST-001, CP-DEPLOY-001.

That is 13 identifiers. CP-TEST-001 and CP-DEPLOY-001 block customer release, not the 0.1B architecture freeze.

```text
TOTAL_GAPS=23
BLOCKING_GAPS=13
BLOCKING_FOR_0_1B_FREEZE=0
```

0.1B can freeze policy while these gaps stay open. Implementation waves cannot ignore the blocking rows.
