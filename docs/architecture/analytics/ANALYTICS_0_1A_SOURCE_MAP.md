# Analytics-0.1A source map

Classification is from code and migrations on `origin/main` `c505c84bd432d7bd039b97968506a375522c165c` (R2). Original 0.1A base: `9ec52621d272b9ca24e5a57f73448acfd0ebef3c`. Runtime was **NOT_RUN**. NLO-0.5B2 is in this main.

History class is one of `CURRENT_STATE_ONLY`, `EVENT_HISTORY`, `STATUS_HISTORY`, `SNAPSHOT_HISTORY`, `FULL_REBUILDABLE_HISTORY`, `UNKNOWN`.

`FULL_REBUILDABLE_HISTORY` was **NOT_FOUND** as a single store for any domain below. Some domains can be partially rebuilt by joining an event or status table with the current row.

## Shipment and TMS

### SRC-SHIPMENT

```
SOURCE_ID=SRC-SHIPMENT
OWNER_AGENT=C
SERVICE=shipment-service
TABLE_OR_OBJECT=transport.shipments
EVENT_OR_API=Shipment aggregate; HTTP shipment APIs
KEY_FIELDS=id, tenant_id, shipment_number, transport_order_id, shipper_company_id, consignee_company_id, carrier_company_id, driver_id, vehicle_id, origin_location_id, destination_location_id, cargo_id, transport_mode, status, version
TIME_FIELDS=planned_pickup_at, planned_delivery_at, actual_pickup_at, actual_delivery_at, created_at, updated_at, deleted_at
TENANT_SCOPE=tenant_id on every repository read (shipment_repository.go)
HISTORY_CLASS=CURRENT_STATE_ONLY
MUTABILITY=status and actual timestamps update in place; actual_* first write wins via COALESCE
REBUILDABLE=NO for past status durations from this row alone
CURRENT_CONSUMERS=api-gateway Control Tower summary, shipment APIs, projections
NOTES=Status check constraint in 000003 includes CREATED through FINANCIALLY_CLOSED and CANCELLED. carrier_company_id is nullable. actual_pickup_at is set when status becomes LOADED; actual_delivery_at when status becomes DELIVERED. TMS path uses command occurred_at (client-required, stored UTC) in transitionShipment. Manual UpdateStatus uses ActualTime. FOUND in shipment_service.go and transport_execution_command_repository.go.
```

### SRC-SHIPMENT-STATUS-HISTORY

```
SOURCE_ID=SRC-SHIPMENT-STATUS-HISTORY
OWNER_AGENT=C
SERVICE=shipment-service
TABLE_OR_OBJECT=transport.shipment_status_history
EVENT_OR_API=insertStatusHistoryAndOutbox
KEY_FIELDS=shipment_id, tenant_id, from_status, to_status, shipment_version
TIME_FIELDS=occurred_at, recorded_at
TENANT_SCOPE=tenant_id
HISTORY_CLASS=STATUS_HISTORY
MUTABILITY=append
REBUILDABLE=PARTIAL. Ordered versions support duration math. status_history.go StatusHistoryIsComplete warns SHIPMENT_STATUS_HISTORY_PARTIAL when the first row is not a null from_status.
CURRENT_CONSUMERS=shipment service writes; not the Control Tower on-time KPI
NOTES=000012. A current status does not by itself prove how long the previous status lasted.
```

### SRC-SHIPMENT-OUTBOX

```
SOURCE_ID=SRC-SHIPMENT-OUTBOX
OWNER_AGENT=C
SERVICE=shipment-service
TABLE_OR_OBJECT=transport.shipment_event_outbox
EVENT_OR_API=shipment and execution events for Kafka
KEY_FIELDS=aggregate id, event type, payload
TIME_FIELDS=UNKNOWN in this inventory beyond outbox recording
TENANT_SCOPE=FOUND with shipment writes
HISTORY_CLASS=EVENT_HISTORY
MUTABILITY=append / dispatch
REBUILDABLE=PARTIAL. Not a complete execution-stop history by itself.
CURRENT_CONSUMERS=downstream projections including Control Tower
NOTES=000014. FK to status history was relaxed in 000031 so execution events can be written without a history row.
```

### SRC-EXECUTION-STOP

```
SOURCE_ID=SRC-EXECUTION-STOP
OWNER_AGENT=C
SERVICE=shipment-service
TABLE_OR_OBJECT=transport.transport_execution_stops
EVENT_OR_API=TMS execution commands
KEY_FIELDS=operating_tenant_id, execution id, stop status (PLANNED, ARRIVED, SERVICE_STARTED, COMPLETED, CANCELLED, SKIPPED), action linkage
TIME_FIELDS=planned_arrival, planned_departure, arrived_at, service_started_at, completed_at, service_duration_seconds (planned hint)
TENANT_SCOPE=operating_tenant_id; shipment_tenant_id on related participant/action rows
HISTORY_CLASS=CURRENT_STATE_ONLY
MUTABILITY=current row; terminal stops immutable (000089 triggers, ADR-TMS-004)
REBUILDABLE=NO as a versioned stop history. Command audit is separate.
CURRENT_CONSUMERS=driver stop tasks, Control Tower execution_stop_projection (000092)
NOTES=Column names are arrived_at and completed_at. NOT_FOUND: arrival_at, service_completed_at, dwell_seconds. ACTION_LEVEL_SHIPMENT_ID_IN_MAIN=YES. DriverStopActionFact.shipmentId is server-derived from transport.transport_execution_actions.shipment_id and is required. MULTI_SHIPMENT_STOP_CONTRACT_IN_MAIN=YES: stop.shipmentId may be null; action.shipmentId identifies the shipment. DRIVER_215_CHANGES_KPI_READINESS=NO. The driver-mobile stops page is a consumer, not a history store.
```

### SRC-EXECUTION-COMMAND-AUDIT

```
SOURCE_ID=SRC-EXECUTION-COMMAND-AUDIT
OWNER_AGENT=C
SERVICE=shipment-service
TABLE_OR_OBJECT=transport.transport_execution_commands and transport.transport_execution_command_audit
EVENT_OR_API=execution commands
KEY_FIELDS=operating_tenant_id, idempotency_key, actor_kind
TIME_FIELDS=occurred_at
TENANT_SCOPE=operating_tenant_id
HISTORY_CLASS=EVENT_HISTORY
MUTABILITY=idempotent insert; replay returns the same result
REBUILDABLE=PARTIAL
CURRENT_CONSUMERS=TMS execution
NOTES=occurred_at is required on the command (transport_execution_commands.go). It is actor-supplied and then stored UTC. It is the business event time the domain accepts. It is not server clock and not created_at.
```

### SRC-DISPOSITION

```
SOURCE_ID=SRC-DISPOSITION
OWNER_AGENT=C
SERVICE=shipment-service
TABLE_OR_OBJECT=transport.delivery_disposition_cases, delivery_attempt_facts, delivery_disposition_audit, delivery_disposition_commands
EVENT_OR_API=RecordDeliveryDisposition
KEY_FIELDS=disposition_type (RETURN_TO_ORIGIN, REDIRECT, HOLD_PENDING_DISPOSITION), status, reason_code, attempted_quantity, accepted_quantity, rejected_quantity, pending_quantity, uom
TIME_FIELDS=case timestamps; audit and attempt facts append
TENANT_SCOPE=operating_tenant_id and shipment_tenant_id
HISTORY_CLASS=CURRENT_STATE_ONLY for the case; EVENT_HISTORY for attempt facts and audit
MUTABILITY=case status changes; quantity facts protected by append-only triggers (000093)
REBUILDABLE=PARTIAL
CURRENT_CONSUMERS=Control Tower delivery_disposition_projection
NOTES=uom check is PALLET. Zero rejection does not open a case.
```

### SRC-CARGO

```
SOURCE_ID=SRC-CARGO
OWNER_AGENT=C
SERVICE=transport-order-service writes cargo; shipment references cargo_id
TABLE_OR_OBJECT=transport.cargoes, transport.cargo_items, transport.shipment_cargo_execution_evidence
EVENT_OR_API=cargo APIs; execution evidence ledger
KEY_FIELDS=cargo_type, gross_weight, net_weight, volume, pallet_count (000077), cargo_items.quantity, cargo_items.unit
TIME_FIELDS=created_at, updated_at; evidence is append-only states PLANNED, PICKED_UP, CONFIRMED_ONBOARD, UNLOADED
TENANT_SCOPE=tenant_id
HISTORY_CLASS=CURRENT_STATE_ONLY for cargoes; EVENT_HISTORY for shipment_cargo_execution_evidence
MUTABILITY=cargo row updates; pallet_count may be null
REBUILDABLE=NO for weight history
CURRENT_CONSUMERS=disposition onboard quantity reads pallet_count; NLO compatibility
NOTES=NOT_FOUND: weight unit column. Cost per ton is blocked by GAP-C-004.
```

### SRC-TRACKING

```
SOURCE_ID=SRC-TRACKING
OWNER_AGENT=C
SERVICE=tracking-service
TABLE_OR_OBJECT=tracking.location_event, shipment_tracking_state, tracking_state_transition, shipment_tracking_binding, eta_observation, shipment_eta_state, execution_stop_eta_state, execution_tracking_state, execution_event_inbox
EVENT_OR_API=telemetry ingest; execution milestone consumption (shipment.route_stop.arrived and related)
KEY_FIELDS=dedup_key, source_type, latitude, longitude, target_type
TIME_FIELDS=recorded_at, received_at, estimated_arrival_at
TENANT_SCOPE=tenant_id or operating_tenant_id
HISTORY_CLASS=EVENT_HISTORY for location_event, eta_observation, tracking_state_transition; CURRENT_STATE_ONLY for *_state tables
MUTABILITY=events append with ON CONFLICT DO NOTHING; state tables update
REBUILDABLE=PARTIAL for ping history
CURRENT_CONSUMERS=ETA and Control Tower execution projection
NOTES=Position pings are not the same fact as stop arrived_at. delivery_delay_seconds and delivery_lag_seconds exist on tracking state and are computed tracking signals, not the shipment actual_delivery_at contract.
```

## Control Tower

### SRC-CT-GATEWAY-SUMMARY

```
SOURCE_ID=SRC-CT-GATEWAY-SUMMARY
OWNER_AGENT=E for any future analytics reuse; current implementation lives in api-gateway Control Tower (operational, not an analytics service)
SERVICE=api-gateway
TABLE_OR_OBJECT=in-memory KPI over fetched shipment rows
EVENT_OR_API=GET /api/v1/control-tower/summary
KEY_FIELDS=kpi.active, kpi.onTime, kpi.atRisk, kpi.delayed, kpi.critical, kpi.awaitingDocuments, kpi.readyForBilling, exceptionKpi, riskKpi, dataFreshness
TIME_FIELDS=generatedAt (request time); SLA uses planned and actual pickup/delivery and LastUpdatedAt
TENANT_SCOPE=JWT tenant_id via gateway auth; client X-Tenant-ID stripped (middleware/auth.go)
HISTORY_CLASS=CURRENT_STATE_ONLY
MUTABILITY=computed per request
REBUILDABLE=NO. Not a stored aggregate.
CURRENT_CONSUMERS=apps/web-admin/pages/control-tower/index.vue via useControlTower.ts summary mode
NOTES=CalculateKPI in filters.go counts only rows that pass filters and IsActiveShipmentStatus. Active means not CANCELLED and not FINANCIALLY_CLOSED (sla.go), so delivered and billing statuses are inside "active". On-time is sla.Compute, which can return ON_TIME with ReasonOnSchedule when planned dates exist and no higher-priority problem was detected. That includes shipments that are not yet delivered. packages/openapi has no Control Tower paths (NOT_FOUND).
```

### SRC-CT-STATUS-PROJECTION

```
SOURCE_ID=SRC-CT-STATUS-PROJECTION
OWNER_AGENT=operational read model consumed by Control Tower
SERVICE=control-tower-read-model-service
TABLE_OR_OBJECT=control_tower.shipment_status_projection
EVENT_OR_API=GET /internal/v1/control-tower/status-summary
KEY_FIELDS=tenant_id, current_status, previous_status, complete, version
TIME_FIELDS=updated_at; consumer freshness lastRecordReceivedAt, lastProjectionAppliedAt
TENANT_SCOPE=resolveVerifiedTenant requires X-Tenant-ID set by the gateway from the JWT tenant
HISTORY_CLASS=SNAPSHOT_HISTORY
MUTABILITY=upsert of the latest projection; inbox records duplicate/stale/gap outcomes (000015)
REBUILDABLE=PARTIAL via projection rebuild tooling, not via a public history API
CURRENT_CONSUMERS=gateway status summary merge
NOTES=Counts by current_status. Does not compute on-time. NOT_FOUND: a field named watermark. Freshness is a consumer snapshot, not a business event watermark.
```

### SRC-CT-RISK-EXCEPTION

```
SOURCE_ID=SRC-CT-RISK-EXCEPTION
OWNER_AGENT=operational Control Tower
SERVICE=control-tower-read-model-service and api-gateway risk evaluator
TABLE_OR_OBJECT=control_tower.shipment_risk, shipment_risk_assessment, shipment_risk_signal, critical_event_workflow, critical_event_action
EVENT_OR_API=GET risks/kpi, cases/kpi, work-items, workload, exception calculations on summary
KEY_FIELDS=risk_level, predicted_exception_type, workflow priority p1 p2 p3 p4, SLA due timestamps
TIME_FIELDS=acknowledge_due_at, assignment_due_at, resolution_due_at, breach timestamps
TENANT_SCOPE=tenant_id
HISTORY_CLASS=CURRENT_STATE_ONLY for workflow and current risk row; EVENT_HISTORY for actions, assessments, signals
MUTABILITY=workflow updates; actions append
REBUILDABLE=PARTIAL
CURRENT_CONSUMERS=Control Tower summary and operator workspace
NOTES=This SLA is exception-handling SLA. It is a different measure from shipment pickup/delivery lateness. Do not merge them into one OPS_SLA_BREACH_RATE without an explicit definition.
```

### SRC-CT-FRONTEND

```
SOURCE_ID=SRC-CT-FRONTEND
OWNER_AGENT=presentation
SERVICE=apps/web-admin
TABLE_OR_OBJECT=useControlTower.ts, utils/controlTowerLogic.ts, utils/controlTowerDemoData.ts
EVENT_OR_API=browser computation
KEY_FIELDS=kpi cards, transportFunnel, tenderFunnel, documentsSummary, billingSummary, riskAlerts, recentActivity, revenueTotal
TIME_FIELDS=client clocks in fallback SLA
TENANT_SCOPE=summary calls rely on Bearer JWT (skipTenant on summary). Fallback lists send tenant from the store. DEV_TENANT_FALLBACK exists in the composable.
HISTORY_CLASS=CURRENT_STATE_ONLY
MUTABILITY=derived in the browser
REBUILDABLE=NO
CURRENT_CONSUMERS=index.vue uses summary kpiMetrics, exception KPI, risk KPI, freshness, and demoMode. Funnels and billing/document summaries are computed in the composable and were NOT_FOUND on index.vue.
NOTES=FRONTEND_DERIVED. Demo data is DEMO_ONLY. Fallback KPI is FALLBACK_ONLY. See current-state document for the disposition of each calculation.
```

## Procurement and rates

### SRC-RFX

```
SOURCE_ID=SRC-RFX
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN (rfx-service)
SERVICE=rfx-service
TABLE_OR_OBJECT=rfx.rfx_events, rfx_lots, rfx_lanes, rfx_participants, rfx_responses, rfx_response_offer_lines, rfx_awards, rfx_award_transport_orders, freight_requests, bids, bid_items, rfx_versions, audit_events
EVENT_OR_API=RFx and freight-request APIs; audit action publish
KEY_FIELDS=status, rfx_type, currency_code, participant status, response amount, award total_amount, lane origin_location_id, destination_location_id
TIME_FIELDS=rfx_events.created_at, response_deadline, participants.invited_at, viewed_at, responded_at, responses.submitted_at, bids.submitted_at, awards.awarded_at, rfx_versions.published_at, audit_events.occurred_at
TENANT_SCOPE=tenant_id
HISTORY_CLASS=CURRENT_STATE_ONLY on entities; EVENT_HISTORY for audit_events
MUTABILITY=entity status updates; audit append-only
REBUILDABLE=PARTIAL via audit, not a status-history table
CURRENT_CONSUMERS=procurement UI; Control Tower fallback tender funnel (not the summary page)
NOTES=NOT_FOUND: rfx_events.published_at. Publish evidence is status PUBLISHED plus audit action publish. NOT_FOUND: tender savings baseline. Two commercial paths exist (RFx event responses and freight-request bids).
```

### SRC-CONTRACT-RATE

```
SOURCE_ID=SRC-CONTRACT-RATE
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN (contract-rate-service)
SERVICE=contract-rate-service
TABLE_OR_OBJECT=contract_rate.transport_contract, rate_card, rate_card_version, rate_line, rate_component, audit_event
EVENT_OR_API=rate resolution
KEY_FIELDS=pricing_source CONTRACT_RATE, MANUAL_SPOT, RFQ_AWARD, SPOT_BID; component_type BASE_FREIGHT, FUEL_SURCHARGE, WAITING, DETENTION; currency_code
TIME_FIELDS=valid_from, valid_to, activated_at
TENANT_SCOPE=tenant_id
HISTORY_CLASS=CURRENT_STATE_ONLY plus audit_event
MUTABILITY=draft versions mutable; active versions superseded
REBUILDABLE=PARTIAL
CURRENT_CONSUMERS=transport order rate snapshots (000051)
NOTES=This is a price source. It is not a tender savings baseline.
```

## Freight cost, billing, payment

### SRC-FREIGHT-COST

```
SOURCE_ID=SRC-FREIGHT-COST
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN (freight-cost-service) until an analytics boundary is frozen
SERVICE=freight-cost-service
TABLE_OR_OBJECT=freight_cost.cost_entry, cost_summary_projection, cost_analytics_* projections
EVENT_OR_API=ledger apply; public analytics projections inside this service
KEY_FIELDS=entry_kind, amount, currency_code, data_stage, financial_finality, planned_amount, accrued_amount, current_actual_amount, final_actual_amount, billing_register_amount, payable_amount, paid_amount
TIME_FIELDS=source_occurred_at, recorded_at
TENANT_SCOPE=tenant_id
HISTORY_CLASS=EVENT_HISTORY for cost_entry (append-only, supersedes_entry_id); CURRENT_STATE_ONLY for cost_summary_projection; derived projections are SERVER_DERIVED_AGGREGATE and rebuildable from the ledger
MUTABILITY=ledger denies update/delete; projection is current rollup
REBUILDABLE=YES for ledger-derived cost stages within one currency per transport order (ErrCurrencyMismatch)
CURRENT_CONSUMERS=freight cost APIs
NOTES=Entry kinds FOUND: PLANNED_COST_SNAPSHOT, ACCRUAL_COST_SNAPSHOT, CURRENT_ACTUAL_COST_SNAPSHOT, FINAL_ACTUAL_COST_SNAPSHOT, BILLED_COST_SNAPSHOT, PAYABLE_AMOUNT_SNAPSHOT, PAID_AMOUNT_SNAPSHOT. NOT_FOUND: ESTIMATED, INVOICED as those exact enums. data_stage includes PLANNED_ONLY. Mixed-currency analytics set MixedCurrency / DataQualityMixedCurrency. This is the strongest existing server-owned money model. It is not a general BI warehouse.
CLASS=CANONICAL_EVENT for cost_entry; SERVER_READ_MODEL for cost_summary_projection; SERVER_DERIVED_AGGREGATE for cost_analytics_* 
```

### SRC-BILLING

```
SOURCE_ID=SRC-BILLING
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN (billing-register-service)
SERVICE=billing-register-service
TABLE_OR_OBJECT=billing.billing_registers, billing_register_items, freight_settlements, settlement_accessorials, invoices, billing_register_audit_events, settlement_audit_events, freight_cost_outbox
EVENT_OR_API=settlement and register lifecycle; outbox snapshots to freight-cost
KEY_FIELDS=base_freight_amount, base_amount, extra_charges, penalties, currency_code, pricing_source, register status through PAID and CLOSED
TIME_FIELDS=service_accepted_at, approved_at, invoice_date, created_at
TENANT_SCOPE=tenant_id
HISTORY_CLASS=CURRENT_STATE_ONLY on registers and settlements; EVENT_HISTORY for audit and outbox
MUTABILITY=status transitions in place
REBUILDABLE=PARTIAL
CURRENT_CONSUMERS=finance UI; Control Tower billing summary (frontend, not summary page); freight-cost ledger
NOTES=money_policy.go rejects mixed currency inside one register. Penalties live on register items, not as a freight-cost ledger component.
```

### SRC-PAYMENT

```
SOURCE_ID=SRC-PAYMENT
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN (payment-service)
SERVICE=payment-service
TABLE_OR_OBJECT=billing.payment_obligations, payments, payment_allocations, payment_audit_events, payment_outbox
EVENT_OR_API=payment_obligation.paid_snapshot.v1
KEY_FIELDS=original_amount, paid_amount, outstanding_amount, due_date, payment status, obligation status
TIME_FIELDS=payment_date, value_date, due_date, reconciled_at, voided_at
TENANT_SCOPE=tenant_id
HISTORY_CLASS=CURRENT_STATE_ONLY plus audit events
MUTABILITY=amounts update in place with version
REBUILDABLE=PARTIAL
CURRENT_CONSUMERS=freight-cost PAID_AMOUNT_SNAPSHOT
NOTES=IsObligationOverdue compares due_date with today. NOT_FOUND: an aging-bucket table.
```

## Documents

### SRC-DOCUMENT

```
SOURCE_ID=SRC-DOCUMENT
OWNER_AGENT=B
SERVICE=document-service
TABLE_OR_OBJECT=documents.documents, document_versions, signing_sessions, signatures
EVENT_OR_API=document and signing-session APIs
KEY_FIELDS=document_status (DRAFT, READY_FOR_SIGNING, SIGNING_IN_PROGRESS, SIGNED, SENT_TO_OPERATOR, ACCEPTED, REJECTED, ARCHIVED, CANCELLED), document_type including POD
TIME_FIELDS=created_at, updated_at, signatures.signed_at, signing_sessions.created_at
TENANT_SCOPE=tenant_id
HISTORY_CLASS=CURRENT_STATE_ONLY for documents; document_versions is version history
MUTABILITY=status updates; updated_at is not a dedicated ready-for-signing event time
REBUILDABLE=PARTIAL
CURRENT_CONSUMERS=Control Tower document list in the fallback composable; shipment execution POD summary
NOTES=SIGNED is a workflow status. ADR-EDO-010: legacy session can set SIGNED and verification_status VALID without cryptographic verification. QUALIFIED_DOCUMENT_TRUST_READY=NO.
```

### SRC-SIGNATURE-EVIDENCE

```
SOURCE_ID=SRC-SIGNATURE-EVIDENCE
OWNER_AGENT=B
SERVICE=document-service
TABLE_OR_OBJECT=documents.attachment_signatures, signature_verification_history (000095), attachment_signature_blobs, signature_verification_evidence (000096)
EVENT_OR_API=detached signature upload; evidence insert
KEY_FIELDS=verification_status, reason_code, policy_id, policy_version, profile CAdES-BES
TIME_FIELDS=signing_time, verification_time, attempted_at, created_at
TENANT_SCOPE=tenant_id
HISTORY_CLASS=EVENT_HISTORY for evidence and verification history
MUTABILITY=000095 triggers force UNVERIFIED on attachment signature rows; 000096 evidence check allows PENDING and VERIFIER_UNAVAILABLE only
REBUILDABLE=YES for the attempts that were stored; those attempts are not successful verifications
CURRENT_CONSUMERS=attachment signature flow
NOTES=CANONICAL_EVENT for evidence rows. They currently prove verifier absence, not qualified trust.
```

## Network optimizer

### SRC-NLO-SEARCH

```
SOURCE_ID=SRC-NLO-SEARCH
OWNER_AGENT=D
SERVICE=network-optimizer-service
TABLE_OR_OBJECT=network_optimizer.next_load_search_runs, match_candidates (000079, 000080)
EVENT_OR_API=next-load search
KEY_FIELDS=routing_provider, eligibility, reject_reasons, road_deadhead_km, road_deadhead_minutes, score_components jsonb
TIME_FIELDS=search run timestamps (column-level freshness UNKNOWN beyond created persistence)
TENANT_SCOPE=owner_tenant_id / tenant_id; anonymized marketplace visibility
HISTORY_CLASS=EVENT_HISTORY for stored runs and candidates if rows are retained; retention policy UNKNOWN
MUTABILITY=search results persisted; in-memory store exists for tests (search_memory.go)
REBUILDABLE=PARTIAL for fields that were persisted. route_increase_km is API-only (GAP-D-005).
CURRENT_CONSUMERS=NLO search API
NOTES=CANDIDATE_ROAD_DEADHEAD_KM_FACT=FOUND on match_candidates.road_deadhead_km. CANDIDATE_ROAD_DEADHEAD_IS_EXECUTED_DEADHEAD=NO. CANDIDATE_ROAD_DEADHEAD_IS_FLEET_DEADHEAD=NO. Do not sum alternative candidates and call the result NET_DEADHEAD_KM. straight.go forbids storing straight-line distance as road_deadhead_km. NLO owns the planned search fact. TMS owns executed distance (GAP-C-006). NLO_0_5B2_IN_MAIN=YES adds routing caps and aggregate counters. It does not add executed distance, deadhead reduction, or a backhaul fact. route_increase_km is still not a match_candidates column (GAP-D-005).
CLASS=CANONICAL_EVENT for persisted candidates; some response fields are PROVISIONAL
```

### SRC-NLO-ROUTE-PLAN

```
SOURCE_ID=SRC-NLO-ROUTE-PLAN
OWNER_AGENT=D
SERVICE=network-optimizer-service
TABLE_OR_OBJECT=network_optimizer.route_plans and related stops, legs, actions (000085, 000086); service_duration_policies (000094)
EVENT_OR_API=MarkRoutePlanAccepted
KEY_FIELDS=status including ACCEPTED, accepted_at, execution_supported
TIME_FIELDS=accepted_at, cancelled_at, superseded_at, created_at
TENANT_SCOPE=tenant on the plan; public_subject_snapshot for anonymized exposure
HISTORY_CLASS=CURRENT_STATE_ONLY for plan status with accepted_at set once (replay test keeps accepted_at)
MUTABILITY=status transition to ACCEPTED
REBUILDABLE=NO as a search-to-accept conversion (GAP-D-004)
CURRENT_CONSUMERS=NLO route planning API
NOTES=ACCEPTED is not a backhaul acceptance. 000094 adds planning service-duration policy. That policy is not measured stop dwell.
```

### SRC-NLO-METRICS

```
SOURCE_ID=SRC-NLO-METRICS
OWNER_AGENT=D
SERVICE=network-optimizer-service
TABLE_OR_OBJECT=Prometheus metrics in internal/platform/metrics/metrics.go
EVENT_OR_API=process metrics
KEY_FIELDS=bno_search_runs_total, bno_search_duration_seconds, bno_routing_errors_total, bno_matrix_batches_total, bno_candidate_discovered_total, bno_candidate_visibility_pass_total, bno_candidate_prefilter_pass_total, bno_candidate_pruned_total, bno_candidate_returned_total, bno_routing_candidates_selected_total, bno_routing_candidates_pruned_total, bno_provider_route_calls_total, bno_provider_matrix_calls_total, bno_provider_budget_exhausted_total
TIME_FIELDS=scrape time
TENANT_SCOPE=counters are process aggregates. bno_provider_budget_exhausted_total label reason is a closed set (deadline, matrix, route, total, provider_error, dimension). No tenant, customer, or shipment label was found on these counters.
HISTORY_CLASS=UNKNOWN (metrics retention is outside this repository)
MUTABILITY=counter / histogram
REBUILDABLE=NO as business history
CURRENT_CONSUMERS=observability
NOTES=OBSERVABILITY_ONLY=YES. PROVIDER_CALL_COUNTERS_FOUND=YES. PROVIDER_CALL_COUNTERS_ARE_OBSERVABILITY=YES. PROVIDER_CALL_COUNTERS_ARE_BUSINESS_KPI=NO. ROUTING_SELECTION_COUNTERS_FOUND=YES. RecordCandidateDiscovery and RecordRoutingSelection add counts only. They are not canonical business history and do not prove accepted backhaul, executed distance, deadhead reduction, or utilization.
```

## Dimensions

### SRC-COMPANY-LOCATION

```
SOURCE_ID=SRC-COMPANY-LOCATION
OWNER_AGENT=shared master data
SERVICE=company-service and transport location tables
TABLE_OR_OBJECT=core.tenants, core.companies, transport.locations, transport.vehicles, transport.drivers
EVENT_OR_API=company APIs; location rows
KEY_FIELDS=companies.id, company_type (SHIPPER, CARRIER, and others), locations.id, location_type, country_code, region, city, timezone, lat, lon
TIME_FIELDS=created_at, updated_at, deleted_at
TENANT_SCOPE=companies bound to caller tenant; locations have tenant_id
HISTORY_CLASS=CURRENT_STATE_ONLY
MUTABILITY=names and coordinates can change; no slowly-changing-dimension table was found
REBUILDABLE=NO for historical display names
CURRENT_CONSUMERS=all domain services via UUID references
NOTES=Carrier and shipper are company_type values on core.companies, not separate id spaces. Location timezone defaults to Europe/Moscow (000003). That default is not a reporting timezone policy.
```

## Class index

| SOURCE_ID | Class |
| --- | --- |
| SRC-SHIPMENT | CANONICAL_TRANSACTIONAL_FACT |
| SRC-SHIPMENT-STATUS-HISTORY | CANONICAL_HISTORY |
| SRC-SHIPMENT-OUTBOX | CANONICAL_EVENT |
| SRC-EXECUTION-STOP | CANONICAL_TRANSACTIONAL_FACT |
| SRC-EXECUTION-COMMAND-AUDIT | CANONICAL_EVENT |
| SRC-DISPOSITION | CANONICAL_TRANSACTIONAL_FACT and CANONICAL_EVENT (attempts) |
| SRC-CARGO | CANONICAL_TRANSACTIONAL_FACT |
| SRC-TRACKING | CANONICAL_EVENT and SERVER_READ_MODEL (state tables) |
| SRC-CT-GATEWAY-SUMMARY | SERVER_DERIVED_AGGREGATE |
| SRC-CT-STATUS-PROJECTION | SERVER_READ_MODEL |
| SRC-CT-RISK-EXCEPTION | SERVER_READ_MODEL |
| SRC-CT-FRONTEND | FRONTEND_DERIVED |
| SRC-RFX | CANONICAL_TRANSACTIONAL_FACT; audit is CANONICAL_EVENT |
| SRC-CONTRACT-RATE | CANONICAL_TRANSACTIONAL_FACT |
| SRC-FREIGHT-COST cost_entry | CANONICAL_EVENT |
| SRC-FREIGHT-COST summary | SERVER_READ_MODEL |
| SRC-FREIGHT-COST analytics projections | SERVER_DERIVED_AGGREGATE |
| SRC-BILLING | CANONICAL_TRANSACTIONAL_FACT |
| SRC-PAYMENT | CANONICAL_TRANSACTIONAL_FACT |
| SRC-DOCUMENT | CANONICAL_TRANSACTIONAL_FACT |
| SRC-SIGNATURE-EVIDENCE | CANONICAL_EVENT |
| SRC-NLO-SEARCH | CANONICAL_EVENT with PROVISIONAL response-only fields |
| SRC-NLO-ROUTE-PLAN | CANONICAL_TRANSACTIONAL_FACT |
| SRC-NLO-METRICS | OBSERVABILITY_ONLY |
| SRC-COMPANY-LOCATION | CANONICAL_TRANSACTIONAL_FACT |

Control Tower demo shipments in `controlTowerDemoData.ts` are a separate DEMO_ONLY source. They are not in the class index as a business fact.
