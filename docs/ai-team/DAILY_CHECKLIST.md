# Daily Checklist — Virtual AI Team

Operator checklist for accelerated freight-platform development.

---

## Morning startup

```powershell
cd D:\Projects\freight-platform
git status --short
git pull --ff-only
make health-check
```

- [ ] Working tree clean or intentional WIP noted
- [ ] All services healthy

---

## Before each pack

```powershell
cd D:\Projects\freight-platform
git status --short
git log --oneline -5
```

- [ ] Assign **owner role** (see `README.md`)
- [ ] Choose track: Fast / Safe / Sprint (`ACCELERATED_WORKFLOW.md`)
- [ ] Paste or fill `CURSOR_TASK_TEMPLATE.md`
- [ ] PM defines scope and **Do Not Change**

---

## After backend changes

```powershell
cd D:\Projects\freight-platform\services\low-code-service
go test ./...
```

For other services, run `go test ./...` in the changed service directory.

If `low-code-service` binary-affecting:

```powershell
cd D:\Projects\freight-platform
make platform-build-service SERVICE=low-code-service
make platform-up-no-build
make health-check
```

---

## After frontend changes

```powershell
cd D:\Projects\freight-platform\apps\web-admin
npm run build
```

---

## Before commit

```powershell
cd D:\Projects\freight-platform
make integration-smoke-test
git status --short
git diff
```

- [ ] QA sign-off (checks pass)
- [ ] Security sign-off (if auth/tenant/import)
- [ ] Only intended files staged
- [ ] No `.env`, secrets, or auth-on in tracked compose

---

## After commit

```powershell
git log --oneline -3
git push origin main
```

- [ ] Final report recorded (commit hash, checks, next action)
- [ ] `docs/NEXT_COMMANDS.md` updated

---

## End of day

- [ ] No uncommitted WIP unless intentional
- [ ] `docker-compose.override.yml` removed if used for temp auth-on
- [ ] Pilot/staging env flags documented if changed (not committed)

---

## Quick reference

| Need | Command |
|------|---------|
| Platform up | `make platform-up-no-build` |
| Health | `make health-check` |
| Low-code seed | `make seed-lowcode-demo` |
| Smoke | `make integration-smoke-test` |
| Go tests | `cd services/low-code-service && go test ./...` |
| Web build | `cd apps/web-admin && npm run build` |

Role playbooks: `docs/ai-team/*.md`
