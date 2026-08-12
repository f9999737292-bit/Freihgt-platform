# Parallel Engineering System v1

7Rights Freight Platform — safe parallel AI-assisted development with Git worktrees and specialized Cursor subagents.

## Lifecycle

```
BUSINESS GOAL
    ↓
MASTER TASK
    ↓
ARCHITECT DECOMPOSITION
    ↓
CONTRACT FREEZE
    ↓
PARALLEL WORKTREES
    ↓
DEVELOPER HANDOFFS
    ↓
SECURITY / REVIEW
    ↓
INTEGRATION
    ↓
RELEASE GATE
```

## Principles

- One implementation task = one branch + one worktree when possible.
- No shared mutable working directory between parallel implementers.
- Frozen contracts (OpenAPI, migrations, shared types) before parallel coding when dependencies overlap.
- Reviewed SHAs only enter integration.
- Repository layout and Makefile are the source of operational truth.

## When tasks may run in parallel

Parallel work is allowed when all are true:

- Workstreams have disjoint **Allowed Paths** (or read-only overlap).
- Shared contracts are frozen and versioned (OpenAPI, migration order, event shapes).
- No workstream depends on unmerged output from another in the same integration batch.
- Tenant/security boundaries are unchanged or independently reviewable.

Examples of parallel-safe splits in this repo:

- Frontend `apps/web-admin` UI work + backend `services/rfx-service` API work (with frozen OpenAPI).
- `services/document-service` changes + `apps/web-finance` display changes after contract freeze.
- Platform docs/ops scripts in `scripts/ops/` separate from service logic.

## When tasks must remain sequential

- Gateway auth/RBAC or tenant trust-boundary changes.
- Shared migration that multiple services depend on.
- OpenAPI breaking changes without downstream coordination.
- Control-tower projection rebuild / activation flows touching multiple services.
- Integration of overlapping files on the same branch.

## Branch naming

| Prefix | Use |
|--------|-----|
| `feat/` | Product feature |
| `fix/` | Bug fix |
| `chore/` | Engineering/tooling |
| `test/` | Tests only |
| `ops/` | Operational packaging/runbooks |

Bootstrap branch for this system: `chore/parallel-engineering-system-v1`.

## Ownership

| Role | Owns |
|------|------|
| Architect | Decomposition, contract freeze plan, dependency graph |
| Developer agent | Implementation within Allowed Paths on assigned branch/worktree |
| Security auditor | Tenant/auth review (readonly) |
| Reviewer | Requirement/diff verification (readonly) |
| Integrator | Merge order, conflict reporting, integration verification |

Agents never modify another agent's branch.

## Dependency handling

1. Architect publishes dependency graph with explicit blockers.
2. Blocked workstreams start only after blocker SHAs are reviewed and recorded.
3. Contract-freeze artifacts are referenced by SHA or file path in each Agent Task.

## Conflict handling

- Conflicts are reported with file list and both sides' intent.
- Integrator does not guess semantic merges for business logic.
- Human or architect decision required for ambiguous conflicts.

## Review gates

Every implementation handoff requires:

- Structured handoff (`HANDOFF_TEMPLATE.md`)
- Reviewer verdict: PASS, PASS_WITH_NOTES, FAIL, or BLOCKED
- Security auditor review for auth, tenant, or data-access changes

## Integration gates

- Only reviewed commits at approved SHAs
- Migration order validated
- OpenAPI consistency checked (`make openapi-validate`)
- Targeted or integration tests per `INTEGRATION_PROTOCOL.md`
- Release gate: health checks and smoke tests when authorized

## Initial recommended team topology (roles only)

Do **not** launch these as branches or tasks automatically. This is organizational guidance for future Master Tasks.

| Team | Primary domains |
|------|-----------------|
| **Team A — Core TMS** | `transport-order-service`, `shipment-service`, core transport flows |
| **Team B — Commercial / RFx** | `rfx-service`, procurement-facing APIs and apps |
| **Team C — Finance / Documents** | `billing-register-service`, `document-service`, `web-finance` |
| **Team D — Frontend / Control Tower** | `apps/web-admin`, `control-tower-read-model-service`, UI packages |
| **Team E — Security / Tenant Isolation** | `api-gateway` auth/RBAC, tenant scoping reviews |
| **Team F — Platform / Infrastructure** | `infrastructure/`, `scripts/`, CI, observability |

Cross-cutting roles:

- **Architect** — decomposition and contract freeze
- **Reviewer** — independent verification
- **Integrator** — merge and integration testing

## Related documents

- `MASTER_TASK_TEMPLATE.md`
- `AGENT_TASK_TEMPLATE.md`
- `HANDOFF_TEMPLATE.md`
- `REVIEW_PROTOCOL.md`
- `INTEGRATION_PROTOCOL.md`
- `.cursor/rules/` — enforced agent rules
- `.cursor/agents/` — specialized subagents
- `AGENTS.md` — concise operating manual

## Windows worktrees

Use Git worktrees under `D:\Projects\freight-platform-*`. Initial setup runs diagnostics only via `.cursor/setup-worktree-windows.ps1` (no dependency install, no migrations, no secret copy).
