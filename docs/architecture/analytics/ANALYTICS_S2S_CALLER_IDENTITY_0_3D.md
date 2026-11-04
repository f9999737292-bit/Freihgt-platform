# Analytics service-to-service caller identity 0.3D

STAGE=ANALYTICS-S2S-CALLER-IDENTITY-0.3D

This note is the rotation and deployment order. It does not contain secret values.

## Contract

Two hops use separate credentials:

| Hop | Holder | Checker | Environment variable |
| --- | --- | --- | --- |
| api-gateway → analytics-service | api-gateway | analytics-service | `ANALYTICS_GATEWAY_CALLER_TOKEN` |
| analytics-service → shipment-service | analytics-service | shipment-service, only `GET /internal/v1/analytics/operations-foundation` and `GET /internal/v1/analytics/operations-foundation-v2` | `ANALYTICS_SOURCE_CALLER_TOKEN` |

`X-Internal-Service-Name` must still match the caller selected by the secret (`api-gateway` or `analytics-service`). The header is not a credential. `INTERNAL_SERVICE_TOKEN` does not authorize either hop. Other shipment-service internal routes keep the shared token.

Both values must be nonempty. They must differ from each other and from `INTERNAL_SERVICE_TOKEN`. Startup fails closed when that is not true. Comparison on the request path is constant-time. Errors and logs must not contain the values.

There is no period in which the old shared token is also accepted.

Public route `GET /api/v1/analytics/kpis/{kpiId}` is unchanged. KPI formulas, `definitionVersion=1`, null and zero rules, sources, and response shape are unchanged.

## Rotation order

1. Generate two new secrets outside git and outside this repository. Do not write them into a committed env file, a staging template, or a log.
2. Confirm the three values are pairwise distinct: gateway caller, source caller, and the existing `INTERNAL_SERVICE_TOKEN`.
3. Place `ANALYTICS_GATEWAY_CALLER_TOKEN` and `ANALYTICS_SOURCE_CALLER_TOKEN` in the protected environment used by the process manager. Do not change a live staging or production host as part of this change set.
4. Deploy shipment-service, analytics-service, and api-gateway in the same window, after both values are present. Analytics-service needs both variables. Shipment-service needs the source variable. Api-gateway needs the gateway variable.
5. Until that window completes, the published KPI route fails closed. That is the intended gap. Do not restore the shared token as a fallback.
6. A later rotation repeats the same window. Do not overlap an old dedicated secret with a new one.

`infrastructure/docker-compose/docker-compose.yml` passes the two variables through with no default. `scripts/ops/bintrans_ct_staging/staging.env.example` lists the keys blank. The staging compose override and the protected staging env file are not given values by this change.

## Out of scope

Autopilot, platform migrations, `CAR_PERFORMANCE_SCORE`, Q1–Q14, and live staging or production hosts are unchanged.
