# RFx v3.0E7 — ERP API E4 Implementation

**Wave:** E4 (CREATE Commit)
**Branch:** `feat/rfx-erp-create-commit-e4-v3.0e7`
**Status:** `IMPLEMENTED_ACCEPTED`

## Authorization markers

```
ERP_API_E4_AUTHORIZED=YES
ERP_API_E4_STATUS=IMPLEMENTED_ACCEPTED
ERP_API_E4_IMPLEMENTATION_STARTED=YES
OPERATION_ID=postErpRfxDraftCreateCommit
CREATE_COMMIT_ROUTE=/api/v1/integrations/erp/rfx/drafts/commit
SERVICE_PATH=/v1/integrations/erp/rfx/drafts/commit
SUCCESS_STATUS=201
CREATION_CHANNEL=ERP
AUTO_PUBLISH_IMPLEMENTED=NO
AUTO_PUBLISH_AUTHORIZED=NO
MIGRATION_000075_CREATED=NO
MIGRATION_000075_AUTHORIZED=NO
ERP_API_E5_STATUS=NOT_STARTED
ERP_API_E5_AUTHORIZED=NO
ERP_API_E6_STATUS=NOT_STARTED
ERP_API_E6_AUTHORIZED=NO
ERP_GET_IMPLEMENTED=NO
INT_196_OCCUPIED=NO
CONTROLLER_VERDICT=ACCEPT_ERP_API_E4
CONTROLLER_REVIEW_HEAD=3ab1c8e51f694583579af504c2eb24ac2cf75729
CONTROLLER_CI_RUN=35459414961
CONTROLLER_CI_RESULT=SUCCESS
FOLLOW_UP_FINDINGS_BLOCK_MERGE=NO
```

## Controller acceptance

| Field | Value |
|---|---|
| Reviewed PR | #145 |
| Reviewed head | `3ab1c8e51f694583579af504c2eb24ac2cf75729` |
| Reviewed base | `b8657efb16dd63c57feef77385dfbc01b9a6c600` |
| Accepted CI | `35459414961` |
| Controller verdict | `ACCEPT_ERP_API_E4` |

## Delivered scope

| Area | Deliverable |
|---|---|
| CREATE Commit | `POST /api/v1/integrations/erp/rfx/drafts/commit` |
| Request | `{analysis_id}` plus mandatory `Idempotency-Key` (max 128) |
| Success | `201` with `rfx_event_id`, `external_link_id`, `creation_channel=ERP`, `external_revision` |
| Transaction | Lock analysis `FOR UPDATE`, create DRAFT event, insert stable external link, consume analysis, store idempotency |
| Idempotency | Same key+body replays `201`; different body is `409 idempotency_conflict`; losing `Store` rolls back |
| Trailing JSON | Second JSON value or corrupt remainder is `400` before service writes |
| Scopes | `rfx:draft:commit` and `rfx:draft:create` |
| Route manifest | `packages/shared-go/rfx/e7_erp_create_commit_routes.go` |
| OpenAPI | `postErpRfxDraftCreateCommit` |
| Primary tests | E7P2-INT-132, 137–141, 144, 145, 177, 180, 191, 193, 194 |

## Explicitly not in E4

UPDATE Commit (E5), GET/status/capabilities (E6), migration 000075, auto-publish, INT-196, TMS, EDI.

## Primary test files

- `services/rfx-service/internal/integration/erp/e4_create_commit_integration_test.go`
- `services/rfx-service/internal/http/e7_erp_create_commit_route_parity_test.go`
- `services/api-gateway/internal/rfxrbac/e7_erp_create_commit_route_parity_test.go`
