# RFx / Tender Platform v0.3 — Enterprise Tendering, Scoring & Quota Allocation

## Overview

Extends authoritative **`rfx-service`** with three **independent** domain concepts:

```text
SCORING TEMPLATE  +  ALLOCATION SCENARIO  +  QUOTA BALANCE POLICY
```

These must not collapse into a single opaque formula.

## Product flow

```text
TENDER (rfx_events)
  ↓
QUALIFICATION (hard gates)
  ↓
SCORING (weighted ranking)
  ↓
ALLOCATION (volume shares)
  ↓
QUOTA BALANCE (target vs actual correction)
  ↓
AWARD PROPOSAL (recommendation)
  ↓
HUMAN APPROVAL
  ↓
AWARD
  ↓
TRANSPORT ORDER (mini-tender / freight-request path)
  ↓
SHIPMENT
  ↓
CONTROL TOWER (future KPI feedback — not wired in v0.3)
  ↺ future TENDER SCORING
```

## Scoring formula (deterministic)

For each qualified carrier:

```text
PriceScore       = (lowestPrice / carrierPrice) × 100
TransitScore     = (bestTransit / carrierTransit) × 100
CapacityScore    = min(100, capacity / requiredVolume × 100) when volume known
SLA/KPI/Reliability = clamped bid inputs (0..100)

TotalScore = Σ (RawFactorScore × Weight / 100)
```

Weights must sum to **100%** (±0.01). Factors are allow-listed; no arbitrary expressions.

## Qualification vs scoring

```text
Qualification = hard gate (DISQUALIFIED carriers excluded from allocation)
Scoring       = ranking among qualified carriers
```

Quota deficit **never** overrides failed mandatory qualification (e.g. SLA gate).

## Allocation strategies

Supported in v0.3 engine:

```text
WINNER_TAKES_MOST | DUAL_SOURCE | DIVERSIFIED | EQUAL_SPLIT
SCORE_WEIGHTED | CAPACITY_WEIGHTED | MANUAL
```

Constraints: min/max suppliers, min/max share, max carrier concentration, capacity limits.

## Quota balance

```text
balance_pp = target_share - actual_share
status     = UNDERALLOCATED | BALANCED | OVERALLOCATED (± tolerance)
adjustment = bounded by max_correction_pct when carry_balance=true
```

## Award governance

```text
DRAFT_PROPOSAL → PENDING_APPROVAL → APPROVED → AWARDED (finalize)
```

No auto-award in v0.3. System **proposes**; authorized user **approves**.

## Database (migration 000036)

New tables in schema `rfx`:

```text
scoring_templates, scoring_template_versions
tender_evaluations, tender_qualification_results, tender_carrier_scores
allocation_scenarios, allocation_results
quota_balance_policies, quota_balance_targets, quota_balance_positions, quota_ledger_entries
award_proposals, award_proposal_lines, awards, award_transport_orders
bid_revisions, rfx_response_revisions
```

## API (rfx-service)

```text
POST /v1/scoring-templates
POST /v1/rfx-events/{id}/evaluate
POST /v1/allocation-scenarios
POST /v1/award-proposals
POST /v1/award-proposals/{id}/submit|approve|finalize
```

## Carrier performance seam

```text
CarrierPerformanceProvider interface
```

v0.3 uses manual/test KPI inputs on bids/responses. Future Control Tower aggregated KPIs connect via service boundary — **no direct CT DB access**.

## Known limitations (v0.3)

- No real-time reverse auction
- No AI auto-award or negotiation
- No live Control Tower KPI feedback loop
- Enterprise RFx UI partial (evaluation panel foundation)
- Freight-request bid revision API/UI incremental

## Safety

```text
DO_NOT_DEPLOY=YES
GLOBAL_GUARDED_ACTIONS_ENABLED=false (unchanged)
Tenant-scoped templates, scenarios, quota, awards
Bid confidentiality enforced at list/get boundaries (carrier scope)
```
