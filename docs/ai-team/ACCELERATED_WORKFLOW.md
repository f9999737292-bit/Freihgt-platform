# Accelerated Workflow

Three tracks for freight-platform development with the Virtual AI Team.

## Overview

```text
PM plan → Owner implement → QA verify → Security review (if needed) → Docs commit report
```

| Track | Use when | Owner typical |
|-------|----------|---------------|
| **Fast Track** | Docs, UI polish, small defensive fixes | Docs / Frontend / PM |
| **Safe Track** | Backend APIs, auth, tenant/security, billing | Backend + QA + Security |
| **Sprint Pack** | 2–3 combined packs in one session | PM coordinates all roles |

---

## Fast Track Pack

### Use for

- Documentation-only packs
- UI polish (loading states, i18n, double-click guards)
- Small defensive fixes (validation, error messages)
- Runbooks, checklists, go/no-go reviews

### Do not use for

- Migrations
- Auth model changes
- Billing / document / transport core logic
- API contract changes

### Roles

| Role | Involvement |
|------|-------------|
| PM | Scope + final report |
| Owner | Docs or Frontend |
| QA | Light verification |
| Security | Skip unless auth/import touched |
| Docs | Always |

### Checks

```powershell
cd D:\Projects\freight-platform
make health-check

# If frontend touched:
cd apps\web-admin
npm run build

# If platform behavior claimed:
make integration-smoke-test
```

---

## Safe Track Pack

### Use for

- Backend API changes (handlers, domain, services)
- Auth / RBAC / admin guard
- Low-code import/export/migration logic
- Tenant-scoped writes
- Billing, documents, transport integrations

### Roles

| Role | Involvement |
|------|-------------|
| PM | Scope, block migrations/API breaks |
| Backend | Implementation + go test |
| QA | Full smoke + curl |
| Security | Required for auth/tenant/import |
| DevOps | If Docker/env/rebuild needed |
| Docs | Pack doc + NEXT_COMMANDS |

### Checks

```powershell
cd D:\Projects\freight-platform\services\{service}
go test ./...

cd D:\Projects\freight-platform
make health-check
make seed-lowcode-demo
make integration-smoke-test

cd apps\web-admin
npm run build
```

If `low-code-service` changed:

```powershell
make platform-build-service SERVICE=low-code-service
make platform-up-no-build
make health-check
```

---

## Sprint Pack

### Use for

Combining **2–3 related packs** in one accelerated session, e.g.:

- Go/No-Go review + Launch runbook
- Auth verification + docs + NEXT_COMMANDS
- Feature + tests + hardening + docs

### Phases

| Phase | Role | Output |
|-------|------|--------|
| 1. Planning | PM | Scope, risks, acceptance criteria, track selection |
| 2. Implementation | Backend / Frontend | Code or docs per scope |
| 3. Verification | QA + DevOps | health-check, tests, smoke, build |
| 4. Review | Security | Sign-off if security-sensitive |
| 5. Documentation | Docs | Pack doc(s), NEXT_COMMANDS |
| 6. Close | PM | Final report, commit, push |

### Sprint rules

- Single commit message per sprint when possible (or logical split per user request).
- No scope creep across combined packs.
- If hard blocker found: complete docs anyway, mark launch/decision **BLOCKED**.

---

## Choosing a track

```text
Does it touch migrations, auth, billing core, or API contracts?
  YES → Safe Track (+ Security)
  NO  → Does it touch backend Go at all?
          YES → Safe Track (lighter Security)
          NO  → Fast Track
Combining multiple packs in one user message?
  → Sprint Pack
```

---

## Integration with pilot / low-code

Current pilot state (see `docs/LOW_CODE_PILOT_LAUNCH_RUNBOOK_V0.1.md`):

- Staging: auth-on required
- Dev: default-off preserved
- Phase 1: TRANSPORT_ORDER only

Use **Safe Track** for any low-code backend change; **Fast Track** for pilot docs/rehearsal.
