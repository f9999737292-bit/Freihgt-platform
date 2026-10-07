# BINTRANS Analytics / Freight Intelligence

Agent E owns analytics. Source transactional domains stay with their owners.

This directory is the Analytics-0.1A inventory. It is not an architecture freeze and not an analytics service.

## Baseline

| Item | Value |
| --- | --- |
| `ORIGINAL_0_1A_BASE` | `9ec52621d272b9ca24e5a57f73448acfd0ebef3c` |
| `ORIGIN_MAIN` inspected by R1 | `ab55af609223c2ad4fcb197b8237b760b6db6104` |
| Branch | `discovery/analytics-current-state-v0.1a` |
| Worktree | `D:\Projects\freight-platform-wt\analytics-current-state-v0.1a` |
| Migration head on that main | `000096_edo_0_3_i4b_signature_verification_foundation` (no migration files in the R1 merge) |
| `NLO_0_5B2_IN_MAIN` | `NO` (PR #218 was OPEN at R1) |
| Inspection | Static repository evidence. Runtime queries were **NOT_RUN**. |

## Documents

| Document | Contents |
| --- | --- |
| [ANALYTICS_0_1A_CURRENT_STATE.md](ANALYTICS_0_1A_CURRENT_STATE.md) | What already exists: Control Tower, frontend calculations, history, time, tenancy, 0.1B inputs |
| [ANALYTICS_0_1A_SOURCE_MAP.md](ANALYTICS_0_1A_SOURCE_MAP.md) | Classified data sources |
| [ANALYTICS_0_1A_KPI_CATALOG.md](ANALYTICS_0_1A_KPI_CATALOG.md) | Canonical KPI catalog and readiness totals |
| [ANALYTICS_0_1A_SOURCE_CONTRACT_GAPS.md](ANALYTICS_0_1A_SOURCE_CONTRACT_GAPS.md) | Missing facts and which agent owns them |

## Rules used in this inventory

- Analytics consumes canonical facts. It does not invent them.
- One business KPI should have one definition. Control Tower SLA `ON_TIME` is not OTIF and is not a completed on-time delivery rate.
- `DocumentStatus=SIGNED` is not qualified document trust (`docs/adr/ADR-EDO-010-attachment-signature-verification.md`).
- Planned freight cost is not actual freight cost.
- Monetary totals are currency-partitioned. Mixed-currency sums are not safe.
- Prometheus counters are observability, not the BI warehouse.
- Network Intelligence KPIs that depend on Agent D's still-moving NLO facts are marked partial, blocked, or not supported. Design notes such as `ADR-NET-023` are not persisted facts: `backhaul` is absent from `services/network-optimizer-service` and from `infrastructure/migrations`.

## Explicitly not decided

`DWH_DECISION=DEFERRED_TO_LATER_PHASE`

`ANALYTICS_STORAGE_DECISION=NOT_FROZEN`

No analytics service, warehouse, table, migration, API, or UI was created in this phase.
