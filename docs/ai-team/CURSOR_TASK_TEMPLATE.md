# Cursor Task

Copy this template into Cursor chat and fill in all sections.

---

## Mode

<!-- Fast Track / Safe Track / Sprint Pack -->

Fast Track

## Owner Role

<!-- PM / Backend / Frontend / QA / DevOps / Security / Docs -->

PM

## Supporting Roles

<!-- Comma-separated; e.g. Backend, QA, Security, Docs -->

QA, Docs

## Context

- **Project:** `D:\Projects\freight-platform`
- **Baseline commit:** `{hash}` — `{message}`
- **Related docs:** `docs/...`
- **Prior packs completed:** ...

## Goals

1. ...
2. ...
3. ...

## Do Not Change

- [ ] Backend core business logic (transport, billing, documents, ...)
- [ ] API contracts (unless explicitly approved)
- [ ] Database migrations (unless explicitly approved)
- [ ] Tracked compose with `LOW_CODE_ADMIN_AUTH_ENABLED=true`
- [ ] Unrelated files / drive-by refactors

## Implementation Steps

1. **PM:** Pre-flight (`git status`, `git log`, `make health-check`)
2. **Owner:** ...
3. **QA:** Run verification checklist
4. **Security:** (if applicable) Sign-off checklist
5. **Docs:** Create/update pack doc + `NEXT_COMMANDS.md`
6. **PM:** Final report

## Verification

```powershell
cd D:\Projects\freight-platform
git status --short
make health-check

# Backend (if changed):
cd services\low-code-service
go test ./...

# Frontend (if changed):
cd apps\web-admin
npm run build

# Platform (if relevant):
make seed-lowcode-demo
make integration-smoke-test
```

## Commit Rules

- Commit only files in scope
- Message: `{type}: {description}` (e.g. `docs: add virtual ai team workflow`)
- Push to `main` only when pack requests it
- Do not commit secrets or auth-on flags in tracked config

```powershell
git add {paths}
git commit -m "{message}"
git push origin main
```

## Final Report

| Item | Result |
|------|--------|
| pack completed | yes/no |
| owner role | ... |
| mode | Fast / Safe / Sprint |
| health-check passed | yes/no |
| go test passed | yes/no/n/a |
| npm build passed | yes/no/n/a |
| integration-smoke-test passed | yes/no/n/a |
| commit hash | ... |
| push completed | yes/no |
| backend code changed | yes/no |
| frontend code changed | yes/no |
| API contracts changed | yes/no |
| migrations created | yes/no |
| next action | ... |

---

## Example: Docs-only pack

```markdown
## Mode
Fast Track

## Owner Role
Docs

## Supporting Roles
PM, QA

## Goals
Create pilot launch rehearsal checklist doc.

## Do Not Change
All code — docs only.

## Verification
make health-check
npm run build
```
