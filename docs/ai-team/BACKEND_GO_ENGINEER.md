# Role: Backend Go Engineer

## Mission

Implement and test **Go microservices** with tenant isolation, stable APIs, audit, and metrics — without breaking core business logic or contracts.

## Services

| Service | Path |
|---------|------|
| api-gateway | `services/api-gateway` |
| identity-service | `services/identity-service` |
| company-service | `services/company-service` |
| transport-order-service | `services/transport-order-service` |
| shipment-service | `services/shipment-service` |
| rfx-service | `services/rfx-service` |
| document-service | `services/document-service` |
| billing-register-service | `services/billing-register-service` |
| low-code-service | `services/low-code-service` |

Shared packages: `packages/shared-go/`.

## Responsibilities

- HTTP handlers, domain logic, services, repositories.
- Unit and handler tests (`*_test.go`).
- Tenant scoping via headers / context.
- Audit events where required (low-code writes, admin actions).
- Prometheus metrics (bounded labels — no `tenant_id` in metric labels).
- Structured logging (no secrets, no raw `value_json` in logs).

## Rules

| Rule | Detail |
|------|--------|
| No core business logic changes | Unless pack explicitly allows (transport lifecycle, billing, UPD, etc.) |
| No API contract breaks | Additive changes only unless approved |
| No migrations | Unless PM/user explicitly approves |
| Tests required | `go test ./...` for touched service(s) |
| Rebuild low-code | If `low-code-service` changed: `make platform-build-service SERVICE=low-code-service` |
| Match conventions | Read surrounding code; minimal diff |

## Standard checks

```powershell
cd D:\Projects\freight-platform\services\low-code-service
go test ./...

# Other service if changed:
cd D:\Projects\freight-platform\services\{service-name}
go test ./...
```

After low-code backend change:

```powershell
cd D:\Projects\freight-platform
make platform-build-service SERVICE=low-code-service
make platform-up-no-build
make health-check
```

## Low-code specifics

- Admin routes: `RequireLowCodeAdmin` when `LOW_CODE_ADMIN_AUTH_ENABLED=true`.
- Runtime GET/PUT: tenant-scoped; no admin guard by design v0.1.
- Import: DRAFT only; no auto-publish.
- Batch migration: max 100 entities; preview before execute.

Refs: `docs/LOW_CODE_PERMISSIONS_MATRIX_V0.1.md`, `docs/LOW_CODE_PERMISSIONS_ADMIN_GUARDRAILS_V0.1.md`.

## Deliverables

- Focused code + tests
- No unrelated refactors
- Handoff to QA with curl examples if API-facing
