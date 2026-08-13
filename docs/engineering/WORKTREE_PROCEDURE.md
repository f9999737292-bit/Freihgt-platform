# Worktree Procedure (Windows)

Safe creation of sibling worktrees for parallel agents. **Never** create worktrees inside the main repository directory (e.g. `D:\Projects\freight-platform\worktrees\...`).

## Naming conventions

### Branch

```text
<type>/<domain>-<task>-v<version>
```

Examples (match existing repo style):

- `feat/control-tower-alert-ack-v0.1`
- `fix/shipment-tenant-isolation-v0.2`
- `test/control-tower-runtime-verification-v0.1`
- `docs/platform-security-handoff-v0.1`
- `chore/parallel-agent-system-v0.2`
- `int/pilot002-procurement-rfx-v1` (integration branches)

Prefixes: `feat/`, `fix/`, `chore/`, `test/`, `ops/`, `docs/`, `arch/`, `int/`.

### Worktree directory

**Existing convention (preferred for consistency with repo history):**

```text
D:\Projects\freight-platform-<short-descriptive-name>
```

Examples already in use: `freight-platform-pilot001-backend`, `freight-platform-audit-fix`.

**Alternative for new parallel tasks:**

```text
D:\Projects\freight-platform-wt\<short-task-name>
```

Task Contract must record the exact path.

## Base SHA handling

Task Contract MUST specify:

- **Base branch** (e.g. `origin/main`)
- **Base SHA** (`git rev-parse origin/main` at creation time)

Worktree is created from base branch; record SHA after creation with `git rev-parse HEAD`.

Do not assume bootstrap base `a1c246d` — always resolve current base at task start.

## Creation procedure (PowerShell)

From any existing worktree of the repo (e.g. main checkout):

```powershell
cd D:\Projects\freight-platform
git fetch origin

$branch = "feat/control-tower-alert-ack-v0.1"
$path   = "D:\Projects\freight-platform-wt\ct-alert-ack"
$base   = "origin/main"

git worktree add $path -b $branch $base
```

CMD equivalent:

```cmd
cd /d D:\Projects\freight-platform
git fetch origin
git worktree add D:\Projects\freight-platform-wt\ct-alert-ack -b feat/control-tower-alert-ack-v0.1 origin/main
```

If branch already exists locally:

```powershell
git worktree add D:\Projects\freight-platform-wt\ct-alert-ack feat/control-tower-alert-ack-v0.1
```

## Post-create verification (mandatory)

Run inside the new worktree:

```powershell
cd D:\Projects\freight-platform-wt\ct-alert-ack
git rev-parse --show-toplevel
git branch --show-current
git rev-parse HEAD
git status --short
```

Optional diagnostics (no install, no migrations):

```powershell
.\.cursor\setup-worktree-windows.ps1
```

## Cleanup policy

- Do **not** remove worktrees or branches from agent tasks without owner approval.
- After integration to `main`, owner may remove worktree:

```powershell
git worktree remove D:\Projects\freight-platform-wt\ct-alert-ack
```

- Delete branch only when merged and owner confirms:

```powershell
git branch -d feat/control-tower-alert-ack-v0.1
```

Never use `git worktree remove --force` or destructive clean without explicit owner approval.

## Dirty worktree policy

If `git status --short` is non-empty in an existing worktree, report it in the Task Contract / handoff. Do not stash or discard foreign changes.
