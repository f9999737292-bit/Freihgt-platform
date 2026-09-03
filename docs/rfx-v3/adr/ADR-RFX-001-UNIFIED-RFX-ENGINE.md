# ADR-RFX-001: Unified RFx Engine

**Status:** Accepted (architecture draft)  
**Date:** 2026-09-03

## Context

BINTRANS has parallel RFx paths: enterprise `rfx_events` and spot `freight_requests`/`bids`. v3.0A requires a unified questionnaire-driven engine without replacing the gateway boundary.

## Decision

Extend `rfx-service` as the **single RFx domain service** for enterprise RFI/RFQ/RFP with shared questionnaire, response, scoring, and qualification models. Spot freight remains a profile/simplified questionnaire — not a separate product silo.

## Consequences

- Shared validation, autosave, and audit semantics across RFx types.
- Migration path from v1 commercial-only responses to full questionnaire answers.
- OpenAPI must be brought to parity with router surface.

## References

- [RFX_V3_GAP_MATRIX.md](../RFX_V3_GAP_MATRIX.md)
- `services/rfx-service/internal/http/router.go`
