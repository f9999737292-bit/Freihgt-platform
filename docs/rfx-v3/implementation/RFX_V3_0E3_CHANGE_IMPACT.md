# RFx v3.0E3 — Change Impact Analysis

**Status:** IMPLEMENTED_PENDING_CONTROLLER_ACCEPTANCE  
**Branch:** `feat/rfx-change-impact-v3.0e3`  
**Normative:** [RFX_V3_0E_ARCHITECTURE_FREEZE.md](./RFX_V3_0E_ARCHITECTURE_FREEZE.md) §3.3, §6, §8–§14

---

## 1. Scope

E3 adds server-side change-impact preview and publish confirmation for questionnaire republish:

- Immutable persisted analyses (`rfx.rfx_change_impact_analyses`)
- Six deterministic impact classes using E2 canonical diff (no second diff engine)
- `POST /api/v1/rfx-events/{id}/change-impact/preview` (BuyerManage)
- Publish confirmation fields: `impact_analysis_id`, `canonical_diff_hash`
- `rescoring_required` on newly published version when scoring/knockout affecting
- Audit: `rfx.change_impact.previewed.v1`, `rfx.change_impact.confirmed.v1`

**Out of scope:** frontend, auto re-score, template library, late submission, Excel, E4–E7.

---

## 2. Migration 000069

| File | Purpose |
|---|---|
| `infrastructure/migrations/000069_rfx_change_impact_v3_0e3.up.sql` | `rfx_change_impact_analyses` table, indexes, composite FKs |
| `infrastructure/migrations/000069_rfx_change_impact_v3_0e3.down.sql` | Symmetric drop |

`rfx_versions.rescoring_required` already exists from migration 000068 (E1).

Repository max migration contract updated to **000069**; **000070** is unreleased.

---

## 3. Impact classes

| Class | Trigger |
|---|---|
| `NON_MATERIAL` | Label/help/reorder-only |
| `MATERIAL_NO_RESPONSES` | Structural change, zero affected responses |
| `MATERIAL_WITH_DRAFT_RESPONSES` | Structural change affecting draft answers |
| `MATERIAL_WITH_SUBMITTED_RESPONSES` | Structural change with submitted responses |
| `SCORING_AFFECTING` | Criteria/bindings/normalization change |
| `KNOCKOUT_AFFECTING` | Knockout rule change |

Multiple classes per analysis; output order is fixed (§6.1 enum order).

---

## 4. API

### Preview

`POST /api/v1/rfx-events/{id}/change-impact/preview`

Request: `{ "candidate_version_id": "uuid" }`  
Response: **200** with `impact_analysis_id`, diff hash, classes, counts, flags, `expires_at` (TTL 15m).

Errors: **400** malformed, **403** carrier/unauthorized buyer, **404** cross-tenant/unknown, **409** non-active draft.

### Publish confirmation

When prior PUBLISHED exists **and** event has responses or scored responses, publish requires:

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

Republish without responses (E1 regression) does not require confirmation.

HTTP semantics: **409** stale diff/consumed analysis/idempotency mismatch; **422** missing confirmation or expired analysis.

---

## 5. Idempotency and consumption

- Same `Idempotency-Key` + same body → stored response (including after `consumed_at`)
- Consumed analysis + new key → **409**
- `consumed_at` set in same transaction as successful publish
- Publish body hash includes all confirmation fields + `change_summary`

---

## 6. Response and score immutability

- Existing responses keep pinned `rfx_version_id`
- Submitted responses and qualification history unchanged
- No automatic re-score; `rescoring_required` is flag-only on new PUBLISHED version

---

## 7. Tests

Integration: `change_impact_integration_test.go` — **E3-INT-01..34** (PostgreSQL 16, `REQUIRE_TEST_DATABASE=1` in CI).

Regression: E1-INT-01..33, E2-INT-01..29 unchanged.

---

## 8. OpenAPI

Generator profiles: `vl_change_impact_preview`, extended `RfxPublishQuestionnaireRequest`.  
Regenerate via `python scripts/openapi/generate_openapi.py`.

---

## 9. Controller acceptance

Pending controller review on exact PR head CI.
