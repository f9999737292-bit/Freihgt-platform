# ADR-AN-005: Control Tower versus analytics

## Status

Accepted for the Analytics-0.1B architecture freeze. No Control Tower or frontend code is changed.

```text
ADR_AN_005=ACCEPTED
CONTROL_TOWER_IS_ANALYTICS_WAREHOUSE=NO
CONTROL_TOWER_OPERATIONAL_READ_MODEL_REUSE_ALLOWED=YES
FRONTEND_MAY_DEFINE_BUSINESS_KPI=NO
OTIF_READY=NO
```

## Context

Control Tower summary KPI is an operational, filter-scoped, current-fetch calculation. `sla.Compute` can mark a row `ON_TIME` with `ReasonOnSchedule` when the shipment is not a completed on-time delivery. Web admin `controlTowerLogic.ts` repeats a fallback formula. 0.1A classified that fallback as `DEPRECATE_LATER`.

## Decision

Control Tower remains operational current-state decision support: exceptions, risk, workflow, and operational SLA presentation. It is not the analytics warehouse.

Analytics owns canonical KPI definitions, repeatable aggregates, historical trends, carrier performance, finance analytics, procurement analytics, network intelligence, and Executive BI.

A Control Tower field may feed an analytics KPI only when the meanings match. These do not match:

- Control Tower active count versus a still-deferred `OPS_SHIPMENTS_ACTIVE`
- Control Tower `ON_TIME` versus analytics on-time delivery
- A current projection row versus historical truth

The frontend fallback may remain until a later task removes it. While it exists, it is presentation fallback, not a second business definition. The deprecation path is to render the server payload and stop recomputing the formula. This ADR does not delete the code.

## Consequences

Analytics-0.2 on-time delivery uses `planned_delivery_at` and `actual_delivery_at` on completed comparisons. It does not call `kpi.onTime`. OTIF stays blocked by GAP-C-001.
