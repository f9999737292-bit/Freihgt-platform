# ADR-RFX-005: Qualification Model

**Status:** Accepted (architecture draft)  
**Date:** 2026-09-03

## Context

RFI workflows require carrier qualification states beyond simple response submit: qualified, conditionally qualified, rejected with evidence.

## Decision

Introduce **`QualificationResult`** aggregate with statuses `QUALIFIED`, `CONDITIONALLY_QUALIFIED`, `REJECTED`, `PENDING_REVIEW`. Qualification derives from scoring + knockout rules on valid persisted answers. Optional **`QualificationPool`** for reusable prequalified carrier sets.

## Consequences

- Knockout answers persisted as audit evidence.
- Pool membership updated via `rfx.carrier.qualified` events.
- Re-qualification on material RFx version change.

## References

- [RFX_V3_SCORING_ENGINE.md](../RFX_V3_SCORING_ENGINE.md) §8
- [RFX_V3_DATA_MODEL.md](../RFX_V3_DATA_MODEL.md)
