# RFx v3.0E6 — Studio Frontend (Templates + Versioning)

**Status:** IMPLEMENTED_ACCEPTED

**Merged to main:** PR #117 via merge commit `176461729dc2200d458eefad70ccdc126a2041b3`
**Migration:** none (000072 not created)
**Scope:** E6 buyer Studio frontend + controller remediation E6-001…E6-012B (no E7, no v3.0F, no staging/pilot)

---

## 1. Controller acceptance record

| Field | Value |
|---|---|
| `STATUS` | IMPLEMENTED_ACCEPTED |
| `CONTROLLER_ACCEPTANCE` | YES |
| `PR117_MERGED` | YES |
| `PR117_HEAD` | `60d5fd28fb4601814a44ff4b7fb8d815b1635130` |
| `PR117_MERGE_SHA` | `176461729dc2200d458eefad70ccdc126a2041b3` |
| `CI_RUN_ID` | 34509790393 |
| `CI_EXACT_HEAD` | YES |
| `CI_CONCLUSION` | success |
| `E6_001_E6_012B_CLOSED` | YES |
| `MIGRATION_000072_CREATED` | NO |

---

## 2. Summary

Buyer Studio frontend for RFx template library, template/event version history, compare/restore, clone-from-template UX, change-impact republish orchestration, and read-only historical version views. Includes controller remediation E6-001…E6-012B, PostgreSQL/HTTP integration acceptance matrix (E6-INT-01..25, E6-EV-01..04, E6-PG-01..03), API Gateway identity spoof guard, and web-admin unit/component/route/middleware tests.

E6 delivers **Studio UI and acceptance evidence only**. Late submission, Excel import/export, ERP/TMS integration, and user training are **not** implemented in E6.

---

## 3. Screens

| Screen | Route | Backend |
|--------|-------|---------|
| Template library | `/rfx/templates` | `GET/POST /api/v1/rfx-templates` |
| Template editor | `/rfx/templates/:id` | Template metadata + DRAFT questionnaire graph |
| Template version history | `/rfx/templates/:id/versions` | `GET /api/v1/rfx-templates/:id` |
| Template version detail | `/rfx/templates/:id/versions/:versionId` | metadata + historical questionnaire graph (read-only) |
| Event version history | `/rfx/:id/versions` | `GET /api/v1/rfx-events/:id/versions` |
| Event version detail | `/rfx/:id/versions/:versionId` | `GET /api/v1/rfx-events/:id/versions/:versionId` |
| Version compare | `/rfx/:id/versions/compare` | `POST …/versions/compare` |
| Restore as draft (dialog) | from version history | `POST …/restore-draft` + stable `Idempotency-Key` |
| Create RFx from template | modal on library | lazy `GET /api/v1/rfx-templates/:id` + `POST /api/v1/rfx-events/from-template` |
| Studio validation/publish | `/rfx/:id/studio?step=validation` | validate-publish, change-impact preview, questionnaire publish |

---

## 4. Remediation markers (E6-001…E6-012B)

| Item | Status |
|------|--------|
| E6-001 clone version loading | PASS |
| E6-002 change impact republish | PASS |
| E6-003 template fork gating | PASS |
| E6-004 event version read-only | PASS |
| E6-004 template version read-only graph | PASS |
| E6-005 server readiness | PASS |
| E6-006 idempotent retry | PASS |
| E6-007 provenance from event GET | PASS |
| E6-008 mounted component + integration tests | PASS |
| E6-009 server-authoritative publish readiness | PASS |
| E6-010 fail-closed version history | PASS |
| E6-011 change-impact recovery | PASS |
| E6-012 safe impact-confirm republish gates | PASS |
| E6-012B post-preview draft freshness revalidation | PASS |

---

## 5. Feature flag

| Flag | Location | Default |
|------|----------|---------|
| `NUXT_PUBLIC_RFX_VERSIONING_V3_ENABLED` | `apps/web-admin/nuxt.config.ts` | `false` |

When off: buyer template routes redirect to `/rfx`; Studio version-history nav and publish panel hidden.

---

## 6. Integration tests (`templatelibrary`)

Package: `services/rfx-service/internal/integration/templatelibrary/`
CI job: `rfx-version-lifecycle-v3-integration` with `REQUIRE_TEST_DATABASE=1`

| ID | Test | Layer |
|----|------|-------|
| E6-INT-01..25 | Historical graph, provenance, tenant isolation, OpenAPI/router parity | PG / HTTP |
| E6-EV-01..04 | No-write GET evidence, deterministic repeat, provenance lifecycle | PG / HTTP |
| E6-PG-01..03 | Archived template semantics, provenance immutability, DB guard | PG / HTTP |

API Gateway: `TestE6GatewayIdentityHeaderSpoofDeniedHistoricalGraphAndProvenance` — carrier/finance denied; buyer JWT headers stripped.

All suites passed on PR #117 head `60d5fd28fb4601814a44ff4b7fb8d815b1635130` (CI run `34509790393`, 48/48 checks).

---

## 7. Frontend test matrix (`web-admin`)

| Suite | Cases |
|-------|-------|
| `rfxE6ComponentAcceptance.test.ts` | E6-CMP-01..23 (23) |
| `rfxE6Remediation.test.ts` | E6-REM-UT-* (15 behavioral) |
| `rfxTemplatesVersioningE6.test.ts` | E6-UT-* (12) |
| `rfxE6RouteGuards.test.ts` | E6-RG-01..04 (4) |
| `rfxE6MiddlewareExecution.test.ts` | E6-MW-01..06 (6) |
| **Total web-admin E6** | **60** |

Browser E2E (E7): **not started**.

---

## 8. Post-merge closeout

- E6 Studio frontend is **accepted and merged to `main`** at `176461729dc2200d458eefad70ccdc126a2041b3` (PR #117 head `60d5fd28fb4601814a44ff4b7fb8d815b1635130`, CI `34509790393`).
- Migration **000072** was not created.
- E6-001…E6-012B, E6-INT-01..25, E6-EV-01..04, E6-PG-01..03, and web-admin E6 suites passed on exact HEAD.
- Staging and pilot were **not** changed as part of E6.
- **E7** (browser acceptance) has **not** started; separate controller authorization is required before E7.
- Complete v3.0E remains **IMPLEMENTATION_IN_PROGRESS** until E7 is accepted.

---

## 9. Out of scope (E6 / E7+)

- Browser acceptance gate (E7)
- v3.0F qualification pool
- Late submission backend/UI
- Excel import/export
- ERP/TMS integration
- User training course (RU/EN/ZH)
- Staging/pilot rollout

**E7:** NOT_STARTED

---

## 10. Mandatory future gates (preserved)

- `LATE_SUBMISSION_AND_DEADLINE_EXCEPTIONS=REQUIRED`
- `BUYER_RFQ_EXCEL_IMPORT=REQUIRED_FUTURE_GATE`
- `BUYER_RFQ_ERP_INTEGRATION=REQUIRED_FUTURE_GATE`
- `CARRIER_OFFER_EXCEL_EXPORT_IMPORT=REQUIRED_FUTURE_GATE`
- `USER_TRAINING_COURSE_RU_EN_ZH=REQUIRED_BEFORE_PILOT`
