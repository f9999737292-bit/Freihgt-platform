# RFx v3.0A — Questionnaire Engine

**Status:** Architecture draft  
**Normative companion:** [RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md](./RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md)

---

## 1. Scope

Configurable questionnaire engine for RFx v3.0A: section/question definitions, conditional visibility, validation rule execution, attachment requirements, and scoring/qualification inputs.

---

## 2. Validation layers

Engine executes four layers (normative detail: validation contract §3):

| Layer | ID | Responsibility |
|---|---|---|
| L1 | `FIELD_VALIDATION` | Type, format, min/max, regex, enum, file constraints |
| L2 | `QUESTION_RULE_VALIDATION` | Requiredness, visibility, per-question attachment rules |
| L3 | `CROSS_FIELD_VALIDATION` | Conditional dependencies (e.g. ADR fields when ADR=true) |
| L4 | `PRE_SUBMIT_VALIDATION` | Full response completeness before submit/publish |

Layers L1–L3 run on autosave batches. L4 runs on pre-submit and publish-readiness gates.

---

## 3. Engine outputs

Each evaluation produces a `ValidationResult` containing zero or more of:

| Output | Class | Blocks persist |
|---|---|---|
| `ValidationError` | `VALIDATION_ERROR` | Yes |
| `Warning` | `WARNING` | No |
| `KnockoutResult` | `KNOCKOUT` | No |
| `BusinessRuleResult` | `BUSINESS_RULE_RESULT` | No |

**Rule:** Never treat knockout or warning as validation error.

---

## 4. Scoring & qualification input rules

### 4.1 Core distinction (mandatory)

```
INVALID VALUE  ≠  VALID NEGATIVE BUSINESS ANSWER
```

| Example | Engine/scoring treatment |
|---|---|
| `fleet_count = -5` | L1 `ValidationError`; **not persisted**; scoring **not invoked** |
| `fleet_count = 15`, min qualification 35 | Valid answer **persisted**; scoring applies penalty/warning |
| `ADR_AVAILABLE = false`, ADR mandatory | Valid answer **persisted**; `KnockoutResult` recorded as rejection evidence |

Scoring and qualification modules consume **authoritative persisted answers only**. They must not read client `AnswerDraft`, preview sandbox data, or invalid local state.

Full scoring architecture: [RFX_V3_SCORING_ENGINE.md](./RFX_V3_SCORING_ENGINE.md).

### 4.2 Version binding

When scoring runs:

- Bind `rule_version` and `score_model_version` on qualification-relevant answers
- Rule changes do **not** retroactively rewrite historical evidence without new calculation version

See [RFX_V3_DOMAIN_MODEL.md](./RFX_V3_DOMAIN_MODEL.md) §3 and [RFX_V3_SECURITY.md](./RFX_V3_SECURITY.md).

### 4.3 Knockout vs validation

| Property | Knockout | Validation error |
|---|---|---|
| `KNOCKOUT_BLOCKS_SAVE` | **NO** | N/A (save blocked) |
| Evidence retained | **YES** | N/A (not persisted) |
| Blocks submit | **NO** (by itself) | **YES** (when errors > 0) |

---

## 5. Conditional logic

Questionnaire definitions express:

- Visibility conditions
- Required-when conditions
- Attachment required-when
- Cross-field validators

Example: `ADR_AVAILABLE=true` ⇒ require `ADR_NUMBER`, `ADR_EXPIRY`, `ADR_DOCUMENT`. Missing any ⇒ `ValidationError` on affected fields; response stays `IN_PROGRESS`.

---

## 6. Buyer preview / test mode

Interactive preview executes engine rules against **sandbox** answer store:

```
PREVIEW_DATA_ONLY=YES
REAL_RESPONSE_CREATED=NO
```

Preview may simulate validation, warnings, and knockout UX without writing production `Answer` rows.

---

## 7. Post-publish rule changes

Material questionnaire/scoring rule changes require new `RfxVersion` and `ChangeImpactAnalysis` (`RESCORING_REQUIRED`, `RECONFIRMATION_REQUIRED`).

Published historical versions remain immutable.

---

## 8. References

- [RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md](./RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md) §2–4, §18–20
- [RFX_V3_API.md](./RFX_V3_API.md)
- [RFX_V3_ROADMAP.md](./RFX_V3_ROADMAP.md) — Wave 2 scoring integration
