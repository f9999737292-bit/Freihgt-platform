# ADR-RFX-007: Answer Provenance

**Status:** Accepted (architecture draft)  
**Date:** 2026-09-03

## Context

Qualification audit requires knowing whether an answer came from carrier entry, profile autofill, document extraction, operational KPI, buyer review, external verification, or AI pending review.

## Decision

Every production **`Answer`** records authoritative `answer_source` from closed enum:

`CARRIER_DECLARED`, `CARRIER_PROFILE`, `DOCUMENT_VERIFIED`, `BINTRANS_OPERATIONAL_DATA`, `BUYER_REVIEW`, `EXTERNAL_VERIFICATION`, `AI_EXTRACTED_PENDING_REVIEW`.

**`BUYER_PREVIEW_TEST` is forbidden** on production Answer rows. Preview data uses isolated storage only.

## Consequences

- Scoring explainability references source.
- AI-extracted values remain pending until human confirms.
- Preview cannot contaminate audit trail.

## References

- [RFX_V3_DOMAIN_MODEL.md](../RFX_V3_DOMAIN_MODEL.md) §4.4
- [RFX_V3_SECURITY.md](../RFX_V3_SECURITY.md) §2–3
- [ADR-RFX-011](./ADR-RFX-011-RESPONSE-VALIDATION-AND-DRAFT-SAFETY.md)
