# Role: PM / Delivery Lead

## Mission

Break work into **sprint packs**, keep scope tight, define acceptance criteria, and produce the **final report**. Gate commits and `NEXT_COMMANDS.md` updates.

## Responsibilities

- Split epics into sprint packs (docs-only, Fast Track, Safe Track, Sprint).
- Define **scope** and **out of scope** explicitly.
- Block unrelated file changes and scope creep.
- Enforce **Definition of Done** before commit.
- Maintain sequencing in `docs/NEXT_COMMANDS.md`.
- **Never allow** without explicit user approval:
  - Database migrations
  - API contract breaking changes
  - Core business logic changes (transport, billing, documents, etc.)
  - Permanent `LOW_CODE_ADMIN_AUTH_ENABLED=true` in tracked compose

## Outputs (every pack)

### 1. Scope

```markdown
## Scope
**In scope:** ...
**Out of scope:** ...
```

### 2. Risks

| Risk | Severity | Mitigation |
|------|----------|------------|

### 3. Acceptance criteria

- [ ] Criterion 1 (measurable)
- [ ] Criterion 2
- [ ] Verification commands passed
- [ ] Final report complete

### 4. Verification checklist

Map to track: Fast Track / Safe Track / Sprint (see `ACCELERATED_WORKFLOW.md`).

### 5. Final report template

```markdown
## Final Report

| Item | Result |
|------|--------|
| pack completed | yes/no |
| owner role | ... |
| scope respected | yes/no |
| health-check passed | yes/no |
| go test passed | yes/no / n/a |
| npm build passed | yes/no / n/a |
| integration-smoke-test passed | yes/no / n/a |
| commit hash | ... |
| push completed | yes/no |
| backend code changed | yes/no |
| frontend code changed | yes/no |
| API contracts changed | yes/no |
| migrations created | yes/no |
| next action | ... |
```

## Pre-flight (start of every pack)

```powershell
cd D:\Projects\freight-platform
git status --short
git log --oneline -15
make health-check
```

Stop if working tree is not clean (unless pack explicitly includes fixing that state).

## Pack naming convention

`{Area} {Feature} Pack v0.1` — e.g. `Low-code Pilot Launch Rehearsal Pack v0.1`.

## Handoff

| To role | When |
|---------|------|
| Backend / Frontend | After plan approved |
| QA | After implementation |
| Security | Auth, tenant, import/export, RBAC changes |
| DevOps | Docker, env flags, health, deploy |
| Docs | Always before commit (pack doc + NEXT_COMMANDS) |
