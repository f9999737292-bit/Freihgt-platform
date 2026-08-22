# Freight Cost Planned vs Actual / Variance v2.1C — Implementation Report

**Status:** RUNTIME COMPLETION (draft PR — adversarial remediation gate)  
**Planning base:** PR #47 merged @ `db5c0c793a7259bccc4b0c389f3b9e3b23f73a2f`  
**PR #48 base SHA:** `db5c0c793a7259bccc4b0c389f3b9e3b23f73a2f`  
**Contract:** `docs/engineering/FREIGHT_COST_VARIANCE_RECONCILIATION_v2.1C_IMPLEMENTATION_PLAN.md`

## Summary

v2.1C runtime adds derived variance/forecast projection fields, charge-code classification, variance attribution (driver vs availability), reconciliation findings, and internal S2S operations — without public API, frontend, FX, or ledger canonical writers.

## Migrations

| Migration | Purpose |
|-----------|---------|
| `000057_freight_cost_variance_projection_v2.1C` | variance + forecast columns on `cost_summary_projection` |
| `000058_freight_cost_variance_explainability_v2.1C` | `charge_code_mapping`, `variance_attribution`, `reconciliation_finding` + platform seed |
| `000059_freight_cost_variance_remediation_v2.1C` | `forecast_source_status`, `derived_state_fingerprint`, mapping overlap exclusion, global mapping version sequence |

## Remediation (H-001 … H-009)

| ID | Issue | Resolution |
|----|-------|------------|
| H-001 | Active mapping SQL precedence / missing `effective_from` | Explicit predicate with evaluation timestamp; tenant + platform branches parenthesized |
| H-002 | Overlapping active mapping windows | PostgreSQL `EXCLUDE` constraint on `(scope, tenant, source_key, tstzrange)` |
| H-003 | Pinned mapping version cosmetic only | `LoadPinnedMappings` selects rules with `mapping_version <= pin` at evaluation time |
| H-004 | Attribution idempotency broken by unconditional revision bump | `derived_state_fingerprint` — revision advances only when canonical inputs change |
| H-005 | Finding ID included mutable revisions | **Contract erratum:** identity = `tenant \| transport_order \| kind \| canonical_reference_key` |
| H-006 | UNKNOWN proposed served stale forecast | `forecast_exposure = NULL`, `forecast_source_status = UNKNOWN` |
| H-007 | Reconciliation not executable | `POST .../transport-orders/{id}/reconcile` (S2S, detection-only) |
| H-008 | Reclassification missing | `POST .../transport-orders/{id}/reclassify-attribution` |
| H-009 | Mapping management missing | `PUT /internal/v1/freight-cost/charge-code-mappings` |

### Contract erratum — reconciliation finding identity (H-005)

The merged plan §16.2 originally hashed `expected_revision | observed_revision` into `finding_id`, but lifecycle semantics require the same logical drift to retain one row when observed revision advances. **Normative identity:**

```text
finding_id = UUID_SHA1(Namespace, tenant_id | transport_order_id | finding_kind | canonical_reference_key)
```

`expected_revision` / `observed_revision` are mutable observation fields updated in place on repeated scans.

### Mapping rule-set identity (H-003 / §13)

Global monotonic `freight_cost.charge_code_mapping_version_seq` assigns each new mapping row a unique version. Standard rebuild pins `attribution_mapping_version` on the projection; `LoadPinnedMappings(tenant, pin, evalTime)` reconstructs the effective combined PLATFORM+TENANT rule set (`mapping_version <= pin`, tenant wins per source key). **PINNED_MAPPING_RULESET_RECONSTRUCTABLE=YES** — no separate platform/tenant pair required because versions share one sequence and selection is deterministic.

## Domain

- `CalculateCurrentVariance` / `CalculateFinalVariance` — EX-VAT, NULL-safe, currency-compatible
- `RecomputeDerivedProjection` — variance, percent, forecast; returns `stateChanged bool`
- `ComputeDerivedStateFingerprint` / `ApplyDerivedStateRevision` — idempotent projection revision
- `BuildVarianceDrivers` / `BuildAvailabilityReasons` — separated semantic classes; attribution fact ID uses `StateFingerprint`
- `DetectReconciliationFindings` — MISSING_PLANNED, MISSING_ACCRUAL, MISSING_FINAL_ACTUAL, BILLING_LINK_MISMATCH, ORPHAN_BILLING_LINK
- `ResolveChargeCategory` — PLATFORM (`tenant_id IS NULL`) + TENANT override

### Reconciliation finding coverage

| Kind | Status |
|------|--------|
| PROJECTION_DRIFT | DEFERRED_BY_FROZEN_PLAN — requires projection snapshot diff worker |
| STALE_CURSOR | DEFERRED_BY_FROZEN_PLAN — requires cursor audit API |
| MISSING_PLANNED_FACT | IMPLEMENTED |
| MISSING_ACCRUAL_FACT | IMPLEMENTED |
| MISSING_FINAL_ACTUAL | IMPLEMENTED |
| ORPHAN_BILLING_LINK | IMPLEMENTED |
| ORPHAN_PAYMENT_LINK | NOT_APPLICABLE — payment linkage not on projection in v2.1B |
| CURRENCY_DRIFT | DEFERRED — no canonical cross-source currency audit API |
| DUPLICATE_ECONOMIC_FACT | NOT_APPLICABLE — prevented by v2.1B `UNIQUE(tenant_id, source_fact_id)` |
| BILLING_LINK_MISMATCH | IMPLEMENTED |

## Internal API (S2S)

| Method | Route | Purpose |
|--------|-------|---------|
| POST | `/internal/v1/freight-cost/transport-orders/{id}/reconcile` | On-demand reconciliation scan |
| POST | `/internal/v1/freight-cost/transport-orders/{id}/reclassify-attribution` | Analytic reclassification (current mappings) |
| PUT | `/internal/v1/freight-cost/charge-code-mappings` | Mapping administration |

Existing rebuild: `POST .../rebuild` (unchanged, financial rebuild with pinned mapping).

**RECONCILIATION_AUTO_REBUILD=NO · RECONCILIATION_AUTO_REPAIR=NO**

## Services

- `DerivedProjectionService` — idempotent recompute in ingest tx; pinned mapping on standard rebuild; post-commit forecast enrich
- `RebuildService` — canonical rebuild + forecast enrich from settlement proposed total

## Billing internal read

Extended `GET /internal/v1/freight-settlements/by-transport-order/{id}`:

- `proposed_accessorial_total_ex_vat` (decimal string)
- `proposed_accessorial_source_status` (`KNOWN` | `UNKNOWN`)

## Security / finance audit

- `FLOAT64_MONEY_ON_FREIGHT_COST_BOUNDARY=0`
- `CROSS_SERVICE_DB_READS_FROM_FREIGHT_COST=0`
- Carrier mask preserved (`view_scope.go`)
- No public API / no frontend

## Test matrix

Machine-readable inventory: `docs/engineering/FC_C_TEST_INVENTORY.json`

| Family | Count | Level |
|--------|------:|-------|
| FC-C-VAR | 12 | domain |
| FC-C-REA | 16 | domain |
| FC-C-CHG | 10 | domain + integration |
| FC-C-DUP | 4 | domain |
| FC-C-FOR | 8 | domain |
| FC-C-REC | 10 | domain + integration |
| FC-C-RBL | 8 | domain + integration |
| FC-C-MON | 4 | domain |
| FC-C-SEC | 8 | domain + integration |
| FC-C-OUT | 4 | domain + integration |
| **Total** | **84** | |

CI job `freight-cost-ledger-integration` runs `./internal/integration/variance/...` alongside ledger tests.

## Deferred (not v2.1C)

- Scheduled reconciliation worker (`V2_1C_RECONCILIATION_JOB_ENABLED`, default OFF)
- PROJECTION_DRIFT / STALE_CURSOR / CURRENCY_DRIFT finding kinds
- v2.1D frontend workspace
- v2.1E public `/api/v1/freight-cost/*`
