# RFx v3.0E6 — Studio Frontend (Templates + Versioning)

**Status:** `IMPLEMENTED_PENDING_CONTROLLER_ACCEPTANCE`  
**Scope:** E6 buyer Studio frontend for accepted E1–E5 backend capabilities  
**Base:** `origin/main` @ `9383c248d090a5c38c2cc06f71437e96801078ef`  
**Branch:** `feat/rfx-template-versioning-studio-v3.0e6`

---

## 1. Screens

| Screen | Route | Backend |
|--------|-------|---------|
| Template library | `/rfx/templates` | `GET/POST /api/v1/rfx-templates` |
| Template editor | `/rfx/templates/:id` | Template metadata + questionnaire graph |
| Template version history | `/rfx/templates/:id/versions` | `GET /api/v1/rfx-templates/:id` |
| Event version history | `/rfx/:id/versions` | `GET /api/v1/rfx-events/:id/versions` |
| Version compare | `/rfx/:id/versions/compare` | `POST …/versions/compare` |
| Restore as draft (dialog) | from version history | `POST …/restore-draft` + `Idempotency-Key` |
| Create RFx from template | modal on library | `POST /api/v1/rfx-events/from-template` |
| Studio version nav | `/rfx/:id/studio` | links to event version history |

Change-impact preview UI component is implemented (`RfxChangeImpactPanel.vue`) for E3 integration in republish flows; full republish wiring in Studio validation step remains handoff to E7 browser acceptance.

---

## 2. Feature flag

| Flag | Location | Default |
|------|----------|---------|
| `NUXT_PUBLIC_RFX_VERSIONING_V3_ENABLED` | `apps/web-admin/nuxt.config.ts` | `false` |

When off: buyer template routes redirect to `/rfx`; Studio version-history nav hidden.

Backend gate: `RFX_VERSIONING_V3_ENABLED` (unchanged).

---

## 3. Permissions

- Buyer manage roles: `PLATFORM_ADMIN`, `SHIPPER_*`, `FORWARDER_MANAGER`, `PROCUREMENT_MANAGER`
- Carrier-only roles: template/version management hidden; middleware `rfx-buyer-manage` fail-closed
- Route guard: `middleware/rfx-buyer-manage.ts`

---

## 4. Error semantics (UI)

| HTTP | i18n root |
|------|-----------|
| 400 | `rfx.errors.badRequest` |
| 401 | `rfx.errors.unauthorized` |
| 403 | `rfx.errors.forbidden` |
| 404 | `rfx.errors.notFound` |
| 409 | `rfx.errors.conflict` + machine code mapping |
| 422 | `rfx.errors.validation` |

---

## 5. Test matrix (unit)

Vitest: `apps/web-admin/tests/rfxTemplatesVersioningE6.test.ts` — covers E6-UT-04..36 helpers, i18n keys, OpenAPI parity anchors, competitor-confidentiality middleware regression, Studio nav regression.

Browser E2E (E7): prepared routes; final Chromium + gateway acceptance **not** claimed in E6.

---

## 6. Not in E6

- Late submission backend/UI
- Excel import/export
- ERP/TMS integration
- v3.0F Qualification Pool
- Automatic re-score execution
- Staging/pilot deployment
- Full change-impact republish orchestration in Studio publish button (component ready; E7 browser proof)

---

## 7. Mandatory future gates (preserved)

| Gate | Status |
|------|--------|
| `LATE_SUBMISSION_AND_DEADLINE_EXCEPTIONS` | REQUIRED |
| `BUYER_RFQ_MANUAL_CREATION` | REQUIRED |
| `BUYER_RFQ_TEMPLATE_CREATION` | PARTIAL_E5_BACKEND_PLUS_E6_UI |
| `BUYER_RFQ_EXCEL_IMPORT` | REQUIRED_FUTURE_GATE |
| `BUYER_RFQ_ERP_INTEGRATION` | REQUIRED_FUTURE_GATE |
| `CARRIER_DIRECT_OFFER_ENTRY` | REQUIRED |
| `CARRIER_OFFER_EXCEL_EXPORT_IMPORT` | REQUIRED_FUTURE_GATE |
| `CARRIER_CAN_VIEW_COMPETITOR_*` | NO |
| `USER_TRAINING_COURSE_RU_EN_ZH` | REQUIRED_BEFORE_PILOT |

`LATE_SUBMISSION_BLOCKS_E7_IF_NOT_IMPLEMENTED=YES`

---

## 8. E7 browser handoff

1. Enable `NUXT_PUBLIC_RFX_VERSIONING_V3_ENABLED=true` + backend `RFX_VERSIONING_V3_ENABLED=true`
2. Run library → publish template → clone event → fork/publish versions → compare → restore → material republish with impact confirmation
3. Verify carrier cannot access `/rfx/templates` (403 / redirect)
4. Capture screenshots per `RFX_V3_0E_TEST_STRATEGY.md` L3 scenarios
