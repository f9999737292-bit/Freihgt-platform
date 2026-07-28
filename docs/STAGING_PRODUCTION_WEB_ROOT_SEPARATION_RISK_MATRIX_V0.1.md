# Staging / Production Web Root Separation Risk Matrix v0.1

## Summary

Risk matrix for separating staging and production web roots.

## Risks

| Risk | Severity | Mitigation |
|---|---|---|
| Staging root update accidentally changes production | high | production root must remain unchanged; edit staging vhost only |
| Wrong Nginx vhost edited | high | identify exact staging config (`staging-bintrans.conf`) before any change |
| Nginx syntax error causes service issue | high | run `nginx -t` before reload |
| Production affected by Nginx reload | medium/high | reload only after syntax pass; verify production immediately |
| Staging content missing after root switch | medium | copy current web root to new staging root before switching |
| File ownership wrong | medium | set www-data ownership if applicable |
| Certbot config affected | high | do not run Certbot; do not edit SSL cert paths |
| DNS affected | high | do not change DNS |
| Rollback unclear | medium/high | document rollback to previous staging root before change |
| RBAC staging deploy attempted before separation | high | keep RBAC deploy blocked until separation complete |
| Symlink resolution confusion | medium | document that production root is currently a symlink to release directory |

## Required Guardrails

```text
1. No production web root change.
2. No production deploy.
3. No backend/API/migration changes.
4. No DNS/Certbot changes.
5. Nginx config edit only after explicit approval.
6. Rollback path must be documented before execution.
```

## Decision

```text
STAGING_PRODUCTION_WEB_ROOT_SEPARATION_RISK_MATRIX_CREATED
```
