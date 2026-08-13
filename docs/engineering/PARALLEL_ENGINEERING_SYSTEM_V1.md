# Parallel Engineering System v1

7Rights Freight Platform — safe parallel AI-assisted development with Git worktrees and specialized Cursor subagents.

**Model-independent:** roles and processes do not assume a specific AI model.

## Documentation map

| Topic | Document |
|-------|----------|
| Quick Start (project owners) | [QUICK_START.md](./QUICK_START.md) |
| Task Contract | [TASK_CONTRACT_TEMPLATE.md](./TASK_CONTRACT_TEMPLATE.md) |
| Agent prompt | [AGENT_PROMPT_TEMPLATE.md](./AGENT_PROMPT_TEMPLATE.md) |
| Orchestrator prompt | [ORCHESTRATOR_PROMPT_TEMPLATE.md](./ORCHESTRATOR_PROMPT_TEMPLATE.md) |
| Handoff | [HANDOFF_TEMPLATE.md](./HANDOFF_TEMPLATE.md) |
| Worktrees (Windows) | [WORKTREE_PROCEDURE.md](./WORKTREE_PROCEDURE.md) |
| Path ownership | [OWNERSHIP.md](./OWNERSHIP.md) |
| Collisions (OpenAPI, migrations) | [COLLISION_POLICY.md](./COLLISION_POLICY.md) |
| Validation levels 0–3 | [VALIDATION_LEVELS.md](./VALIDATION_LEVELS.md) |
| Security / architecture triggers | [REVIEW_TRIGGERS.md](./REVIEW_TRIGGERS.md) |
| Orchestrator decision matrix | [ORCHESTRATOR_DECISION_MATRIX.md](./ORCHESTRATOR_DECISION_MATRIX.md) |
| Task registry | [parallel/tasks/README.md](./parallel/tasks/README.md) |
| Master Task | [MASTER_TASK_TEMPLATE.md](./MASTER_TASK_TEMPLATE.md) |
| Integration | [INTEGRATION_PROTOCOL.md](./INTEGRATION_PROTOCOL.md) |
| Review | [REVIEW_PROTOCOL.md](./REVIEW_PROTOCOL.md) |
| Operating manual | [../../AGENTS.md](../../AGENTS.md) |

## Lifecycle

```
BUSINESS GOAL
    ↓
ORCHESTRATOR + MASTER TASK
    ↓
ARCHITECT DECOMPOSITION (when needed)
    ↓
CONTRACT FREEZE (OpenAPI / events)
    ↓
PARALLEL WORKTREES (one agent each)
    ↓
DEVELOPER HANDOFFS
    ↓
SECURITY / QA / REVIEW
    ↓
INTEGRATION
    ↓
RELEASE GATE
```

## Principles

- One implementation task = one branch + one worktree + one ownership scope.
- No shared mutable working directory between parallel implementers.
- Frozen contracts (OpenAPI, migrations, shared types) before parallel coding when dependencies overlap.
- Reviewed SHAs only enter integration.
- Repository layout and Makefile are the source of operational truth.
- High-collision file edits must appear in Task Contract (`COLLISION_POLICY.md`).

## Agent roles (v1)

| Role | Subagent | Primary function |
|------|----------|------------------|
| Orchestrator | `orchestrator` | Decompose, assign, detect collisions |
| Architect | `architect` | Boundaries, ADRs, contract freeze plan |
| Backend | `backend-engineer` | `services/**`, coordinated `packages/shared-go/**` |
| Frontend | `frontend-engineer` | `apps/**`, UI packages |
| Security | `security-auditor` | Auth, tenant, IDOR review (readonly) |
| QA / Verification | `qa-verification` | Acceptance criteria, validation evidence |
| DevOps | `devops-engineer` | `infrastructure/**`, CI, ops scripts |
| Documentation | `documentation-engineer` | `docs/**`, runbooks |
| Reviewer | `reviewer` | Diff and scope review (readonly) |
| Integrator | `integrator` | Merge order and integration tests |

Cursor rules: `.cursor/rules/` (especially `05-parallel-engineering.mdc`).

## When tasks may run in parallel

Parallel work is allowed when all are true:

- Workstreams have disjoint **Allowed Paths** (or read-only overlap).
- Shared contracts are frozen and versioned (OpenAPI, migration order, event shapes).
- No workstream depends on unmerged output from another in the same integration batch.
- Tenant/security boundaries are unchanged or independently reviewable.

See [ORCHESTRATOR_DECISION_MATRIX.md](./ORCHESTRATOR_DECISION_MATRIX.md).

## When tasks must remain sequential

- Gateway auth/RBAC or tenant trust-boundary changes (usually).
- Shared migration batch without coordinator.
- OpenAPI breaking changes without downstream coordination.
- Control-tower projection rebuild / activation flows touching multiple services.
- Integration of overlapping files on the same branch.
- Two tasks editing `packages/openapi/openapi.yaml` without contract owner.

## Branch naming

```text
<type>/<domain>-<task>-v<version>
```

| Prefix | Use |
|--------|-----|
| `feat/` | Product feature |
| `fix/` | Bug fix |
| `chore/` | Engineering/tooling |
| `test/` | Tests / acceptance |
| `ops/` | Operational packaging/runbooks |
| `docs/` | Documentation-only |
| `arch/` | Architecture / contract |
| `int/` | Integration branch |

Bootstrap branch: `chore/parallel-engineering-system-v1`.

## Worktree naming (Windows)

Existing repo convention (many active worktrees):

```text
D:\Projects\freight-platform-<descriptive-name>
```

Recommended for new parallel tasks:

```text
D:\Projects\freight-platform-wt\<short-task-name>
```

**Never** nest worktrees inside `D:\Projects\freight-platform\`. Details: [WORKTREE_PROCEDURE.md](./WORKTREE_PROCEDURE.md).

## Frontend / backend contract flow

```
API requirement → contract definition → OpenAPI → backend → frontend → contract validation
```

Parallel backend + frontend only after OpenAPI freeze SHA is recorded.

## Integration model

```
feature worktree → commit → validation → handoff → review → integration branch → main
```

See [INTEGRATION_PROTOCOL.md](./INTEGRATION_PROTOCOL.md).

## Conflict handling

- Conflicts reported with file list and both sides' intent.
- Integrator does not guess semantic merges for business logic.
- Human or architect decision for ambiguous conflicts.

## Review gates

Every implementation handoff requires:

- Structured handoff (`HANDOFF_TEMPLATE.md`)
- Reviewer verdict: PASS, PASS_WITH_NOTES, FAIL, or BLOCKED
- QA verification when acceptance criteria apply
- Security auditor for triggers in `REVIEW_TRIGGERS.md`

## Windows worktrees

Diagnostics only via `.cursor/setup-worktree-windows.ps1` (no dependency install, no migrations, no secret copy).

Configuration: `.cursor/worktrees.json`.

## Initial recommended team topology (roles only)

Do **not** launch these as branches automatically. Organizational guidance for future Master Tasks.

| Team | Primary domains |
|------|-----------------|
| **Team A — Core TMS** | `transport-order-service`, `shipment-service` |
| **Team B — Commercial / RFx** | `rfx-service`, procurement apps |
| **Team C — Finance / Documents** | `billing-register-service`, `document-service`, `web-finance` |
| **Team D — Frontend / Control Tower** | `web-admin`, `control-tower-read-model-service` |
| **Team E — Security / Tenant** | `api-gateway`, tenant scoping reviews |
| **Team F — Platform / Infrastructure** | `infrastructure/`, `scripts/`, CI |

Cross-cutting: orchestrator, architect, reviewer, integrator, qa-verification.
