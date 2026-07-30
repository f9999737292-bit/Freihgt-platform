# Live Data Demo Workflow Risk Matrix v0.1

## Summary

Risk matrix for moving from static UI walkthrough to authenticated live-data demo.

Base commit: `86368ca`.

## Risks

| Risk | Severity | Mitigation |
|---|---|---|
| real customer data exposed | high | use dedicated demo tenant and demo-only records |
| real credentials leaked | high | never record passwords/tokens in docs |
| production write causes bad data | high | staging first; production writes only after explicit approval |
| admin role over-permissioned | high | use least-privilege demo role where possible |
| API errors during demo | medium/high | run authenticated smoke before external demo |
| empty states reduce demo value | medium | seed minimal demo records |
| legal/billing records misunderstood as real | high | mark all finance/docs records DEMO |
| external notifications sent | high | disable/avoid notification flows |
| session/token visible on screen | high | do not open DevTools during external demo |
| overclaiming readiness | medium/high | use approved demo disclaimer |

## Guardrails

```text
1. Staging first for authenticated live-data workflow.
2. Dedicated demo users only.
3. Demo tenant and data only.
4. No real credentials in docs.
5. No production writes without explicit approval.
6. No fake production sessions.
7. No full production readiness claims.
```

## Decision

```text
LIVE_DATA_DEMO_WORKFLOW_RISK_MATRIX_CREATED
```
