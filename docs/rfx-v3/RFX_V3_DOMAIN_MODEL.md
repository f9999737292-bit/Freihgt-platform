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
- `answer_source` — **authoritative production sources only** (see §4.4)
- `updated_by`, `updated_at`
- `validation_version`

Qualification-relevant answers also store `rule_version`, `score_model_version`.

### 4.4 Authoritative `answer_source` values

Production `Answer` records may record provenance from:

| Source | Meaning |
|---|---|
| `CARRIER_DECLARED` | Carrier entered value in response workspace |
| `CARRIER_PROFILE` | Reused from Carrier 360 / company profile |
| `DOCUMENT_VERIFIED` | Extracted or confirmed from uploaded document |
| `BINTRANS_OPERATIONAL_DATA` | Derived from shipment/KPI/operational history |
| `BUYER_REVIEW` | Buyer-entered correction during evaluation (audited) |
| `EXTERNAL_VERIFICATION` | Third-party or registry verification |
| `AI_EXTRACTED_PENDING_REVIEW` | AI extraction awaiting human confirmation |

**Forbidden as authoritative `Answer` source:**

| Source | Rule |
|---|---|
| `BUYER_PREVIEW_TEST` | **MUST NOT** appear on production `Answer` / `Response` rows |

Preview/test data lives in **ephemeral preview state** or **isolated preview storage** only:

```
PREVIEW_DATA_ONLY=YES
REAL_RESPONSE_CREATED=NO
PREVIEW_DATA_NOT_IN_PRODUCTION_ANSWER=YES
```

See [RFX_V3_SECURITY.md](./RFX_V3_SECURITY.md) §3 and [RFX_V3_API.md](./RFX_V3_API.md) §6.

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
| `PREVIEW_DATA_NOT_IN_PRODUCTION_ANSWER` | Preview data **must not** use authoritative `answer_source` |
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

## 9. Company authority (trust boundary)

```
CLIENT_SUPPLIED_COMPANY_AUTHORITY = FORBIDDEN
```

Company context for RFx operations is **never** taken from arbitrary browser-supplied identifiers. If `X-Company-ID` (or equivalent internal context) is present, it must represent company membership **validated and resolved server-side** from:

```
authenticated user + tenant + membership + participant/owner authorization
```

| Context | Resolution rule |
|---|---|
| Carrier response | Company must resolve to **authorized participant membership** for the RFx event |
| Buyer RFx Studio | Company must resolve to **authorized buyer membership** (owner company or delegated buyer role) |

Mandatory flags:

```text
TENANT_AUTHORITY=SERVER_VERIFIED
USER_AUTHORITY=SERVER_VERIFIED
COMPANY_AUTHORITY=SERVER_VERIFIED
CROSS_COMPANY_SPOOF=DENIED
CROSS_TENANT_SPOOF=DENIED
```

See [RFX_V3_SECURITY.md](./RFX_V3_SECURITY.md) §1–2 and [RFX_V3_API.md](./RFX_V3_API.md) §1.

---

## 10. References

- [RFX_V3_STATE_MACHINES.md](./RFX_V3_STATE_MACHINES.md) — response lifecycle
- [RFX_V3_API.md](./RFX_V3_API.md) — persist and validation endpoints
- [RFX_V3_SECURITY.md](./RFX_V3_SECURITY.md) — provenance and audit
- [ADR-RFX-011](./adr/ADR-RFX-011-RESPONSE-VALIDATION-AND-DRAFT-SAFETY.md)
