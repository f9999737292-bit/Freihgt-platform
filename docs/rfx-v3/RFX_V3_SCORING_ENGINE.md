# RFx v3.0A — Scoring Engine

**Status:** Architecture draft  
**Normative companions:** [RFX_V3_QUESTIONNAIRE_ENGINE.md](./RFX_V3_QUESTIONNAIRE_ENGINE.md) §4, [RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md](./RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md)

---

## 1. Purpose

Canonical scoring and qualification architecture for Enterprise RFx v3.0A. Consumes **persisted valid answers only** — never client drafts or preview data.

---

## 2. Scoring modes

| Mode | Description |
|---|---|
| `AUTOMATIC` | Deterministic rules applied to answer values |
| `MANUAL` | Buyer evaluator enters/adjusts score with audit |
| `HYBRID` | Automatic baseline + manual override within authority |
| `SYSTEM_DERIVED` | Score from BINTRANS operational KPI / Carrier 360 data |

Current v1 implementation uses fixed commercial/manual weighting (`services/rfx-service/internal/domain/rfx_evaluation.go`) — v3 replaces with configurable `score_models`.

---

## 3. Critical invariant

```
VALIDATION_ERROR ≠ BUSINESS_FAILURE
```

| Example | Classification | Persist? | Scoring? |
|---|---|---|---|
| `fleet_count = -5` | `VALIDATION_ERROR` | **NO** | Not invoked |
| `fleet_count = 15`, min qualification 35 | Valid answer | **YES** | Warning/penalty applied |
| `ADR_AVAILABLE = false` (mandatory ADR) | Valid answer + `KNOCKOUT` | **YES** | Knockout/disqualification |

Invalid values are rejected before scoring. Valid negative business answers are persisted and scored/knocked out.

`KNOCKOUT_BLOCKS_SAVE=NO` — knockout is a business outcome, not a validation failure.

---

## 4. Score model structure

| Component | Role |
|---|---|
| `ScoreModel` | Versioned model bound to `RfxVersion` |
| `ScoreCriterion` | Weighted dimension (technical, HSE, commercial, …) |
| `NormalizationRule` | Scale raw values to comparable range |
| `Threshold` | Qualification minimum per criterion or aggregate |
| `KnockoutRule` | Hard disqualification on valid answer pattern |

### 4.1 Weights

- Criterion weights sum to configurable total (typically 100%).
- Question-level bindings reference criterion + local weight modifier.
- Weight changes require new `score_model_version`.

### 4.2 Normalization

| Type | Use |
|---|---|
| Linear scale | Numeric ranges → 0–100 |
| Boolean map | true/false → fixed points |
| Option map | SELECT options → discrete scores |
| KPI percentile | Operational metrics vs tenant baseline |

---

## 5. Calculation pipeline

```
Persisted valid answers
  → Load score_model_version for RfxVersion
  → Apply criterion rules per answer
  → Aggregate weighted contributions
  → Evaluate thresholds
  → Evaluate knockout rules
  → Emit QualificationResult + AnswerScore rows
  → Publish rfx.score.calculated event
```

Recalculation triggers:

- Response submit
- Manual score override (audited)
- Material RFx version change (`RESCORING_REQUIRED`)
- Score model version bump

---

## 6. Manual review & override

| Action | Authority | Audit |
|---|---|---|
| Manual criterion score | Buyer evaluator role | `updated_by`, reason, prior value |
| Override knockout | Senior buyer / admin | Mandatory comment |
| System-derived refresh | Scheduled job | `score_model_version`, source timestamp |

Override does not rewrite historical `Answer` values — only score/qualification records.

---

## 7. Explainability (mandatory)

Every score result must explain:

| Field | Content |
|---|---|
| `source` | `CARRIER_DECLARED`, `CARRIER_PROFILE`, `BINTRANS_OPERATIONAL_DATA`, … |
| `input` | Answer value or KPI snapshot used |
| `rule` | Rule/criterion code + version |
| `weight` | Applied weight |
| `score` | Raw and normalized score |
| `contribution` | Weighted contribution to total |
| `knockout_reason` | Present if knockout triggered |

Stored in `rfx_answer_scores.explanation_json` and surfaced in buyer evaluation UI.

---

## 8. Knockout semantics

Knockout rules evaluate **after** validation passes and answer is persisted.

| Outcome | `qualification_results.status` |
|---|---|
| All knockouts pass | `QUALIFIED` or `CONDITIONALLY_QUALIFIED` |
| Knockout triggered | `REJECTED` with `knockout_reason_json` |
| Pending manual review | `PENDING_REVIEW` |

Evidence preserved: `KNOCKOUT_ANSWER_PERSISTED=YES`.

---

## 9. Version binding

| Artifact | Version field |
|---|---|
| Answers | `rule_version`, `score_model_version` on qualification-relevant answers |
| Score runs | `score_model_version` on `qualification_results` |
| Historical replay | Immutable `rfx_versions` + model version |

Changed rules **never** retroactively rewrite historical evidence without explicit recalculation event.

---

## 10. References

- [RFX_V3_DATA_MODEL.md](./RFX_V3_DATA_MODEL.md) — `rfx_score_models`, `rfx_answer_scores`
- [RFX_V3_EVENTS.md](./RFX_V3_EVENTS.md) — `rfx.score.calculated`
- [ADR-RFX-004](./adr/ADR-RFX-004-SCORING-ARCHITECTURE.md)
- Current v1: `services/rfx-service/internal/service/evaluation_service.go`
