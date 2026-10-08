# BINTRANS Analytics / Freight Intelligence

Agent E owns analytics. Source transactional domains stay with their owners.

This directory holds the Analytics-0.1A inventory and the Analytics-0.1B architecture freeze. 0.1A evidence is unchanged. 0.1B does not create an analytics service.

## Baseline

| Item | Value |
| --- | --- |
| `ORIGINAL_0_1A_BASE` | `9ec52621d272b9ca24e5a57f73448acfd0ebef3c` |
| `FINAL_BASELINE_INSPECTED` | `c505c84bd432d7bd039b97968506a375522c165c` |
| Branch | `discovery/analytics-current-state-v0.1a` |
| Worktree | `D:\Projects\freight-platform-wt\analytics-current-state-v0.1a` |
| Migration head | `000096` (R2 merge added no migration files) |
| `NLO_0_5B2_IN_MAIN` | `YES` |
| Inspection | Static repository evidence. Runtime queries were **NOT_RUN**. |

## Documents

| Document | Contents |
| --- | --- |
| [ANALYTICS_0_1A_CURRENT_STATE.md](ANALYTICS_0_1A_CURRENT_STATE.md) | What already exists: Control Tower, frontend calculations, history, time, tenancy, 0.1B inputs |
| [ANALYTICS_0_1A_SOURCE_MAP.md](ANALYTICS_0_1A_SOURCE_MAP.md) | Classified data sources |
| [ANALYTICS_0_1A_KPI_CATALOG.md](ANALYTICS_0_1A_KPI_CATALOG.md) | Canonical KPI catalog and readiness totals |
| [ANALYTICS_0_1A_SOURCE_CONTRACT_GAPS.md](ANALYTICS_0_1A_SOURCE_CONTRACT_GAPS.md) | Missing facts and which agent owns them |
| [ANALYTICS_0_1B_ARCHITECTURE.md](ANALYTICS_0_1B_ARCHITECTURE.md) | Canonical KPI and analytics architecture freeze |
| [ANALYTICS_0_3C_CARRIER_SCORE_POLICY.md](ANALYTICS_0_3C_CARRIER_SCORE_POLICY.md) | Carrier performance score policy decision |
| [adr/ADR-AN-001-canonical-kpi-ownership.md](adr/ADR-AN-001-canonical-kpi-ownership.md) | Who owns KPI meaning |
| [adr/ADR-AN-002-analytics-storage-boundary.md](adr/ADR-AN-002-analytics-storage-boundary.md) | First storage boundary |
| [adr/ADR-AN-003-time-late-event-restatement.md](adr/ADR-AN-003-time-late-event-restatement.md) | Time, late events, restatement |
| [adr/ADR-AN-004-tenant-and-dimension-model.md](adr/ADR-AN-004-tenant-and-dimension-model.md) | Tenant scope and dimensions |
| [adr/ADR-AN-005-control-tower-vs-analytics.md](adr/ADR-AN-005-control-tower-vs-analytics.md) | Control Tower boundary |

## Rules used in this inventory

- Analytics consumes canonical facts. It does not invent them.
- One business KPI should have one definition. Control Tower SLA `ON_TIME` is not OTIF and is not a completed on-time delivery rate.
- `DocumentStatus=SIGNED` is not qualified document trust (`docs/adr/ADR-EDO-010-attachment-signature-verification.md`).
- Planned freight cost is not actual freight cost.
- Monetary totals are currency-partitioned. Mixed-currency sums are not safe.
- Prometheus counters are observability, not the BI warehouse.
- Network Intelligence KPIs that depend on Agent D's still-moving NLO facts are marked partial, blocked, or not supported. Design notes such as `ADR-NET-023` are not persisted facts: `backhaul` is absent from `services/network-optimizer-service` and from `infrastructure/migrations`.

## 0.1A left these open

The inventory recorded `DWH_DECISION=DEFERRED_TO_LATER_PHASE` and `ANALYTICS_STORAGE_DECISION=NOT_FROZEN`. Those sentences remain true of 0.1A. Analytics-0.1B freezes storage as hybrid operational reads plus later analytics projections, with `DWH_V1_REQUIRED_NOW=NO`.

No analytics service, warehouse, table, migration, API, or UI is created by 0.1A or 0.1B. Analytics-0.1B required a shipment-owned read contract before computation. PR #222 (Analytics-0.2A) implemented and merged `GET /internal/v1/analytics/operations-foundation`. Analytics-0.2B consumes that contract. The public route stays on the API gateway.

## Analytics-0.2B

`ANALYTICS_SERVICE_IMPLEMENTED=YES`

`services/analytics-service` is a stateless process on port 8097. It has no database, no analytics tables, and no migration. It reads `GET /internal/v1/analytics/operations-foundation` from shipment-service and maps exactly five KPIs at `definitionVersion=1`: `OPS_SHIPMENTS_TOTAL`, `OPS_ON_TIME_DELIVERY`, `OPS_ON_TIME_DELIVERY_RATE`, `OPS_RETURN_CASES`, and `OPS_REDIRECT_CASES`.

The public route is `GET /api/v1/analytics/kpis/{kpiId}` on api-gateway. The gateway authenticates the JWT, authorizes the analytics read role, and forwards the payload. It does not compute the KPI. Driver is denied. Query filters and a details route are not part of this release.

`DWH_IMPLEMENTED=NO`

`HISTORICAL_ANALYTICS_READY=NO`

`ANALYTICS_STATELESS=YES`

`MIGRATION_CREATED=NO`

0.1A readiness totals are unchanged.

## Analytics-0.3A

Discovery only. `ANALYTICS_0_3A_OPERATIONS_CARRIER_SCOPE.md` freezes which operations and carrier KPIs can enter a later implementation wave. It does not change the five Analytics-0.2B KPIs, their meaning, or `definitionVersion=1`. No product code, API, migration, or warehouse is part of 0.3A.

## Analytics-0.3B

`ANALYTICS_0_3B_IMPLEMENTED=YES`

`TOTAL_IMPLEMENTED_KPI_COUNT=11`

Analytics-service reads `GET /internal/v1/analytics/operations-foundation-v2`. The original five KPIs stay at `definitionVersion=1`. The six added KPIs are `OPS_ON_TIME_PICKUP`, `OPS_ON_TIME_PICKUP_RATE`, `OPS_LATE_PICKUP`, `OPS_LATE_DELIVERY`, `CAR_ON_TIME_PICKUP_RATE`, and `CAR_ON_TIME_DELIVERY_RATE`. Late counts are `denominator - numerator` inside analytics. Carrier rates are one ratio per `carrierCompanyId`, ordered by that id, with `dimension=CARRIER`. The service remains stateless. `DWH_IMPLEMENTED=NO`. `MIGRATION_CREATED=NO`. GAP-C-001, GAP-C-002, GAP-C-005, and GAP-E-001 stay open.

Shipment-service still owns the source facts. `GET /internal/v1/analytics/operations-foundation` returns only the five Analytics-0.2 fields. The v2 read adds `onTimePickupDenominator`, `onTimePickupNumerator`, and `carriers`.

## Analytics-0.3C

Policy only. `ANALYTICS_0_3C_CARRIER_SCORE_POLICY.md` records `CARRIER_SCORE_POLICY_DECISION=DEFER`.

`CAR_PERFORMANCE_SCORE` and `EXEC_CARRIER_PERFORMANCE` are not defined. The two implemented carrier rates stay punctuality facts. No weights are assigned. `CARRIER_SCORE_READY=NO`. `CARRIER_SCORE_WEIGHT_VERSIONING=YES`. GAP-E-001 stays open, as do GAP-C-001, GAP-C-002, and GAP-C-005. 0.1A readiness totals are unchanged. No product code, API, migration, or warehouse is part of 0.3C.

`NEXT_STAGE=WAIT_FOR_CARRIER_SCORE_BUSINESS_POLICY`
