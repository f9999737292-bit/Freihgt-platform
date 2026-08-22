# Freight Cost Planned vs Actual / Variance v2.1C — Implementation Plan & Contract Freeze

**Status:** PLANNING / CONTRACT FREEZE (no runtime in this document)  
**Official slice name (architecture §46):** **v2.1C — Planned vs Actual / Variance**  
**Base SHA (post-PR #46 merge):** `75994efecb5c96bf6608f891fad3b3d0865a593f`  
**Merged v2.1B feature HEAD:** `5fc2fa3c1dba5853ebc15a5f43623f964c2371d4`  
**PR #46 merge commit:** `75994efecb5c96bf6608f891fad3b3d0865a593f`
**Review gate:** PR #47 independent architecture + financial semantics review (R-001…R-008 closed)

**Architecture baselines:**
- `docs/engineering/FREIGHT_COST_MANAGEMENT_v2.1_ARCHITECTURE.md`
- `docs/architecture/FREIGHT_COST_MANAGEMENT_v2.1_FINAL_REVIEW.md`
- `docs/engineering/FREIGHT_COST_ACCRUAL_LEDGER_v2.1B_IMPLEMENTATION_PLAN.md`
- `docs/engineering/FREIGHT_COST_ACCRUAL_LEDGER_v2.1B_IMPLEMENTATION.md`

**Purpose:** Freeze v2.1C contracts so subsequent implementation is mechanical. **Do not implement runtime in the planning PR.**

---

## 1. Executive summary

v2.1B delivered the derived append-only cost ledger, accrual/actual/billed/paid projection persistence, billing/payment outbox integration, and canonical rebuild. v2.1C adds **derived planned-vs-actual variance**, **three separated explainability classes** (variance driver / availability reason / reconciliation finding), **charge_code semantic classification**, **forecast exposure projection** (non-ledger KPI), and **reconciliation drift detection** — all within `freight-cost-service` as derived projections over existing canonical facts.

v2.1C **does not** add public API, frontend, FX, or ledger canonical writers. All money remains decimal-safe; NULL never becomes zero.

---

## 2. Baseline and merged v2.1B SHA

| Item | SHA / evidence |
|------|----------------|
| Post-merge `main` | `75994efecb5c96bf6608f891fad3b3d0865a593f` |
| v2.1B feature tip | `5fc2fa3c1dba5853ebc15a5f43623f964c2371d4` |
| PR #46 | MERGED 2026-08-22T15:41:15Z |
| Post-merge main CI | run `32582445160` — **success** @ `75994ef` |
| Migrations through | `000056_payment_outbox_aggregate_version_v2.1B` |
| FC-B integration tests | 57 + accrual semantics regressions |

---

## 3. Discovery findings

### 3.1 Deferred from v2.1B → v2.1C (frozen architecture + v2.1B plan §5)

| Capability | Source |
|------------|--------|
| `current_variance_amount` / `final_variance_amount` | Architecture §19; v2.1B plan §18 projection defer |
| Variance reason attribution | Architecture §20; v2.1B plan §5 |
| `charge_code` semantic classification | Architecture §20, OQ-005; v2.1B plan §5 |
| `forecast_exposure` persistence | Architecture OQ-001; v2.1B plan §11 |
| Reconciliation background / drift automation | v2.1B plan §45 handoff |
| Double-count prevention via classification | Architecture §46 v2.1C bullet |

### 3.2 Deferred to v2.1D / v2.1E — MUST NOT leak into v2.1C

| Capability | Target slice | Evidence |
|------------|--------------|----------|
| Cost analytics workspace / UI | v2.1D | Architecture §46, §24 workspace table |
| Buyer/carrier analytics screens | v2.1D | Architecture §46 |
| Public `/api/v1/freight-costs/*` | v2.1E | Architecture §24, §46 |
| Gateway RBAC permissions (`freight_cost.read`) | v2.1E | Architecture §24 |
| Public E2E hardening | v2.1E | Architecture §46 |
| Feature-flagged workspace rollout | v2.1D | Architecture §46 |

### 3.3 Current implementation state (post-v2.1B)

| Area | State |
|------|-------|
| `cost_summary_projection` DB table | Has planned/accrual/actual/billed/paid; **no variance/forecast columns** |
| Domain `CostSummary` | Has `CurrentVarianceAmount`, `FinalVarianceAmount`, `ForecastExposure` fields — **not populated from projection** |
| Internal DTO | Serializes variance/forecast as nullable decimal strings — **always null today** |
| `CalculateForecastExposure` | Pure function exists (`domain/accrual.go`) — **not wired to projection** |
| `DetermineBillingReconciliation` | **Implemented** in v2.1B (`domain/reconciliation.go`) |
| Variance pure functions | **NOT_FOUND** — v2.1C must add |
| `charge_code` | `billing.settlement_accessorials.charge_code VARCHAR(50) NOT NULL` — **free text**, validated non-empty only |
| Public freight-cost API | **NOT_FOUND** in api-gateway |
| Freight-cost frontend workspace | **NOT_FOUND** in web-admin (procurement has settlement UI only) |
| Dedicated reconciliation worker | **NOT_FOUND** — outbox workers exist in billing/payment/shipment only |
| Cross-service DB reads from freight-cost | **0** (HTTP clients only) — verified v2.1B |

### 3.4 Canonical facts available today

| Fact | Owner | Table/Aggregate | Immutable? | Revisioned? |
|------|-------|-----------------|------------|-------------|
| Planned cost | transport-order-service | `transport.transport_order_rate_snapshots.total_amount` | YES (SNAPSHOT_V1) | IMMUTABLE semantic |
| Settlement base freight | billing-register-service | `freight_settlements.base_freight_amount` | YES after create | N/A |
| Approved accessorial | billing-register-service | `settlement_accessorials` WHERE APPROVED | NO | via settlement version |
| Proposed accessorial | billing-register-service | `settlement_accessorials` WHERE PROPOSED | NO | — |
| Financial accrual | freight-cost-service (derived) | `cost_summary_projection.accrued_amount` | N/A | derived |
| Current actual | billing-register-service | `freight_settlements.total_without_vat` + status/disputes | PARTIAL | settlement.version |
| Final actual | billing-register-service | same; READY_FOR_PAYMENT | PARTIAL | settlement.version |
| Billed line | billing-register-service | frozen at register include | YES | billing_link_revision |
| Paid | payment-service | `payment_obligations.paid_amount` | PARTIAL | obligation.version |
| Ledger entries | freight-cost-service | `freight_cost.cost_entry` | YES append-only | source_revision |

### 3.5 Signals NOT currently available for v2.1C

| Signal | Status |
|--------|--------|
| Normalized charge_code taxonomy | NOT_FOUND — must be introduced in v2.1C |
| Variance driver / availability persistence | NOT_FOUND |
| Forecast exposure DB column | NOT_FOUND |
| Variance amount DB columns | NOT_FOUND |
| Route-change execution cost fact | NOT_FOUND (architecture §20) |
| FX rate / conversion | NOT_FOUND |
| Service-level cost dimension | NOT_AVAILABLE (architecture §22) |

---

## 4. Exact v2.1C scope

### IN_SCOPE_V2_1C

| # | Deliverable |
|---|-------------|
| 1 | Pure domain functions: `CalculateCurrentVariance`, `CalculateFinalVariance`, percent helpers |
| 2 | Persist `current_variance_amount`, `final_variance_amount` on `cost_summary_projection` |
| 3 | Optional persist `current_variance_percent`, `final_variance_percent` (derived, NULL-safe) |
| 4 | Recompute variance on projection update (after ingest/rebuild) |
| 5 | Populate internal cost summary API fields from projection |
| 6 | `forecast_exposure` projection: `planned + SUM(PROPOSED accessorials)` via billing internal read extension |
| 7 | Persist `forecast_exposure` on projection (non-ledger KPI) |
| 8 | Three-class explainability model + persistence (driver / availability / reconciliation finding) |
| 9 | `charge_code` → normalized semantic category mapping (versioned rules table) |
| 10 | Double-count classification guards (analytics-only; no ledger amount mutation) |
| 11 | Reconciliation drift detection job (read-only findings + metrics) |
| 12 | Manual internal rebuild trigger only (reuse v2.1B rebuild route; **no automatic rebuild on finding**) |
| 13 | Test matrix FC-C-* |
| 14 | CI job extension for v2.1C integration suites |

### DEFERRED_V2_1D

- Cost analytics workspace (Vue/Nuxt)
- Aggregated dashboards (carrier/lane performance UI)
- Feature-flagged buyer workspace rollout

### DEFERRED_V2_1E

- Public gateway routes
- `freight_cost.read` / export RBAC
- Public variance list APIs

### FUTURE / NOT_SUPPORTED_BY_CURRENT_CANONICAL_DATA

| Item | Classification |
|------|----------------|
| ROUTE_CHANGE variance reason | NOT_SUPPORTED — no execution cost fact |
| RATE_CHANGE auto attribution | NOT_SUPPORTED — snapshot immutable |
| FX-normalized variance | FUTURE — no FX source |
| ML/AI reason guessing | FORBIDDEN |
| General ledger posting | OUT_OF_V2_1 |

---

## 5. Financial formulas (frozen)

### 5.1 Preserved v2.1B semantics

```text
PLANNED_COST = transport_order_rate_snapshots.total_amount  (EX_VAT)

SETTLEMENT_BASE_FREIGHT_ROLE = IMMUTABLE_COPY_OF_RATE_SNAPSHOT_PRINCIPAL  (SNAPSHOT_V1)

FINANCIAL_ACCRUAL =
  planned_principal + EXACT_NUMERIC_SUM(APPROVED accessorials)
  ≡ base_freight_amount + EXACT_NUMERIC_SUM(APPROVED accessorials)  [proven F-002]

CURRENT_ACTUAL = settlement.total_without_vat
  WHEN status ∈ {APPROVED, DOCUMENTS_READY, READY_FOR_PAYMENT}
  AND open_dispute_count = 0
  ELSE NULL

FINAL_ACTUAL = settlement.total_without_vat
  WHEN status = READY_FOR_PAYMENT AND open_dispute_count = 0
  ELSE NULL
```

### 5.2 Variance (architecture §19 — frozen)

All variance comparisons **EX_VAT only**.

```text
CURRENT_VARIANCE_AMOUNT =
  IF planned_amount IS NOT NULL
     AND current_actual_amount IS NOT NULL
     AND currency_code matches
  THEN current_actual_amount - planned_amount
  ELSE NULL

FINAL_VARIANCE_AMOUNT =
  IF planned_amount IS NOT NULL
     AND final_actual_amount IS NOT NULL
     AND currency_code matches
  THEN final_actual_amount - planned_amount
  ELSE NULL

CURRENT_VARIANCE_PERCENT =
  IF planned_amount > 0 AND current_variance_amount IS NOT NULL
  THEN (current_variance_amount / planned_amount) * 100
  ELSE NULL

FINAL_VARIANCE_PERCENT =
  IF planned_amount > 0 AND final_variance_amount IS NOT NULL
  THEN (final_variance_amount / planned_amount) * 100
  ELSE NULL
```

**Sign:** positive = over plan; negative = under plan (saving).

### 5.3 Forecast exposure (v2.1B plan §11 + architecture OQ-001 — frozen, unchanged)

```text
FORECAST_EXPOSURE = planned_amount + EXACT_NUMERIC_SUM(PROPOSED accessorials)
  WHEN currency matches AND proposed source is KNOWN
  ELSE NULL (fail closed on currency mismatch; NULL on unknown proposed source)

FORECAST_FORMULA_MATCHES_V2_1B_FREEZE=YES
FORECAST_IN_COST_LEDGER=NO
FORECAST_IS_NOT_LEDGER_ACTUAL=YES
FORECAST_NOT_WRITTEN_TO_COST_ENTRY=YES
```

Runtime pure function (already merged v2.1B):

```go
CalculateForecastExposure(planned *Money, proposedAccessorials []Money) (*Money, error)
```

Proposed accessorials **never** affect accrual (v2.1B invariant preserved).

---

## 6. NULL semantics

| Condition | planned | accrual | current_actual | final_actual | variance | forecast |
|-----------|---------|---------|----------------|--------------|----------|----------|
| Pre-ingest | NULL | NULL | NULL | NULL | NULL | NULL |
| Planned only | VALUE | NULL/VALUE | NULL | NULL | NULL | PLANNED if proposed source known-empty |
| Disputed settlement | VALUE | VALUE | **NULL** | NULL | **NULL** | per proposed set |
| Currency mismatch | VALUE | FAIL_CLOSED | — | — | **NULL** | **NULL** |
| Legacy no snapshot | award-base | derived | per settlement | per finality | per formula | per proposed |
| Cancelled order | retained historical | — | NULL | NULL | exclude active aggregates | NULL |
| Proposed source read failed | VALUE | VALUE | per actual rules | per finality | per formula | **NULL** (prior value retained until successful recompute) |

```text
NULL_IS_ZERO=NO
UNKNOWN_USES_NULL=YES
KNOWN_EMPTY_PROPOSED_SET_IS_ZERO=YES
FORECAST_WITH_KNOWN_EMPTY_PROPOSED_SET=PLANNED
UNKNOWN_PROPOSED_SOURCE_IS_ZERO=NO
```

**Known-empty vs unknown proposed set:**

| Case | Billing read result | Forecast behavior |
|------|---------------------|-------------------|
| A — known empty | HTTP 200; `proposed_accessorial_total_ex_vat` present; zero PROPOSED rows | `FORECAST_EXPOSURE = PLANNED` |
| B — unknown | HTTP error / timeout / field absent / ambiguous partial failure | `forecast_exposure = NULL`; retain prior projection value; emit stale metric |
| C — rebuild failure | Non-404 billing error during rebuild | Rebuild fails; projection unchanged (v2.1B `RebuildTransportOrder` convention) |

---

## 7. Currency semantics

```text
CURRENCY_MISMATCH_POLICY=FAIL_CLOSED
CROSS_CURRENCY_VARIANCE=NOT_AVAILABLE
FX_CONVERSION_IN_V2_1C=FORBIDDEN
FX_SYNTHESIS=FORBIDDEN
SINGLE_CURRENCY_PER_TO=YES
```

Evidence: architecture §18 — no FX provider found; v2.1B mixed-currency ingest fails closed (FC-B-ACC-005).

---

## 8. Tax basis semantics

| Fact | Tax basis |
|------|-----------|
| planned, accrual, current/final actual, variance | EX_VAT |
| payable, paid | WITH_VAT (separate fields; **not** compared to variance) |

```text
TAX_BASIS_COMPATIBILITY_POLICY=REQUIRE_EX_VAT_FOR_VARIANCE
MIXED_TAX_BASIS_COMPARE=DENY
```

Variance functions MUST NOT read `payable_amount` or `paid_amount`.

---

## 9. Explainability model — three separated semantic classes (R-001)

**Critical boundary:** variance drivers, availability reasons, and reconciliation findings MUST NOT share one mixed enum/table semantics.

```text
VARIANCE_DRIVER_REQUIRES_NON_NULL_VARIANCE=YES
VARIANCE_AVAILABILITY_REASON_CHANGES_MONEY=NO
RECONCILIATION_FINDING_IS_VARIANCE_DRIVER=NO
MANUAL_REASON_CHANGES_CANONICAL_MONEY=NO
```

### 9.1 Class A — `VARIANCE_DRIVER` (financial explainability)

**Purpose:** Explain a **non-NULL** variance amount.
**Storage:** `freight_cost.variance_attribution` with `semantic_class = VARIANCE_DRIVER`.
**Precondition:** `current_variance_amount IS NOT NULL` OR `final_variance_amount IS NOT NULL` (per `variance_kind`).

| Code | Trigger | Auto? | Delta evidence required |
|------|---------|-------|-------------------------|
| `ACCESSORIAL` | Approved accessorial amount explains part/all of variance | AUTO | YES — approved accessorial row(s) |
| `FUEL` | Approved accessorial with `charge_code` mapped to FUEL category | AUTO partial | YES — approved accessorial delta; **NOT snapshot component alone** |
| `DETENTION` | Approved accessorial mapped to DETENTION | AUTO partial | YES |
| `WAITING` | Approved accessorial mapped to WAITING | AUTO partial | YES |
| `LEGACY_PRICING` | Non-SNAPSHOT_V1 settlement principal differs from planned | AUTO | YES — canonical principal delta |
| `UNATTRIBUTED` | Variance exists; no driver rule matched | AUTO fallback | N/A |
| `MANUAL_ADJUSTMENT` | Operator label when auto rules insufficient | MANUAL | Optional note only |
| `OTHER` | Operator-provided internal label | MANUAL | Optional note only |

**Explicitly NOT variance drivers:**

| Code / concept | Correct class | Why |
|----------------|---------------|-----|
| `OPEN_DISPUTE` | VARIANCE_AVAILABILITY_REASON | actual NULL → variance NULL |
| `CANCELLED` | VARIANCE_AVAILABILITY_REASON | actual NULL → variance NULL |
| `MISSING_ACTUAL` | VARIANCE_AVAILABILITY_REASON | no variance to explain |
| `CURRENCY_MISMATCH` | VARIANCE_AVAILABILITY_REASON | variance NULL |
| `TAX_BASIS_MISMATCH` | VARIANCE_AVAILABILITY_REASON | variance NULL |
| `BILLING_LINK_MISMATCH` | RECONCILIATION_FINDING | source/projection consistency |
| `PROJECTION_DRIFT` | RECONCILIATION_FINDING | rebuild mismatch |
| `STALE_CURSOR` | RECONCILIATION_FINDING | ingest lag |

**NOT_SUPPORTED drivers:** `RATE_CHANGE`, `ROUTE_CHANGE`.

### 9.2 Class B — `VARIANCE_AVAILABILITY_REASON` (non-financial)

**Purpose:** Explain why variance **cannot** be calculated (variance is NULL).
**Storage:** same table `freight_cost.variance_attribution` with `semantic_class = VARIANCE_AVAILABILITY_REASON`.
**Precondition:** corresponding variance amount IS NULL.

| Code | Trigger |
|------|---------|
| `OPEN_DISPUTE` | `open_dispute_count > 0` |
| `CANCELLED` | settlement/order cancelled |
| `MISSING_ACTUAL` | settlement not in actual-eligible state |
| `MISSING_PLANNED` | planned fact absent |
| `CURRENCY_MISMATCH` | planned vs actual currency differ |
| `TAX_BASIS_MISMATCH` | incompatible tax basis detected (fail closed) |

These rows **never alter** planned, accrual, actual, or variance amounts.

### 9.3 Class C — `RECONCILIATION_FINDING` (source/projection consistency)

**Purpose:** Detect divergence between canonical sources and freight-cost projections.
**Storage:** `freight_cost.reconciliation_finding` (**separate table**; not variance_attribution).
**Never** presented as variance driver.

| Kind | Meaning |
|------|---------|
| `BILLING_LINK_MISMATCH` | `billing_reconciliation_status = MISMATCH` |
| `PROJECTION_DRIFT` | live projection ≠ canonical rebuild outcome |
| `STALE_CURSOR` | source cursor behind canonical revision |
| `MISSING_PLANNED_FACT` | planned ingest missing |
| `MISSING_ACCRUAL_FACT` | accrual projection missing when expected |
| `MISSING_FINAL_ACTUAL` | final actual expected but absent |
| `ORPHAN_BILLING_LINK` | billing link without settlement |
| `ORPHAN_PAYMENT_LINK` | payment obligation orphan |
| `CURRENCY_DRIFT` | projection currency ≠ canonical |
| `DUPLICATE_ECONOMIC_FACT` | duplicate source delivery detected |

v2.1B `billing_reconciliation_status` (MATCH/MISMATCH/UNLINKED) remains on projection; reconciliation findings extend drift automation.

---

## 10. Variance driver delta evidence rules (R-002)

```text
SNAPSHOT_COMPONENT_PRESENCE_ALONE_CAN_CAUSE_VARIANCE_REASON=NO
FUEL_DOUBLE_COUNT=DENY
VARIANCE_REASON_REQUIRES_DELTA_EVIDENCE=YES
```

**FUEL false-positive guard:**

- `transport_order_rate_snapshots` FUEL **component** is already embedded in `total_amount` → `PLANNED_COST`.
- Presence of a planned FUEL component **alone** MUST NOT create a FUEL variance driver.
- Acceptable FUEL driver evidence: **approved** settlement accessorial whose source `charge_code` maps to category `FUEL`, with amount contributing to explainable variance delta.
- Attribution logic MUST NOT add snapshot component amounts on top of `total_amount`.

**General rule:** auto-attribution requires reproducible canonical delta facts (approved accessorial rows, legacy principal delta, etc.).

---

## 11. Charge code classification (R-003)

### 11.1 Discovery

- Source: `billing.settlement_accessorials.charge_code VARCHAR(50) NOT NULL`
- Validation today: non-empty string only (`domain/settlement_accessorial.go`)
- Examples in tests/UI: `DETENTION`, `FUEL`, `LUMPER`

### 11.2 Mapping scope model (repository convention)

Repository precedent: platform-global rows use **`tenant_id IS NULL`** (e.g. `core.roles WHERE tenant_id IS NULL`). **No magic platform tenant UUID.**

```text
CHARGE_MAPPING_SCOPE_MODEL=PLATFORM_OR_TENANT
PLATFORM_DEFAULT_REPRESENTATION=tenant_id IS NULL AND mapping_scope='PLATFORM'
TENANT_OVERRIDE_REPRESENTATION=tenant_id IS NOT NULL AND mapping_scope='TENANT'
TENANT_OVERRIDE_PRECEDENCE=TENANT beats PLATFORM for same normalized source key
CROSS_TENANT_MAPPING_LOOKUP=DENY
```

**Table:** `freight_cost.charge_code_mapping`

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `mapping_scope` | TEXT | `PLATFORM` \| `TENANT` CHECK |
| `tenant_id` | UUID NULL | NULL iff PLATFORM |
| `source_charge_code_normalized` | VARCHAR(50) | uppercase trimmed key |
| `normalized_category` | VARCHAR(50) | DETENTION, FUEL, WAITING, LUMPER, OTHER, … |
| `mapping_version` | BIGINT | monotonic per scope |
| `effective_from` | TIMESTAMPTZ | |
| `effective_to` | TIMESTAMPTZ NULL | optional |
| `created_at` | TIMESTAMPTZ | |

**CHECK constraints:**

```sql
CHECK (
  (mapping_scope = 'PLATFORM' AND tenant_id IS NULL)
  OR (mapping_scope = 'TENANT' AND tenant_id IS NOT NULL)
)
```

**Uniqueness:** no overlapping active mapping for same lookup key:

```sql
UNIQUE (mapping_scope, tenant_id, source_charge_code_normalized, effective_from)
-- plus application guard: at most one active row per (scope, tenant, source key) at evaluation time
```

**Normalization pipeline (frozen):**

1. `TRIM` whitespace
2. `UPPER` case
3. max length 50 (reject/truncate policy: **reject** if >50 after trim — matches source column)
4. unknown/unmapped → category `OTHER` + metric `charge_code_unmapped_total`
5. preserve original source `charge_code` in attribution evidence JSON

Classification is **analytics-only** — does not change accrual/actual/variance amounts.

**Seed:** platform-default rows (`tenant_id IS NULL`) in migration; no fake tenant UUID.

---

## 12. Attribution idempotency / append-only identity (R-004)

Follow v2.1B deterministic ID pattern (`DeriveSourceFactID` uses `uuid.NewSHA1` namespace in `domain/ledger.go`).

**New namespace (implementation):** `NamespaceFreightCostVarianceAttribution`

```text
ATTRIBUTION_IDENTITY_MODEL=UUID_SHA1_CANONICAL_KEY
ATTRIBUTION_UNIQUE_KEY=(tenant_id, attribution_fact_id)
ATTRIBUTION_DUPLICATE_REBUILD_ROWS=DENY
ATTRIBUTION_IDEMPOTENT_RECOMPUTE=YES
```

**Deterministic identity:**

```text
attribution_fact_id = UUID-SHA1(
  NamespaceFreightCostVarianceAttribution,
  tenant_id |
  transport_order_id |
  variance_kind                    -- CURRENT | FINAL
  semantic_class                   -- VARIANCE_DRIVER | VARIANCE_AVAILABILITY_REASON
  projection_revision              -- monotonic per TO projection
  reason_code |
  evidence_fingerprint             -- stable hash of canonical evidence refs
  mapping_version                  -- 0 for availability reasons
)
```

**Insert semantics:** `INSERT … ON CONFLICT (tenant_id, attribution_fact_id) DO NOTHING`

**Historical vs current:**

- All rows append-only; never DELETE.
- `is_current BOOLEAN` — set TRUE on insert; prior rows for same `(tenant, transport_order_id, variance_kind, semantic_class)` with lower `projection_revision` flip to FALSE.
- Audit history preserved.

**Idempotency scenarios:**

| Scenario | Behavior |
|----------|----------|
| Normal projection recompute (same inputs) | 0 new rows |
| Duplicate event delivery | 0 new rows |
| Repeated rebuild (same canonical state) | 0 new rows |
| Reconciliation-triggered **manual** rebuild | same as rebuild |
| Retry after crash mid-insert | ON CONFLICT DO NOTHING |

---

## 13. Mapping version / rebuild semantics (R-005)

Distinguish **financial rebuild** from **analytic reclassification**.

```text
MAPPING_CHANGE_ALTERS_FINANCIAL_VARIANCE=NO
FINANCIAL_REBUILD_MAPPING_INDEPENDENT=YES
ATTRIBUTION_REBUILD_POLICY=PINNED_MAPPING_VERSION_FOR_STANDARD_REBUILD
HISTORICAL_ATTRIBUTION_PRESERVED=YES
```

| Operation | Variance amounts | Attribution labels |
|-----------|------------------|-------------------|
| **Financial rebuild** (standard v2.1B path) | Recomputed from canonical facts only; **invariant to mapping_version** | Re-derived using `attribution_mapping_version` **pinned on projection** at first v2.1C compute; same pin + same facts ⇒ idempotent rows |
| **Analytic reclassification** (explicit internal op) | **UNCHANGED** | New attribution rows using **current** mapping_version; old rows `is_current=FALSE` |

**Projection column (proposed):** `attribution_mapping_version BIGINT NULL`

- Set on first attribution compute for transport order.
- Standard rebuild reuses pinned version — not silent relabel with latest rules.
- Reclassification endpoint (internal, S2S, buyer-admin scope): `POST /internal/v1/freight-cost/transport-orders/{id}/reclassify-attribution` — creates new attribution revision only.

**Financial rebuild equivalence (FC-C-RBL):** variance amounts match regardless of mapping table changes.

---

## 14. Double-count protection

Classification and variance attribution are **analytics-only**. Financial totals remain governed by v2.1B ledger rules.

Safeguards:
- Accrual uses APPROVED set only (v2.1B)
- Planned uses snapshot `total_amount` (includes contractual fuel in total — CSet002)
- Attribution MUST NOT add component amounts on top of total_amount
- Duplicate economic facts prevented by `UNIQUE(tenant_id, source_fact_id)` (v2.1B)

```text
DOUBLE_COUNT_CLASSIFICATION_CHANGES_TOTALS=NO
```

---

## 15. Forecast exposure — business meaning (R-007, R-008)

```text
FORECAST_FORMULA=PLANNED + SUM(PROPOSED accessorials EX_VAT)
FORECAST_FORMULA_MATCHES_V2_1B_FREEZE=YES
FORECAST_EXPOSURE_SEMANTICS=planned commercial principal + currently pending proposed accessorial exposure
FORECAST_IS_TOTAL_EXPECTED_LIABILITY=NO
FORECAST_IS_LEDGER_FACT=NO
```

**Interpretation for v2.1D UI labels:**

- Forecast is a **non-ledger potential exposure KPI**, not booked accrual and not ultimate liability estimate.
- When PROPOSED → APPROVED: accrual increases; forecast_exposure **decreases** (pending proposed exposure falls) — **expected and correct**.
- Forecast MUST NOT be labeled "expected total cost" or "ultimate liability" without explicit future architecture amendment.

| Rule | Value |
|------|-------|
| Owner | freight-cost-service (derived projection) |
| Inputs | planned + PROPOSED accessorials from billing internal read |
| Ledger | **NOT written** |
| Rebuild | YES — from canonical HTTP reads when source known |
| Carrier visibility | **DENY** (extend v2.1A view_scope mask) |

Requires billing internal read extension:

```http
GET /internal/v1/freight-settlements/by-transport-order/{id}
```

Response additions:
- `proposed_accessorial_total_ex_vat` (decimal string) — present when read succeeds
- `proposed_accessorial_source_status` — `KNOWN` | `UNKNOWN` (explicit; never infer zero from absence)

---

## 16. Reconciliation (R-006, R-011)

### 16.1 Already in v2.1B

`billing_reconciliation_status`: MATCH | MISMATCH | UNLINKED — computed on projection update.

### 16.2 v2.1C drift detection

Read-only scheduled/on-demand scan producing `reconciliation_finding` rows.

```text
RECONCILIATION_DETECTION=YES
RECONCILIATION_AUTO_REPAIR=PROHIBITED
RECONCILIATION_AUTO_REBUILD=NO
MANUAL_INTERNAL_REBUILD_TRIGGER=YES
AUTOMATIC_REBUILD_ON_FINDING=NO
AUTOMATIC_DESTRUCTIVE_REPAIR=NO
```

**Manual rebuild:** reuse existing `POST /internal/v1/freight-cost/transport-orders/{id}/rebuild` (v2.1B). Requires existing S2S auth; not triggered by finding worker.

**Finding identity:**

```text
RECONCILIATION_FINDING_IDENTITY=UUID_SHA1(
  NamespaceFreightCostReconciliationFinding,
  tenant_id | transport_order_id | finding_kind |
  canonical_reference_key | expected_revision | observed_revision
)
RECONCILIATION_DUPLICATE_OPEN_FINDINGS=DENY
```

**Unique constraint:** `(tenant_id, finding_id)` + partial unique index on open findings:

```sql
UNIQUE (tenant_id, finding_id)
-- application: at most one OPEN row per finding_id
```

**Lifecycle:**

| Status | Meaning |
|--------|---------|
| `OPEN` | Drift active |
| `RESOLVED` | Drift cleared (canonical match restored or condition no longer applies) |
| `REOPENED` | Same finding_id returned after RESOLVED |

**Repeated scan behavior:**

| Event | Action |
|-------|--------|
| Same drift, finding OPEN | Update `last_observed_at`; no new row |
| Drift cleared | Mark OPEN → RESOLVED |
| Same drift returns after RESOLVED | Mark REOPENED (or OPEN with reopened_count++) |
| Canonical revision advances, drift persists | Update `observed_revision`; same finding_id if kind+reference unchanged |
| Manual rebuild resolves mismatch | Next scan → RESOLVED |

**Worker safeguards:** rate limit per tenant; max retries; no rebuild loop — detection emits metrics/findings only.

---

## 17. Data model proposal (planning — no migrations in this PR)

### 17.1 Alter `freight_cost.cost_summary_projection`

| Column | Type | Notes |
|--------|------|-------|
| `current_variance_amount` | NUMERIC(18,2) NULL | EX_VAT |
| `final_variance_amount` | NUMERIC(18,2) NULL | EX_VAT |
| `current_variance_percent` | NUMERIC(9,4) NULL | nullable |
| `final_variance_percent` | NUMERIC(9,4) NULL | nullable |
| `forecast_exposure` | NUMERIC(18,2) NULL | EX_VAT KPI |
| `attribution_mapping_version` | BIGINT NULL | pinned for standard rebuild |
| `projection_revision` | BIGINT NOT NULL DEFAULT 1 | monotonic per TO |

Migration owner: freight-cost-service — next **000057** (verify sequence at implementation time).

### 17.2 New tables

**`freight_cost.variance_attribution`** — append-only explainability (classes A + B)

| Column | Notes |
|--------|-------|
| `attribution_fact_id` | deterministic PK component |
| `semantic_class` | VARIANCE_DRIVER \| VARIANCE_AVAILABILITY_REASON |
| `variance_kind` | CURRENT \| FINAL |
| `reason_code` | |
| `evidence_json` | canonical refs only |
| `mapping_version` | for driver rows |
| `projection_revision` | |
| `is_current` | BOOL |

**`freight_cost.charge_code_mapping`** — see §11.2

**`freight_cost.reconciliation_finding`** — class C

| Column | Notes |
|--------|-------|
| `finding_id` | deterministic |
| `finding_kind` | |
| `status` | OPEN \| RESOLVED \| REOPENED |
| `expected_revision` / `observed_revision` | |
| `first_observed_at` / `last_observed_at` / `resolved_at` | |

Tenant scoping: all tables require `tenant_id` **except** `charge_code_mapping` platform rows where `tenant_id IS NULL`.

---

## 18. Event contract proposal (planning)

v2.1C **does not require new canonical outbox events** for variance — variance is derived on projection update from existing v2.1B ingest paths.

Optional internal observability events (freight-cost-service only, not canonical):

```text
freight_cost.variance_recomputed.v1
freight_cost.reconciliation_finding.v1
```

If added: versioned JSON, decimal strings, tenant-scoped, idempotent on `(tenant, transport_order_id, projection_revision)`.

---

## 19. Internal API proposal

Extend existing (no new public routes):

```http
GET /internal/v1/freight-cost/transport-orders/{id}
```

Response additions (populate from projection):
- `current_variance_amount`, `final_variance_amount`
- `forecast_exposure`
- `variance_drivers[]` — buyer scope only; VARIANCE_DRIVER class only
- `variance_availability_reasons[]` — buyer scope only
- `reconciliation_findings[]` — internal/buyer scope only

**Mapping management (internal, S2S + platform-admin or buyer-admin per existing actor model):**

```http
PUT /internal/v1/freight-cost/charge-code-mappings
```

Tenant override requires authenticated tenant context; platform rows require platform-admin S2S role. No client-controlled tenant header.

---

## 20. Security / visibility

Preserve v2.1A/v2.1B masks (`domain/view_scope.go`):

| Field | Buyer | Carrier |
|-------|-------|---------|
| planned, actual | YES | receivable subset only |
| accrual, forecast, variance | YES | **DENY** |
| variance drivers / availability / findings | YES | **DENY** |
| charge_code_mapping (tenant override) | tenant admin | **DENY** |

```text
CARRIER_CAN_VIEW_BUYER_INTERNAL_COST=NO
CROSS_TENANT_MAPPING=DENY
CROSS_TENANT_ATTRIBUTION=DENY
CROSS_TENANT_RECONCILIATION=DENY
TENANT_ISOLATION=REQUIRED
S2S_AUTH=REQUIRED
```

---

## 21. Rebuild / replay

```text
DERIVED_VARIANCE_REBUILDABLE=YES
DERIVED_FORECAST_REBUILDABLE=YES
VARIANCE_ATTRIBUTION_REBUILDABLE=YES
FINANCIAL_REBUILD_IDEMPOTENT=YES
```

Rebuild path: reuse v2.1B `RebuildTransportOrder` → recompute projection fields → recompute variance/forecast → re-derive attribution with pinned mapping version.

Billing read failure during rebuild: return error; projection unchanged (existing v2.1B behavior for non-404 errors).

---

## 22. Migration strategy (implementation slice)

1. `000057` freight-cost projection variance/forecast/attribution_mapping_version/projection_revision columns
2. `000058` variance_attribution + charge_code_mapping + reconciliation_finding
3. Seed platform-default charge_code mappings (`tenant_id IS NULL`, `mapping_scope='PLATFORM'`)
4. Billing internal read extension (`proposed_accessorial_total_ex_vat`, `proposed_accessorial_source_status`)

Rollback: drop new columns/tables; projection reverts to v2.1B shape.

---

## 23. Rollout / feature flags

```text
V2_1C_VARIANCE_PROJECTION_ENABLED=tenant flag optional (default ON after migration)
V2_1C_RECONCILIATION_JOB_ENABLED=default OFF until soak
```

No frontend rollout in v2.1C.

---

## 24. Observability

Metrics (no PII/money in labels):

- `freight_cost_variance_recomputed_total{result}`
- `freight_cost_forecast_recomputed_total{result}`
- `freight_cost_forecast_proposed_source_unknown_total`
- `freight_cost_reconciliation_finding_total{kind,severity}`
- `freight_cost_charge_code_unmapped_total`

---

## 25. Failure modes

| Failure | Handling |
|---------|----------|
| Missing planned | variance NULL; availability reason; finding OPTIONAL |
| Billing internal read unavailable (forecast) | forecast NULL; retain prior; metric |
| Billing internal read unavailable (rebuild) | rebuild error; projection unchanged |
| Mapping table empty | unmapped → OTHER |
| Percent overflow | NULL if denominator zero |
| Repeated reconciliation scan | update OPEN finding timestamp; no duplicate |

---

## 26. Test matrix (frozen IDs)

| Family | IDs | Count | Focus |
|--------|-----|------:|-------|
| FC-C-VAR | 001–012 | 12 | formula, NULL, currency, dispute, sign |
| FC-C-REA | 001–016 | 16 | driver vs availability separation; delta evidence; FUEL false positive; idempotent attribution |
| FC-C-CHG | 001–010 | 10 | platform NULL tenant; tenant override; cross-tenant deny; normalization |
| FC-C-DUP | 001–004 | 4 | no double-count |
| FC-C-FOR | 001–008 | 8 | planned+proposed; known-empty=planned; unknown≠zero; PROPOSED→APPROVED transition |
| FC-C-REC | 001–010 | 10 | finding identity; OPEN/RESOLVED/REOPENED; no auto-rebuild |
| FC-C-RBL | 001–008 | 8 | idempotent attribution; mapping-independent variance; pinned vs reclassify |
| FC-C-MON | 001–004 | 4 | decimal integrity |
| FC-C-SEC | 001–008 | 8 | tenant isolation; carrier mask; mapping auth |
| FC-C-OUT | 001–004 | 4 | idempotent recompute; duplicate event |
| **Total** | | **84** | |

**New test highlights (PR #47 review):**

| ID | Requirement |
|----|-------------|
| FC-C-REA-009 | Snapshot FUEL component alone does NOT create FUEL driver |
| FC-C-REA-010 | Approved FUEL accessorial CAN create FUEL driver |
| FC-C-REA-011 | OPEN_DISPUTE → availability reason, not driver |
| FC-C-REA-012 | BILLING_LINK_MISMATCH → reconciliation finding, not driver |
| FC-C-REA-013 | Duplicate recompute → 0 new attribution rows |
| FC-C-FOR-006 | Known-empty PROPOSED set → forecast = planned |
| FC-C-FOR-007 | Unknown proposed source → forecast NULL, not zero |
| FC-C-FOR-008 | PROPOSED→APPROVED decreases forecast, increases accrual |
| FC-C-REC-007 | Repeated scan same drift → one OPEN finding |
| FC-C-REC-008 | Drift cleared → RESOLVED |
| FC-C-RBL-005 | Mapping version change → variance unchanged |
| FC-C-RBL-006 | Standard rebuild uses pinned mapping version |
| FC-C-CHG-001 | Platform default via tenant_id IS NULL |
| FC-C-SEC-005 | Tenant A mapping invisible to tenant B |

Every normative requirement maps to ≥1 test.

---

## 27. Acceptance gates (v2.1C implementation)

| Gate | Required |
|------|----------|
| CURRENT_VARIANCE_COMPUTED | PASS |
| FINAL_VARIANCE_COMPUTED | PASS |
| NULL_ZERO_SEMANTICS | PASS |
| CURRENCY_MISMATCH_FAIL_CLOSED | PASS |
| TAX_BASIS_EX_VAT_ONLY | PASS |
| FORECAST_NOT_IN_LEDGER | PASS |
| FORECAST_KNOWN_EMPTY_VS_UNKNOWN | PASS |
| VARIANCE_DRIVER_AVAILABILITY_SEPARATED | PASS |
| CHARGE_CODE_MAPPING_VERSIONED | PASS |
| ATTRIBUTION_IDEMPOTENT | PASS |
| RECONCILIATION_NO_DUPLICATE_OPEN | PASS |
| RECONCILIATION_DETECTION | PASS |
| REBUILD_EQUIVALENCE | PASS |
| MAPPING_INDEPENDENT_FINANCIAL_REBUILD | PASS |
| CARRIER_MASK | PASS |
| CROSS_SERVICE_DB_READS | 0 |
| PUBLIC_API_ADDED | NO |
| FRONTEND_CHANGED | NO |

---

## 28. Deferred v2.1D / v2.1E

See §4. v2.1D owns analytics workspace; v2.1E owns public API and RBAC.

---

## 29. Adversarial self-review (PR #47 closure)

| Check | Result |
|-------|--------|
| R-001 driver vs availability vs reconciliation separated | **PASS** |
| R-002 FUEL snapshot false positive blocked | **PASS** |
| R-003 platform mapping uses tenant_id IS NULL (no magic UUID) | **PASS** |
| R-004 attribution idempotent identity frozen | **PASS** |
| R-005 financial rebuild mapping-independent | **PASS** |
| R-006 reconciliation finding dedup lifecycle | **PASS** |
| R-007 known-empty vs unknown proposed set | **PASS** |
| R-008 forecast KPI semantics explicit | **PASS** |
| billing canonical vs freight-cost derived | OK |
| planned = snapshot vs settlement base | OK — F-002 |
| ex-VAT vs with-VAT mix | OK |
| NULL → zero | OK |
| mixed currency sum | OK — fail closed |
| forecast in ledger | OK — forbidden |
| mapping changes financial totals | OK — analytics-only |
| cross-service DB reads | OK — HTTP only |
| carrier buyer leak | OK |
| v2.1D/E scope leak | OK |
| auto-rebuild on finding | OK — prohibited |

---

## 30. Open questions

```text
OPEN_IMPLEMENTATION_BLOCKER=0
OPEN_HIGH=0
OPEN_MEDIUM=0
```

OQ-005 (charge_code convention) **closed** via versioned mapping table + normalization rules.

---

## 31. Frozen decisions table

```text
V2_1C_SCOPE_NAME=Planned vs Actual / Variance
V2_1C_BASE_SHA=75994efecb5c96bf6608f891fad3b3d0865a593f

PLANNED_COST_OWNER=transport-order-service
ACCRUAL_OWNER=freight-cost-service (derived)
CURRENT_ACTUAL_OWNER=billing-register-service
FINAL_ACTUAL_OWNER=billing-register-service

CURRENT_VARIANCE_FORMULA=current_actual_ex_vat - planned_ex_vat (NULL-safe, same currency)
FINAL_VARIANCE_FORMULA=final_actual_ex_vat - planned_ex_vat (NULL-safe, same currency)

NULL_IS_ZERO=NO

CURRENCY_MISMATCH_POLICY=FAIL_CLOSED
FX_CONVERSION_IN_V2_1C=FORBIDDEN

TAX_BASIS_COMPATIBILITY_POLICY=EX_VAT_ONLY_FOR_VARIANCE

VARIANCE_DRIVER_REQUIRES_NON_NULL_VARIANCE=YES
VARIANCE_AVAILABILITY_REASON_CHANGES_MONEY=NO
RECONCILIATION_FINDING_IS_VARIANCE_DRIVER=NO
MANUAL_REASON_CHANGES_CANONICAL_MONEY=NO

SNAPSHOT_COMPONENT_PRESENCE_ALONE_CAN_CAUSE_VARIANCE_REASON=NO
VARIANCE_REASON_REQUIRES_DELTA_EVIDENCE=YES
FUEL_DOUBLE_COUNT=DENY

CHARGE_MAPPING_SCOPE_MODEL=PLATFORM_OR_TENANT
PLATFORM_DEFAULT_REPRESENTATION=tenant_id IS NULL
TENANT_OVERRIDE_PRECEDENCE=TENANT_OVER_PLATFORM
CROSS_TENANT_MAPPING_LOOKUP=DENY

ATTRIBUTION_IDENTITY_MODEL=UUID_SHA1_CANONICAL_KEY
ATTRIBUTION_UNIQUE_KEY=(tenant_id, attribution_fact_id)
ATTRIBUTION_IDEMPOTENT_RECOMPUTE=YES
ATTRIBUTION_DUPLICATE_REBUILD_ROWS=DENY

MAPPING_CHANGE_ALTERS_FINANCIAL_VARIANCE=NO
FINANCIAL_REBUILD_MAPPING_INDEPENDENT=YES
ATTRIBUTION_REBUILD_POLICY=PINNED_MAPPING_VERSION_FOR_STANDARD_REBUILD
HISTORICAL_ATTRIBUTION_PRESERVED=YES

RECONCILIATION_DETECTION=YES
RECONCILIATION_AUTO_REPAIR=PROHIBITED
RECONCILIATION_AUTO_REBUILD=NO
MANUAL_INTERNAL_REBUILD_TRIGGER=YES
AUTOMATIC_REBUILD_ON_FINDING=NO
RECONCILIATION_DUPLICATE_OPEN_FINDINGS=DENY
RECONCILIATION_FINDING_IDENTITY=UUID_SHA1(tenant|to|kind|reference|expected_rev|observed_rev)
RECONCILIATION_RESOLUTION_POLICY=OPEN_TO_RESOLVED_ON_DRIFT_CLEAR
RECONCILIATION_REOPEN_POLICY=REOPENED_WHEN_SAME_FINDING_RETURNS

FORECAST_FORMULA=PLANNED + SUM(PROPOSED accessorials EX_VAT)
FORECAST_FORMULA_MATCHES_V2_1B_FREEZE=YES
KNOWN_EMPTY_PROPOSED_SET_IS_ZERO=YES
FORECAST_WITH_KNOWN_EMPTY_PROPOSED_SET=PLANNED
UNKNOWN_PROPOSED_SOURCE_IS_ZERO=NO
FORECAST_EXPOSURE_SEMANTICS=planned principal + pending proposed accessorial exposure
FORECAST_IS_TOTAL_EXPECTED_LIABILITY=NO
FORECAST_IS_LEDGER_FACT=NO

VARIANCE_REASON_ATTRIBUTION_IN_SCOPE=YES
CHARGE_CODE_CLASSIFICATION_IN_SCOPE=YES
DOUBLE_COUNT_CLASSIFICATION_IN_SCOPE=YES
FORECAST_EXPOSURE_IN_SCOPE=YES
FORECAST_WRITTEN_TO_LEDGER=NO
RECONCILIATION_IN_SCOPE=YES (detection-first)

PUBLIC_API_IN_V2_1C=NO
FRONTEND_IN_V2_1C=NO

CROSS_SERVICE_DB_READS_FROM_FREIGHT_COST=NO
FLOAT64_MONEY_ON_FREIGHT_COST_BOUNDARY=NO

CARRIER_CAN_VIEW_BUYER_INTERNAL_COST=NO
CROSS_TENANT_MAPPING=DENY
CROSS_TENANT_ATTRIBUTION=DENY
CROSS_TENANT_RECONCILIATION=DENY

DERIVED_VARIANCE_REBUILDABLE=YES

RUNTIME_IMPLEMENTATION_STARTED=NO

OPEN_IMPLEMENTATION_BLOCKER=0
OPEN_HIGH=0
OPEN_MEDIUM=0
```

---

## Document control

| Field | Value |
|-------|-------|
| Author | v2.1C planning + PR #47 review |
| Base SHA | `75994efecb5c96bf6608f891fad3b3d0865a593f` |
| Review status | PR #47 findings R-001…R-008 closed |
| Runtime changes | **NONE** |
