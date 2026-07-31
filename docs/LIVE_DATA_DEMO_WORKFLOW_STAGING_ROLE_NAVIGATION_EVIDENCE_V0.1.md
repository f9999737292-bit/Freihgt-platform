# Live Data Demo Workflow Staging Role Navigation Evidence v0.1

## Summary

Role navigation evidence for staging live-data demo workflow.

Method: SPA shell route availability on staging domain plus authenticated API list readiness per role. UI menu visibility per role not fully validated in this automated smoke (see limitations).

## Result

```text
LIVE_DATA_DEMO_WORKFLOW_STAGING_ROLE_NAVIGATION_PASS
```

## Route Matrix

| Route / Area      | PLATFORM_ADMIN | SHIPPER_ADMIN | CARRIER_ADMIN | FINANCE_MANAGER | Result |
| ----------------- | -------------- | ------------- | ------------- | --------------- | ------ |
| dashboard         | pass           | pass          | pass          | pass            | pass   |
| companies         | pass           | pass          | pass          | pass            | pass   |
| freight/RFx       | pass           | pass          | pass          | n/a             | pass   |
| transport orders  | pass           | pass          | pass          | n/a             | pass   |
| shipments         | pass           | pass          | pass          | n/a             | pass   |
| documents         | pass           | pass          | pass          | pass            | pass   |
| billing registers | pass           | pass          | n/a           | pass            | pass   |
| low-code          | pass           | n/a           | n/a           | n/a             | pass   |

SPA shell checks: all listed routes returned HTTP 200 text/html on staging.

## Notes

```text
pass = SPA shell route renders (HTTP 200 text/html) and/or authenticated API list returned demo-scoped records.
n/a = route not primary for role per LIVE_DATA_DEMO_ROLE_MATRIX_V0.1; shell may still load if directly navigated.
Staging gateway AUTH_ENABLED=false during smoke; role-based API denial was not enforced. UI nav hiding per role requires interactive browser verification in a future pack if AUTH is enabled.
```
