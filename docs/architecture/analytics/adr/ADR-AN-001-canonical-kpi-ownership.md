# ADR-AN-001: Canonical KPI ownership

## Status

Accepted for the Analytics-0.1B architecture freeze. No analytics service is implemented.

```text
ADR_AN_001=ACCEPTED
ANALYTICS_DOMAIN_OWNER=E
ONE_BUSINESS_KPI_ONE_DEFINITION=YES
KPI_DEFINITION_VERSIONING=YES
ANALYTICS_MAY_DERIVE=YES
ANALYTICS_MAY_INVENT_SOURCE_FACTS=NO
FRONTEND_BUSINESS_KPI_OWNER=NO
```

## Context

Analytics-0.1A catalogued 114 KPIs and left definition ownership open. Control Tower already computes an operational SLA. Web admin can recompute a fallback. Those formulas are not the analytics catalog.

## Decision

Agent E owns analytics KPI meaning after a KPI is promoted. Source domains keep their facts. Analytics may derive a number from stored facts. Analytics may not invent a missing source fact.

One business KPI has one canonical definition. The definition identity is `KPI_ID` plus `DEFINITION_VERSION`. A meaning change creates a new version and leaves historical results on the old version.

A frontend may present a canonical numerator and denominator. It may not define OTIF, SLA, savings, carrier score, network utilization, financial totals, or verified document counts.

## Consequences

Promoted definitions live in the semantic contract in `ANALYTICS_0_1B_ARCHITECTURE.md`. The 0.1A catalog remains the evidence record. Blocked KPIs stay blocked until the named source gap closes.
