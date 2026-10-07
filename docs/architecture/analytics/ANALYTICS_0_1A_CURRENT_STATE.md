# Analytics-0.1A current state

R2 refreshed this inventory against `origin/main` `c505c84bd432d7bd039b97968506a375522c165c`. The original 0.1A base remains `9ec52621d272b9ca24e5a57f73448acfd0ebef3c`. Migration head is still `000096`. Runtime and database queries were **NOT_RUN**.

NLO-0.5B2 and the driver-stop frontend from PR #215 are in this main. The R2 merge did not add an analytics history store.

```
NLO_0_5B2_IN_MAIN=YES
NLO_CURRENT_STATE_REFRESH_REQUIRED_AFTER_PR218=NO
NLO_PLAN_OWNER=YES
NLO_EXECUTION_OWNER=NO
TMS_EXECUTION_OWNER=YES
NLO_WRITES_TMS_DB=NO
TMS_WRITES_NLO_DB=NO
DISCOVERY_CAP=1000
ROUTING_CAP=25
MATRIX_DIMENSION_LIMIT=25
MAX_MATRIX_CALLS=4
MAX_ROUTE_CALLS=2
MAX_PROVIDER_CALLS=6
PROVIDER_REQUEST_TIMEOUT=5s
SEARCH_HARD_BUDGET=30s
RETRY_COUNT=0
PARALLELISM=1
FINAL_EVALUATION_CAP_RUNTIME_ENFORCED=NO
BACKHAUL_RUNTIME_COMPLETE=NO
ROUNDTRIP_RUNTIME_IMPLEMENTED=NO
```

The caps are `CandidateDiscoveryCap` and the constants in `services/network-optimizer-service/internal/domain/routing_bound.go`. Search applies the routing budget in `internal/service/search.go`. `CandidateFinalEvaluationCap` is declared and is not enforced on scoring or persistence (`routing_bound.go` comment; `docs/architecture/network-optimizer/adr/ADR-NET-024-bounded-backhaul-search-policy.md`). `SearchParallelism = 1` and `ProviderRetryCount = 0` are the declared bounds. ADR-NET-024 states calls are sequential and there is no retry. That ADR also states backhaul chain runtime and roundtrip runtime are not implemented. Test names that say "roundtrip" in the optimizer are request/response fingerprints, not a roundtrip product.

Those ownership flags are frozen in `docs/architecture/network-optimizer/NLO_0_4D_ARCHITECTURE_FREEZE.md` and `NLO_0_4D_I1_EXECUTION_PROJECTION_API.md`. Executed loaded, empty, and deadhead distance belong to shipment-service / TMS (Agent C). NLO may own planned or optimized search and plan distances only.

PR #215 is in this main. `DriverStopActionFact.shipmentId` is required and is replaced from `transport.transport_execution_actions.shipment_id` (`enrichActionShipmentIDs` in `driver_stop_task_repository.go`). A stored action summary is not authoritative. A multi-shipment stop may have `driver_stop_tasks.shipment_id` null while each action carries its shipment. `apps/driver-mobile` reads `action.shipmentId`. That is frontend behavior on an existing execution fact.

```
ACTION_LEVEL_SHIPMENT_ID_IN_MAIN=YES
MULTI_SHIPMENT_STOP_CONTRACT_IN_MAIN=YES
DRIVER_215_CHANGES_KPI_READINESS=NO
```

It does not add an analytics history store, an in-full quantity, executed distance, or a new OTIF component.

`ANALYTICS_SERVICE_EXISTS=NO`. No `services/analytics-service` directory.

`DWH_EXISTS=NO`. Freight-cost ledger projections are a cost read model inside `freight-cost-service`, not a warehouse.

## What already exists

| Area | Found | Role today |
| --- | --- | --- |
| Control Tower summary | `GET /api/v1/control-tower/summary` in api-gateway | Operational decision support. Server-derived counts for the filtered shipment set. |
| Control Tower read model | `control-tower-read-model-service` | Status, risk, exception workflow, execution, and disposition projections. |
| Shipment / TMS | `transport.shipments`, status history, execution stops, disposition | Canonical operational facts. |
| Tracking | `tracking.location_event` and ETA state | Telemetry and ETA. Not a substitute for stop timestamps. |
| RFx | `rfx` schema | Tender and freight-request facts. No savings baseline. |
| Contract rate | `contract_rate` schema | Price components and `pricing_source`. |
| Freight cost | `freight_cost.cost_entry` and `cost_summary_projection` | Stage-aware money, one currency per transport order. |
| Billing and payment | `billing` schema | Base, surcharges, penalties, invoices, paid, outstanding. |
| Documents | `documents` schema through migration 000096 | Workflow status plus fail-closed verification evidence. |
| NLO | search runs, candidates, route plans | Search and plan facts. Not backhaul, not loaded/empty km. |
| web-admin Control Tower | summary page plus unused composable aggregates | Display, plus fallback and demo paths. |

## Control Tower audit

### SERVER_DERIVED

Implemented in `services/api-gateway/internal/controltower` and the read model.

| Concept | Where | Formula as implemented | History | Freshness |
| --- | --- | --- | --- | --- |
| Active shipments | `CalculateKPI` (`filters.go`) using `IsActiveShipmentStatus` (`platform/sla/sla.go`) | Count of filtered rows whose status is not `CANCELLED` and not `FINANCIALLY_CLOSED` | Current fetch only | `generatedAt`; `dataFreshness.partial` and warnings |
| On-time, at-risk, delayed, critical | `sla.Compute` then `CalculateKPI` | One SLA status per row. Threshold defaults: at-risk 120 minutes, critical delay 240 minutes. Pickup overdue applies only before `actual_pickup_at` on pre-pickup statuses. Delivery overdue applies before `actual_delivery_at`. After delivery, late actual versus planned can set `ReasonCompletedLate`. If nothing worse wins, status becomes `ON_TIME` with `ReasonOnSchedule`, including when the shipment merely has planned dates or any actual timestamp. | Current fetch only | Same request |
| Awaiting documents | `CalculateKPI` | Status is `DELIVERED` or `DELIVERY_CONFIRMED` | Current | Same |
| Ready for billing | `CalculateKPI` | `row.ReadyForBilling`, set when shipment status is `READY_FOR_BILLING` (`service.go`) | Current | Same |
| Exception KPI | `CalculateExceptionKPI` | Counts over filtered critical events | Current events | Same |
| Risk KPI | read-model `CountKPI` on `control_tower.shipment_risk` | Counts by status, risk level, predicted type | Current risk row | Read-model freshness snapshot |
| Status summary | `shipment_status_projection` grouped by `current_status` | Count, not on-time | Latest projection | `lastRecordReceivedAt`, `lastProjectionAppliedAt`. `watermark` NOT_FOUND |
| Work queue | work items union workflow and risk | Workspace counts | Current | Request time |
| Automation KPI | automation repository | Pending recommendations, active and completed playbook executions | Current | Request time |

`GetSummary` calculates KPI from **filtered** rows. Risk evaluation uses the mapped set before that KPI filter. A filtered on-time count is not a tenant-wide rate.

Partial behavior: `dataFreshness.partial`, per-source loaded flags, and warnings including limited dataset. The UI can show `statusSummaryFreshness.fallbackUsed`. Completeness is not silent when those flags are set. There is still no business-event watermark.

### FRONTEND_DERIVED

`apps/web-admin/utils/controlTowerLogic.ts` `buildKpiMetrics` and `computeSlaStatus` recompute active, on-time, at-risk, delayed, and critical from client-mapped rows with fixed hour/day windows. That path is the fallback when summary KPI is absent (`useControlTower.ts`). It is not the same function as `sla.Compute`.

Summary mode still changes presentation: `buildKpiMetricsFromSummary` adds `kpi.critical + kpi.delayed` into one critical card.

### FALLBACK_ONLY

`useControlTower.ts` computes these from list payloads. `pages/control-tower/index.vue` does not reference them (search of that page found `kpiMetrics`, exception metrics, demo mode, and freshness; funnels were **NOT_FOUND** there).

| Calculation | Location | Classification | Why it is not canonical |
| --- | --- | --- | --- |
| Transport funnel | `transportFunnel` | `MOVE_TO_SERVER_ANALYTICS_CANDIDATE` | Counts list rows by status. Page-limited lists are not a population. |
| Tender funnel | `tenderFunnel` | `MOVE_TO_SERVER_ANALYTICS_CANDIDATE` | `participantsInvited` counts RFx in `PUBLISHED`, not `rfx_participants`. `bidsCount` is hardcoded `0`. `bidSubmitted` count is `0`. |
| Documents summary | `documentsSummary` | `MOVE_TO_SERVER_ANALYTICS_CANDIDATE` | Counts loaded document rows. Treats `SIGNED` and `ACCEPTED` as signed. That is presentation, and it is not qualified trust. |
| Billing summary | `billingSummary` | `MOVE_TO_SERVER_ANALYTICS_CANDIDATE` | Status counts of loaded registers. |
| `revenueTotal` | sum of `total_with_vat` | `DEPRECATE_LATER` | Sums amounts with no currency partition. Unsafe as a money KPI. |
| Risk alerts | `riskAlerts` | `KEEP_AS_PRESENTATION` for gateway/company availability alerts | Several alerts are UI connectivity, not business risk. Business risk KPI already exists on the server. |
| Recent activity | `recentActivity` | `MOVE_TO_SERVER_ANALYTICS_CANDIDATE` | Client merge of recent rows. `RecentActivity.vue` was not found imported by the control-tower page. |
| Fallback SLA KPI | `controlTowerLogic.ts` | `DEPRECATE_LATER` as a second business definition | Keep only if labeled fallback. Do not let it become a second OTIF or on-time contract. |

### DEMO_ONLY

`controlTowerDemoData.ts` supplies demo shipments and events when dev mode core fetch fails (`demoMode` on the page). Demo numbers are not facts.

### USE_EXISTING_SERVER_READ_MODEL

Summary KPI, exception KPI, risk KPI, status summary, work queue, and automation KPI. Analytics-0.1B should decide which of these remain operational Control Tower measures and which get a versioned analytics twin. Do not copy the SLA `ON_TIME` bucket into `OPS_ON_TIME_DELIVERY_RATE`.

`DUPLICATE_KPI_LOGIC_FOUND=YES` for shipment SLA (gateway `sla.Compute` versus `controlTowerLogic.ts`).

## Frontend versus server

```
CONTROL_TOWER_SERVER_KPI_FOUND=YES
CONTROL_TOWER_FRONTEND_DERIVED_FOUND=YES
DUPLICATE_KPI_LOGIC_FOUND=YES
SERVER_OWNED_MIGRATION_CANDIDATES=transport funnel, tender funnel, documents summary, billing summary, recent activity, fallback SLA KPI
```

`0_1B_DECISION_REQUIRED=YES` for each candidate. Do not migrate connectivity alerts or harmless display percents of an already server-owned numerator.

## OTIF

```
OTIF_ON_TIME_COMPONENT=PARTIAL
OTIF_IN_FULL_COMPONENT=BLOCKED
OTIF_CURRENT_READINESS=BLOCKED
```

On-time evidence that does exist:

- `transport.shipments.planned_delivery_at` and `actual_delivery_at` (`000003`, also maintained on execution transitions).
- `actual_delivery_at` is set to command `occurred_at` when status becomes `DELIVERED` (`transitionShipment`). Manual status update can set `ActualTime` on the same transition.
- `occurred_at` is required, actor-supplied, and stored UTC.

On-time evidence that must not be reused:

- Control Tower `kpi.onTime`. `sla.Compute` assigns `ON_TIME` / `ReasonOnSchedule` to shipments that are not a completed on-time delivery.
- `DELIVERED` as a synonym for in-full. **NOT proven.** Final required delivery action completion sets `DELIVERED`. Test `PARTIAL_DELIVERY_NO_EARLY_DELIVERED` keeps a partial delivery out of `DELIVERED`. Quantity acceptance is a disposition concern.

In-full evidence:

- **FOUND** only on `transport.delivery_disposition_cases` quantities, UOM `PALLET`, when a case exists (`000093`).
- **FOUND** comment: zero rejection does not open a case (`transport_execution_disposition.go`).
- **NOT_FOUND**: a shipment-level `in_full` flag.

`OPS_OTIF`, `CAR_OTIF`, and `EXEC_OTIF` are **BLOCKED** by GAP-C-001.

## Dwell

Measured dwell column: **NOT_FOUND**.

Stop timestamps **FOUND** on `transport.transport_execution_stops` (`000088`) and copied to `control_tower.execution_stop_projection` (`000092`):

| Alternative | Interval | Evidence |
| --- | --- | --- |
| A | `arrived_at` → `service_started_at` | Both columns exist. Meaning would be wait before service. Not selected. |
| B | `service_started_at` → `completed_at` | `completed_at` is the stop completion column. There is no `service_completed_at`. Meaning would be service duration. Not selected. |
| C | `arrived_at` → `completed_at` | Total time on stop. Not selected. |

`service_duration_seconds` on the stop is a planned duration. `network_optimizer.service_duration_policy_entries` (`000094`) is a planning policy. Neither is measured dwell.

Pickup versus delivery dwell would use stop action type `PICKUP` or `DELIVERY`. That split is available in execution actions. The interval choice is still open.

`OPS_DWELL_PICKUP_MIN` and `OPS_DWELL_DELIVERY_MIN` are **PARTIAL**: timestamps exist; the business interval is undefined. This is an Analytics-0.1B decision, not a missing column, so it is not a source-contract gap.

Do not use `created_at` or shipment `actual_pickup_at` as dwell. `actual_pickup_at` is the `LOADED` transition time, which can differ from `arrived_at`.

## Time and timezone

```
CANONICAL_STORAGE_TIMEZONE=UTC via TIMESTAMPTZ
REPORTING_TIMEZONE_POLICY_CURRENT=UNDEFINED
```

Operational timestamps in the reviewed migrations use `TIMESTAMPTZ`. Go writers call `.UTC()` for execution `occurred_at` and for several API formatters.

`transport.locations.timezone` is `NOT NULL DEFAULT 'Europe/Moscow'` (`000003`). Slot tables also have a timezone column (`000028`). A location default is not a reporting day-boundary policy.

Transport order requested pickup and delivery are `DATE` columns, not timestamptz. Do not mix those dates with shipment timestamptz without an explicit rule.

Actor-supplied `occurred_at` is the canonical event time for TMS commands. `created_at` / `updated_at` / `recorded_at` are processing or audit times. Analytics must not substitute them for arrival, pickup, delivery, service start, or completion.

Day-bounded KPIs are not safe until 0.1B freezes a reporting timezone. Minute deltas between two timestamptz values do not need a local day boundary.

## Historical rebuildability

| DOMAIN | CURRENT_STATE | EVENT_HISTORY | STATUS_HISTORY | REPLAYABLE | HISTORICAL_KPI_POSSIBLE | KNOWN_LOSS |
| --- | --- | --- | --- | --- | --- | --- |
| Shipment header | `transport.shipments` | outbox | `shipment_status_history` | PARTIAL; history can be partial | Status duration only when history is complete. Current row final status does not prove duration. | Soft delete. Incomplete history flag. |
| TMS stop | current stop row | command audit, progress events | NOT_FOUND as a stop status history table | Command idempotency replay. Terminal stop immutable. | Dwell only if the three timestamps were written. Later correction of a terminal stop is NOT_FOUND. | Overwritten stop row. |
| Driver | current tasks | `driver_reported_exception`, `driver_reported_delay`, command audit | NOT_FOUND as one driver event stream | Idempotency keys | Split stores. No single driver-performance fact. | No unified stream. |
| Tracking | `shipment_tracking_state` | `location_event`, `eta_observation` | `tracking_state_transition` | Dedup inserts | Ping coverage over `recorded_at` if retention holds | Retention UNKNOWN |
| Control Tower | projections and workflows | inbox, actions, risk signals | previous_status on the projection only | Projection rebuild exists in the service | Not a historical OTIF store | KPI itself is not stored |
| Procurement | RFx and bid rows | `rfx.audit_events` | NOT_FOUND | Audit can show `publish` | Created and awarded timestamps yes. Time-to-publish only via audit. | No `published_at` on the event |
| Freight cost | summary projection | `cost_entry` append-only | stage amounts on the current projection | Ledger can rebuild the projection | Stage amounts per currency yes | Summary row is current only |
| Billing / payment | register, settlement, obligation | audit and outbox | NOT_FOUND as a full status history | Outbox snapshots feed the ledger | Invoice and payment dates yes. Aging as-of history NOT_FOUND | In-place amount updates |
| Documents | `documents` status | signature evidence, attachment audit, versions | status on the current row | Evidence append | Status count is current. Ready-for-signing event time NOT_FOUND as its own column | `updated_at` is not that event |
| NLO search | latest API view | persisted runs and candidates for stored columns | NOT_FOUND | Search replay NOT proven | Deadhead on stored candidates. Route increase NOT stored. | API-only fields. Retention UNKNOWN |
| NLO accept | `route_plans` | accepted_at on the plan | status on the plan | Accept replay keeps `accepted_at` | Count of accepted plans. Not backhaul. Not search conversion. | No backhaul fact |

```
HISTORICAL_ANALYTICS_READY=NO
```

Individual KPIs can still be current-state ready. A trend needs the history class on that KPI, not the existence of a table.

## Late events, replay, corrections

| Domain | Current behavior found |
| --- | --- |
| TMS commands | Idempotency unique on `(operating_tenant_id, idempotency_key)`. Replay test: `actual_pickup_at` does not change on idempotent replay. |
| Actual timestamps | `COALESCE` keeps the first non-null actual. A later command does not replace it. |
| Terminal stops | Immutable. A late correction API was NOT_FOUND. |
| Status history | `occurred_at` stored separately from `recorded_at`. Late insert behavior beyond that was not fully traced. |
| Tracking | Dedup via `dedup_key` and `ON CONFLICT DO NOTHING`. |
| Control Tower inbox | Outcomes include `STALE`, `DUPLICATE`, `GAP_APPLIED`. |
| Freight cost | Append-only entries and `supersedes_entry_id`. Currency mismatch rejected inside one order. |
| Disposition | Append-only quantity facts. |
| Documents | Evidence append. 000096 cannot store a successful verification yet. |
| NLO accept | Replay keeps the first `accepted_at`. |

No cross-domain late-event or restatement policy exists. Daily, monthly, carrier-score, and financial closes cannot be declared reproducible until 0.1B defines one. This inventory does not define it.

## Dimensions

| Dimension | Canonical id | Display label | Tenant safety | Historical stability |
| --- | --- | --- | --- | --- |
| TIME | timestamptz instant | UNDEFINED reporting zone | n/a | Instant is stable; local day is not |
| TENANT | `core.tenants.id` | UNKNOWN | Isolation required | Id stable |
| SHIPPER | `transport.shipments.shipper_company_id` → `core.companies.id` | company name, mutable | tenant on shipment and company | Label not stable |
| CARRIER | `shipments.carrier_company_id` nullable; `company_type=CARRIER` | company name | same | Label not stable; null means unassigned |
| COMPANY | `core.companies.id` | name | caller tenant on create | Label not stable |
| SHIPMENT | `transport.shipments.id` | `shipment_number` | `tenant_id` | Number uniqueness not re-audited |
| TRANSPORT_ORDER | `transport.transport_orders.id` | order number fields | `tenant_id` | DATE requested windows, not timestamptz |
| RFx | `rfx.rfx_events.id` | `rfx_number` | `tenant_id` | Status mutable |
| CONTRACT | `contract_rate.transport_contract` | contract identity | `tenant_id` | Versioned rate cards |
| LANE | RFx `rfx_lanes.id` or contract rate line origin/destination; freight-cost `lane_key` string | labels differ by service | tenant on parent | Three different lane keys. Not one dimension yet. |
| ORIGIN / DESTINATION | `origin_location_id`, `destination_location_id` | `transport.locations` name, city | location `tenant_id` | Name and coordinates mutable |
| REGION | `locations.region` and `country_code` | same row | tenant | Not a separate region id |
| WAREHOUSE / LOCATION | `locations.id`, `location_type` includes `WAREHOUSE` | name | tenant | Label mutable |
| TRANSPORT_MODE | column on shipment, RFx lane, rate line | the code | parent tenant | Code is the dimension |
| EQUIPMENT_TYPE | RFx lane and rate line | code | parent tenant | NOT proven on every shipment |
| VEHICLE | `transport.vehicles.id` | identifiers on the vehicle row | `carrier_company_id` | NOT a full history |
| DRIVER | `transport.drivers.id`, `shipments.driver_id` | driver row | carrier scope | NOT a performance fact by itself |
| CARGO_TYPE | `cargoes.cargo_type` | the code | `tenant_id` | Mutable cargo row |
| CUSTOMER / CONSIGNEE | `consignee_company_id` | company name | shipment tenant | Label mutable |
| CURRENCY | `currency_code` on money tables | ISO code | must stay inside the tenant money fact | Do not convert without a rate contract. Rate source NOT_FOUND as a platform FX service in this inventory. |
| RISK_LEVEL | `control_tower.shipment_risk.risk_level` | enum | tenant | Current assessment |
| EXCEPTION_TYPE | workflow and driver-reported categories | enums differ by table | tenant | Not one taxonomy |
| DOCUMENT_TYPE | `documents.document_type` | code, includes `POD` | tenant | Status is separate from type |

`TENANT_ISOLATION_REQUIRED=YES`. Cross-tenant raw analytics are denied. Platform-wide analytics are not authorized.

NLO anonymized marketplace views must stay coarse (`LoadOpportunity.MarketplaceView`, `visibility_scope` `ANONYMIZED_MARKETPLACE`). Analytics must not re-identify customer, facility, or coordinates from those aggregates.

## Money

Freight cost distinguishes stages. `data_stage=PLANNED_ONLY` and `entry_kind=PLANNED_COST_SNAPSHOT` are planned. `CURRENT_ACTUAL_*` and `FINAL_ACTUAL_*` are different later stages. `financial_finality` includes `FINAL_ACTUAL`.

Billing adds `base_amount` / `base_freight_amount`, `extra_charges` / accessorials, and `penalties`.

Payment adds `paid_amount` and `outstanding_amount`.

Three "closed" markers exist and are not the same fact:

- shipment status `FINANCIALLY_CLOSED`
- billing register status `CLOSED`
- freight-cost `financial_finality=FINAL_ACTUAL`

0.1B must pick which marker `FIN_FINANCIALLY_CLOSED_AMOUNT` uses. This inventory does not.

`MIXED_CURRENCY_SAFE=NO` for every monetary KPI until a conversion policy, rate source, and rate timestamp exist. Per-currency sums are allowed where the source already stores `currency_code` and rejects mixed currency inside the grain (freight-cost order projection, billing register).

## Analytics-0.1B inputs

Decisions only. No choice is made here.

1. Analytics service boundary versus extending freight-cost analytics and Control Tower.
2. Which Control Tower aggregates stay operational and which become versioned KPIs.
3. Event ingestion versus query-time reads of current tables.
4. Rebuild and replay scope per domain in the history matrix.
5. `KPI_ID` + `KPI_VERSION` storage when a formula changes.
6. What history is persisted, and what is only queryable while source rows remain.
7. Time grain and `REPORTING_TIMEZONE_POLICY` (currently `UNDEFINED`).
8. One lane dimension across RFx, contract rate, and freight-cost `lane_key`.
9. Currency normalization. Until then, partition by currency.
10. Freshness: `generated_at`, source watermark, partial, warnings. Control Tower has the first and the last two, not a business watermark.
11. Late events and corrections, given first-write `COALESCE` and terminal immutability.
12. RBAC for tenant analytics versus any future platform-wide view.
13. Retention of tracking pings, NLO search runs, and outbox payloads.
14. API shape for KPI results (version, currency, partial, warnings).
15. Export / external BI boundary. Prometheus is out of that boundary.
16. Dwell interval A, B, or C.
17. Which timestamp pair is on-time pickup and on-time delivery, and that Control Tower `ON_TIME` is not that KPI.
18. Participation and award-rate denominators for RFx events versus freight-request bids.
19. Actual cost means `CURRENT_ACTUAL` or `FINAL_ACTUAL`.
20. Financial close marker.
21. Carrier score policy (GAP-E-001) after source components exist.
22. NLO planned facts stay with Agent D. Executed distance stays with Agent C (GAP-C-006). Provider route and matrix counters on this main are observability. They do not close deadhead reduction, backhaul, search-to-accept conversion, or executed distance.

```
DWH_DECISION=DEFERRED_TO_LATER_PHASE
ANALYTICS_STORAGE_DECISION=NOT_FROZEN
READY_FOR_ANALYTICS_0_1B=NO
```

This file is still an inventory. Controller acceptance of the R1 remediation is required before Analytics-0.1B. It is not a license to create tables.
