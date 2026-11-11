# Analytics source readiness 0.4A

Discovery only. This note records whether the current Analytics read can use real Core MVP facts. It does not add a KPI, change a formula, change a service, or change a database schema.

```
STAGE=ANALYTICS-SOURCE-READINESS-0.4A
BASE_SHA=3f306fc407aeda4bdc652d825954292f4a54fad6
MODE=READ_ONLY_DISCOVERY
DOCS_ONLY=YES
RUNTIME_CHANGED=NO
SCHEMA_CHANGED=NO
LIVE_DATA_MEASURED=NO
MOCK_DATA_IS_PRODUCTION_EVIDENCE=NO
KPI_COUNT=11
DEFINITION_VERSION=1
FINAL_VERDICT=ANALYTICS_READINESS_ASSESSED
```

Inventory SHA is `origin/main` `3f306fc407aeda4bdc652d825954292f4a54fad6` (merge of PR #348). `git diff --stat 1902a2b4a77e9f2dcf823dae34f85328e8903117..3f306fc407aeda4bdc652d825954292f4a54fad6` does not touch `services/analytics-service`, the operations-foundation repository, or `docs/architecture/analytics`. The caller-identity contract merged in PR #336 is still the code under inspection. No live tenant was queried, and no KPI number in this note is a measured production value.

Core MVP readiness (`docs/reports/BINTRANS_CORE_MVP_READINESS_2026_11.md`) keeps analytics KPI in the full platform, outside the Core MVP path. The shipment rows that the eleven KPIs read can be written by TMS. RFx, tracking, and Control Tower are Core MVP concerns, and none of them is an input to these eleven KPIs.

## Classes

| Class | Meaning in this note |
| --- | --- |
| VERIFIED | The endpoint exists and reads the server-owned current-state fact named by the frozen definition. Freshness is still `UNKNOWN`. This is not a live measurement. |
| PARTIAL | The endpoint exists, and the fact is a proxy or the population omits missing events without an explicit unknown row. |
| MOCK_ONLY | The only evidence is demo or browser fixture data. |
| NOT_IMPLEMENTED | No analytics endpoint computes the indicator. |

`completeness=COMPLETE` on three count KPIs is the response field from `kpi.go`. It is not the class `VERIFIED`. `completeness=PARTIAL` on the other eight is the response field and matches the class `PARTIAL`.

## Endpoints

| Hop | Service | Method and path | Role |
| --- | --- | --- | --- |
| Public | api-gateway | `GET /api/v1/analytics/kpis/{kpiId}` | JWT role check, then forward. Does not compute. |
| Compute | analytics-service | `GET /v1/analytics/kpis/{kpiId}` | Maps one snapshot to one KPI. Stateless. No database. |
| Source | shipment-service | `GET /internal/v1/analytics/operations-foundation` | Five current-state totals. |
| Source | shipment-service | `GET /internal/v1/analytics/operations-foundation-v2` | Those totals plus pickup counts and carrier rows. |

Any query string on the analytics route is `400` before the source fetch. Unknown `kpiId` is `404`. Analytics is not a Compose service. The two caller tokens have no Compose default. This discovery did not start Compose and did not read a live environment.

## Implemented KPI matrix

`definitionVersion` is `1` for every row. `dataFreshness.status` is `UNKNOWN` for every row. A ratio with denominator `0` keeps numerator `0`, denominator `0`, and `value=null`.

| KPI | Class | Source fact | Response completeness |
| --- | --- | --- | --- |
| OPS_SHIPMENTS_TOTAL | VERIFIED | `count(*)` of `transport.shipments` for `tenant_id=$1` and `deleted_at IS NULL` | COMPLETE |
| OPS_RETURN_CASES | VERIFIED | `count(*)` of `transport.delivery_disposition_cases` with `disposition_type=RETURN_TO_ORIGIN` and `operating_tenant_id=$1` | COMPLETE |
| OPS_REDIRECT_CASES | VERIFIED | Same case table, `disposition_type=REDIRECT` | COMPLETE |
| OPS_ON_TIME_PICKUP | PARTIAL | Numerator of pickup comparison below | PARTIAL |
| OPS_ON_TIME_PICKUP_RATE | PARTIAL | Pickup numerator / pickup denominator | PARTIAL |
| OPS_LATE_PICKUP | PARTIAL | Pickup denominator minus numerator, rejected if that would be negative | PARTIAL |
| OPS_ON_TIME_DELIVERY | PARTIAL | Numerator of delivery comparison below | PARTIAL |
| OPS_ON_TIME_DELIVERY_RATE | PARTIAL | Delivery numerator / delivery denominator | PARTIAL |
| OPS_LATE_DELIVERY | PARTIAL | Delivery denominator minus numerator | PARTIAL |
| CAR_ON_TIME_PICKUP_RATE | PARTIAL | One ratio per current `carrier_company_id`, `dimension=CARRIER` | PARTIAL |
| CAR_ON_TIME_DELIVERY_RATE | PARTIAL | Same carrier grain for delivery | PARTIAL |

Pickup population: both `planned_pickup_at` and `actual_pickup_at` are non-null. On time means `actual_pickup_at <= planned_pickup_at`. Delivery uses `planned_delivery_at` and `actual_delivery_at` the same way. Grace is zero.

`transitionShipment` writes `actual_pickup_at` from command `OccurredAt` when status becomes `LOADED`, and `actual_delivery_at` from `OccurredAt` when status becomes `DELIVERED`. `COALESCE` keeps the first stored timestamp. That command time is not a tracking observation and not a stop `arrived_at`.

Carrier rows are every non-deleted shipment in the tenant with `carrier_company_id IS NOT NULL`, grouped by that current pointer. The caller's company is not a predicate.

No implemented KPI is `MOCK_ONLY`. Browser mock shipments in portal e2e specs are not evidence for these endpoints.

## Sources that do not feed the eleven KPIs

### TMS

TMS execution owns the shipment row, the disposition case, and the command that stamps actual timestamps. That is the only TMS input used. Stop-level `arrived_at` / `completed_at`, delivery quantity facts, and assignment decisions are not read. GAP-C-001, GAP-C-002, and GAP-C-005 stay unimplemented. `docs/architecture/tms/TMS_ANALYTICS_SOURCE_CONTRACTS_0_4B.md` requests those facts and does not expose them.

`docs/architecture/tms/TMS_STOP_EVENT_TRACKING_CONTRACT_0_9B.md` proves, in a local test, that shipment execution outbox bytes can be passed to tracking `Apply`. Tracking deduplicates on `payload.event_id`. Those events do not update `actual_pickup_at` or `actual_delivery_at`, and analytics does not read them.

### RFx

`rfx-service` has tenant-scoped events, responses, and awards. `services/analytics-service` does not call RFx. An award does not change any of the eleven KPIs. Procurement analytics and a savings KPI are `NOT_IMPLEMENTED`.

### Tracking

`tracking-service` stores location events with `dedup_key` and `ON CONFLICT DO NOTHING`, plus its own freshness (`unknown`, `fresh`, `stale`, `lost`). Analytics does not read those tables. A position or ETA does not change on-time pickup or delivery. The Core MVP report records Issue #344 as open for the tracking consumer. This discovery did not re-check that issue. Tracking-derived punctuality is `NOT_IMPLEMENTED`.

### Control Tower

ADR-AN-005 keeps Control Tower as an operational read model. `GET /api/v1/control-tower/summary` computes `kpi.onTime` with `sla.Compute`, which can mark a row on time while it is not a completed delivery. That value is not `OPS_ON_TIME_DELIVERY` and not `OPS_ON_TIME_DELIVERY_RATE`. Status projection counts do not compute punctuality. `apps/web-admin` demo and fallback KPI cards are `MOCK_ONLY` presentation. They are not analytics responses. No frontend in `apps/` calls `/api/v1/analytics/kpis`.

## Identity and cross-tenant aggregation

Public tenant comes from `routeauth.BuildRequestContext` on the gateway. Analytics copies that tenant into `X-Tenant-ID` on the source call and sends `X-Internal-Service-Name=analytics-service` with `ANALYTICS_SOURCE_CALLER_TOKEN`. Shipment `resolveVerifiedTenant` reads that header and passes one UUID into the statement. Query `tenant_id` is not authority: analytics rejects any query, and the source handler does not read the query.

The statement has one tenant parameter. Shipment counts use `tenant_id=$1`. Case counts use `operating_tenant_id=$1`. Carrier groups use the same `tenant_id=$1`. There is no cross-tenant `GROUP BY` and no union of tenants.

```
CLIENT_SUPPLIED_TENANT_AUTHORITY=NO
CROSS_TENANT_AGGREGATION=NO
CROSS_TENANT_AGGREGATION_SCOPE=operations-foundation statement only
COMPANY_OF_CALLER_FILTER=NO
COMPANY_LEVEL_ISOLATION_VERIFIED=NO
CARRIER_DIMENSION=CURRENT_SHIPMENT_POINTER
```

The Core MVP report still records `SHIPMENT_TRUSTS_GATEWAY_TENANT_HEADER` as open for shipment reads in general. This note does not close that boundary. It only describes the analytics path above. A carrier role that can call the public KPI route receives every carrier in the tenant, not only the caller's company.

Dedicated caller tokens stay as in `ANALYTICS_S2S_CALLER_IDENTITY_0_3D.md`. Empty, shared, and mismatched caller names fail closed. This discovery did not provision secrets and did not re-run those tests.

## Freshness, deduplication, missing events, completeness

Freshness and completeness are different fields. Every implemented response sets `dataFreshness.status=UNKNOWN`. There is no `sourceObservedAt`, no age, and no watermark. Tracking freshness and Control Tower `generatedAt` are not copied.

Deduplication of the analytics read is current-row counting, not an event fold. A shipment is one row. A return or redirect case is one case row, not one shipment. Replaying a transition does not move an actual timestamp after the first `COALESCE` write. Command idempotency lives on the TMS write path. Tracking `dedup_key` and `payload.event_id` do not affect these KPIs, because those events are not inputs.

Missing actual or planned time removes the shipment from the punctuality denominator. The shipment still counts in `OPS_SHIPMENTS_TOTAL`. The response does not list the omitted shipments and does not mark that omission as a per-row `UNKNOWN`. A zero denominator is the only implemented null ratio, and it is a null value with zero numerator, not a zero rate. Unknown in-full, POD, and assignment facts are not coerced to false or zero, because those facts are not read.

`PARTIAL` must not be presented as complete. `UNKNOWN` freshness must not be presented as fresh. There is no as-of or period filter. Historical replay is not available.

## Not implemented

These stay `NOT_IMPLEMENTED`. This stage does not promote them.

| Indicator | Blocker |
| --- | --- |
| OPS_OTIF, CAR_OTIF, EXEC_OTIF | GAP-C-001. On-time is not in-full. `DELIVERED` and a missing case are not in-full. |
| POD completeness | GAP-C-002. No server-owned POD requirement. A document row is not satisfaction. |
| Carrier rejection rate | GAP-C-005. No stable offer and accept/reject decision. |
| Document completeness | GAP-B-001. Contract requested, not implemented. |
| CAR_PERFORMANCE_SCORE, EXEC_CARRIER_PERFORMANCE | GAP-E-001 open. Questions not answered. Score is not published. |
| RFx cycle time, savings, award rate | No analytics source client. |
| Tracking ETA, dwell, geofence punctuality | No analytics source client. |
| Control Tower active, on time, risk, exception SLA | Different operational formulas. ADR-AN-005. |

`CAR_PERFORMANCE_SCORE` is not included. Eight punctuality KPIs stay partial.

## Technical pilot scenarios

These are the first technical checks. They were not executed against a live stand in this stage. A passing local chain test from the caller-identity work is not a measurement of Core MVP data. Do not use mock browser shipments as the fixture.

1. Tenant A has shipments and tenant B has none. `OPS_SHIPMENTS_TOTAL` for A counts only A. The same call for B does not include A's rows.
2. A client `X-Tenant-ID` and a query `tenant_id` for the other tenant do not change the JWT tenant. Analytics query form returns `400` and no `value`.
3. The shared internal token and a wrong caller name are rejected. No source row is read for those calls.
4. One shipment with both pickup timestamps and `actual <= planned` increments the pickup numerator and denominator. One with `actual > planned` increments only the denominator and therefore late pickup.
5. One shipment with a null actual timestamp increments `OPS_SHIPMENTS_TOTAL` and neither punctuality numerator nor denominator. The response stays `completeness=PARTIAL` and `dataFreshness.status=UNKNOWN`.
6. A tenant with no comparable deliveries returns a null rate, numerator `0`, denominator `0`.
7. One `RETURN_TO_ORIGIN` case and one `REDIRECT` case increment different KPIs.
8. Two carriers in the same tenant both appear in `CAR_ON_TIME_PICKUP_RATE`. The caller's company does not remove the other carrier. This is the current contract, and it is not a pass for company isolation.
9. A tracking position or ETA, an RFx award, and a Control Tower summary refresh leave all eleven responses unchanged.
10. Compare Control Tower `kpi.onTime` with `OPS_ON_TIME_DELIVERY_RATE` only to show they differ. Do not reconcile them in the client.

Stop the technical pilot if any scenario returns another tenant's count, if `PARTIAL` or `UNKNOWN` is hidden, or if a mock fixture is the only data behind a reported number.

## Missing sources and dependencies

| Dependency | State |
| --- | --- |
| GAP-C-001 in-full quantity | OPEN. Contract requested. Not derivable from current rows. |
| GAP-C-002 POD requirement and satisfaction | OPEN. Needs TMS requirement plus document evidence. |
| GAP-C-005 assignment decision | OPEN. RFx award is not that decision row. |
| GAP-B-001 document completeness | OPEN. Owner remains documents. |
| GAP-E-001 carrier score policy | OPEN. |
| Tracking consumer | Core MVP report records Issue #344 open. Not an analytics input even after that issue closes. |
| POD write boundary | Core MVP report records Issue #343. Not re-checked here. |
| Company authorization | Core MVP report records company authorization as not verified. Analytics carrier rows are tenant-wide. |
| Shipment trust of gateway tenant header | Open in the Core MVP report for shipment reads generally. |
| Caller tokens on a real process | Not provisioned by this stage. Analytics is absent from Compose. |
| Analytics UI | Not implemented. Do not add it in this stage. |
| Warehouse, period, as-of | `DWH_IMPLEMENTED=NO`. `HISTORICAL_ANALYTICS_READY=NO`. |

```
COMMERCIAL_PILOT_READY=NO
CORE_MVP_ANALYTICS_PATH=OUT_OF_CORE_SCOPE
REAL_SOURCE_FOR_THREE_COUNTS=CURRENT_STATE_SQL
REAL_SOURCE_FOR_EIGHT_PUNCTUALITY_KPIS=COMMAND_TIME_PROXY
RFX_EVENTS_CONSUMED=NO
TRACKING_EVENTS_CONSUMED=NO
CONTROL_TOWER_EVENTS_CONSUMED=NO
```
