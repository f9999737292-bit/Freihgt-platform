# Path Ownership Zones

Default ownership for parallel workstream planning. Override only in Task Contract with explicit justification.

| Zone | Primary owner role | Paths |
|------|-------------------|-------|
| API Gateway / edge auth | backend-engineer + security-auditor review | `services/api-gateway/**` |
| Identity | backend-engineer | `services/identity-service/**` |
| Company | backend-engineer | `services/company-service/**` |
| Transport orders | backend-engineer | `services/transport-order-service/**` |
| RFx / procurement API | backend-engineer | `services/rfx-service/**` |
| Shipments / fleet | backend-engineer | `services/shipment-service/**` |
| Documents | backend-engineer | `services/document-service/**` |
| Billing register | backend-engineer | `services/billing-register-service/**` |
| Control Tower read model | backend-engineer | `services/control-tower-read-model-service/**` |
| Localization | backend-engineer | `services/localization-service/**` |
| Low-code platform | backend-engineer | `services/low-code-service/**` |
| Admin UI | frontend-engineer | `apps/web-admin/**` |
| Shipper UI | frontend-engineer | `apps/web-shipper/**` |
| Carrier UI | frontend-engineer | `apps/web-carrier/**` |
| Consignee UI | frontend-engineer | `apps/web-consignee/**` |
| Finance UI | frontend-engineer | `apps/web-finance/**` |
| Procurement UI | frontend-engineer | `apps/web-procurement/**` |
| Shared UI / TS | frontend-engineer | `packages/ui/**`, `packages/shared-ts/**`, `packages/i18n/**` |
| OpenAPI contracts | architect + contract owner task | `packages/openapi/**` |
| Shared Go libraries | backend-engineer (coordinated) | `packages/shared-go/**`, `packages/statussnapshot/**` |
| Central migrations | devops-engineer + migration coordinator | `infrastructure/migrations/**` |
| Docker / monitoring | devops-engineer | `infrastructure/docker-compose/**`, `infrastructure/monitoring/**` |
| CI | devops-engineer | `.github/workflows/**` |
| Ops scripts | devops-engineer | `scripts/ops/**`, `scripts/dev/**` (when assigned) |
| Platform Makefile | devops-engineer (high-collision) | `Makefile`, `go.work`, root `package.json`, `pnpm-workspace.yaml` |
| Documentation | documentation-engineer | `docs/**`, `README.md` |
| Engineering system | documentation-engineer / orchestrator | `.cursor/**`, `AGENTS.md`, `docs/engineering/**` |

## Shared / high-collision zones

These require explicit Task Contract declaration and orchestrator coordination:

- `packages/openapi/**` — single contract owner per integration batch
- `infrastructure/migrations/**` — sequential numbering via migration coordinator
- `services/api-gateway/internal/http/router.go` — route registration
- `services/api-gateway/internal/shipmentrbac/**` — RBAC policies
- Root build/workspace files — one owner per batch

## Cross-cutting reviewers (not default implementers)

| Role | Scope |
|------|-------|
| architect | decomposition, ADRs, contract freeze |
| security-auditor | auth, tenant, IDOR, secrets |
| qa-verification | acceptance criteria, validation evidence |
| reviewer | diff and scope review |
| integrator | merge order and integration verification |

One agent — one task — one branch — one worktree — one ownership scope.
