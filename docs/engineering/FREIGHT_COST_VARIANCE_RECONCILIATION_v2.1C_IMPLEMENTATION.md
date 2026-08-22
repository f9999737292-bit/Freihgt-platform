# Freight Cost Planned vs Actual / Variance v2.1C — Implementation Report

**Status:** RUNTIME COMPLETION (draft PR — F48 independent closure gate)  
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
| `000060_freight_cost_mapping_evaluated_at_v2.1C` | `attribution_mapping_evaluated_at` on `cost_summary_projection` |

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

## F48 Independent Closure (F48-001 … F48-008)

| ID | Severity | Issue | Resolution |
|----|----------|-------|------------|
| F48-001 | HIGH | Pinned rebuild filtered expired rules via current `effective_from/effective_to` | **Split loaders:** `LoadActiveMappings(tenant, evalTime)` applies effective window; `LoadPinnedMappings(tenant, pin)` selects `mapping_version <= pin` only — **time-independent historical ruleset reconstruction** |
| F48-002 | HIGH | `X-Freight-Cost-Platform-Admin: true` self-asserted privilege | **Fail closed:** `PLATFORM_MAPPING_MUTATION_ENABLED=NO`; unsigned header removed; tenant-scoped PUT only with verified `X-Tenant-ID` |
| F48-003 | HIGH | Reclassify accepted caller financial evidence | **Intent-only API:** empty body; hydrate `approved_accessorials[]` + `base_freight_amount` from billing-register + transport internal reads |
| F48-004 | MED/HIGH | Reconciliation local-projection-only | **`detectCanonicalReconciliationFindings`:** read-only compare stored projection vs canonical transport snapshot + billing settlement/link + source cursors; no auto-rebuild/repair |
| F48-005 | HIGH GATE | Remediation tests outside frozen 84 | **25 bonus tests** in `BONUS_REMEDIATION_TEST_INVENTORY.json` (excluded from FC-C count) |
| F48-006 | MED | Mapping window / category validation | Reject `effective_to <= effective_from`; validate target category against frozen vocabulary before DB |
| F48-007 | MED | Reclassify pin semantics | **`RECLASSIFICATION_UPDATES_PINNED_RULESET=YES`:** successful reclassify sets `attribution_mapping_version` to current active mapping version; subsequent standard rebuild reproduces reclassification |
| F48-008 | MED | Legacy NULL `derived_state_fingerprint` | Bootstrap via `ComputeDerivedStateFingerprint` with `ProposedSourceUnknown` only — **never derive proposed total from `forecast_exposure`** |
| F48-010 | HIGH | Version-only pinned loader leaked future/expired rules | Pin **version boundary + evaluation timestamp**; `LoadPinnedMappings(tenant, pinVersion, pinEvalTime)` applies historical effective window |

### Pinned mapping identity (F48-010)

**PINNED_MAPPING_IDENTITY = VERSION_BOUNDARY + EVALUATION_TIMESTAMP**

Projection fields:
- `attribution_mapping_version` — global mapping version boundary at evaluation
- `attribution_mapping_evaluated_at` — trusted evaluation instant frozen on pin

| Operation | Loader | Clock used |
|-----------|--------|------------|
| Initial compute | `LoadActiveMappings(tenant, evalTime)` | Current service time (first pin) |
| Standard rebuild | `LoadPinnedMappings(tenant, pinVersion, pinEvalTime)` | **Pinned** evaluation timestamp only |
| Explicit reclassify | `LoadActiveMappings(tenant, now)` | Current time; updates both pin fields |
| Legacy upgrade (version set, time NULL) | `LoadActiveMappings` bootstrap once | Current time — explicit normalization, not historical claim |

**LoadPinnedMappings SQL semantics:**

```text
mapping_version <= pinnedVersion
AND effective_from <= pinnedEvaluationTime
AND (effective_to IS NULL OR effective_to > pinnedEvaluationTime)
```

Then TENANT-over-PLATFORM precedence per source key.

**PINNED_MAPPING_REBUILD_TIME_INDEPENDENT=YES** — independent of current wall-clock, evaluated against persisted pin timestamp.

**STANDARD_REBUILD_USES_CURRENT_TIME_FOR_MAPPING=NO**

**LEGACY_MAPPING_TIME_PIN_BOOTSTRAP=EXPLICIT** — `EnsureLegacyDerivedStateBootstrapped` re-pins from current active mappings when `attribution_mapping_evaluated_at IS NULL`.

**LEGACY_PIN_USES_WINDOWLESS_RECONSTRUCTION=NO**

**RECLASSIFICATION_UPDATES_MAPPING_VERSION_PIN=YES**

**RECLASSIFICATION_UPDATES_MAPPING_TIME_PIN=YES**

**STANDARD_REBUILD_AFTER_RECLASSIFICATION_REPRODUCES_RECLASSIFICATION=YES**

### Platform admin authorization (F48-002)

No cryptographically verified platform-admin actor exists in freight-cost internal auth today. Repository and HTTP handler reject `mapping_scope=PLATFORM` mutations.

**SELF_ASSERTED_PLATFORM_ADMIN_HEADER=DENY**  
**PLATFORM_MAPPING_MUTATION_ENABLED=NO**

Tenant mapping remains: verified tenant only; cross-tenant deny.

### Canonical reclassification (F48-003)

`POST /internal/v1/freight-cost/transport-orders/{id}/reclassify-attribution` — no financial fact payload.

**RECLASSIFICATION_FINANCIAL_EVIDENCE_SOURCE=CANONICAL_INTERNAL_APIS**  
**CLIENT_SUPPLIED_ACCESSORIAL_AMOUNT_TRUSTED=NO**  
**CLIENT_SUPPLIED_BASE_FREIGHT_TRUSTED=NO**

Billing internal read extended with `approved_accessorials[]` and `base_freight_amount` (decimal strings).

### Reconciliation finding coverage (F48-004)

| Kind | Status | Notes |
|------|--------|-------|
| PROJECTION_DRIFT | **IMPLEMENTED_CANONICALLY** | Compare stored projection amounts vs canonical transport snapshot + billing settlement derived amounts |
| STALE_CURSOR | **IMPLEMENTED_CANONICALLY** | Compare `source_cursor.last_source_revision` vs canonical settlement version |
| MISSING_PLANNED_FACT | **IMPLEMENTED_CANONICALLY** | Snapshot absent with stored planned present |
| MISSING_ACCRUAL_FACT | **IMPLEMENTED_CANONICALLY** | Planned present, settlement accrual present, projection accrual absent |
| MISSING_FINAL_ACTUAL | **IMPLEMENTED_CANONICALLY** | Ready-for-payment settlement without final actual on projection |
| ORPHAN_BILLING_LINK | **IMPLEMENTED_CANONICALLY** | Canonical unlinked vs projection linked |
| ORPHAN_PAYMENT_LINK | **NOT_DETECTABLE_WITH_CURRENT_CANONICAL_DATA** | No payment linkage canonical read on projection in v2.1B/v2.1C |
| CURRENCY_DRIFT | **IMPLEMENTED_CANONICALLY** | Projection currency vs canonical settlement/snapshot currency |
| DUPLICATE_ECONOMIC_FACT | **NOT_APPLICABLE** | Prevented by v2.1B `UNIQUE(tenant_id, source_fact_id)` |
| BILLING_LINK_MISMATCH | **IMPLEMENTED_CANONICALLY** | Canonical link state vs projection billing fields |

**RECONCILIATION_READ_ONLY=YES**  
**RECONCILIATION_CANONICAL_SOURCE_READ=YES**  
**AUTO_REBUILD=NO**  
**AUTO_REPAIR=NO**

Local-only `DetectReconciliationFindings(projection)` retained for domain unit tests (REA-012); runtime reconcile uses canonical path.

### Contract erratum — reconciliation finding identity (H-005)

```text
finding_id = UUID_SHA1(Namespace, tenant_id | transport_order_id | finding_kind | canonical_reference_key)
```

`expected_revision` / `observed_revision` are mutable observation fields updated in place on repeated scans.

## Domain

- `CalculateCurrentVariance` / `CalculateFinalVariance` — EX-VAT, NULL-safe, currency-compatible
- `RecomputeDerivedProjection` — variance, percent, forecast; returns `stateChanged bool`
- `ComputeDerivedStateFingerprint` / `ApplyDerivedStateRevision` — idempotent projection revision
- `BuildVarianceDrivers` / `BuildAvailabilityReasons` — separated semantic classes; attribution fact ID uses `StateFingerprint`
- `ValidateMappingCategory` / window validation on upsert
- `ResolveChargeCategory` — PLATFORM (`tenant_id IS NULL`) + TENANT override

## Internal API (S2S)

| Method | Route | Purpose |
|--------|-------|---------|
| POST | `/internal/v1/freight-cost/transport-orders/{id}/reconcile` | On-demand canonical reconciliation scan (read-only) |
| POST | `/internal/v1/freight-cost/transport-orders/{id}/reclassify-attribution` | Analytic reclassification — intent only, canonical hydration |
| PUT | `/internal/v1/freight-cost/charge-code-mappings` | Tenant mapping administration (platform denied) |

Existing rebuild: `POST .../rebuild` (unchanged, financial rebuild with pinned mapping).

## Services

- `DerivedProjectionService` — idempotent recompute; pinned vs active mapping loaders; canonical reconcile/reclassify
- `detectCanonicalReconciliationFindings` — in-memory expected vs stored compare
- `RebuildService` — canonical rebuild + forecast enrich from settlement proposed total

## Billing internal read

Extended `GET /internal/v1/freight-settlements/by-transport-order/{id}`:

- `proposed_accessorial_total_ex_vat` (decimal string)
- `proposed_accessorial_source_status` (`KNOWN` | `UNKNOWN`)
- `approved_accessorials[]` — `{accessorial_id, charge_code, amount_ex_vat}`
- `base_freight_amount` (decimal string, optional)

## Security / finance audit

- `FLOAT64_MONEY_ON_FREIGHT_COST_BOUNDARY=0`
- `CROSS_SERVICE_DB_READS_FROM_FREIGHT_COST=0`
- Carrier mask preserved (`view_scope.go`)
- No unsigned platform-admin header
- No caller-supplied reclassification financial evidence
- No public API / no frontend

## Test matrix

Machine-readable inventories:

- Frozen FC-C: `docs/engineering/FC_C_TEST_INVENTORY.json` (**84 tests — unchanged**)
- Bonus remediation: `services/freight-cost-service/docs/engineering/BONUS_REMEDIATION_TEST_INVENTORY.json` (**25 tests**)

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
| **Frozen Total** | **84** | |
| **Bonus Remediation** | **25** | domain + integration |

CI job `freight-cost-ledger-integration` runs `./internal/integration/variance/...` (frozen + bonus) alongside ledger tests.

## Deferred (not v2.1C)

- Scheduled reconciliation worker (`V2_1C_RECONCILIATION_JOB_ENABLED`, default OFF)
- Platform mapping mutation (await verified platform-admin identity architecture)
- ORPHAN_PAYMENT_LINK detection (await payment canonical linkage read)
- v2.1D frontend workspace
- v2.1E public `/api/v1/freight-cost/*`
