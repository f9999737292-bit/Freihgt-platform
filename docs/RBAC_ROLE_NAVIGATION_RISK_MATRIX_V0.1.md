# RBAC Role Navigation Risk Matrix v0.1

## Summary

Risk matrix for future RBAC and role navigation implementation.

## Decision

```text
RBAC_ROLE_NAVIGATION_RISK_MATRIX_CREATED
```

## Risks

| Risk | Severity | Mitigation |
| ---- | -------- | ---------- |
| Frontend nav hiding mistaken for real security | high | Document backend authorization boundary in implementation and acceptance docs |
| Incorrect role mapping hides needed modules | high | Acceptance checklist per role; manual spot-check matrix |
| Admin loses access to low-code/admin pages | high | Admin full-menu test (RBAC-AC-004, RBAC-AC-005) |
| Demo credentials remain visible in production | high | Remove or dev-only guard (Phase 6); RBAC-AC-008 |
| Role apps accidentally exposed | medium/high | Do not deploy role apps; hybrid strategy guardrail |
| Carrier/shipper flows confusing due to generic dashboard | medium | Role first-screen strategy (Phase 4) |
| Direct URL access bypasses hidden nav | medium | Access denied state (Phase 5) and backend enforcement reminder |
| Build regression | medium | Run build/typecheck before source commit (Phase 7) |
| Overloaded web-admin | medium | Keep hybrid strategy; defer role app extraction |
| Multi-role users get wrong nav | medium | Union permission resolver with admin override |
| mockAuth dev fallback masks RBAC bugs | medium | Test with real auth roles before production deploy |

## Required Guardrails

```text
Implementation pack must be source-code scoped to web-admin only.
No server/deploy actions in implementation pack.
No backend/API/migration changes unless separately approved.
No production deploy after implementation without deployment approval.
Navigation visibility is UX only — document in code comments where helpful.
```

## Next

```text
RBAC_AND_ROLE_NAVIGATION_IMPLEMENTATION_APPROVAL_PACK v0.1
```
