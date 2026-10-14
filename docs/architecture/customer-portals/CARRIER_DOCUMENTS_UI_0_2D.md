# Carrier documents UI 0.2D

Read-only shipment documents in the existing Carrier Portal. The screen opens from a transport order that already returns a shipment reference. There is no typed shipment id and no top-level documents item.

The browser calls only:

```text
GET /api/v1/carrier/documents?carrier_company_id=&shipment_id=
GET /api/v1/carrier/documents/{documentId}?carrier_company_id=
GET /api/v1/carrier/documents/{documentId}/attachments?carrier_company_id=
GET /api/v1/carrier/documents/{documentId}/attachments/{attachmentId}/content?carrier_company_id=
```

It does not call `/api/v1/documents`. It does not send `tenant_id`, `owner_company_id`, `shipper_company_id`, `actor`, `role`, or `related_entity_type`. A document is shown only when it belongs to the shipment in the route. Attachments are requested after that check. Download uses `PortalClient.download` and a browser object URL. The file name comes from `Content-Disposition` when that header is a safe file name, otherwise from `file_name`. Storage bucket and object key headers are not read or shown.

Changing the selected company on the documents page clears the list, the open document, and the attachments, then leaves that shipment. A response for the previous company is ignored.

```text
CARRIER_DOCUMENTS_UI=YES_READ_ONLY
SHIPMENT_CONTEXT_ONLY=YES
FREEFORM_SHIPMENT_ID_PRESENT=NO
SELECTED_COMPANY_SCOPED=YES
COMPANY_SWITCH_CLEARS_DOCUMENT_STATE=YES
LEGACY_DOCUMENT_API_USED=NO
DOCUMENT_MUTATION_IMPLEMENTED=NO
STORAGE_METADATA_EXPOSED=NO
BACKEND_CHANGED=NO
GATEWAY_CHANGED=NO
CORS_CHANGED=NO
MIGRATION_CREATED=NO
STAGING_CHANGED=NO
```

Empty documents stay an empty state. Forbidden, not found, and unavailable stay separate from that empty state. `CARRIER_ADMIN` and `CARRIER_DISPATCHER` see the documents action. `DRIVER` and `CARRIER_ACCOUNTANT` do not. The server remains the authorization authority.
