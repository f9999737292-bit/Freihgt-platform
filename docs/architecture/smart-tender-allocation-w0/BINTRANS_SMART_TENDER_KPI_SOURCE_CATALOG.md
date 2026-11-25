# BINTRANS Smart Tender — KPI Source Catalog (Spec v0.9)

**Decision D4:** EVIDENCE_BASED_KPI  
**Main:** `892361db87985f35132aef86550a025909af648a`  
**Rule:** Missing KPI ≠ fake. Production KPI Award forbidden without real sources.

Required fields per metric: SOURCE_SERVICE, SOURCE_TABLE_OR_EVENT, TENANT_SCOPE, COMPANY_SCOPE, FORMULA, OBSERVATION_COUNT, FRESHNESS, DATA_CONFIDENCE, MISSING_DATA_POLICY.

Fallbacks: FAIL_ELIGIBILITY | FALLBACK_OVERALL_KPI | FALLBACK_COMPARABLE_LANE | RESERVE_ONLY | PILOT_VOLUME_CAP | MANUAL_REVIEW

---

## Catalog (baseline honesty)

| ID | Metric | Status |
|---|---|---|
| K1 | Overall KPI | NOT_IMPLEMENTED (no RFx tender KPI API). Adjacent CT projections not wired |
| K2 | Lane KPI (directional) | NOT_IMPLEMENTED. Adjacent `freight_cost.cost_analytics_benchmark_projection` is cost, not SLA |
| K3 | On-time pickup | NOT_IMPLEMENTED (tracking/CT stops not aggregated for tender) |
| K4 | On-time delivery | NOT_IMPLEMENTED |
| K5 | Refusal rate | NOT_IMPLEMENTED |
| K6 | Claim rate | NOT_IMPLEMENTED |
| K7 | Document quality | NOT_IMPLEMENTED; document write P0 open — do not expand writes for KPI |
| K8 | Capacity availability | NOT_IMPLEMENTED until commitment/capacity (W6) |
| K9 | Bid submission timeliness | N/A until commercial rounds exist |
| K10 | Guarantee performance | N/A until ledger + execution (W6/W8) |

### Legacy non-KPI score

`ComputeCommercialScores` / 70/30 in `rfx_evaluation.go` — price/manual only; frozen; not evidence-based lane KPI.

---

## W1/W2 policy (Controller D4)

- AST may reference KPI ids only if catalog row exists.  
- If sources still NOT_IMPLEMENTED: **fail-closed** HARD filters that need KPI, **or** run **explicitly labeled test simulation** (not production Award).  
- Do not declare KPI-based Award production-ready until K1/K2 (at minimum for MVP) have verified sources with all evidence fields.

---

## Runtime closure

Design catalog = remediates J-STA-002 at spec level.  
Runtime closure = CI-proven reads from real sources — future wave.
