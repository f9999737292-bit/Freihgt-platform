# Freight Cost Planned vs Actual / Variance v2.1C — Implementation Plan & Contract Freeze

**Status:** PLANNING / CONTRACT FREEZE (no runtime in this document)  
**Official slice name (architecture §46):** **v2.1C — Planned vs Actual / Variance**  
**Base SHA (post-PR #46 merge):** `75994efecb5c96bf6608f891fad3b3d0865a593f`  
**Merged v2.1B feature HEAD:** `5fc2fa3c1dba5853ebc15a5f43623f964c2371d4`  
**PR #46 merge commit:** `75994efecb5c96bf6608f891fad3b3d0865a593f`

**Architecture baselines:**
- `docs/engineering/FREIGHT_COST_MANAGEMENT_v2.1_ARCHITECTURE.md`
- `docs/architecture/FREIGHT_COST_MANAGEMENT_v2.1_FINAL_REVIEW.md`
- `docs/engineering/FREIGHT_COST_ACCRUAL_LEDGER_v2.1B_IMPLEMENTATION_PLAN.md`
- `docs/engineering/FREIGHT_COST_ACCRUAL_LEDGER_v2.1B_IMPLEMENTATION.md`

**Purpose:** Freeze v2.1C contracts so subsequent implementation is mechanical. **Do not implement runtime in the planning PR.**

---

## 1. Executive summary

v2.1B delivered the derived append-only cost ledger, accrual/actual/billed/paid projection persistence, billing/payment outbox integration, and canonical rebuild. v2.1C adds **derived planned-vs-actual variance**, **deterministic variance reason attribution**, **charge_code semantic classification**, **forecast exposure projection** (non-ledger), and **reconciliation drift detection** — all within `freight-cost-service` as derived projections over existing canonical facts.

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
| Variance reason persistence | NOT_FOUND |
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
| 6 | `forecast_exposure` projection: `planned + SUM(PPROPOSED accessorials)` via billing internal read extension |
| 7 | Persist `forecast_exposure` on projection (non-ledger KPI) |
| 8 | Deterministic variance reason attribution model + persistence table |
| 9 | `charge_code` → normalized semantic category mapping (versioned rules table) |
| 10 | Double-count classification guards (analytics-only; no ledger amount mutation) |
| 11 | Reconciliation drift detection job (read-only findings + metrics) |
| 12 | Internal admin rebuild-on-drift trigger (reuse v2.1B rebuild route; no auto-destructive repair) |
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

### 5.3 Forecast exposure (architecture OQ-001 — frozen)

```text
FORECAST_EXPOSURE = planned_amount + EXACT_NUMERIC_SUM(PPROPOSED accessorials)
  WHEN currency matches
  ELSE NULL (fail closed)

FORECAST_IS_NOT_LEDGER_ACTUAL=YES
FORECAST_NOT_WRITTEN_TO_COST_ENTRY=YES
```

Proposed accessorials **never** affect accrual (v2.1B invariant preserved).

---

## 6. NULL semantics

| Condition | planned | accrual | current_actual | final_actual | variance | forecast |
|-----------|---------|---------|----------------|--------------|----------|----------|
| Pre-ingest | NULL | NULL | NULL | NULL | NULL | NULL |
| Planned only | VALUE | NULL/VALUE | NULL | NULL | NULL | VALUE if proposed known |
| Disputed settlement | VALUE | VALUE | **NULL** | NULL | **NULL** | per proposed set |
| Currency mismatch | VALUE | FAIL_CLOSED | — | — | **NULL** | **NULL** |
| Legacy no snapshot | award-base | derived | per settlement | per finality | per formula | per proposed |
| Cancelled order | retained historical | — | NULL | NULL | exclude active aggregates | NULL |

```text
NULL_IS_ZERO=NO
UNKNOWN_USES_NULL=YES
```

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

## 9. Variance reason attribution

Deterministic attribution only. Manual reasons are labels — **never alter canonical money**.

### 9.1 Reason codes (frozen set)

| Code | Trigger | Auto? |
|------|---------|-------|
| `ACCESSORIAL` | Approved accessorial delta explains variance | AUTO |
| `FUEL` | Snapshot component FUEL + breakdown AVAILABLE | AUTO partial |
| `DETENTION` | Approved accessorial charge_code maps to DETENTION category | AUTO partial |
| `WAITING` | Approved accessorial charge_code maps to WAITING category | AUTO partial |
| `CANCELLATION` | Settlement/order cancelled | AUTO |
| `BILLING_ADJUSTMENT` | Billing link mismatch while actual available | AUTO |
| `DISPUTE_UNAVAILABILITY` | Open dispute → actual NULL | AUTO |
| `LEGACY_PRICING` | Non-SNAPSHOT_V1 settlement principal | AUTO |
| `MANUAL_ADJUSTMENT` | Accessorial present but no rule match | MANUAL tag allowed |
| `UNATTRIBUTED` | Variance exists, no rule matched | AUTO fallback |
| `OTHER` | Operator-provided reason (internal only) | MANUAL |

**NOT_SUPPORTED:** `RATE_CHANGE`, `ROUTE_CHANGE`.

### 9.2 Persistence

New table `freight_cost.variance_attribution` (planning schema — migration in implementation slice):

- Append-only finding rows per `(tenant, transport_order_id, variance_kind, source_revision_snapshot)`
- Stores reason_code, evidence JSON, mapping_rule_version
- Does **not** replace ledger; audit/explainability only

---

## 10. Charge code classification

### 10.1 Discovery

- Source: `billing.settlement_accessorials.charge_code VARCHAR(50) NOT NULL`
- Validation today: non-empty string only (`domain/settlement_accessorial.go`)
- Examples in tests/UI: `DETENTION`, `FUEL`, `LUMPER`

### 10.2 v2.1C design

New table `freight_cost.charge_code_mapping` (tenant-scoped optional overrides + platform defaults):

| Column | Purpose |
|--------|---------|
| `normalized_category` | e.g. DETENTION, FUEL, WAITING, LUMPER, OTHER |
| `source_charge_code` | original uppercase token |
| `mapping_version` | monotonic rule set version |
| `effective_from` | timestamp |

Rules:
- Case-insensitive match on trimmed `charge_code`
- Unknown → `OTHER` + metric `charge_code_unmapped_total`
- Classification is **analytics-only** — does not change accrual/actual amounts
- Mapping version recorded on attribution rows for rebuild reproducibility

---

## 11. Double-count protection

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

## 12. Forecast exposure

| Rule | Value |
|------|-------|
| Owner | freight-cost-service (derived projection) |
| Inputs | planned + PROPOSED accessorials from billing internal read |
| Ledger | **NOT written** |
| Rebuild | YES — from canonical HTTP reads |
| Carrier visibility | **DENY** (extend v2.1A view_scope mask) |

Requires billing internal read extension to expose PROPOSED accessorial sum (decimal string) — **new internal field only**, no public API.

---

## 13. Reconciliation

### 13.1 Already in v2.1B

`billing_reconciliation_status`: MATCH | MISMATCH | UNLINKED — computed on projection update.

### 13.1 v2.1C extensions

New drift detection (read-only):

| Check | Action |
|-------|--------|
| Projection vs rebuild canonical mismatch | FINDING + metric |
| Missing planned fact | FINDING |
| Stale source_cursor | FINDING |
| currency_code drift | FINDING |
| orphan billing/payment link | FINDING |

```text
RECONCILIATION_AUTO_REPAIR=PROHIBITED
RECONCILIATION_AUTO_REBUILD=OPTIONAL_INTERNAL_ONLY
RECONCILIATION_DEFAULT=READ_ONLY_DETECTION
```

Implementation: scheduled job in freight-cost-service (pattern: payment/shipment outbox worker conventions) OR on-demand internal endpoint — **detection only** by default.

---

## 14. Data model proposal (planning — no migrations in this PR)

### 14.1 Alter `freight_cost.cost_summary_projection`

| Column | Type | Notes |
|--------|------|-------|
| `current_variance_amount` | NUMERIC(18,2) NULL | EX_VAT |
| `final_variance_amount` | NUMERIC(18,2) NULL | EX_VAT |
| `current_variance_percent` | NUMERIC(9,4) NULL | nullable |
| `final_variance_percent` | NUMERIC(9,4) NULL | nullable |
| `forecast_exposure` | NUMERIC(18,2) NULL | EX_VAT KPI |

Migration owner: freight-cost-service — next **000057** (verify sequence at implementation time).

### 14.2 New tables

**`freight_cost.variance_attribution`** — append-only explainability  
**`freight_cost.charge_code_mapping`** — versioned normalization rules  
**`freight_cost.reconciliation_finding`** — drift detection results (mutable status ok for findings workflow)

All tables: `tenant_id` required; indexes on `(tenant_id, transport_order_id)`.

---

## 15. Event contract proposal (planning)

v2.1C **does not require new canonical outbox events** for variance — variance is derived on projection update from existing v2.1B ingest paths.

Optional internal observability event (freight-cost-service only, not canonical):

```text
freight_cost.variance_recomputed.v1
freight_cost.reconciliation_finding.v1
```

If added: versioned JSON, decimal strings, tenant-scoped, idempotent on `(tenant, transport_order_id, projection_revision)`.

---

## 16. Internal API proposal

Extend existing (no new public routes):

```http
GET /internal/v1/freight-cost/transport-orders/{id}
```

Response additions (already in DTO — populate from projection):
- `current_variance_amount`, `final_variance_amount`
- `forecast_exposure`
- optional `variance_reasons[]` (buyer scope only)

New internal read dependency:

```http
GET /internal/v1/freight-settlements/by-transport-order/{id}
  + proposed_accessorial_total_ex_vat  (decimal string)
```

Billing-register-service change: extend existing internal handler response — **decimal string only**.

---

## 17. Security / visibility

Preserve v2.1A/v2.1B masks (`domain/view_scope.go`):

| Field | Buyer | Carrier |
|-------|-------|---------|
| planned, actual | YES | receivable subset only |
| accrual, forecast, variance | YES | **DENY** |
| variance reasons | YES | **DENY** |

```text
CARRIER_CAN_VIEW_BUYER_INTERNAL_COST=NO
TENANT_ISOLATION=REQUIRED
S2S_AUTH=REQUIRED
```

---

## 18. Rebuild / replay

```text
DERIVED_VARIANCE_REBUILDABLE=YES
DERIVED_FORECAST_REBUILDABLE=YES
VARIANCE_REASON_REBUILDABLE=YES (with mapping_version pinned in attribution row)
```

Rebuild path: reuse v2.1B `RebuildTransportOrder` → recompute projection fields → recompute variance/forecast → re-derive attribution deterministically.

Mapping rule changes MUST NOT retroactively alter persisted attribution rows; new rebuild creates new attribution rows with new `mapping_version`.

---

## 19. Migration strategy (implementation slice)

1. `000057` freight-cost projection variance/forecast columns
2. `000058` variance_attribution + charge_code_mapping + reconciliation_finding
3. Seed default charge_code mappings (platform tenant)
4. Billing internal read extension (no billing schema change required if sum computed in query)

Rollback: drop new columns/tables; projection reverts to v2.1B shape.

---

## 20. Rollout / feature flags

```text
V2_1C_VARIANCE_PROJECTION_ENABLED=tenant flag optional (default ON after migration)
V2_1C_RECONCILIATION_JOB_ENABLED=default OFF until soak
```

No frontend rollout in v2.1C.

---

## 21. Observability

Metrics (no PII/money in labels):

- `freight_cost_variance_recomputed_total{result}`
- `freight_cost_forecast_recomputed_total{result}`
- `freight_cost_reconciliation_finding_total{kind,severity}`
- `freight_cost_charge_code_unmapped_total`

---

## 22. Failure modes

| Failure | Handling |
|---------|----------|
| Missing planned | variance NULL; finding OPTIONAL |
| Billing internal read unavailable | rebuild retry; projection unchanged |
| Mapping table empty | unmapped → OTHER |
| Percent overflow | cap NULL if denominator zero |

---

## 23. Test matrix (frozen IDs)

| Family | IDs | Count | Focus |
|--------|-----|------:|-------|
| FC-C-VAR | 001–012 | 12 | formula, NULL, currency, dispute, sign |
| FC-C-REA | 001–008 | 8 | reason attribution deterministic |
| FC-C-CHG | 001–006 | 6 | charge_code mapping |
| FC-C-DUP | 001–004 | 4 | no double-count |
| FC-C-FOR | 001–005 | 5 | forecast separation |
| FC-C-REC | 001–006 | 6 | drift detection |
| FC-C-RBL | 001–004 | 4 | rebuild equivalence |
| FC-C-MON | 001–004 | 4 | decimal integrity |
| FC-C-SEC | 001–004 | 4 | tenant/carrier mask |
| FC-C-OUT | 001–003 | 3 | idempotent recompute |
| **Total** | | **56** | |

Every normative requirement maps to ≥1 test.

---

## 24. Acceptance gates (v2.1C implementation)

| Gate | Required |
|------|----------|
| CURRENT_VARIANCE_COMPUTED | PASS |
| FINAL_VARIANCE_COMPUTED | PASS |
| NULL_ZERO_SEMANTICS | PASS |
| CURRENCY_MISMATCH_FAIL_CLOSED | PASS |
| TAX_BASIS_EX_VAT_ONLY | PASS |
| FORECAST_NOT_IN_LEDGER | PASS |
| CHARGE_CODE_MAPPING_VERSIONED | PASS |
| VARIANCE_REASONS_DETERMINISTIC | PASS |
| RECONCILIATION_DETECTION | PASS |
| REBUILD_EQUIVALENCE | PASS |
| CARRIER_MASK | PASS |
| CROSS_SERVICE_DB_READS | 0 |
| PUBLIC_API_ADDED | NO |
| FRONTEND_CHANGED | NO |

---

## 25. Deferred v2.1D / v2.1E

See §4. v2.1D owns analytics workspace; v2.1E owns public API and RBAC.

---

## 26. Adversarial self-review (completed)

| Check | Result |
|-------|--------|
| billing canonical vs freight-cost derived | OK — variance owner is freight-cost derived |
| planned = snapshot vs settlement base | OK — F-002 proven equivalence SNAPSHOT_V1 |
| ex-VAT vs with-VAT mix | OK — variance excludes payable/paid |
| NULL → zero | OK — explicit NULL rules |
| mixed currency sum | OK — fail closed |
| forecast in ledger | OK — forbidden |
| mapping changes historical totals | OK — analytics-only + versioned attribution |
| cross-service DB reads | OK — HTTP only |
| carrier buyer leak | OK — view_scope extended |
| v2.1D/E scope leak | OK — none in v2.1C deliverables |

---

## 27. Open questions

```text
OPEN_IMPLEMENTATION_BLOCKER=0
OPEN_HIGH=0
OPEN_MEDIUM=0
```

OQ-005 (charge_code convention) **closed in this plan** via versioned mapping table.

---

## 28. Frozen decisions table

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
| Author | v2.1C planning agent |
| Base SHA | `75994efecb5c96bf6608f891fad3b3d0865a593f` |
| Review status | Ready for independent review |
| Runtime changes | **NONE** |
