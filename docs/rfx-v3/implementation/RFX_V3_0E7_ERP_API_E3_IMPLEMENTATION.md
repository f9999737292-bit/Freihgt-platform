# RFx v3.0E7 — ERP API E3 Implementation

**Wave:** E3 (JSON parser, mapping pin, CREATE/UPDATE Preview)
**Branch:** `feat/rfx-erp-api-e3-preview-v3.0e7`
**Status:** `IMPLEMENTED_ACCEPTED`

## Authorization markers

```
ERP_API_E3_AUTHORIZED=YES
ERP_API_E3_STATUS=IMPLEMENTED_ACCEPTED
ERP_API_E3_1_AUTHORIZED=YES
ERP_API_E3_1_STATUS=IMPLEMENTED_ACCEPTED
ERP_API_E4_STATUS=NOT_STARTED
ERP_API_E4_AUTHORIZED=NO
CONTROLLER_VERDICT=ACCEPT_ERP_API_E3_1
NEXT_ACTION=ERP_API_E3_1_CONTROLLED_MERGE
CONTRACT_ALIGNMENT_GATE=PASS
ERP_JSON_MAX_DEPTH=32
DUPLICATE_JSON_KEYS=REJECT_FAIL_CLOSED
MIGRATION_000075_CREATED=NO
ERP_COMMIT_IMPLEMENTED=NO
ERP_GET_IMPLEMENTED=NO
PUBLIC_ERP_PREVIEW_ROUTES=2
FOLLOW_UP_FINDINGS_BLOCK_E3_MERGE=NO
FOLLOW_UP_FINDINGS_BLOCK_MERGE=NO
```

## E3.1 hardening (accepted)

```
F-E3-C1=CONTENT_TYPE_ENFORCEMENT
F-E3-C2=NESTED_UNKNOWN_FIELD_REJECTION
F-E3-C3=UNIFIED_INGEST_PATH
F-E3-C4=PER_MAPPING_TYPE_PINNING
F-E3-C5=STABLE_ERROR_MESSAGE_KEYS
F_E31_L1=UPDATE_REJECTED_MEDIA_TYPE_ZERO_WRITE_ASSERTION
F_E31_L2=PIN_ONLY_HASH_MUTATION_EXPLICIT_TEST
FOLLOW_UP_FINDINGS_BLOCK_MERGE=NO
ERP_API_E4_STATUS=NOT_STARTED
ERP_API_E4_AUTHORIZED=NO
MIGRATION_000075_CREATED=NO
```

## Controller acceptance

| Field | Value |
|---|---|
| Reviewed PR | #143 |
| Reviewed head | `f60e1917979cb2133f7d5fcd74cbdd8d66ddea79` |
| Reviewed base | `33a3db78762073e12c551883829b044212f6f15e` |
| Accepted CI | `35382305777` (51/51 success) |
| Controller verdict | `ACCEPT_ERP_API_E3` |

Accepted non-blocking follow-up findings (do not block E3 merge; do not start E4):

```
F-E3-C1=CONTENT_TYPE_ENFORCEMENT
F-E3-C2=NESTED_UNKNOWN_FIELD_REJECTION
F-E3-C3=UNIFIED_INGEST_PATH
F-E3-C4=PER_MAPPING_TYPE_PINNING
F-E3-C5=STABLE_ERROR_MESSAGE_KEYS
```

## E3.1 controller acceptance

| Field | Value |
|---|---|
| Reviewed PR | #144 |
| Reviewed head | `f4a0ed73c733198b451561e4b49a9acde46eba37` |
| Reviewed base | `5ea656bf39507caaa6dc3b57768db8f6892523a3` |
| Accepted CI | `35390138371` (51/51 success) |
| Controller verdict | `ACCEPT_ERP_API_E3_1` |

Accepted non-blocking follow-up findings (do not block E3.1 merge; do not start E4):

```
F_E31_L1=UPDATE_REJECTED_MEDIA_TYPE_ZERO_WRITE_ASSERTION
F_E31_L2=PIN_ONLY_HASH_MUTATION_EXPLICIT_TEST
FOLLOW_UP_FINDINGS_BLOCK_MERGE=NO
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
