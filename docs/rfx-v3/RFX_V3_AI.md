# RFx v3.0A — AI Architecture

**Status:** Architecture draft  
**Current repository state:** AI integration **ABSENT** in RFx paths (see [RFX_V3_GAP_MATRIX.md](./RFX_V3_GAP_MATRIX.md))

---

## 1. Purpose

Define bounded AI assistance for Enterprise RFx v3.0A. AI augments human decision-making — never autonomously changes authoritative tender or qualification state.

---

## 2. Allowed capabilities

| Capability | Description | Human gate |
|---|---|---|
| Questionnaire draft generation | Suggest sections/questions from template + lane context | Buyer review before publish |
| Question suggestions | Recommend additional questions based on category | Buyer accepts/rejects |
| Template recommendations | Match historical tenders to new RFx | Buyer selects |
| Document extraction | Parse uploaded cert/insurance into field candidates | Carrier/buyer confirmation required |
| Anomaly detection | Flag inconsistent answers vs Carrier 360 | Buyer review queue |
| Response summary | Summarize carrier response for evaluator | Read-only assist |
| Qualification explanation | Natural language explanation of score/knockout | Derived from deterministic scoring |

---

## 3. Forbidden autonomous actions

```text
AI_AUTO_PUBLISH=NO
AI_AUTO_INVITE=NO
AI_SILENT_SCORE_CHANGE=NO
AI_AUTO_REJECT_WITHOUT_DETERMINISTIC_AUTHORITY=NO
```

| Forbidden action | Reason |
|---|---|
| Auto-publish RFx | Publish gate requires human approval + validation |
| Auto-invite carriers | Participant list is buyer-controlled |
| Silent score change | Scores must be explainable and versioned |
| Auto-reject without rule | Knockout requires deterministic rule + valid persisted answer |

---

## 4. AI-derived value metadata

Every AI-produced value stored or suggested must carry:

| Field | Required |
|---|---|
| `SOURCE` | `AI_EXTRACTED_PENDING_REVIEW` until confirmed |
| `MODEL_VERSION` | Model or process identifier |
| `PROCESS_VERSION` | Pipeline version |
| `TIMESTAMP` | Generation time |
| `CONFIDENCE` | 0–1 score |
| `REVIEW_STATE` | `PENDING`, `CONFIRMED`, `REJECTED` |

Confirmed values promote to appropriate authoritative `answer_source` (e.g. `DOCUMENT_VERIFIED` after human confirms extraction).

Pending AI values **must not** trigger knockout or final qualification without review.

---

## 5. Integration points

| Integration | Pattern |
|---|---|
| Document extraction | Async job → candidate fields → carrier confirm UI |
| Question generation | Sync API → draft JSON → buyer editor |
| Anomaly detection | Post-save hook → `WARNING` item (non-blocking) |
| Explanation | Read scoring `explanation_json` → LLM narrative (no score mutation) |

All AI service calls are tenant-scoped and audited. No cross-tenant training on tenant data without explicit contract.

---

## 6. Security & tenancy

- AI requests include server-verified `tenant_id` and `user_id` only.
- Document content sent to AI must respect document RBAC.
- AI outputs logged with model version; no secrets in prompts.
- Preview sandbox AI calls tagged `PREVIEW_DATA_ONLY=YES`.

---

## 7. References

- [RFX_V3_SCORING_ENGINE.md](./RFX_V3_SCORING_ENGINE.md) — explainability
- [RFX_V3_CARRIER_360.md](./RFX_V3_CARRIER_360.md) — autofill extraction
- [RFX_V3_SECURITY.md](./RFX_V3_SECURITY.md)
- [ADR-RFX-010](./adr/ADR-RFX-010-AI-SAFETY-BOUNDARY.md)
