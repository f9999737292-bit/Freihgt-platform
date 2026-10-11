# Carrier tracking UI 0.1

Read-only tracking, ETA, and slot screen in the existing Carrier Portal. No new carrier shipment API was added. The browser does not call legacy tenant-wide tracking routes.

The screen opens from a transport order only when the server already returned a shipment reference. The list uses `shipment_id`. The detail uses `shipment.id`. The transport order id is not treated as a shipment id, and the page does not ask the user to type a shipment id.

```text
CARRIER_TRACKING_UI_IMPLEMENTED=YES
CARRIER_ETA_UI_IMPLEMENTED=YES
CARRIER_SLOTS_UI_IMPLEMENTED=YES
CARRIER_TRACKING_CUSTOMER_SAFE=YES
CARRIER_ETA_CUSTOMER_SAFE=YES
CARRIER_SLOTS_CUSTOMER_SAFE=YES
CARRIER_TRACKING_ENTRYPOINT=SERVER_TRANSPORT_ORDER_SHIPMENT_REFERENCE
NEW_CARRIER_SHIPMENT_API_CREATED=NO
FREE_FORM_SHIPMENT_ID_INPUT=NO
CARRIER_TRACKING_UI_READ_ONLY=YES
DRIVER_ACTIONS_EXPOSED_IN_WEB_CARRIER=NO
GLOBAL_TRACKING_NAV_ADDED=NO
EXTERNAL_MAP_PROVIDER_ADDED=NO
MAP_API_KEY_ADDED=NO
CUSTOMER_TRACKING_SAFE=NO_GLOBAL_SHIPPER_AND_CARRIER_ONLY
CP_API_006_CONSIGNEE=OPEN
BACKEND_CODE_CHANGED=NO
MIGRATION_CREATED=NO
STAGING_CHANGED=NO
CARRIER_PORTAL_USES_LEGACY_TRACKING_ROUTE=NO
SELECTED_CARRIER_COMPANY_USED_ON_ALL_TRACKING_READS=YES
CARRIER_HISTORY_PAGE_SIZE_MAX=50
EXECUTION_STOP_CUSTOMER_ETA_TARGET=NO
SECTION_FAILURE_ISOLATION=YES
SLOT_TIMEZONE_SERVER_VALUE_PRESERVED=YES
PROVIDER_CORRELATION_DATA_RENDERED=NO
CROSS_CARRIER_DISCLOSURE_IN_UI=NO
```

Each of tracking, locations, ETA, ETA history, slots, and slot history loads on its own. A failure in one section leaves the others on screen. `NOT_CONFIGURED`, a missing forecast, a missing window, and an empty history stay product states. Slot times keep the timezone value returned by the server. Coordinates are text. There is no map provider and no map API key.

A direct visit to another carrier's shipment stays on the selected company. The gateway response is shown as not available for that company, without naming the other carrier.
