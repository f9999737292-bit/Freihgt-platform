# Analytics-0.1B canonical KPI and architecture freeze

This document freezes analytics architecture. It does not implement it.

Input is the Analytics-0.1A inventory on main `ac609c58b69a52d87b002ae9ecb7feb69f5d5a9a`:

- [ANALYTICS_0_1A_CURRENT_STATE.md](ANALYTICS_0_1A_CURRENT_STATE.md)
- [ANALYTICS_0_1A_KPI_CATALOG.md](ANALYTICS_0_1A_KPI_CATALOG.md)
- [ANALYTICS_0_1A_SOURCE_MAP.md](ANALYTICS_0_1A_SOURCE_MAP.md)
- [ANALYTICS_0_1A_SOURCE_CONTRACT_GAPS.md](ANALYTICS_0_1A_SOURCE_CONTRACT_GAPS.md)

0.1A totals stay in force: 114 KPIs, 17 READY, 65 PARTIAL, 23 BLOCKED, 9 NOT_SUPPORTED, overall readiness 0.434, 15 source-contract gaps. This freeze does not promote a blocked fact.

Status words used below:

- `FROZEN` means the rule is decided.
- `DEFERRED` means a business choice is explicitly not made.
- `BLOCKED_BY_SOURCE_GAP` means a named 0.1A gap still owns the missing fact.

## A. Analytics ownership

`FROZEN`. `ANALYTICS_DOMAIN_OWNER=Agent E`.

Analytics owns KPI definitions, semantic contracts, analytics projections, analytics APIs, analytics aggregates, analytics-specific historical storage, BI dimensions, and Executive BI definitions.

Analytics does not own shipment execution facts, document verification facts, RFx transactional facts, NLO planning facts, payment transactional facts, or carrier and shipper master data.

```
ANALYTICS_MAY_DERIVE=YES
ANALYTICS_MAY_INVENT_SOURCE_FACTS=NO
```

A derivation may combine source facts the owner already stores. It may not fill a 0.1A gap by assumption.

## B. KPI definition ownership

`FROZEN`. `ONE_BUSINESS_KPI_ONE_CANONICAL_DEFINITION=YES`.

Once a KPI is promoted into analytics, Agent E owns its meaning. The source service keeps the raw fact. A frontend application must not publish a second formula for the same business KPI.

These are different facts:

| Left | Right |
| --- | --- |
| Control Tower SLA `ON_TIME` | Analytics on-time delivery |
| `DocumentStatus=SIGNED` | Cryptographically verified |
| Cryptographically verified | Qualified trusted |
| NLO candidate `road_deadhead_km` | Executed fleet or network deadhead |

`KPI_DEFINITION_VERSIONING=YES`. The identity is `KPI_ID` + `DEFINITION_VERSION`. A change that changes business meaning creates a new definition version. Historical results keep the version that produced them. A wording-only note does not increment the version.

## C. Source-system ownership

`FROZEN`. `SOURCE_DOMAIN_FACT_OWNERSHIP_PRESERVED=YES`.

| Fact class | Owner |
| --- | --- |
| Shipment and execution | Agent C, shipment-service |
| Documents and verification | Agent B, document-service |
| RFx and tender baseline | RFx owner, GAP-RFX-001 |
| NLO search and plan | Agent D, network-optimizer-service |
| Executed distance | Agent C, GAP-C-006 |
| Payments and billing rows | Finance source services |
| Company, location, vehicle, driver master | Owning master-data services |
| Carrier score weights | Agent E policy, GAP-E-001, after component facts exist |

## D. Semantic layer

`FROZEN` as a logical contract. It is not implemented.

Each promoted KPI definition carries:

`KPI_ID`, `DEFINITION_VERSION`, `BUSINESS_NAME`, `BUSINESS_DEFINITION`, `NUMERATOR`, `DENOMINATOR`, `INCLUSION_RULE`, `EXCLUSION_RULE`, `TIME_BASIS`, `TIMEZONE_POLICY`, `SOURCE_FACTS`, `SOURCE_OWNER`, `DIMENSIONS`, `CURRENCY_POLICY`, `LATE_EVENT_POLICY`, `READINESS`, `HISTORY_CLASS`.

0.1A catalog fields remain the evidence record. Promotion copies a version into this contract. It does not edit the inventory's readiness.

## E. Historical fact policy

`FROZEN`. `CURRENT_STATE_IS_HISTORY=NO`.

| Class | What it is | Point-in-time | Trend | As-of | Restatement | Replay |
| --- | --- | --- | --- | --- | --- | --- |
| `CURRENT_STATE` | The current row | No | No | No | No | No |
| `CANONICAL_EVENT` | Append-only source event with its own identity and business event time | Only if the stream is complete for that question | Yes | Yes, from the stream | Yes, by a later event | Yes, idempotent on event identity |
| `STATUS_HISTORY` | Status transitions with `occurred_at` and `recorded_at` | Only for status, and only when the chain is complete | Status duration only | Status only | A missing transition stays a gap | Idempotent insert |
| `SNAPSHOT` | Immutable analytics publication of one definition version | The publication instant only | No, unless a series of snapshots is stored | The publication only | A new snapshot version | Rebuild only from source events that still exist |
| `DERIVED_ANALYTICS_FACT` | Analytics-owned row computed from source facts | After it is stored with event time, tenant, and definition version | After storage | After storage | New version, old row kept | Rebuild from named inputs |

A current mutable row is never described as history. `HISTORICAL_ANALYTICS_READY=NO` remains true.

## F. Current-state analytics

`FROZEN`. Current-state KPIs may be served from current source rows when the KPI's `HISTORY_CLASS` is `CURRENT_STATE` and the definition says so. The response must say the result is current-state. It must not be labeled a trend, a past-day population, or an as-of result.

Soft-deleted shipments stay out of `OPS_SHIPMENTS_TOTAL` version 1, matching the 0.1A rule `deleted_at IS NULL`. A past day that needs rows deleted later is a later definition version and is not supported now.

## G. Reporting timezone

`FROZEN`.

```
CANONICAL_STORAGE_TIMEZONE=UTC
DEFAULT_REPORTING_TIMEZONE_POLICY=TENANT_EXPLICIT_DEFAULT_REQUIRED
LOCATION_TIMEZONE_USAGE=KPI_DEFINITION_ONLY
```

Storage stays `TIMESTAMPTZ` interpreted as UTC instants. Comparisons of two instants, including on-time delivery version 1, use those instants and do not need a local day.

A local business day is grouped only in a timezone named by the KPI definition. The default name is the tenant's explicit reporting timezone. If that value is absent, business-day grouping for that tenant is `BLOCKED`. `Europe/Moscow` is the location-row default from migration `000003`. It is not the platform reporting timezone.

`LOCATION_TIMEZONE_USAGE=KPI_DEFINITION_ONLY`. A location timezone is used only when that KPI definition says the day belongs to the location. It is not a global fallback.

Transport-order `DATE` columns are calendar dates without a time. A KPI that mixes those dates with shipment instants is `BLOCKED` until its definition states which timezone gives the date a day boundary. No such mix is in Analytics-0.2.

## H. Late events and restatement

`FROZEN` as technical behavior. `BUSINESS_CLOSE_DURATION=TBD`. No accounting or legal close length is invented.

```
RESTATEMENT_WINDOW=BUSINESS_DURATION_TBD
HARD_CLOSE_POLICY=PUBLISHED_SNAPSHOT_IMMUTABLE
SOFT_CLOSE_POLICY=PRELIMINARY_PERIOD_MAY_ACCEPT_LATE_EVENTS_AND_MUST_MARK_RESTATED
LATE_EVENT_AFTER_CLOSE_POLICY=APPEND_CORRECTION_DO_NOT_REWRITE_SNAPSHOT
IDEMPOTENT_REPLAY_POLICY=SAME_SOURCE_EVENT_IDENTITY_IS_ONE_EFFECT
```

A hard close here is an analytics publication identified by period and definition version. It is not a financial, tax, or legal close. Nothing in this freeze is hard-closed.

Source behavior already found in 0.1A stays in force and is not widened: TMS actual timestamps keep the first non-null value, terminal stops stay immutable, freight-cost corrections use `supersedes_entry_id`, and replay identity for TMS commands is `(operating_tenant_id, idempotency_key)`.

## I. Tenant isolation

`FROZEN`.

```
TENANT_ID_REQUIRED_ON_ANALYTICS_FACTS=YES
CROSS_TENANT_QUERY_DEFAULT=NO
PLATFORM_SCOPE_AGGREGATION_POLICY=PLATFORM_AUTHORIZATION_REQUIRED
```

Every analytics fact and every derived aggregate stores the tenant it was computed for. A tenant request sees that tenant only. Platform-wide aggregation is a separate authorization, not the default and not a missing tenant filter.

Caller-supplied tenant headers or query parameters do not override the tenant established from the verified JWT at the API gateway. Dimensions that use tenant-scoped source ids stay inside that tenant. NLO anonymized marketplace views stay coarse. Analytics does not re-identify customer, facility, or coordinates from them.

## J. Dimensional model

`FROZEN` as logical dimensions. Physical dimension tables are not created. `DIMENSION_SCD_IMPLEMENTATION=DEFERRED`.

Shipper, carrier, and consignee are roles of `core.companies.id` in a shipment, RFx, or contract context. They are not separate company id spaces.

| Dimension | Canonical id | Source owner | Tenant scope | Historical label | Later SCD |
| --- | --- | --- | --- | --- | --- |
| TIME | UTC instant | Storage clock | Not a tenant | Instant is stable | No |
| TENANT | `core.tenants.id` | Identity / core | The id itself | Id stable | No |
| COMPANY | `core.companies.id` | Company master | Company tenant | Name is mutable | Yes, when history is stored |
| SHIPPER | Same company id via `shipper_company_id` | Shipment | Shipment tenant | Name is mutable | Yes, label only |
| CARRIER | Same company id via `carrier_company_id` | Shipment | Shipment tenant | Null means unassigned | Yes, label only |
| CONSIGNEE | Same company id via `consignee_company_id` | Shipment | Shipment tenant | Name is mutable | Yes, label only |
| SHIPMENT | `transport.shipments.id` | Agent C | `tenant_id` | Number may change only under source rules | Id is the key |
| TRANSPORT_ORDER | `transport.transport_orders.id` | Transport order | `tenant_id` | Requested windows are `DATE` | Id is the key |
| RFx | `rfx.rfx_events.id` | RFx | `tenant_id` | Status is mutable | Current row is not history |
| CONTRACT | `contract_rate.transport_contract` | Contract rate | `tenant_id` | Rate cards are versioned in source | Use source version |
| LANE | Not one id | RFx lane, contract rate line, or freight-cost `lane_key` | Parent tenant | Three keys | `LANE_UNIFICATION=DEFERRED` |
| ORIGIN | `origin_location_id` | Location master | Location tenant | Name and coordinates mutable | Yes, label only |
| DESTINATION | `destination_location_id` | Location master | Location tenant | Name and coordinates mutable | Yes, label only |
| REGION | `locations.region` plus `country_code` | Location row | Location tenant | Not a region id | No separate dimension row |
| LOCATION | `transport.locations.id` | Location master | Location tenant | Name mutable | Yes, label only |
| TRANSPORT_MODE | Mode code on the parent fact | Parent fact | Parent tenant | Code is the value | No |
| EQUIPMENT_TYPE | Code on RFx lane or rate line | Those sources | Parent tenant | Not on every shipment | No until a shipment fact exists |
| VEHICLE | `transport.vehicles.id` | Fleet master | Carrier company | Not a full history | `DEFERRED` |
| DRIVER | `transport.drivers.id` | Fleet master | Carrier scope | Not a performance fact | `DEFERRED` |
| CARGO_TYPE | `cargoes.cargo_type` | Cargo row | Cargo tenant | Mutable row | Current code only |
| CURRENCY | ISO `currency_code` on the money fact | Money source | Same tenant as the money fact | Code is stable | No conversion dimension |

## K. Control Tower boundary

`FROZEN`. Detail is [ADR-AN-005](adr/ADR-AN-005-control-tower-vs-analytics.md).

```
CONTROL_TOWER_IS_ANALYTICS_WAREHOUSE=NO
CONTROL_TOWER_OPERATIONAL_READ_MODEL_REUSE_ALLOWED=YES
```

Reuse is allowed only when the read-model field has the same meaning as the KPI definition. Server-derived does not mean historical. Control Tower `generatedAt` and `dataFreshness.partial` are the pattern for response metadata. They are not a business watermark and not a restatement record.

## L. Frontend KPI policy

`FROZEN`.

```
FRONTEND_MAY_CALCULATE_PRESENTATION_ONLY=YES
FRONTEND_MAY_DEFINE_BUSINESS_KPI=NO
FRONTEND_BUSINESS_KPI_OWNER=NO
```

A frontend may format a value, sort it, and divide a numerator by a denominator that the canonical response already returned. It must not define OTIF, SLA, savings, carrier score, network utilization, financial totals, or verified document counts.

`apps/web-admin/utils/controlTowerLogic.ts` `buildKpiMetrics` and `computeSlaStatus` are a fallback when the Control Tower summary KPI is absent. That fallback is not an analytics definition. Deprecation path: when the summary or a later analytics payload is present, the client renders that payload and does not recompute the formula. Removing the fallback is a later code change. This freeze does not edit the frontend.

## M. Money and currency

`FROZEN`.

```
MIXED_CURRENCY_UNSAFE_SUM_FORBIDDEN=YES
MIXED_CURRENCY_SUM_WITHOUT_CONVERSION=FORBIDDEN
IMPLICIT_RUB_TOTAL=NO
FX_CONVERSION_SOURCE=NOT_FOUND
CROSS_CURRENCY_CONVERSION=BLOCKED
```

A money KPI states a currency dimension, or it states an approved FX source, rate date, and method. No FX source was found in 0.1A, so any total that needs conversion stays blocked. Per-currency sums remain allowed where the source already stores `currency_code` and rejects a mixed-currency grain.

`FIN_ACTUAL_STAGE=DEFERRED`. `CURRENT_ACTUAL` and `FINAL_ACTUAL` stay different. `FINANCIAL_CLOSE_MARKER=DEFERRED`. Shipment `FINANCIALLY_CLOSED`, billing `CLOSED`, and freight-cost `FINAL_ACTUAL` stay three facts. `FIN_FINANCIALLY_CLOSED_AMOUNT` is not in Analytics-0.2.

## N. Network analytics boundary

`FROZEN`.

```
NLO_CANDIDATE_DISTANCE_IS_EXECUTED_DISTANCE=NO
NETWORK_EXECUTED_DISTANCE_READY=NO
BACKHAUL_ANALYTICS_READY=NO
OBSERVABILITY_ONLY=YES
```

Planned and optimized distances stay on NLO facts owned by Agent D. Executed loaded, empty, and deadhead distance stay on TMS facts owned by Agent C. `NET_DEADHEAD_KM` and `NET_DEADHEAD_PCT` stay blocked by GAP-C-006. Candidate deadhead, planned route increase, accepted route plans, and executed deadhead are four different things.

Backhaul opportunity, acceptance, and conversion stay blocked until Agent D persists those facts. ADR-NET-024 says backhaul runtime and roundtrip runtime are not implemented. Prometheus route, matrix, selection, and prune counters stay observability. They are not business KPIs and they do not raise network readiness.

## O. Document trust boundary

`FROZEN`. These states are never one count:

| State | Meaning | 0.1A KPI |
| --- | --- | --- |
| `WORKFLOW_SIGNED` | Document workflow status signed | `DOC_SIGNED_STATUS_COUNT` is READY as that count only |
| `CRYPTOGRAPHICALLY_VERIFIED` | A verifier result that the signature checked | `DOC_VERIFIED_SIGNATURE_COUNT` stays BLOCKED, GAP-B-003 |
| `QUALIFIED_TRUSTED` | Qualified policy `QUALIFIED_CADES_BES` | `DOC_QUALIFIED_TRUST_COUNT` stays BLOCKED, GAP-B-002 |

```
DOC_VERIFIED_SIGNATURE_COUNT_READY=NO
QUALIFIED_DOCUMENT_TRUST_READY=NO
```

Legacy `VALID`, `DocumentStatus=SIGNED`, and `VERIFIER_UNAVAILABLE` are not verified and not qualified. No I4C production verifier is in this baseline.

## Event time

`FROZEN`.

| Clock | Meaning | May define a business KPI time |
| --- | --- | --- |
| `BUSINESS_EVENT_TIME` | Source business time, such as `occurred_at`, `actual_delivery_at`, `actual_pickup_at`, `awarded_at`, `signed_at`, `payment_date` | Yes, when the KPI names it |
| `PROCESSING_TIME` | `created_at`, `updated_at`, `recorded_at`, Control Tower `generatedAt` | Only when the definition says the KPI is a processing or insert count |
| `INGESTION_TIME` | When an analytics projection first stored the row | No |

If the KPI's canonical business event time is null or absent, that row is excluded. The response completeness is `PARTIAL`, or the KPI stays `BLOCKED` when the time is missing for the whole definition. `updated_at` is not a silent substitute.

`OPS_SHIPMENTS_TOTAL` version 1 may use `created_at` because the 0.1A definition is a created-row count, not a delivery event.

## OTIF

`FROZEN` as a formula shape. The source gap is not closed.

On-time delivery version 1 is defined in the Analytics-0.2 section. In-full still requires authoritative accepted quantity versus shipped quantity, including deliveries that open no disposition case.

```
OTIF_ON_TIME_COMPONENT=PARTIAL
OTIF_IN_FULL_COMPONENT=BLOCKED
OTIF_READY=NO
```

`OPS_OTIF`, `CAR_OTIF`, and `EXEC_OTIF` stay `BLOCKED` until GAP-C-001 closes. `DELIVERED`, a completed stop, a delivery action, and the absence of a disposition case are not in-full.

## Dwell

`FROZEN` as three measures. `DWELL_BUSINESS_TERM=DEFERRED`.

| Measure id | Interval |
| --- | --- |
| `STOP_WAIT_TIME` | `arrived_at` to `service_started_at` |
| `STOP_SERVICE_TIME` | `service_started_at` to `completed_at` |
| `STOP_TOTAL_TIME` | `arrived_at` to `completed_at` |

Pickup and delivery are the execution action types `PICKUP` and `DELIVERY`. None of the three measures is named dwell. `OPS_DWELL_PICKUP_MIN` and `OPS_DWELL_DELIVERY_MIN` are not selected as one of these intervals and are not in Analytics-0.2. Planned `service_duration_seconds` and NLO policy duration are not measured time. `created_at` and `actual_pickup_at` are not stop intervals.

## Carrier score

`FROZEN` as a shape. `CARRIER_SCORE_READY=NO`. `CARRIER_SCORE_WEIGHT_VERSIONING=YES`.

A carrier score is a composite of named component KPI ids. Each weight is explicit and versioned with the score definition. Missing weights are not implied. GAP-E-001 stays open. `CAR_PERFORMANCE_SCORE` and `EXEC_CARRIER_PERFORMANCE` stay blocked.

## Procurement savings

`BLOCKED_BY_SOURCE_GAP` GAP-RFX-001. Lowest bid, previous bid, contract rate, and budget are not interchangeable baselines.

A future source contract, owned by the RFx domain, needs one baseline kind per KPI version, plus `baseline_amount`, `baseline_currency`, `baseline_source`, and `baseline_as_of`. Analytics will not pick a baseline in this freeze. `PROCUREMENT_PARTICIPATION_DENOMINATOR=DEFERRED`.

## Other definition choices

| Choice | Status |
| --- | --- |
| `ACTIVE_SHIPMENT_DEFINITION` | `DEFERRED`. Control Tower "not CANCELLED and not FINANCIALLY_CLOSED" is not frozen as `OPS_SHIPMENTS_ACTIVE`. |
| On-time pickup version 1 | `FROZEN` with the same shape as on-time delivery, using pickup timestamps. Not in Analytics-0.2. |
| Late pickup and late delivery | `FROZEN` as the completed population where actual is after planned. In-flight SLA overdue is a different measure. Not in Analytics-0.2. |
| Early arrival inside an on-time count | `FROZEN`. Early is on time when actual is at or before planned. |
| Average delay and early arrivals | `DEFERRED`. |
| Exception population | `DEFERRED`. Workflow exceptions and driver-reported exceptions stay different. |
| Shipment cycle time bounds | `DEFERRED`. |
| Tracking coverage | `DEFERRED`. |
| `LANE_UNIFICATION` | `DEFERRED`. |
| `FIN_ACTUAL_STAGE` | `DEFERRED`. |
| `FINANCIAL_CLOSE_MARKER` | `DEFERRED`. |

## Analytics-0.2 scope

`FROZEN`. `ANALYTICS_0_2_SCOPE=OPERATIONS_ANALYTICS_FOUNDATION`. `ANALYTICS_0_2_KPI_COUNT=5`. `ANALYTICS_0_2_LOCAL_DAY_BUCKETING=NO`.

The first implementation wave is a current-state operations foundation. It does not include OTIF. It does not bucket by local day. Responses carry `generatedAt`, `definitionVersion`, `dataFreshness`, and `completeness`. Currency and timezone are omitted because these five KPIs are not money totals and not local-day groups.

On-time delivery version 1:

- Population and denominator: `deleted_at IS NULL`, `planned_delivery_at` present, `actual_delivery_at` present.
- Numerator: `actual_delivery_at` is at or before `planned_delivery_at`.
- Grace minutes: 0. A later grace contract is a new definition version.
- Event time: `actual_delivery_at`, which 0.1A traces to actor-supplied `occurred_at` on delivery. Completeness stays `PARTIAL` for that reason.
- Excludes shipments with a null actual delivery time.
- Is not Control Tower `kpi.onTime` and is not `ReasonOnSchedule`.

`OPS_SHIPMENTS_TOTAL` version 1 is the current tenant population with `deleted_at IS NULL`. `OPS_RETURN_CASES` and `OPS_REDIRECT_CASES` are current disposition case counts, not rates.

| KPI_ID | IN_0_2 | REASON | DEPENDENCIES |
| --- | --- | --- | --- |
| OPS_SHIPMENTS_TOTAL | YES | READY current-state count. v1 population is deleted_at IS NULL. | shipment row |
| OPS_SHIPMENTS_ACTIVE | NO | ACTIVE_SHIPMENT_DEFINITION=DEFERRED | see reason |
| OPS_ON_TIME_PICKUP | NO | Pickup formula is frozen and is outside this five-KPI wave. | see reason |
| OPS_ON_TIME_PICKUP_RATE | NO | Pickup formula is frozen and is outside this five-KPI wave. | see reason |
| OPS_LATE_PICKUP | NO | Completed-late formula is frozen and is outside this wave. | see reason |
| OPS_AVG_PICKUP_DELAY_MIN | NO | AVG_DELAY_EARLY_POLICY=DEFERRED | see reason |
| OPS_ON_TIME_DELIVERY | YES | PARTIAL source. v1 meaning frozen: both delivery timestamps present and actual <= planned. Grace 0. Not Control Tower ON_TIME. | planned_delivery_at, actual_delivery_at |
| OPS_ON_TIME_DELIVERY_RATE | YES | Denominator frozen as that completed population. Completeness stays PARTIAL because occurred_at is actor-supplied. | same delivery timestamps |
| OPS_LATE_DELIVERY | NO | Completed-late formula is frozen and is outside this wave. | see reason |
| OPS_AVG_DELIVERY_DELAY_MIN | NO | AVG_DELAY_EARLY_POLICY=DEFERRED | see reason |
| OPS_OTIF | NO | BLOCKED_BY_SOURCE_GAP | GAP-C-001 |
| OPS_DWELL_PICKUP_MIN | NO | Three stop measures are defined. The dwell name is deferred. Not this wave. | see reason |
| OPS_DWELL_DELIVERY_MIN | NO | Three stop measures are defined. The dwell name is deferred. Not this wave. | see reason |
| OPS_TRACKING_COVERAGE | NO | TRACKING_COVERAGE=DEFERRED | see reason |
| OPS_POD_COMPLETENESS | NO | BLOCKED_BY_SOURCE_GAP | GAP-C-002 |
| OPS_DELIVERY_REJECTION_RATE | NO | Fleet denominator is not frozen. Zero-rejection gap remains GAP-C-001. | see reason |
| OPS_PARTIAL_REJECTION_RATE | NO | BLOCKED_BY_SOURCE_GAP for a fleet rate. GAP-C-001. | see reason |
| OPS_RETURN_CASES | YES | READY current disposition case count. Not a rate. | disposition case |
| OPS_REDIRECT_CASES | YES | READY current disposition case count. Not a rate. | disposition case |
| OPS_EXCEPTION_RATE | NO | EXCEPTION_POPULATION=DEFERRED | see reason |
| OPS_P1_EXCEPTION_RATE | NO | EXCEPTION_POPULATION=DEFERRED | see reason |
| OPS_P2_EXCEPTION_RATE | NO | EXCEPTION_POPULATION=DEFERRED | see reason |
| OPS_SLA_BREACH_RATE | NO | Control Tower SLA and analytics on-time stay different. Breach population is DEFERRED. | see reason |
| OPS_SHIPMENT_CYCLE_TIME | NO | CYCLE_TIME_BOUNDS=DEFERRED | see reason |
| OPS_DOCUMENT_COMPLETENESS | NO | BLOCKED_BY_SOURCE_GAP | GAP-B-001 |
| PROC_RFX_CREATED | NO | READY, and outside the operations foundation wave. | later wave |
| PROC_RFX_PUBLISHED | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| PROC_CARRIERS_INVITED | NO | READY, and outside the operations foundation wave. | later wave |
| PROC_CARRIERS_PARTICIPATED | NO | PROCUREMENT_PARTICIPATION_DENOMINATOR=DEFERRED | see reason |
| PROC_PARTICIPATION_RATE | NO | PROCUREMENT_PARTICIPATION_DENOMINATOR=DEFERRED | see reason |
| PROC_BIDS_RECEIVED | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| PROC_BIDS_PER_RFX | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| PROC_FIRST_BID_TIME | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| PROC_TIME_TO_AWARD | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| PROC_AWARD_RATE | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| PROC_WINNING_BID_VALUE | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| PROC_TENDER_SAVINGS | NO | BLOCKED_BY_SOURCE_GAP | GAP-RFX-001 |
| PROC_TENDER_SAVINGS_PCT | NO | BLOCKED_BY_SOURCE_GAP | GAP-RFX-001 |
| PROC_CONTRACT_VS_SPOT_SHARE | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| PROC_LANE_COMPETITION | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| PROC_CARRIER_RESPONSE_RATE | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| CAR_SHIPMENTS_ASSIGNED | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| CAR_SHIPMENTS_COMPLETED | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| CAR_ACCEPTANCE_RATE | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| CAR_REJECTION_RATE | NO | BLOCKED_BY_SOURCE_GAP | GAP-C-005 |
| CAR_CANCELLATION_RATE | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| CAR_ON_TIME_PICKUP_RATE | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| CAR_ON_TIME_DELIVERY_RATE | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| CAR_OTIF | NO | BLOCKED_BY_SOURCE_GAP | GAP-C-001 |
| CAR_AVG_DELAY_MIN | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| CAR_DELIVERY_REJECTION_RATE | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| CAR_CARGO_ISSUE_RATE | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| CAR_DOCUMENT_COMPLETENESS | NO | BLOCKED_BY_SOURCE_GAP | GAP-B-001 |
| CAR_POD_COMPLETENESS | NO | BLOCKED_BY_SOURCE_GAP | GAP-C-002 |
| CAR_EXCEPTION_RATE | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| CAR_CRITICAL_EXCEPTION_RATE | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| CAR_PERFORMANCE_SCORE | NO | BLOCKED_BY_SOURCE_GAP | GAP-E-001 |
| FIN_PLANNED_FREIGHT_COST | NO | READY, and outside the operations foundation wave. | later wave |
| FIN_ESTIMATED_FREIGHT_COST | NO | NOT_SUPPORTED in 0.1A. This freeze does not invent the fact. | missing source fact |
| FIN_ACTUAL_FREIGHT_COST | NO | FIN_ACTUAL_STAGE=DEFERRED | see reason |
| FIN_COST_VARIANCE | NO | Depends on the deferred actual stage. | see reason |
| FIN_COST_VARIANCE_PCT | NO | Depends on the deferred actual stage. | see reason |
| FIN_COST_PER_SHIPMENT | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| FIN_COST_PER_KM | NO | BLOCKED_BY_SOURCE_GAP | GAP-C-003 |
| FIN_COST_PER_PALLET | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| FIN_COST_PER_TON | NO | BLOCKED_BY_SOURCE_GAP | GAP-C-004 |
| FIN_BASE_FREIGHT | NO | READY, and outside the operations foundation wave. | later wave |
| FIN_SURCHARGES | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| FIN_PENALTIES | NO | READY, and outside the operations foundation wave. | later wave |
| FIN_BILLED_AMOUNT | NO | READY, and outside the operations foundation wave. | later wave |
| FIN_PAID_AMOUNT | NO | READY, and outside the operations foundation wave. | later wave |
| FIN_OUTSTANDING_AMOUNT | NO | READY, and outside the operations foundation wave. | later wave |
| FIN_PAYMENT_AGING_DAYS | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| FIN_DAYS_TO_INVOICE | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| FIN_DAYS_TO_PAYMENT | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| FIN_FINANCIALLY_CLOSED_AMOUNT | NO | FINANCIAL_CLOSE_MARKER=DEFERRED | see reason |
| DOC_DOCUMENTS_CREATED | NO | READY, and outside the operations foundation wave. | later wave |
| DOC_READY_FOR_SIGNING | NO | READY, and outside the operations foundation wave. | later wave |
| DOC_SIGNED_STATUS_COUNT | NO | READY, and outside the operations foundation wave. | later wave |
| DOC_VERIFIED_SIGNATURE_COUNT | NO | BLOCKED_BY_SOURCE_GAP | GAP-B-003 |
| DOC_QUALIFIED_TRUST_COUNT | NO | BLOCKED_BY_SOURCE_GAP | GAP-B-002 |
| DOC_SIGNATURE_PENDING_COUNT | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| DOC_DOCUMENT_COMPLETENESS | NO | BLOCKED_BY_SOURCE_GAP | GAP-B-001 |
| DOC_TIME_TO_SIGNATURE | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| NET_CANDIDATES_DISCOVERED | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| NET_CANDIDATES_PREFILTERED | NO | NOT_SUPPORTED in 0.1A. This freeze does not invent the fact. | missing source fact |
| NET_CANDIDATES_ROUTED | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| NET_CANDIDATES_FEASIBLE | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| NET_CANDIDATES_RANKED | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| NET_SEARCH_DURATION | NO | NOT_SUPPORTED in 0.1A. This freeze does not invent the fact. | missing source fact |
| NET_PROVIDER_CALLS | NO | OBSERVABILITY_ONLY. Not a business KPI. | see reason |
| NET_DEADHEAD_KM | NO | BLOCKED_BY_SOURCE_GAP | GAP-C-006 |
| NET_LOADED_KM | NO | EXECUTED distance is GAP-C-006. NOT_SUPPORTED until that fact exists. | see reason |
| NET_EMPTY_KM | NO | EXECUTED distance is GAP-C-006. NOT_SUPPORTED until that fact exists. | see reason |
| NET_DEADHEAD_PCT | NO | BLOCKED_BY_SOURCE_GAP | GAP-C-006 |
| NET_CAPACITY_UTILIZATION | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| NET_BACKHAUL_OPPORTUNITIES | NO | BACKHAUL_ANALYTICS_READY=NO. GAP-D-003 remains open. | see reason |
| NET_BACKHAUL_ACCEPTED | NO | BACKHAUL_ANALYTICS_READY=NO. GAP-D-003 remains open. | see reason |
| NET_BACKHAUL_CONVERSION | NO | BACKHAUL_ANALYTICS_READY=NO. GAP-D-003 remains open. | see reason |
| NET_AVG_DEADHEAD_REDUCTION_KM | NO | BLOCKED_BY_SOURCE_GAP | GAP-D-001 |
| NET_ROUTE_INCREASE_KM | NO | GAP-D-005. Planned route increase is not executed distance. | see reason |
| NET_SEARCH_TO_ACCEPT_CONVERSION | NO | BLOCKED_BY_SOURCE_GAP | GAP-D-004 |
| EXEC_SHIPMENT_VOLUME | NO | READY, and outside the operations foundation wave. | later wave |
| EXEC_OTIF | NO | BLOCKED_BY_SOURCE_GAP | GAP-C-001 |
| EXEC_TOTAL_FREIGHT_COST | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| EXEC_COST_VARIANCE | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| EXEC_TENDER_SAVINGS | NO | BLOCKED_BY_SOURCE_GAP | GAP-RFX-001 |
| EXEC_CARRIER_PERFORMANCE | NO | BLOCKED_BY_SOURCE_GAP | GAP-E-001 |
| EXEC_CRITICAL_EXCEPTION_RATE | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| EXEC_DELIVERY_REJECTION_RATE | NO | PARTIAL, and outside the operations foundation wave. | later wave |
| EXEC_BILLED_AMOUNT | NO | READY, and outside the operations foundation wave. | later wave |
| EXEC_OUTSTANDING_AMOUNT | NO | READY, and outside the operations foundation wave. | later wave |
| EXEC_DEADHEAD_PCT | NO | BLOCKED_BY_SOURCE_GAP | GAP-C-006 |
| EXEC_CAPACITY_UTILIZATION | NO | PARTIAL, and outside the operations foundation wave. | later wave |

## API conventions

`FROZEN` as a contract. No endpoint is implemented.

```
GET /api/v1/analytics/kpis/{kpiId}
GET /api/v1/analytics/kpis/{kpiId}/details
```

The tenant comes from the verified auth context. Filters, when the definition allows them: time range, company, carrier, shipper, lane, location, and currency. Detail sets are paginated. An aggregate body includes `generatedAt`, `definitionVersion`, `dataFreshness`, and `completeness`, plus `currency` and `timezone` when those apply.

The future owner of these routes is an analytics service that does not exist. Analytics-0.2 may be attached at the API gateway later. This document does not create that service or those routes.

## Data quality

`FROZEN`. `DATA_QUALITY_FAIL_OPEN=NO`.

| State | Meaning |
| --- | --- |
| `COMPLETE` | The definition's source coverage is sufficient for the returned grain |
| `PARTIAL` | A defined input is missing for some rows, and those rows are excluded or marked |
| `STALE` | The source watermark is older than the freshness rule on the definition |
| `UNKNOWN` | Coverage cannot be judged |
| `BLOCKED` | The KPI must not be published as a number |

A missing canonical input does not fall back to another field. The client may render the KPI as unavailable. It may not replace it with a local formula.

## Observability

`FROZEN`. Future analytics-service metrics are technical only. They are not business KPIs. Labels must not carry tenant, customer, or shipment ids.

Allowed future metric names: request count, latency, source fetch errors, projection lag, rebuild lag, stale KPI count.

## Storage decision

`FROZEN` in [ADR-AN-002](adr/ADR-AN-002-analytics-storage-boundary.md).

```
ANALYTICS_STORAGE_V0_2_DECISION=OPTION_D_HYBRID
DWH_V1_REQUIRED_NOW=NO
DWH_FUTURE_OPTION=YES
ANALYTICS_SERVICE_IMPLEMENTED=NO
DWH_IMPLEMENTED=NO
```

Option D means Analytics-0.2 reads canonical operational current-state facts for the five KPIs above. Analytics-owned PostgreSQL projections are allowed only in a later wave, and only for a KPI whose history class needs stored events, snapshots, or derived facts that the source does not already keep. A warehouse remains a future option. It is not required now: 17 of 114 KPIs are READY, 15 source gaps are open, historical analytics are not ready, and no analytics service exists.

## Freeze flags

```
ANALYTICS_0_1B_ARCHITECTURE_FROZEN=YES
ANALYTICS_DOMAIN_OWNER=E
SOURCE_DOMAIN_FACT_OWNERSHIP_PRESERVED=YES
ONE_BUSINESS_KPI_ONE_DEFINITION=YES
KPI_DEFINITION_VERSIONING=YES
FRONTEND_BUSINESS_KPI_OWNER=NO
CONTROL_TOWER_IS_ANALYTICS_WAREHOUSE=NO
TENANT_ID_REQUIRED_ON_ANALYTICS_FACTS=YES
CROSS_TENANT_QUERY_DEFAULT=NO
CANONICAL_STORAGE_TIMEZONE=UTC
MIXED_CURRENCY_UNSAFE_SUM_FORBIDDEN=YES
CURRENT_STATE_IS_HISTORY=NO
OTIF_READY=NO
CARRIER_SCORE_READY=NO
QUALIFIED_DOCUMENT_TRUST_READY=NO
NETWORK_EXECUTED_DISTANCE_READY=NO
ANALYTICS_SERVICE_IMPLEMENTED=NO
DWH_IMPLEMENTED=NO
ANALYTICS_0_2_SCOPE_FROZEN=YES
```
