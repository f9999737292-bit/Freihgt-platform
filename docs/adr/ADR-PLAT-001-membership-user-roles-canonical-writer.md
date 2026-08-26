# ADR-PLAT-001: Membership and user_roles Canonical Writer

## Status

Accepted — architecture freeze (EDO-0.2)

## Context

Discovery finding **F-002 (Dual writers on user_roles)**: both `identity-service` and `company-service` INSERT/DELETE into `core.user_roles` while `core.company_memberships` is primarily managed by company-service.

### Evidence — write paths

| Service | Operation | Table | Code path |
|---------|-----------|-------|-----------|
| identity-service | INSERT role assignment | `core.user_roles` | `MembershipRepository.AddCompanyRoleToUser` |
| identity-service | DELETE role assignment | `core.user_roles` | `MembershipRepository.RemoveCompanyRoleFromUser` |
| company-service | INSERT role on member create/update | `core.user_roles` | `MembershipRepository.AddUserRoleForCompany` |
| company-service | DELETE all roles for user+company | `core.user_roles` | `MembershipRepository.RemoveUserRolesForCompany` |

### Evidence — read consumers

| Consumer | Usage |
|----------|-------|
| identity-service | Lists roles via join on membership APIs |
| company-service | Membership CRUD with embedded role assignment |
| billing-register-service | JOIN `user_roles` for authorization context |
| payment-service | JOIN `user_roles` for authorization context |
| contract-rate-service | JOIN `user_roles` for authorization context |
| rfx-service | JOIN `user_roles` for authorization context |
| api-gateway | JWT + RBAC policy evaluation (role codes from token claims, not direct DB) |

Both services write to the same table without a single transactional owner boundary.

## Decision

### Canonical writers (frozen)

| Table / concern | Canonical writer | Rationale |
|-----------------|------------------|-----------|
| `core.company_memberships` | **company-service** | Company membership CRUD, position, status, soft-delete |
| `core.user_roles` | **identity-service** | RBAC assignments are identity domain; aligns with `core.roles` ownership |

### Target interaction model

1. **company-service** creates/updates/deletes memberships **without direct `user_roles` INSERT/DELETE**.
2. On membership create with initial roles, company-service calls **identity-service internal API** (or publishes `plat.membership.roles_assignment_requested` event — future) to assign roles.
3. On membership delete, company-service requests identity-service to **revoke all roles** for `(tenant_id, user_id, company_id)`.
4. **Standalone role APIs** remain on identity-service only.

### Migration risk

| Risk | Severity | Mitigation |
|------|----------|------------|
| Race between company membership delete and identity role revoke | MEDIUM | Saga or same-request synchronous call with idempotent revoke |
| Duplicate INSERT if both paths remain | HIGH | PLAT-0.1 removes company-service direct writes behind feature flag |
| Integration tests seeding `user_roles` directly | LOW | Test fixtures unchanged until PLAT implementation |
| Gateway token claims stale after role change | MEDIUM | Existing re-login / token refresh policy |

### Current state (no code change in EDO-0.2)

Dual-write **continues in production code** until PLAT-0.1 implementation. Architecture ownership is **frozen** now to guide that phase.

## Consequences

- Resolves F-002 at architecture level with sufficient evidence
- PLAT-0.1 Task Contract required before code change
- EDO/TEDO/FF agents must not "fix" user_roles in passing

## References

- `services/identity-service/internal/repository/membership_repository.go`
- `services/company-service/internal/repository/membership_repository.go`
- `infrastructure/migrations/000002_create_core_tables.up.sql`
