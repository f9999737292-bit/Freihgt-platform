# ADR-RFX-003: Conditional Rule Engine

**Status:** Accepted (architecture draft)  
**Date:** 2026-09-03

## Context

Enterprise questionnaires require visibility, requiredness, validation, and knockout rules dependent on other answers. Low-code custom fields provide JSON rules but are not integrated with RFx carrier responses.

## Decision

Implement a **native RFx rule engine** in `rfx-service` with versioned `rfx_question_rules`. Rule types: `VISIBILITY`, `REQUIRED`, `VALIDATION`, `KNOCKOUT`. Rules evaluated server-side on every save and submit.

## Consequences

- Rule evaluation is deterministic and replayable via `rule_version`.
- Client may preview rules for UX; server is authoritative.
- Conditional logic tested in preview sandbox without production data.

## References

- [RFX_V3_QUESTIONNAIRE_ENGINE.md](../RFX_V3_QUESTIONNAIRE_ENGINE.md)
- Low-code precedent: `infrastructure/migrations/000011_create_lowcode_custom_fields_v0.1.up.sql`
