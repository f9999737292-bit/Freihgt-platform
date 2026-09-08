# RFx v3.0E3 — Change Impact Analysis

**Status:** IMPLEMENTED_ACCEPTED

**Merged to main:** PR #111 via merge commit `69eacb3c1f1e9cea5e9fd735f40d1b256bc33587`
**Normative:** [RFX_V3_0E_ARCHITECTURE_FREEZE.md](./RFX_V3_0E_ARCHITECTURE_FREEZE.md) §3.3, §6, §8–§14

---

## 1. Controller acceptance record

| Field | Value |
|---|---|
| `E3_STATUS` | IMPLEMENTED_ACCEPTED |
| `CONTROLLER_ACCEPTANCE` | YES |
| `PR111_MERGED` | YES |
| `PR111_HEAD` | `d9aa539ae1288ddbcdfd4427d595b353a1db0c57` |
| `PR111_MERGE_SHA` | `69eacb3c1f1e9cea5e9fd735f40d1b256bc33587` |
| `CI_RUN_ID` | 34259692961 |
| `CI_EXACT_HEAD` | YES |
| `CI_CONCLUSION` | success |
| `E3_001_E3_006_CLOSED` | YES |
| `MIGRATION_000069_ON_MAIN` | YES |
| `MIGRATION_000070_CREATED` | NO |

---

## 2. Scope

E3 adds server-side change-impact preview and publish confirmation for questionnaire republish:

- Immutable persisted analyses (`rfx.rfx_change_impact_analyses`)
- Six deterministic impact classes using E2 canonical diff (no second diff engine)
- `POST /api/v1/rfx-events/{id}/change-impact/preview` (BuyerManage)
- Publish confirmation fields: `impact_analysis_id`, `canonical_diff_hash`
- `rescoring_required` on newly published version when scoring/knockout affecting
- Audit: `rfx.change_impact.previewed.v1`, `rfx.change_impact.confirmed.v1`

**Out of scope:** frontend, auto re-score, template library, late submission, Excel, E4–E7.

Preview and publish-confirmation routes are registered under the existing v3 versioning route group and are gated by `RFX_VERSIONING_V3_ENABLED` (default **false** in rfx-service config).

---

## 3. Migration 000069

| File | Purpose |
|---|---|
| `infrastructure/migrations/000069_rfx_change_impact_v3_0e3.up.sql` | `rfx_change_impact_analyses` table, indexes, composite FKs, enum CHECK |
| `infrastructure/migrations/000069_rfx_change_impact_v3_0e3.down.sql` | Schema-qualified symmetric drop |

`rfx_versions.rescoring_required` already exists from migration 000068 (E1).

Repository max migration contract updated to **000069**; **000070** is unreleased.

`uq_rfx_events_tenant_id` is created only by migration 000069 (composite FK support for E3).

---

## 4. Impact classes

| Class | Trigger |
|---|---|
| `NON_MATERIAL` | Label/help/reorder-only |
| `MATERIAL_NO_RESPONSES` | Structural change, zero affected responses |
| `MATERIAL_WITH_DRAFT_RESPONSES` | Structural change affecting draft answers |
| `MATERIAL_WITH_SUBMITTED_RESPONSES` | Structural change with submitted responses |
| `SCORING_AFFECTING` | Criteria/bindings/normalization change |
| `KNOCKOUT_AFFECTING` | Knockout rule content change (not scoring-only binding add/remove) |

Multiple classes per analysis; output order is fixed (§6.1 enum order).  
Repository canonicalizes duplicates before INSERT; PostgreSQL `<@` containment CHECK rejects unknown classes (no subqueries).

---

## 5. API

### Preview

`POST /api/v1/rfx-events/{id}/change-impact/preview`

Request: `{ "candidate_version_id": "uuid" }`  
Response: **200** with `impact_analysis_id`, diff hash, classes, counts, flags, `expires_at` (TTL 15m).

Errors: **400** malformed, **403** carrier/unauthorized buyer, **404** cross-tenant/unknown, **409** non-active draft.

Preview persistence and preview audit run in a single database transaction.

### Publish confirmation

When a prior **PUBLISHED** version exists, republish **always** requires:

```json
{
  "expected_event_version": 1,
  "expected_draft_version": 2,
  "change_summary": "...",
  "impact_analysis_id": "uuid",
  "canonical_diff_hash": "sha256"
}
```

Plus required `Idempotency-Key`.

- First publish (no prior PUBLISHED) does **not** require confirmation.
- `MATERIAL_NO_RESPONSES` republish still requires a valid, unconsumed analysis.
- Missing confirmation on republish → **422** `CHANGE_IMPACT_ANALYSIS_REQUIRED` (not gated on response/score counts).

### Delegate confirmation (E3-004)

Any verified buyer-manage user of the owner company may confirm publish.  
Confirmation audit stores `preview_actor_id` and `confirming_actor_id`; analysis `actor_id` remains the preview author.

HTTP semantics: **409** stale diff/consumed analysis/idempotency mismatch; **422** missing confirmation or expired analysis.

---

## 6. Idempotency and consumption

- Same `Idempotency-Key` + same body → stored response (including after `consumed_at`)
- Consumed analysis + new key → **409**
- `consumed_at` set in same transaction as successful publish
- Publish body hash includes all confirmation fields + `change_summary`

---

## 7. Response and score immutability

- Existing responses keep pinned `rfx_version_id`
- Submitted responses and qualification history unchanged
- No automatic re-score; `rescoring_required` is flag-only on new PUBLISHED version
- Analysis rows are immutable except `consumed_at`

---

## 8. Affected response counting (E3-003)

- Added required question → counts all DRAFT/SUBMITTED responses on source version (even without answer on new question)
- Removed/changed question → counts responses with answers on affected question codes
- Questionnaire/section-wide structural changes → counts matching responses on source version
- Each response counted once; DRAFT and SUBMITTED tallied separately
- Optional unfilled question label-only change → zero affected counts (`NON_MATERIAL`)

---

## 9. Tests

| Suite | Coverage |
|---|---|
| `change_impact_integration_test.go` | E3-INT-01..34 |
| `change_impact_remediation_integration_test.go` | E3-REM-001..016 (controller remediation) |
| `change_impact_test.go` | Domain unit tests (scoring/knockout, response scope) |

Regression: E1-INT-01..33, E2-INT-01..29 (republish helpers updated for mandatory confirmation).

PostgreSQL 16 with `REQUIRE_TEST_DATABASE=1` in CI.

---

## 10. Controller remediation map (E3-001..006)

| ID | Fix | Verification |
|---|---|---|
| **E3-001** | Republish always requires impact confirmation when prior PUBLISHED exists | E3-REM-001..004, E1-INT-05/06/08, idempotency republish helpers |
| **E3-002** | KNOCKOUT_AFFECTING only when knockout rule content changes | E3-REM-005..006, domain unit tests |
| **E3-003** | Correct affected response counts (added required, distinct, multi-carrier) | E3-REM-007..011 |
| **E3-004** | Buyer-manage delegate can confirm; unauthorized same-tenant → 403 | E3-REM-012..013 |
| **E3-005** | Migration 000069 CHECK for six-class JSON array enum | E3-REM-014..015 |
| **E3-006** | Schema-qualified down migration; `search_path=public` down test | E3-REM-016 |

---

## 11. OpenAPI

Generator profiles: `vl_change_impact_preview`, extended `RfxPublishQuestionnaireRequest`.  
Regenerate via `python scripts/openapi/generate_openapi.py`.

---

## 12. Post-merge closeout

- E3 change-impact backend is **accepted and merged to `main`** at `69eacb3c1f1e9cea5e9fd735f40d1b256bc33587` (PR #111 head `d9aa539ae1288ddbcdfd4427d595b353a1db0c57`, CI `34259692961`).
- Versioning v3 routes (including change-impact preview and publish confirmation) remain behind `RFX_VERSIONING_V3_ENABLED=false` by default in rfx-service config.
- Staging and pilot were **not** changed as part of E3.
- **E4** (template library) has **not** started; separate controller authorization is required before implementation.
- Complete v3.0E remains **IMPLEMENTATION_IN_PROGRESS** until E4–E7 are accepted.

---

## 13. Out of scope (E3)

- E4–E5 template library / clone provenance
- E6 Studio frontend (version history, compare, restore, change-impact UI)
- E7 browser acceptance
- Late-submission implementation
- Automatic re-scoring
- Excel import/export
- Staging/pilot rollout

**E4:** PENDING_CONTROLLER_AUTHORIZATION
**E5–E7:** NOT_STARTED
