# Analytics-0.3A operations and carrier scope

Discovery and scope freeze. This document does not implement Analytics-0.3B.

Reviewed against `origin/main` `b6b1c5163aea80e99b11e991335f2f74bfc33cba`. 0.1A readiness totals stay unchanged. 0.1A evidence is not rewritten.

## Baseline

```
ANALYTICS_0_2B_MERGED=YES
ANALYTICS_0_2B_MERGE_SHA=b6b1c5163aea80e99b11e991335f2f74bfc33cba
ANALYTICS_SERVICE_IMPLEMENTED=YES
CURRENT_IMPLEMENTED_KPI_COUNT=5
DWH_IMPLEMENTED=NO
ANALYTICS_STORAGE_MODEL=HYBRID
DWH_REQUIRED_FOR_0_3=NO
DIRECT_SOURCE_DB_READ=NO
SOURCE_OWNED_INTERNAL_CONTRACTS=YES
ANALYTICS_DEFINES_KPI_SEMANTICS=YES
ANALYTICS_OWNS_SOURCE_FACTS=NO
TMS_SHIPMENT_EXECUTION_FACT_OWNER=Agent C
EDO_DOCUMENT_FACT_OWNER=Agent B
NLO_PLANNING_FACT_OWNER=Agent D
ANALYTICS_KPI_OWNER=Agent E
```

The five implemented KPIs stay at `definitionVersion=1`. Their meaning is not changed:

```
OPS_SHIPMENTS_TOTAL
OPS_ON_TIME_DELIVERY
OPS_ON_TIME_DELIVERY_RATE
OPS_RETURN_CASES
OPS_REDIRECT_CASES
```

They are not in the review counts below. The current source contract is `GET /internal/v1/analytics/operations-foundation`. It returns tenant totals only: `shipmentTotal`, `onTimeDeliveryDenominator`, `onTimeDeliveryNumerator`, `returnCaseCount`, `redirectCaseCount`. Analytics does not read `transport.shipments` itself.

## Classification

| Class | Meaning in this freeze |
| --- | --- |
| `READY_FOR_0_3` | Formula already frozen and the current source contract already returns the counts. |
| `PARTIAL_BUT_SAFE` | Formula already frozen in 0.1B. Columns exist. Completeness stays `PARTIAL` where the event time is actor-supplied `occurred_at`. A named source-contract extension is required before implementation. |
| `BLOCKED_BY_SOURCE_GAP` | A required fact is still absent. |
| `DEFERRED_BY_DEFINITION` | A business rule is still unfrozen. This freeze does not invent it. |
| `OUT_OF_SCOPE` | Outside operations and carrier analytics. None of the reviewed ids are in this class. |

`READY_FOR_0_3_COUNT=0`. The current contract does not expose pickup, late, or carrier-partitioned counts.

## Operations review

20 KPIs.

| KPI_ID | Class | Evidence |
| --- | --- | --- |
| OPS_SHIPMENTS_ACTIVE | DEFERRED_BY_DEFINITION | `ACTIVE_SHIPMENT_DEFINITION` stays deferred. Control Tower "not CANCELLED and not FINANCIALLY_CLOSED" is not this KPI. |
| OPS_ON_TIME_PICKUP | PARTIAL_BUT_SAFE | 0.1B froze the same shape as on-time delivery, using `planned_pickup_at` and `actual_pickup_at`. Both columns are on `transport.shipments`. The 0.2 contract does not return them. Completeness stays `PARTIAL`. |
| OPS_ON_TIME_PICKUP_RATE | PARTIAL_BUT_SAFE | Denominator is the same completed population: `deleted_at IS NULL` and both pickup timestamps present. Numerator is `actual_pickup_at <= planned_pickup_at`. Grace 0. Not Control Tower on-time. |
| OPS_LATE_PICKUP | PARTIAL_BUT_SAFE | Derived in analytics: `onTimePickupDenominator - onTimePickupNumerator`. That is the completed population where actual is after planned. In-flight overdue is not this count. It is not a source field. |
| OPS_AVG_PICKUP_DELAY_MIN | DEFERRED_BY_DEFINITION | Early-arrival treatment is not frozen. |
| OPS_LATE_DELIVERY | PARTIAL_BUT_SAFE | Derived in analytics: `onTimeDeliveryDenominator - onTimeDeliveryNumerator`. The Analytics-0.2 source already supplies both counts, so this KPI needs no new shipment-service field. Not in-flight `ReasonDeliveryOverdue`. |
| OPS_AVG_DELIVERY_DELAY_MIN | DEFERRED_BY_DEFINITION | Early-arrival treatment is not frozen. |
| OPS_OTIF | BLOCKED_BY_SOURCE_GAP | GAP-C-001. On-time alone is not OTIF. |
| OPS_DWELL_PICKUP_MIN | DEFERRED_BY_DEFINITION | `STOP_WAIT_TIME`, `STOP_SERVICE_TIME`, and `STOP_TOTAL_TIME` exist as intervals. None is selected as dwell. |
| OPS_DWELL_DELIVERY_MIN | DEFERRED_BY_DEFINITION | Same. |
| OPS_TRACKING_COVERAGE | DEFERRED_BY_DEFINITION | A location event, a binding, a recent ping, and a stop milestone are different facts. No one of them is selected. |
| OPS_POD_COMPLETENESS | BLOCKED_BY_SOURCE_GAP | GAP-C-002. |
| OPS_DELIVERY_REJECTION_RATE | BLOCKED_BY_SOURCE_GAP | Fleet quantity coverage is GAP-C-001. The rate grain is also unfrozen, so closing the quantity fact would still not make this KPI ready. |
| OPS_PARTIAL_REJECTION_RATE | BLOCKED_BY_SOURCE_GAP | Same quantity gap. Full rejection, partial rejection, and a delivery with no disposition command are different. |
| OPS_EXCEPTION_RATE | DEFERRED_BY_DEFINITION | Workflow exceptions and driver-reported exceptions stay different populations. |
| OPS_P1_EXCEPTION_RATE | DEFERRED_BY_DEFINITION | No P1 population is frozen. |
| OPS_P2_EXCEPTION_RATE | DEFERRED_BY_DEFINITION | No P2 population is frozen. |
| OPS_SLA_BREACH_RATE | DEFERRED_BY_DEFINITION | Control Tower SLA and analytics on-time stay different. Breach population is not frozen. |
| OPS_SHIPMENT_CYCLE_TIME | DEFERRED_BY_DEFINITION | Start and end events are not frozen. |
| OPS_DOCUMENT_COMPLETENESS | BLOCKED_BY_SOURCE_GAP | GAP-B-001. Agent B owns the required document set. |

Operations counts: `PARTIAL_BUT_SAFE=4`, `BLOCKED_BY_SOURCE_GAP=5`, `DEFERRED_BY_DEFINITION=11`.

## Carrier review

16 KPIs. A carrier KPI reuses the operations formula. This freeze does not create a second on-time, OTIF, or rejection formula.

| KPI_ID | Class | Evidence |
| --- | --- | --- |
| CAR_SHIPMENTS_ASSIGNED | DEFERRED_BY_DEFINITION | `CARRIER_ASSIGNED` and a non-null `carrier_company_id` can diverge. Neither rule is selected. |
| CAR_SHIPMENTS_COMPLETED | DEFERRED_BY_DEFINITION | Completion is not frozen among `DELIVERED`, `DELIVERY_CONFIRMED`, `DOCUMENTS_COMPLETED`, `READY_FOR_BILLING`, and `FINANCIALLY_CLOSED`. |
| CAR_ACCEPTANCE_RATE | DEFERRED_BY_DEFINITION | `ACCEPTED_BY_CARRIER` exists. The offered-assignment denominator is not frozen. Partial status history is not a complete offer population. |
| CAR_REJECTION_RATE | BLOCKED_BY_SOURCE_GAP | GAP-C-005. |
| CAR_CANCELLATION_RATE | DEFERRED_BY_DEFINITION | `CANCELLED` exists. The actor is not proven to be the carrier on every row. Unknown actor is not a carrier cancellation. |
| CAR_ON_TIME_PICKUP_RATE | PARTIAL_BUT_SAFE | `OPS_ON_TIME_PICKUP_RATE` grouped by `carrier_company_id`. Null carrier is excluded. No second formula. |
| CAR_ON_TIME_DELIVERY_RATE | PARTIAL_BUT_SAFE | `OPS_ON_TIME_DELIVERY_RATE` grouped by `carrier_company_id`. Same rule as the implemented delivery KPI. |
| CAR_OTIF | BLOCKED_BY_SOURCE_GAP | GAP-C-001. Same components as `OPS_OTIF`. |
| CAR_AVG_DELAY_MIN | DEFERRED_BY_DEFINITION | Pickup delay and delivery delay are not interchangeable. Early policy is also unfrozen. |
| CAR_DELIVERY_REJECTION_RATE | BLOCKED_BY_SOURCE_GAP | Cargo rejection at delivery. Same quantity gap as `OPS_DELIVERY_REJECTION_RATE`. Not `CAR_REJECTION_RATE`. |
| CAR_CARGO_ISSUE_RATE | DEFERRED_BY_DEFINITION | Disposition reason codes and driver-reported exceptions are not one set. A delivery with no case is not a known zero-issue delivery. |
| CAR_DOCUMENT_COMPLETENESS | BLOCKED_BY_SOURCE_GAP | GAP-B-001. |
| CAR_POD_COMPLETENESS | BLOCKED_BY_SOURCE_GAP | GAP-C-002. |
| CAR_EXCEPTION_RATE | DEFERRED_BY_DEFINITION | Same unfrozen exception population as operations. |
| CAR_CRITICAL_EXCEPTION_RATE | DEFERRED_BY_DEFINITION | Critical is not frozen as P1 or as any other code. |
| CAR_PERFORMANCE_SCORE | DEFERRED_BY_DEFINITION | GAP-E-001. Component set and weights are not selected. |

Carrier counts: `PARTIAL_BUT_SAFE=2`, `BLOCKED_BY_SOURCE_GAP=5`, `DEFERRED_BY_DEFINITION=9`.

Combined: `OPERATIONS_KPI_REVIEWED=20`, `CARRIER_KPI_REVIEWED=16`, `READY_FOR_0_3_COUNT=0`, `PARTIAL_BUT_SAFE_COUNT=6`, `BLOCKED_BY_SOURCE_GAP_COUNT=10`, `DEFERRED_BY_DEFINITION_COUNT=20`.

## Definition decisions

| Decision | Status |
| --- | --- |
| ACTIVE_SHIPMENT_DEFINITION | DEFERRED |
| ON_TIME_PICKUP_DENOMINATOR | FROZEN in 0.1B. Same shape as on-time delivery: `deleted_at IS NULL`, both pickup timestamps present. This freeze does not reopen it. |
| AVG_PICKUP_DELAY_EARLY_POLICY | DEFERRED |
| AVG_DELIVERY_DELAY_EARLY_POLICY | DEFERRED |
| DWELL_MEASURE_SELECTION | DEFERRED |
| TRACKING_COVERAGE_DEFINITION | DEFERRED |
| EXCEPTION_POPULATION | DEFERRED |
| P1_EXCEPTION_DEFINITION | DEFERRED |
| P2_EXCEPTION_DEFINITION | DEFERRED |
| SLA_BREACH_POPULATION | DEFERRED |
| SHIPMENT_CYCLE_TIME_START | DEFERRED |
| SHIPMENT_CYCLE_TIME_END | DEFERRED |
| CARRIER_AVG_DELAY_MEANING | DEFERRED |
| CARRIER_CRITICAL_EXCEPTION_MEANING | DEFERRED |

Early arrival inside an on-time count stays the 0.1B rule: early is on time when actual is at or before planned. That rule is not an average-delay policy.

## Source gaps rechecked on this baseline

### GAP-C-001 OPEN

`transport.delivery_attempt_facts` stores `attempted_quantity`, `accepted_quantity`, and `rejected_quantity` when `RECORD_DELIVERY_DISPOSITION` runs. A rejected quantity above zero also opens `transport.delivery_disposition_cases`. The domain comment still says a zero rejection does not open a disposition case.

The attempt row is written only for that command. Driver delivery completion does not write it. `ReportDeliveryDisposition` is a separate call. A shipment can become `DELIVERED` with no attempt row. Absence of a case, absence of an attempt row, and status `DELIVERED` are not in-full.

No fleet field `in_full` was found. `OPS_OTIF`, `CAR_OTIF`, `EXEC_OTIF`, `OPS_DELIVERY_REJECTION_RATE`, and `OPS_PARTIAL_REJECTION_RATE` stay blocked.

### GAP-C-002 OPEN

`pod_required`, `required_document_type`, `satisfied_document_id`, and `satisfied_at` were not found as an execution completeness fact. `execution_tracking.go` sets `RequiresPOD: false` on the `DELIVERY_COMPLETED` milestone. Counting POD document rows would invent the requirement.

### GAP-C-005 OPEN

Shipment statuses include `ACCEPTED_BY_CARRIER` and `CANCELLED`. No carrier-assignment rejection status was found. `CUSTOMER_REFUSAL` is a delivery disposition reason. An unanswered RFx invite is not this fact. Shipment cancellation is not this fact.

### GAP-E-001 OPEN

```
CARRIER_SCORE_COMPONENT_SET_READY=NO
CARRIER_SCORE_WEIGHTS_READY=NO
```

No score policy and no weights were found. Candidate component ids that already have a frozen formula, and are not selected, are `CAR_ON_TIME_PICKUP_RATE` and `CAR_ON_TIME_DELIVERY_RATE`. Selecting them, or assigning weights, is a later definition version. `CAR_PERFORMANCE_SCORE` and `EXEC_CARRIER_PERFORMANCE` stay out of 0.3B.

GAP-B-001 stays open. It blocks `OPS_DOCUMENT_COMPLETENESS` and `CAR_DOCUMENT_COMPLETENESS`. Owner remains Agent B. This wave does not write that contract.

## Late counts derived in analytics

```
LATE_PICKUP_DERIVED_IN_ANALYTICS=YES
LATE_DELIVERY_DERIVED_IN_ANALYTICS=YES
OPS_LATE_PICKUP=onTimePickupDenominator-onTimePickupNumerator
OPS_LATE_DELIVERY=onTimeDeliveryDenominator-onTimeDeliveryNumerator
```

For non-null timestamps, actual is either at or before planned, or after planned. The late count is the denominator minus the numerator. Analytics owns that subtraction. Shipment-service does not emit `latePickupCount` or `lateDeliveryCount`.

The Analytics-0.2 source already supplies `onTimeDeliveryDenominator` and `onTimeDeliveryNumerator`. `OPS_LATE_DELIVERY` uses those two fields and requires no new shipment-service field.

## Agent C source contract for 0.3B

`AGENT_C_SOURCE_WORK_REQUIRED=YES`

`NEW_TENANT_SOURCE_FIELDS=2`

`NEW_CARRIER_AGGREGATE_MEASURES=4`

Do not change the meaning of the five 0.2 fields. Add a new shipment-owned internal read, or a new contract version, that analytics calls. Analytics still does not read the shipment database.

Tenant aggregate additions, same population rule as on-time delivery (`deleted_at IS NULL`):

| Fact | Predicate |
| --- | --- |
| `onTimePickupDenominator` | `planned_pickup_at` and `actual_pickup_at` both present |
| `onTimePickupNumerator` | those rows where `actual_pickup_at <= planned_pickup_at` |

Carrier aggregate rows:

| Field | Role |
| --- | --- |
| `carrierCompanyId` | Row identity. Non-null `carrier_company_id` only. |
| `onTimePickupDenominator` | Same pickup predicate as the tenant operations KPI. |
| `onTimePickupNumerator` | Same pickup predicate as the tenant operations KPI. |
| `onTimeDeliveryDenominator` | Same delivery predicate as the implemented tenant operations KPI. |
| `onTimeDeliveryNumerator` | Same delivery predicate as the implemented tenant operations KPI. |

Carrier rows must:

- belong to the server-derived tenant;
- include only non-null `carrier_company_id`;
- use the exact same timestamp predicates as the corresponding tenant Operations KPI;
- not define separate KPI formulas.

The carrier payload is a breakdown of carriers in the tenant. It is not a company filter.

Not in this contract: `latePickupCount`, `lateDeliveryCount`, active status, dwell, average delay, OTIF, in-full, POD, assignment rejection, cancellation actor, exceptions, SLA breach, cycle time, document completeness, or a performance score.

## Carrier attribution on this baseline

On current main, `transport.shipments.carrier_company_id` is written when the shipment is inserted (`CreateShipment` and `insertShipmentTx`). Shipment update statements change status, driver, vehicle, and actual pickup or delivery timestamps. No shipment-service mutation path that changes `carrier_company_id` was found.

```
CARRIER_REASSIGNMENT_SUPPORTED_ON_CURRENT_BASELINE=NO
CARRIER_EVENT_TIME_ATTRIBUTION_REQUIRED_NOW=NO
```

Current-state grouping by `carrier_company_id` is allowed for 0.3B. This decision must be revisited if carrier reassignment is introduced later.

`ANALYTICS_PROJECTION_REQUIRED_FOR_0_3=NO`. These counts are current-state rows. No retained history is required for the 0.3B set.

## Analytics-0.3B implementation set

```
ANALYTICS_0_3B_KPI_SET=OPS_ON_TIME_PICKUP,OPS_ON_TIME_PICKUP_RATE,OPS_LATE_PICKUP,OPS_LATE_DELIVERY,CAR_ON_TIME_PICKUP_RATE,CAR_ON_TIME_DELIVERY_RATE
```

`ANALYTICS_0_3B_KPI_COUNT=6`

Six additional KPIs. The review does not support a larger set without inventing a denominator, an early-delay policy, a dwell interval, an exception population, or a score weight.

Implementation notes for that later wave, not done here:

- `OPS_LATE_PICKUP` and `OPS_LATE_DELIVERY` are derived in analytics from the on-time denominator and numerator. They are not source fields.
- `OPS_LATE_DELIVERY` uses the existing Analytics-0.2 delivery counts.
- The operations KPIs fit the current single-measure response.
- The two carrier KPIs need the partitioned source payload and a response that returns one measure per carrier. A single tenant total must not be published under those ids.
- Pickup and late completeness stay `PARTIAL` for the same reason as `OPS_ON_TIME_DELIVERY`: `actual_pickup_at` and `actual_delivery_at` are actor-supplied event times.
- `definitionVersion=1` on the five shipped KPIs does not change.
- Zero denominator stays numerator 0, denominator 0, value null.

## Deferred set

Not in 0.3B:

```
OPS_SHIPMENTS_ACTIVE
OPS_AVG_PICKUP_DELAY_MIN
OPS_AVG_DELIVERY_DELAY_MIN
OPS_OTIF
OPS_DWELL_PICKUP_MIN
OPS_DWELL_DELIVERY_MIN
OPS_TRACKING_COVERAGE
OPS_POD_COMPLETENESS
OPS_DELIVERY_REJECTION_RATE
OPS_PARTIAL_REJECTION_RATE
OPS_EXCEPTION_RATE
OPS_P1_EXCEPTION_RATE
OPS_P2_EXCEPTION_RATE
OPS_SLA_BREACH_RATE
OPS_SHIPMENT_CYCLE_TIME
OPS_DOCUMENT_COMPLETENESS
CAR_SHIPMENTS_ASSIGNED
CAR_SHIPMENTS_COMPLETED
CAR_ACCEPTANCE_RATE
CAR_REJECTION_RATE
CAR_CANCELLATION_RATE
CAR_OTIF
CAR_AVG_DELAY_MIN
CAR_DELIVERY_REJECTION_RATE
CAR_CARGO_ISSUE_RATE
CAR_DOCUMENT_COMPLETENESS
CAR_POD_COMPLETENESS
CAR_EXCEPTION_RATE
CAR_CRITICAL_EXCEPTION_RATE
CAR_PERFORMANCE_SCORE
```

## Non-goals

```
PRODUCT_CODE_CHANGED=NO
ANALYTICS_SERVICE_CODE_CHANGED=NO
API_CHANGED=NO
UI_CHANGED=NO
MIGRATION_CREATED=NO
DWH_CREATED=NO
STAGING_CHANGED=NO
DEPLOYMENT_PERFORMED=NO
ANALYTICS_0_3B_IMPLEMENTED=NO
```
