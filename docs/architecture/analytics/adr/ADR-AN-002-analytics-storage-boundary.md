# ADR-AN-002: Analytics storage boundary

## Status

Accepted for the Analytics-0.1B architecture freeze. No warehouse, projection table, or analytics service is created.

```text
ADR_AN_002=ACCEPTED
ANALYTICS_STORAGE_V0_2_DECISION=OPTION_D_HYBRID
DWH_V1_REQUIRED_NOW=NO
DWH_FUTURE_OPTION=YES
ANALYTICS_SERVICE_IMPLEMENTED=NO
DWH_IMPLEMENTED=NO
CURRENT_STATE_IS_HISTORY=NO
```

## Context

0.1A found no analytics service and no warehouse. 17 of 114 KPIs are READY. 15 source-contract gaps are open. Historical analytics are not ready. Source domains are still changing. A warehouse would freeze incomplete semantics.

The options considered were: query operational sources only; analytics projections in PostgreSQL only; a dedicated warehouse now; or a hybrid of current operational reads plus later analytics-owned history.

## Decision

Option D is selected.

Analytics-0.2 reads canonical current-state operational facts for the five frozen operations KPIs. Those reads are not history.

Analytics-owned PostgreSQL projections are allowed only in a later wave, and only when a KPI's history class needs a canonical event, a snapshot, or a derived fact that the source does not already store. A projection must not fill a source gap.

A dedicated warehouse is a future option. It is not required for v1.

## Consequences

0.1A's `ANALYTICS_STORAGE_DECISION=NOT_FROZEN` described the inventory. This ADR freezes the decision. Implementation of projections, a service, or a warehouse needs a later authorized task.
