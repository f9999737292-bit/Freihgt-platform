# Task Contract

## Task ID

CT-AA-006

## Title

Control Tower alert acknowledgement — integration

## Owner

orchestrator

## Role

integrator

## Repository

D:\Projects\freight-platform-wt\ct-alert-ack-integration

## Base branch

origin/main (updated before merge)

## Base SHA

`<latest origin/main at integration start>`

## Working branch

int/control-tower-alert-ack-v0.1

## Worktree

D:\Projects\freight-platform-wt\ct-alert-ack-integration

---

## Objective

Merge contract, backend, and frontend workstreams into `int/control-tower-alert-ack-v0.1`, resolve conflicts, verify CI-ready state, open PR to main. No force push.

## In scope

- Create/update integration branch from origin/main
- Merge order: CT-AA-001 → CT-AA-002 → CT-AA-003 (resolve conflicts)
- Verify combined diff scope matches all task contracts
- Run integration validation (Level 2 minimum)
- Open GitHub PR to main
- Update task registry statuses to INTEGRATED on success

## Out of scope

- New feature scope beyond v0.1 pilot
- Force push
- Deleting worktrees or branches
- Phase 3B.2+ features

## Allowed paths

- All paths required for conflict resolution during merge
- `docs/engineering/parallel/tasks/ct-aa-*.md` (status updates only)
- `docs/engineering/parallel/pilots/control-tower-alert-ack-v0.1/INTEGRATION.md`

## Forbidden paths

- Scope expansion beyond merged workstreams
- Rewriting unrelated code

## Dependencies

- CT-AA-001 CONTRACT_FREEZE_SHA merged first
- CT-AA-002 backend complete
- CT-AA-003 frontend complete
- CT-AA-004 security PASS or CONDITIONAL PASS
- CT-AA-005 QA PASS (no blocking FAIL)

## Security invariants

- Do not weaken tenant or auth checks during conflict resolution
- Escalate if merge introduces OpenAPI drift from freeze

## Acceptance criteria

1. Integration branch contains all pilot changes.
2. `git diff --check` clean.
3. CI checks pass on PR (or documented pre-existing failures).
4. PR opened to main with test plan.
5. Task registry updated.

## Required validation

Level: 2

Commands:

- `git diff --check`
- `make openapi-validate`
- Targeted `go test` for changed services
- `pnpm --filter web-admin build`

## Required deliverables

- Integration branch SHA
- PR URL
- INTEGRATION.md handoff

## Integration target

int/control-tower-alert-ack-v0.1 → main

## Handoff requirements

- Merge commit SHAs
- PR link
- CI status
- Final origin/main merge status (after PR merge — separate step)
