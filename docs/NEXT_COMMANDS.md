# Daily Commands

## Project root

```powershell
cd D:\Projects\freight-platform
```

## Check current state

```powershell
git status --short
git log --oneline -5
```

## Start backend

```powershell
make platform-up-no-build
make health-check
```

## Check bash (Windows)

```powershell
make bash-check
```

On Windows, Makefile uses Git Bash for `.sh` scripts (not WSL `bash` from PATH). See `docs/WINDOWS_MAKE_BASH.md`.

## Seed dev data

```powershell
make seed-dev-admin
make seed-demo-data
make seed-lowcode-demo
```

Custom field values API (after seed-demo-data + seed-lowcode-demo):

```powershell
curl -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" "http://localhost:8088/v1/low-code/custom-field-values?entity_type=TRANSPORT_ORDER&entity_id=<ENTITY_ID>"
```

See `docs/LOW_CODE_CUSTOM_FIELD_VALUES_API_V0.1.md`.

Low-code admin UI (read-only preview):

```text
http://localhost:3000/low-code
http://localhost:3000/low-code/form-templates
http://localhost:3000/low-code/custom-field-values
```

See `docs/LOW_CODE_ADMIN_UI_PREVIEW_V0.1.md`.

If a target fails with WSL/bash errors, override:

```powershell
make BASH="C:/Program Files/Git/bin/bash.exe" seed-dev-admin
```

## Run smoke test

```powershell
make integration-smoke-test
```

## Start frontend

```powershell
cd D:\Projects\freight-platform\apps\web-admin
npm run dev
```

Open:

```text
http://localhost:3000/login
```

Login:

```text
Tenant ID: 74519f22-ff9b-4a8b-8fff-a958c689682f
Email: admin@7rights.local
Password: Admin123456!
```

## Commit

```powershell
cd D:\Projects\freight-platform
git status --short
git add .
git commit -m "..."
git push origin main
```

## Last commits

```powershell
git log --oneline -5
```
