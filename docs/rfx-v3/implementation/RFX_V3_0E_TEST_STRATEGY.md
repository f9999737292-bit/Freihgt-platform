# RFx v3.0E — Templates + Versioning Test Strategy

**Status:** `TEST_STRATEGY_FROZEN_PENDING_CONTROLLER_ACCEPTANCE`  
**Mode:** `DISCOVERY_AND_ARCHITECTURE_FREEZE_ONLY`  
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
| T-INT-02 | Publish template | PUBLISHED immutable; second publish → 409 |
| T-INT-03 | Cross-tenant get template | 404 |
| T-INT-04 | Archive template | Cannot clone from archived |
| T-INT-05 | Duplicate template_code | 409 |

### 3.2 RFx versioning

| ID | Scenario | Assert |
|---|---|---|
| V-INT-01 | Publish questionnaire v1 | Status PUBLISHED; sections readable |
| V-INT-02 | Fork draft from published v1 → edit → publish v2 | v1 SUPERSEDED; v2 PUBLISHED |
| V-INT-03 | Mutate published version graph | 409 Conflict |
| V-INT-04 | Compare v1 vs v2 | Correct diff counts |
| V-INT-05 | Restore v1 as new draft v3 | New DRAFT; v1/v2 unchanged |
| V-INT-06 | Restore while draft exists | 409 without force |
| V-INT-07 | Optimistic concurrency on draft | Stale version → 409 |

### 3.3 Change impact

| ID | Scenario | Assert |
|---|---|---|
| I-INT-01 | Typo label only | NON_MATERIAL |
| I-INT-02 | Add required question, no responses | MATERIAL_NO_RESPONSES |
| I-INT-03 | Add question with draft response | MATERIAL_WITH_DRAFT_RESPONSES |
| I-INT-04 | Knockout rule change with submitted response | KNOCKOUT_AFFECTING; RESCORING_REQUIRED flag |
| I-INT-05 | Publish without confirmation when required | 422 |

### 3.4 v3.0D compatibility

| ID | Scenario | Assert |
|---|---|---|
| D-INT-01 | Submitted response pins v1; publish v2 | Response still on v1 |
| D-INT-02 | Score history for v1 response after v2 publish | qualification rows unchanged |
| D-INT-03 | New response after v2 publish | Pins v2; scores use v2 model |
| D-INT-04 | No silent re-score of v1 submission | Row count stable |

### 3.5 Clone from template

| ID | Scenario | Assert |
|---|---|---|
| C-INT-01 | Clone event from published template | Event + draft version + graph |
| C-INT-02 | Provenance FK set | `source_template_version_id` |
| C-INT-03 | Carrier non-member cannot clone | 403 |

### 3.6 Security

| ID | Scenario | Assert |
|---|---|---|
| S-INT-01 | Cross-tenant compare | 404 |
| S-INT-02 | Carrier template list | 403 |
| S-INT-03 | tenant_id query spoof | 403 (gateway/service) |

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
| 8 | Restore v1 as new draft | New draft; v1 answers unchanged on old responses |
| 9 | Submitted response immutability | Carrier submission on v1; after v2 publish, v1 score/answers visible unchanged |
| 10 | Score history preserved | v3 score column shows v1 results for old response |
| 11 | Cross-tenant denial | Buyer B cannot open Buyer A template |
| 12 | Concurrency conflict | Two tabs edit draft → conflict message |
| 13 | RU/EN/ZH | Template name and Studio labels render in each locale |
| 14 | Legacy evaluation/award regression | web-procurement evaluation page loads; award flow unchanged |

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
| Change impact | I-INT-01–05 |
| v3.0D compatibility | D-INT-01–04, Browser 9–10 |
| Tenant isolation | S-INT-01–03, Browser 11 |
| Concurrency | V-INT-07, Browser 12 |
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

**NEXT_ACTION:** Controller review (`CONTROLLER_REVIEW_V3_0E_ARCHITECTURE`)
