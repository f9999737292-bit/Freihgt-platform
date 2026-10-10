# Customer portals 0.2C — shipper tracking, ETA, and slots

Shipper Portal shows read-only tracking, arrival forecasts, and load windows for one shipment. The page is `/shipments/{id}/tracking`, opened from the shipment detail screen, inside the existing authenticated shipper shell.

Both `SHIPPER_ADMIN` and `SHIPPER_LOGIST` see the same screen. The office still requires a fresh eligible shipper membership before any of these reads. The selected company is the same company used by the shipment inbox and detail.

## Routes

The browser calls only:

- `GET /api/v1/shipper/shipments/{id}/tracking`
- `GET /api/v1/shipper/shipments/{id}/tracking/locations`
- `GET /api/v1/shipper/shipments/{id}/eta`
- `GET /api/v1/shipper/shipments/{id}/eta/history`
- `GET /api/v1/shipper/shipments/{id}/slots`
- `GET /api/v1/shipper/shipments/{id}/slots/history`

Every request sends `shipper_company_id` for the selected company. `X-Company-ID` is set by `PortalClient` after that membership check. The portal does not call the legacy `/api/v1/shipments/{id}/tracking`, `/eta`, or `/slots` routes.

History reads use `limit` and `offset`. The client caps `limit` at 50. Forecast history always sends `targetType=pickup` or `targetType=delivery`. Window history always sends `slotType=pickup` or `slotType=delivery`. Optional `from` and `to` are passed through as entered. The portal does not send shipment, forecast, or window business facts as query authority.

## Screen

Tracking, forecast, and windows load as separate sections. A failure in one section leaves the others on screen. `NOT_CONFIGURED`, a missing forecast, and a missing window are product states. Freshness is shown with the server value (`fresh`, `stale`, `unknown`) and is not turned into another shipment status.

Coordinates and history are text and tables. This slice does not add a map vendor, map library, or map API key.

Timestamps stay in the RFC3339 form returned by the API. When a window includes `timezone`, that value is shown beside the window. The screen does not treat the browser timezone as the operational window timezone.

The safe slot payload does not include `projectedLateBySeconds`, `earlyBySeconds`, or `marginSeconds`. Those fields are not invented in the client. Provider device, event, and slot identifiers are not part of the client types and are not rendered.

The screen is read-only. It does not correct locations, override forecasts, book or edit windows, assign a driver or vehicle, or change shipment status. Documents, EDO, billing, settlements, and payments are not linked from this page.

## Status

```text
SHIPPER_TRACKING_UI_IMPLEMENTED=YES
SHIPPER_ETA_UI_IMPLEMENTED=YES
SHIPPER_SLOTS_UI_IMPLEMENTED=YES
SHIPPER_TRACKING_CUSTOMER_SAFE=YES
SHIPPER_ETA_CUSTOMER_SAFE=YES
SHIPPER_SLOTS_CUSTOMER_SAFE=YES
CUSTOMER_TRACKING_SAFE=NO_GLOBAL_SHIPPER_ONLY
CUSTOMER_DOCUMENTS_SAFE=NO
CONSIGNEE_PORTAL_IMPLEMENTATION_BLOCKED=YES
EXTERNAL_MAP_PROVIDER_ADDED=NO
BACKEND_CODE_CHANGED=NO
```
