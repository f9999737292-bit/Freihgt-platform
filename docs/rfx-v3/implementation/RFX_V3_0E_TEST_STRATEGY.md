# RFx v3.0E — Templates + Versioning Test Strategy

**Status:** `TEST_STRATEGY_FROZEN_PENDING_CONTROLLER_ACCEPTANCE`  
**Mode:** `DISCOVERY_AND_ARCHITECTURE_FREEZE_ONLY` — remediation PR #106
**Implementation:** **NOT authorized**  
**Base:** `origin/main` @ `0db472d6a4b79d987cb98f28253383fd4a88821d`

---

## 1. Test pyramid

| Layer | Scope | Gate |
|---|---|---|
| L0 | Static: diff, OpenAPI parity script, bash syntax | CI `scripts-check`, `repository-safety` |
| L1 | Domain unit: diff classifier, impact class resolver, version FSM | New Go packages under `rfx-service/internal/domain/` |
| L2 | Integration PostgreSQL 16: publish, compare, restore, template clone, tenant deny | New CI job `rfx-templates-versioning-v3-integration` |
| L3 | Browser E2E: Studio template library, version history, compare, restore | New CI job `rfx-templates-versioning-v3-browser-e2e` |
| Regression | v3.0B/C/D existing jobs | Must remain PASS on every PR |

Fail-closed: `REQUIRE_TEST_DATABASE=1` for integration; no mocks for browser acceptance chain (match v3.0D pattern).

---

## 2. Domain unit tests (L1)

| Package | Cases |
|---|---|
| `version_diff` | ADDED/REMOVED/CHANGED/REORDERED/UNCHANGED for each entity type |
| `change_impact` | Classify NON_MATERIAL → KNOCKOUT_AFFECTING |
| `version_fsm` | Illegal DRAFT→DRAFT publish; PUBLISHED mutation deny |
| `restore_policy` | Restore increments version_number; never demotes PUBLISHED |

---

## 3. Integration tests (L2)

**Job name:** `rfx-templates-versioning-v3-integration`  
**PostgreSQL:** 16  
**Fixtures:** Reuse `scoringv3` / `questionnaire` test helpers pattern.

### 3.1 Template library

| ID | Scenario | Assert |
|---|---|---|
| T-INT-01 | Create template DRAFT | Row exists, tenant scoped |
| T-INT-02 | Publish template version | PUBLISHED immutable; prior PUBLISHED → SUPERSEDED; second concurrent publish → 409 |
| T-INT-03 | Cross-tenant get template | **404** |
| T-INT-04 | Archive template aggregate | Normal clone **409**; unarchive **deferred** |
| T-INT-05 | Duplicate template_code | 409 |
| T-INT-06 | Concurrent template draft creation | One winner; loser **409** |
| T-INT-07 | Clone from SUPERSEDED template version | Allowed with explicit action; warning flag in response |

### 3.2 RFx versioning

| ID | Scenario | Assert |
|---|---|---|
| V-INT-01 | Publish questionnaire v1 | Status PUBLISHED; sections readable |
| V-INT-02 | Fork draft from published v1 → edit → publish v2 | v1 SUPERSEDED; v2 PUBLISHED |
| V-INT-03 | Mutate published version graph | 409 Conflict |
| V-INT-04 | Compare v1 vs v2 | Correct diff counts |
| V-INT-05 | Restore v1 as new draft v3 | New DRAFT; v1/v2 unchanged |
| V-INT-06 | Restore while draft exists | **409** (no `force`) |
| V-INT-07 | Optimistic concurrency on draft | Stale version → 409 |
| V-INT-08 | Concurrent event draft fork | One winner; loser **409** |
| V-INT-09 | Publish TX clears `draft_version_id` | No implicit new DRAFT in publish |

### 3.3 Change impact

| ID | Scenario | Assert |
|---|---|---|
| I-INT-01 | Typo label only | NON_MATERIAL |
| I-INT-02 | Add required question, no responses | MATERIAL_NO_RESPONSES |
| I-INT-03 | Add question with draft response | MATERIAL_WITH_DRAFT_RESPONSES |
| I-INT-04 | Knockout rule change with submitted response | KNOCKOUT_AFFECTING; `rescoring_required=TRUE` on new published version |
| I-INT-05 | Publish without confirmation when required | **422** |
| I-INT-06 | Stale impact confirmation after draft edit | **409** |
| I-INT-07 | Expired impact confirmation | **422** |
| I-INT-08 | Impact analysis for different event (same tenant) | **404** |
| I-INT-09 | Row consumed; reuse with new/missing Idempotency-Key | **409**; original matching idempotent replay returns stored response |

### 3.4 v3.0D compatibility and carrier continuity (mandatory)

| ID | Scenario | Assert |
|---|---|---|
| D-INT-01 | Submitted response pins v1; publish v2 | Response still on v1 |
| D-INT-02 | Score history for v1 response after v2 publish | qualification rows unchanged |
| D-INT-03 | New response after v2 publish | Pins v2; scores use v2 model |
| D-INT-04 | No silent re-score of v1 submission | Row count stable |
| D-INT-05 | **Old draft response saveable after v2 publish** | PATCH autosave **200** against pinned v1 |
| D-INT-06 | **Old draft response submittable against pinned v1** | Submit **200**; not 409 on version drift |
| D-INT-07 | **New response pins newest PUBLISHED** | `rfx_version_id = v2.id` |
| D-INT-08 | **No silent re-score after scoring-affecting change** | `rescoring_required=TRUE` on v2; v1 score rows unchanged |
| D-INT-09 | **Attempt to update response.rfx_version_id after creation** | Denied; original pin preserved |

### 3.5 Clone from template

| ID | Scenario | Assert |
|---|---|---|
| C-INT-01 | Clone event from published template | Event + draft version + graph |
| C-INT-02 | Provenance FK set | `source_template_version_id` |
| C-INT-03 | Carrier non-member cannot clone | 403 |

### 3.6 Idempotency

| ID | Scenario | Assert |
|---|---|---|
| ID-INT-01 | Retry publish with same Idempotency-Key + body | Same response; single version row |
| ID-INT-02 | Same key with different payload | **409** |
| ID-INT-03 | Retry restore-as-draft after timeout | Same draft version returned |
| ID-INT-04 | Retry clone-from-template | Same event id returned |
| ID-INT-05 | Successful publish replay with same key/body after consumed analysis | Original stored response; no duplicate publish |

### 3.7 Restore safety

| ID | Scenario | Assert |
|---|---|---|
| R-INT-01 | Existing draft blocks restore | **409** |
| R-INT-02 | Restore never deletes draft | Draft row count unchanged on 409 |
| R-INT-03 | SUPERSEDED source remains comparable | Compare API returns diff |

### 3.8 Security (error semantics)

| ID | Scenario | Assert |
|---|---|---|
| S-INT-01 | Cross-tenant compare | **404** |
| S-INT-02 | Carrier template list | **403** |
| S-INT-03 | Same-tenant buyer non-owner restore | **403** |
| S-INT-04 | tenant_id query spoof | **403** (gateway/service) |

---

## 4. Browser acceptance (L3)

**Job name:** `rfx-templates-versioning-v3-browser-e2e`  
**Chain:** Chromium → web-admin → api-gateway → rfx-service → PostgreSQL 16  
**No API mocks** for template/version flows.

### 4.1 Scenarios (mandatory)

| # | Scenario | Key assertions |
|---|---|---|
| 1 | Create template | Template appears in library |
| 2 | Publish template | Status PUBLISHED; edit disabled |
| 3 | Create RFx from template | Event created; questionnaire populated |
| 4 | Edit event draft | Autosave works |
| 5 | Publish version 1 | Readiness pass; published locked |
| 6 | Create new draft (fork) | Draft v2 editable |
| 7 | Compare v1 vs v2 | Diff shows expected changes |
| 8 | Restore v1 as new draft | New draft; **409** if draft already exists |
| 9 | Submitted response immutability | Carrier submission on v1; after v2 publish, v1 answers/scores unchanged |
| 10 | Score history preserved | v1 results visible for old response |
| 11 | Cross-tenant denial | Buyer B → **404** on Buyer A template |
| 12 | Concurrency conflict | Two tabs edit draft → conflict message |
| 13 | RU/EN/ZH | Template name and Studio labels in each locale |
| 14 | Legacy evaluation/award regression | web-procurement evaluation page loads |
| 15 | **Draft response save after new publish** | Carrier continues editing on pinned v1 |
| 16 | **Draft response submit after new publish** | Submit succeeds on pinned v1 |
| 17 | **Template archive blocks clone** | Clone from ARCHIVED template → blocked |
| 18 | **SUPERSEDED template version compare** | History compare still works |

### 4.2 Harness location (implementation phase)

```
apps/web-admin/e2e/rfx-templates-versioning-v3/
services/rfx-service/internal/integration/templatesv3/
```

Pattern: mirror `rfx-scoring-v3` browser harness from v3.0D.

---

## 5. Frontend unit tests (L1)

| App | Tests |
|---|---|
| web-admin | Template list helpers, diff summary formatter, restore dialog state machine |
| web-admin | Studio nav includes Templates + Version History when feature flag on |

Scope: vitest only; no new typecheck debt in unrelated modules.

---

## 6. CI integration plan (proposal)

Add to `.github/workflows/ci.yml` when implementation authorized:

```yaml
rfx-templates-versioning-v3-integration:
  services: postgres:16
  run: go test -tags=integration ./services/rfx-service/internal/integration/templatesv3/...

rfx-templates-versioning-v3-browser-e2e:
  services: postgres:16
  run: playwright in apps/web-admin/e2e/rfx-templates-versioning-v3/
```

Existing gates (`rfx-scoring-v3-integration`, `rfx-scoring-v3-browser-e2e`, `rfx-questionnaire-v3-integration`, etc.) remain required on every PR.

---

## 7. Acceptance criteria mapping

| Architecture capability | Primary test IDs |
|---|---|
| Template library | T-INT-01–05, Browser 1–3 |
| Immutable published version | V-INT-01–03, Browser 5 |
| Compare | V-INT-04, Browser 7 |
| Restore as draft | V-INT-05–06, Browser 8 |
| Change impact | I-INT-01–09 |
| v3.0D compatibility + carrier continuity | D-INT-01–09, Browser 9–10, 15–16 |
| Tenant isolation + error semantics | S-INT-01–04, Browser 11 |
| Concurrency | V-INT-07–08, T-INT-06, Browser 12 |
| Idempotency | ID-INT-01–05, I-INT-09 |
| Restore safety | R-INT-01–03, V-INT-06, Browser 8 |
| Template archive / SUPERSEDED | T-INT-04–07, Browser 17–18 |
| i18n | Browser 13 |
| Legacy regression | Browser 14 |

---

## 8. Evidence requirements (controller gate)

Before v3.0E implementation acceptance:

- [ ] All L2 integration IDs PASS on exact PR head SHA
- [ ] All L3 browser scenarios PASS on exact PR head SHA
- [ ] v3.0D regression jobs PASS unchanged
- [ ] No migration 000068 applied on staging without operator authorization

---

**NEXT_ACTION:** `CONTROLLER_ACCEPTANCE_V3_0E_ARCHITECTURE`
