# RFx v3.0B — Discovery Report

**Lane:** BACKEND + SHARED (pre-implementation)  
**Base SHA:** `6262dcbdb08edbb66f1698195d2885fcc9fcc46f`  
**Architecture:** v3.0A docs staged from `discovery/bintrans-enterprise-rfx-v3.0a` (PR #101 merge blocked by policy; content included in worktree)

## CURRENT_RFX_CREATION

- Buyer creates RFx via `POST /api/v1/rfx-events` → status `DRAFT`
- UI: `apps/web-admin/components/rfx/RfxCreateModal.vue` (small modal, unchanged in v3.0B)
- Validation: `domain.ValidateCreateRfxEventInput`, owner company from server-resolved membership

## CURRENT_DRAFT_MODEL

- Event-level `DRAFT` status; update allowed only in `DRAFT`
- No questionnaire version before v3.0B
- **v3.0B adds:** `rfx.rfx_versions` draft working copy per event (`draft_version_id` on `rfx_events`)

## CURRENT_QUESTIONNAIRE_MODEL

- **Before v3.0B:** Not implemented (no sections/questions tables)
- **v3.0B adds:** `rfx_sections`, `rfx_questions`, `rfx_question_options`, `rfx_question_rules`

## CURRENT_PUBLISH_MODEL

- `POST /api/v1/rfx-events/{id}/publish` from `DRAFT` only
- Long-form types require ≥1 lot
- **v3.0B adds:** `POST /api/v1/rfx-events/{id}/validate-publish` readiness gate (no auto-publish)

## CURRENT_AUTH_MODEL

- Gateway RBAC: `PolicyBuyerManage` / `PolicyBuyerRead`
- Service: `ResolveBuyerCompanyID`, cross-tenant → 404, buyer role required
- **Reused unchanged** for questionnaire mutations

## REUSE_POINTS

| Area | Path |
|------|------|
| Event CRUD | `services/rfx-service/internal/service/rfx_service.go` |
| Buyer auth | `owner_authorization.go`, `membership_repository.go` |
| Audit | `audit_support.go`, `rfx.audit_events` |
| Repository patterns | `rfx_repository.go`, `db_helpers.go` |
| Handler context | `handlers/context.go` |
| Frontend RFx list/detail | `pages/rfx/`, `useRfxApi.ts` |

## MIGRATION_REQUIRED

**Yes** — migration `000065_rfx_questionnaire_v3_0b` adds versioning + questionnaire tables.

## LEGACY COMPATIBILITY

- `questionnaire_enabled` defaults `FALSE`; legacy RFx events unaffected
- Existing publish/participant/evaluation/award flows unchanged

## SYNC GATE STATUS

| Gate | Status |
|------|--------|
| SYNC_GATE_1 (domain + migration + API skeleton) | **PASS** (backend lane) |
| SYNC_GATE_2 (OpenAPI + frontend contract) | **IN_PROGRESS** |
| SYNC_GATE_3 (integration tests) | **NOT_RUN** |
| SYNC_GATE_4 (full CI) | **NOT_RUN** |
