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

No analytics service, warehouse, table, migration, API, or UI is created by 0.1A or 0.1B. Analytics-0.2 computation is frozen to a future analytics-service. The public route stays on the API gateway. The shipment read contract is still required from shipment-service.

## Analytics-0.2B

`ANALYTICS_SERVICE_IMPLEMENTED=YES`

`services/analytics-service` is a stateless process on port 8097. It has no database, no analytics tables, and no migration. It reads `GET /internal/v1/analytics/operations-foundation` from shipment-service and maps exactly five KPIs at `definitionVersion=1`: `OPS_SHIPMENTS_TOTAL`, `OPS_ON_TIME_DELIVERY`, `OPS_ON_TIME_DELIVERY_RATE`, `OPS_RETURN_CASES`, and `OPS_REDIRECT_CASES`.

The public route is `GET /api/v1/analytics/kpis/{kpiId}` on api-gateway. The gateway authenticates the JWT, authorizes the analytics read role, and forwards the payload. It does not compute the KPI. Driver is denied. Query filters and a details route are not part of this release.

`DWH_IMPLEMENTED=NO`

`HISTORICAL_ANALYTICS_READY=NO`

`ANALYTICS_STATELESS=YES`

`MIGRATION_CREATED=NO`

0.1A readiness totals are unchanged.
