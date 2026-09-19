# RFx v3.0E7 — ERP API E5 Implementation

**Wave:** E5 (UPDATE Commit)
**Branch:** `feat/rfx-erp-update-commit-e5-v3.0e7`
**Status:** `IMPLEMENTED_ACCEPTED`

## Authorization markers

```
ERP_API_E5_AUTHORIZED=YES
ERP_API_E5_STATUS=IMPLEMENTED_ACCEPTED
ERP_API_E5_IMPLEMENTATION_STARTED=YES
OPERATION_ID=postErpRfxDraftUpdateCommit
UPDATE_COMMIT_ROUTE=/api/v1/rfx-events/{id}/erp-import/commit
SERVICE_PATH=/v1/rfx-events/{id}/erp-import/commit
SUCCESS_STATUS=200
AUTO_PUBLISH_IMPLEMENTED=NO
AUTO_PUBLISH_AUTHORIZED=NO
MIGRATION_000075_CREATED=NO
MIGRATION_000075_AUTHORIZED=NO
ERP_API_E6_STATUS=NOT_STARTED
ERP_API_E6_AUTHORIZED=NO
ERP_GET_IMPLEMENTED=NO
INT_196_OCCUPIED=NO
CONTROLLER_VERDICT=ACCEPT_ERP_API_E5
CONTROLLER_REVIEW_HEAD=1a12c999d5cea2b2cc72d185773b22090f8ed261
CONTROLLER_CI_RUN=35466989829
CONTROLLER_CI_RESULT=SUCCESS
REVISION_POLICY=B_EXISTING_LINK_REQUIRES_NEW_REVISION
```

## Controller acceptance

| Field | Value |
|---|---|
| Reviewed PR | #146 |
| Reviewed head | `1a12c999d5cea2b2cc72d185773b22090f8ed261` |
| Reviewed base | `85c2cc00e13225c1a146c9667d10270d1edbaf70` |
| Accepted CI | `35466989829` |
| Controller verdict | `ACCEPT_ERP_API_E5` |
| Revision policy | **B** — existing-link UPDATE requires a new unused `external.revision` and a history row |

## Revision policy B (accepted)

If the target event already has an external link, UPDATE Preview must send the same normalized `system`/`object_id` and a new `revision` unused in that link's history. Missing or repeated revision is **422 `VALIDATION_ERROR`** (`details.field=external.revision`) with no analysis persist. Commit backfills the E4 current revision when history is empty, appends the new row, and updates link metadata in place. Events without a link may omit `external`. E6 GET-by-revision, migration `000075`, auto-publish, and INT-196 remain unauthorized.

## Baseline policy (normative reuse, not invented)

| Token | Preview source | Commit compare | Mismatch |
|---|---|---|---|
| `event_row_version` | `rfx_events.version` at UPDATE Preview (`EventVersionState.EventVersion` / `RfxEvent.Version`) | locked event version | `409 stale_target` |
| `draft_row_version` | `rfx_versions.version` of the active DRAFT (optimistic lock column, **not** `version_number`) | locked draft `Version` | `409 proposal_revalidation_failed` |
| `baseline_lots_fingerprint` | `xlsxexchange.ComputeBaselineLotsFingerprint` of the server lot set at Preview | locked current lots | `409 stale_target` |

`analysis.TargetVersion` remains the business `version_number` and is **not** used as a substitute for these tokens.

## Delivered scope

| Area | Deliverable |
|---|---|
| UPDATE Preview | Persist baseline tokens (`event_row_version`, `draft_row_version`, `baseline_lots_fingerprint`, and when a link exists `baseline_external_link_id` / `baseline_external_revision`) in canonical JSON before hash. Existing-link revision policy is enforced before analysis persist. |
| UPDATE Commit | `POST /api/v1/rfx-events/{id}/erp-import/commit` |
| Request | `{analysis_id}` plus mandatory `Idempotency-Key` (max 128, one JSON document) |
| Success | `200` with `rfx_event_id`, `applied_at`; event stays DRAFT |
| Scopes | `rfx:draft:commit` and `rfx:draft:read` |
| Idempotency | `{tenant, integration_principal_id, ERP_BUYER_UPDATE_COMMIT, aggregate_scope=eventID}` |
| External link | Existing link requires a new unused `external.revision` at Preview. Commit backfills the E4 current revision into history if needed, appends the new row, then updates link metadata in place. No `UpsertLink`; no `rfx_event_id` rebind. Unbound UI UPDATE may omit `external`; first bind creates link + history atomically. Link/revision drift after Preview → `409 stale_target`. |
| Audit | `rfx.erp.draft.updated.v1` |
| Route manifest | `packages/shared-go/rfx/e7_erp_update_commit_routes.go` |
| OpenAPI | `postErpRfxDraftUpdateCommit` |

## Explicitly not in E5

GET/status/capabilities (E6), migration `000075`, auto-publish, INT-196, TMS, EDI.
