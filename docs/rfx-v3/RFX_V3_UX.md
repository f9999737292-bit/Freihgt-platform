# RFx v3.0A — UX Architecture

**Status:** Architecture draft  
**Normative companion:** [RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md](./RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md)

---

## 1. Surfaces

| Surface | Primary actor | Key flows |
|---|---|---|
| **RFx Studio** (buyer) | Shipper / procurement | Draft, autosave, preview-as-carrier, publish readiness |
| **Carrier Response Workspace** | Carrier | Draft/resume, autosave, validation UX, submit |
| **Preview / test mode** | Buyer | Interactive walkthrough without real responses |

---

## 2. Mandatory carrier workspace capabilities

| Flag | Requirement |
|---|---|
| `FIELD_ERROR_INLINE` | Per-field error under control |
| `SECTION_ERROR_COUNTER` | Section nav shows error/warning/incomplete counts |
| `GLOBAL_ERROR_SUMMARY` | Aggregated blocking error list |
| `ERROR_DEEP_LINK` | `GO_TO_ERROR` navigates to field |
| `LAST_SAVED_AT` | Visible authoritative save timestamp |
| `UNSAVED_INVALID_CHANGES` | Banner when local invalid/unsaved edits exist |
| `SAVE_DRAFT` | Explicit save action |
| `CONTINUE_LATER` | Exit with last valid server state preserved |
| `RESUME` | Return to `IN_PROGRESS` workspace |
| `PREVIEW_AS_CARRIER` | Buyer-only; see validation contract §16 |
| `PRE_SUBMIT_VALIDATION_GATE` | Submit disabled until blocking errors = 0 |

---

## 3. Save status messaging

### 3.1 Allowed messages

| State | Example copy |
|---|---|
| All valid edits saved | `✓ Сохранено` / `Все изменения сохранены` |
| Unsaved valid dirty | `Есть несохранённые изменения` |
| Invalid local edits | `НЕ СОХРАНЕНО` + inline reason |
| Last valid preserved | `Последняя корректная версия сохранена <time>` |

### 3.2 Forbidden messaging

```
"Все изменения сохранены" / "Все сохранено"
```

**MUST NEVER** be shown while invalid unsaved edits exist.

---

## 4. Section navigation UX

Section list shows status icons and counts:

```
✓ Компания
⚠ Документы        2    ← SECTION_WARNING_COUNT
✕ HSE              1    ← SECTION_ERROR_COUNT
○ IT                 ← SECTION_INCOMPLETE_COUNT
```

Counters defined in validation contract §9.

---

## 5. Global error summary

Required panel (e.g. `Есть 3 ошибки`) listing:

- Section + question
- Error description + expected requirement
- Current value (if safe)
- `[ Исправить ]` → triggers `GO_TO_ERROR`

Warnings and knockouts use **separate** summary sections.

---

## 6. Invalid edit preservation

When user enters invalid value:

- Field **remains visible** with invalid content
- Inline error shown
- `НЕ СОХРАНЕНО` indicator
- Last valid server value **not** destroyed

Example:

```
ИНН [123]
✕ ИНН должен содержать 10 или 12 цифр
```

After fix → `✓ Сохранено`.

---

## 7. Leave-page confirmation

When navigating away with invalid/unsaved edits:

```
Есть несохранённые изменения.
2 ответа содержат ошибки и не были сохранены.
```

Actions: `STAY_AND_FIX` | `LEAVE_WITHOUT_INVALID_CHANGES`.

---

## 8. Buyer RFx Studio UX

| Capability | UX |
|---|---|
| `BUYER_AUTOSAVE` | Background save with version indicator |
| `MANUAL_SAVE_DRAFT` | Explicit save control |
| `BUYER_RESUME` | Return to last draft |
| `VERSION_HISTORY` | Timeline + compare + restore draft version |
| `PREVIEW_AS_CARRIER` | Desktop/tablet/mobile preview of draft |
| `PREVIEW_INTERACTIVE_TEST_MODE` | «Пройти как перевозчик» — sandbox only |
| `PUBLISH_READINESS_GATE` | Checklist panel; publish CTA disabled until errors = 0 |

---

## 9. Submit UX (carrier)

When `ERROR_COUNT > 0`:

- Submit button disabled (`SUBMIT_DISABLED=YES`)
- Message: `Нельзя отправить: исправьте N ошибок`
- Deep links to each blocking error

Knockout alone does **not** disable submit.

---

## 10. References

- [RFX_V3_STATE_MACHINES.md](./RFX_V3_STATE_MACHINES.md)
- [RFX_V3_API.md](./RFX_V3_API.md)
- [RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md](./RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md) §6–12
