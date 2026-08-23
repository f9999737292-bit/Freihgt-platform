# FREIGHT COST INTELLIGENCE v2.2 — Implementation Plan

**Status:** Proposed decomposition (v2.2A freeze)  
**Date:** 2026-08-23  
**Prerequisite:** v2.1E merged (PR #52 @ `e80af52`)

---

## 1. Overview

v2.2 is split into incremental releases after this architecture freeze. **v2.2A delivers docs only.** Implementation starts at v2.2B.

```text
v2.2A  Architecture & contract freeze     ← THIS PR
v2.2B  Analytics projection core
v2.2C  Lane & carrier intelligence
v2.2D  Accessorial intelligence & enrichment
v2.2E  Tenant benchmarking & savings
v2.2F  Public analytics API & workspace UI
v2.2G  Security, performance, rebuild & E2E closure
```

Order rationale: projection core must exist before dimension aggregations; enrichment batch APIs before lane/carrier labels; benchmarks require stable projections; public API last.

---

## 2. v2.2B — Analytics Projection Core

**Goal:** Rebuildable order-level analytics fact projection.

### Deliverables

- [ ] Migration: `freight_cost_analytics.order_fact_projection` (conceptual DDL from ARCHITECTURE.md)
- [ ] `AnalyticsProjectionService` — build from `cost_summary_projection`
- [ ] Extend ingest hook: after summary update → enqueue order fact refresh
- [ ] Scheduled full rebuild job (control-tower pattern)
- [ ] Projection metadata: `calculated_at`, `data_through`, `projection_version`
- [ ] Idempotency tests (replay, out-of-order, superseded rows)
- [ ] Internal admin endpoint: rebuild tenant / transport order (internal only)

### Dependencies

- None beyond v2.1 ledger

### Exit criteria

- [x] Full tenant rebuild reproduces identical order facts from canonical sources
- [x] Superseded ledger rows never double-count
- [x] CI integration test for rebuild parity

**Status:** Implemented in v2.2B (see `v2.2B-PROJECTION-CORE.md`).

---

## 3. v2.2C — Lane & Carrier Intelligence

**Goal:** Period aggregations by lane and carrier.

### Deliverables

- [ ] Migration: `lane_period_projection`, `carrier_period_projection`
- [ ] Lane key builder from `transport_orders` + `locations` (city grain ADR-22-004)
- [ ] Aggregation job from `order_fact_projection`
- [ ] Internal read queries for workspace/dev validation
- [ ] Handle missing city → exclude from lane cohort with audit counter

### Dependencies

- v2.2B order fact projection
- Transport order + location read access (existing DB or internal API)

### Exit criteria

- Lane/carrier aggregates match manual SQL spot-check on seed data
- Tenant isolation verified

**Status:** Implemented in v2.2C (see `v2.2C-LANE-CARRIER-INTELLIGENCE.md`).

**Note:** v2.2C adds `POST /internal/v1/transport-orders/batch-analytics-dimensions` (dimension-only). v2.2D `batch-summary` with display fields remains planned.

## 4. v2.2D — Accessorial Intelligence & Dimension Enrichment

**Goal:** Accessorial analytics + authoritative display enrichment.

### Deliverables

- [x] Migration: `accessorial_period_projection`
- [x] Batch internal API: `company-service` — `POST /internal/v1/companies/batch-get`
- [x] Batch internal API: transport analytics dimensions extended with `order_number`
- [x] Billing batch read for accessorial lines by transport_order_ids
- [x] Populate snapshot columns: `carrier_display_name`, `order_reference`, `lane_label`
- [x] Accessorial aggregation with pinned `mapping_version`
- [x] Replace workspace UUID labels via order fact snapshots

### Dependencies

- v2.2B, v2.2C
- billing `settlement_accessorials` read path

### Exit criteria

- [x] Accessorial totals reconcile with settlement `approved_accessorial_total` sample
- [x] Batch enrichment (no per-order HTTP in rebuild path when DB readers / clients wired)

**Status:** Implemented in v2.2D (see `v2.2D-ACCESSORIAL-ENRICHMENT.md`).

---

## 5. v2.2E — Tenant Benchmarking & Savings Opportunities

**Goal:** Explainable tenant-only benchmarks and rule-based opportunities.

### Deliverables

- [x] Configurable `min_benchmark_sample` (default 5)
- [x] Benchmark calculator: median, p25, p75, p90 per cohort (ADR-22-005)
- [x] Migration: `benchmark_projection`, `opportunity_projection` (000064)
- [x] Rule engine: LANE_COST_OUTLIER, COST_ABOVE_LANE_MEDIAN, HIGH_ACCESSORIAL_RATE, REPEATED_VARIANCE, CARRIER_COST_OUTLIER (lane-normalized)
- [x] Exclude CLASS C types (no ML, no market benchmark)
- [x] `INSUFFICIENT_SAMPLE` semantics on small cohorts
- [x] Internal S2S validation reads for benchmarks and opportunities

### Dependencies

- v2.2C lane/carrier projections
- v2.2D accessorial projection

### Exit criteria

- [x] Every opportunity has evidence JSON + sample_size + currency
- [x] No savings percentage without observed/baseline amounts
- [x] Full rebuild ≡ incremental equivalence (FC22E-EQV-001)

**Status:** Implemented in v2.2E (see `v2.2E-BENCHMARK-SAVINGS.md`).

---

## 6. v2.2F — Public Analytics API & Intelligence Workspace

**Goal:** Ship buyer-facing analytics under frozen contract.

### Deliverables

- [ ] Gateway routes: `/api/v1/freight-costs/analytics/*`, `/opportunities`
- [ ] RBAC: `PolicyBuyerAnalytics` on all routes
- [ ] OpenAPI entries + parity gate
- [ ] Frontend tabs: Overview, Lanes, Carriers, Accessorials, Opportunities (CLASS A/B only)
- [ ] Preserve `NOT_AVAILABLE` semantics — no screens for CLASS C (budget, cost/km, forecast)
- [ ] Feature flag: remain default OFF until v2.2G sign-off

### Dependencies

- v2.2B–E

### Exit criteria

- Contract tests match ANALYTICS_CONTRACT.md
- Workspace lanes/accessorials return real data (not stub NOT_AVAILABLE)

---

## 7. v2.2G — Security, Performance, Rebuild & E2E Closure

**Goal:** Production readiness without enabling feature flag by default.

### Deliverables

- [ ] FC-D-SEC-011..015 carrier/benchmark leakage tests
- [ ] Performance: EXPLAIN on tenant+period queries; index validation
- [ ] Disaster recovery runbook: full tenant rebuild
- [ ] Load test: bounded pagination under 100k order tenants (synthetic)
- [ ] Documentation update + FC test inventory
- [ ] CI job: `freight-cost-analytics-e2e`

### Exit criteria

- CI green
- CRITICAL/HIGH security findings = 0
- Rebuild SLA documented

---

## 8. Explicitly Deferred (post-v2.2)

| Item | Reason |
|------|--------|
| Distance / cost-per-km | No authoritative distance source |
| Pallet / LDM unit costs | No cargo fields |
| Budget vs actual | No budget entity |
| Forecast runtime | Prerequisites incomplete |
| Cross-tenant market benchmark | Policy + legal |
| FX engine | Not in platform |

---

## 9. Risk Register

| Risk | Mitigation | Phase |
|------|------------|-------|
| Lane city nulls reduce sample | Fallback to region grain label; exclude from benchmark | v2.2C |
| Company rename stale labels | Scheduled snapshot refresh | v2.2D |
| Mapping version drift | Pin version on attribution + rebuild | v2.2B/E |
| Carrier data leak | Server-side redaction + SEC E2E | v2.2G |
| Projection stale | Freshness fields + STALE quality | v2.2B |

---

## 10. Success Metrics

- Buyer can view lane/carrier spend with median benchmark (tenant-only)
- Carrier cannot access buyer benchmark API (403)
- Accessorial breakdown by normalized category
- ≥1 explainable opportunity type live with evidence
- Full rebuild completes deterministically

---

## 11. v2.2A Deliverables (Complete)

- [x] `ARCHITECTURE.md`
- [x] `DATA_READINESS.md`
- [x] `ANALYTICS_CONTRACT.md`
- [x] `SECURITY.md`
- [x] `IMPLEMENTATION_PLAN.md`
- [x] ADR-22-001..009 in ARCHITECTURE.md

**STOP_AFTER_V2_2A=YES** — no v2.2B code in this branch.
