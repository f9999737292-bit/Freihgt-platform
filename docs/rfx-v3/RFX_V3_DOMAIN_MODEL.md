# RFx v3.0A — Domain Model

**Status:** Architecture draft  
**Normative companion:** [RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md](./RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md)

---

## 1. Scope

Domain model for Enterprise RFx v3.0A covering buyer-authored tenders, carrier responses, questionnaire answers, validation outcomes, scoring/qualification effects, and versioned publication history.

---

## 2. Core aggregates

| Aggregate | Responsibility |
|---|---|
| `RfxEvent` | Buyer-owned RFx header, publication state, version pointer |
| `RfxVersion` | Immutable published snapshot; draft working copy before publish |
| `QuestionnaireDefinition` | Sections, questions, rules, scoring bindings (versioned with RFx) |
| `Response` | Carrier participation instance against an RFx |
| `ResponseVersion` | Optimistic-concurrency revision of persisted response answers |
| `Answer` | **Authoritative** persisted answer for one question in a response |
| `AnswerDraft` | Client-side pending edit; **non-authoritative** until validation passes |
| `Participant` | Invitation and access scope for carrier company |

---

## 3. Value objects & outcome types

| Type | Role |
|---|---|
| `ValidationResult` | Outcome of a validation pass (layer, status, items) |
| `ValidationError` | Blocking `VALIDATION_ERROR`; prevents persist/submit |
| `Warning` | Non-blocking advisory/score hint; save allowed |
| `KnockoutResult` | Qualification disqualification derived from **valid** persisted answer |
| `BusinessRuleResult` | Deterministic scoring/qualification side-effect record |
| `AnswerProvenance` | Source, actor, timestamps, rule/score model versions |
| `ChangeImpactAnalysis` | Material post-publish change impact on responses/participants |

Classification semantics: see validation contract §2. Do not collapse `ValidationError`, `Warning`, and `KnockoutResult`.

---

## 4. Answer authority model

### 4.1 Authoritative `Answer`

An `Answer` record exists **only after** server/domain validation accepts the value.

Required fields on persist:

- `answer_value`
- `answer_version`
- `answer_source` (`CARRIER`, `BUYER_PREVIEW_TEST`, …)
- `updated_by`, `updated_at`
- `validation_version`

Qualification-relevant answers also store `rule_version`, `score_model_version`.

### 4.2 Non-authoritative `AnswerDraft`

`AnswerDraft` represents client pending state:

- Invalid values **may** exist in browser/local recovery stores
- Invalid drafts **must not** be modeled as authoritative `Answer` rows
- On successful validation, draft promotes to `Answer` in an atomic batch

### 4.3 Last valid server state

Each response maintains:

```
LAST_VALID_SERVER_VERSION
```

When invalid client edits exist:

```
LAST_VALID_SERVER_STATE_PRESERVED = YES
```

Prior valid answers are never destroyed by a failed or rejected edit.

---

## 5. Domain invariants

| Invariant | Rule |
|---|---|
| `INVALID_DATA_PERSISTENCE` | **FORBIDDEN** |
| `LAST_VALID_SERVER_STATE_PRESERVED` | **YES** |
| `VALID_NEGATIVE_ANSWER_PERSISTED` | **YES** — e.g. `ADR_AVAILABLE=false` when valid |
| `KNOCKOUT_ANSWER_PERSISTED` | **YES** — knockout evidence must be stored |
| `PREVIEW_DATA_NOT_IN_RESPONSE` | Preview/test answers never become `Response`/`Answer` |
| `PUBLISHED_VERSION_IMMUTABLE` | Published `RfxVersion` rows are not overwritten |

---

## 6. Invalid vs valid negative answers

| Case | Domain treatment |
|---|---|
| `fleet_count = -5` | `ValidationError`; **no** `Answer` row |
| `fleet_count = 15` (threshold 35) | Valid `Answer`; `Warning` / score effect |
| `ADR_AVAILABLE = false` (mandatory ADR) | Valid `Answer`; `KnockoutResult` |

Scoring architecture consumes **persisted valid answers only**. See [RFX_V3_QUESTIONNAIRE_ENGINE.md](./RFX_V3_QUESTIONNAIRE_ENGINE.md) §4.

---

## 7. Versioning

| Entity | Versioning |
|---|---|
| RFx draft (buyer) | Working draft + `RfxVersion` history |
| RFx published | Immutable `RfxVersion` |
| Response | `ResponseVersion` / `save_version` optimistic concurrency |
| Material post-publish edit | **New** `RfxVersion`; `ChangeImpactAnalysis` |

Capabilities: `VERSION_HISTORY`, `COMPARE_VERSIONS`, `RESTORE_DRAFT_VERSION` — see validation contract §20–21.

---

## 8. References

- [RFX_V3_STATE_MACHINES.md](./RFX_V3_STATE_MACHINES.md) — response lifecycle
- [RFX_V3_API.md](./RFX_V3_API.md) — persist and validation endpoints
- [RFX_V3_SECURITY.md](./RFX_V3_SECURITY.md) — provenance and audit
- [ADR-RFX-011](./adr/ADR-RFX-011-RESPONSE-VALIDATION-AND-DRAFT-SAFETY.md)
