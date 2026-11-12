# Analytics company scope 0.4B

Discovery only. This note records who can call the eleven KPIs and where company scope is missing. It does not change a formula, a route, a SQL statement, or a schema. PR #354 is not modified.

```
STAGE=ANALYTICS-COMPANY-SCOPE-DISCOVERY-0.4B
BASE_SHA=3f306fc407aeda4bdc652d825954292f4a54fad6
PREVIOUS_PR=354
MODE=READ_ONLY_DISCOVERY
DOCS_ONLY=YES
RUNTIME_CHANGED=NO
SQL_CHANGED=NO
SCHEMA_CHANGED=NO
KPI_FORMULA_CHANGED=NO
KPI_COUNT=11
DEFINITION_VERSION=1
VERIFIED_SOURCE_MAPPINGS=3
PARTIAL_PUNCTUALITY=8
DATA_FRESHNESS=UNKNOWN
COMPANY_LEVEL_ISOLATION_VERIFIED=NO
CLIENT_COMPANY_ID_AUTHORITY=NO
FINAL_VERDICT=COMPANY_SCOPE_ASSESSED/POLICY_DECISION_REQUIRED
```

The classification from the source-readiness assessment stays:

| Class | KPIs |
| --- | --- |
| VERIFIED | `OPS_SHIPMENTS_TOTAL`, `OPS_RETURN_CASES`, `OPS_REDIRECT_CASES` |
| PARTIAL | `OPS_ON_TIME_PICKUP`, `OPS_ON_TIME_PICKUP_RATE`, `OPS_LATE_PICKUP`, `OPS_ON_TIME_DELIVERY`, `OPS_ON_TIME_DELIVERY_RATE`, `OPS_LATE_DELIVERY`, `CAR_ON_TIME_PICKUP_RATE`, `CAR_ON_TIME_DELIVERY_RATE` |

`dataFreshness.status` remains `UNKNOWN` on every implemented response. There is no `sourceObservedAt` and no watermark. `UNKNOWN` is not fresh. This stage does not relabel it `FRESH` or `STALE`.

`PARTIAL` stays partial. Company scope does not promote a punctuality KPI to `VERIFIED`.

## One route, one role list

All eleven KPIs use `GET /api/v1/analytics/kpis/{kpiId}` on api-gateway. `analytics.Handler.Get` loads roles from identity `GET /v1/auth/me` and calls `analyticsrbac.AllowRead`. The allow list is not split by KPI.

```
PLATFORM_ADMIN
PROCUREMENT_MANAGER
SHIPPER_ADMIN
SHIPPER_LOGIST
FORWARDER_MANAGER
FINANCE_MANAGER
CARRIER_ADMIN
CARRIER_DISPATCHER
CARRIER_ACCOUNTANT
```

`DRIVER` is absent. `TestAnalyticsGatewayAccessAndSpoofing` expects `403` for `DRIVER` and does not call analytics. The same test expects `200` for every role in the list above, with the downstream tenant taken from the JWT.

The gateway client sets `X-Tenant-ID` from that JWT tenant. It does not send `X-Company-ID`, `X-User-ID`, or `Authorization` downstream. Analytics rejects any query string before the source fetch. Shipment `operations-foundation` filters by that one tenant id. Carrier rows are every non-deleted shipment in the tenant with `carrier_company_id IS NOT NULL`, grouped by the current carrier pointer. `shipper_company_id` is not a predicate.

`/v1/auth/me` roles are tenant-user roles. They are not a company membership on the analytics route. `companycontext.RequireCarrierRead` and `RequireShipperRead` are not mounted on this route. Those gates still take a client company id and then require a matching membership. Analytics must not copy that selector.

## Role matrix

Current scope is the same payload for every allowed role: the JWT tenant, all shipments in that tenant, and all carrier rows in that tenant.

| Role | May call each of the 11 KPIs today | Company filter today | What the payload can reveal |
| --- | --- | --- | --- |
| SHIPPER_ADMIN | YES | none | Tenant operational totals and every carrier rate in the tenant. Another shipper company in the same tenant is included in the totals. |
| SHIPPER_LOGIST | YES | none | Same payload as `SHIPPER_ADMIN`. |
| CARRIER_DISPATCHER | YES | none | Same payload, including punctuality rows for other `carrierCompanyId` values. |
| PLATFORM_ADMIN | YES | none inside the JWT tenant | Same tenant payload. A second tenant is not selected. ADR-AN-004 still forbids a missing tenant filter as a platform report. |

`CARRIER_ADMIN` and `CARRIER_ACCOUNTANT` have the same current access as `CARRIER_DISPATCHER`. `PROCUREMENT_MANAGER`, `FORWARDER_MANAGER`, and `FINANCE_MANAGER` have the same current access as `SHIPPER_ADMIN`. Those roles are outside the four-row matrix and are not a separate company scope.

## Operational tenant data and commercial company data

| Data | Grain today | Class of disclosure |
| --- | --- | --- |
| Shipment count, return cases, redirect cases | Tenant | Operational. Still company-sensitive when more than one customer company shares the tenant, because the count is not limited to the caller's shipper company. |
| Eight punctuality KPIs | Tenant population, command-time actuals | Operational and `PARTIAL`. The same tenant-wide population applies. |
| `CAR_ON_TIME_PICKUP_RATE`, `CAR_ON_TIME_DELIVERY_RATE` | One ratio per current `carrier_company_id` | Commercial comparison inside the tenant. A carrier caller receives other carriers' aggregates. |
| Prices, awards, margins, carrier score | Not implemented | Not a source of company scope. `CAR_PERFORMANCE_SCORE` stays unpublished. |

RFx, tracking, and Control Tower remain outside these eleven KPIs. Control Tower `kpi.onTime` is a different formula and is not a company-scoped substitute.

## Disclosure risks

```
OTHER_CARRIER_AGGREGATE_VISIBLE_TO_CARRIER_ROLE=YES
OTHER_SHIPPER_SHIPMENTS_INCLUDED_IN_TENANT_TOTAL=YES
CLIENT_COMPANY_HEADER_IGNORED=YES
CROSS_TENANT_AGGREGATION=NO
```

1. `CARRIER_DISPATCHER` can read `CAR_ON_TIME_PICKUP_RATE` and `CAR_ON_TIME_DELIVERY_RATE` for every carrier in the tenant. That is another carrier's punctuality, not the caller's own company metric.
2. The same caller can read tenant totals. Those totals include shipments assigned to other carriers and, where several customer companies share a tenant, shipments of other shipper companies.
3. A role granted on company A is sufficient for the tenant payload. The analytics route does not check that the role sits on the company being viewed.
4. `X-Company-ID`, `company_id`, `shipper_company_id`, and `carrier_company_id` do not select the analytics scope. A client cannot use them to narrow the response, and a future implementation must not use them as authority.
5. Cross-tenant disclosure is a different control. The SQL parameter is one tenant, and the public route uses the JWT tenant. This discovery does not reopen that control and does not call company isolation verified.

## Policy decision

The code answers the current behavior: a carrier role sees every carrier in the tenant.

The business choice is open.

| Option | Carrier role sees | Shipper role sees | Status |
| --- | --- | --- | --- |
| A | Only the server-resolved own company row. Tenant totals that mix other companies are withheld. | Tenant operational totals for the server-resolved shipper company, plus carrier rows for shipments of that company. | Proposed safe default. Not implemented. |
| B | Every carrier row in the tenant. | Every shipment and carrier row in the tenant. | Current code. Discloses other carriers' aggregates. |

```
POLICY_DECISION=REQUIRED
RECOMMENDED_UNTIL_DECISION=OPTION_A
IMPLEMENTATION_AUTHORIZED=NO
```

Option A is the recommendation because option B publishes other carriers' aggregates to `CARRIER_DISPATCHER`. The controller has to record the choice before any runtime change. This document does not make option A the live rule.

`PLATFORM_ADMIN` remains a tenant operator under ADR-AN-004. The decision must say whether that role may see every carrier row inside the JWT tenant. It does not grant a second tenant.

## Server-owned company scope

Proposed contract, not implemented:

```
COMPANY_SCOPE_OWNER=API_GATEWAY
COMPANY_SCOPE_SOURCE=AUTHENTICATED_MEMBERSHIP
CLIENT_COMPANY_ID_AUTHORITY=NO
QUERY_COMPANY_SELECTOR=REJECT
HEADER_COMPANY_SELECTOR=IGNORE
DOWNSTREAM_COMPANY_HEADER=SERVER_RESOLVED_ONLY
```

The gateway resolves company memberships for the authenticated user inside the JWT tenant. The KPI request does not carry a company id that the server treats as the choice.

| Memberships of the required company type | Result |
| --- | --- |
| none | `403`. No KPI body and no source read. |
| one | That company id is the scope. |
| more than one | Fail closed. The KPI route does not accept a client id to disambiguate. |

A carrier scope uses a membership whose company type is `CARRIER` and whose roles include the carrier role being exercised. A shipper scope uses a membership whose company type is `SHIPPER` and whose roles include the shipper role being exercised. A role on another company does not qualify. `DRIVER` stays denied.

Downstream, analytics and shipment receive the server-resolved company only after that resolution. They do not trust a client `company_id`. Until this contract is implemented, the current tenant-wide SQL remains the live behavior.

`PLATFORM_ADMIN` does not take a company id from the client. Tenant scope stays the JWT tenant.

## Negative tests

These checks are specified here and are not added as code in this stage. A test that encoded option A now would fail against the current route. A test that only documents today's leak must not be described as a pass for isolation.

| Case | Request | Current behavior | Required after the policy is implemented |
| --- | --- | --- | --- |
| Cross-tenant header | JWT tenant A, client `X-Tenant-ID` tenant B | Downstream tenant is A. Already covered by the gateway spoof test. | Remains A. No row from B. |
| Cross-tenant query | `?tenant_id=` of tenant B | Analytics `400` before the source fetch. No `value`. | Remains `400`. No `value`. |
| Cross-company carrier | Membership only for carrier A, KPI `CAR_ON_TIME_DELIVERY_RATE` | `200` with every carrier row in the tenant, including carrier B. | `200` contains only carrier A, or `403` when the membership cannot be resolved. No carrier B `carrierCompanyId`. |
| Client company selector | `X-Company-ID` or `carrier_company_id` of carrier B while the only membership is A | Header is not forwarded. Any query is `400`. | Still not authority. B is not selected. |
| Foreign shipper in the same tenant | `SHIPPER_ADMIN` membership only for shipper A | Tenant totals include other shipper companies' shipments. | Totals include only the server-resolved shipper company. |
| Role on another company | `CARRIER_DISPATCHER` on company A, no membership on B | The tenant payload is returned. | B is not readable. |
| DRIVER | `DRIVER` | `403`. Analytics is not called. | Unchanged. |
| Platform cross-tenant | `PLATFORM_ADMIN` of tenant A | Tenant A only. | Tenant A only. |
| Freshness label | Any allowed role | `dataFreshness.status=UNKNOWN`. | The assertion fails if the body says `FRESH` or `STALE` without `sourceObservedAt` and a watermark. |
| Shared token and wrong caller | Dedicated caller contract | Rejected. No source row. | Unchanged. |

`UNAUTHORIZED_DATA_RETURNED` for another tenant is not the current result. `UNAUTHORIZED_DATA_RETURNED` for another carrier company is the current result for an allowed carrier role.

## Unchanged boundaries

```
LIVE_DATA_MEASURED=NO
COMMERCIAL_PILOT_READY=NO
CAR_PERFORMANCE_SCORE_PUBLISHED=NO
DATA_FRESHNESS_POLICY_UNCHANGED=YES
GAP-C-001=OPEN
GAP-C-002=OPEN
GAP-C-005=OPEN
GAP-B-001=OPEN
GAP-E-001=OPEN
```

No UI is added. No new KPI is added. Eight partial KPIs are not called complete.
