# Backend Public API Readiness Risk Matrix v0.1

## Summary

Risk matrix for future public backend/API readiness changes.

## Risks

| Risk | Severity | Mitigation |
|---|---|---|
| Public API exposed without auth/security boundary | high | define allowed public routes and auth requirements before execution |
| Nginx `/api/` proxy misroutes SPA/static assets | high | test SPA routes and API routes separately |
| API prefix mismatch causes persistent 404 | medium/high | verify gateway route table and frontend base URL before execution |
| Backend services unhealthy internally | high | require internal service health before public exposure |
| CORS/cookie/session issues after API exposure | medium/high | define origin/session strategy before live-data demo |
| Breaking current production static UI | high | pre/post UI checks and rollback plan |
| Nginx reload causes downtime | medium | only approve Nginx change when necessary; test syntax before reload |
| Opening internal ports publicly | high | keep UFW/security group closed; expose through Nginx only if approved |
| Secrets leaked in docs/logs | high | never record tokens/passwords/private keys |
| Overclaiming production readiness | medium/high | keep claim limited to approved backend/API scope |
| Punycode vs Unicode host mismatch breaks browser health fetch | medium | verify login backend status from both host forms in approval pack |
| Adding `/api/health` alias diverges from internal `/health` contract | low/medium | document canonical public health path; avoid duplicate divergent semantics |

## Guardrails

```text
1. No public API change without explicit approval.
2. No backend deploy without explicit approval.
3. No Nginx edit/reload without explicit approval.
4. No database migration/write without explicit approval.
5. Do not open internal ports publicly.
6. Do not claim full production readiness from API path readiness alone.
7. Keep static UI demo readiness separate from backend/API readiness.
```

## Decision

```text
BACKEND_PUBLIC_API_READINESS_RISK_MATRIX_CREATED
```
