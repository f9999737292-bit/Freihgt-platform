# Analytics carrier score policy question 0.1

Question sheet only. This document does not answer the questions, assign a weight, select a component, publish a score, change a KPI formula, add an API, add a migration, or deploy.

```
STAGE=ANALYTICS-CARRIER-SCORE-POLICY-QUESTION-0.1
BASE_MAIN_SHA=2a99b615a237b6d1f6e6038ca86a7433c567dc78
MODE=DOCS_ONLY
INSPECTION=STATIC_SOURCE
RUNTIME_KPI_QUERY=NOT_RUN
CARRIER_SCORE_POLICY_DECISION=DEFER
POLICY_ANSWERED=NO
QUESTIONS_OPEN=YES
WEIGHTS_ASSIGNED=NO
SCORE_IMPLEMENTED=NO
CARRIER_SCORE_READY=NO
GAP_E_001=OPEN
PRODUCT_CODE_CHANGED=NO
MIGRATIONS_CHANGED=NO
MIGRATION_HEAD_OBSERVED=000105
```

Analytics-0.3C recorded `CARRIER_SCORE_POLICY_DECISION=DEFER` and `NEXT_STAGE=WAIT_FOR_CARRIER_SCORE_BUSINESS_POLICY`. This sheet is that wait, written as questions for the business controller. Every answer below is `NOT_PROVIDED`.

## Sources

All citations are the tree at `BASE_MAIN_SHA`. No later commit is an input.

| Source | What it establishes |
| --- | --- |
| `docs/architecture/analytics/ANALYTICS_0_3C_CARRIER_SCORE_POLICY.md` | Score deferred. Two punctuality rates are not selected components. Weights `NOT_FOUND`. |
| `docs/architecture/analytics/ANALYTICS_0_3A_OPERATIONS_CARRIER_SCOPE.md` | Pickup formula freeze: both timestamps present, `actual_pickup_at <= planned_pickup_at`, grace 0. GAP-E-001 open. |
| `docs/architecture/analytics/ANALYTICS_0_1B_ARCHITECTURE.md` | Delivery grace 0. Score weights must be explicit and versioned. `CARRIER_SCORE_READY=NO`. |
| `docs/architecture/analytics/ANALYTICS_0_1A_KPI_CATALOG.md` | `CAR_PERFORMANCE_SCORE` is a weighted score whose weights were not found. `EXEC_CARRIER_PERFORMANCE` is a pointer to that score. |
| `docs/architecture/analytics/ANALYTICS_0_4A_KPI_SOURCE_CONTRACTS.md` | `CURRENT_KPI_COUNT=11`. GAP-C-001, GAP-C-002, GAP-C-005, and GAP-B-001 are `CONTRACT_REQUESTED` and unimplemented. GAP-E-001 stays `OPEN`. |
| `docs/architecture/analytics/adr/ADR-AN-001-canonical-kpi-ownership.md` | Agent E owns KPI meaning after promotion. Analytics does not invent a missing source fact. |
| `services/analytics-service/internal/kpi/kpi.go` | Implemented ids, `definitionVersion=1`, carrier envelope, ratio null rule, `completeness=PARTIAL`, `dataFreshness.status=UNKNOWN`. |
| `services/shipment-service/internal/repository/operations_analytics_source_repository.go` | Current-state SQL for shipment totals and per-carrier counts. |
| `services/shipment-service/internal/service/operations_analytics_source_service.go` | Nil carrier id rejected. Duplicate carrier id rejected. Numerator above denominator rejected. |
| `services/api-gateway/internal/http/router.go` | Public route `GET /api/v1/analytics/kpis/{kpiId}`. |
| `services/api-gateway/internal/analyticsrbac/policy.go` | Read roles. `DRIVER` is absent from `readRoles`. |
| `docs/freight-cost-management/v2.2/FINAL_CLOSURE.md` | FCI v2.2G.1 technical closure. Flags default off. `PRODUCTION_ROLLOUT=NO`. |
| `services/freight-cost-service/internal/config/config.go` | `FREIGHT_COST_ANALYTICS_PROJECTION_ENABLED` defaults to `false`. |

`services/freight-cost-service` at this SHA has no reference to `CAR_PERFORMANCE_SCORE`, `CAR_ON_TIME_PICKUP_RATE`, or `CAR_ON_TIME_DELIVERY_RATE`.

## Implemented punctuality facts

These two KPI ids are implemented. They are not a score.

| Field | `CAR_ON_TIME_PICKUP_RATE` | `CAR_ON_TIME_DELIVERY_RATE` |
| --- | --- | --- |
| Status | IMPLEMENTED | IMPLEMENTED |
| `definitionVersion` | 1 | 1 |
| Formula | Same counts as `OPS_ON_TIME_PICKUP_RATE`, grouped by `carrier_company_id` | Same counts as `OPS_ON_TIME_DELIVERY_RATE`, grouped by `carrier_company_id` |
| Denominator | Non-deleted shipment with both `planned_pickup_at` and `actual_pickup_at` | Non-deleted shipment with both `planned_delivery_at` and `actual_delivery_at` |
| Numerator | Denominator row where `actual_pickup_at <= planned_pickup_at` | Denominator row where `actual_delivery_at <= planned_delivery_at` |
| Grace | 0 | 0 |
| Measure type | `RATIO` | `RATIO` |
| Completeness | `PARTIAL` | `PARTIAL` |
| Freshness | `UNKNOWN` | `UNKNOWN` |
| Dimension | `CARRIER` | `CARRIER` |

`PARTIAL` stays because actual pickup and delivery timestamps are actor-supplied `occurred_at`. That reason is already frozen. This sheet does not reclassify it as `COMPLETE`.

The public route is `GET /api/v1/analytics/kpis/{kpiId}` on api-gateway. The gateway authorizes a read role and forwards the body. It does not compute the KPI. `Known` in analytics-service accepts exactly eleven ids. `CAR_PERFORMANCE_SCORE` is not one of them.

`readRoles` contains `PLATFORM_ADMIN`, `PROCUREMENT_MANAGER`, `SHIPPER_ADMIN`, `SHIPPER_LOGIST`, `FORWARDER_MANAGER`, `FINANCE_MANAGER`, `CARRIER_ADMIN`, `CARRIER_DISPATCHER`, and `CARRIER_ACCOUNTANT`. `DRIVER` is not in that map.

Carrier rows come from `GET /internal/v1/analytics/operations-foundation-v2` facts in one shipment-owned statement:

- Tenant predicate is `tenant_id = $1` on `transport.shipments`. The service sets that tenant from the verified caller and rejects a nil tenant.
- Carrier aggregate requires `deleted_at IS NULL` and `carrier_company_id IS NOT NULL`.
- Shipment-level operations counts do not require `carrier_company_id`. A shipment with no carrier can enter `OPS_SHIPMENTS_TOTAL` and the operations on-time populations. It cannot enter a carrier row.
- A carrier with only shipments that lack the timestamps for a leg still appears. That leg's numerator and denominator are 0.
- Items are ordered by `carrierCompanyId` string in analytics-service.
- There is no carrier filter and no query parameter on the public route.

Ratio rule already implemented, and not a score rule: denominator 0 with numerator 0 yields `value=null` and still returns the item. Numerator not 0 with denominator 0 is an error in analytics-service. The shipment validator rejects a numerator above its denominator before that pair is served. Empty `carriers` yields `items=[]`.

The service is stateless. These rates are current-state aggregates. They are not a period, a trend, or an as-of score.

```
CURRENT_KPI_COUNT=11
IMPLEMENTED_CARRIER_KPI_COUNT=2
HISTORY_CLASS=CURRENT_STATE
DWH_IMPLEMENTED=NO
ANALYTICS_STATELESS=YES
HISTORICAL_ANALYTICS_READY=NO
```

Live response of the route against a running stack was not executed. `RUNTIME_KPI_QUERY=NOT_RUN`.

## Score names that stay unpublished

| KPI_ID | Catalog state | This sheet |
| --- | --- | --- |
| `CAR_PERFORMANCE_SCORE` | BLOCKED. Weighted score. Weights `NOT_FOUND`. Exclusion: do not invent weights from on-time and OTIF. Blocker GAP-E-001. | NOT_PUBLISHED |
| `EXEC_CARRIER_PERFORMANCE` | BLOCKED. Pointer to `CAR_PERFORMANCE_SCORE`. Exclusion: do not average partial rates and call the result the score. | NOT_PUBLISHED |

0.3C reviewed 16 carrier catalog rows. Two are implemented punctuality rates and were not selected as score components. Five are `BLOCKED_SOURCE`: `CAR_REJECTION_RATE` (GAP-C-005), `CAR_OTIF` (GAP-C-001), `CAR_DELIVERY_REJECTION_RATE`, `CAR_DOCUMENT_COMPLETENESS` (GAP-B-001), `CAR_POD_COMPLETENESS` (GAP-C-002). Nine others, including the score itself, are `DEFERRED_DEFINITION`. 0.4A later marked the four source gaps `CONTRACT_REQUESTED`. That marker means the consumer request exists. It does not mean the fact exists. Implementation of those gaps is `NO`.

No repository rule gives the two rates a business name that means "punctuality only" under `CAR_PERFORMANCE_SCORE`.

RFx scoring weights in `docs/rfx-v3/adr/ADR-RFX-004-SCORING-ARCHITECTURE.md` score an RFx response. They do not name `CAR_PERFORMANCE_SCORE`.

## Freight Cost Intelligence boundary

FCI is a different product. It is not an input to this question sheet's score.

| Item | State | Evidence |
| --- | --- | --- |
| FCI v2.2G.1 | IMPLEMENTED as technical closure | PR #60 merged 2026-08-24, `test/freight-cost-intelligence-v2.2g1-closure-remediation` |
| `V2_2_TECHNICAL_COMPLETE` | YES in `FINAL_CLOSURE.md` | Same document. This stage did not re-run that CI. |
| `PRODUCTION_ROLLOUT` | NO | `FINAL_CLOSURE.md` |
| `FREIGHT_COST_ANALYTICS_PROJECTION_ENABLED` | Default `false` | `services/freight-cost-service/internal/config/config.go` |
| `NUXT_PUBLIC_FREIGHT_COST_WORKSPACE_ENABLED` | Default off | `docs/freight-cost-management/v2.2/FINAL_CLOSURE.md` and `apps/web-procurement/nuxt.config.ts` |
| PR #51 | OPEN. Do not resume. | `review/freight-cost-management-v2.1d-final-gate` is 1274 commits behind `origin/main` at this SHA |
| PR #61 | OPEN. Do not resume. | `ops/freight-cost-intelligence-controlled-rollout-v2.2` is 1182 commits behind `origin/main` at this SHA |

Spend, benchmark, accessorial, and opportunity projections are not catalog components of `CAR_PERFORMANCE_SCORE`. This stage does not add them.

## Controller questions

The controller is the business owner who can supply the missing policy. Agent E does not supply it here. An answer that names a weight, a threshold, or a component set belongs in a later document. It does not belong in this file.

Answer shape, when a later stage is authorized, is integer basis points per included component that sum to 10000, each component's KPI id and definition version, plus the policies named in the questions. Implicit renormalization is not a default. Equal weights are not a default. Convenience is not evidence.

### Q1. May the two implemented rates be score components?

```
QUESTION_ID=Q1
SUBJECT=CAR_ON_TIME_PICKUP_RATE,CAR_ON_TIME_DELIVERY_RATE
DEFINITION_VERSION=1
ANSWER=NOT_PROVIDED
```

For each id, the controller would have to say `YES`, `NO`, or `DEFER`. `YES` would select that id as a component of a future score version. This sheet selects neither.

### Q2. Is any unimplemented catalog component required first?

```
QUESTION_ID=Q2
ANSWER=NOT_PROVIDED
```

The controller would have to say whether `CAR_PERFORMANCE_SCORE` stays blocked until one or more of the blocked or deferred catalog components exist. Candidates that are not implemented are listed in 0.3C. GAP-C-001, GAP-C-002, GAP-C-005, and GAP-B-001 remain unimplemented. This sheet does not make them mandatory and does not drop them.

### Q3. What are the weights?

```
QUESTION_ID=Q3
WEIGHT_UNIT=BASIS_POINTS
WEIGHT_SUM_REQUIRED=10000
WEIGHTS_ASSIGNED=NO
ANSWER=NOT_PROVIDED
```

A weight is in scope only after Q1 and Q2 name a component set. 0, 5000, 10000, and every other split are unevidenced. This sheet assigns none.

### Q4. May the existing score id mean punctuality only?

```
QUESTION_ID=Q4
KPI_ID=CAR_PERFORMANCE_SCORE
ANSWER=NOT_PROVIDED
NEW_KPI_ID_PROPOSED=NO
```

The catalog name is "Carrier performance score" and the exclusion rule forbids inventing weights from on-time and OTIF. Publishing the two rates under that id would hide the punctuality limit unless the controller explicitly accepts that meaning for one named definition version. This sheet does not accept it and does not create a replacement KPI id.

### Q5. What happens when a selected component is missing?

```
QUESTION_ID=Q5
MISSING_COMPONENT_POLICY=DEFERRED
ANSWER=NOT_PROVIDED
```

0.3C evaluated these policies and selected none: `FAIL_SCORE`, `NULL_SCORE`, `PARTIAL_SCORE`, `RENORMALIZE_AVAILABLE_COMPONENTS`. The controller would have to name one of those or a different explicit rule. Renormalizing the two rates so that one available rate equals the score is the blend `EXEC_CARRIER_PERFORMANCE` excludes. This sheet does not choose it.

### Q6. What is the score outcome for an empty leg?

```
QUESTION_ID=Q6
ANSWER=NOT_PROVIDED
```

Separate cases, all without a score outcome today: pickup denominator 0, delivery denominator 0, one component unavailable, both unavailable. The implemented ratio already returns `value=null` for 0/0. That null is not `CAR_PERFORMANCE_SCORE`.

### Q7. Is there a minimum sample?

```
QUESTION_ID=Q7
MIN_SAMPLE_POLICY=DEFERRED
ANSWER=NOT_PROVIDED
```

No shipment count was found. 1, 2, 5, 10, and "no minimum" are all unevidenced. The controller would have to name a count or state that no minimum applies. This sheet does neither. 0.3C keeps `MIN_SAMPLE_BLOCKS_PUBLICATION=YES` until that policy exists.

### Q8. What scale is published?

```
QUESTION_ID=Q8
SCORE_SCALE=DEFERRED
ANSWER=NOT_PROVIDED
```

The implemented component scale is a decimal `RATIO`, not a percent. 0..100 and 0..10000 were not found as business requirements for this KPI. Basis points are the required weight unit. They are not thereby the published score unit. One later definition version can have one scale.

### Q9. Is current state enough?

```
QUESTION_ID=Q9
ANSWER=NOT_PROVIDED
HISTORICAL_SCORE_DEFINED=NO
PERIOD_SCORE_DEFINED=NO
```

The only inputs are current-state rows. A past window, a trend, or an as-of score needs historical analytics storage. That storage is not implemented: `DWH_IMPLEMENTED=NO` and `ANALYTICS_STATELESS=YES`. The controller would have to say whether a current-state score is acceptable. A requirement for a window stops until a later stage authorizes storage. This sheet does not authorize storage.

### Q10. Are PARTIAL inputs acceptable for a score?

```
QUESTION_ID=Q10
ANSWER=NOT_PROVIDED
```

Both implemented rates are `completeness=PARTIAL` and `dataFreshness.status=UNKNOWN`. No maximum age is approved, so `FRESH` and `STALE` are not available. The controller would have to say whether a score may be published from those inputs. This sheet does not promote completeness to `COMPLETE`.

### Q11. Which population is the carrier population?

```
QUESTION_ID=Q11
ANSWER=NOT_PROVIDED
```

Facts already implemented, which a later policy must either accept or replace with a new source contract:

- Null `carrier_company_id` is excluded from carrier rows and included in shipment-level operations counts.
- Deleted shipments are excluded from both.
- A leg enters its denominator only when both planned and actual timestamps are present.
- A carrier with no such leg is still returned, with 0/0 and `value=null` for that rate.
- Grace stays 0. A non-zero grace is a new component definition version, not a weight.
- The rates are not Control Tower `ON_TIME`.

This sheet does not change that population.

### Q12. Do FCI measures enter the score?

```
QUESTION_ID=Q12
ANSWER=NOT_PROVIDED
FCI_MEASURES_SELECTED=NO
```

No FCI spend, benchmark, accessorial, or savings measure is a selected component. The controller would have to leave them out or name a new component contract. This sheet does not name one. PR #51 and PR #61 stay unmerged and are not the vehicle for that answer.

### Q13. Does an executive pointer stay blocked?

```
QUESTION_ID=Q13
KPI_ID=EXEC_CARRIER_PERFORMANCE
ANSWER=NOT_PROVIDED
EXEC_CARRIER_PERFORMANCE_READY=NO
```

The catalog defines this id as a pointer to `CAR_PERFORMANCE_SCORE`, not as an average of the two rates. Closing it also needs an execution grain, an execution revision, carrier attribution at that grain, and a historical snapshot or version. Current analytics has none of those. A later freeze of `CAR_PERFORMANCE_SCORE` would not by itself close this id. This sheet does not close it.

### Q14. What rounding does a score version use?

```
QUESTION_ID=Q14
ROUNDING_POLICY=NOT_PUBLISHED
ANSWER=NOT_PROVIDED
```

Component ratios use a finite decimal of up to 16 places with trailing zeros trimmed. That formatting is not a score rounding policy.

## Dependencies

| Dependency | State | Effect on this sheet |
| --- | --- | --- |
| Business controller answers to Q1–Q14 | NOT_PROVIDED | GAP-E-001 stays open. No score implementation starts. |
| Agent C source facts GAP-C-001, GAP-C-002, GAP-C-005 | CONTRACT_REQUESTED, implementation NO | Required only if a later answer selects those components. Not selected here. |
| Agent B document fact GAP-B-001 | CONTRACT_REQUESTED, implementation NO | Same. Blocks `CAR_DOCUMENT_COMPLETENESS` only. |
| Historical analytics storage | NOT_IMPLEMENTED | Blocks a period score. Does not block writing this question sheet. |
| FCI v2.2G.1 | Technical closure recorded. Production rollout NO. | Not a score input. Rollout PR #61 is not resumed. |
| PR #322 | Draft, head `5c6a44f9d6adbc1b606b3e49b0168f3d103f90d5` | Out of scope. This stage does not change its files. |

Migration head observed on this SHA is `000105_platform_postgres_role_foundation_v0_2_i1`. No `000106` file is in `infrastructure/migrations`. This stage does not add one.

## Open gaps

```
GAP_E_001=OPEN
GAP_C_001=CONTRACT_REQUESTED
GAP_C_002=CONTRACT_REQUESTED
GAP_C_005=CONTRACT_REQUESTED
GAP_B_001=CONTRACT_REQUESTED
CARRIER_SCORE_COMPONENT_SET_READY=NO
CARRIER_SCORE_WEIGHTS_READY=NO
CAR_PERFORMANCE_SCORE_READY=NO
EXEC_CARRIER_PERFORMANCE_READY=NO
```

0.1A readiness totals stay `KPI_TOTAL=114`, `READY=17`, `PARTIAL=65`, `BLOCKED=23`, `NOT_SUPPORTED=9`, `OVERALL=0.434`. This sheet closes none of them.

## Next stage

```
NEXT_RECOMMENDED_STAGE=WAIT_FOR_CONTROLLER_ANSWERS
ANALYTICS_0_3D_AUTHORIZED=NO
SCORE_IMPLEMENTATION_AUTHORIZED=NO
SOURCE_IMPLEMENTATION_AUTHORIZED=NO
FCI_ROLLOUT_AUTHORIZED=NO
```

The next useful document is a policy freeze that copies controller answers to Q1–Q14 without inventing any blank. That freeze is not authorized by this sheet. Analytics-0.3D implementation remains unauthorized until a freeze names the component KPI ids, their definition versions, integer basis-point weights summing to 10000, inclusion, missing-component behavior, minimum sample, scale, time basis, and completeness acceptance. A freeze that needs a period window or a missing source fact stops on that dependency instead of implementing the score.
