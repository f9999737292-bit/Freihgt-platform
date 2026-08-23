# FREIGHT COST INTELLIGENCE v2.2 — Architecture

**Status:** Architecture freeze (v2.2A — discovery & contract only)  
**Branch:** `arch/freight-cost-intelligence-v2.2a`  
**Base:** `origin/main` @ `e80af5238bb93d8f29432582beba50103e3b9367` (PR #52 merged)  
**Date:** 2026-08-23  
**Scope:** Design only — **no production runtime in v2.2A**

---

## 1. Context

Freight Cost Management v2.1 delivered:

- Append-only **cost ledger** (`freight_cost.cost_entry`)
- Per-order **cost summary projection** (`freight_cost.cost_summary_projection`)
- Variance explainability (`variance_attribution`, `reconciliation_finding`, `charge_code_mapping`)
- Public workspace API under `/api/v1/freight-costs/*` (feature-flag default OFF)
- RBAC via `freightcostrbac` (buyer analytics vs carrier read)

v2.2 **Freight Cost Intelligence** adds tenant-scoped analytics read models above the canonical ledger:

```text
Canonical Freight Cost Facts / Ledger
        ↓
Derived Analytics Projections (v2.2 — NEW)
        ↓
Freight Cost Intelligence
        ├─ Lane Intelligence
        ├─ Carrier Cost Intelligence
        ├─ Accessorial Intelligence
        ├─ Cost Drivers (reuse v2.1 variance)
        ├─ Tenant Historical Benchmarking
        ├─ Savings Opportunities
        └─ Forecast Readiness (assessment only in v2.2A)
        ↓
Future Freight Cost Intelligence Workspace (v2.2F)
```

---

## 2. Scope & Non-Goals

### In scope (v2.2A)

- Source-of-truth analysis
- Data readiness classification (CLASS A/B/C)
- Projection architecture & grains
- Benchmark & savings opportunity design
- RBAC / tenant isolation design
- Future API contract freeze
- ADRs

### Non-goals (v2.2A and explicitly deferred)

| Non-goal | Rationale |
|----------|-----------|
| Production migrations / projection tables | v2.2B+ |
| Projection workers / Kafka consumers | v2.2B+ |
| Gateway analytics routes implementation | v2.2F |
| Frontend intelligence screens | v2.2F |
| ML / predictive forecasting runtime | Out of v2.2 |
| Cross-tenant market benchmarking | Prohibited in v2.2 |
| FX engine / cross-currency totals | No FX in platform |
| Feature flag enablement | Unchanged (default OFF) |

---

## 3. Source of Truth

### 3.1 Financial canonical facts

| Layer | Service | Table / Model | Authoritative |
|-------|---------|---------------|---------------|
| Cost ledger entries | `freight-cost-service` | `freight_cost.cost_entry` | **YES** (append-only) |
| Per-order cost summary | `freight-cost-service` | `freight_cost.cost_summary_projection` | Derived, rebuildable |
| Planned cost | `transport-order-service` → ledger | `transport.transport_order_rate_snapshots` → `PLANNED_COST_SNAPSHOT` | Canonical at TO time |
| Accrual / actual | `billing-register-service` → ledger | `billing.freight_settlements` → outbox → cost entries | Canonical settlement |
| Billed / payable / paid | billing + payment → ledger | Outbox events → cost entries | Canonical billing/payment |

**Invariant:** Analytics projections **never** become authoritative financial source. See **ADR-22-001**.

### 3.2 Operational / dimension facts (not financial)

| Dimension | Authoritative service | Table / field |
|-----------|----------------------|---------------|
| Transport order reference | `transport-order-service` | `transport.transport_orders.order_number` |
| Origin / destination geography | `transport-order-service` | `transport.locations` via TO location FKs |
| Cargo weight / volume | `transport-order-service` | `transport.cargoes.gross_weight`, `volume` |
| Carrier / buyer company | `company-service` | `core.companies.legal_name`, `short_name` |
| Accessorial lines | `billing-register-service` | `billing.settlement_accessorials` |
| Base vs accessorial split | `billing-register-service` | `freight_settlements.base_freight_amount`, `approved_accessorial_total` |

### 3.3 Explicitly NOT authoritative

| Signal | Role |
|--------|------|
| `reconciliation_finding` | Analytics signal only — does not rewrite costs |
| `variance_attribution` | Explainability — derived from projection + mapping pin |
| Tracking-service Haversine distance | GPS quality heuristic — **not** route distance |
| Workspace KPI aggregates | Read-model over projection — not SSOT |

---

## 4. Cost Lifecycle

Discovered states from `freight-cost-service/internal/domain`:

```text
PLANNED_COST_SNAPSHOT          (immutable TO rate snapshot)
        ↓
ACCRUAL_COST_SNAPSHOT          (settlement accrual — may include open disputes)
        ↓
CURRENT_ACTUAL_COST_SNAPSHOT   (settlement approved+, no open disputes)
        ↓
FINAL_ACTUAL_COST_SNAPSHOT     (READY_FOR_PAYMENT, no disputes)
        ↓
BILLED_COST_SNAPSHOT           (billing register link)
        ↓
PAYABLE_AMOUNT_SNAPSHOT        (register payable)
        ↓
PAID_AMOUNT_SNAPSHOT           (payment obligation)
```

**Data stages** (`ComputeDataStage`): `ACCRUAL_AVAILABLE` → `CURRENT_ACTUAL_AVAILABLE` → `FINAL_ACTUAL_AVAILABLE` → `BILLING_LINKED` → `PAID`

**Financial finality** (`FinancialFinality`): `NOT_EVALUATED` → `DRAFT` → `CURRENT_ACTUAL` → `FINAL_ACTUAL` → `CANCELLED`

### Rules

| Rule | Definition |
|------|------------|
| `ACTIVE_ENTRY_RULE` | Latest cost entry per `(tenant, transport_order, source_service, source_type, source_id, entry_kind)` — linked via `source_cursor.last_cost_entry_id` |
| `SUPERSEDED_ENTRY_RULE` | Prior row retained with `supersedes_entry_id` pointer; **excluded from analytics sums** |
| `FINALITY_RULE` | `FINAL_ACTUAL` requires settlement `READY_FOR_PAYMENT` + zero open disputes |
| `ANALYTICS_ELIGIBILITY_RULE` | Include active projection row per transport order; use pinned mapping version for historical classification; open-cost accruals **may** enter tenant totals with `data_quality=PARTIAL` flag when disputes/open items exist |

| Question | Answer |
|----------|--------|
| `CAN_OPEN_COSTS_ENTER_ANALYTICS` | **YES** — with explicit `data_stage` / dispute flags; accrual totals labeled separately from final actual |
| `CAN_SUPERSEDED_ROWS_ENTER_ANALYTICS` | **NO** — ledger append-only history preserved; projection uses latest active amounts only |
| `CAN_RECONCILIATION_FINDINGS_CHANGE_COST` | **NO** — findings are signals; canonical facts unchanged |

---

## 5. Data Flow

```text
[billing-register-service]
  freight_settlements / settlement_accessorials
  billing.freight_cost_outbox (PENDING → PUBLISHED)
        ↓ HTTP ingest / rebuild
[freight-cost-service]
  cost_entry (append-only)
  source_cursor
  cost_summary_projection
  variance_attribution / reconciliation_finding
        ↓ v2.2B+ rebuild job
  analytics projections (lane / carrier / accessorial / period / opportunity)
        ↓ v2.2F
  GET /api/v1/freight-costs/analytics/*
        ↓
[api-gateway + RBAC]
        ↓
[Intelligence Workspace UI]
```

**Enrichment at projection build time (not browser N+1):**

- Batch read `transport.transport_orders` + `transport.locations` for lane keys
- Batch read `core.companies` for carrier/buyer display names
- Batch read `billing.settlement_accessorials` + apply pinned `charge_code_mapping`

---

## 6. Projection Architecture

### 6.1 Options evaluated

| Option | Verdict |
|--------|---------|
| A. Direct raw ledger queries at API time | Rejected — cross-service joins, N+1 enrichment, unbounded aggregation |
| B. SQL views / materialized views only | Partial — good for single-service aggregates; insufficient for cross-service dimensions |
| C. Dedicated projection tables | **Recommended** — matches existing `cost_summary_projection` + Control Tower rebuild pattern |
| D. Hybrid | **Selected** — per-order summary stays; v2.2 adds dedicated analytics projection tables rebuilt from summary + dimensions |

See **ADR-22-002**.

### 6.2 Provisional projection tables (conceptual — no migration in v2.2A)

#### `freight_cost_analytics.order_fact_projection`

| Attribute | Value |
|-----------|-------|
| **GRAIN** | One row per `(tenant_id, transport_order_id, currency_code)` |
| **PRIMARY_KEY** | `(tenant_id, transport_order_id, currency_code)` |
| **TENANT_KEY** | `tenant_id` (required on all queries) |
| **COMPANY_KEY** | `buyer_company_id` (buyer scope); carrier sees own rows only |
| **DIMENSION_KEYS** | `carrier_company_id`, `lane_key`, `transport_mode`, `equipment_type`, `shipment_id` |
| **PERIOD** | `cost_effective_date` (derived from settlement/service date or TO pickup date) |
| **METRICS** | `planned_amount`, `current_actual_amount`, `final_actual_amount`, `variance_amount`, `accessorial_total`, `base_freight_amount` |
| **SNAPSHOTS** | `order_reference`, `carrier_display_name`, `lane_label` |
| **FRESHNESS** | `calculated_at`, `data_through`, `projection_version` |

#### `freight_cost_analytics.lane_period_projection`

| Attribute | Value |
|-----------|-------|
| **GRAIN** | `(tenant_id, lane_key, transport_mode, equipment_type, currency_code, period_month)` |
| **METRICS** | `order_count`, `spend_total`, `planned_total`, `variance_total`, `median_cost`, `p25`, `p75`, `accessorial_rate`, `carrier_count` |
| **SAMPLE_COUNT** | `order_count` |
| **UNIQUE_CONSTRAINT** | Full grain + `projection_version` for rebuild audit |

#### `freight_cost_analytics.carrier_period_projection`

| Attribute | Value |
|-----------|-------|
| **GRAIN** | `(tenant_id, carrier_company_id, currency_code, period_month)` |
| **METRICS** | `order_count`, `lane_count`, `spend_total`, `median_cost`, `variance_rate`, `accessorial_rate`, `reconciliation_finding_rate` |

#### `freight_cost_analytics.accessorial_period_projection`

| Attribute | Value |
|-----------|-------|
| **GRAIN** | `(tenant_id, normalized_category, currency_code, period_month)` optional `(carrier_company_id, lane_key)` drill-down rows |
| **METRICS** | `total_amount`, `order_count`, `share_of_spend` |

#### `freight_cost_analytics.opportunity_projection`

| Attribute | Value |
|-----------|-------|
| **GRAIN** | One row per detected opportunity `(tenant_id, opportunity_id)` |
| **METRICS** | `type`, `scope`, `entity_key`, `observed_value`, `baseline_value`, `estimated_delta`, `sample_size`, `evidence_json`, `data_quality` |

**Schema namespace:** provisional `freight_cost_analytics` or extend `freight_cost` — decision deferred to v2.2B migration PR.

---

## 7. Projection Update Model

See **ADR-22-003**.

| Mechanism | Source | Use |
|-----------|--------|-----|
| Live incremental | `billing.freight_cost_outbox` → ingest → `cost_summary_projection` | Existing v2.1B path |
| v2.2 incremental | Same outbox + derived projection fan-out hook | After summary update, enqueue analytics slice rebuild for affected TO |
| Scheduled reconciliation | Cron / job (pattern: Control Tower `shipment_status_projection_rebuild_*`) | Catch missed events, dimension drift |
| Full tenant rebuild | `RebuildService.RebuildTransportOrder` extended | Disaster recovery, mapping version pin change |

**PROJECTION_UPDATE_MODEL:** `HYBRID` (event-driven incremental + scheduled full rebuild)

| Flag | Value |
|------|-------|
| `FULL_REBUILD_SUPPORTED` | YES — from `cost_summary_projection` + authoritative dimension batch reads |
| `INCREMENTAL_UPDATE_SUPPORTED` | YES — per transport_order_id on outbox ingest |
| `SOURCE_WATERMARK` | `max(cost_summary_projection.updated_at)` + outbox `published_at` |
| `PROJECTION_VERSION` | Monotonic integer bumped on mapping pin or algorithm change |

---

## 8. Event Model

### Available (discovered)

| Event | Source | `event_type` |
|-------|--------|--------------|
| Accrual snapshot | `billing.freight_cost_outbox` | `freight_settlement.accrual_snapshot.v1` |
| Current actual snapshot | same | `freight_settlement.current_actual_snapshot.v1` |
| Final actual snapshot | same | `freight_settlement.final_actual_snapshot.v1` |
| Billing link snapshot | same | `billing_register.settlement_billing_link_snapshot.v1` |
| Payable snapshot | same | `billing_register.payable_snapshot.v1` |
| Planned cost | rebuild from TO | `transport_order_rate_snapshots` (no Kafka — pull/rebuild) |

### Missing (for v2.2 enrichment)

| Event | Gap | Resolution |
|-------|-----|------------|
| Transport order created/updated | No outbox to freight-cost | Batch read at projection build |
| Company renamed | No event | Dimension snapshot at build + scheduled refresh |
| Accessorial approved | Implicit in settlement revision | Settlement outbox revision covers |

---

## 9. Idempotency & Replay

Aligned with existing ingest (`ingest_service.go`):

| Policy | Value |
|--------|-------|
| `IDEMPOTENCY_KEY` | `(tenant_id, source_event_id)` unique; `(tenant_id, source_fact_id)` unique |
| `VERSION_POLICY` | `source_revision` monotonic per source dimension; stale revisions journaled but do not regress projection |
| `OUT_OF_ORDER_POLICY` | Entry inserted; projection updated only when `source_revision > cursor.last_source_revision` |
| `REPLAY_POLICY` | Identical payload → NO-OP; conflicting payload → integrity error; full rebuild replays canonical sources |

---

## 10. Historical Semantics

Preserve v2.1 invariants:

| Rule | Value |
|------|-------|
| `HISTORICAL_CLASSIFICATION_SOURCE` | `variance_attribution.mapping_version` pinned at evaluation time |
| `CURRENT_MAPPING_USED_FOR_HISTORY` | **NO** — rebuild uses pinned mapping version per attribution fact |
| `SUPERSEDED_ROWS_POLICY` | Exclude from all sums; retain for audit |
| `ACTIVE_RECLASSIFIED_STATE_POLICY` | Analytics reflects **current active** normalized category; superseded attributions marked `is_current=false` |

### Reclassification algorithm (architecture)

1. On mapping version bump or tenant mapping change, run targeted rebuild for affected transport orders.
2. Recompute `variance_attribution` with new mapping pin; mark prior rows `is_current=false`.
3. Fan-out analytics projection rebuild for affected orders / periods.
4. Never mutate `cost_entry` amounts — classification change only affects derived attribution and analytics buckets.

---

## 11. Reconciliation

| Rule | Value |
|------|-------|
| `RECONCILIATION_FINDING_IS_FINANCIAL_SOURCE` | **NO** |
| Analytics use | Finding count, rate, type distribution, variance magnitude signal |
| Policy | Findings do not alter `cost_entry` or settlement canonical amounts |

---

## 12. Canonical Lane Model

See **ADR-22-004**.

| Flag | Value |
|------|-------|
| `CANONICAL_LANE_EXISTS` | **PARTIAL** — geography exists; no first-class `lane` entity |
| `LANE_GRAIN` | City→city within country, plus `transport_mode`, `equipment_type` |
| `LANE_KEY_COMPONENTS` | `origin_country`, `origin_city`, `destination_country`, `destination_city`, `transport_mode`, `equipment_type` |
| `LANE_DIRECTIONAL` | **YES** — origin→destination order matters |
| `NORMALIZATION_RULE` | Uppercase TRIM city; NULL city → `NOT_AVAILABLE` for lane KPI; facility ID optional enrichment only |
| `CARDINALITY_RISK` | MEDIUM — city-level balances sample size vs precision; postal code too sparse today |

**Rejected grains:**

- Facility→facility only — high cardinality, unstable renames
- Region→region only — too coarse for carrier comparison
- Origin+destination UUID — not business meaningful

---

## 13. Freshness & Failure Semantics

### Freshness fields (all analytics responses)

- `calculated_at` — projection compute timestamp
- `data_through` — max source `updated_at` / event time included
- `projection_version` — algorithm/mapping version

### Failure semantics (distinct states)

| State | Meaning | Must NOT appear as |
|-------|---------|-------------------|
| `NO_ROWS` | Valid query, zero matching cohort | — |
| `NOT_AVAILABLE` | Capability/data source absent | `0`, `[]` |
| `INSUFFICIENT_SAMPLE` | Cohort below min sample policy | `0`, unreliable percentile |
| `PROJECTION_STALE` | Projection older than threshold | live data |
| `DEPENDENCY_UNAVAILABLE` | Enrichment service down | empty success |
| `MIXED_CURRENCY` | Multi-currency in requested aggregate | single currency total |

**Invariant:** `UNAVAILABLE_EQUALS_EMPTY=NO` (preserved from v2.1E)

---

## 14. Performance Design

- Tenant-first indexes on all projection PKs
- Period + currency composite indexes for range queries
- Bounded pagination (max 100, default 20 — matches existing workspace API)
- Pre-aggregation at lane/carrier/period grains — **no** client-side 100k row aggregation
- Custom date ranges bounded (e.g. max 24 months) — v2.2F contract detail

---

## 15. ADRs

### ADR-22-001 Analytics source of truth

**Decision:** `freight_cost.cost_entry` + upstream canonical billing/TO facts are the financial SSOT. All v2.2 analytics projections are derived, rebuildable read models and must never write back to financial tables.

**Status:** Accepted

---

### ADR-22-002 Projection storage architecture

**Decision:** Dedicated projection tables (Option C/D hybrid), following `cost_summary_projection` and Control Tower rebuild patterns. No runtime cross-service joins in public analytics API.

**Status:** Accepted

---

### ADR-22-003 Projection update model

**Decision:** Hybrid — extend existing `billing.freight_cost_outbox` ingest path for incremental per-order updates; add scheduled full rebuild job for reconciliation and dimension refresh.

**Status:** Accepted

---

### ADR-22-004 Canonical lane grain

**Decision:** Directional city→city lane within country, keyed with `transport_mode` and `equipment_type`. No standalone lane table in v2.2B.

**Status:** Accepted

---

### ADR-22-005 Benchmark cohort

**Decision:** Tenant-only historical benchmark. Cohort grain: `(tenant_id, lane_key, transport_mode, equipment_type, currency_code, period)`. No cross-tenant pricing exposure.

**Status:** Accepted

---

### ADR-22-006 Currency policy

**Decision:** No FX engine. All totals and benchmarks are currency-scoped. Multi-currency requests return `MIXED_CURRENCY` or per-currency breakdown — never a synthetic converted total.

**Status:** Accepted

---

### ADR-22-007 Carrier visibility

**Decision:** Carriers receive receivable-scoped read access only (`PolicyRead`). Buyer benchmarks, savings opportunities, competitor comparisons, and buyer total spend are excluded via server-side projection redaction (`PolicyBuyerAnalytics` routes).

**Status:** Accepted

---

### ADR-22-008 Data quality semantics

**Decision:** Use explicit quality enums: `AVAILABLE`, `PARTIAL`, `NOT_AVAILABLE`, `INSUFFICIENT_SAMPLE`, `STALE`, `MIXED_CURRENCY`. Absent data must not serialize as zero or empty list.

**Status:** Accepted

---

### ADR-22-009 Enrichment strategy

**Decision:** Hybrid dimension snapshots — at projection build time, batch-fetch company names and order references; store snapshot columns in analytics projections. Scheduled rebuild refreshes display labels; financial IDs remain stable UUIDs. No browser or API N+1 live lookup.

**Status:** Accepted

---

## 16. References

| Artifact | Path |
|----------|------|
| v2.1 architecture | `docs/engineering/FREIGHT_COST_MANAGEMENT_v2.1_ARCHITECTURE.md` |
| Ledger migration | `infrastructure/migrations/000054_freight_cost_ledger_v2.1B.up.sql` |
| Outbox events | `services/billing-register-service/internal/domain/freight_cost_outbox.go` |
| RBAC | `services/api-gateway/internal/freightcostrbac/policies.go` |
| Control Tower rebuild pattern | `infrastructure/migrations/000015_create_control_tower_shipment_status_projection_v0.1.up.sql` |
