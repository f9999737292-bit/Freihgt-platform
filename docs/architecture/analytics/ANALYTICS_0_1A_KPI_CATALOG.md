# Analytics-0.1A canonical KPI catalog

`KPI_VERSION_PROPOSED=1` for every row. Nothing here is implemented as an analytics KPI. `CURRENT_IMPLEMENTATION` names an existing screen or table when one exists. It is not a claim that the business KPI is already canonical.

Readiness:

| Status | Rule used |
| --- | --- |
| READY | Source fields and meaning are enough to implement a server-owned KPI without inventing a fact. Currency totals stay per currency. |
| PARTIAL | Useful facts exist. Definition, coverage, grain, or history is incomplete. |
| BLOCKED | A missing fact or an explicit missing policy prevents a correct implementation. |
| NOT_SUPPORTED | No meaningful source fact for this KPI. |

```
READINESS_METHOD=READY counts 1.0, PARTIAL counts 0.5, BLOCKED counts 0, NOT_SUPPORTED counts 0. Domain score = sum / KPI count in that domain. Overall = sum / 114.
```

Runtime validation of these formulas was **NOT_RUN**.

Shared defaults when a field below says `SEE_DEFAULT`:

```
KPI_VERSION_PROPOSED=1
TIME_WINDOW=UNDEFINED
TENANT_SCOPE=source tenant_id or operating_tenant_id; client-supplied tenant is not authoritative
FRESHNESS=NOT_FOUND as a KPI watermark; Control Tower requests expose generatedAt and dataFreshness.partial only for Control Tower payloads
LATE_EVENT_POLICY_CURRENT=NOT_FOUND as a KPI restatement policy
CORRECTION_POLICY_CURRENT=NOT_FOUND unless the source note says first-write COALESCE or append-only supersede
NULL_POLICY=NOT_FOUND as an analytics null policy; do not coerce null timestamps to created_at
```

## OTIF special review

```
OTIF_ON_TIME_COMPONENT=PARTIAL
OTIF_IN_FULL_COMPONENT=BLOCKED
OTIF_CURRENT_READINESS=BLOCKED
```

On-time can be investigated from `planned_delivery_at` and `actual_delivery_at`. Control Tower `kpi.onTime` is a different formula (`sla.Compute` `ReasonOnSchedule`). In-full is not established by `DELIVERED`. Quantity fields exist only on disposition cases, and a zero-rejection delivery does not open a case. Detail is in `ANALYTICS_0_1A_CURRENT_STATE.md`. Gap: `GAP-C-001`.

## Dwell special review

No interval is selected.

| ID | From | To | Column evidence |
| --- | --- | --- | --- |
| A | `arrived_at` | `service_started_at` | `transport.transport_execution_stops` (`000088`) |
| B | `service_started_at` | `completed_at` | same; not `service_completed_at` |
| C | `arrived_at` | `completed_at` | same |

`service_duration_seconds` and NLO `000094` policy durations are planned, not measured dwell. Dwell KPIs are PARTIAL until 0.1B chooses A, B, or C.

## Operations

### OPS_SHIPMENTS_TOTAL

```
KPI_ID=OPS_SHIPMENTS_TOTAL
KPI_NAME=Shipments total
KPI_DOMAIN=OPERATIONS
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=v1 count of shipment rows the canonical read already returns: tenant match and deleted_at IS NULL (shipment_repository.go).
NUMERATOR=shipments matching that read
DENOMINATOR=1
INCLUSION_RULE=transport.shipments for the server-derived tenant with deleted_at IS NULL
EXCLUSION_RULE=deleted_at IS NOT NULL. A later version would be required before soft-deleted rows re-enter the population. A past-day snapshot that includes rows deleted after that day is out of v1.
TIME_BASIS=created_at for a created-in-window count
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-SHIPMENT
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=transport.shipments.id, tenant_id, created_at, deleted_at
EVENT_TIME=created_at
PROCESSING_TIME=created_at on insert
DIMENSIONS=TENANT, SHIPPER, CARRIER when carrier_company_id is not null, TRANSPORT_MODE
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=Shipment list totals and Control Tower status summary total. Neither is this KPI version.
READINESS=READY
BLOCKER=
OWNER_AGENT=C for the fact; E for the KPI definition
NOTES=A point-in-time population of a past day is NOT supported. v1 cohorts use created_at on rows that still pass deleted_at IS NULL.
```

### OPS_SHIPMENTS_ACTIVE

```
KPI_ID=OPS_SHIPMENTS_ACTIVE
KPI_NAME=Active shipments
KPI_DOMAIN=OPERATIONS
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=NOT frozen. Control Tower active means status other than CANCELLED and FINANCIALLY_CLOSED, which includes delivered and billing statuses.
NUMERATOR=rows passing that predicate
DENOMINATOR=1
INCLUSION_RULE=IsActiveShipmentStatus in services/api-gateway/internal/platform/sla/sla.go
EXCLUSION_RULE=CANCELLED, FINANCIALLY_CLOSED
TIME_BASIS=current status, not an activation event
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-SHIPMENT and SRC-CT-GATEWAY-SUMMARY
SOURCE_SERVICE=shipment-service status; api-gateway count
SOURCE_FIELDS=transport.shipments.status
EVENT_TIME=NOT_FOUND as an "became active" timestamp; status history has transitions
PROCESSING_TIME=summary generatedAt when read through Control Tower
DIMENSIONS=TENANT and summary filters
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=Control Tower dataFreshness when using the summary
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=unknown status becomes SLA UNKNOWN; active predicate is not status-unknown
CURRENT_IMPLEMENTATION=CalculateKPI active count, filter-scoped
READINESS=PARTIAL
BLOCKER=Business "active" may mean in execution. The code predicate is wider. Do not publish both under one id.
OWNER_AGENT=E for the definition; C for status
NOTES=Frontend fallback uses the same two exclusions. Summary mode is server-derived. The count is not tenant-wide when filters are applied.
```

### OPS_ON_TIME_PICKUP

```
KPI_ID=OPS_ON_TIME_PICKUP
KPI_NAME=On-time pickup count
KPI_DOMAIN=OPERATIONS
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Completed pickups whose actual pickup time is at or before planned pickup. Grace minutes NOT_FOUND as a pickup-specific contract. Control Tower SLA is not this definition.
NUMERATOR=shipments with both timestamps and actual_pickup_at <= planned_pickup_at
DENOMINATOR=not used on the count
INCLUSION_RULE=actual_pickup_at and planned_pickup_at both present
EXCLUSION_RULE=CANCELLED; rows missing either timestamp
TIME_BASIS=actual_pickup_at versus planned_pickup_at
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-SHIPMENT
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=planned_pickup_at, actual_pickup_at
EVENT_TIME=actual_pickup_at, set to command occurred_at or manual ActualTime when status becomes LOADED
PROCESSING_TIME=updated_at on the shipment row; status history recorded_at
DIMENSIONS=TENANT, CARRIER, SHIPPER, ORIGIN
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY for the two timestamps; STATUS_HISTORY can show the LOADED transition
LATE_EVENT_POLICY_CURRENT=first non-null actual_pickup_at wins (COALESCE)
CORRECTION_POLICY_CURRENT=later commands do not replace the first actual
NULL_POLICY=missing actual or planned excludes the row; do not substitute created_at
CURRENT_IMPLEMENTATION=sla.Compute ReasonPickupOverdue is an in-flight projection (now versus planned while actual is null). It is not this completed count.
READINESS=PARTIAL
BLOCKER=Actor-supplied occurred_at; no grace contract; null coverage UNKNOWN
OWNER_AGENT=C
NOTES=Stop arrived_at is a different timestamp.
```

### OPS_ON_TIME_PICKUP_RATE

```
KPI_ID=OPS_ON_TIME_PICKUP_RATE
KPI_NAME=On-time pickup rate
KPI_DOMAIN=OPERATIONS
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=OPS_ON_TIME_PICKUP divided by shipments that were supposed to be picked up. Denominator population is not frozen (planned pickup in window versus reached LOADED versus all non-cancelled).
NUMERATOR=OPS_ON_TIME_PICKUP
DENOMINATOR=UNKNOWN until 0.1B
INCLUSION_RULE=same timestamps as the count
EXCLUSION_RULE=empty denominator must not be rendered as 0% or 100%
TIME_BASIS=actual_pickup_at versus planned_pickup_at
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-SHIPMENT
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=planned_pickup_at, actual_pickup_at, status
EVENT_TIME=actual_pickup_at
PROCESSING_TIME=updated_at
DIMENSIONS=TENANT, CARRIER
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=COALESCE first write
CORRECTION_POLICY_CURRENT=NOT_FOUND
NULL_POLICY=null timestamps out of both numerator and the completed denominator; in-flight overdue is a different measure
CURRENT_IMPLEMENTATION=Control Tower onTime percent is sla status over active filtered rows, not this rate
READINESS=PARTIAL
BLOCKER=Denominator not frozen
OWNER_AGENT=E for the rate definition; C for timestamps
NOTES=Do not reuse kpi.onTime / kpi.active.
```

### OPS_LATE_PICKUP

```
KPI_ID=OPS_LATE_PICKUP
KPI_NAME=Late pickup count
KPI_DOMAIN=OPERATIONS
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Completed pickups with actual_pickup_at after planned_pickup_at. In-flight overdue is a projection, not this count.
NUMERATOR=actual_pickup_at > planned_pickup_at
DENOMINATOR=1
INCLUSION_RULE=both timestamps present
EXCLUSION_RULE=missing timestamps
TIME_BASIS=actual versus planned pickup
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-SHIPMENT
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=planned_pickup_at, actual_pickup_at
EVENT_TIME=actual_pickup_at
PROCESSING_TIME=updated_at
DIMENSIONS=TENANT, CARRIER, ORIGIN
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=COALESCE first write
CORRECTION_POLICY_CURRENT=NOT_FOUND
NULL_POLICY=exclude nulls
CURRENT_IMPLEMENTATION=ReasonPickupOverdue while actual is still null
READINESS=PARTIAL
BLOCKER=Same timestamp trust issues as on-time pickup
OWNER_AGENT=C
NOTES=
```

### OPS_AVG_PICKUP_DELAY_MIN

```
KPI_ID=OPS_AVG_PICKUP_DELAY_MIN
KPI_NAME=Average pickup delay minutes
KPI_DOMAIN=OPERATIONS
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Mean of max(0, actual_pickup_at - planned_pickup_at) in minutes. Whether early pickups enter as zero or are excluded is NOT frozen. sla.minutesBetween clamps negatives to zero for its own delay fields.
NUMERATOR=sum of delay minutes
DENOMINATOR=count of included pickups
INCLUSION_RULE=both timestamps
EXCLUSION_RULE=nulls; empty denominator
TIME_BASIS=timestamptz difference; no local day required
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-SHIPMENT
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=planned_pickup_at, actual_pickup_at
EVENT_TIME=actual_pickup_at
PROCESSING_TIME=updated_at
DIMENSIONS=TENANT, CARRIER
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=COALESCE first write
CORRECTION_POLICY_CURRENT=NOT_FOUND
NULL_POLICY=exclude nulls
CURRENT_IMPLEMENTATION=SLA DelayMinutes on overdue results, not an average KPI
READINESS=PARTIAL
BLOCKER=Inclusion of early arrivals not frozen
OWNER_AGENT=C
NOTES=
```

### OPS_ON_TIME_DELIVERY

```
KPI_ID=OPS_ON_TIME_DELIVERY
KPI_NAME=On-time delivery count
KPI_DOMAIN=OPERATIONS
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Deliveries with actual_delivery_at at or before planned_delivery_at. Not Control Tower ON_TIME. Not in-full.
NUMERATOR=both timestamps and actual <= planned
DENOMINATOR=1
INCLUSION_RULE=actual_delivery_at set on transition to DELIVERED
EXCLUSION_RULE=missing planned or actual
TIME_BASIS=actual_delivery_at versus planned_delivery_at
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-SHIPMENT
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=planned_delivery_at, actual_delivery_at, status
EVENT_TIME=actual_delivery_at from command occurred_at or manual ActualTime
PROCESSING_TIME=updated_at
DIMENSIONS=TENANT, CARRIER, CONSIGNEE, DESTINATION
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=COALESCE first write
CORRECTION_POLICY_CURRENT=NOT_FOUND
NULL_POLICY=exclude nulls; do not use created_at
CURRENT_IMPLEMENTATION=sla.Compute ReasonCompletedLate versus ReasonCompletedOnTime when both actual and planned delivery are set. ReasonOnSchedule is a different branch and must be excluded.
READINESS=PARTIAL
BLOCKER=Must filter to the completed comparison. Raw kpi.onTime includes ReasonOnSchedule.
OWNER_AGENT=C
NOTES=tracking delivery_delay_seconds is a tracking signal, not this field.
```

### OPS_ON_TIME_DELIVERY_RATE

```
KPI_ID=OPS_ON_TIME_DELIVERY_RATE
KPI_NAME=On-time delivery rate
KPI_DOMAIN=OPERATIONS
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=OPS_ON_TIME_DELIVERY over deliveries with a planned delivery and a completion. Denominator not frozen.
NUMERATOR=OPS_ON_TIME_DELIVERY
DENOMINATOR=UNKNOWN
INCLUSION_RULE=completed delivery comparison only
EXCLUSION_RULE=in-flight SLA buckets; empty denominator
TIME_BASIS=actual_delivery_at versus planned_delivery_at
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-SHIPMENT
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=planned_delivery_at, actual_delivery_at
EVENT_TIME=actual_delivery_at
PROCESSING_TIME=updated_at
DIMENSIONS=TENANT, CARRIER
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=COALESCE first write
CORRECTION_POLICY_CURRENT=NOT_FOUND
NULL_POLICY=exclude nulls
CURRENT_IMPLEMENTATION=Control Tower on-time percent. Not this rate.
READINESS=PARTIAL
BLOCKER=Denominator not frozen
OWNER_AGENT=E for the rate; C for timestamps
NOTES=
```

### OPS_LATE_DELIVERY

```
KPI_ID=OPS_LATE_DELIVERY
KPI_NAME=Late delivery count
KPI_DOMAIN=OPERATIONS
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=actual_delivery_at after planned_delivery_at
NUMERATOR=that comparison
DENOMINATOR=1
INCLUSION_RULE=both timestamps
EXCLUSION_RULE=nulls
TIME_BASIS=actual versus planned delivery
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-SHIPMENT
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=planned_delivery_at, actual_delivery_at
EVENT_TIME=actual_delivery_at
PROCESSING_TIME=updated_at
DIMENSIONS=TENANT, CARRIER, DESTINATION
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=COALESCE first write
CORRECTION_POLICY_CURRENT=NOT_FOUND
NULL_POLICY=exclude nulls
CURRENT_IMPLEMENTATION=ReasonCompletedLate and in-flight ReasonDeliveryOverdue (different)
READINESS=PARTIAL
BLOCKER=Completed versus in-flight must stay separate
OWNER_AGENT=C
NOTES=
```

### OPS_AVG_DELIVERY_DELAY_MIN

```
KPI_ID=OPS_AVG_DELIVERY_DELAY_MIN
KPI_NAME=Average delivery delay minutes
KPI_DOMAIN=OPERATIONS
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Mean positive delivery delay. Early-delivery treatment NOT frozen.
NUMERATOR=sum of delay minutes
DENOMINATOR=included deliveries
INCLUSION_RULE=both delivery timestamps
EXCLUSION_RULE=nulls; empty denominator
TIME_BASIS=timestamptz delta
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-SHIPMENT
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=planned_delivery_at, actual_delivery_at
EVENT_TIME=actual_delivery_at
PROCESSING_TIME=updated_at
DIMENSIONS=TENANT, CARRIER
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=COALESCE first write
CORRECTION_POLICY_CURRENT=NOT_FOUND
NULL_POLICY=exclude nulls
CURRENT_IMPLEMENTATION=SLA DelayMinutes, not an average
READINESS=PARTIAL
BLOCKER=Early arrival policy
OWNER_AGENT=C
NOTES=
```

### OPS_OTIF

```
KPI_ID=OPS_OTIF
KPI_NAME=OTIF
KPI_DOMAIN=OPERATIONS
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=On time and in full. Both components are required. DELIVERED is not in full.
NUMERATOR=deliveries that meet both the on-time rule and an in-full fact
DENOMINATOR=completed deliveries in scope; not frozen
INCLUSION_RULE=cannot be evaluated for the fleet
EXCLUSION_RULE=do not treat missing disposition as in full
TIME_BASIS=actual_delivery_at for the time component; in-full event time NOT_FOUND fleet-wide
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-SHIPMENT plus SRC-DISPOSITION
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=planned_delivery_at, actual_delivery_at; disposition quantities only when a case exists
EVENT_TIME=actual_delivery_at; disposition event time on the case when present
PROCESSING_TIME=updated_at
DIMENSIONS=TENANT, CARRIER
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=missing in-full evidence fails the KPI; it does not default to true
CURRENT_IMPLEMENTATION=NOT_FOUND. Control Tower has no OTIF.
READINESS=BLOCKED
BLOCKER=GAP-C-001
OWNER_AGENT=C
NOTES=On-time component alone would be PARTIAL. The combined KPI is BLOCKED.
```

### OPS_DWELL_PICKUP_MIN

```
KPI_ID=OPS_DWELL_PICKUP_MIN
KPI_NAME=Pickup dwell minutes
KPI_DOMAIN=OPERATIONS
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=NOT selected. Alternatives A, B, and C in the dwell review, restricted to pickup stops.
NUMERATOR=sum of the chosen interval
DENOMINATOR=stops with both endpoints
INCLUSION_RULE=pickup action or stop; interval NOT frozen
EXCLUSION_RULE=planned service_duration_seconds; NLO policy duration
TIME_BASIS=stop timestamps
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-EXECUTION-STOP
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=arrived_at, service_started_at, completed_at
EVENT_TIME=the chosen endpoint
PROCESSING_TIME=UNKNOWN for when the stop row was updated
DIMENSIONS=TENANT, LOCATION, CARRIER via shipment
TENANT_SCOPE=operating_tenant_id
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=terminal stop immutable
CORRECTION_POLICY_CURRENT=NOT_FOUND after completion
NULL_POLICY=exclude stops missing either endpoint of the chosen interval
CURRENT_IMPLEMENTATION=NOT_FOUND
READINESS=PARTIAL
BLOCKER=Interval not chosen (0.1B). Not a missing column.
OWNER_AGENT=E for the interval; C for timestamps
NOTES=
```

### OPS_DWELL_DELIVERY_MIN

```
KPI_ID=OPS_DWELL_DELIVERY_MIN
KPI_NAME=Delivery dwell minutes
KPI_DOMAIN=OPERATIONS
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Same alternatives as pickup dwell, for delivery stops.
NUMERATOR=sum of the chosen interval
DENOMINATOR=stops with both endpoints
INCLUSION_RULE=delivery stops
EXCLUSION_RULE=planned durations
TIME_BASIS=stop timestamps
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-EXECUTION-STOP
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=arrived_at, service_started_at, completed_at
EVENT_TIME=the chosen endpoint
PROCESSING_TIME=UNKNOWN
DIMENSIONS=TENANT, LOCATION
TENANT_SCOPE=operating_tenant_id
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=terminal stop immutable
CORRECTION_POLICY_CURRENT=NOT_FOUND
NULL_POLICY=exclude incomplete endpoints
CURRENT_IMPLEMENTATION=NOT_FOUND
READINESS=PARTIAL
BLOCKER=Interval not chosen
OWNER_AGENT=E for the interval; C for timestamps
NOTES=
```

### OPS_TRACKING_COVERAGE

```
KPI_ID=OPS_TRACKING_COVERAGE
KPI_NAME=Tracking coverage
KPI_DOMAIN=OPERATIONS
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=NOT frozen. Could mean any location_event, a binding, a recent ping, or stop milestone consumption. Those are different facts.
NUMERATOR=UNKNOWN
DENOMINATOR=shipments in scope
INCLUSION_RULE=UNKNOWN
EXCLUSION_RULE=do not treat ETA state alone as position coverage
TIME_BASIS=location_event.recorded_at if the definition is ping-based
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-TRACKING
SOURCE_SERVICE=tracking-service
SOURCE_FIELDS=location_event, shipment_tracking_binding, execution_tracking_state
EVENT_TIME=recorded_at
PROCESSING_TIME=received_at
DIMENSIONS=TENANT, SHIPMENT, source_type
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=received_at versus recorded_at can diverge
HISTORY_CLASS=EVENT_HISTORY for pings if retained
LATE_EVENT_POLICY_CURRENT=dedup ON CONFLICT DO NOTHING
CORRECTION_POLICY_CURRENT=NOT_FOUND
NULL_POLICY=no ping means uncovered only after the definition exists
CURRENT_IMPLEMENTATION=tracking state and Control Tower tracking summary components exist as operational views, not this KPI
READINESS=PARTIAL
BLOCKER=Coverage definition
OWNER_AGENT=C
NOTES=Retention of location_event was not proven.
```

### OPS_POD_COMPLETENESS

```
KPI_ID=OPS_POD_COMPLETENESS
KPI_NAME=POD completeness
KPI_DOMAIN=OPERATIONS
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Share of shipments that require a POD and have a satisfying POD document.
NUMERATOR=satisfied POD requirements
DENOMINATOR=shipments that require POD
INCLUSION_RULE=NOT_FOUND
EXCLUSION_RULE=do not divide all POD documents by all delivered shipments
TIME_BASIS=UNKNOWN
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=documents of type POD exist; the requirement does not
SOURCE_SERVICE=document-service and shipment-service
SOURCE_FIELDS=documents.document_type=POD; execution RequiresPOD is false on DELIVERY_COMPLETED
EVENT_TIME=NOT_FOUND for satisfaction
PROCESSING_TIME=document created_at
DIMENSIONS=TENANT, SHIPMENT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=Shipment execution view can list POD documents. That is not completeness.
READINESS=BLOCKED
BLOCKER=GAP-C-002
OWNER_AGENT=C
NOTES=Document trust of the POD file remains Agent B.
```

### OPS_DELIVERY_REJECTION_RATE

```
KPI_ID=OPS_DELIVERY_REJECTION_RATE
KPI_NAME=Delivery rejection rate
KPI_DOMAIN=OPERATIONS
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Rejected quantity over attempted quantity, or shipments with a rejection. Grain not frozen. Cases do not exist for zero rejection, so a fleet rate cannot treat missing cases as zero rejected without a new fact.
NUMERATOR=rejected_quantity where a case exists
DENOMINATOR=UNKNOWN for the fleet
INCLUSION_RULE=disposition cases
EXCLUSION_RULE=do not infer zero from absence
TIME_BASIS=case or attempt fact time
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-DISPOSITION
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=rejected_quantity, attempted_quantity, uom=PALLET
EVENT_TIME=attempt fact time
PROCESSING_TIME=audit created time
DIMENSIONS=TENANT, CARRIER, REASON
TENANT_SCOPE=operating_tenant_id and shipment_tenant_id
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=EVENT_HISTORY for attempt facts; CURRENT_STATE_ONLY for the case
LATE_EVENT_POLICY_CURRENT=append-only quantity facts
CORRECTION_POLICY_CURRENT=append-only
NULL_POLICY=absence of a case is not zero
CURRENT_IMPLEMENTATION=disposition projection in Control Tower
READINESS=PARTIAL
BLOCKER=Fleet denominator
OWNER_AGENT=C
NOTES=
```

### OPS_PARTIAL_REJECTION_RATE

```
KPI_ID=OPS_PARTIAL_REJECTION_RATE
KPI_NAME=Partial rejection rate
KPI_DOMAIN=OPERATIONS
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Cases where accepted_quantity > 0 and rejected_quantity > 0, over a denominator that is not frozen.
NUMERATOR=those cases
DENOMINATOR=UNKNOWN
INCLUSION_RULE=uom PALLET on the case
EXCLUSION_RULE=full rejection and zero rejection are different
TIME_BASIS=attempt facts
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-DISPOSITION
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=accepted_quantity, rejected_quantity, attempted_quantity
EVENT_TIME=attempt fact
PROCESSING_TIME=audit
DIMENSIONS=TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=EVENT_HISTORY for attempts
LATE_EVENT_POLICY_CURRENT=append-only
CORRECTION_POLICY_CURRENT=append-only
NULL_POLICY=absence is not partial
CURRENT_IMPLEMENTATION=NOT_FOUND as a rate
READINESS=PARTIAL
BLOCKER=Denominator and the zero-rejection gap
OWNER_AGENT=C
NOTES=
```

### OPS_RETURN_CASES

```
KPI_ID=OPS_RETURN_CASES
KPI_NAME=Return cases
KPI_DOMAIN=OPERATIONS
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Count of disposition cases whose disposition_type is RETURN_TO_ORIGIN.
NUMERATOR=those cases
DENOMINATOR=1
INCLUSION_RULE=disposition_type=RETURN_TO_ORIGIN
EXCLUSION_RULE=REDIRECT and HOLD
TIME_BASIS=case current state
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-DISPOSITION
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=delivery_disposition_cases.disposition_type, status
EVENT_TIME=UNKNOWN which case column is the business open time; audit exists
PROCESSING_TIME=audit
DIMENSIONS=TENANT, status
TENANT_SCOPE=operating_tenant_id
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=status mutates; type facts are constrained
NULL_POLICY=null disposition_type is not a return
CURRENT_IMPLEMENTATION=disposition model 000093
READINESS=READY
BLOCKER=
OWNER_AGENT=C
NOTES=This is a case count, not a rate of shipments.
```

### OPS_REDIRECT_CASES

```
KPI_ID=OPS_REDIRECT_CASES
KPI_NAME=Redirect cases
KPI_DOMAIN=OPERATIONS
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Count of disposition cases whose disposition_type is REDIRECT.
NUMERATOR=those cases
DENOMINATOR=1
INCLUSION_RULE=disposition_type=REDIRECT
EXCLUSION_RULE=RETURN_TO_ORIGIN and HOLD
TIME_BASIS=current case
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-DISPOSITION
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=disposition_type
EVENT_TIME=UNKNOWN dedicated open timestamp; audit exists
PROCESSING_TIME=audit
DIMENSIONS=TENANT
TENANT_SCOPE=operating_tenant_id
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=null type is not a redirect
CURRENT_IMPLEMENTATION=000093
READINESS=READY
BLOCKER=
OWNER_AGENT=C
NOTES=
```

### OPS_EXCEPTION_RATE

```
KPI_ID=OPS_EXCEPTION_RATE
KPI_NAME=Exception rate
KPI_DOMAIN=OPERATIONS
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=NOT frozen. critical_event_workflow, driver_reported_exception, and shipment risk are different populations.
NUMERATOR=UNKNOWN which store
DENOMINATOR=shipments or stops; UNKNOWN
INCLUSION_RULE=UNKNOWN
EXCLUSION_RULE=do not add the stores together without a crosswalk
TIME_BASIS=workflow or driver report time
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-CT-RISK-EXCEPTION and driver_reported_exception
SOURCE_SERVICE=control-tower-read-model-service and shipment-service
SOURCE_FIELDS=critical_event_workflow; transport.driver_reported_exception
EVENT_TIME=UNKNOWN single column
PROCESSING_TIME=UNKNOWN
DIMENSIONS=TENANT, priority, category
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=Control Tower summary freshness for the workflow side
HISTORY_CLASS=CURRENT_STATE_ONLY for workflow; EVENT_HISTORY for driver reports and actions
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=CalculateExceptionKPI over critical events
READINESS=PARTIAL
BLOCKER=One exception population
OWNER_AGENT=C for driver reports; Control Tower workflow is operational
NOTES=
```

### OPS_P1_EXCEPTION_RATE

```
KPI_ID=OPS_P1_EXCEPTION_RATE
KPI_NAME=P1 exception rate
KPI_DOMAIN=OPERATIONS
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=critical_event_workflow priority p1 over a denominator that is not frozen.
NUMERATOR=workflows with priority p1
DENOMINATOR=UNKNOWN
INCLUSION_RULE=priority check constraint p1, p2, p3, p4 on the workflow (000022)
EXCLUSION_RULE=driver task priority NORMAL/HIGH/CRITICAL is a different scale
TIME_BASIS=workflow timestamps
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-CT-RISK-EXCEPTION
SOURCE_SERVICE=control-tower-read-model-service
SOURCE_FIELDS=critical_event_workflow.priority
EVENT_TIME=UNKNOWN
PROCESSING_TIME=UNKNOWN
DIMENSIONS=TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=exception KPI priority counts
READINESS=PARTIAL
BLOCKER=Denominator; not every operational problem becomes a workflow
OWNER_AGENT=E for the rate; workflow data is Control Tower
NOTES=
```

### OPS_P2_EXCEPTION_RATE

```
KPI_ID=OPS_P2_EXCEPTION_RATE
KPI_NAME=P2 exception rate
KPI_DOMAIN=OPERATIONS
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Same as P1 with priority p2.
NUMERATOR=priority p2
DENOMINATOR=UNKNOWN
INCLUSION_RULE=workflow priority p2
EXCLUSION_RULE=other priority scales
TIME_BASIS=workflow
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-CT-RISK-EXCEPTION
SOURCE_SERVICE=control-tower-read-model-service
SOURCE_FIELDS=priority
EVENT_TIME=UNKNOWN
PROCESSING_TIME=UNKNOWN
DIMENSIONS=TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=exception KPI
READINESS=PARTIAL
BLOCKER=Denominator
OWNER_AGENT=E
NOTES=
```

### OPS_SLA_BREACH_RATE

```
KPI_ID=OPS_SLA_BREACH_RATE
KPI_NAME=SLA breach rate
KPI_DOMAIN=OPERATIONS
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=NOT frozen. Exception workflow breach timestamps and shipment pickup/delivery lateness are different SLAs.
NUMERATOR=UNKNOWN
DENOMINATOR=UNKNOWN
INCLUSION_RULE=UNKNOWN
EXCLUSION_RULE=do not mix workflow ack/assign/resolve breaches with delivery lateness
TIME_BASIS=acknowledge_due_at and related breach columns, or shipment planned versus actual, depending on which SLA
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-CT-RISK-EXCEPTION or SRC-SHIPMENT
SOURCE_SERVICE=control-tower-read-model-service or shipment-service
SOURCE_FIELDS=ack_sla_breached_at, assign_sla_breached_at, resolve_sla_breached_at; or shipment planned/actual
EVENT_TIME=breach timestamp when using the workflow
PROCESSING_TIME=UNKNOWN
DIMENSIONS=TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=null breach timestamp is not a breach
CURRENT_IMPLEMENTATION=workflow SLA fields and shipment sla.Compute
READINESS=PARTIAL
BLOCKER=Two SLA products
OWNER_AGENT=E to choose; C owns execution lateness
NOTES=
```

### OPS_SHIPMENT_CYCLE_TIME

```
KPI_ID=OPS_SHIPMENT_CYCLE_TIME
KPI_NAME=Shipment cycle time
KPI_DOMAIN=OPERATIONS
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=NOT frozen. Candidate pairs include created_at to actual_delivery_at, actual_pickup_at to actual_delivery_at, and status-history intervals. No cycle_time column.
NUMERATOR=sum of the chosen duration
DENOMINATOR=shipments with both endpoints
INCLUSION_RULE=UNKNOWN
EXCLUSION_RULE=do not silently use created_at if the business start is pickup
TIME_BASIS=the chosen pair of timestamptz
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-SHIPMENT and SRC-SHIPMENT-STATUS-HISTORY
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=created_at, actual_pickup_at, actual_delivery_at, status history occurred_at
EVENT_TIME=the end event of the chosen pair
PROCESSING_TIME=recorded_at on history
DIMENSIONS=TENANT, CARRIER
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=STATUS_HISTORY can support status durations when history is complete
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=exclude missing endpoints
CURRENT_IMPLEMENTATION=NOT_FOUND
READINESS=PARTIAL
BLOCKER=Start and end not chosen
OWNER_AGENT=E for the pair; C for timestamps
NOTES=
```

### OPS_DOCUMENT_COMPLETENESS

```
KPI_ID=OPS_DOCUMENT_COMPLETENESS
KPI_NAME=Document completeness
KPI_DOMAIN=OPERATIONS
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Required documents present for the shipment.
NUMERATOR=UNKNOWN
DENOMINATOR=UNKNOWN
INCLUSION_RULE=NOT_FOUND
EXCLUSION_RULE=Control Tower awaitingDocuments is a shipment status count, not this KPI
TIME_BASIS=UNKNOWN
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-DOCUMENT status exists; required set does not
SOURCE_SERVICE=document-service
SOURCE_FIELDS=document_status, document_type, related_entity_id
EVENT_TIME=NOT_FOUND
PROCESSING_TIME=updated_at is not a completeness time
DIMENSIONS=TENANT, SHIPMENT, DOCUMENT_TYPE
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=CalculateKPI awaitingDocuments
READINESS=BLOCKED
BLOCKER=GAP-B-001
OWNER_AGENT=B
NOTES=
```

## Procurement

Two commercial paths exist: `rfx.rfx_events` / responses / awards, and `rfx.freight_requests` / `bids`. A KPI that does not name the path is PARTIAL even when each path has timestamps.

### PROC_RFX_CREATED

```
KPI_ID=PROC_RFX_CREATED
KPI_NAME=RFx created
KPI_DOMAIN=PROCUREMENT
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Count of rfx.rfx_events created in the window. Freight requests are a separate population.
NUMERATOR=events
DENOMINATOR=1
INCLUSION_RULE=rfx_events for the tenant
EXCLUSION_RULE=do not add freight_requests unless a version says so
TIME_BASIS=created_at
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-RFX
SOURCE_SERVICE=rfx-service
SOURCE_FIELDS=rfx_events.created_at, status, tenant_id
EVENT_TIME=created_at
PROCESSING_TIME=created_at
DIMENSIONS=TENANT, rfx_type, CURRENCY
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=RFx APIs
READINESS=READY
BLOCKER=
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN
NOTES=Owner of rfx-service is not Agent A–E.
```

### PROC_RFX_PUBLISHED

```
KPI_ID=PROC_RFX_PUBLISHED
KPI_NAME=RFx published
KPI_DOMAIN=PROCUREMENT
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Events that reached PUBLISHED. Current status is not the publish event. rfx_events.published_at was NOT_FOUND. audit_events action publish with occurred_at is the event signal. rfx_versions.published_at is the questionnaire version, not the event.
NUMERATOR=events with a publish audit or current status in a published-or-later set
DENOMINATOR=1
INCLUSION_RULE=NOT frozen
EXCLUSION_RULE=do not use rfx_versions.published_at as the event publish time without proof they are the same event
TIME_BASIS=audit occurred_at when using the audit
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-RFX
SOURCE_SERVICE=rfx-service
SOURCE_FIELDS=rfx_events.status, audit_events.action, audit_events.occurred_at, rfx_versions.published_at
EVENT_TIME=audit occurred_at for action publish
PROCESSING_TIME=audit write time UNKNOWN relative to occurred_at
DIMENSIONS=TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=EVENT_HISTORY via audit; current status is CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=no audit row means publish time UNKNOWN even if status is PUBLISHED
CURRENT_IMPLEMENTATION=status PUBLISHED; frontend tender funnel counts current PUBLISHED only
READINESS=PARTIAL
BLOCKER=No published_at on the event
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN
NOTES=
```

### PROC_CARRIERS_INVITED

```
KPI_ID=PROC_CARRIERS_INVITED
KPI_NAME=Carriers invited
KPI_DOMAIN=PROCUREMENT
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Count of rfx_participants, which store invited_at.
NUMERATOR=participant rows
DENOMINATOR=1
INCLUSION_RULE=rfx_participants
EXCLUSION_RULE=do not count PUBLISHED events as invitations (the frontend funnel does that)
TIME_BASIS=invited_at
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-RFX
SOURCE_SERVICE=rfx-service
SOURCE_FIELDS=rfx_participants.invited_at, status
EVENT_TIME=invited_at
PROCESSING_TIME=UNKNOWN
DIMENSIONS=TENANT, RFx, CARRIER
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=null invited_at excluded from a time window
CURRENT_IMPLEMENTATION=participant table; frontend count is not this
READINESS=READY
BLOCKER=
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN
NOTES=
```

### PROC_CARRIERS_PARTICIPATED

```
KPI_ID=PROC_CARRIERS_PARTICIPATED
KPI_NAME=Carriers participated
KPI_DOMAIN=PROCUREMENT
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=NOT frozen. viewed_at, responded_at, rfx_responses.submitted_at, and bids.submitted_at are different acts.
NUMERATOR=UNKNOWN
DENOMINATOR=1
INCLUSION_RULE=UNKNOWN
EXCLUSION_RULE=viewed is not participated unless 0.1B says so
TIME_BASIS=responded_at or submitted_at
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-RFX
SOURCE_SERVICE=rfx-service
SOURCE_FIELDS=responded_at, rfx_responses.submitted_at, bids.submitted_at
EVENT_TIME=the chosen submit time
PROCESSING_TIME=UNKNOWN
DIMENSIONS=TENANT, CARRIER, RFx
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=null responded_at is not participation
CURRENT_IMPLEMENTATION=participant and response tables
READINESS=PARTIAL
BLOCKER=Participation definition and two paths
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN
NOTES=
```

### PROC_PARTICIPATION_RATE

```
KPI_ID=PROC_PARTICIPATION_RATE
KPI_NAME=Participation rate
KPI_DOMAIN=PROCUREMENT
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Participants who participated divided by participants invited. Numerator definition is the blocker above.
NUMERATOR=PROC_CARRIERS_PARTICIPATED
DENOMINATOR=PROC_CARRIERS_INVITED
INCLUSION_RULE=same RFx path
EXCLUSION_RULE=empty invited set
TIME_BASIS=invited_at and the participation time
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-RFX
SOURCE_SERVICE=rfx-service
SOURCE_FIELDS=invited_at, responded_at or submitted_at
EVENT_TIME=participation time
PROCESSING_TIME=UNKNOWN
DIMENSIONS=TENANT, RFx
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=NOT_FOUND as a server rate
READINESS=PARTIAL
BLOCKER=Numerator definition
OWNER_AGENT=E for the rate; source is rfx-service
NOTES=
```

### PROC_BIDS_RECEIVED

```
KPI_ID=PROC_BIDS_RECEIVED
KPI_NAME=Bids received
KPI_DOMAIN=PROCUREMENT
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=NOT frozen between rfx_responses and rfx.bids.
NUMERATOR=submitted responses or bids
DENOMINATOR=1
INCLUSION_RULE=submitted_at not null
EXCLUSION_RULE=frontend bidsCount is hardcoded 0 and is not a source
TIME_BASIS=submitted_at
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-RFX
SOURCE_SERVICE=rfx-service
SOURCE_FIELDS=rfx_responses.submitted_at, bids.submitted_at, total_amount, currency_code
EVENT_TIME=submitted_at
PROCESSING_TIME=UNKNOWN
DIMENSIONS=TENANT, RFx, CURRENCY
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=draft without submitted_at excluded
CURRENT_IMPLEMENTATION=both tables exist
READINESS=PARTIAL
BLOCKER=Which object is a bid
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN
NOTES=
```

### PROC_BIDS_PER_RFX

```
KPI_ID=PROC_BIDS_PER_RFX
KPI_NAME=Bids per RFx
KPI_DOMAIN=PROCUREMENT
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Mean bid count per event after PROC_BIDS_RECEIVED is defined.
NUMERATOR=bids
DENOMINATOR=RFx events in the denominator set
INCLUSION_RULE=UNKNOWN
EXCLUSION_RULE=empty denominator
TIME_BASIS=submitted_at
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-RFX
SOURCE_SERVICE=rfx-service
SOURCE_FIELDS=response or bid rows, rfx_event_id
EVENT_TIME=submitted_at
PROCESSING_TIME=UNKNOWN
DIMENSIONS=TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=NOT_FOUND
READINESS=PARTIAL
BLOCKER=Bid identity
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN
NOTES=
```

### PROC_FIRST_BID_TIME

```
KPI_ID=PROC_FIRST_BID_TIME
KPI_NAME=Time to first bid
KPI_DOMAIN=PROCUREMENT
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Earliest submitted_at minus publish time. Publish time is only partial (audit). FIRST_BID is not a stored baseline price.
NUMERATOR=duration
DENOMINATOR=events with both ends
INCLUSION_RULE=min(submitted_at) after a known publish time
EXCLUSION_RULE=events with no publish audit
TIME_BASIS=submitted_at and audit occurred_at
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-RFX
SOURCE_SERVICE=rfx-service
SOURCE_FIELDS=submitted_at, audit action publish
EVENT_TIME=first submitted_at
PROCESSING_TIME=UNKNOWN
DIMENSIONS=TENANT, RFx
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=EVENT_HISTORY if audit is retained
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=missing publish time excludes the event
CURRENT_IMPLEMENTATION=NOT_FOUND
READINESS=PARTIAL
BLOCKER=Publish timestamp
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN
NOTES=
```

### PROC_TIME_TO_AWARD

```
KPI_ID=PROC_TIME_TO_AWARD
KPI_NAME=Time to award
KPI_DOMAIN=PROCUREMENT
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=awarded_at minus a start. Start is not frozen (created_at versus publish).
NUMERATOR=duration
DENOMINATOR=awarded events
INCLUSION_RULE=rfx_awards.awarded_at present
EXCLUSION_RULE=missing start
TIME_BASIS=awarded_at
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-RFX
SOURCE_SERVICE=rfx-service
SOURCE_FIELDS=rfx_awards.awarded_at, rfx_events.created_at, audit publish
EVENT_TIME=awarded_at
PROCESSING_TIME=UNKNOWN
DIMENSIONS=TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY for the award row
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=no award means not in the average
CURRENT_IMPLEMENTATION=000038 awards
READINESS=PARTIAL
BLOCKER=Start anchor
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN
NOTES=UNIQUE (rfx_event_id) on rfx_awards. Partial-award behavior versus that unique key was not fully proven. Status PARTIALLY_AWARDED exists.
```

### PROC_AWARD_RATE

```
KPI_ID=PROC_AWARD_RATE
KPI_NAME=Award rate
KPI_DOMAIN=PROCUREMENT
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Awarded events over published or created events. Denominator not frozen. PARTIALLY_AWARDED is a distinct status.
NUMERATOR=events with an award or status AWARDED
DENOMINATOR=UNKNOWN
INCLUSION_RULE=UNKNOWN
EXCLUSION_RULE=cancelled events treatment UNKNOWN
TIME_BASIS=awarded_at
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-RFX
SOURCE_SERVICE=rfx-service
SOURCE_FIELDS=rfx_awards, rfx_events.status
EVENT_TIME=awarded_at
PROCESSING_TIME=UNKNOWN
DIMENSIONS=TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=award table
READINESS=PARTIAL
BLOCKER=Denominator and partial award
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN
NOTES=
```

### PROC_WINNING_BID_VALUE

```
KPI_ID=PROC_WINNING_BID_VALUE
KPI_NAME=Winning bid value
KPI_DOMAIN=PROCUREMENT
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Award total_amount in its currency_code. Sum only inside one currency.
NUMERATOR=sum of total_amount per currency
DENOMINATOR=1
INCLUSION_RULE=rfx_awards
EXCLUSION_RULE=mixed currency
TIME_BASIS=awarded_at
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-RFX
SOURCE_SERVICE=rfx-service
SOURCE_FIELDS=total_amount, currency_code, awarded_at
EVENT_TIME=awarded_at
PROCESSING_TIME=UNKNOWN
DIMENSIONS=TENANT, CURRENCY, CARRIER
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=null amount excluded
CURRENT_IMPLEMENTATION=000038
READINESS=PARTIAL
BLOCKER=Partial award and offer-line versus award-total grain
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN
NOTES=CURRENCY_SOURCE=rfx_awards.currency_code. FINANCIAL_FINALITY=award commercial value, not paid freight. MIXED_CURRENCY_SAFE=NO.
```

### PROC_TENDER_SAVINGS

```
KPI_ID=PROC_TENDER_SAVINGS
KPI_NAME=Tender savings
KPI_DOMAIN=PROCUREMENT
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Baseline minus awarded amount. Baseline kind is NOT_FOUND.
NUMERATOR=UNKNOWN
DENOMINATOR=1
INCLUSION_RULE=NOT_FOUND
EXCLUSION_RULE=do not use lot estimated_value, first bid, contract rate, or freight-cost lane median unless a contract names that baseline
TIME_BASIS=UNKNOWN
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=NOT_FOUND
SOURCE_SERVICE=rfx-service
SOURCE_FIELDS=NOT_FOUND
EVENT_TIME=NOT_FOUND
PROCESSING_TIME=NOT_FOUND
DIMENSIONS=CURRENCY would be required
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=UNKNOWN
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=NOT_FOUND
READINESS=BLOCKED
BLOCKER=GAP-RFX-001
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN
NOTES=CURRENCY_SOURCE=NOT_FOUND. FINANCIAL_FINALITY=NOT_FOUND. MIXED_CURRENCY_SAFE=NO.
```

### PROC_TENDER_SAVINGS_PCT

```
KPI_ID=PROC_TENDER_SAVINGS_PCT
KPI_NAME=Tender savings percent
KPI_DOMAIN=PROCUREMENT
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Savings divided by baseline. Baseline NOT_FOUND.
NUMERATOR=PROC_TENDER_SAVINGS
DENOMINATOR=baseline amount
INCLUSION_RULE=NOT_FOUND
EXCLUSION_RULE=zero baseline
TIME_BASIS=UNKNOWN
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=NOT_FOUND
SOURCE_SERVICE=rfx-service
SOURCE_FIELDS=NOT_FOUND
EVENT_TIME=NOT_FOUND
PROCESSING_TIME=NOT_FOUND
DIMENSIONS=CURRENCY
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=UNKNOWN
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=NOT_FOUND
READINESS=BLOCKED
BLOCKER=GAP-RFX-001
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN
NOTES=MIXED_CURRENCY_SAFE=NO.
```

### PROC_CONTRACT_VS_SPOT_SHARE

```
KPI_ID=PROC_CONTRACT_VS_SPOT_SHARE
KPI_NAME=Contract versus spot share
KPI_DOMAIN=PROCUREMENT
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=NOT frozen. rfx_type (SPOT_RFQ, CONTRACT_TENDER), freight request request_type, and pricing_source (CONTRACT_RATE, SPOT_BID, RFQ_AWARD, MANUAL_SPOT) are three classifications. Share may be count or money.
NUMERATOR=UNKNOWN class
DENOMINATOR=UNKNOWN
INCLUSION_RULE=UNKNOWN
EXCLUSION_RULE=do not collapse the three taxonomies
TIME_BASIS=award or snapshot time
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-RFX and SRC-CONTRACT-RATE and transport_order_rate_snapshots
SOURCE_SERVICE=rfx-service, contract-rate-service
SOURCE_FIELDS=rfx_type, request_type, pricing_source
EVENT_TIME=UNKNOWN single field
PROCESSING_TIME=UNKNOWN
DIMENSIONS=TENANT, CURRENCY if money share
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=null pricing_source excluded
CURRENT_IMPLEMENTATION=pricing_source on snapshots and settlements (000051, 000052)
READINESS=PARTIAL
BLOCKER=Which taxonomy and whether share is count or spend
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN
NOTES=MIXED_CURRENCY_SAFE=NO if the share is spend.
```

### PROC_LANE_COMPETITION

```
KPI_ID=PROC_LANE_COMPETITION
KPI_NAME=Lane competition
KPI_DOMAIN=PROCUREMENT
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=NOT frozen. Could be bidders per rfx_lane. Lane identity is not one key across RFx, contract rate, and freight-cost lane_key.
NUMERATOR=distinct bidders
DENOMINATOR=lanes
INCLUSION_RULE=UNKNOWN
EXCLUSION_RULE=do not use freight-cost lane_key as an RFx lane without a map
TIME_BASIS=submitted_at
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-RFX
SOURCE_SERVICE=rfx-service
SOURCE_FIELDS=rfx_lanes.id, origin_location_id, destination_location_id, responses
EVENT_TIME=submitted_at
PROCESSING_TIME=UNKNOWN
DIMENSIONS=LANE
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=lane rows exist
READINESS=PARTIAL
BLOCKER=Competition formula and lane identity
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN
NOTES=
```

### PROC_CARRIER_RESPONSE_RATE

```
KPI_ID=PROC_CARRIER_RESPONSE_RATE
KPI_NAME=Carrier response rate
KPI_DOMAIN=PROCUREMENT
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Invited carriers with a response divided by invited carriers. Close to participation. Response versus bid not frozen.
NUMERATOR=participants with responded_at or a submitted response
DENOMINATOR=participants with invited_at
INCLUSION_RULE=rfx_participants
EXCLUSION_RULE=empty denominator
TIME_BASIS=invited_at, responded_at
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-RFX
SOURCE_SERVICE=rfx-service
SOURCE_FIELDS=invited_at, responded_at
EVENT_TIME=responded_at
PROCESSING_TIME=UNKNOWN
DIMENSIONS=TENANT, CARRIER
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=null responded_at is a non-response
CURRENT_IMPLEMENTATION=participant columns
READINESS=PARTIAL
BLOCKER=Response versus submitted bid
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN
NOTES=
```

## Carrier performance

Carrier dimension is `transport.shipments.carrier_company_id` when not null. Scores are not defined.

### CAR_SHIPMENTS_ASSIGNED

```
KPI_ID=CAR_SHIPMENTS_ASSIGNED
KPI_NAME=Shipments assigned
KPI_DOMAIN=CARRIER
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Shipments with a carrier. Status CARRIER_ASSIGNED and a non-null carrier_company_id can diverge. Which one is assigned is not frozen.
NUMERATOR=shipments
DENOMINATOR=1
INCLUSION_RULE=UNKNOWN of the two signals
EXCLUSION_RULE=null carrier if the rule is the column
TIME_BASIS=status history transition into CARRIER_ASSIGNED if that rule is chosen; otherwise current row
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-SHIPMENT
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=carrier_company_id, status
EVENT_TIME=status history occurred_at when present
PROCESSING_TIME=recorded_at
DIMENSIONS=CARRIER, TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=STATUS_HISTORY partial; current carrier is CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=history can be partial
NULL_POLICY=null carrier_company_id is unassigned
CURRENT_IMPLEMENTATION=shipment row
READINESS=PARTIAL
BLOCKER=Status versus column
OWNER_AGENT=C
NOTES=
```

### CAR_SHIPMENTS_COMPLETED

```
KPI_ID=CAR_SHIPMENTS_COMPLETED
KPI_NAME=Shipments completed
KPI_DOMAIN=CARRIER
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=NOT frozen among DELIVERED, DELIVERY_CONFIRMED, DOCUMENTS_COMPLETED, READY_FOR_BILLING, FINANCIALLY_CLOSED.
NUMERATOR=shipments in the chosen status set
DENOMINATOR=1
INCLUSION_RULE=UNKNOWN
EXCLUSION_RULE=CANCELLED
TIME_BASIS=actual_delivery_at if completion means delivery
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-SHIPMENT
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=status, actual_delivery_at, carrier_company_id
EVENT_TIME=actual_delivery_at for delivery completion
PROCESSING_TIME=updated_at
DIMENSIONS=CARRIER, TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=COALESCE on actual_delivery_at
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=status model
READINESS=PARTIAL
BLOCKER=Completion status set
OWNER_AGENT=C
NOTES=
```

### CAR_ACCEPTANCE_RATE

```
KPI_ID=CAR_ACCEPTANCE_RATE
KPI_NAME=Carrier acceptance rate
KPI_DOMAIN=CARRIER
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Transitions to ACCEPTED_BY_CARRIER over assignments offered. Offered population depends on complete status history.
NUMERATOR=accepted transitions
DENOMINATOR=assignment offers; UNKNOWN completeness
INCLUSION_RULE=status history to_status ACCEPTED_BY_CARRIER
EXCLUSION_RULE=partial history flagged SHIPMENT_STATUS_HISTORY_PARTIAL
TIME_BASIS=occurred_at
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-SHIPMENT-STATUS-HISTORY
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=from_status, to_status, occurred_at, carrier_company_id
EVENT_TIME=occurred_at
PROCESSING_TIME=recorded_at
DIMENSIONS=CARRIER
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=STATUS_HISTORY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=partial history warning
NULL_POLICY=incomplete history excluded or marked partial
CURRENT_IMPLEMENTATION=status history 000012
READINESS=PARTIAL
BLOCKER=Offered denominator and partial history
OWNER_AGENT=C
NOTES=
```

### CAR_REJECTION_RATE

```
KPI_ID=CAR_REJECTION_RATE
KPI_NAME=Carrier rejection rate
KPI_DOMAIN=CARRIER
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Carrier rejected an assignment. This is not delivery quantity rejection (see CAR_DELIVERY_REJECTION_RATE) and not an unanswered RFx invite.
NUMERATOR=NOT_FOUND
DENOMINATOR=NOT_FOUND
INCLUSION_RULE=NOT_FOUND
EXCLUSION_RULE=CUSTOMER_REFUSAL disposition is not this KPI
TIME_BASIS=NOT_FOUND
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=NOT_FOUND
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=NOT_FOUND
EVENT_TIME=NOT_FOUND
PROCESSING_TIME=NOT_FOUND
DIMENSIONS=CARRIER
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=UNKNOWN
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=NOT_FOUND
READINESS=BLOCKED
BLOCKER=GAP-C-005
OWNER_AGENT=C
NOTES=
```

### CAR_CANCELLATION_RATE

```
KPI_ID=CAR_CANCELLATION_RATE
KPI_NAME=Carrier cancellation rate
KPI_DOMAIN=CARRIER
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Shipments cancelled by the carrier. Status CANCELLED exists. Actor of the cancel is not proven to be the carrier on every row.
NUMERATOR=cancellations attributed to the carrier
DENOMINATOR=assigned shipments
INCLUSION_RULE=status history to CANCELLED
EXCLUSION_RULE=shipper or system cancels if actor is known
TIME_BASIS=history occurred_at
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-SHIPMENT-STATUS-HISTORY
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=status, status history actor, reason code
EVENT_TIME=occurred_at
PROCESSING_TIME=recorded_at
DIMENSIONS=CARRIER
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=STATUS_HISTORY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=unknown actor must not be counted as the carrier
CURRENT_IMPLEMENTATION=Cancel API and history
READINESS=PARTIAL
BLOCKER=Actor attribution
OWNER_AGENT=C
NOTES=
```

### CAR_ON_TIME_PICKUP_RATE

```
KPI_ID=CAR_ON_TIME_PICKUP_RATE
KPI_NAME=Carrier on-time pickup rate
KPI_DOMAIN=CARRIER
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=OPS_ON_TIME_PICKUP_RATE grouped by carrier_company_id. No second formula.
NUMERATOR=OPS_ON_TIME_PICKUP
DENOMINATOR=same as OPS_ON_TIME_PICKUP_RATE
INCLUSION_RULE=carrier_company_id not null
EXCLUSION_RULE=Control Tower onTime bucket
TIME_BASIS=actual_pickup_at versus planned_pickup_at
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-SHIPMENT
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=carrier_company_id, planned_pickup_at, actual_pickup_at
EVENT_TIME=actual_pickup_at
PROCESSING_TIME=updated_at
DIMENSIONS=CARRIER, TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=COALESCE
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=same as the operations rate
CURRENT_IMPLEMENTATION=NOT_FOUND as a carrier rate
READINESS=PARTIAL
BLOCKER=Same as OPS_ON_TIME_PICKUP_RATE
OWNER_AGENT=C
NOTES=
```

### CAR_ON_TIME_DELIVERY_RATE

```
KPI_ID=CAR_ON_TIME_DELIVERY_RATE
KPI_NAME=Carrier on-time delivery rate
KPI_DOMAIN=CARRIER
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=OPS_ON_TIME_DELIVERY_RATE by carrier. No second formula.
NUMERATOR=OPS_ON_TIME_DELIVERY
DENOMINATOR=same as the operations rate
INCLUSION_RULE=carrier present
EXCLUSION_RULE=ReasonOnSchedule bucket
TIME_BASIS=actual_delivery_at versus planned_delivery_at
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-SHIPMENT
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=carrier_company_id, planned_delivery_at, actual_delivery_at
EVENT_TIME=actual_delivery_at
PROCESSING_TIME=updated_at
DIMENSIONS=CARRIER
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=COALESCE
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=exclude nulls
CURRENT_IMPLEMENTATION=NOT_FOUND
READINESS=PARTIAL
BLOCKER=Denominator not frozen
OWNER_AGENT=C
NOTES=
```

### CAR_OTIF

```
KPI_ID=CAR_OTIF
KPI_NAME=Carrier OTIF
KPI_DOMAIN=CARRIER
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=OPS_OTIF by carrier. Same components. No second formula.
NUMERATOR=OPS_OTIF numerator
DENOMINATOR=OPS_OTIF denominator
INCLUSION_RULE=same as OPS_OTIF
EXCLUSION_RULE=same
TIME_BASIS=same
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-SHIPMENT and SRC-DISPOSITION
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=same as OPS_OTIF plus carrier_company_id
EVENT_TIME=same
PROCESSING_TIME=same
DIMENSIONS=CARRIER
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=missing in-full is not true
CURRENT_IMPLEMENTATION=NOT_FOUND
READINESS=BLOCKED
BLOCKER=GAP-C-001
OWNER_AGENT=C
NOTES=
```

### CAR_AVG_DELAY_MIN

```
KPI_ID=CAR_AVG_DELAY_MIN
KPI_NAME=Carrier average delay minutes
KPI_DOMAIN=CARRIER
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=NOT frozen between pickup delay and delivery delay. Must point at OPS_AVG_PICKUP_DELAY_MIN or OPS_AVG_DELIVERY_DELAY_MIN, or a named sum. A third formula is not allowed.
NUMERATOR=the chosen operations average
DENOMINATOR=same
INCLUSION_RULE=carrier present
EXCLUSION_RULE=UNKNOWN which delay
TIME_BASIS=the chosen actual versus planned
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-SHIPMENT
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=pickup and delivery planned/actual
EVENT_TIME=the chosen actual
PROCESSING_TIME=updated_at
DIMENSIONS=CARRIER
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=COALESCE
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=exclude nulls
CURRENT_IMPLEMENTATION=NOT_FOUND
READINESS=PARTIAL
BLOCKER=Pickup versus delivery
OWNER_AGENT=E to choose the pointer; C owns timestamps
NOTES=
```

### CAR_DELIVERY_REJECTION_RATE

```
KPI_ID=CAR_DELIVERY_REJECTION_RATE
KPI_NAME=Carrier delivery rejection rate
KPI_DOMAIN=CARRIER
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=OPS_DELIVERY_REJECTION_RATE by carrier via shipment. No second formula.
NUMERATOR=OPS_DELIVERY_REJECTION_RATE numerator
DENOMINATOR=same
INCLUSION_RULE=same
EXCLUSION_RULE=absence of a case is not zero
TIME_BASIS=disposition attempt time
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-DISPOSITION
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=rejected_quantity, shipment carrier_company_id
EVENT_TIME=attempt fact
PROCESSING_TIME=audit
DIMENSIONS=CARRIER
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=EVENT_HISTORY for attempts
LATE_EVENT_POLICY_CURRENT=append-only
CORRECTION_POLICY_CURRENT=append-only
NULL_POLICY=same as operations
CURRENT_IMPLEMENTATION=disposition cases
READINESS=PARTIAL
BLOCKER=Fleet denominator
OWNER_AGENT=C
NOTES=
```

### CAR_CARGO_ISSUE_RATE

```
KPI_ID=CAR_CARGO_ISSUE_RATE
KPI_NAME=Cargo issue rate
KPI_DOMAIN=CARRIER
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=NOT frozen. Disposition reason codes include DAMAGE, SHORTAGE, and others. Driver-reported exceptions are another list. Zero-issue deliveries do not open cases.
NUMERATOR=cases or reports in an issue set
DENOMINATOR=UNKNOWN
INCLUSION_RULE=UNKNOWN reason set
EXCLUSION_RULE=do not merge reason lists without a crosswalk
TIME_BASIS=attempt or report time
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-DISPOSITION
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=reason_code
EVENT_TIME=attempt fact
PROCESSING_TIME=audit
DIMENSIONS=CARRIER, reason_code
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=EVENT_HISTORY
LATE_EVENT_POLICY_CURRENT=append-only
CORRECTION_POLICY_CURRENT=append-only
NULL_POLICY=no case is not a proven clean delivery
CURRENT_IMPLEMENTATION=reason codes on cases
READINESS=PARTIAL
BLOCKER=Issue taxonomy and denominator
OWNER_AGENT=C
NOTES=
```

### CAR_DOCUMENT_COMPLETENESS

```
KPI_ID=CAR_DOCUMENT_COMPLETENESS
KPI_NAME=Carrier document completeness
KPI_DOMAIN=CARRIER
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=OPS_DOCUMENT_COMPLETENESS by carrier. No second formula.
NUMERATOR=same
DENOMINATOR=same
INCLUSION_RULE=NOT_FOUND
EXCLUSION_RULE=same
TIME_BASIS=UNKNOWN
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-DOCUMENT
SOURCE_SERVICE=document-service
SOURCE_FIELDS=same gap as OPS_DOCUMENT_COMPLETENESS
EVENT_TIME=NOT_FOUND
PROCESSING_TIME=NOT_FOUND
DIMENSIONS=CARRIER
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=NOT_FOUND
READINESS=BLOCKED
BLOCKER=GAP-B-001
OWNER_AGENT=B
NOTES=
```

### CAR_POD_COMPLETENESS

```
KPI_ID=CAR_POD_COMPLETENESS
KPI_NAME=Carrier POD completeness
KPI_DOMAIN=CARRIER
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=OPS_POD_COMPLETENESS by carrier. No second formula.
NUMERATOR=same
DENOMINATOR=same
INCLUSION_RULE=NOT_FOUND
EXCLUSION_RULE=same
TIME_BASIS=UNKNOWN
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=POD documents exist; requirement does not
SOURCE_SERVICE=shipment-service and document-service
SOURCE_FIELDS=same as OPS_POD_COMPLETENESS
EVENT_TIME=NOT_FOUND
PROCESSING_TIME=NOT_FOUND
DIMENSIONS=CARRIER
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=NOT_FOUND
READINESS=BLOCKED
BLOCKER=GAP-C-002
OWNER_AGENT=C
NOTES=
```

### CAR_EXCEPTION_RATE

```
KPI_ID=CAR_EXCEPTION_RATE
KPI_NAME=Carrier exception rate
KPI_DOMAIN=CARRIER
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=OPS_EXCEPTION_RATE by carrier. No second formula.
NUMERATOR=same
DENOMINATOR=same
INCLUSION_RULE=same
EXCLUSION_RULE=same
TIME_BASIS=same
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-CT-RISK-EXCEPTION
SOURCE_SERVICE=control-tower-read-model-service
SOURCE_FIELDS=workflow plus carrier via shipment
EVENT_TIME=UNKNOWN
PROCESSING_TIME=UNKNOWN
DIMENSIONS=CARRIER
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=exception KPI is not carrier-scoped as a published rate
READINESS=PARTIAL
BLOCKER=Same as OPS_EXCEPTION_RATE
OWNER_AGENT=C
NOTES=
```

### CAR_CRITICAL_EXCEPTION_RATE

```
KPI_ID=CAR_CRITICAL_EXCEPTION_RATE
KPI_NAME=Carrier critical exception rate
KPI_DOMAIN=CARRIER
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=NOT frozen between workflow priority p1 and risk_level critical. Must become a pointer to one of those, not a blend.
NUMERATOR=UNKNOWN
DENOMINATOR=UNKNOWN
INCLUSION_RULE=UNKNOWN
EXCLUSION_RULE=do not add p1 workflows to critical risks
TIME_BASIS=UNKNOWN
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-CT-RISK-EXCEPTION
SOURCE_SERVICE=control-tower-read-model-service
SOURCE_FIELDS=priority, risk_level
EVENT_TIME=UNKNOWN
PROCESSING_TIME=UNKNOWN
DIMENSIONS=CARRIER
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=both counts exist separately
READINESS=PARTIAL
BLOCKER=Which population is critical
OWNER_AGENT=E
NOTES=
```

### CAR_PERFORMANCE_SCORE

```
KPI_ID=CAR_PERFORMANCE_SCORE
KPI_NAME=Carrier performance score
KPI_DOMAIN=CARRIER
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Weighted score. Weights NOT_FOUND.
NUMERATOR=NOT_FOUND
DENOMINATOR=NOT_FOUND
INCLUSION_RULE=NOT_FOUND
EXCLUSION_RULE=do not invent weights from on-time and OTIF
TIME_BASIS=UNKNOWN
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=NOT_FOUND
SOURCE_SERVICE=NOT_FOUND
SOURCE_FIELDS=NOT_FOUND
EVENT_TIME=NOT_FOUND
PROCESSING_TIME=NOT_FOUND
DIMENSIONS=CARRIER
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=UNKNOWN
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=NOT_FOUND
READINESS=BLOCKED
BLOCKER=GAP-E-001
OWNER_AGENT=E
NOTES=SCORING_POLICY_REQUIRED=YES
```

## Cost and finance

Every monetary KPI:

```
MIXED_CURRENCY_SAFE=NO
```

Sum only where `currency_code` is equal. No FX policy, rate source, or rate timestamp was found. Do not present planned amounts as actual amounts.

### FIN_PLANNED_FREIGHT_COST

```
KPI_ID=FIN_PLANNED_FREIGHT_COST
KPI_NAME=Planned freight cost
KPI_DOMAIN=FINANCE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=planned_amount on the cost summary, fed by PLANNED_COST_SNAPSHOT. Not actual, accrued, billed, or paid.
NUMERATOR=sum of planned_amount per currency
DENOMINATOR=1
INCLUSION_RULE=freight_cost.cost_summary_projection rows; data_stage may be PLANNED_ONLY or a later stage that still carries planned_amount
EXCLUSION_RULE=other entry kinds when selecting the planned component
TIME_BASIS=source_occurred_at on the planned entry; projection is current
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-FREIGHT-COST
SOURCE_SERVICE=freight-cost-service
SOURCE_FIELDS=planned_amount, currency_code, data_stage, entry_kind=PLANNED_COST_SNAPSHOT
EVENT_TIME=cost_entry.source_occurred_at
PROCESSING_TIME=cost_entry.recorded_at
DIMENSIONS=TENANT, CURRENCY, TRANSPORT_ORDER
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=projection is current; ledger is the history
HISTORY_CLASS=EVENT_HISTORY via cost_entry; projection is CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=append-only supersedes_entry_id
CORRECTION_POLICY_CURRENT=supersede, not in-place edit
NULL_POLICY=null planned_amount excluded
CURRENT_IMPLEMENTATION=cost_summary_projection
READINESS=READY
BLOCKER=
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN (freight-cost-service)
NOTES=CURRENCY_SOURCE=currency_code. FINANCIAL_FINALITY=planned, not final. Grain is transport order, not shipment.
```

### FIN_ESTIMATED_FREIGHT_COST

```
KPI_ID=FIN_ESTIMATED_FREIGHT_COST
KPI_NAME=Estimated freight cost
KPI_DOMAIN=FINANCE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=An estimate stage distinct from planned.
NUMERATOR=NOT_FOUND
DENOMINATOR=1
INCLUSION_RULE=NOT_FOUND
EXCLUSION_RULE=do not rename PLANNED_COST_SNAPSHOT to estimated
TIME_BASIS=NOT_FOUND
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=NOT_FOUND
SOURCE_SERVICE=freight-cost-service
SOURCE_FIELDS=NOT_FOUND as ESTIMATED
EVENT_TIME=NOT_FOUND
PROCESSING_TIME=NOT_FOUND
DIMENSIONS=CURRENCY
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=UNKNOWN
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=NOT_FOUND
READINESS=NOT_SUPPORTED
BLOCKER=No estimated enum
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN
NOTES=CURRENCY_SOURCE=NOT_FOUND. FINANCIAL_FINALITY=NOT_FOUND. MIXED_CURRENCY_SAFE=NO.
```

### FIN_ACTUAL_FREIGHT_COST

```
KPI_ID=FIN_ACTUAL_FREIGHT_COST
KPI_NAME=Actual freight cost
KPI_DOMAIN=FINANCE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=NOT frozen between current_actual_amount and final_actual_amount. Both exist. Neither is planned_amount.
NUMERATOR=the chosen actual amount per currency
DENOMINATOR=1
INCLUSION_RULE=entry_kind CURRENT_ACTUAL_COST_SNAPSHOT or FINAL_ACTUAL_COST_SNAPSHOT
EXCLUSION_RULE=PLANNED_COST_SNAPSHOT
TIME_BASIS=source_occurred_at
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-FREIGHT-COST
SOURCE_SERVICE=freight-cost-service
SOURCE_FIELDS=current_actual_amount, final_actual_amount, financial_finality, data_stage
EVENT_TIME=source_occurred_at
PROCESSING_TIME=recorded_at
DIMENSIONS=TENANT, CURRENCY, TRANSPORT_ORDER
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=EVENT_HISTORY on the ledger
LATE_EVENT_POLICY_CURRENT=supersede
CORRECTION_POLICY_CURRENT=supersede
NULL_POLICY=null actual is not zero and is not planned
CURRENT_IMPLEMENTATION=cost summary columns
READINESS=PARTIAL
BLOCKER=CURRENT_ACTUAL versus FINAL_ACTUAL
OWNER_AGENT=E to choose the stage; freight-cost-service owns the amounts
NOTES=CURRENCY_SOURCE=currency_code. FINANCIAL_FINALITY=FINAL_ACTUAL only when financial_finality is FINAL_ACTUAL. MIXED_CURRENCY_SAFE=NO. ErrCurrencyMismatch blocks mixed currency inside one order.
```

### FIN_COST_VARIANCE

```
KPI_ID=FIN_COST_VARIANCE
KPI_NAME=Cost variance
KPI_DOMAIN=FINANCE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Chosen actual minus planned, same currency, same transport order. Sign convention NOT frozen.
NUMERATOR=actual minus planned
DENOMINATOR=1
INCLUSION_RULE=both amounts present and currency matches
EXCLUSION_RULE=planned-only rows
TIME_BASIS=the actual entry time
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-FREIGHT-COST
SOURCE_SERVICE=freight-cost-service
SOURCE_FIELDS=planned_amount and the chosen actual column
EVENT_TIME=actual source_occurred_at
PROCESSING_TIME=recorded_at
DIMENSIONS=TENANT, CURRENCY
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=EVENT_HISTORY
LATE_EVENT_POLICY_CURRENT=supersede
CORRECTION_POLICY_CURRENT=supersede
NULL_POLICY=missing actual means variance unknown, not zero
CURRENT_IMPLEMENTATION=both columns on the projection; analytics variance tables exist as derived projections and were not treated as the business definition
READINESS=PARTIAL
BLOCKER=Which actual
OWNER_AGENT=E
NOTES=MIXED_CURRENCY_SAFE=NO.
```

### FIN_COST_VARIANCE_PCT

```
KPI_ID=FIN_COST_VARIANCE_PCT
KPI_NAME=Cost variance percent
KPI_DOMAIN=FINANCE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=FIN_COST_VARIANCE divided by planned amount.
NUMERATOR=FIN_COST_VARIANCE
DENOMINATOR=planned_amount
INCLUSION_RULE=planned_amount non-zero and same currency
EXCLUSION_RULE=zero planned
TIME_BASIS=same as variance
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-FREIGHT-COST
SOURCE_SERVICE=freight-cost-service
SOURCE_FIELDS=same
EVENT_TIME=same
PROCESSING_TIME=same
DIMENSIONS=CURRENCY
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=EVENT_HISTORY
LATE_EVENT_POLICY_CURRENT=supersede
CORRECTION_POLICY_CURRENT=supersede
NULL_POLICY=same
CURRENT_IMPLEMENTATION=NOT_FOUND as a canonical percent
READINESS=PARTIAL
BLOCKER=Which actual
OWNER_AGENT=E
NOTES=MIXED_CURRENCY_SAFE=NO.
```

### FIN_COST_PER_SHIPMENT

```
KPI_ID=FIN_COST_PER_SHIPMENT
KPI_NAME=Cost per shipment
KPI_DOMAIN=FINANCE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Freight cost divided by shipment count. Ledger grain is transport order. Shipment link exists on billing items. The join and the cost stage are not frozen.
NUMERATOR=chosen cost
DENOMINATOR=shipments
INCLUSION_RULE=UNKNOWN join
EXCLUSION_RULE=mixed currency
TIME_BASIS=cost event time
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-FREIGHT-COST and SRC-BILLING
SOURCE_SERVICE=freight-cost-service
SOURCE_FIELDS=projection amounts, billing_register_items shipment link
EVENT_TIME=source_occurred_at
PROCESSING_TIME=recorded_at
DIMENSIONS=TENANT, CURRENCY
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=EVENT_HISTORY
LATE_EVENT_POLICY_CURRENT=supersede
CORRECTION_POLICY_CURRENT=supersede
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=order-level projection
READINESS=PARTIAL
BLOCKER=Order versus shipment grain and cost stage
OWNER_AGENT=E
NOTES=MIXED_CURRENCY_SAFE=NO.
```

### FIN_COST_PER_KM

```
KPI_ID=FIN_COST_PER_KM
KPI_NAME=Cost per kilometre
KPI_DOMAIN=FINANCE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Cost divided by executed kilometres.
NUMERATOR=chosen cost
DENOMINATOR=executed distance km
INCLUSION_RULE=NOT_FOUND
EXCLUSION_RULE=do not use NLO deadhead or min_loaded_distance_km policy
TIME_BASIS=NOT_FOUND
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=cost exists; distance does not
SOURCE_SERVICE=freight-cost-service
SOURCE_FIELDS=NOT_FOUND distance_km
EVENT_TIME=NOT_FOUND
PROCESSING_TIME=NOT_FOUND
DIMENSIONS=CURRENCY
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=UNKNOWN
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=NOT_FOUND
READINESS=BLOCKED
BLOCKER=GAP-C-003
OWNER_AGENT=C
NOTES=MIXED_CURRENCY_SAFE=NO.
```

### FIN_COST_PER_PALLET

```
KPI_ID=FIN_COST_PER_PALLET
KPI_NAME=Cost per pallet
KPI_DOMAIN=FINANCE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Cost divided by pallet_count. pallet_count is nullable. Cost grain is the transport order. Multi-cargo mapping was not proven.
NUMERATOR=chosen cost
DENOMINATOR=sum of pallet_count
INCLUSION_RULE=pallet_count > 0
EXCLUSION_RULE=null pallet_count
TIME_BASIS=cost time; cargo time is current row
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-CARGO and SRC-FREIGHT-COST
SOURCE_SERVICE=transport cargo and freight-cost-service
SOURCE_FIELDS=cargoes.pallet_count, cost amounts, currency_code
EVENT_TIME=cost source_occurred_at
PROCESSING_TIME=cargo updated_at is not a cost time
DIMENSIONS=TENANT, CURRENCY
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY for pallet_count
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=pallet_count can change on the cargo row
NULL_POLICY=null pallet_count excluded
CURRENT_IMPLEMENTATION=000077 pallet_count
READINESS=PARTIAL
BLOCKER=Null coverage, grain, and cost stage
OWNER_AGENT=C for pallets; freight-cost for money
NOTES=MIXED_CURRENCY_SAFE=NO.
```

### FIN_COST_PER_TON

```
KPI_ID=FIN_COST_PER_TON
KPI_NAME=Cost per ton
KPI_DOMAIN=FINANCE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Cost divided by weight in tons.
NUMERATOR=chosen cost
DENOMINATOR=weight in tons
INCLUSION_RULE=NOT_FOUND
EXCLUSION_RULE=do not assume gross_weight is kilograms
TIME_BASIS=NOT_FOUND
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=gross_weight NUMERIC(18,3) without a unit
SOURCE_SERVICE=transport cargo
SOURCE_FIELDS=gross_weight, net_weight
EVENT_TIME=NOT_FOUND
PROCESSING_TIME=cargo updated_at
DIMENSIONS=CURRENCY
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=000003 weight columns
READINESS=BLOCKED
BLOCKER=GAP-C-004
OWNER_AGENT=C
NOTES=MIXED_CURRENCY_SAFE=NO.
```

### FIN_BASE_FREIGHT

```
KPI_ID=FIN_BASE_FREIGHT
KPI_NAME=Base freight
KPI_DOMAIN=FINANCE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=base_freight_amount on settlements and base_amount on register items. Not the whole freight cost.
NUMERATOR=sum per currency
DENOMINATOR=1
INCLUSION_RULE=settlement or register item rows
EXCLUSION_RULE=accessorials and penalties
TIME_BASIS=service_accepted_at or register item time; exact column for every row was not unified
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-BILLING
SOURCE_SERVICE=billing-register-service
SOURCE_FIELDS=freight_settlements.base_freight_amount, billing_register_items.base_amount, currency_code
EVENT_TIME=service_accepted_at on the settlement
PROCESSING_TIME=created_at
DIMENSIONS=TENANT, CURRENCY
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY plus settlement audit
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=audit events
NULL_POLICY=null base excluded
CURRENT_IMPLEMENTATION=000042 settlements, register items
READINESS=READY
BLOCKER=
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN
NOTES=CURRENCY_SOURCE=currency_code. FINANCIAL_FINALITY=settlement or register line, not necessarily paid. MIXED_CURRENCY_SAFE=NO. Population is settlements, not all shipments.
```

### FIN_SURCHARGES

```
KPI_ID=FIN_SURCHARGES
KPI_NAME=Surcharges
KPI_DOMAIN=FINANCE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Accessorial amounts exist in more than one store: settlement_accessorials, billing_register_items.extra_charges, and contract rate components. v1 does not choose one population, and adding them would double-count.
NUMERATOR=NOT frozen
DENOMINATOR=1
INCLUSION_RULE=NOT frozen
EXCLUSION_RULE=base freight and penalties; do not sum the three stores into one total
TIME_BASIS=settlement time when the settlement store is used; not frozen as the KPI clock
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-BILLING
SOURCE_SERVICE=billing-register-service
SOURCE_FIELDS=extra_charges, settlement_accessorials.amount, charge_code
EVENT_TIME=UNKNOWN single column for every accessorial
PROCESSING_TIME=UNKNOWN
DIMENSIONS=TENANT, CURRENCY, charge_code
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY; freight-cost accessorial facts are a derived projection
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=null excluded
CURRENT_IMPLEMENTATION=settlement accessorials and register extra_charges
READINESS=PARTIAL
BLOCKER=One surcharge population is not frozen
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN
NOTES=CURRENCY_SOURCE=parent currency. FINANCIAL_FINALITY=billing line. MIXED_CURRENCY_SAFE=NO. Downgraded from READY in R1 because the inclusion rule was an unresolved OR.
```

### FIN_PENALTIES

```
KPI_ID=FIN_PENALTIES
KPI_NAME=Penalties
KPI_DOMAIN=FINANCE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=billing_register_items.penalties. Not a freight-cost ledger component.
NUMERATOR=sum of penalties per currency
DENOMINATOR=1
INCLUSION_RULE=register items
EXCLUSION_RULE=do not look for penalties on cost_entry
TIME_BASIS=register item time
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-BILLING
SOURCE_SERVICE=billing-register-service
SOURCE_FIELDS=penalties, currency via register
EVENT_TIME=UNKNOWN dedicated penalty event time
PROCESSING_TIME=register timestamps
DIMENSIONS=TENANT, CURRENCY
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY plus register audit
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=audit
NULL_POLICY=null penalties excluded
CURRENT_IMPLEMENTATION=billing register items
READINESS=READY
BLOCKER=
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN
NOTES=CURRENCY_SOURCE=register currency_code. FINANCIAL_FINALITY=register line. MIXED_CURRENCY_SAFE=NO.
```

### FIN_BILLED_AMOUNT

```
KPI_ID=FIN_BILLED_AMOUNT
KPI_NAME=Billed amount
KPI_DOMAIN=FINANCE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=BILLED_COST_SNAPSHOT / billing_register_amount, or register total_with_vat. Those can differ. This catalog pins the KPI to freight_cost billing_register_amount and BILLED_COST_SNAPSHOT, per currency. Register totals are a related source, not a second KPI id.
NUMERATOR=sum of billing_register_amount per currency
DENOMINATOR=1
INCLUSION_RULE=data_stage at or beyond BILLING_LINKED where the amount is present
EXCLUSION_RULE=planned amount
TIME_BASIS=billing snapshot source_occurred_at
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-FREIGHT-COST
SOURCE_SERVICE=freight-cost-service
SOURCE_FIELDS=billing_register_amount, entry_kind=BILLED_COST_SNAPSHOT, currency_code
EVENT_TIME=source_occurred_at
PROCESSING_TIME=recorded_at
DIMENSIONS=TENANT, CURRENCY
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=EVENT_HISTORY
LATE_EVENT_POLICY_CURRENT=supersede
CORRECTION_POLICY_CURRENT=supersede
NULL_POLICY=null means not billed
CURRENT_IMPLEMENTATION=ledger and projection
READINESS=READY
BLOCKER=
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN
NOTES=There is no INVOICED entry kind. invoices.invoice_date is a closing document date, not this amount. CURRENCY_SOURCE=currency_code. FINANCIAL_FINALITY=billed, not paid. MIXED_CURRENCY_SAFE=NO.
```

### FIN_PAID_AMOUNT

```
KPI_ID=FIN_PAID_AMOUNT
KPI_NAME=Paid amount
KPI_DOMAIN=FINANCE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=paid_amount on the cost projection (PAID_AMOUNT_SNAPSHOT) and payment_obligations.paid_amount. Pin this KPI to payment_obligations.paid_amount per currency so it stays the payment fact. The ledger snapshot is the downstream copy.
NUMERATOR=sum of paid_amount per currency
DENOMINATOR=1
INCLUSION_RULE=payment obligations
EXCLUSION_RULE=voided payments treatment must follow payment status; void semantics exist (voided_at) and a full void policy for the KPI is still the payment row
TIME_BASIS=payment_date
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-PAYMENT
SOURCE_SERVICE=payment-service
SOURCE_FIELDS=payment_obligations.paid_amount, payments.amount, currency_code, status
EVENT_TIME=payment_date
PROCESSING_TIME=UNKNOWN relative to payment_date
DIMENSIONS=TENANT, CURRENCY
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY plus payment audit and paid snapshot events
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=voided_at
NULL_POLICY=null paid is not outstanding
CURRENT_IMPLEMENTATION=000045 and paid snapshot to freight-cost
READINESS=READY
BLOCKER=
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN
NOTES=CURRENCY_SOURCE=payments.currency_code. FINANCIAL_FINALITY=paid, not reconciled unless status is RECONCILED. MIXED_CURRENCY_SAFE=NO.
```

### FIN_OUTSTANDING_AMOUNT

```
KPI_ID=FIN_OUTSTANDING_AMOUNT
KPI_NAME=Outstanding amount
KPI_DOMAIN=FINANCE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=payment_obligations.outstanding_amount for open or partially paid obligations.
NUMERATOR=sum per currency
DENOMINATOR=1
INCLUSION_RULE=status OPEN or PARTIALLY_PAID
EXCLUSION_RULE=PAID, CANCELLED, VOIDED
TIME_BASIS=as-of now; historical as-of NOT_FOUND
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-PAYMENT
SOURCE_SERVICE=payment-service
SOURCE_FIELDS=outstanding_amount, status, currency
EVENT_TIME=UNKNOWN; the column is current
PROCESSING_TIME=obligation updated time
DIMENSIONS=TENANT, CURRENCY
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=current obligation row
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=in-place amount updates
NULL_POLICY=null excluded
CURRENT_IMPLEMENTATION=payment_obligations
READINESS=READY
BLOCKER=
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN
NOTES=CURRENCY_SOURCE=obligation currency. FINANCIAL_FINALITY=open exposure, not final. MIXED_CURRENCY_SAFE=NO. Not a historical aging stock.
```

### FIN_PAYMENT_AGING_DAYS

```
KPI_ID=FIN_PAYMENT_AGING_DAYS
KPI_NAME=Payment aging days
KPI_DOMAIN=FINANCE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Days past due_date for open obligations. Bucket boundaries NOT frozen. due_date is a date. Reporting timezone is UNDEFINED, which matters less for a date than for a timestamptz but the as-of clock is still not a policy.
NUMERATOR=age days
DENOMINATOR=open obligations
INCLUSION_RULE=IsObligationOverdue uses due_date versus today
EXCLUSION_RULE=paid obligations
TIME_BASIS=due_date
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-PAYMENT
SOURCE_SERVICE=payment-service
SOURCE_FIELDS=due_date, outstanding_amount, status
EVENT_TIME=due_date
PROCESSING_TIME=UNKNOWN
DIMENSIONS=TENANT, CURRENCY
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=current row
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=null due_date excluded
CURRENT_IMPLEMENTATION=IsObligationOverdue; no bucket table
READINESS=PARTIAL
BLOCKER=As-of policy and buckets
OWNER_AGENT=E for buckets; payment-service for due_date
NOTES=MIXED_CURRENCY_SAFE=NO if aging is shown as money.
```

### FIN_DAYS_TO_INVOICE

```
KPI_ID=FIN_DAYS_TO_INVOICE
KPI_NAME=Days to invoice
KPI_DOMAIN=FINANCE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=invoice_date minus a start event. Start is not frozen (actual delivery, service_accepted_at, or delivered status).
NUMERATOR=day difference
DENOMINATOR=invoiced shipments or registers
INCLUSION_RULE=invoices.invoice_date present
EXCLUSION_RULE=missing start
TIME_BASIS=invoice_date
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-BILLING
SOURCE_SERVICE=billing-register-service
SOURCE_FIELDS=invoices.invoice_date, freight_settlements.service_accepted_at
EVENT_TIME=invoice_date
PROCESSING_TIME=invoices.created_at
DIMENSIONS=TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=missing anchor excluded
CURRENT_IMPLEMENTATION=invoices table
READINESS=PARTIAL
BLOCKER=Start event
OWNER_AGENT=E
NOTES=
```

### FIN_DAYS_TO_PAYMENT

```
KPI_ID=FIN_DAYS_TO_PAYMENT
KPI_NAME=Days to payment
KPI_DOMAIN=FINANCE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=payment_date minus invoice_date or due_date. Anchor not frozen.
NUMERATOR=day difference
DENOMINATOR=payments
INCLUSION_RULE=payment_date present
EXCLUSION_RULE=voided payments
TIME_BASIS=payment_date
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-PAYMENT and SRC-BILLING
SOURCE_SERVICE=payment-service
SOURCE_FIELDS=payment_date, value_date, invoice_date, due_date
EVENT_TIME=payment_date
PROCESSING_TIME=UNKNOWN
DIMENSIONS=TENANT, CURRENCY
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=voided_at
NULL_POLICY=missing anchor excluded
CURRENT_IMPLEMENTATION=payments and invoices
READINESS=PARTIAL
BLOCKER=Anchor
OWNER_AGENT=E
NOTES=value_date is a different date from payment_date.
```

### FIN_FINANCIALLY_CLOSED_AMOUNT

```
KPI_ID=FIN_FINANCIALLY_CLOSED_AMOUNT
KPI_NAME=Financially closed amount
KPI_DOMAIN=FINANCE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=NOT frozen. Candidates: billing register status CLOSED, freight-cost financial_finality FINAL_ACTUAL, shipment status FINANCIALLY_CLOSED. They are not the same set.
NUMERATOR=amount under the chosen marker
DENOMINATOR=1
INCLUSION_RULE=UNKNOWN
EXCLUSION_RULE=do not OR the three markers together
TIME_BASIS=UNKNOWN
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-BILLING, SRC-FREIGHT-COST, SRC-SHIPMENT
SOURCE_SERVICE=billing-register-service, freight-cost-service, shipment-service
SOURCE_FIELDS=register status, financial_finality, shipment status
EVENT_TIME=UNKNOWN single close time
PROCESSING_TIME=audit
DIMENSIONS=TENANT, CURRENCY
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=all three markers exist
READINESS=PARTIAL
BLOCKER=Close marker
OWNER_AGENT=E
NOTES=MIXED_CURRENCY_SAFE=NO. Shipment FINANCIALLY_CLOSED has no amount by itself.
```

## Documents

Three document concepts stay separate:

| Concept | KPI | What it counts |
| --- | --- | --- |
| WORKFLOW_SIGNED | DOC_SIGNED_STATUS_COUNT | Current `document_status=SIGNED` |
| CRYPTOGRAPHICALLY_VERIFIED | DOC_VERIFIED_SIGNATURE_COUNT | A verifier result that the signature checked out |
| QUALIFIED_TRUSTED | DOC_QUALIFIED_TRUST_COUNT | Qualified policy `QUALIFIED_CADES_BES` |

`DocumentStatus=SIGNED` is not cryptographic verification and not qualified trust. Legacy `verification_status=VALID` is not cryptographic verification. `VERIFIER_UNAVAILABLE` is not verified.

### DOC_DOCUMENTS_CREATED

```
KPI_ID=DOC_DOCUMENTS_CREATED
KPI_NAME=Documents created
KPI_DOMAIN=DOCUMENT
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=v1 count of documents.documents rows the canonical read already returns: tenant match and deleted_at IS NULL (document_repository.go list filter).
NUMERATOR=those rows
DENOMINATOR=1
INCLUSION_RULE=tenant_id match and deleted_at IS NULL
EXCLUSION_RULE=deleted_at IS NOT NULL. A past-day snapshot that puts later-deleted documents back in is out of v1.
TIME_BASIS=created_at
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-DOCUMENT
SOURCE_SERVICE=document-service
SOURCE_FIELDS=created_at, document_type, tenant_id
EVENT_TIME=created_at
PROCESSING_TIME=created_at
DIMENSIONS=TENANT, DOCUMENT_TYPE
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=000005
READINESS=READY
BLOCKER=
OWNER_AGENT=B
NOTES=
```

### DOC_READY_FOR_SIGNING

```
KPI_ID=DOC_READY_FOR_SIGNING
KPI_NAME=Ready for signing
KPI_DOMAIN=DOCUMENT
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Current document_status READY_FOR_SIGNING. No dedicated ready timestamp.
NUMERATOR=rows in that status
DENOMINATOR=1
INCLUSION_RULE=status READY_FOR_SIGNING
EXCLUSION_RULE=frontend also folds SIGNING_IN_PROGRESS into a ready bucket; this KPI does not
TIME_BASIS=current status
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-DOCUMENT
SOURCE_SERVICE=document-service
SOURCE_FIELDS=document_status
EVENT_TIME=NOT_FOUND
PROCESSING_TIME=updated_at is not the ready event
DIMENSIONS=TENANT, DOCUMENT_TYPE
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=status enum
READINESS=READY
BLOCKER=
OWNER_AGENT=B
NOTES=Current stock only. A historical "became ready on day D" series is not supported.
```

### DOC_SIGNED_STATUS_COUNT

```
KPI_ID=DOC_SIGNED_STATUS_COUNT
KPI_NAME=Signed status count
KPI_DOMAIN=DOCUMENT
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Current document_status SIGNED. Workflow only.
NUMERATOR=rows
DENOMINATOR=1
INCLUSION_RULE=document_status=SIGNED
EXCLUSION_RULE=do not include ACCEPTED unless a later version says so; the frontend summary does include ACCEPTED
TIME_BASIS=current status
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-DOCUMENT
SOURCE_SERVICE=document-service
SOURCE_FIELDS=document_status
EVENT_TIME=signatures.signed_at exists for the legacy session; it is not qualified verification time
PROCESSING_TIME=updated_at
DIMENSIONS=TENANT, DOCUMENT_TYPE
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=status enum; ADR-EDO-010
READINESS=READY
BLOCKER=
OWNER_AGENT=B
NOTES=READY only as a status count. It is not trust.
```

### DOC_VERIFIED_SIGNATURE_COUNT

```
KPI_ID=DOC_VERIFIED_SIGNATURE_COUNT
KPI_NAME=Verified signature count
KPI_DOMAIN=DOCUMENT
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Count of signatures that a cryptographic verifier has accepted. This is not WORKFLOW_SIGNED and not QUALIFIED_TRUSTED.
NUMERATOR=NOT_FOUND as a trustworthy population
DENOMINATOR=signatures in scope once a verifier result exists
INCLUSION_RULE=NOT_FOUND
EXCLUSION_RULE=legacy verification_status=VALID; document_status=SIGNED; evidence reason VERIFIER_UNAVAILABLE; 000096 PENDING
TIME_BASIS=NOT_FOUND
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-SIGNATURE-EVIDENCE
SOURCE_SERVICE=document-service
SOURCE_FIELDS=legacy signatures.verification_status; signature_verification_evidence.verification_status and reason_code
EVENT_TIME=NOT_FOUND for a real verification
PROCESSING_TIME=evidence created_at records an unavailable verifier, not a success
DIMENSIONS=TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=EVENT_HISTORY for evidence attempts
LATE_EVENT_POLICY_CURRENT=append-only evidence
CORRECTION_POLICY_CURRENT=a later attempt would be a new row; none can be VALID under 000096
NULL_POLICY=do not coerce missing verification to verified
CURRENT_IMPLEMENTATION=ADR-EDO-010 and 000096
READINESS=BLOCKED
BLOCKER=GAP-B-003
OWNER_AGENT=B
NOTES=LEGACY_VALID_COUNTED_AS_CRYPTO_VERIFIED=NO. DOCUMENT_STATUS_SIGNED_COUNTED_AS_QUALIFIED_TRUST=NO. Qualified trust remains GAP-B-002.
```

### DOC_QUALIFIED_TRUST_COUNT

```
KPI_ID=DOC_QUALIFIED_TRUST_COUNT
KPI_NAME=Qualified trust count
KPI_DOMAIN=DOCUMENT
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Documents or signatures with qualified verification under policy QUALIFIED_CADES_BES.
NUMERATOR=successful qualified verifications
DENOMINATOR=attempts or documents
INCLUSION_RULE=evidence status other than PENDING with a real verifier
EXCLUSION_RULE=document_status SIGNED; legacy VALID
TIME_BASIS=attempted_at
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-SIGNATURE-EVIDENCE
SOURCE_SERVICE=document-service
SOURCE_FIELDS=signature_verification_evidence.verification_status, reason_code, policy_id
EVENT_TIME=attempted_at
PROCESSING_TIME=created_at
DIMENSIONS=TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=EVENT_HISTORY
LATE_EVENT_POLICY_CURRENT=append-only evidence
CORRECTION_POLICY_CURRENT=new attempt row
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=000096 allows PENDING and VERIFIER_UNAVAILABLE only. ADR-EDO-010 QUALIFIED_DOCUMENT_TRUST_READY=NO.
READINESS=BLOCKED
BLOCKER=GAP-B-002
OWNER_AGENT=B
NOTES=A count of VERIFIER_UNAVAILABLE attempts is possible and is not this KPI.
```

### DOC_SIGNATURE_PENDING_COUNT

```
KPI_ID=DOC_SIGNATURE_PENDING_COUNT
KPI_NAME=Signature pending count
KPI_DOMAIN=DOCUMENT
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=NOT frozen between signing session in progress, attachment verification_status PENDING, and evidence reason VERIFIER_UNAVAILABLE.
NUMERATOR=UNKNOWN
DENOMINATOR=1
INCLUSION_RULE=UNKNOWN
EXCLUSION_RULE=do not add the populations
TIME_BASIS=session created_at or attempted_at
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-DOCUMENT and SRC-SIGNATURE-EVIDENCE
SOURCE_SERVICE=document-service
SOURCE_FIELDS=document_status SIGNING_IN_PROGRESS, evidence PENDING
EVENT_TIME=UNKNOWN single field
PROCESSING_TIME=created_at
DIMENSIONS=TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY for document status; EVENT_HISTORY for evidence
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=both states exist
READINESS=PARTIAL
BLOCKER=Which pending
OWNER_AGENT=B
NOTES=
```

### DOC_DOCUMENT_COMPLETENESS

```
KPI_ID=DOC_DOCUMENT_COMPLETENESS
KPI_NAME=Document completeness
KPI_DOMAIN=DOCUMENT
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Same fact as OPS_DOCUMENT_COMPLETENESS. No second formula.
NUMERATOR=OPS_DOCUMENT_COMPLETENESS
DENOMINATOR=same
INCLUSION_RULE=NOT_FOUND
EXCLUSION_RULE=same
TIME_BASIS=UNKNOWN
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-DOCUMENT
SOURCE_SERVICE=document-service
SOURCE_FIELDS=NOT_FOUND completeness
EVENT_TIME=NOT_FOUND
PROCESSING_TIME=NOT_FOUND
DIMENSIONS=TENANT, SHIPMENT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=UNKNOWN
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=NOT_FOUND
READINESS=BLOCKED
BLOCKER=GAP-B-001
OWNER_AGENT=B
NOTES=
```

### DOC_TIME_TO_SIGNATURE

```
KPI_ID=DOC_TIME_TO_SIGNATURE
KPI_NAME=Time to signature
KPI_DOMAIN=DOCUMENT
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=signed_at minus ready time. Ready time has no column. signing_sessions.created_at is a candidate start and is not proven to be the business ready event. updated_at must not be used.
NUMERATOR=duration
DENOMINATOR=signed documents with a known start
INCLUSION_RULE=legacy signatures.signed_at
EXCLUSION_RULE=updated_at as the start
TIME_BASIS=signed_at
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-DOCUMENT
SOURCE_SERVICE=document-service
SOURCE_FIELDS=signatures.signed_at, signing_sessions.created_at
EVENT_TIME=signed_at
PROCESSING_TIME=UNKNOWN
DIMENSIONS=TENANT, DOCUMENT_TYPE
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=missing start excluded
CURRENT_IMPLEMENTATION=legacy signature row
READINESS=PARTIAL
BLOCKER=Start timestamp
OWNER_AGENT=B
NOTES=This duration is not a qualified-trust duration.
```

## Network intelligence

Observability metrics are listed so they are not later mistaken for business KPIs. Agent D owns planned search and plan facts. Agent C owns executed distance. Design documents for backhaul are not facts. PR #218 was not in main at R1.

```
CANDIDATE_ROAD_DEADHEAD_KM_FACT=FOUND
CANDIDATE_ROAD_DEADHEAD_COLUMN=network_optimizer.match_candidates.road_deadhead_km
CANDIDATE_ROAD_DEADHEAD_IS_EXECUTED_DEADHEAD=NO
CANDIDATE_ROAD_DEADHEAD_IS_FLEET_DEADHEAD=NO
NLO_PLAN_OWNER=YES
NLO_EXECUTION_OWNER=NO
TMS_EXECUTION_OWNER=YES
```

`NET_LOADED_KM`, `NET_EMPTY_KM`, `NET_DEADHEAD_KM`, and `NET_DEADHEAD_PCT` mean executed network distance. They do not mean a sum of candidate-search deadhead. Planned candidate deadhead may be used for search diagnostics and candidate comparison. It is not one of these KPI ids.

### NET_CANDIDATES_DISCOVERED

```
KPI_ID=NET_CANDIDATES_DISCOVERED
KPI_NAME=Candidates discovered
KPI_DOMAIN=NETWORK
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Candidates before prefilter. Persisted rows are match_candidates after search processing. A distinct discovered population was not proven.
NUMERATOR=UNKNOWN
DENOMINATOR=1
INCLUSION_RULE=UNKNOWN
EXCLUSION_RULE=do not count Prometheus discovery-cap counters as the business population
TIME_BASIS=search run time
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-NLO-SEARCH
SOURCE_SERVICE=network-optimizer-service
SOURCE_FIELDS=next_load_search_runs, match_candidates
EVENT_TIME=UNKNOWN exact run column in this inventory
PROCESSING_TIME=UNKNOWN
DIMENSIONS=TENANT; anonymized marketplace must stay anonymized
TENANT_SCOPE=owner_tenant_id
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=EVENT_HISTORY if runs are retained; retention UNKNOWN
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=search persistence
READINESS=PARTIAL
BLOCKER=Discovered-versus-stored stage
OWNER_AGENT=D
NOTES=Unstable if D changes search stages.
```

### NET_CANDIDATES_PREFILTERED

```
KPI_ID=NET_CANDIDATES_PREFILTERED
KPI_NAME=Candidates prefiltered
KPI_DOMAIN=NETWORK
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=A persisted prefilter stage.
NUMERATOR=NOT_FOUND
DENOMINATOR=1
INCLUSION_RULE=NOT_FOUND
EXCLUSION_RULE=Prometheus cap counters are observability
TIME_BASIS=NOT_FOUND
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=NOT_FOUND as its own fact
SOURCE_SERVICE=network-optimizer-service
SOURCE_FIELDS=NOT_FOUND
EVENT_TIME=NOT_FOUND
PROCESSING_TIME=NOT_FOUND
DIMENSIONS=TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=UNKNOWN
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=OBSERVABILITY_ONLY counters
READINESS=NOT_SUPPORTED
BLOCKER=No business prefilter fact
OWNER_AGENT=D
NOTES=
```

### NET_CANDIDATES_ROUTED

```
KPI_ID=NET_CANDIDATES_ROUTED
KPI_NAME=Candidates routed
KPI_DOMAIN=NETWORK
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Candidates with a road distance result. road_deadhead_km is stored. Straight-line must not be stored as that field.
NUMERATOR=candidates with road_deadhead_km
DENOMINATOR=1
INCLUSION_RULE=non-null road_deadhead_km
EXCLUSION_RULE=straight-line substitutes
TIME_BASIS=search run
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-NLO-SEARCH
SOURCE_SERVICE=network-optimizer-service
SOURCE_FIELDS=match_candidates.road_deadhead_km, routing_provider
EVENT_TIME=search time
PROCESSING_TIME=UNKNOWN
DIMENSIONS=TENANT, routing_provider
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=EVENT_HISTORY if retained
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=null road distance is not routed
CURRENT_IMPLEMENTATION=000079
READINESS=PARTIAL
BLOCKER=Whether every routed candidate persists that column was not exhaustively proven for all search modes
OWNER_AGENT=D
NOTES=
```

### NET_CANDIDATES_FEASIBLE

```
KPI_ID=NET_CANDIDATES_FEASIBLE
KPI_NAME=Candidates feasible
KPI_DOMAIN=NETWORK
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Candidates whose eligibility says feasible. Exact eligibility enum values were not copied into this catalog.
NUMERATOR=eligible candidates
DENOMINATOR=1
INCLUSION_RULE=match_candidates.eligibility
EXCLUSION_RULE=reject_reasons population
TIME_BASIS=search time
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-NLO-SEARCH
SOURCE_SERVICE=network-optimizer-service
SOURCE_FIELDS=eligibility, reject_reasons
EVENT_TIME=search time
PROCESSING_TIME=UNKNOWN
DIMENSIONS=TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=EVENT_HISTORY if retained
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=000079 columns
READINESS=PARTIAL
BLOCKER=Eligibility vocabulary not frozen for analytics; D may change feasibility
OWNER_AGENT=D
NOTES=
```

### NET_CANDIDATES_RANKED

```
KPI_ID=NET_CANDIDATES_RANKED
KPI_NAME=Candidates ranked
KPI_DOMAIN=NETWORK
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Ranked candidates. score_components jsonb exists (000080). A stable rank column was not confirmed as the analytics contract.
NUMERATOR=ranked rows
DENOMINATOR=1
INCLUSION_RULE=UNKNOWN
EXCLUSION_RULE=do not treat score JSON as a versioned KPI
TIME_BASIS=search time
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-NLO-SEARCH
SOURCE_SERVICE=network-optimizer-service
SOURCE_FIELDS=score_components
EVENT_TIME=search time
PROCESSING_TIME=UNKNOWN
DIMENSIONS=TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=EVENT_HISTORY if retained
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=score JSON
READINESS=PARTIAL
BLOCKER=Rank contract
OWNER_AGENT=D
NOTES=Scoring changes are an explicit D dependency.
```

### NET_SEARCH_DURATION

```
KPI_ID=NET_SEARCH_DURATION
KPI_NAME=Search duration
KPI_DOMAIN=NETWORK
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Not a business KPI. Prometheus histogram bno_search_duration_seconds.
NUMERATOR=NOT a business fact
DENOMINATOR=NOT a business fact
INCLUSION_RULE=observability
EXCLUSION_RULE=do not put this in executive BI
TIME_BASIS=scrape
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-NLO-METRICS
SOURCE_SERVICE=network-optimizer-service
SOURCE_FIELDS=bno_search_duration_seconds
EVENT_TIME=NOT_FOUND as a business event
PROCESSING_TIME=metric observation
DIMENSIONS=must not use high-cardinality business ids
TENANT_SCOPE=UNKNOWN metric labels
FRESHNESS=scrape
HISTORY_CLASS=UNKNOWN
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=metrics.go
READINESS=NOT_SUPPORTED
BLOCKER=Observability only
OWNER_AGENT=D
NOTES=
```

### NET_PROVIDER_CALLS

```
KPI_ID=NET_PROVIDER_CALLS
KPI_NAME=Provider calls
KPI_DOMAIN=NETWORK
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Not a business KPI. NOT_FOUND as provider_calls_total. routing_provider is stored on the search run. Routing errors and matrix batches are Prometheus counters.
NUMERATOR=NOT_FOUND
DENOMINATOR=1
INCLUSION_RULE=NOT_FOUND
EXCLUSION_RULE=do not count bno_routing_errors_total as calls
TIME_BASIS=NOT_FOUND
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-NLO-METRICS
SOURCE_SERVICE=network-optimizer-service
SOURCE_FIELDS=routing_provider on the run; error and batch metrics
EVENT_TIME=NOT_FOUND
PROCESSING_TIME=scrape
DIMENSIONS=routing_provider on the business row is allowed; metric labels must stay low cardinality
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=UNKNOWN
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=metrics.go and search run column
READINESS=NOT_SUPPORTED
BLOCKER=No call fact
OWNER_AGENT=D
NOTES=
```

### NET_DEADHEAD_KM

```
KPI_ID=NET_DEADHEAD_KM
KPI_NAME=Deadhead kilometres
KPI_DOMAIN=NETWORK
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Executed network deadhead kilometres. Not a sum of candidate-search alternatives.
NUMERATOR=NOT_FOUND
DENOMINATOR=1
INCLUSION_RULE=NOT_FOUND
EXCLUSION_RULE=do not sum match_candidates.road_deadhead_km; that column is candidate diagnostics only
TIME_BASIS=NOT_FOUND
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=NOT_FOUND for executed deadhead
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=NOT_FOUND executed deadhead; candidate column is not this KPI
EVENT_TIME=NOT_FOUND
PROCESSING_TIME=NOT_FOUND
DIMENSIONS=TENANT
TENANT_SCOPE=operating tenant of the execution
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=UNKNOWN
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=NOT_FOUND
READINESS=BLOCKED
BLOCKER=GAP-C-006
OWNER_AGENT=C
NOTES=CANDIDATE_ROAD_DEADHEAD_KM_FACT=FOUND. CANDIDATE_ROAD_DEADHEAD_IS_EXECUTED_DEADHEAD=NO. CANDIDATE_ROAD_DEADHEAD_IS_FLEET_DEADHEAD=NO. BLOCKED because the candidate column can be mistaken for this KPI. Planned NLO distance, if added later, needs its own KPI id and stays PLANNED.
```

### NET_LOADED_KM

```
KPI_ID=NET_LOADED_KM
KPI_NAME=Loaded kilometres
KPI_DOMAIN=NETWORK
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Executed loaded distance.
NUMERATOR=NOT_FOUND
DENOMINATOR=1
INCLUSION_RULE=NOT_FOUND
EXCLUSION_RULE=min_loaded_distance_km is an NLO policy threshold, not executed loaded distance
TIME_BASIS=NOT_FOUND
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=NOT_FOUND
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=NOT_FOUND
EVENT_TIME=NOT_FOUND
PROCESSING_TIME=NOT_FOUND
DIMENSIONS=TENANT
TENANT_SCOPE=operating tenant of the execution
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=UNKNOWN
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=NOT_FOUND
READINESS=NOT_SUPPORTED
BLOCKER=GAP-C-006
OWNER_AGENT=C
NOTES=No executed loaded_km column. NOT_SUPPORTED until TMS records it. Do not substitute an NLO planned leg.
```

### NET_EMPTY_KM

```
KPI_ID=NET_EMPTY_KM
KPI_NAME=Empty kilometres
KPI_DOMAIN=NETWORK
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Executed empty kilometres. Not candidate deadhead.
NUMERATOR=NOT_FOUND
DENOMINATOR=1
INCLUSION_RULE=NOT_FOUND
EXCLUSION_RULE=match_candidates.road_deadhead_km is not executed empty km
TIME_BASIS=NOT_FOUND
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=NOT_FOUND
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=NOT_FOUND
EVENT_TIME=NOT_FOUND
PROCESSING_TIME=NOT_FOUND
DIMENSIONS=TENANT
TENANT_SCOPE=operating tenant of the execution
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=UNKNOWN
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=NOT_FOUND
READINESS=NOT_SUPPORTED
BLOCKER=GAP-C-006
OWNER_AGENT=C
NOTES=No executed empty_km column.
```

### NET_DEADHEAD_PCT

```
KPI_ID=NET_DEADHEAD_PCT
KPI_NAME=Deadhead percent
KPI_DOMAIN=NETWORK
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Executed deadhead kilometres divided by executed total kilometres. Candidate deadhead has no executed total.
NUMERATOR=NET_DEADHEAD_KM
DENOMINATOR=executed loaded plus executed empty, or another TMS total once GAP-C-006 exists
INCLUSION_RULE=NOT_FOUND
EXCLUSION_RULE=do not divide summed road_deadhead_km by itself or by a planned NLO distance
TIME_BASIS=NOT_FOUND
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=NOT_FOUND
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=NOT_FOUND
EVENT_TIME=NOT_FOUND
PROCESSING_TIME=NOT_FOUND
DIMENSIONS=TENANT
TENANT_SCOPE=operating tenant of the execution
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=UNKNOWN
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=empty denominator forbidden
CURRENT_IMPLEMENTATION=NOT_FOUND
READINESS=BLOCKED
BLOCKER=GAP-C-006
OWNER_AGENT=C
NOTES=BLOCKED because candidate road_deadhead_km exists and a false percent could be built from it. The percent is an executed TMS measure.
```

### NET_CAPACITY_UTILIZATION

```
KPI_ID=NET_CAPACITY_UTILIZATION
KPI_NAME=Capacity utilization
KPI_DOMAIN=NETWORK
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Score component CAPACITY_UTILIZATION inside score JSON. Not a top-level fact and not a versioned KPI.
NUMERATOR=UNKNOWN outside the score
DENOMINATOR=UNKNOWN
INCLUSION_RULE=score_components
EXCLUSION_RULE=do not treat the score weight as the business utilization contract
TIME_BASIS=search time
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-NLO-SEARCH
SOURCE_SERVICE=network-optimizer-service
SOURCE_FIELDS=score_components
EVENT_TIME=search time
PROCESSING_TIME=UNKNOWN
DIMENSIONS=TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=EVENT_HISTORY if JSON is retained
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=domain score component
READINESS=PARTIAL
BLOCKER=Not a canonical utilization fact; D scoring may change
OWNER_AGENT=D
NOTES=
```

### NET_BACKHAUL_OPPORTUNITIES

```
KPI_ID=NET_BACKHAUL_OPPORTUNITIES
KPI_NAME=Backhaul opportunities
KPI_DOMAIN=NETWORK
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Backhaul opportunities. NOT_FOUND in service code and migrations.
NUMERATOR=NOT_FOUND
DENOMINATOR=1
INCLUSION_RULE=NOT_FOUND
EXCLUSION_RULE=do not count load_opportunities or route plans as backhaul
TIME_BASIS=NOT_FOUND
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=NOT_FOUND
SOURCE_SERVICE=network-optimizer-service
SOURCE_FIELDS=NOT_FOUND
EVENT_TIME=NOT_FOUND
PROCESSING_TIME=NOT_FOUND
DIMENSIONS=TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=UNKNOWN
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=ADR-NET-023 is a design document only
READINESS=NOT_SUPPORTED
BLOCKER=GAP-D-003
OWNER_AGENT=D
NOTES=
```

### NET_BACKHAUL_ACCEPTED

```
KPI_ID=NET_BACKHAUL_ACCEPTED
KPI_NAME=Backhaul accepted
KPI_DOMAIN=NETWORK
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Accepted backhaul. route_plans status ACCEPTED is a different fact.
NUMERATOR=NOT_FOUND
DENOMINATOR=1
INCLUSION_RULE=NOT_FOUND
EXCLUSION_RULE=route plan ACCEPTED
TIME_BASIS=NOT_FOUND
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=NOT_FOUND
SOURCE_SERVICE=network-optimizer-service
SOURCE_FIELDS=NOT_FOUND
EVENT_TIME=NOT_FOUND
PROCESSING_TIME=NOT_FOUND
DIMENSIONS=TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=UNKNOWN
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=NOT_FOUND
READINESS=NOT_SUPPORTED
BLOCKER=GAP-D-003
OWNER_AGENT=D
NOTES=
```

### NET_BACKHAUL_CONVERSION

```
KPI_ID=NET_BACKHAUL_CONVERSION
KPI_NAME=Backhaul conversion
KPI_DOMAIN=NETWORK
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Accepted backhaul divided by backhaul opportunities.
NUMERATOR=NOT_FOUND
DENOMINATOR=NOT_FOUND
INCLUSION_RULE=NOT_FOUND
EXCLUSION_RULE=search-to-plan is not this rate
TIME_BASIS=NOT_FOUND
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=NOT_FOUND
SOURCE_SERVICE=network-optimizer-service
SOURCE_FIELDS=NOT_FOUND
EVENT_TIME=NOT_FOUND
PROCESSING_TIME=NOT_FOUND
DIMENSIONS=TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=UNKNOWN
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=NOT_FOUND
READINESS=NOT_SUPPORTED
BLOCKER=GAP-D-003
OWNER_AGENT=D
NOTES=
```

### NET_AVG_DEADHEAD_REDUCTION_KM

```
KPI_ID=NET_AVG_DEADHEAD_REDUCTION_KM
KPI_NAME=Average deadhead reduction kilometres
KPI_DOMAIN=NETWORK
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Mean deadhead saved versus a baseline on an accepted plan.
NUMERATOR=NOT_FOUND
DENOMINATOR=accepted plans with a baseline
INCLUSION_RULE=NOT_FOUND
EXCLUSION_RULE=candidate road_deadhead_km is not a reduction
TIME_BASIS=NOT_FOUND
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=NOT_FOUND
SOURCE_SERVICE=network-optimizer-service
SOURCE_FIELDS=NOT_FOUND
EVENT_TIME=NOT_FOUND
PROCESSING_TIME=NOT_FOUND
DIMENSIONS=TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=UNKNOWN
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=NOT_FOUND
READINESS=BLOCKED
BLOCKER=GAP-D-001
OWNER_AGENT=D
NOTES=BLOCKED because a deadhead field exists and could be misread as savings.
```

### NET_ROUTE_INCREASE_KM

```
KPI_ID=NET_ROUTE_INCREASE_KM
KPI_NAME=Route increase kilometres
KPI_DOMAIN=NETWORK
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=route_increase_km returned on the search candidate view. Not persisted on match_candidates. max_route_increase_km is a cap.
NUMERATOR=sum of route_increase_km on a live response
DENOMINATOR=1
INCLUSION_RULE=API field
EXCLUSION_RULE=policy cap; straight-line
TIME_BASIS=request time
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-NLO-SEARCH response, not the candidate table
SOURCE_SERVICE=network-optimizer-service
SOURCE_FIELDS=SearchCandidateView.route_increase_km
EVENT_TIME=request
PROCESSING_TIME=request
DIMENSIONS=TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=live response only
HISTORY_CLASS=UNKNOWN
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=null excluded
CURRENT_IMPLEMENTATION=search.go candidate view
READINESS=PARTIAL
BLOCKER=GAP-D-005 for history
OWNER_AGENT=D
NOTES=Live value only. Not a historical KPI.
```

### NET_SEARCH_TO_ACCEPT_CONVERSION

```
KPI_ID=NET_SEARCH_TO_ACCEPT_CONVERSION
KPI_NAME=Search to accept conversion
KPI_DOMAIN=NETWORK
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Accepted plans that came from a search, divided by searches. The link was not proven. Accepted plans exist (accepted_at).
NUMERATOR=NOT_FOUND as linked accepts
DENOMINATOR=search runs
INCLUSION_RULE=NOT_FOUND
EXCLUSION_RULE=do not divide all ACCEPTED plans by all search runs
TIME_BASIS=accepted_at
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-NLO-SEARCH and SRC-NLO-ROUTE-PLAN
SOURCE_SERVICE=network-optimizer-service
SOURCE_FIELDS=route_plans.accepted_at, next_load_search_runs
EVENT_TIME=accepted_at for the plan; link NOT_FOUND
PROCESSING_TIME=UNKNOWN
DIMENSIONS=TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY for the plan
LATE_EVENT_POLICY_CURRENT=accept replay keeps the first accepted_at
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=both sides exist; the join does not
READINESS=BLOCKED
BLOCKER=GAP-D-004
OWNER_AGENT=D
NOTES=
```

## Executive

Executive KPIs are pointers. They do not introduce a second formula.

### EXEC_SHIPMENT_VOLUME

```
KPI_ID=EXEC_SHIPMENT_VOLUME
KPI_NAME=Executive shipment volume
KPI_DOMAIN=EXECUTIVE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Pointer to OPS_SHIPMENTS_TOTAL.
NUMERATOR=OPS_SHIPMENTS_TOTAL
DENOMINATOR=1
INCLUSION_RULE=same
EXCLUSION_RULE=same
TIME_BASIS=created_at
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-SHIPMENT
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=same as OPS_SHIPMENTS_TOTAL
EVENT_TIME=created_at
PROCESSING_TIME=created_at
DIMENSIONS=TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=none as an executive API
READINESS=READY
BLOCKER=
OWNER_AGENT=E as a pointer; C owns the fact
NOTES=
```

### EXEC_OTIF

```
KPI_ID=EXEC_OTIF
KPI_NAME=Executive OTIF
KPI_DOMAIN=EXECUTIVE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Pointer to OPS_OTIF.
NUMERATOR=OPS_OTIF
DENOMINATOR=OPS_OTIF
INCLUSION_RULE=same
EXCLUSION_RULE=same
TIME_BASIS=same
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=same
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=same
EVENT_TIME=same
PROCESSING_TIME=same
DIMENSIONS=TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=same
CURRENT_IMPLEMENTATION=NOT_FOUND
READINESS=BLOCKED
BLOCKER=GAP-C-001
OWNER_AGENT=C
NOTES=
```

### EXEC_TOTAL_FREIGHT_COST

```
KPI_ID=EXEC_TOTAL_FREIGHT_COST
KPI_NAME=Executive total freight cost
KPI_DOMAIN=EXECUTIVE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Must point at one finance KPI. Planned, current actual, and final actual are different. No independent total.
NUMERATOR=the chosen finance KPI
DENOMINATOR=1
INCLUSION_RULE=per currency
EXCLUSION_RULE=mixed currency; do not add stages
TIME_BASIS=the chosen stage
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-FREIGHT-COST
SOURCE_SERVICE=freight-cost-service
SOURCE_FIELDS=planned_amount, current_actual_amount, final_actual_amount
EVENT_TIME=source_occurred_at
PROCESSING_TIME=recorded_at
DIMENSIONS=TENANT, CURRENCY
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=EVENT_HISTORY
LATE_EVENT_POLICY_CURRENT=supersede
CORRECTION_POLICY_CURRENT=supersede
NULL_POLICY=null stage is not zero
CURRENT_IMPLEMENTATION=cost summary
READINESS=PARTIAL
BLOCKER=Stage not chosen
OWNER_AGENT=E
NOTES=MIXED_CURRENCY_SAFE=NO. Frontend revenueTotal is not this KPI.
```

### EXEC_COST_VARIANCE

```
KPI_ID=EXEC_COST_VARIANCE
KPI_NAME=Executive cost variance
KPI_DOMAIN=EXECUTIVE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Pointer to FIN_COST_VARIANCE.
NUMERATOR=FIN_COST_VARIANCE
DENOMINATOR=1
INCLUSION_RULE=same
EXCLUSION_RULE=same
TIME_BASIS=same
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-FREIGHT-COST
SOURCE_SERVICE=freight-cost-service
SOURCE_FIELDS=same
EVENT_TIME=same
PROCESSING_TIME=same
DIMENSIONS=CURRENCY
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=EVENT_HISTORY
LATE_EVENT_POLICY_CURRENT=supersede
CORRECTION_POLICY_CURRENT=supersede
NULL_POLICY=same
CURRENT_IMPLEMENTATION=same
READINESS=PARTIAL
BLOCKER=Which actual
OWNER_AGENT=E
NOTES=MIXED_CURRENCY_SAFE=NO.
```

### EXEC_TENDER_SAVINGS

```
KPI_ID=EXEC_TENDER_SAVINGS
KPI_NAME=Executive tender savings
KPI_DOMAIN=EXECUTIVE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Pointer to PROC_TENDER_SAVINGS.
NUMERATOR=PROC_TENDER_SAVINGS
DENOMINATOR=1
INCLUSION_RULE=NOT_FOUND
EXCLUSION_RULE=same
TIME_BASIS=NOT_FOUND
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=NOT_FOUND
SOURCE_SERVICE=rfx-service
SOURCE_FIELDS=NOT_FOUND
EVENT_TIME=NOT_FOUND
PROCESSING_TIME=NOT_FOUND
DIMENSIONS=CURRENCY
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=UNKNOWN
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=NOT_FOUND
READINESS=BLOCKED
BLOCKER=GAP-RFX-001
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN
NOTES=MIXED_CURRENCY_SAFE=NO.
```

### EXEC_CARRIER_PERFORMANCE

```
KPI_ID=EXEC_CARRIER_PERFORMANCE
KPI_NAME=Executive carrier performance
KPI_DOMAIN=EXECUTIVE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Pointer to CAR_PERFORMANCE_SCORE. Not a blend of on-time rates.
NUMERATOR=CAR_PERFORMANCE_SCORE
DENOMINATOR=same
INCLUSION_RULE=NOT_FOUND
EXCLUSION_RULE=do not average partial rates and call the result the score
TIME_BASIS=UNKNOWN
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=NOT_FOUND
SOURCE_SERVICE=NOT_FOUND
SOURCE_FIELDS=NOT_FOUND
EVENT_TIME=NOT_FOUND
PROCESSING_TIME=NOT_FOUND
DIMENSIONS=CARRIER
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=UNKNOWN
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=NOT_FOUND
READINESS=BLOCKED
BLOCKER=GAP-E-001
OWNER_AGENT=E
NOTES=SCORING_POLICY_REQUIRED=YES
```

### EXEC_CRITICAL_EXCEPTION_RATE

```
KPI_ID=EXEC_CRITICAL_EXCEPTION_RATE
KPI_NAME=Executive critical exception rate
KPI_DOMAIN=EXECUTIVE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Pointer to CAR_CRITICAL_EXCEPTION_RATE, which is itself unresolved between p1 and risk critical.
NUMERATOR=that KPI
DENOMINATOR=that KPI
INCLUSION_RULE=same
EXCLUSION_RULE=same
TIME_BASIS=UNKNOWN
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-CT-RISK-EXCEPTION
SOURCE_SERVICE=control-tower-read-model-service
SOURCE_FIELDS=priority, risk_level
EVENT_TIME=UNKNOWN
PROCESSING_TIME=UNKNOWN
DIMENSIONS=TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=separate counts exist
READINESS=PARTIAL
BLOCKER=Same as CAR_CRITICAL_EXCEPTION_RATE
OWNER_AGENT=E
NOTES=
```

### EXEC_DELIVERY_REJECTION_RATE

```
KPI_ID=EXEC_DELIVERY_REJECTION_RATE
KPI_NAME=Executive delivery rejection rate
KPI_DOMAIN=EXECUTIVE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Pointer to OPS_DELIVERY_REJECTION_RATE.
NUMERATOR=OPS_DELIVERY_REJECTION_RATE
DENOMINATOR=same
INCLUSION_RULE=same
EXCLUSION_RULE=same
TIME_BASIS=attempt time
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-DISPOSITION
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=rejected_quantity
EVENT_TIME=attempt fact
PROCESSING_TIME=audit
DIMENSIONS=TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=EVENT_HISTORY for attempts
LATE_EVENT_POLICY_CURRENT=append-only
CORRECTION_POLICY_CURRENT=append-only
NULL_POLICY=absence is not zero
CURRENT_IMPLEMENTATION=disposition
READINESS=PARTIAL
BLOCKER=Fleet denominator
OWNER_AGENT=C
NOTES=
```

### EXEC_BILLED_AMOUNT

```
KPI_ID=EXEC_BILLED_AMOUNT
KPI_NAME=Executive billed amount
KPI_DOMAIN=EXECUTIVE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Pointer to FIN_BILLED_AMOUNT.
NUMERATOR=FIN_BILLED_AMOUNT
DENOMINATOR=1
INCLUSION_RULE=same
EXCLUSION_RULE=mixed currency
TIME_BASIS=same
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-FREIGHT-COST
SOURCE_SERVICE=freight-cost-service
SOURCE_FIELDS=billing_register_amount
EVENT_TIME=source_occurred_at
PROCESSING_TIME=recorded_at
DIMENSIONS=TENANT, CURRENCY
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=EVENT_HISTORY
LATE_EVENT_POLICY_CURRENT=supersede
CORRECTION_POLICY_CURRENT=supersede
NULL_POLICY=null means not billed
CURRENT_IMPLEMENTATION=ledger
READINESS=READY
BLOCKER=
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN
NOTES=MIXED_CURRENCY_SAFE=NO. FINANCIAL_FINALITY=billed.
```

### EXEC_OUTSTANDING_AMOUNT

```
KPI_ID=EXEC_OUTSTANDING_AMOUNT
KPI_NAME=Executive outstanding amount
KPI_DOMAIN=EXECUTIVE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Pointer to FIN_OUTSTANDING_AMOUNT.
NUMERATOR=FIN_OUTSTANDING_AMOUNT
DENOMINATOR=1
INCLUSION_RULE=same
EXCLUSION_RULE=mixed currency
TIME_BASIS=current
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-PAYMENT
SOURCE_SERVICE=payment-service
SOURCE_FIELDS=outstanding_amount
EVENT_TIME=current column
PROCESSING_TIME=obligation update
DIMENSIONS=TENANT, CURRENCY
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=current
HISTORY_CLASS=CURRENT_STATE_ONLY
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=in-place
NULL_POLICY=same
CURRENT_IMPLEMENTATION=payment obligations
READINESS=READY
BLOCKER=
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN
NOTES=MIXED_CURRENCY_SAFE=NO.
```

### EXEC_DEADHEAD_PCT

```
KPI_ID=EXEC_DEADHEAD_PCT
KPI_NAME=Executive deadhead percent
KPI_DOMAIN=EXECUTIVE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Pointer to NET_DEADHEAD_PCT. Executed TMS percent. Not an NLO candidate sum.
NUMERATOR=NET_DEADHEAD_PCT
DENOMINATOR=same
INCLUSION_RULE=NOT_FOUND
EXCLUSION_RULE=same
TIME_BASIS=NOT_FOUND
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=NOT_FOUND
SOURCE_SERVICE=shipment-service
SOURCE_FIELDS=NOT_FOUND
EVENT_TIME=NOT_FOUND
PROCESSING_TIME=NOT_FOUND
DIMENSIONS=TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=UNKNOWN
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=NOT_FOUND
READINESS=BLOCKED
BLOCKER=GAP-C-006
OWNER_AGENT=C
NOTES=
```

### EXEC_CAPACITY_UTILIZATION

```
KPI_ID=EXEC_CAPACITY_UTILIZATION
KPI_NAME=Executive capacity utilization
KPI_DOMAIN=EXECUTIVE
KPI_VERSION_PROPOSED=1
BUSINESS_DEFINITION=Pointer to NET_CAPACITY_UTILIZATION.
NUMERATOR=NET_CAPACITY_UTILIZATION
DENOMINATOR=same
INCLUSION_RULE=score JSON only
EXCLUSION_RULE=same
TIME_BASIS=search time
TIME_WINDOW=SEE_DEFAULT
SOURCE_OF_TRUTH=SRC-NLO-SEARCH
SOURCE_SERVICE=network-optimizer-service
SOURCE_FIELDS=score_components
EVENT_TIME=search time
PROCESSING_TIME=UNKNOWN
DIMENSIONS=TENANT
TENANT_SCOPE=SEE_DEFAULT
FRESHNESS=SEE_DEFAULT
HISTORY_CLASS=EVENT_HISTORY if retained
LATE_EVENT_POLICY_CURRENT=SEE_DEFAULT
CORRECTION_POLICY_CURRENT=SEE_DEFAULT
NULL_POLICY=SEE_DEFAULT
CURRENT_IMPLEMENTATION=score component
READINESS=PARTIAL
BLOCKER=Same as NET_CAPACITY_UTILIZATION
OWNER_AGENT=D
NOTES=
```

## Summary

| KPI_ID | DOMAIN | READINESS | HISTORY_CLASS | SOURCE_OWNER | PRIMARY_BLOCKER |
| --- | --- | --- | --- | --- | --- |
| OPS_SHIPMENTS_TOTAL | OPERATIONS | READY | CURRENT_STATE_ONLY | C | |
| OPS_SHIPMENTS_ACTIVE | OPERATIONS | PARTIAL | CURRENT_STATE_ONLY | E | active definition |
| OPS_ON_TIME_PICKUP | OPERATIONS | PARTIAL | CURRENT_STATE_ONLY | C | grace and occurred_at |
| OPS_ON_TIME_PICKUP_RATE | OPERATIONS | PARTIAL | CURRENT_STATE_ONLY | E | denominator |
| OPS_LATE_PICKUP | OPERATIONS | PARTIAL | CURRENT_STATE_ONLY | C | completed versus in-flight |
| OPS_AVG_PICKUP_DELAY_MIN | OPERATIONS | PARTIAL | CURRENT_STATE_ONLY | C | early pickup policy |
| OPS_ON_TIME_DELIVERY | OPERATIONS | PARTIAL | CURRENT_STATE_ONLY | C | exclude ReasonOnSchedule |
| OPS_ON_TIME_DELIVERY_RATE | OPERATIONS | PARTIAL | CURRENT_STATE_ONLY | E | denominator |
| OPS_LATE_DELIVERY | OPERATIONS | PARTIAL | CURRENT_STATE_ONLY | C | completed versus in-flight |
| OPS_AVG_DELIVERY_DELAY_MIN | OPERATIONS | PARTIAL | CURRENT_STATE_ONLY | C | early delivery policy |
| OPS_OTIF | OPERATIONS | BLOCKED | CURRENT_STATE_ONLY | C | GAP-C-001 |
| OPS_DWELL_PICKUP_MIN | OPERATIONS | PARTIAL | CURRENT_STATE_ONLY | E | interval |
| OPS_DWELL_DELIVERY_MIN | OPERATIONS | PARTIAL | CURRENT_STATE_ONLY | E | interval |
| OPS_TRACKING_COVERAGE | OPERATIONS | PARTIAL | EVENT_HISTORY | C | coverage definition |
| OPS_POD_COMPLETENESS | OPERATIONS | BLOCKED | CURRENT_STATE_ONLY | C | GAP-C-002 |
| OPS_DELIVERY_REJECTION_RATE | OPERATIONS | PARTIAL | EVENT_HISTORY | C | fleet denominator |
| OPS_PARTIAL_REJECTION_RATE | OPERATIONS | PARTIAL | EVENT_HISTORY | C | denominator |
| OPS_RETURN_CASES | OPERATIONS | READY | CURRENT_STATE_ONLY | C | |
| OPS_REDIRECT_CASES | OPERATIONS | READY | CURRENT_STATE_ONLY | C | |
| OPS_EXCEPTION_RATE | OPERATIONS | PARTIAL | CURRENT_STATE_ONLY | C | one population |
| OPS_P1_EXCEPTION_RATE | OPERATIONS | PARTIAL | CURRENT_STATE_ONLY | E | denominator |
| OPS_P2_EXCEPTION_RATE | OPERATIONS | PARTIAL | CURRENT_STATE_ONLY | E | denominator |
| OPS_SLA_BREACH_RATE | OPERATIONS | PARTIAL | CURRENT_STATE_ONLY | E | two SLAs |
| OPS_SHIPMENT_CYCLE_TIME | OPERATIONS | PARTIAL | STATUS_HISTORY | E | start and end |
| OPS_DOCUMENT_COMPLETENESS | OPERATIONS | BLOCKED | CURRENT_STATE_ONLY | B | GAP-B-001 |
| PROC_RFX_CREATED | PROCUREMENT | READY | CURRENT_STATE_ONLY | UNASSIGNED | |
| PROC_RFX_PUBLISHED | PROCUREMENT | PARTIAL | EVENT_HISTORY | UNASSIGNED | no published_at |
| PROC_CARRIERS_INVITED | PROCUREMENT | READY | CURRENT_STATE_ONLY | UNASSIGNED | |
| PROC_CARRIERS_PARTICIPATED | PROCUREMENT | PARTIAL | CURRENT_STATE_ONLY | UNASSIGNED | participation act |
| PROC_PARTICIPATION_RATE | PROCUREMENT | PARTIAL | CURRENT_STATE_ONLY | UNASSIGNED | numerator |
| PROC_BIDS_RECEIVED | PROCUREMENT | PARTIAL | CURRENT_STATE_ONLY | UNASSIGNED | two bid paths |
| PROC_BIDS_PER_RFX | PROCUREMENT | PARTIAL | CURRENT_STATE_ONLY | UNASSIGNED | bid identity |
| PROC_FIRST_BID_TIME | PROCUREMENT | PARTIAL | EVENT_HISTORY | UNASSIGNED | publish time |
| PROC_TIME_TO_AWARD | PROCUREMENT | PARTIAL | CURRENT_STATE_ONLY | UNASSIGNED | start anchor |
| PROC_AWARD_RATE | PROCUREMENT | PARTIAL | CURRENT_STATE_ONLY | UNASSIGNED | denominator |
| PROC_WINNING_BID_VALUE | PROCUREMENT | PARTIAL | CURRENT_STATE_ONLY | UNASSIGNED | partial award grain |
| PROC_TENDER_SAVINGS | PROCUREMENT | BLOCKED | UNKNOWN | UNASSIGNED | GAP-RFX-001 |
| PROC_TENDER_SAVINGS_PCT | PROCUREMENT | BLOCKED | UNKNOWN | UNASSIGNED | GAP-RFX-001 |
| PROC_CONTRACT_VS_SPOT_SHARE | PROCUREMENT | PARTIAL | CURRENT_STATE_ONLY | UNASSIGNED | taxonomy |
| PROC_LANE_COMPETITION | PROCUREMENT | PARTIAL | CURRENT_STATE_ONLY | UNASSIGNED | lane identity |
| PROC_CARRIER_RESPONSE_RATE | PROCUREMENT | PARTIAL | CURRENT_STATE_ONLY | UNASSIGNED | response versus bid |
| CAR_SHIPMENTS_ASSIGNED | CARRIER | PARTIAL | STATUS_HISTORY | C | status versus column |
| CAR_SHIPMENTS_COMPLETED | CARRIER | PARTIAL | CURRENT_STATE_ONLY | C | completion set |
| CAR_ACCEPTANCE_RATE | CARRIER | PARTIAL | STATUS_HISTORY | C | partial history |
| CAR_REJECTION_RATE | CARRIER | BLOCKED | UNKNOWN | C | GAP-C-005 |
| CAR_CANCELLATION_RATE | CARRIER | PARTIAL | STATUS_HISTORY | C | actor |
| CAR_ON_TIME_PICKUP_RATE | CARRIER | PARTIAL | CURRENT_STATE_ONLY | C | same as operations |
| CAR_ON_TIME_DELIVERY_RATE | CARRIER | PARTIAL | CURRENT_STATE_ONLY | C | denominator |
| CAR_OTIF | CARRIER | BLOCKED | CURRENT_STATE_ONLY | C | GAP-C-001 |
| CAR_AVG_DELAY_MIN | CARRIER | PARTIAL | CURRENT_STATE_ONLY | E | pickup versus delivery |
| CAR_DELIVERY_REJECTION_RATE | CARRIER | PARTIAL | EVENT_HISTORY | C | denominator |
| CAR_CARGO_ISSUE_RATE | CARRIER | PARTIAL | EVENT_HISTORY | C | taxonomy |
| CAR_DOCUMENT_COMPLETENESS | CARRIER | BLOCKED | CURRENT_STATE_ONLY | B | GAP-B-001 |
| CAR_POD_COMPLETENESS | CARRIER | BLOCKED | CURRENT_STATE_ONLY | C | GAP-C-002 |
| CAR_EXCEPTION_RATE | CARRIER | PARTIAL | CURRENT_STATE_ONLY | C | population |
| CAR_CRITICAL_EXCEPTION_RATE | CARRIER | PARTIAL | CURRENT_STATE_ONLY | E | p1 versus risk |
| CAR_PERFORMANCE_SCORE | CARRIER | BLOCKED | UNKNOWN | E | GAP-E-001 |
| FIN_PLANNED_FREIGHT_COST | FINANCE | READY | EVENT_HISTORY | UNASSIGNED | |
| FIN_ESTIMATED_FREIGHT_COST | FINANCE | NOT_SUPPORTED | UNKNOWN | UNASSIGNED | no estimated stage |
| FIN_ACTUAL_FREIGHT_COST | FINANCE | PARTIAL | EVENT_HISTORY | E | current versus final |
| FIN_COST_VARIANCE | FINANCE | PARTIAL | EVENT_HISTORY | E | which actual |
| FIN_COST_VARIANCE_PCT | FINANCE | PARTIAL | EVENT_HISTORY | E | which actual |
| FIN_COST_PER_SHIPMENT | FINANCE | PARTIAL | EVENT_HISTORY | E | grain |
| FIN_COST_PER_KM | FINANCE | BLOCKED | UNKNOWN | C | GAP-C-003 |
| FIN_COST_PER_PALLET | FINANCE | PARTIAL | CURRENT_STATE_ONLY | C | null pallets |
| FIN_COST_PER_TON | FINANCE | BLOCKED | CURRENT_STATE_ONLY | C | GAP-C-004 |
| FIN_BASE_FREIGHT | FINANCE | READY | CURRENT_STATE_ONLY | UNASSIGNED | |
| FIN_SURCHARGES | FINANCE | PARTIAL | CURRENT_STATE_ONLY | UNASSIGNED | surcharge population |
| FIN_PENALTIES | FINANCE | READY | CURRENT_STATE_ONLY | UNASSIGNED | |
| FIN_BILLED_AMOUNT | FINANCE | READY | EVENT_HISTORY | UNASSIGNED | |
| FIN_PAID_AMOUNT | FINANCE | READY | CURRENT_STATE_ONLY | UNASSIGNED | |
| FIN_OUTSTANDING_AMOUNT | FINANCE | READY | CURRENT_STATE_ONLY | UNASSIGNED | |
| FIN_PAYMENT_AGING_DAYS | FINANCE | PARTIAL | CURRENT_STATE_ONLY | E | buckets |
| FIN_DAYS_TO_INVOICE | FINANCE | PARTIAL | CURRENT_STATE_ONLY | E | start event |
| FIN_DAYS_TO_PAYMENT | FINANCE | PARTIAL | CURRENT_STATE_ONLY | E | anchor |
| FIN_FINANCIALLY_CLOSED_AMOUNT | FINANCE | PARTIAL | CURRENT_STATE_ONLY | E | close marker |
| DOC_DOCUMENTS_CREATED | DOCUMENT | READY | CURRENT_STATE_ONLY | B | |
| DOC_READY_FOR_SIGNING | DOCUMENT | READY | CURRENT_STATE_ONLY | B | |
| DOC_SIGNED_STATUS_COUNT | DOCUMENT | READY | CURRENT_STATE_ONLY | B | status only, not trust |
| DOC_VERIFIED_SIGNATURE_COUNT | DOCUMENT | BLOCKED | EVENT_HISTORY | B | GAP-B-003 |
| DOC_QUALIFIED_TRUST_COUNT | DOCUMENT | BLOCKED | EVENT_HISTORY | B | GAP-B-002 |
| DOC_SIGNATURE_PENDING_COUNT | DOCUMENT | PARTIAL | EVENT_HISTORY | B | which pending |
| DOC_DOCUMENT_COMPLETENESS | DOCUMENT | BLOCKED | UNKNOWN | B | GAP-B-001 |
| DOC_TIME_TO_SIGNATURE | DOCUMENT | PARTIAL | CURRENT_STATE_ONLY | B | start time |
| NET_CANDIDATES_DISCOVERED | NETWORK | PARTIAL | EVENT_HISTORY | D | stage |
| NET_CANDIDATES_PREFILTERED | NETWORK | NOT_SUPPORTED | UNKNOWN | D | no fact |
| NET_CANDIDATES_ROUTED | NETWORK | PARTIAL | EVENT_HISTORY | D | persistence coverage |
| NET_CANDIDATES_FEASIBLE | NETWORK | PARTIAL | EVENT_HISTORY | D | eligibility vocabulary |
| NET_CANDIDATES_RANKED | NETWORK | PARTIAL | EVENT_HISTORY | D | rank contract |
| NET_SEARCH_DURATION | NETWORK | NOT_SUPPORTED | UNKNOWN | D | observability |
| NET_PROVIDER_CALLS | NETWORK | NOT_SUPPORTED | UNKNOWN | D | no call fact |
| NET_DEADHEAD_KM | NETWORK | BLOCKED | UNKNOWN | C | GAP-C-006 |
| NET_LOADED_KM | NETWORK | NOT_SUPPORTED | UNKNOWN | C | GAP-C-006 |
| NET_EMPTY_KM | NETWORK | NOT_SUPPORTED | UNKNOWN | C | GAP-C-006 |
| NET_DEADHEAD_PCT | NETWORK | BLOCKED | UNKNOWN | C | GAP-C-006 |
| NET_CAPACITY_UTILIZATION | NETWORK | PARTIAL | EVENT_HISTORY | D | score JSON only |
| NET_BACKHAUL_OPPORTUNITIES | NETWORK | NOT_SUPPORTED | UNKNOWN | D | GAP-D-003 |
| NET_BACKHAUL_ACCEPTED | NETWORK | NOT_SUPPORTED | UNKNOWN | D | GAP-D-003 |
| NET_BACKHAUL_CONVERSION | NETWORK | NOT_SUPPORTED | UNKNOWN | D | GAP-D-003 |
| NET_AVG_DEADHEAD_REDUCTION_KM | NETWORK | BLOCKED | UNKNOWN | D | GAP-D-001 |
| NET_ROUTE_INCREASE_KM | NETWORK | PARTIAL | UNKNOWN | D | GAP-D-005 |
| NET_SEARCH_TO_ACCEPT_CONVERSION | NETWORK | BLOCKED | CURRENT_STATE_ONLY | D | GAP-D-004 |
| EXEC_SHIPMENT_VOLUME | EXECUTIVE | READY | CURRENT_STATE_ONLY | E | pointer to OPS_SHIPMENTS_TOTAL |
| EXEC_OTIF | EXECUTIVE | BLOCKED | CURRENT_STATE_ONLY | C | GAP-C-001 |
| EXEC_TOTAL_FREIGHT_COST | EXECUTIVE | PARTIAL | EVENT_HISTORY | E | cost stage |
| EXEC_COST_VARIANCE | EXECUTIVE | PARTIAL | EVENT_HISTORY | E | which actual |
| EXEC_TENDER_SAVINGS | EXECUTIVE | BLOCKED | UNKNOWN | UNASSIGNED | GAP-RFX-001 |
| EXEC_CARRIER_PERFORMANCE | EXECUTIVE | BLOCKED | UNKNOWN | E | GAP-E-001 |
| EXEC_CRITICAL_EXCEPTION_RATE | EXECUTIVE | PARTIAL | CURRENT_STATE_ONLY | E | p1 versus risk |
| EXEC_DELIVERY_REJECTION_RATE | EXECUTIVE | PARTIAL | EVENT_HISTORY | C | denominator |
| EXEC_BILLED_AMOUNT | EXECUTIVE | READY | EVENT_HISTORY | UNASSIGNED | |
| EXEC_OUTSTANDING_AMOUNT | EXECUTIVE | READY | CURRENT_STATE_ONLY | UNASSIGNED | |
| EXEC_DEADHEAD_PCT | EXECUTIVE | BLOCKED | UNKNOWN | C | GAP-C-006 |
| EXEC_CAPACITY_UTILIZATION | EXECUTIVE | PARTIAL | EVENT_HISTORY | D | score JSON |

## Totals

```
KPI_TOTAL=114
KPI_READY=17
KPI_PARTIAL=65
KPI_BLOCKED=23
KPI_NOT_SUPPORTED=9
```

By domain:

| Domain | Count | READY | PARTIAL | BLOCKED | NOT_SUPPORTED | Score |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| OPERATIONS | 25 | 3 | 19 | 3 | 0 | 0.500 |
| PROCUREMENT | 16 | 2 | 12 | 2 | 0 | 0.500 |
| CARRIER | 16 | 0 | 11 | 5 | 0 | 0.344 |
| FINANCE | 19 | 6 | 10 | 2 | 1 | 0.579 |
| DOCUMENT | 8 | 3 | 2 | 3 | 0 | 0.500 |
| NETWORK | 18 | 0 | 6 | 4 | 8 | 0.167 |
| EXECUTIVE | 12 | 3 | 5 | 4 | 0 | 0.458 |
| ALL | 114 | 17 | 65 | 23 | 9 | 0.434 |

```
OPERATIONS_ANALYTICS_READINESS=0.500
PROCUREMENT_ANALYTICS_READINESS=0.500
CARRIER_ANALYTICS_READINESS=0.344
FINANCE_ANALYTICS_READINESS=0.579
DOCUMENT_ANALYTICS_READINESS=0.500
NETWORK_ANALYTICS_READINESS=0.167
EXECUTIVE_ANALYTICS_READINESS=0.458
OVERALL_CATALOG_READINESS=0.434
READINESS_METHOD=READY=1, PARTIAL=0.5, BLOCKED=0, NOT_SUPPORTED=0; score = weighted sum / count. 49.5 / 114 = 0.434.
```

Check: 3+19+3=25. 2+12+2=16. 0+11+5=16. 6+10+2+1=19. 3+2+3=8. 0+6+4+8=18. 3+5+4=12. 25+16+16+19+8+18+12=114. 17+65+23+9=114. R1 moved FIN_SURCHARGES READY to PARTIAL, DOC_VERIFIED_SIGNATURE_COUNT PARTIAL to BLOCKED, and NET_DEADHEAD_KM PARTIAL to BLOCKED.

