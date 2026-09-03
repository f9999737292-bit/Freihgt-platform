# RFx v3 — Response Validation & Draft Safety

**Status:** Mandatory enterprise contract  
**Version:** v0.1  
**Scope:** Carrier response workspace, buyer RFx Studio, preview/test modes, publish/submit gates  
**Applies to:** RFx v3 questionnaire responses, autosave, draft/resume, version history, qualification/scoring side-effects

---

## 1. Core principle

```
INVALID_DATA_PERSISTENCE = FORBIDDEN
```

Invalid user input must **not** become authoritative server-side RFx response data.

The platform must simultaneously guarantee:

| Requirement | Rule |
|---|---|
| Previously valid saved answers | Must **never** be lost because of a later invalid edit |
| Invalid edits | Must remain **visible locally** so the user can fix them |
| Valid negative business answers | Must be **persisted** (including disqualifying answers) |
| Warnings | Must **not** be confused with validation errors |
| Knockout results | Must **not** be treated as invalid input |

---

## 2. Classification taxonomy

Four distinct outcome classes are mandatory. Implementations must never collapse these into a single generic “error” bucket.

### 2.1 `VALIDATION_ERROR`

**Definition:** Input violates schema, format, range, requiredness, cross-field consistency, or file constraints. The value cannot be accepted as authoritative response data.

**Behavior:**

```
VALIDATION_ERROR → SAVE_BLOCKED
```

**Examples:**

| Input | Result |
|---|---|
| `OWN_FLEET_COUNT = -5` | `VALIDATION_ERROR`, `SAVE=BLOCKED` |
| Invalid INN format | `VALIDATION_ERROR`, `SAVE=BLOCKED` |
| Required attachment missing when condition met | `VALIDATION_ERROR`, `SAVE=BLOCKED` |

### 2.2 `WARNING`

**Definition:** Input is syntactically and structurally valid, but business evaluation may flag concern, lower score, or require acknowledgement. The answer itself is legitimate evidence.

**Behavior:**

```
WARNING → SAVE_ALLOWED
```

**Example:**

| Input | Rule | Result |
|---|---|---|
| `OWN_FLEET_COUNT = 15` | Minimum for qualification = 35 | `VALUE_VALID=YES`, `SAVE=ALLOWED`, `WARNING_OR_SCORE_EFFECT=YES` |

Warnings may affect scoring or qualification messaging but **must not block save**.

### 2.3 `BUSINESS_RULE_RESULT`

**Definition:** Deterministic outcome of configured business/scoring rules applied to a **valid** persisted answer. Not a validation failure.

**Behavior:** Persist answer; record rule outcome/version; surface explanation in UX.

### 2.4 `KNOCKOUT` (`KNOCKOUT_CONDITION`)

**Definition:** A valid negative answer triggers disqualification or hard qualification failure under configured rules.

**Behavior:**

```
KNOCKOUT_CONDITION → SAVE_ALLOWED → QUALIFICATION_EFFECT_APPLIED
```

**Example:**

| Input | Rule | Result |
|---|---|---|
| `ADR_AVAILABLE = false` | ADR required for qualification | `VALUE_VALID=YES`, `SAVE=ALLOWED`, `KNOCKOUT_TRIGGERED=YES` |

The system **must retain the negative answer as evidence** for rejection. Knockout must **not** block save and must **not** be classified as `VALIDATION_ERROR`.

---

## 3. Validation layers

Validation runs in four ordered layers. Lower layers may short-circuit upper layers for a given field, but pre-submit must re-evaluate all layers.

| Layer | Name | Responsibility |
|---|---|---|
| L1 | `FIELD_VALIDATION` | Type, format, min/max, regex, enum, file type/size |
| L2 | `QUESTION_RULE_VALIDATION` | Requiredness, visibility, attachment rules per question definition |
| L3 | `CROSS_FIELD_VALIDATION` | Conditional dependencies across questions/sections |
| L4 | `PRE_SUBMIT_VALIDATION` | Full response completeness and publish/submit readiness |

### 3.1 Cross-field example

When:

```
ADR_AVAILABLE = true
```

Then required:

- `ADR_NUMBER`
- `ADR_EXPIRY`
- `ADR_DOCUMENT`

If any is absent:

| State | Value |
|---|---|
| Response status | `IN_PROGRESS` |
| Submit | Blocked |
| UX | Specific question-level errors shown |

---

## 4. Server authority

Client-side validation is **UX only**.

Server/domain validation is **authoritative**.

### 4.1 Required processing flow

```
Browser validation
  → API validation
    → Domain validation
      → Transaction
        → Persist
```

### 4.2 Server validation failure contract

On failure:

| Field | Value |
|---|---|
| HTTP status | `422 Unprocessable Entity` |
| Error envelope | Structured; no stack traces |

**Example:**

```json
{
  "code": "VALIDATION_FAILED",
  "errors": [
    {
      "section_id": "hse",
      "question_id": "own_fleet_count",
      "field": "value",
      "rule": "MIN_VALUE",
      "message_key": "rfx.validation.minimum",
      "params": {
        "minimum": 0
      }
    }
  ]
}
```

Rules:

- Do **not** expose internal stack traces, SQL, or service internals.
- `message_key` + `params` are i18n-safe; raw server exceptions are forbidden in client-visible payloads.
- Partial persistence of invalid values in the same autosave batch is forbidden (see §5).

---

## 5. Atomic autosave

Each autosave operation is one logical revision.

### 5.1 Transaction semantics

For one autosave batch:

```
VALIDATE_ALL
→ ERROR_PRESENT?
     YES → ROLLBACK WHOLE BATCH
     NO  → COMMIT
```

Do **not** persist half-valid versions of one logical autosave revision.

### 5.2 Autosave response metadata

Successful commit must return:

| Field | Purpose |
|---|---|
| `save_version` | Optimistic concurrency token |
| `last_saved_at` | Authoritative server timestamp |
| `last_saved_by` | Actor identity (trusted gateway context) |

Clients must send `save_version` (or equivalent etag/revision) on subsequent writes. Stale version → `409 Conflict` with recovery guidance.

---

## 6. Answer UI states

Each answer control must expose exactly one primary UI state:

| State | Meaning |
|---|---|
| `EMPTY` | Not entered |
| `DIRTY` | Changed locally since last successful server ack |
| `VALIDATING` | Client or server validation in flight |
| `INVALID` | Validation error present; not persisted |
| `VALID` | Passes validation locally; may still be unsaved |
| `SAVING` | Autosave/manual save in progress |
| `SAVED` | Last server ack matches current valid value |
| `SAVE_FAILED` | Network/server failure; last valid server state preserved |

Users must clearly distinguish:

- not entered
- entered but not saved
- invalid
- saved

---

## 7. Invalid edit preservation

When the user enters an invalid value:

| Rule | Required |
|---|---|
| Persist invalid value to server | **NO** |
| Clear the field | **NO** |
| Keep value visible in browser | **YES** |
| Show inline explanation | **YES** |

**Example (RU UX):**

```
ИНН
[123]

✕ ИНН должен содержать 10 или 12 цифр
НЕ СОХРАНЕНО
```

After correction:

```
[7701234567]

✓ Сохранено
```

---

## 8. Last valid server state

The client and server must always preserve:

```
LAST_VALID_SERVER_VERSION
```

When invalid client edits exist:

```
LAST_VALID_SERVER_STATE_PRESERVED = YES
```

UI must show:

- `Есть несохранённые изменения`
- `Последняя корректная версия сохранена <time>`

Invalid local drafts must never overwrite or delete the last committed valid snapshot.

---

## 9. Section error UX

Carrier section navigation must expose counts:

```
✓ Компания
⚠ Документы        2
✕ HSE              1
○ IT
```

Required counters per section:

| Counter | Includes |
|---|---|
| `SECTION_ERROR_COUNT` | `VALIDATION_ERROR` items blocking save/submit |
| `SECTION_WARNING_COUNT` | `WARNING` items |
| `SECTION_INCOMPLETE_COUNT` | Required but empty/in-progress items |

Icons/states:

| Symbol | Meaning |
|---|---|
| `✓` | No blocking errors; section complete or saveable |
| `⚠` | Warnings present |
| `✕` | Validation errors present |
| `○` | Incomplete / not started |

---

## 10. Global error summary

Carrier Response Workspace must include a global validation summary, e.g.:

```
Есть 3 ошибки
```

Each item must show:

| Element | Required |
|---|---|
| Section | Yes |
| Question | Yes |
| Error description | Yes |
| Expected requirement | Yes |
| Current value | If safe to display |
| Action | Yes |

**Example:**

```
Страхование
→ Страховое покрытие

Указано:
10 000 000 RUB

Требование:
не менее 50 000 000 RUB

[ Исправить ]
```

Warnings and knockouts appear in separate summary sections, not mixed with blocking validation errors.

---

## 11. Error deep link

Every global validation error must support:

```
GO_TO_ERROR
```

On click:

1. Open correct section
2. Scroll to question
3. Focus relevant field
4. Preserve user draft state (including invalid local edits)

Deep link must not discard unsaved local input.

---

## 12. Leaving page with invalid unsaved data

If the user navigates away with invalid and/or unsaved edits, show explicit confirmation:

```
Есть несохранённые изменения.
2 ответа содержат ошибки и не были сохранены.
```

Actions:

| Action | Behavior |
|---|---|
| `STAY_AND_FIX` | Remain on page |
| `LEAVE_WITHOUT_INVALID_CHANGES` | Abandon invalid local edits; retain last valid server state |

Forbidden messaging:

- Do **not** say `Все сохранено` if invalid edits remain.

---

## 13. Local recovery (optional)

Browser-local recovery may be implemented for resilience.

If implemented:

```
LOCAL_RECOVERY_IS_NOT_AUTHORITATIVE = YES
```

On return:

```
Найдены локальные несохранённые изменения
```

Actions:

| Action | Behavior |
|---|---|
| `RESTORE` | Load local draft into editor |
| `DISCARD` | Use last valid server version |

Restored values must pass validation again before server persistence.

---

## 14. Carrier draft / resume

### 14.1 Lifecycle states

| Status | Meaning |
|---|---|
| `NOT_STARTED` | No response work begun |
| `IN_PROGRESS` | Draft/active editing |
| `SUBMITTED` | Final submission complete |
| `WITHDRAWN` | Carrier withdrew |
| `LOCKED` | No further edits allowed |

Draft save is represented by:

```
IN_PROGRESS + last_saved_at + completion_percent
```

A separate `DRAFT_SAVED` domain status is **not required** unless future workflow needs explicit distinction.

### 14.2 Carrier capabilities

| Capability | Required |
|---|---|
| `SAVE_DRAFT` | Yes |
| `CONTINUE_LATER` | Yes |
| `RESUME` | Yes |

---

## 15. Buyer draft / resume

Buyer RFx Studio must support:

| Capability | Required |
|---|---|
| `RFx DRAFT` | Yes |
| `AUTOSAVE` | Yes |
| `MANUAL_SAVE_DRAFT` | Yes |
| `RESUME` | Yes |
| `VERSION_HISTORY` | Yes |

Draft may be edited freely before publication.

---

## 16. Preview as carrier

Mandatory capability:

```
PREVIEW_AS_CARRIER = YES
```

Preview must use the **current unpublished draft version**.

Supported form factors:

- `DESKTOP`
- `TABLET`
- `MOBILE`

Preview must render:

- sections
- questions
- required indicators
- conditional logic
- attachments
- validation behavior (client-side)
- progress
- communication text
- deadline
- save-draft behavior (simulated or sandboxed)
- submit behavior (simulated or sandboxed)

---

## 17. Interactive preview test mode

Buyer must be able to:

```
Пройти как перевозчик
```

using temporary preview answers.

Hard requirements:

```
PREVIEW_DATA_ONLY = YES
REAL_RESPONSE_CREATED = NO
```

Buyer can test:

- conditional questions
- validation
- warnings
- knockout
- required fields
- document requirements
- completion behavior

Preview test answers must never enter production response tables, scoring runs, or participant-visible state.

---

## 18. Publish readiness gate (buyer)

Before publish, RFx Studio runs readiness validation.

**Example checklist:**

| Area | Result |
|---|---|
| Основные данные | PASS |
| Анкета | PASS |
| Required questions | PASS |
| Scoring weights | 100% |
| Knockout rules | VALID |
| Participants | READY |
| Invitation | READY |
| Preview | CHECKED |

Gate rule:

```
ERRORS = 0 → PUBLISH_ALLOWED = YES
```

Warnings may require explicit acknowledgement but must be classified separately from blocking errors.

---

## 19. Pre-submit carrier gate

Before carrier Submit, run **full response validation** again (all four layers).

Required checks:

| Check | Required |
|---|---|
| `REQUIRED_FIELDS_VALID` | Yes |
| `DOCUMENT_REQUIREMENTS_VALID` | Yes |
| `CONDITIONAL_RULES_VALID` | Yes |
| `NUMBER_VALIDATION_VALID` | Yes |
| `DATE_VALIDATION_VALID` | Yes |
| `CROSS_FIELD_VALIDATION_VALID` | Yes |

If:

```
ERROR_COUNT > 0 → SUBMIT_ALLOWED = NO
SUBMIT_DISABLED = YES
```

Show:

```
Нельзя отправить: исправьте N ошибок
```

Warnings and knockouts do **not** disable submit by themselves.

---

## 20. Post-publish editing

Published RFx (`PUBLISHED`) must **not** be silently mutated.

Any material change must create:

```
NEW_RFX_VERSION
```

### 20.1 Change classification

| Class | Examples |
|---|---|
| `NON_MATERIAL` | Typo correction, clarifying help text |
| `MATERIAL` | ADR qualification threshold change, knockout rule change, required question added |

Material change must trigger:

```
CHANGE_IMPACT_ANALYSIS
```

Return:

| Field | Purpose |
|---|---|
| `AFFECTED_RESPONSES` | Existing carrier responses impacted |
| `AFFECTED_PARTICIPANTS` | Participants requiring notice |
| `RECONFIRMATION_REQUIRED` | Whether re-acceptance needed |
| `RESCORING_REQUIRED` | Whether scoring/qualification must rerun |

---

## 21. Version history

Support:

| Capability | Required |
|---|---|
| `VERSION_HISTORY` | Yes |
| `COMPARE_VERSIONS` | Yes |
| `RESTORE_DRAFT_VERSION` | Yes |

For every version record:

| Field | Required |
|---|---|
| `version` | Yes |
| `changed_by` | Yes |
| `changed_at` | Yes |
| `change_summary` | Yes |

Published versions are immutable audit records.

---

## 22. Mandatory architecture flags

Non-negotiable platform flags:

```text
INVALID_DATA_PERSISTENCE=FORBIDDEN
SERVER_VALIDATION_REQUIRED=YES
SAVE_ONLY_VALID_STATE=YES
INVALID_CLIENT_EDIT_RETAINED_LOCALLY=YES
LAST_VALID_SERVER_STATE_PRESERVED=YES

FIELD_ERROR_INLINE=YES
SECTION_ERROR_COUNTER=YES
GLOBAL_ERROR_SUMMARY=YES
ERROR_DEEP_LINK=YES

BUYER_DRAFT_SAVE=YES
BUYER_AUTOSAVE=YES
BUYER_RESUME=YES

CARRIER_RESPONSE_DRAFT=YES
CARRIER_RESPONSE_AUTOSAVE=YES
CARRIER_RESPONSE_RESUME=YES

PREVIEW_AS_CARRIER=YES
PREVIEW_DRAFT_VERSION=YES
PREVIEW_INTERACTIVE_TEST_MODE=YES

SUBMIT_WITH_ERRORS=FORBIDDEN
PUBLISH_WITH_ERRORS=FORBIDDEN

WARNING_BLOCKS_SAVE=NO
KNOCKOUT_BLOCKS_SAVE=NO
KNOCKOUT_RECORDED_AND_EXPLAINED=YES

PRE_PUBLISH_EDIT=YES
POST_PUBLISH_VERSIONED_EDIT=YES
CHANGE_IMPACT_ANALYSIS=YES

VERSION_HISTORY=YES
COMPARE_VERSIONS=YES
RESTORE_DRAFT_VERSION=YES

PUBLISH_READINESS_GATE=YES
PRE_SUBMIT_VALIDATION_GATE=YES
```

---

## 23. Security & audit

For every **accepted** answer persisted, store:

| Field | Required |
|---|---|
| `answer_value` | Yes |
| `answer_version` | Yes |
| `answer_source` | Yes (`CARRIER`, `BUYER_PREVIEW_TEST`, etc.) |
| `updated_by` | Yes |
| `updated_at` | Yes |
| `validation_version` | Yes |

For qualification-relevant answers also preserve:

| Field | Required |
|---|---|
| `rule_version` | Yes |
| `score_model_version` | Yes |

Rule:

> Never allow a changed rule to retroactively rewrite historical evidence without a new qualification calculation/version.

Preview/test answers must be tagged so they are excluded from audit evidence and scoring history.

---

## 24. Future acceptance tests

Architecture mandates the following acceptance tests for implementation phases:

### 24.1 Persistence & classification

| Test ID | Assertion |
|---|---|
| `INVALID_NUMBER_NOT_PERSISTED` | Negative/out-of-range number rejected server-side |
| `INVALID_DATE_NOT_PERSISTED` | Invalid date rejected server-side |
| `INVALID_FILE_NOT_PERSISTED` | Invalid attachment rejected server-side |
| `VALID_NEGATIVE_ANSWER_PERSISTED` | Legitimate “no/false/zero” answers saved |
| `KNOCKOUT_ANSWER_PERSISTED` | Disqualifying valid answer saved with knockout flag |
| `WARNING_ANSWER_PERSISTED` | Warning-triggering valid answer saved |

### 24.2 UX & state preservation

| Test ID | Assertion |
|---|---|
| `PREVIOUS_VALID_DATA_NOT_LOST` | Invalid edit does not destroy prior valid snapshot |
| `INVALID_EDIT_VISIBLE_TO_USER` | Invalid local value remains visible |
| `ERROR_INLINE_VISIBLE` | Field-level message shown |
| `ERROR_SUMMARY_VISIBLE` | Global summary lists blocking errors |
| `ERROR_DEEP_LINK_WORKS` | Go-to-error navigation focuses field |

### 24.3 Autosave & resume

| Test ID | Assertion |
|---|---|
| `AUTOSAVE_VALID_BATCH_PASS` | Valid batch commits atomically |
| `AUTOSAVE_INVALID_BATCH_ROLLBACK` | Invalid batch fully rolled back |
| `RESUME_FROM_LAST_VALID_VERSION` | Resume restores last valid server state |

### 24.4 Preview & submit/publish gates

| Test ID | Assertion |
|---|---|
| `PREVIEW_DOES_NOT_CREATE_RESPONSE` | Preview/test creates no real response rows |
| `SUBMIT_WITH_ERROR_DENIED` | Submit blocked when errors > 0 |
| `SUBMIT_WITH_ZERO_ERRORS_PASS` | Submit allowed when blocking errors = 0 |
| `PUBLISHED_RFX_DIRECT_EDIT_DENIED` | Published RFx cannot mutate in place |
| `NEW_VERSION_CREATED_FOR_MATERIAL_CHANGE` | Material edit creates new RFx version |

---

## 25. Contract summary matrix

| Outcome class | Blocks save | Blocks submit | Persisted | User-visible locally if invalid |
|---|---|---|---|---|
| `VALIDATION_ERROR` | Yes | Yes | No | Yes (draft only) |
| `WARNING` | No | No | Yes | N/A |
| `BUSINESS_RULE_RESULT` | No | No | Yes | N/A |
| `KNOCKOUT` | No | No | Yes | N/A |

---

## 26. References

- [RFX_V3_DOMAIN_MODEL.md](./RFX_V3_DOMAIN_MODEL.md)
- [RFX_V3_API.md](./RFX_V3_API.md)
- [RFX_V3_STATE_MACHINES.md](./RFX_V3_STATE_MACHINES.md)
- [RFX_V3_QUESTIONNAIRE_ENGINE.md](./RFX_V3_QUESTIONNAIRE_ENGINE.md)
- [RFX_V3_UX.md](./RFX_V3_UX.md)
- [RFX_V3_SECURITY.md](./RFX_V3_SECURITY.md)
- [RFX_V3_ROADMAP.md](./RFX_V3_ROADMAP.md)
- [ADR-RFX-011](./adr/ADR-RFX-011-RESPONSE-VALIDATION-AND-DRAFT-SAFETY.md)
- Platform validation levels: `docs/engineering/VALIDATION_LEVELS.md`
- API error envelope conventions: OpenAPI `ErrorResponse` schemas

---

**Document control:** Changes to this contract require architecture review. Implementation tasks must cite applicable sections and acceptance test IDs from §24.
