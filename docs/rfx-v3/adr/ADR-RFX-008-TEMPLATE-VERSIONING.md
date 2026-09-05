# ADR-RFX-008: Template Versioning

**Status:** Accepted (architecture draft)  
**Date:** 2026-09-03

## Context

Buyers repeat similar RFx structures (RFI HSE, lane tender, seasonal). Low-code form templates exist but are not RFx-native.

## Decision

Introduce **`RfxTemplate`** + **`RfxTemplateVersion`** with immutable published template snapshots. Clone-from-template creates new `RfxEvent` draft linked to template provenance.

Template versioning independent from event versioning but uses same section/question schema.

## Consequences

- Template library per tenant (optional per owner company).
- Template changes do not retroactively alter published RFx events.
- AI template recommendations reference template catalogue.

## References

- [RFX_V3_DATA_MODEL.md](../RFX_V3_DATA_MODEL.md) §3.1
