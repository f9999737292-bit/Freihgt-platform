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
| Transport-order list | IMPLEMENTED_BLOCKED_BY_BACKEND | `GET /api/v1/carrier/transport-orders`. See CP-CARRIER-API-002. |
| Transport-order detail | IMPLEMENTED_BLOCKED_BY_BACKEND | `GET /api/v1/order-execution/transport-orders/{id}` read only. See CP-CARRIER-API-002. |
| Fleet view | IMPLEMENTED_BLOCKED_BY_BACKEND | `GET /api/v1/drivers`, `GET /api/v1/vehicles`. Read-only. See CP-CARRIER-API-001. |
| Error states | IMPLEMENTED | 401 clears the tab session and returns to login. 403, 404, 5xx, and network failure replace the page. Empty results and a missing carrier company have their own states. |

`X-Company-ID` and `carrier_company_id` are sent only when the id is in the server membership list. Selection does not authorize the call. Tender status, deadline, participant identity, award, and response validity are shown as the server returned them.

The invited-tender list is the existing `/api/v1/carrier` gateway prefix. Detail, response, award, transport-order, and fleet reads use the existing gateway routes. RFx carrier membership is resolved on the server. No backend file was changed.

## Blocking backend gaps

```text
NEW_BLOCKING_BACKEND_GAPS=2
CARRIER_FLEET_CUSTOMER_SAFE=NO
CARRIER_TRANSPORT_ORDERS_CUSTOMER_SAFE=NO
CARRIER_RFX_SAFE=YES
PERSISTED_MEMBERSHIPS=NO
```

### CP-CARRIER-API-001

Fleet company isolation. `GET /api/v1/drivers` and `GET /api/v1/vehicles` are tenant-wide when `carrier_company_id` is absent. Gateway fleet RBAC checks the role only. `CARRIER_FLEET_CUSTOMER_SAFE=NO`. Owner: Agent A. The carrier office does not add fleet writes.

### CP-CARRIER-API-002

Carrier transport-order company isolation. `carrier_company_id` and `company_id` are client-selected, and the current execution gateway guard is role-only. `CARRIER_TRANSPORT_ORDERS_CUSTOMER_SAFE=NO`. Owner: Agent A. The carrier office does not add execution commands.

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
