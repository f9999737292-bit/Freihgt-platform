# ADR-RFX-011: Response Validation and Draft Safety

**Status:** Accepted (architecture draft)  
**Date:** 2026-09-03  
**Deciders:** Enterprise RFx v3.0A architecture stream  
**Normative detail:** [RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md](../RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md)

---

## Context

Enterprise RFx v3.0A introduces long-running buyer and carrier questionnaires with autosave, draft/resume, scoring, knockout qualification, and preview-as-carrier workflows.

Without explicit architecture:

- Invalid client input could be persisted as authoritative answers.
- Valid disqualifying answers (e.g. «ADR not available») could be rejected as «validation errors».
- Autosave could commit partial invalid batches.
- Preview/test activity could contaminate production response data.
- Users could lose previously valid saved answers when correcting invalid edits.

Pilot and v1 RFx flows use simpler forms; v3 requires enterprise-grade draft safety and auditability.

---

## Decision

**Authoritative RFx response state contains valid persisted data only.**

1. **Valid-only persistence** — Server/domain validation is authoritative. Invalid values are never written to production `Answer` records.
2. **Client draft retention** — Invalid edits remain visible locally (`AnswerDraft`) until corrected; they are not cleared silently.
3. **Last valid server state** — Each response preserves the last successfully committed revision; failed saves do not destroy prior valid answers.
4. **Outcome taxonomy** — Four classes: `VALIDATION_ERROR`, `WARNING`, `BUSINESS_RULE_RESULT`, `KNOCKOUT`. Only validation errors block save/submit.
5. **Atomic autosave** — One logical autosave batch validates entirely; on any blocking error, rollback the whole batch.
6. **Preview isolation** — Preview/test answers are `PREVIEW_DATA_ONLY`; they never create real carrier responses.
7. **Versioned publication** — Published RFx versions are immutable; material changes create new versions with impact analysis.

---

## Alternatives considered

### A. Persist everything; validate only on submit

**Rejected.** Would store invalid authoritative data, break audit/scoring evidence, and allow partial corrupt snapshots in autosave.

### B. Reject invalid input by clearing the field

**Rejected.** Hides user mistakes, increases support burden, violates «invalid edit visible locally» requirement.

### C. Treat knockout as validation error

**Rejected.** Would block save of legitimate negative business evidence required for qualification audit.

### D. Separate `DRAFT_SAVED` response status

**Rejected.** Adds state complexity without business need; `IN_PROGRESS + last_saved_at + save_version` suffices.

---

## Consequences

### Positive

- Clear audit trail for qualification and rejection.
- Predictable autosave semantics for carriers on long questionnaires.
- Buyer preview can safely test rules without data contamination.
- UX can distinguish errors, warnings, and knockouts.

### Negative / cost

- Client must maintain dual state: local draft + last valid server snapshot.
- Server must implement batch validation transactions.
- API must return structured 422/warning/knockout payloads.
- Scoring must bind to rule/model versions.

---

## Security

- Production `answer_source` distinguishes carrier, profile, document, operational, buyer review, external, and AI-pending sources — **not** preview test data.
- Preview data is isolated (`PREVIEW_DATA_ONLY=YES`); `BUYER_PREVIEW_TEST` must never appear on production `Answer` rows.
- Company authority is server-verified (`CLIENT_SUPPLIED_COMPANY_AUTHORITY=FORBIDDEN`).
- 422 responses must not expose stack traces.
- Tenant/membership scoping unchanged from platform baseline.
- `SUBMIT_WITH_ERRORS=FORBIDDEN`, `PUBLISH_WITH_ERRORS=FORBIDDEN`.

---

## Audit

Persist on accepted answers: value, version, source, actor, timestamps, validation version; plus rule/score versions when qualification-relevant.

Knockout answers are evidence — must be persisted, not blocked.

---

## Concurrency

- Optimistic concurrency via `save_version` / `ResponseVersion`.
- Stale write → `409`; client reloads last valid server state.
- Atomic batch commit prevents torn autosave revisions.

---

## Offline / local recovery boundaries

Optional browser-local recovery is permitted only when:

```
LOCAL_RECOVERY_IS_NOT_AUTHORITATIVE = YES
```

Restored values must re-pass server validation before becoming `Answer` records.

---

## Compliance with architecture flags

This ADR satisfies mandatory flags in validation contract §22, including:

`INVALID_DATA_PERSISTENCE=FORBIDDEN`, `SERVER_VALIDATION_REQUIRED=YES`, `SAVE_ONLY_VALID_STATE=YES`, `KNOCKOUT_BLOCKS_SAVE=NO`, `PREVIEW_AS_CARRIER=YES`.

---

## References

- [RFX_V3_DOMAIN_MODEL.md](../RFX_V3_DOMAIN_MODEL.md)
- [RFX_V3_API.md](../RFX_V3_API.md)
- [RFX_V3_QUESTIONNAIRE_ENGINE.md](../RFX_V3_QUESTIONNAIRE_ENGINE.md)
- [RFX_V3_UX.md](../RFX_V3_UX.md)
- [RFX_V3_SECURITY.md](../RFX_V3_SECURITY.md)
- [RFX_V3_ROADMAP.md](../RFX_V3_ROADMAP.md)
