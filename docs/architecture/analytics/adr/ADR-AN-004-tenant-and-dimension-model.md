# ADR-AN-004: Tenant isolation and dimension model

## Status

Accepted for the Analytics-0.1B architecture freeze. Dimension tables are not created.

```text
ADR_AN_004=ACCEPTED
TENANT_ID_REQUIRED_ON_ANALYTICS_FACTS=YES
CROSS_TENANT_QUERY_DEFAULT=NO
PLATFORM_SCOPE_AGGREGATION_POLICY=PLATFORM_AUTHORIZATION_REQUIRED
LANE_UNIFICATION=DEFERRED
DIMENSION_SCD_IMPLEMENTATION=DEFERRED
MIXED_CURRENCY_UNSAFE_SUM_FORBIDDEN=YES
```

## Context

0.1A required tenant isolation and found separate lane keys in RFx, contract rate, and freight cost. Company names are mutable. Shipper and carrier are roles of one company id. No platform FX source exists.

## Decision

Every analytics fact carries the tenant it was computed for. The default query is that tenant only. A platform aggregate needs explicit platform authorization. JWT-derived tenant scope wins over a caller-supplied tenant.

Logical dimensions are listed in `ANALYTICS_0_1B_ARCHITECTURE.md`. Shipper, carrier, and consignee do not get a second company id space. Lane unification and slowly changing dimension storage are deferred. Money totals stay inside one currency until an approved FX source, rate date, and method exist.

## Consequences

Analytics cannot treat a missing tenant filter as a platform report. Cross-currency executive totals stay blocked. Historical company labels are not reliable from the current name.
