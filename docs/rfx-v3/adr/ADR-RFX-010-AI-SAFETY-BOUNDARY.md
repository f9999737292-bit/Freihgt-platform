# ADR-RFX-010: AI Safety Boundary

**Status:** Accepted (architecture draft)  
**Date:** 2026-09-03

## Context

AI assistance can accelerate questionnaire design and document extraction but introduces risk of autonomous decisions affecting tender fairness and auditability.

## Decision

AI is **assistive only**. Mandatory prohibitions:

```text
AI_AUTO_PUBLISH=NO
AI_AUTO_INVITE=NO
AI_SILENT_SCORE_CHANGE=NO
AI_AUTO_REJECT_WITHOUT_DETERMINISTIC_AUTHORITY=NO
```

All AI-derived values carry `MODEL_VERSION`, `CONFIDENCE`, `REVIEW_STATE`. Pending AI values use `AI_EXTRACTED_PENDING_REVIEW` until confirmed.

## Consequences

- Human gates on publish, invite, score finalization, rejection.
- AI explanation narratives are read-only views over deterministic scoring.
- Tenant-scoped AI audit log required before production enablement.

## References

- [RFX_V3_AI.md](../RFX_V3_AI.md)
- [ADR-RFX-007](./ADR-RFX-007-ANSWER-PROVENANCE.md)
