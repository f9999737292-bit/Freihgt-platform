# RFx v3.0A — API Architecture

**Status:** Architecture draft  
**Normative companion:** [RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md](./RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md)

---

## 1. Principles

1. **Server validation is authoritative** — client validation is UX-only.
2. **Valid-only persistence** — invalid values return `422`, nothing committed.
3. **Atomic autosave batches** — all-or-nothing per logical revision.
4. **Optimistic concurrency** — stale `save_version` → `409 Conflict`.
5. **Structured errors** — no stack traces in client-visible payloads.
6. **Server-verified company context** — client-supplied company authority is forbidden.

---

## 1.1 Identity and company authority

External clients authenticate at API Gateway (JWT Bearer). Trusted identity headers (`X-Tenant-ID`, `X-User-ID`) are gateway-established only.

```
CLIENT_SUPPLIED_COMPANY_AUTHORITY = FORBIDDEN
```

| Flag | Value |
|---|---|
| `TENANT_AUTHORITY` | SERVER_VERIFIED |
| `USER_AUTHORITY` | SERVER_VERIFIED |
| `COMPANY_AUTHORITY` | SERVER_VERIFIED |
| `CROSS_COMPANY_SPOOF` | DENIED |
| `CROSS_TENANT_SPOOF` | DENIED |

**Company resolution:**

| Flow | Rule |
|---|---|
| Carrier response write | Resolve `participant_company_id` from authenticated user + tenant + **participant membership** for the RFx event |
| Buyer RFx write | Resolve `owner_company_id` from authenticated user + tenant + **buyer membership** (`SHIPPER`, `FORWARDER`, `LSP`) |
| Optional `X-Company-ID` | May be echoed **only** when it matches server-resolved membership; otherwise `403` |

Never trust arbitrary browser-supplied company ID as authorization source. See [RFX_V3_SECURITY.md](./RFX_V3_SECURITY.md).

---

## 2. Response answer write surfaces

### 2.1 Patch / autosave answers

| Concern | Contract |
|---|---|
| Method | `PATCH` (batch) preferred for autosave |
| Scope | `/api/v1/rfx-events/{id}/responses/{response_id}/answers` (illustrative) |
| Body | Array of `{ section_id, question_id, field, value, attachment_refs? }` |
| Headers | JWT; gateway-verified `X-Tenant-ID`, `X-User-ID`; company context server-resolved (see §1.1) |
| Concurrency | `If-Match: save_version` or body `expected_save_version` |

**Processing flow:**

```
Browser validation → API validation → Domain validation → Transaction → Persist
```

### 2.2 Atomic autosave semantics

Per batch:

```
VALIDATE_ALL → ERROR_PRESENT? YES: ROLLBACK WHOLE BATCH / NO: COMMIT
```

Success response includes:

| Field | Type | Notes |
|---|---|---|
| `save_version` | string/int | New optimistic token |
| `last_saved_at` | ISO8601 | Server timestamp |
| `last_saved_by` | uuid | Trusted actor |
| `warnings` | array | Non-blocking items |
| `knockouts` | array | Qualification effects applied |

### 2.3 Validation failure — HTTP 422

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
      "params": { "minimum": 0 }
    }
  ]
}
```

Error class in payload: always `VALIDATION_ERROR` for blocking items.

### 2.4 Warnings and knockouts in success responses

Non-blocking outcomes are **distinct** from `errors`:

| Class | HTTP | Blocks save |
|---|---|---|
| `VALIDATION_ERROR` | 422 | Yes |
| `WARNING` | 200 + `warnings[]` | No |
| `KNOCKOUT` | 200 + `knockouts[]` | No |

Example warning item:

```json
{
  "class": "WARNING",
  "section_id": "fleet",
  "question_id": "own_fleet_count",
  "code": "BELOW_QUALIFICATION_THRESHOLD",
  "message_key": "rfx.warning.fleet_below_minimum",
  "params": { "actual": 15, "minimum": 35 }
}
```

Example knockout item:

```json
{
  "class": "KNOCKOUT",
  "section_id": "hse",
  "question_id": "adr_available",
  "code": "ADR_REQUIRED_KNOCKOUT",
  "qualification_effect": "DISQUALIFIED"
}
```

---

## 3. Pre-submit validation

Dedicated endpoint or action (illustrative):

```
POST /api/v1/rfx-events/{id}/responses/{response_id}/validate-submit
```

Runs all four validation layers (see questionnaire engine). Returns:

| Field | Meaning |
|---|---|
| `submit_allowed` | `true` iff blocking error count = 0 |
| `error_count` | Blocking validation errors |
| `warning_count` | Non-blocking warnings |
| `errors[]` | Structured list for global summary + deep link |
| `completion_percent` | Progress indicator |

Gate: `SUBMIT_WITH_ERRORS=FORBIDDEN` — see validation contract §19.

---

## 4. Buyer publish readiness

```
POST /api/v1/rfx-events/{id}/validate-publish
```

Returns checklist items with `PASS` / `FAIL` / `WARN`. Publish permitted only when blocking `FAIL` count = 0 (`PUBLISH_WITH_ERRORS=FORBIDDEN`).

---

## 5. Buyer draft / autosave

| Operation | Notes |
|---|---|
| `PUT/PATCH` RFx draft | Buyer Studio autosave |
| Manual save draft | Same validation path; returns version metadata |
| Version history | `GET .../versions` |

Buyer draft edits before publish are not carrier responses; preview test data uses separate sandbox endpoints tagged `PREVIEW_DATA_ONLY=YES`.

---

## 6. Preview-as-carrier (buyer)

Sandbox endpoints must:

- Accept preview session token scoped to buyer + draft version
- Never create `Response` / production `Answer` rows
- Store preview answers in **ephemeral preview state** or **isolated preview storage** (`rfx_preview_sessions` / TTL cache — see [RFX_V3_DATA_MODEL.md](./RFX_V3_DATA_MODEL.md))

Mandatory flags:

```
PREVIEW_DATA_ONLY=YES
REAL_RESPONSE_CREATED=NO
PREVIEW_DATA_NOT_IN_PRODUCTION_ANSWER=YES
```

Preview data **must not** use authoritative `answer_source` values. `BUYER_PREVIEW_TEST` is a preview-session tag only — never persisted on production `Answer` rows.

`REAL_RESPONSE_CREATED=NO` — validation contract §16–17.

---

## 7. Post-publish versioned edit

Material changes:

```
POST /api/v1/rfx-events/{id}/versions
→ triggers CHANGE_IMPACT_ANALYSIS
→ returns AFFECTED_RESPONSES, RECONFIRMATION_REQUIRED, RESCORING_REQUIRED
```

Direct in-place mutation of published version: **denied** (`409` or `403`).

---

## 8. Concurrency & conflicts

| Situation | HTTP | Client action |
|---|---|---|
| Stale `save_version` | `409 Conflict` | Reload last valid server state |
| Validation failure | `422` | Retain local invalid draft; show inline errors |
| Auth / tenant mismatch | `401` / `403` | No partial persist |

---

## 9. References

- [RFX_V3_DOMAIN_MODEL.md](./RFX_V3_DOMAIN_MODEL.md)
- [RFX_V3_QUESTIONNAIRE_ENGINE.md](./RFX_V3_QUESTIONNAIRE_ENGINE.md)
- OpenAPI `ErrorResponse` — `packages/openapi/`
- [ADR-RFX-011](./adr/ADR-RFX-011-RESPONSE-VALIDATION-AND-DRAFT-SAFETY.md)
