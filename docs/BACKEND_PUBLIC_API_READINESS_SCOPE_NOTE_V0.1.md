# Backend Public API Readiness Scope Note v0.1

## Summary

This note defines the scope of backend public API readiness work.

## Current Signed-off Scope

```text
Production static UI demo readiness is signed off.
```

## Current Limitation

```text
Production live-data demo readiness is partial because public /api/health and /api/ return 404 and backend-offline banner is visible.
```

Additional audit detail: public `/health` returns 200 (gateway). Frontend health probe uses `/health`, not `/api/health`. Live-data flows depend on authenticated `/api/v1/*` routes which are proxied but require tenant/auth headers.

## Backend Public API Readiness Means

```text
The production frontend can reach approved backend API/health endpoints through a documented public route without opening internal ports directly.
```

## Out of Scope Unless Separately Approved

```text
Full production readiness.
SLA readiness.
Security audit signoff.
Full E2E business workflow readiness.
Legal/document/billing readiness.
Load/performance readiness.
Database migration.
New backend feature development.
```

## Decision

```text
BACKEND_PUBLIC_API_READINESS_SCOPE_RECORDED
```
