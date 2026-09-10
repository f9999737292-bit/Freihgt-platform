# RFx v3.0E6 — Studio Frontend (Templates + Versioning)

**Status:** `IMPLEMENTED_PENDING_CONTROLLER_ACCEPTANCE`
**Scope:** E6 buyer Studio frontend + controller remediation E6-001…E6-008
**Base:** `origin/main` @ `9383c248d090a5c38c2cc06f71437e96801078ef`
**Branch:** `feat/rfx-template-versioning-studio-v3.0e6`

---

## 1. Screens

| Screen | Route | Backend |
|--------|-------|---------|
| Template library | `/rfx/templates` | `GET/POST /api/v1/rfx-templates` |
| Template editor | `/rfx/templates/:id` | Template metadata + DRAFT questionnaire graph |
| Template version history | `/rfx/templates/:id/versions` | `GET /api/v1/rfx-templates/:id` |
| Template version detail | `/rfx/templates/:id/versions/:versionId` | metadata from detail; graph **BLOCKED** for PUBLISHED/SUPERSEDED |
| Event version history | `/rfx/:id/versions` | `GET /api/v1/rfx-events/:id/versions` |
| Event version detail | `/rfx/:id/versions/:versionId` | `GET /api/v1/rfx-events/:id/versions/:versionId` |
| Version compare | `/rfx/:id/versions/compare` | `POST …/versions/compare` |
| Restore as draft (dialog) | from version history | `POST …/restore-draft` + stable `Idempotency-Key` |
| Create RFx from template | modal on library | lazy `GET /api/v1/rfx-templates/:id` + `POST /api/v1/rfx-events/from-template` |
| Studio validation/publish | `/rfx/:id/studio?step=validation` | validate-publish, change-impact preview, questionnaire publish |

---

## 2. Remediation markers (E6-001…E6-008)

| ID | Marker | Status |
|----|--------|--------|
| E6-001 | `E6_001_CLONE_VERSION_LOADING` | PASS — lazy version load + session cache |
| E6-001 | `CREATE_EVENT_FROM_TEMPLATE_UI` | PASS — provenance confirmation before Studio |
| E6-002 | `E6_002_CHANGE_IMPACT_ORCHESTRATION` | PASS — `RfxEventPublishPanel` wired in Studio |
| E6-002 | `REPUBLISH_CONFIRMATION_UI` | PASS |
| E6-003 | `E6_003_TEMPLATE_FORK_GATING` | PASS — `canForkTemplateDraft()` |
| E6-004 | `E6_004_EVENT_VERSION_READ_ONLY` | PASS — event version detail page |
| E6-004 | `E6_004_TEMPLATE_VERSION_READ_ONLY` | BLOCKED_BACKEND — no historical template graph endpoint |
| E6-005 | `E6_005_SERVER_AUTHORITATIVE_READINESS` | PASS — local precheck only; publish 422 parsed |
| E6-006 | `E6_006_IDEMPOTENT_RETRY` | PASS — `IdempotentOperation` for clone/publish/fork/restore |
| E6-007 | `E6_007_PROVENANCE_VISIBLE` | PARTIAL — clone modal + optional GET fields; see gaps |
| E6-008 | `E6_008_COMPONENT_TESTS` | PASS — `rfxE6Remediation.test.ts` (15 behavioral cases) |

---

## 3. Backend contract gaps

| Gap | Impact |
|-----|--------|
| `GET /rfx-templates/{id}/versions/{version_id}` (questionnaire graph) | Template historical read-only graph — metadata only |
| `GET /rfx-events/{id}` provenance fields not in OpenAPI | Post-navigation provenance on event detail blocked unless backend exposes fields |

`BACKEND_CONTRACT_GAP_TEMPLATE_VERSION_DETAIL=YES`
`BACKEND_CONTRACT_GAP_EVENT_PROVENANCE_GET=YES`

No backend changes in E6 remediation scope.

---

## 4. Feature flag

| Flag | Location | Default |
|------|----------|---------|
| `NUXT_PUBLIC_RFX_VERSIONING_V3_ENABLED` | `apps/web-admin/nuxt.config.ts` | `false` |

When off: buyer template routes redirect to `/rfx`; Studio version-history nav and publish panel hidden.

---

## 5. Test matrix

| File | Coverage |
|------|----------|
| `tests/rfxTemplatesVersioningE6.test.ts` | helpers, i18n, OpenAPI anchors (retained) |
| `tests/rfxE6Remediation.test.ts` | E6-REM behavioral logic: clone selection, fork gating, idempotency, publish orchestration, readiness 422 |

Browser E2E (E7): **not started**.

---

## 6. Not in E6

- Late submission backend/UI
- Excel import/export
- ERP/TMS integration
- v3.0F Qualification Pool
- Staging/pilot deployment
- Merge (controller acceptance required)

---

## 7. Mandatory future gates (preserved)

- `LATE_SUBMISSION_AND_DEADLINE_EXCEPTIONS=REQUIRED`
- `BUYER_RFQ_EXCEL_IMPORT=REQUIRED_FUTURE_GATE`
- `BUYER_RFQ_ERP_INTEGRATION=REQUIRED_FUTURE_GATE`
- `CARRIER_OFFER_EXCEL_EXPORT_IMPORT=REQUIRED_FUTURE_GATE`
- `USER_TRAINING_COURSE_RU_EN_ZH=REQUIRED_BEFORE_PILOT`

