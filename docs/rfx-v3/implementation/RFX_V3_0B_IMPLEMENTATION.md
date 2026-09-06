# RFx v3.0B — Implementation Evidence

**Status:** `v3.0B=IMPLEMENTED_PENDING_CONTROLLER_ACCEPTANCE`  
**Worktree:** `D:\Projects\freight-platform-wt\rfx-questionnaire-core-v3.0b`  
**Branch:** `feat/bintrans-enterprise-rfx-questionnaire-core-v3.0b`  
**Scope:** Buyer RFx Studio questionnaire builder (Wave 1 types), draft versioning, publish readiness gate

---

## 1. Delivery summary

| Lane | Deliverable | Evidence |
|------|-------------|----------|
| Backend | Migration `000065`, questionnaire service + HTTP handlers | `services/rfx-service/internal/integration/questionnaire/` |
| OpenAPI | Studio/questionnaire routes in `rfx-service.yaml` + aggregate | `packages/openapi/rfx-service.yaml` |
| Frontend | Studio shell, builder, preview, autosave composable | `apps/web-admin/pages/rfx/[id]/studio/` |
| Tests | Vitest unit/parity + Go studio API E2E | See §5 |
| CI | `rfx-studio-browser-e2e` job | `.github/workflows/ci.yml` |

---

## 2. Architecture deviations (v3.0B)

### 2.1 `questionnaire_enabled` on `rfx_versions`

- Column added by migration `000065`; defaults **`FALSE`** for legacy compatibility.
- Buyer must enable questionnaire on the draft version before publish-readiness checks apply to questionnaire content.
- Frontend surfaces `questionnaire_enabled` on `RfxVersionRecord` / `RfxQuestionnaireDefinition`.
- Legacy RFx events without questionnaire remain unaffected.

### 2.2 HTTP 400 validation (draft safety)

- Invalid mutations return **HTTP 400** with structured `ErrorResponse`; failed writes are **not persisted**.
- Frontend autosave maps 400 → `invalid` state; **saved indicator is suppressed** while invalid.
- Normative contract: [RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md](../RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md).

### 2.3 No option reorder (v3.0B)

- OpenAPI has **no** `POST .../options/reorder` route.
- Frontend constant: `OPTION_REORDER_UI = 'NOT_AVAILABLE_V3_0B'` in `types/rfx-questionnaire.ts`.
- Option order is set at create time via `sort_order` only; UI must not call a reorder endpoint.

### 2.4 Other intentional limits

- Wave 1 question types only in builder UI (`WAVE1_QUESTION_TYPES`); extended types marked coming next.
- Publish remains separate from validate: `validate-publish` is readiness gate only; event publish unchanged.

---

## 3. Frontend architecture

```
pages/rfx/[id]/studio/index.vue
  └── RfxStudioShell + RfxStudioHeader (autosave)
  └── RfxQuestionnaireBuilder
  └── RfxPublishReadinessPanel

composables/useRfxQuestionnaireApi.ts  — API + debounced autosave
utils/rfxQuestionnaireApiRoutes.ts     — FRONTEND_OPENAPI_PARITY matrix
utils/rfxStudioQuestionnaire.ts        — pure autosave/readiness/preview helpers
```

### Autosave state machine

| State | UX | Blocks saved badge |
|-------|-----|-------------------|
| `idle` | Initial / reloaded | — |
| `dirty` | Pending debounced patch | — |
| `saving` | In-flight request | — |
| `saved` | Last write OK | — |
| `invalid` | HTTP 400 validation | **Yes** |
| `conflict` | HTTP 409 version conflict | **Yes** |
| `save_failed` | Other errors | — |

---

## 4. API surface (frontend parity)

All routes in `FRONTEND_OPENAPI_PARITY` (`apps/web-admin/utils/rfxQuestionnaireApiRoutes.ts`) are verified against `packages/openapi/rfx-service.yaml` in vitest.

| Method | Path |
|--------|------|
| GET | `/api/v1/rfx-events/{id}/studio` |
| GET | `/api/v1/rfx-events/{id}/questionnaire` |
| POST | `/api/v1/rfx-events/{id}/save-draft` |
| POST | `/api/v1/rfx-events/{id}/validate-publish` |
| POST/PATCH/DELETE | sections, questions, options, rules (+ reorder for sections/questions) |

**Excluded v3.0B:** option reorder endpoint.

---

## 5. Test evidence

### 5.1 Vitest (web-admin)

**File:** `apps/web-admin/tests/rfxStudioQuestionnaire.test.ts`

Covers:

- `resolveAutosaveLabel` / autosave helpers
- `FRONTEND_OPENAPI_PARITY` vs OpenAPI YAML
- Autosave states including **invalid never shows saved**
- Readiness render helpers
- Preview render helpers

**Command:**

```bash
cd apps/web-admin
npm test -- tests/rfxStudioQuestionnaire.test.ts
```

### 5.2 Go studio API E2E

**Package:** `services/rfx-service/internal/integration/studio/`

**Tests:**

- `TestStudioQuestionnaireAPIFlow_E2E` — studio → sections → questions → rules → save-draft → validate-publish
- `TestStudioQuestionnaireValidationHTTP400` — invalid create returns 400

**Command:**

```bash
cd services/rfx-service
export TEST_DATABASE_URL='postgres://rfx_test:rfx_test@localhost:5432/freight_test?sslmode=disable'
export REQUIRE_TEST_DATABASE=1
go test -tags=integration ./internal/integration/studio/... -count=1 -run 'TestStudioQuestionnaire' -v
```

### 5.3 Browser E2E (production api-gateway path)

**Package:** `services/rfx-service/internal/integration/studio/`  
**Playwright:** `apps/web-procurement/e2e/rfx-studio/`

Live stack:

```
Chromium
  → web-admin (Nuxt dev, UI + session seed)
  → production api-gateway (cmd/server: NewRouter + NewProxyHandler + Auth + rfxrbac)
  → disposable rfx-service questionnaire handlers/service/repository
  → PostgreSQL 16 (isolated temp DB)
```

Identity for gateway RBAC uses a contract-accurate HTTP stub for `GET /v1/auth/me` (production `IdentityClient` codepath). JWT is validated by production gateway auth middleware; questionnaire API mocks are forbidden.

**Command (local, requires Postgres + Node + Playwright):**

```bash
cd services/rfx-service
export TEST_DATABASE_URL='postgres://rfx_test:rfx_test@localhost:5432/freight_test?sslmode=disable'
export REQUIRE_TEST_DATABASE=1
export BROWSER_E2E=1
go test -tags=integration ./internal/integration/studio/... -count=1 -run TestRfxStudio_BrowserE2E_LiveBuyerFlow -timeout 25m -v
```

Security proofs (same package, no browser): `TestBrowserProductionGateway_SecurityProofs`.

### 5.4 CI

Job **`rfx-studio-browser-e2e`** — postgres:16, builds production `api-gateway`, runs Playwright buyer studio acceptance through the real gateway process (fail-closed).

Related: **`rfx-questionnaire-v3-integration`** runs service-layer questionnaire tests; **`rfx-studio-api-e2e`** runs Go HTTP studio flow tests.

---

## 6. Validation levels executed

| Check | Result |
|-------|--------|
| L0 — vitest static/unit | Run locally (see §5.1) |
| L1 — Go studio E2E | Run locally when Postgres available (see §5.2) |
| L2 — full questionnaire integration suite | CI job `rfx-questionnaire-v3-integration` |
| L3 — browser Playwright via production api-gateway | CI job `rfx-studio-browser-e2e` |

---

## 7. v3.0C handoff

| Item | Notes for v3.0C |
|------|-----------------|
| Carrier response workspace | Wire questionnaire definition to response autosave (validation contract §2–4) |
| Extended question types | Enable `COMING_NEXT_WAVE_TYPES` in builder + preview |
| Option reorder | Add OpenAPI + UI if product approves; remove `OPTION_REORDER_UI` guard |
| Scoring / qualification | Consume persisted answers only; bind rule versions per engine doc |
| Publish orchestration | Optional auto-publish after readiness PASS (controller decision) |
| Browser E2E | Production gateway browser gate closed in F102-002; extend coverage in v3.0C if needed |

**Controller acceptance checklist:**

- [ ] Confirm `questionnaire_enabled` UX (enable/disable on draft version)
- [ ] Sign off HTTP 400 draft-safety behavior in studio
- [ ] Accept no option reorder for v3.0B
- [ ] Review CI green on `rfx-studio-browser-e2e` + `rfx-questionnaire-v3-integration`

---

## 8. References

- [RFX_V3_0B_DISCOVERY.md](./RFX_V3_0B_DISCOVERY.md)
- [RFX_V3_QUESTIONNAIRE_ENGINE.md](../RFX_V3_QUESTIONNAIRE_ENGINE.md)
- [ADR-RFX-002-QUESTIONNAIRE-VERSIONING.md](../adr/ADR-RFX-002-QUESTIONNAIRE-VERSIONING.md)
- Migration: `infrastructure/migrations/000065_rfx_questionnaire_v3_0b.up.sql`
