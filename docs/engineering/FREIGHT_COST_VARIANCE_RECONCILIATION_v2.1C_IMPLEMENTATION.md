# Freight Cost Planned vs Actual / Variance v2.1C — Implementation Report

**Status:** RUNTIME IMPLEMENTATION (draft PR)  
**Planning base:** PR #47 merged @ `db5c0c793a7259bccc4b0c389f3b3d0865a593f`  
**Contract:** `docs/engineering/FREIGHT_COST_VARIANCE_RECONCILIATION_v2.1C_IMPLEMENTATION_PLAN.md`

## Summary

v2.1C runtime adds derived variance/forecast projection fields, charge-code classification, variance attribution (driver vs availability), and reconciliation findings — without public API, frontend, FX, or ledger canonical writers.

## Migrations

| Migration | Purpose |
|-----------|---------|
| `000057_freight_cost_variance_projection_v2.1C` | variance + forecast columns on `cost_summary_projection` |
| `000058_freight_cost_variance_explainability_v2.1C` | `charge_code_mapping`, `variance_attribution`, `reconciliation_finding` |

## Domain

- `CalculateCurrentVariance` / `CalculateFinalVariance` — EX-VAT, NULL-safe, currency-compatible
- `RecomputeDerivedProjection` — variance, percent, forecast (frozen formula)
- `BuildVarianceDrivers` / `BuildAvailabilityReasons` — separated semantic classes
- `DetectReconciliationFindings` — billing link mismatch, missing planned
- `ResolveChargeCategory` — PLATFORM (`tenant_id IS NULL`) + TENANT override

## Services

- `DerivedProjectionService` — recompute in ingest tx; post-commit forecast enrich via billing internal read
- `RebuildService` — unchanged canonical rebuild + forecast enrich from settlement proposed total

## Billing internal read

Extended `GET /internal/v1/freight-settlements/by-transport-order/{id}`:

- `proposed_accessorial_total_ex_vat` (decimal string)
- `proposed_accessorial_source_status` (`KNOWN`)

## Security

- Carrier mask preserved (`view_scope.go`)
- No public API / no frontend
- No cross-service DB reads from freight-cost

## Test inventory (in progress)

| Family | Unit | Integration | Notes |
|--------|-----:|------------:|-------|
| FC-C-VAR | 5 | pending | formula/NULL/currency |
| FC-C-FOR | 2 | pending | known-empty, unknown source |
| FC-C-REA | 1 | pending | FUEL false-positive guard |
| FC-C v2.1B regression | — | existing ledger/* | unchanged |

**Frozen total:** 84 — full matrix completion tracked in follow-up commits.

## Known limitations

- Reconciliation scheduled worker: detection logic in service; background job feature-flag deferred
- Analytic reclassification endpoint: planned, not yet exposed
- Full FC-C integration suite: in progress

## Deferred

- v2.1D frontend workspace
- v2.1E public `/api/v1/freight-cost/*`
