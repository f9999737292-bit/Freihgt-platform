# Shipper documents UI 0.1

The existing Shipper Portal shows a read-only documents workspace. It does not add a second portal or a second authentication client.

Documents are listed only after the user selects one shipment. The screen does not walk every shipment to imitate a company-wide register. A link on the shipment detail page opens documents for that shipment. The main navigation item asks for a shipment before any document request.

Reads use the accepted gateway routes and the selected shipper company:

```text
GET /api/v1/shipper/documents?shipper_company_id=&shipment_id=
GET /api/v1/shipper/documents/{documentId}?shipper_company_id=
GET /api/v1/shipper/documents/{documentId}/attachments?shipper_company_id=
GET /api/v1/shipper/documents/{documentId}/attachments/{attachmentId}?shipper_company_id=
GET /api/v1/shipper/documents/{documentId}/attachments/{attachmentId}/content?shipper_company_id=
```

The browser does not call `/api/v1/documents`, `/api/v1/signing-sessions`, or document-service paths. It does not send `tenant_id` or `related_entity_type`. Attachment bytes are downloaded through `PortalClient.download`, which keeps the gateway, membership, and 401 behavior of the JSON client. The file name comes from `Content-Disposition` when that header is a safe file name, otherwise from the attachment `file_name`. The object URL is revoked after the browser starts the download. `PortalClient.download` returns `Content-Type`, `Content-Disposition`, `Content-Length`, and `X-Content-SHA256` when the response exposes them. The gateway already sets `X-Content-SHA256`. Its CORS expose list currently names `Content-Disposition` and does not name `X-Content-SHA256`, so a cross-origin browser may not reveal that checksum header to script. The download itself does not depend on reading it.

The company switch is shown only on the documents workspace. Shipment inbox, shipment detail, and tracking do not gain a company switch in this stage, so those screens cannot keep another company's rows after a documents-only switch.

Changing the selected company on the documents page clears the document, attachment, and shipment context before another read. The previous company's rows are not left on screen, and the next request uses the new company.

A document is rendered only when `related_entity_type` is `SHIPMENT` and `related_entity_id` is the selected shipment. Attachments are requested after that check. A mismatched document is a not-found state: it is not rendered, its attachments are not requested, and download is not offered.

The screen has no create, edit, delete, archive, upload, finalize, sign, QES, or operator-exchange control.

The browser contract is covered by mocked Playwright. This stage does not call a live gateway or document storage.

```text
SHIPPER_DOCUMENTS_UI=YES_READ_ONLY
SHIPMENT_SCOPED_DOCUMENT_LIST=YES
GATEWAY_ONLY=YES
LEGACY_DOCUMENT_ROUTE_USAGE=NO
DIRECT_DOCUMENT_SERVICE_CALL=NO
CUSTOMER_DOCUMENT_WRITE_CONTROL=NO
CUSTOMER_SIGNING_CONTROL=NO
CUSTOMER_QES_CONTROL=NO
SHIPPER_DOCUMENTS_CUSTOMER_SAFE=YES_READ_ONLY
SHIPPER_ATTACHMENT_DOWNLOAD_CUSTOMER_SAFE=YES
CUSTOMER_DOCUMENTS_SAFE=NO_GLOBAL_SHIPPER_READ_ONLY
SHIPPER_DOCUMENT_WRITE_SAFE=NO
SHIPPER_EDO_SIGNING_SAFE=NO
SHIPPER_QES_SAFE=NO
MIGRATION_CREATED=NO
BACKEND_CODE_CHANGED=NO
STAGING_CHANGED=NO
```

The checksum exposure note above is the UI-stage record. The later gate is [SHIPPER_DOCUMENTS_CORS_0_1.md](SHIPPER_DOCUMENTS_CORS_0_1.md).
