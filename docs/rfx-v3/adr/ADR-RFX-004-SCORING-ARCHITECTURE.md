# ADR-RFX-004: Scoring Architecture

**Status:** Accepted (architecture draft)  
**Date:** 2026-09-03

## Context

v1 uses fixed commercial 70% / manual 30% weighting. v3 requires configurable scoring with knockout, explainability, and valid/invalid answer distinction.

## Decision

Introduce versioned **`ScoreModel`** with modes `AUTOMATIC`, `MANUAL`, `HYBRID`, `SYSTEM_DERIVED`. Scoring consumes persisted valid answers only. Every result includes explainability payload.

**Invariant:** `VALIDATION_ERROR ≠ BUSINESS_FAILURE`.

## Consequences

- Replace hardcoded evaluation formula with configurable criteria.
- Knockout is business outcome, not validation error (`KNOCKOUT_BLOCKS_SAVE=NO`).
- Manual overrides audited separately from answer values.

## References

- [RFX_V3_SCORING_ENGINE.md](../RFX_V3_SCORING_ENGINE.md)
- `services/rfx-service/internal/domain/rfx_evaluation.go` (current v1)
