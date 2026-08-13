# Task Contract Template

Canonical assignment standard for every parallel agent task. Copy this file per workstream; do not edit this template in place.

Orchestrator fills all sections before assigning work. Implementing agents must read the contract before editing.

---

# Task Contract

## Task ID

<!-- e.g. CT-ALERT-ACK-WS-2 -->

## Title

<!-- short outcome title -->

## Owner

<!-- human or orchestrator alias -->

## Role

<!-- orchestrator | architect | backend-engineer | frontend-engineer | security-auditor | qa-verification | devops-engineer | documentation-engineer | reviewer | integrator -->

## Repository

<!-- D:\Projects\freight-platform-wt\<task> or assigned worktree path -->

## Base branch

<!-- e.g. origin/main -->

## Base SHA

<!-- git rev-parse <base-branch> at task creation -->

## Working branch

<!-- e.g. feat/control-tower-alert-ack-v0.1 -->

## Worktree

<!-- e.g. D:\Projects\freight-platform-wt\ct-alert-ack-backend -->

---

## Objective

<!-- one paragraph: what success looks like -->

## In scope

- ...

## Out of scope

- ...

## Allowed paths

- ...

## Forbidden paths

- ...

## Dependencies

<!-- Task IDs and SHAs that must complete first; or "none" -->

- ...

## Security invariants

- Tenant isolation unchanged unless explicitly authorized
- Gateway JWT trust boundary preserved
- ...

## Acceptance criteria

1.
2.
3.

## Required validation

<!-- Level 0–3 from VALIDATION_LEVELS.md -->

Level: <!-- 0 | 1 | 2 | 3 -->

Commands:

- ...

## Required deliverables

- Implementation diff within Allowed Paths
- Handoff (`HANDOFF_TEMPLATE.md`)
- ...

## Integration target

<!-- e.g. int/control-tower-alert-ack-v0.1 → main -->

## Handoff requirements

- Base SHA, final SHA, branch, worktree
- Changed files list
- Contracts changed (OpenAPI / migrations)
- Validation results with NOT_RUN explicitly listed
- `git status --short`

---

## Related templates

- Agent prompt: `AGENT_PROMPT_TEMPLATE.md`
- Handoff: `HANDOFF_TEMPLATE.md`
- Registry entry: `parallel/tasks/_EXAMPLE.md`
