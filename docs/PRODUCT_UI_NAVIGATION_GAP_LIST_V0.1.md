# Product UI Navigation Gap List v0.1

## Summary

This gap list captures UI/navigation improvements needed before deeper product flow work.

## P0 Gaps

| ID | Gap | Impact | Proposed Next Action |
| --- | --- | --- | --- |
| UI-P0-001 | Brand aligned to Bintrans in active UI (wave A1) | Owner confusion reduced | Monitor in next UI review |
| UI-P0-002 | No role-based navigation filtering in sidebar | All users see full admin/TMS/RFx stack regardless of role | ROLE_BASED_CABINETS_GAP_ANALYSIS_PACK → nav matrix |
| UI-P0-003 | RBAC not wired: `usePermissions` TODO for roles/permissions from `/auth/me` | Permission checks rely on mock/dev fallback | RBAC integration gap analysis pack |
| UI-P0-004 | Login pre-fills demo credentials | Production-facing review looks like dev sandbox | Environment-gate demo defaults |
| UI-P0-005 | No defined first-screen product strategy | Unclear whether users land on login, dashboard, or landing | Owner decision + first-screen pack |

## P1 Gaps

| ID | Gap | Impact | Proposed Next Action |
| --- | --- | --- | --- |
| UI-P1-001 | Control tower links to localhost Swagger/Prometheus/Grafana | Broken/confusing links in production review | Environment-aware link config |
| UI-P1-002 | Health page exposes localhost service URLs | Dev-oriented UX in production admin | Environment-aware health page |
| UI-P1-003 | Dashboard shows "Local" environment label | Misleading environment context on production | Fix environment display |
| UI-P1-004 | List pages may have unwired create actions | Owner clicks create and sees no flow | Module flow gap analysis packs |
| UI-P1-005 | No demo data for meaningful owner walkthrough | Empty tables reduce review value | Deferred — pilot demo data pack (paused) |
| UI-P1-006 | Separate role apps exist but production deploys web-admin only | Role cabinet strategy unclear | ROLE_BASED_CABINETS_GAP_ANALYSIS_PACK |

## P2 Gaps

| ID | Gap | Impact | Proposed Next Action |
| --- | --- | --- | --- |
| UI-P2-001 | Sidebar uses ad-hoc Unicode icons | Inconsistent visual polish | UI design system pass |
| UI-P2-002 | Low-code hub complexity for non-technical reviewers | Hard first impression for business owners | Simplified admin overview or guided tour |
| UI-P2-003 | Breadcrumb/navigation depth varies across modules | Minor orientation friction | Navigation consistency audit |
| UI-P2-004 | Settings page is technical (API URL, tenant IDs) | Not user-friendly for business settings | Settings UX redesign later |

## Recommended First Development Pack

```text
ROLE_BASED_CABINETS_GAP_ANALYSIS_PACK v0.1
```

## Not In Scope

```text
Pilot users
Production deploy
Database migrations
API contract changes
Server changes
```
