# Windows Make + Bash Compatibility

## Problem

On Windows, `make seed-dev-admin` and similar targets run shell scripts (`.sh`). If the recipe calls plain `bash`, Windows resolves it from `PATH` — often to **WSL** (`C:\Windows\System32\bash.exe` or the Store stub), not Git Bash.

WSL bash in this setup may fail with errors like:

```text
execvpe(/bin/bash) failed: No such file or directory
```

The same scripts work when invoked directly via Git Bash:

```powershell
& "C:\Program Files\Git\bin\bash.exe" scripts/dev/seed_dev_admin.sh
```

## Solution in this repo

The Makefile defines a **`BASH`** variable:

- On **`OS=Windows_NT`** → default `C:/Program Files/Git/bin/bash.exe` (Git for Windows).
- On Linux / macOS / GitHub Actions → default `bash`.

Seed and smoke targets call `"$(BASH)" script.sh` instead of plain `bash script.sh`.

> **Note:** GnuWin32 `make` on Windows does not resolve `$(wildcard ...)` for `Program Files` paths, so detection uses `OS=Windows_NT` rather than file existence checks.

Make's own **`SHELL`** is also set to Git Bash on Windows so recipe lines run under a POSIX shell.

## Verify your environment

```powershell
cd D:\Projects\freight-platform
make bash-check
```

Expected output includes:

- `BASH=C:/Program Files/Git/bin/bash.exe` (on Windows with Git installed)
- `OK: bash available`
- bash version line

## Daily commands (should work via `make`)

```powershell
make seed-dev-admin
make seed-demo-data
make integration-smoke-test
make health-check
```

## Manual override

If auto-detect picks the wrong interpreter:

```powershell
make BASH="C:/Program Files/Git/bin/bash.exe" seed-dev-admin
make BASH="C:/Program Files/Git/bin/bash.exe" seed-demo-data
make BASH="C:/Program Files/Git/bin/bash.exe" integration-smoke-test
```

## Why Git Bash is preferred here

- Project dev scripts use `curl`, `grep`, and Unix-style paths — Git Bash provides them on Windows.
- Scripts talk to `localhost` services (API Gateway, Postgres via Docker); Git Bash matches how CI/Linux devs run the same scripts.
- WSL is a separate environment; mixing WSL bash with Docker Desktop / host `localhost` can be fragile depending on network mode.

## Related

- `docs/NEXT_COMMANDS.md` — daily workflow
- `Makefile` — `BASH`, `GIT_BASH`, `bash-check`, seed/smoke targets
