# Customer portals 0.2B — shipper shipment office

Implemented in `apps/web-shipper` on the shared customer platform (`packages/portal-client`, `packages/ui`, `packages/i18n`, `packages/shared-ts`).

The browser talks only to the API Gateway. The client sends `Authorization`, optional `X-Company-ID` from a fresh eligible shipper membership, `X-Locale`, and `X-Request-ID`. It does not send `X-Tenant-ID`, `X-User-ID`, `X-User-Email`, `X-Actor-Kind`, or browser role headers.

Session policy remains `TAB_SESSION_STORAGE_ACCESS_TOKEN`. The shipper tab uses `freight_shipper_tab_session`, separate from `freight_carrier_tab_session`. `sessionStorage` holds the access token, the server user snapshot, the selected company id, and the server tenant id as display metadata. Memberships are not persisted. After a reload the office fetches memberships again, keeps only `ACTIVE` memberships whose `company_type` is `SHIPPER` and whose roles include `SHIPPER_ADMIN` or `SHIPPER_LOGIST`, and keeps `selectedCompanyId` only when that id is still in that list. Shipment calls wait until that refresh finishes.

`FORWARDER`, `LSP`, and `CARRIER` are not treated as shipper companies. A shipper role on another company does not make that company selectable.

## Implemented

| Slice | State | Gateway routes used |
| --- | --- | --- |
| Login | IMPLEMENTED | `POST /api/v1/auth/login` with `tenant_id`, `email`, `password` |
| Tab session and role gate | IMPLEMENTED | Entry requires an eligible shipper membership. `SHIPPER_ADMIN` and `SHIPPER_LOGIST` may enter. `PROCUREMENT_MANAGER`, `FORWARDER_MANAGER`, `CARRIER_ADMIN`, `CARRIER_DISPATCHER`, `CARRIER_ACCOUNTANT`, `DRIVER`, and `CONSIGNEE_OPERATOR` do not. |
| Company context | IMPLEMENTED | `GET /api/v1/users/{user_id}/companies?tenant_id=&status=ACTIVE`. `tenant_id` is the login snapshot query. It is not an authority header. |
| Customer shell and Home | IMPLEMENTED | Shell landing only. No KPI widgets. |
| Shipment inbox | IMPLEMENTED | `GET /api/v1/shipper/shipments?shipper_company_id=<selected membership>`. Optional `status` is passed through when the user supplies it. |
| Shipment detail | IMPLEMENTED | `GET /api/v1/shipper/shipments/{id}?shipper_company_id=<selected membership>` |
| Error states | IMPLEMENTED | 401 clears the shipper tab session and returns to login. 403, 404, 5xx, and network failure replace the page. An empty list and a missing shipper company have their own states. |

Company and location values are shown as the identifiers the shipment payload returns. The office does not call another API to turn those ids into names.

The legacy operator routes `GET /api/v1/shipments` and `GET /api/v1/shipments/{id}` are not used.

## Portal screen gap

`CP-SHIPPER-001` stays in the 0.1A gap list as the historical screen gap. This slice closes that screen for login, company context, and read-only shipment inbox/detail.

```text
CP-SHIPPER-001=CLOSED
SHIPPER_SHIPMENT_INBOX_SAFE=YES
SHIPPER_SHIPMENT_LIST_USES_SAFE_ROUTE=YES
SHIPPER_SHIPMENT_DETAIL_USES_SAFE_ROUTE=YES
LEGACY_SHIPMENT_READ_USED_BY_PORTAL=NO
CUSTOMER_TRACKING_SAFE=NO
CUSTOMER_DOCUMENTS_SAFE=NO
CONSIGNEE_PORTAL_IMPLEMENTATION_BLOCKED=YES
FORWARDER_DUAL_SIDE_BACKEND_READY=NO
SHIPPER_COMPANY_TYPE_REQUIRED=YES
FORWARDER_AS_SHIPPER_ALIAS=NO
LSP_AS_SHIPPER_ALIAS=NO
PERSISTED_MEMBERSHIPS=NO
```

## Not implemented

```text
TRACKING_IMPLEMENTED=NO
ETA_IMPLEMENTED=NO
SLOTS_IMPLEMENTED=NO
DOCUMENTS_IMPLEMENTED=NO
EDO_IMPLEMENTED=NO
SHIPMENT_MUTATIONS_IMPLEMENTED=NO
RFX_IMPLEMENTED=NO
FINANCE_IMPLEMENTED=NO
```

No shipment create, cancel, or status change. No carrier, driver, or vehicle assignment. No RFx, procurement, transport-order editing, documents, EDO, settlements, billing, payments, analytics, Control Tower, NLO/backhaul, company administration, consignee portal, or forwarder portal.
