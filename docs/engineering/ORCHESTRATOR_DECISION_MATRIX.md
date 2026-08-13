# Orchestrator Decision Matrix

Use when splitting Master Tasks. Extend with Task Contract specifics.

| Situation | Parallel? | Requirement |
|-----------|----------:|-------------|
| Different backend services (disjoint paths) | Yes | Isolated Allowed Paths; Level 1 validation each |
| Backend + frontend with frozen OpenAPI SHA | Yes | Contract owner task complete; reference freeze SHA |
| Same service, same files | No | Serialize tasks |
| Two OpenAPI edits in same batch | Usually no | Single contract owner; sequential merge |
| Two new migrations | Conditional | Migration coordinator assigns numbers first |
| Gateway auth / RBAC / tenant trust | Conditional | security-auditor required; often sequential |
| `router.go` route registration | No | One task owns gateway routing per batch |
| Docs-only under `docs/**` | Yes | Isolated paths; Level 0 |
| CI / Compose / Makefile | No | One devops task per batch |
| Control Tower projection + activation | No | Multi-service; architect-led sequencing |
| Shared package `packages/shared-go/**` | Conditional | Architect review; avoid parallel breaking changes |
| Integration merge task | No | integrator after reviewer + qa PASS |
| Security boundary change | Conditional | security-auditor + architect review |
| New microservice | No | Architect task first |
| Event contract change | Conditional | Architect review; consumer tasks depend on freeze |
| Database ownership change | No | Architect + migration coordinator |
| Frontend-only UI (no API change) | Yes | Frozen API; no OpenAPI task |
| Ops runbook + service code | Yes | If paths disjoint (`docs/` vs `services/`) |

## Task type labels

| Label | Orchestrator action |
|-------|---------------------|
| parallel-safe | Assign separate branches/worktrees |
| dependent | Block until dependency SHA recorded |
| high-collision | Assign coordinator or serialize |
| integration | integrator role after reviews |

## Integration order (default)

```
contract freeze → backend (+ migration coordinator if needed) → frontend → security review → qa verification → integrator → main
```

Adjust per Master Task dependency graph.
