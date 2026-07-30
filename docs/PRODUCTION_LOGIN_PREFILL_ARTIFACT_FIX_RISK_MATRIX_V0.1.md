# Production Login Prefill Artifact Fix Risk Matrix v0.1

## Summary

Risk matrix for fixing production login credential prefill caused by an older production web-admin artifact.

## Risks

| Risk | Severity | Mitigation |
|---|---|---|
| Production deploy changes more than login behavior | high | require production readiness scope and artifact diff review |
| Production root overwritten incorrectly | high | backup production root before execution; deploy only approved artifact |
| Production outage after artifact refresh | high | pre/post endpoint checks and rollback path |
| Staging/prod drift remains | medium | prefer QA-signed staging artifact promotion |
| Hidden dev-only behavior remains in production | high | verify login fields empty after deploy |
| Incorrect rollback restores prefill | medium | document rollback trade-off before execution |
| Nginx changed unnecessarily | medium/high | no Nginx changes required for artifact fix |
| Secrets captured in docs | high | do not record credentials/tokens/screenshots with secrets |
| Production deploy without approval | high | require explicit execution approval |

## Guardrails

```text
1. No production change without explicit production execution approval.
2. No Nginx/DNS/Certbot changes.
3. No backend/API/migration/database changes.
4. Production root backup required before execution.
5. Verify production /, /login, /health before and after.
6. Verify staging remains healthy.
7. Do not record real credentials or tokens.
```

## Decision

```text
PRODUCTION_LOGIN_PREFILL_ARTIFACT_FIX_RISK_MATRIX_CREATED
```
