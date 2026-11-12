# Analytics company isolation 0.4C

Policy A is the approved company scope for the eleven implemented operations KPIs.

`POLICY=A`

`IMPLEMENTATION_AUTHORIZED=YES`

`SERVER_OWNED_COMPANY_SCOPE=REQUIRED`

`RETURN_REDIRECT_COMPANY_LINK=UNVERIFIED`

`UNVERIFIED_COMPANY_LINK=FAIL_CLOSED`

`CROSS_COMPANY_ACCESS=DENIED`

`CROSS_TENANT_ACCESS=DENIED`

`AUTO_MERGE=NO`

`AUTO_DEPLOY=NO`

`INDEPENDENT_SECURITY_REVIEW=REQUIRED`

## Server-owned scope

The public route remains `GET /api/v1/analytics/kpis/{kpiId}`. The gateway confirms the JWT tenant, reads company membership from Identity, and resolves one scope:

- `PLATFORM_ADMIN` from a tenant-global Identity role (`company_id` absent) receives `TENANT`. A company-scoped platform role does not.
- One active `SHIPPER` membership with `SHIPPER_ADMIN` or `SHIPPER_LOGIST` receives `SHIPPER` for that company.
- One active `CARRIER` membership with `CARRIER_ADMIN`, `CARRIER_DISPATCHER`, or `CARRIER_ACCOUNTANT` receives `CARRIER` for that company.
- No qualifying company, more than one qualifying company, or two types for the same company returns 403 `analytics company scope is not available`. The gateway does not choose the first company.
- `PROCUREMENT_MANAGER`, `FORWARDER_MANAGER`, `FINANCE_MANAGER`, `DRIVER`, and any other role without a qualifying company membership fail closed.
- Authentication disabled fails closed. An empty user id is 401.

`X-Company-ID`, `X-Analytics-Scope`, `X-Analytics-Company-ID`, and query company identifiers supplied by the client are not authority. The gateway sets `X-Analytics-Scope` and, for a company scope, `X-Analytics-Company-ID` on its own request to analytics-service. Analytics-service sets the same headers on its request to shipment-service after the dedicated service tokens are accepted. A missing or invalid scope is 403. `TENANT` together with a company id is 403.

Shipment SQL applies the tenant and the one company predicate. Carrier scope uses `carrier_company_id`. Shipper scope uses `shipper_company_id`. Another tenant's rows are excluded by `tenant_id`.

## Return and redirect

`transport.delivery_disposition_cases` has `operating_tenant_id`, `shipment_id`, `shipment_tenant_id`, `cargo_id`, and `disposition_type`. It has no shipper or carrier company column. The current `transport.shipments.carrier_company_id` is not a case-time company fact. That link is unverified.

Company-scoped reads do not query `delivery_disposition_cases`. The v2 source marks `dispositionScope=UNAVAILABLE` and returns zero case counts only as the absence of a measurement. Analytics denies `OPS_RETURN_CASES` and `OPS_REDIRECT_CASES` for `SHIPPER` and `CARRIER` with 403 and no KPI value. A company request that arrives with `dispositionScope=TENANT`, or a tenant request that arrives without it, is 503. The v1 source contract stays the five-field tenant payload. A company scope on v1 is 403 before the reader runs.

`PLATFORM_ADMIN` keeps the existing tenant query, including the real case counts, and `dispositionScope=TENANT`.

## Unchanged KPI contract

The eleven KPI ids stay. `definitionVersion=1` stays. Punctuality is still `actual <= planned` with grace 0. A ratio with denominator 0 still has `value=null`. `dataFreshness.status` stays `UNKNOWN`. The three count KPIs stay `COMPLETE` where a company population exists. The eight punctuality KPIs stay `PARTIAL`. Company scope changes which shipment rows enter the population. It does not change the comparison.

`OPS_RETURN_CASES` and `OPS_REDIRECT_CASES` remain available only for tenant scope. They are not given a company formula.

GAP-C-001, GAP-C-002, GAP-C-005, GAP-B-001, and GAP-E-001 stay open. `CAR_PERFORMANCE_SCORE` stays unpublished. `DWH_IMPLEMENTED=NO`. No staging or production deploy is part of this change.

## Review gate

This document does not record an independent security review. That review is still required before merge. Merge and deploy are not authorized.
