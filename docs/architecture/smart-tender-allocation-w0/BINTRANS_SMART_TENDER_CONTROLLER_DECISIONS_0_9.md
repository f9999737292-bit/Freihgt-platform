# BINTRANS Smart Tender Allocation — Controller Decisions Freeze v0.9

**Status:** CONTROLLER-APPROVED ARCHITECTURE DECISIONS  
**Date:** 2026-11-25  
**Main baseline:** `892361db87985f35132aef86550a025909af648a`  
**Source package:** W0 Contract Remediation 0.8A  
**Implementation:** NOT AUTHORIZED (W0 docs freeze only)

---

## Decision register

| ID | Topic | APPROVED value |
|---|---|---|
| D1 | Tender campaign type | `LEGACY_SINGLE_WINNER` **and** `ALLOCATION_CAMPAIGN` in parallel |
| D2 | Multi-carrier model | **OPTION_A** — Allocation Proposal → Decision → Carrier Commitment → Contract Appendix → TMS Activation; **no** synthetic legacy Award per carrier; **keep** `uq_rfx_award_event` |
| D3 | Share policy | Modes `CAP_ONLY`, `TARGET_WITH_CAP`; **DEFAULT=`CAP_ONLY`**. Preliminary share 0–100% is **not** an automatic obligation. `CONTRACTUAL_MINIMUM` only with separate contractual + legal basis |
| D4 | KPI | **EVIDENCE_BASED_KPI** — verified source, period, numerator/denominator, observation count, freshness, confidence, tenant/company scope. Missing KPI ≠ fake value. W1/W2 may fail-closed or use separately labeled test simulation. No KPI-based Award production without real sources |
| D5 | Money | **DECIMAL_FOR_NEW_TENDER_PATHS** — `shopspring/decimal` preferred; freeze units, NUMERIC scale, rounding, currency, tax basis, FX version. Legacy float64 unchanged in W0 |
| D6 | API namespace | **Prefix `/api/v1/rfx/allocations`** — tenant/company authz, RBAC/ABAC, idempotency, version preconditions, audit, fail-closed, stable errors. Legacy Award API unchanged |
| D7 | PARTIALLY_AWARDED | **NOT_USED_FOR_ALLOCATION_CAMPAIGNS** — separate Allocation Proposal/Decision FSM; do not reuse legacy Award lifecycle without a separate contract |

---

## Binding rules

1. Parallel domains: allocation never deletes or relaxes Single Winner Award contracts.  
2. OPTION_A forbids fictitious `rfx.rfx_awards` rows for multi-carrier winners.  
3. Default share mode for first working version: **CAP_ONLY**.  
4. Production KPI Award requires evidence-based catalog rows with real sources.  
5. New money paths are decimal-only; legacy scoring/award float64 frozen until a later GO.  
6. All new HTTP under `/api/v1/rfx/allocations…` (plus nested resources as specified in API plan).  
7. Allocation campaigns must not set or rely on event status `PARTIALLY_AWARDED`.

---

## Non-decisions (still open for product/legal, not blocking this freeze text)

- Exact MVP KPI subset for first pilot (which catalog IDs must leave NOT_IMPLEMENTED).  
- Legal force of e-signature types and silence≠consent (TZ §13.2 / §15.3).  
- Whether any reporting bridge to a single legacy award is ever added later (explicitly **out** of OPTION_A MVP).

---

## Authority

`CONTROLLER_DECISION=APPROVED_SEVEN_ARCHITECTURE_DECISIONS`  
`IMPLEMENTATION_ALLOWED=NO` until Controller GO for a specific wave after Agent J final review of v0.9.
