# ADR-AN-006: Canonical analytics semantic layer

## Status

Accepted — strategic architecture freeze; implementation separately authorized

```text
ADR_AN_006=ACCEPTED
RUNTIME_IMPLEMENTATION_AUTHORIZED=NO
ANALYTICS_MAY_INVENT_SOURCE_FACTS=NO
ONE_BUSINESS_KPI_ONE_CANONICAL_DEFINITION=YES
DWH_V1_REQUIRED_NOW=NO
```

## Context

Analytics-0.1A inventoried KPI readiness and source gaps. Analytics-0.1B froze ownership, history classes, and the logical semantic contract without implementing a warehouse. Detailed records already exist under `docs/architecture/analytics/`, including KPI ownership ADR-AN-001, storage boundary ADR-AN-002, and Control Tower versus analytics ADR-AN-005 in that tree.

NEXT needs a registry-level decision: the semantic layer is the product contract for KPI meaning across portals, Control Tower, and future BI. This record is numbered ADR-AN-006 so it does not collide with the existing analytics-tree ADR-AN-001.

## Decision

### 1. Semantic layer is the KPI contract

Each promoted KPI carries at least:

`KPI_ID`, `DEFINITION_VERSION`, `BUSINESS_NAME`, `BUSINESS_DEFINITION`, numerator/denominator or equivalent measure shape, inclusion/exclusion rules, time basis, timezone policy, source facts, source owner, dimensions, currency policy when money is involved, late-event policy, readiness, and history class.

Identity is `KPI_ID` + `DEFINITION_VERSION`. A meaning change creates a new version. Historical results keep the version that produced them.

### 2. One business KPI, one canonical definition

Once promoted, analytics owns the business meaning. Source services keep raw facts. Frontends must not publish a second formula for the same business KPI. Presentation fallbacks may render a payload; they are not a competing definition.

### 3. Derive, do not invent

```text
ANALYTICS_MAY_DERIVE=YES
ANALYTICS_MAY_INVENT_SOURCE_FACTS=NO
```

Blocked source gaps stay blocked until the owning domain publishes the fact under an authorized contract.

### 4. Control Tower is not the warehouse

Control Tower remains operational current-state decision support. A Control Tower field may feed an analytics KPI only when meanings match. Operational SLA `ON_TIME` is not automatically analytics on-time delivery.

### 5. Storage posture for NEXT

The hybrid path from Analytics-0.1B / ADR-AN-002 remains: current-state reads via source-owned internal contracts first; analytics-owned projections later when history class requires them; dedicated DWH optional later, not required for v1.

```text
CURRENT_STATE_IS_HISTORY=NO
HISTORICAL_ANALYTICS_READY=NO
```

until an authorized stage proves otherwise for a named KPI set.

### 6. Registry relationship

This ADR is the strategic registry entry that freezes the NEXT semantic-layer product contract under `ADR-AN-*`. Existing analytics-tree records ADR-AN-001 through ADR-AN-005 remain unchanged and binding for their scopes. Where a later authorized analytics ADR narrows a KPI slice, it must not weaken the rules above without superseding this ADR.

## Consequences

- KPI promotion and analytics-service implementation need separate Task Contracts.
- OTIF and other blocked KPIs stay blocked until source gaps close.
- Gateway role stays public routing and auth for analytics APIs; analytics does not become a second write path into source schemas.

## Non-authorization

This ADR does not create `analytics-service`, projections, migrations, OpenAPI routes, BI tooling, or staging analytics enablement.

## References

- [BINTRANS_PRODUCT_ROADMAP_2026_2027_NEXT.md](../architecture/BINTRANS_PRODUCT_ROADMAP_2026_2027_NEXT.md)
- [ANALYTICS_0_1B_ARCHITECTURE.md](../architecture/analytics/ANALYTICS_0_1B_ARCHITECTURE.md)
- [ADR-AN-002 (analytics tree)](../architecture/analytics/adr/ADR-AN-002-analytics-storage-boundary.md)
- [ADR-AN-005 (analytics tree)](../architecture/analytics/adr/ADR-AN-005-control-tower-vs-analytics.md)
