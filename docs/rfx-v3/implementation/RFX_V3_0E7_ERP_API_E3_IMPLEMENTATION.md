# RFx v3.0E7 — ERP API E3 Implementation

**Wave:** E3 (JSON parser, mapping pin, CREATE/UPDATE Preview)
**Branch:** `feat/rfx-erp-api-e3-preview-v3.0e7`
**Status:** `IMPLEMENTED_PENDING_REVIEW`

## Authorization markers

```
ERP_API_E3_AUTHORIZED=YES
ERP_API_E3_STATUS=IMPLEMENTED_PENDING_REVIEW
CONTRACT_ALIGNMENT_GATE=PASS
ERP_JSON_MAX_DEPTH=32
DUPLICATE_JSON_KEYS=REJECT_FAIL_CLOSED
MIGRATION_000075_CREATED=NO
ERP_COMMIT_IMPLEMENTED=NO
ERP_GET_IMPLEMENTED=NO
PUBLIC_ERP_PREVIEW_ROUTES=2
```

## Delivered scope

| Area | Deliverable |
|---|---|
| ERP JSON parser | `services/rfx-service/internal/erpjson/` |
| Mapping resolver | `services/rfx-service/internal/service/erp_mapping_resolver.go` |
| CREATE Preview | `POST /v1/integrations/erp/rfx/drafts/preview` |
| UPDATE Preview | `POST /v1/rfx-events/{id}/erp-import/preview` |
| Feature flag | `RFX_ERP_INTEGRATION_ENABLED` → 404 when disabled |
| Gateway proxy | `/api/v1/integrations/erp` → rfx-service |
| Route manifest | `packages/shared-go/rfx/e7_erp_preview_routes.go` |
| OpenAPI | `postErpRfxDraftCreatePreview`, `postErpRfxDraftUpdatePreview` |
| Primary tests | E7P2-INT-133,135,136,142,143,146,147,148,149,165–176,195 |

## Explicitly not in E3

CREATE/UPDATE Commit, idempotency HTTP Commit, GET ERP RFx, status/capabilities, publish, migration 000075, INT-196, UI, XLSX behavior changes.

## Primary test files

- `services/rfx-service/internal/integration/erp/e3_create_preview_integration_test.go`
- `services/rfx-service/internal/integration/erp/e3_preview_primary_integration_test.go`
