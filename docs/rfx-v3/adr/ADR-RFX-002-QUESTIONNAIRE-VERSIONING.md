# ADR-RFX-002: Questionnaire Versioning

**Status:** Accepted (architecture draft)  
**Date:** 2026-09-03

## Context

Published RFx questionnaires must be immutable historical records. Material edits after publication affect in-flight carrier responses.

## Decision

Bind questionnaire definition to **`RfxVersion`** rows. Published versions are immutable. Material changes create a new version with `ChangeImpactAnalysis` (reconfirmation, rescoring flags).

## Alternatives

- In-place edit of published questionnaire — **rejected** (breaks audit and carrier fairness).

## Consequences

- `rfx_sections`, `rfx_questions` scoped to `rfx_version_id`.
- Responses reference `rfx_version_id` they were submitted against.
- Compare/restore draft capabilities required for buyer UX.

## References

- [RFX_V3_DOMAIN_MODEL.md](../RFX_V3_DOMAIN_MODEL.md) §7
- [RFX_V3_DATA_MODEL.md](../RFX_V3_DATA_MODEL.md)
