# Live Data Demo Workflow Staging API Smoke Evidence v0.1

## Summary

Staging API smoke evidence for live-data demo workflow.

Base: `https://staging.xn--80abvubqje.xn--p1ai/api/v1`

## Result

```text
LIVE_DATA_DEMO_WORKFLOW_STAGING_API_SMOKE_PASS
```

## API Safety

| Check                         | Result |
| ----------------------------- | ------ |
| staging API used              | yes    |
| production API writes avoided | yes    |
| tokens recorded               | no     |
| passwords recorded            | no     |
| no production data exposure   | yes    |

## API Families

All probes used authenticated login per role with `tenant_id` query parameter. Recorded values are HTTP status and list count only.

| API Family                       | PLATFORM_ADMIN | SHIPPER_ADMIN | CARRIER_ADMIN | FINANCE_MANAGER | Result |
| -------------------------------- | -------------- | ------------- | ------------- | --------------- | ------ |
| /companies                       | 200 / 8        | 200 / 8       | 200 / 8       | 200 / 8         | pass   |
| /freight-requests                | 200 / 3        | 200 / 3       | 200 / 3       | 200 / 3         | pass   |
| /rfx-events                      | 200 / 1        | 200 / 1       | 200 / 1       | 200 / 1         | pass   |
| /transport-orders                | 200 / 5        | 200 / 5       | 200 / 5       | 200 / 5         | pass   |
| /shipments                       | 200 / 3        | 200 / 3       | 200 / 3       | 200 / 3         | pass   |
| /documents                       | 200 / 1        | 200 / 1       | 200 / 1       | 200 / 1         | pass   |
| /billing-registers               | 200 / 1        | 200 / 1       | 200 / 1       | 200 / 1         | pass   |

Demo markers (DEMO-prefixed strings) observed in list payloads for seeded entities. No real customer data observed.

Unauthenticated baseline: `GET /api/v1/companies?tenant_id=<tenant>` without bearer returned HTTP 400 (route exists, auth/tenant required).

## Notes

```text
401/403 may be PASS if expected by role permissions.
5xx on core demo routes is fail unless documented as known limitation.
Staging gateway AUTH_ENABLED=false during smoke; all four roles received HTTP 200 on all probed families. Role differentiation not validated at API layer in this run.
```
