# Customer portals 0.2A — carrier office MVP

Implemented in `apps/web-carrier` on top of `packages/portal-client` and the customer components in `packages/ui`.

The browser talks only to the API Gateway. The client sends `Authorization`, optional `X-Company-ID` from a server-returned carrier membership, `X-Locale`, and `X-Request-ID`. It does not send `X-Tenant-ID`, `X-User-ID`, `X-User-Email`, or browser role headers.

Session policy remains `TAB_SESSION_STORAGE_ACCESS_TOKEN`: `sessionStorage` holds the access token, the server user snapshot, the selected company id, and the server tenant id as display metadata. Memberships are not persisted. After a tab reload the office fetches memberships again and keeps `selectedCompanyId` only when that id is an active `CARRIER_ADMIN` or `CARRIER_DISPATCHER` membership in the fresh response. The access token is not written to `localStorage`.

## Implemented

| Slice | State | Gateway routes used |
| --- | --- | --- |
| Login | IMPLEMENTED | `POST /api/v1/auth/login` with `tenant_id`, `email`, `password` |
| Tab session and role gate | IMPLEMENTED | Portal entry allows `CARRIER_ADMIN` and `CARRIER_DISPATCHER` only |
| Company context | IMPLEMENTED | `GET /api/v1/users/{user_id}/companies?tenant_id=&status=ACTIVE`. `tenant_id` is the login snapshot query the identity contract requires. It is not an authority header. |
| Customer shell and Home | IMPLEMENTED | Shell landing only. No KPI widgets. |
| Tender inbox | IMPLEMENTED | `GET /api/v1/carrier/rfx-events` with gateway-supported `status`, `response_filter`, and `search` |
| Tender detail | IMPLEMENTED | `GET /api/v1/carrier/rfx-events/{id}` |
| Bid / response | IMPLEMENTED | `POST /api/v1/rfx-events/{id}/responses`, `PATCH /api/v1/rfx-responses/{id}`, `POST /api/v1/rfx-responses/{id}/submit`, `GET /api/v1/rfx-events/{id}/own-response` |
| Own award | IMPLEMENTED | `GET /api/v1/rfx-events/{id}/own-award` |
| Transport-order list | IMPLEMENTED | `GET /api/v1/carrier/transport-orders?carrier_company_id=<selected membership>` |
| Transport-order detail | IMPLEMENTED | `GET /api/v1/order-execution/transport-orders/{id}?company_id=<selected membership>&actor=CARRIER` |
| Fleet view | IMPLEMENTED | `GET /api/v1/drivers?carrier_company_id=<selected membership>` and `GET /api/v1/vehicles?carrier_company_id=<selected membership>`. Read-only for `CARRIER_ADMIN` and `CARRIER_DISPATCHER`. |
| Error states | IMPLEMENTED | 401 clears the tab session and returns to login. 403, 404, 5xx, and network failure replace the page. Empty results and a missing carrier company have their own states. |

`X-Company-ID` and `carrier_company_id` are sent only when the id is in the server membership list. Selection does not authorize the call. Tender status, deadline, participant identity, award, and response validity are shown as the server returned them.

The invited-tender list is the existing `/api/v1/carrier` gateway prefix. Detail, response, award, transport-order, and fleet reads use the existing gateway routes. RFx carrier membership is resolved on the server. No backend file was changed.

## Backend gaps closed by PR #236

The historical findings stay recorded. PR #236 merged company-context enforcement, and the carrier office now sends the selected membership on the query the gateway requires. `X-Company-ID` remains selection context and is not the only company scope.

```text
CP-CARRIER-API-001=RESOLVED_BY_PR_236
CP-CARRIER-API-002=RESOLVED_BY_PR_236
NEW_BLOCKING_BACKEND_GAPS=0
CARRIER_FLEET_CUSTOMER_SAFE=YES
CARRIER_TRANSPORT_ORDERS_CUSTOMER_SAFE=YES
CARRIER_RFX_SAFE=YES
PERSISTED_MEMBERSHIPS=NO
```

### CP-CARRIER-API-001

`RESOLVED_BY_PR_236`. Fleet list reads send `carrier_company_id` from the fresh server membership. The gateway validates that membership and binds the role on that company. The office stays read-only.

### CP-CARRIER-API-002

`RESOLVED_BY_PR_236`. The carrier transport-order list sends `carrier_company_id`. Detail sends `company_id` and `actor=CARRIER`. The gateway validates the selected membership. The office does not add execution commands.

## Not implemented

Tracking, ETA, slots, documents, EDO, settlements, billing, payments, analytics, Control Tower, backhaul / network optimizer, driver actions, and company or user administration are not in this slice.

```text
CUSTOMER_TRACKING_SAFE=NO
CUSTOMER_DOCUMENTS_SAFE=NO
CONSIGNEE_PORTAL_IMPLEMENTATION_BLOCKED=YES
FORWARDER_DUAL_SIDE_BACKEND_READY=NO
TRACKING_IMPLEMENTED=NO
DOCUMENTS_IMPLEMENTED=NO
SETTLEMENTS_IMPLEMENTED=NO
ANALYTICS_IMPLEMENTED=NO
BACKHAUL_IMPLEMENTED=NO
DRIVER_ACTIONS_IMPLEMENTED=NO
```

Packaging is `apps/web-carrier/Dockerfile`. Health remains `GET /api/health`. Staging compose was not changed.
