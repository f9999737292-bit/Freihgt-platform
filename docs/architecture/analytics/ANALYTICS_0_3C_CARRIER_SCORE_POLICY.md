# Analytics-0.3C carrier performance score policy

Policy decision only. This document does not implement a score, change a KPI formula, add an API, add a migration, or deploy staging.

```
STAGE=ANALYTICS-0.3C
BASE_SHA=32031b2e71669cbface0dbb02893aba3f99cd0e8
CARRIER_SCORE_POLICY_DECISION=DEFER
CARRIER_SCORE_READY=NO
CARRIER_SCORE_WEIGHT_VERSIONING=YES
CARRIER_SCORE_COMPONENT_SET_READY=NO
CARRIER_SCORE_WEIGHTS_READY=NO
WEIGHT_EVIDENCE_FOUND=NO
CAR_PERFORMANCE_SCORE_READY=NO
EXEC_CARRIER_PERFORMANCE_READY=NO
GAP_E_001_STATUS=OPEN
NEXT_STAGE=WAIT_FOR_CARRIER_SCORE_BUSINESS_POLICY
```

`CARRIER_SCORE_READY=NO` and `CARRIER_SCORE_WEIGHT_VERSIONING=YES` stay as frozen in Analytics-0.1B. Versioning says that a future weight set must be explicit and tied to a score definition version. It is not evidence that any weight exists.

0.1A readiness totals stay `KPI_TOTAL=114`, `READY=17`, `PARTIAL=65`, `BLOCKED=23`, `NOT_SUPPORTED=9`, `OVERALL=0.434`. This decision does not close a catalog gap.

## Decision

`CAR_PERFORMANCE_SCORE` version 1 is not frozen.

The catalog name is "Carrier performance score". The catalog definition is a weighted score whose weights were not found. The exclusion rule is: do not invent weights from on-time and OTIF. `EXEC_CARRIER_PERFORMANCE` is a pointer to that score, not a blend of on-time rates, and its exclusion rule forbids averaging partial rates and calling the result the score.

The only implemented carrier KPI facts are:

```
CAR_ON_TIME_PICKUP_RATE
CAR_ON_TIME_DELIVERY_RATE
```

Both are punctuality rates. Both keep `definitionVersion=1` and completeness `PARTIAL`, because actual pickup and delivery timestamps are actor-supplied `occurred_at`. Two punctuality rates do not represent acceptance, rejection, cancellation, OTIF, delay, cargo condition, documents, POD, or exceptions.

No repository, product, architecture, or external rule gives those two rates a business name that means "punctuality only" under `CAR_PERFORMANCE_SCORE`. Publishing them under the existing performance-score name would hide that limit. A new punctuality-only KPI id is not created here.

## Component inventory

The 0.1A catalog has 16 carrier KPIs. `CAR_PERFORMANCE_SCORE` is the composite under decision. The other 15 are candidate components. A name in the catalog is not inclusion.

0.3A classes are kept. Analytics-0.3B then implemented the two rates that 0.3A called `PARTIAL_BUT_SAFE`. Their formulas were already frozen. Implementation does not make them a score.

| KPI_ID | 0.3C class | Why it is not a selected score component |
| --- | --- | --- |
| CAR_SHIPMENTS_ASSIGNED | DEFERRED_DEFINITION | `CARRIER_ASSIGNED` and a non-null `carrier_company_id` can diverge. Neither rule is selected. |
| CAR_SHIPMENTS_COMPLETED | DEFERRED_DEFINITION | Completion is not frozen among `DELIVERED`, `DELIVERY_CONFIRMED`, `DOCUMENTS_COMPLETED`, `READY_FOR_BILLING`, and `FINANCIALLY_CLOSED`. |
| CAR_ACCEPTANCE_RATE | DEFERRED_DEFINITION | `ACCEPTED_BY_CARRIER` exists. The offered-assignment denominator is not frozen. Partial status history is not a complete offer population. |
| CAR_REJECTION_RATE | BLOCKED_SOURCE | GAP-C-005. No assignment-rejection status. `CUSTOMER_REFUSAL` and an unanswered RFx invite are different facts. |
| CAR_CANCELLATION_RATE | DEFERRED_DEFINITION | `CANCELLED` exists. The actor is not proven to be the carrier on every row. Unknown actor is not a carrier cancellation. |
| CAR_ON_TIME_PICKUP_RATE | IMPLEMENTED | Analytics-0.3B. Same formula as `OPS_ON_TIME_PICKUP_RATE`, grouped by `carrier_company_id`. Completeness remains `PARTIAL`. Not selected as a score component. |
| CAR_ON_TIME_DELIVERY_RATE | IMPLEMENTED | Analytics-0.3B. Same formula as `OPS_ON_TIME_DELIVERY_RATE`, grouped by `carrier_company_id`. Completeness remains `PARTIAL`. Not selected as a score component. |
| CAR_OTIF | BLOCKED_SOURCE | GAP-C-001. In-full is not established by `DELIVERED`, a missing disposition case, or a missing attempt row. |
| CAR_AVG_DELAY_MIN | DEFERRED_DEFINITION | Pickup delay and delivery delay are not interchangeable. Early-delay policy is unfrozen. |
| CAR_DELIVERY_REJECTION_RATE | BLOCKED_SOURCE | Same quantity gap as `OPS_DELIVERY_REJECTION_RATE`. Not `CAR_REJECTION_RATE`. 0.1A catalog readiness was `PARTIAL`; Analytics-0.3A reclassified this fact as blocked. |
| CAR_CARGO_ISSUE_RATE | DEFERRED_DEFINITION | Disposition reason codes and driver-reported exceptions are not one set. A delivery with no case is not a known zero-issue delivery. |
| CAR_DOCUMENT_COMPLETENESS | BLOCKED_SOURCE | GAP-B-001. Agent B owns the required document set. |
| CAR_POD_COMPLETENESS | BLOCKED_SOURCE | GAP-C-002. `RequiresPOD` is hardcoded false. No required-POD fact. |
| CAR_EXCEPTION_RATE | DEFERRED_DEFINITION | Workflow exceptions and driver-reported exceptions stay different populations. |
| CAR_CRITICAL_EXCEPTION_RATE | DEFERRED_DEFINITION | Critical is not frozen as P1 or as any other code. |
| CAR_PERFORMANCE_SCORE | DEFERRED_DEFINITION | This is the score. Component set and weights are not selected. |

```
CARRIER_COMPONENTS_REVIEWED=16
IMPLEMENTED_COMPONENTS=2
FORMULA_FROZEN_SOURCE_READY=0
PARTIAL_COMPONENTS=0
BLOCKED_COMPONENTS=5
DEFERRED_COMPONENTS=9
NOT_SUPPORTED_COMPONENTS=0
```

`FORMULA_FROZEN_SOURCE_READY=0` because the two frozen carrier rates are already implemented. No other carrier KPI has both a frozen formula and a source contract that is ready to compute. `PARTIAL_COMPONENTS=0` because the former `PARTIAL_BUT_SAFE` pair is now implemented, and the remaining catalog `PARTIAL` rows were already `DEFERRED_BY_DEFINITION` or `BLOCKED_BY_SOURCE_GAP` in Analytics-0.3A.

## Weight evidence

No weight is assigned. These values were not found for `CAR_PERFORMANCE_SCORE`:

| Evidence class | Result |
| --- | --- |
| BUSINESS_POLICY | NOT_FOUND |
| EXISTING_PRODUCT_RULE | NOT_FOUND for this KPI |
| APPROVED_ARCHITECTURE_DECISION | Shape only. Analytics-0.1B requires explicit versioned weights and sets `CARRIER_SCORE_READY=NO`. |
| EXTERNAL_REQUIREMENT | NOT_FOUND |

```
WEIGHT_EVIDENCE_FOUND=NO
```

RFx scoring weights are a different product. `docs/rfx-v3/adr/ADR-RFX-004-SCORING-ARCHITECTURE.md` records a commercial/manual bid-evaluation weight. That rule scores an RFx response. It does not name `CAR_PERFORMANCE_SCORE`, pickup, or delivery.

Equal weights, 50/50, 60/40, and any other split are not used. Convenience is not evidence.

## Score version contract

No score version is published.

| Field | Value |
| --- | --- |
| SCORE_KPI_ID | NOT_PUBLISHED |
| DEFINITION_VERSION | NOT_PUBLISHED |
| BUSINESS_NAME | NOT_PUBLISHED |
| BUSINESS_DEFINITION | NOT_PUBLISHED |
| COMPONENT_KPI_IDS | NONE |
| COMPONENT_DEFINITION_VERSIONS | NOT_APPLICABLE |
| COMPONENT_WEIGHTS_BPS | NOT_APPLICABLE |
| WEIGHT_SUM_BPS | NOT_APPLICABLE |
| INCLUSION_RULE | NOT_PUBLISHED |
| MISSING_COMPONENT_POLICY | DEFERRED |
| ZERO_DENOMINATOR_POLICY | DEFERRED for a score. Implemented ratio components already return `value=null` when the denominator is 0 and the numerator is 0. |
| ROUNDING_POLICY | NOT_PUBLISHED |
| MIN_SAMPLE_POLICY | DEFERRED |
| TIME_BASIS | NOT_PUBLISHED |
| TENANT_SCOPE | NOT_PUBLISHED |
| CARRIER_DIMENSION | NOT_PUBLISHED |
| COMPLETENESS_POLICY | NOT_PUBLISHED |

```
SCORE_COMPONENT_COUNT=0
SCORE_WEIGHT_SUM_BPS=NOT_APPLICABLE
```

A later published score, if a business policy supplies one, still has to use integer basis points that sum to 10000, name each component KPI id and that component's definition version, and state inclusion. Implicit renormalization is not allowed unless that later policy freezes it in the same version.

## Missing data

The implemented carrier rates already have a component rule: denominator 0 with numerator 0 yields `value=null`; a non-zero numerator with denominator 0 is an error. Empty `carriers` yields `items=[]`. That rule is not a score rule.

These score outcomes were evaluated and not selected:

| Policy | Implication if selected without further evidence |
| --- | --- |
| FAIL_SCORE | One empty population fails the carrier's whole score. That treats pickup and delivery as jointly mandatory. No rule says that. |
| NULL_SCORE | The score is null when a required component is missing or its denominator is 0. Which components are required is itself unselected. |
| PARTIAL_SCORE | A score is published from the remaining components and marked partial. The unused weight still has to go somewhere. No rule says where. |
| RENORMALIZE_AVAILABLE_COMPONENTS | Remaining weights are rescaled to 10000. A single available rate would then equal the score. That is the blend `EXEC_CARRIER_PERFORMANCE` excludes. |

```
MISSING_COMPONENT_POLICY=DEFERRED
```

Pickup denominator 0, delivery denominator 0, one component unavailable, and both components unavailable therefore have no score outcome. They do not publish `CAR_PERFORMANCE_SCORE`.

## Minimum sample

No policy was found for how many shipments a carrier needs before a score may be published. 1, 2, 5, and 10 are all unevidenced. Choosing any of them, including choosing "no minimum", would invent a threshold.

```
MIN_SAMPLE_POLICY=DEFERRED
MIN_SAMPLE_BLOCKS_PUBLICATION=YES
```

## Score scale

Evaluated scales:

| Scale | Result |
| --- | --- |
| 0..1 ratio | Matches the implemented analytics `RATIO` measure. `CAR_ON_TIME_PICKUP_RATE` and `CAR_ON_TIME_DELIVERY_RATE` are decimal fractions, not percents. |
| 0..100 decimal | NOT_FOUND as a business requirement for this KPI. |
| 0..10000 fixed-point | NOT_FOUND as a business requirement for this KPI. Basis points remain the required weight unit for a future weight table. They are not selected as the published score unit. |

No score version exists, so no scale is attached to `CAR_PERFORMANCE_SCORE`.

```
SCORE_SCALE=DEFERRED
```

A later freeze must expose one scale for that version. The implemented ratio convention is the candidate unless a named business policy requires another scale. Two scales for one definition version are not allowed.

## Executive carrier performance

`EXEC_CARRIER_PERFORMANCE` stays unready. The catalog defines it as a pointer to `CAR_PERFORMANCE_SCORE`, not as an average of on-time rates.

Closing it would also need an execution grain, an execution revision, carrier attribution at that grain, and a historical snapshot or version. Current analytics has none of those. The current carrier rates group the current shipment row by `carrier_company_id`. That is not an execution score.

```
CAR_PERFORMANCE_SCORE_READY=NO
EXEC_CARRIER_PERFORMANCE_READY=NO
```

`EXEC_CARRIER_PERFORMANCE` stays open even if a later policy freezes only `CAR_PERFORMANCE_SCORE`.

## Historical semantics

Analytics remains stateless. The two implemented carrier rates are current-state aggregates from `GET /internal/v1/analytics/operations-foundation-v2`. They are not a period, trend, or as-of score.

A carrier score built only from those current-state inputs could be current-state. This stage does not publish that score. It does not define a past-period window. A later policy that needs a window stops and requires historical analytics storage.

```
HISTORY_CLASS=CURRENT_STATE
HISTORICAL_SCORE_DEFINED=NO
PERIOD_SCORE_DEFINED=NO
DWH_IMPLEMENTED=NO
ANALYTICS_STATELESS=YES
```

`HISTORY_CLASS=CURRENT_STATE` describes the only inputs that exist. It is not a published score contract.

## API shape

No API change.

The current carrier response, used by `CAR_ON_TIME_PICKUP_RATE` and `CAR_ON_TIME_DELIVERY_RATE`, is:

```
kpiId
definitionVersion
dimension=CARRIER
items[]
items[].carrierCompanyId
items[].measure
generatedAt
dataFreshness.status=UNKNOWN
completeness=PARTIAL
```

Items are ordered by `carrierCompanyId`. There is no carrier filter and no query parameter. The gateway forwards the analytics body and does not compute the KPI.

If a later policy makes `CAR_PERFORMANCE_SCORE` ready, the intended public shape reuses that envelope: `dimension=CARRIER`, one item per `carrierCompanyId`, and one measure. The current measure type `RATIO` means one numerator over one denominator. A weighted composite is not that ratio. This stage does not invent a measure type, a numerator, a denominator, or a filter.

```
API_CHANGED=NO
INTENDED_ENVELOPE_IF_LATER_READY=EXISTING_CARRIER_RESPONSE
MEASURE_TYPE_FOR_SCORE=NOT_FROZEN
QUERY_FILTERS=NONE
```

## Gap impact

```
GAP_E_001_STATUS=OPEN
GAP_C_001_STATUS=OPEN
GAP_C_002_STATUS=OPEN
GAP_C_005_STATUS=OPEN
```

GAP-E-001 stays open because the component set, the weights, the inclusion rule, the missing-component rule, and the minimum sample are not evidenced. GAP-C-001, GAP-C-002, and GAP-C-005 are source gaps. This policy does not close them.

## Next stage

```
NEXT_STAGE=WAIT_FOR_CARRIER_SCORE_BUSINESS_POLICY
ANALYTICS_0_3D_AUTHORIZED=NO
```

Analytics-0.3D carrier-score implementation waits until a business policy names the component KPI ids, their definition versions, integer basis-point weights summing to 10000, inclusion, missing-component behavior, and whether a minimum sample applies. This stage does not start that work.
