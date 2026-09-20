# RFx v3.0E7 — ERP API E6 Implementation

**Wave:** E6 (GET / status / capabilities)
**Branch:** `feat/rfx-erp-contract-closure-e6-v3.0e7`
**Status:** `IMPLEMENTED_ACCEPTED`

## Authorization markers

```
ERP_API_E6_AUTHORIZED=YES
ERP_API_E6_STATUS=IMPLEMENTED_ACCEPTED
ERP_API_E6_IMPLEMENTATION_STARTED=YES
E6_OPENAPI_ROLE=FINAL_EXHAUSTIVE_CONSOLIDATION
E6_IS_FIRST_PUBLICATION_FOR_E2_E5=NO
AUTO_PUBLISH_IMPLEMENTED=NO
AUTO_PUBLISH_AUTHORIZED=NO
MIGRATION_000075_CREATED=NO
MIGRATION_000075_AUTHORIZED=NO
MIGRATION_000075_REQUIRED=NO
INT_196_OCCUPIED=NO
ERP_GET_IMPLEMENTED=YES
CONTROLLER_VERDICT=ACCEPT_ERP_API_E6
CONTROLLER_REVIEW_HEAD=72950d64a0f16251f2b94abfe02c3cfd346f6d45
CONTROLLER_CI_RUN=35491362938
CONTROLLER_CI_RESULT=SUCCESS
FOLLOW_UP_FINDINGS_BLOCK_E6_MERGE=NO
```

## Controller acceptance

| Field | Value |
|---|---|
| Reviewed PR | #147 |
| Reviewed head | `72950d64a0f16251f2b94abfe02c3cfd346f6d45` |
| Reviewed base | `1db7aedcb479af35f77b644b9d61934ea5d05224` |
| Accepted product CI | `35491362938` |
| Controller verdict | `ACCEPT_ERP_API_E6` |

## Non-blocking findings

| ID | Severity | Status | Note |
|---|---|---|---|
| F_E6_L1 | LOW | REMAINS_OPEN | OpenAPI lists `409 event_not_draft` on `getErpImportAnalysisStatus` and `getErpIntegrationCapabilities`. Runtime does not emit that code for those operations; it applies only to the two RFx GET operations. Does not block merge. |

## Delivered operations

| operationId | Gateway | Scope | Success DTO |
|---|---|---|---|
| `getErpRfxEventById` | `GET /api/v1/integrations/erp/rfx-events/{id}` | `rfx:draft:read` | `ErpRfxDraftSummary` |
| `getErpRfxByExternalId` | `GET /api/v1/integrations/erp/rfx/by-external-id` | `rfx:draft:read` | `ErpRfxDraftSummary` |
| `getErpImportAnalysisStatus` | `GET /api/v1/integrations/erp/analyses/{analysis_id}` | `rfx:status:read` | `ErpAnalysisStatus` |
| `getErpIntegrationCapabilities` | `GET /api/v1/integrations/erp/capabilities` | `rfx:status:read` | `ErpIntegrationCapabilities` |

All four routes are integration-protected (machine bearer), feature-flagged (`RFX_ERP_INTEGRATION_ENABLED=false` → 404), and inherit the E2 per-principal rate limit (429). GET writes no audit rows.

## Controller decisions applied

- PUBLISHED / non-DRAFT → **409** `event_not_draft` for both RFx GET operations.
- Query `external_revision` equal to the current E4 link revision → **200** even when history is empty; any other unrecorded revision → **404**.
- Analysis status is principal-owner only; foreign tenant/principal/company → **404**.
- `external_link` uses one object in both RFx responses (`system`, `object_type`, `object_id`, `revision`, optional `requested_revision`).
- `publish_readiness_summary` is counts only (`ready`, `blocking_fail_count`, `warning_count`).
- `draft_row_version` is `rfx_versions.version` (optimistic lock), not `version_number`.
- INT-182 inspects existing E4/E5 audit payloads; GET does not append audit.

## Explicitly not in E6

Migration `000075`, auto-publish, INT-196, TMS/EDI, human DTO reuse.
