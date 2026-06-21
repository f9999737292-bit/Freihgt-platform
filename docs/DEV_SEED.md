# Dev Admin Seed

Idempotent development seed for the 7Rights web-admin portal.

## Prerequisites

1. Backend platform running and healthy:

```powershell
cd D:\Projects\freight-platform
make platform-up-no-build   # or make platform-up-safe on Windows/WSL
make health-check
```

2. Migrations applied (first time only):

```powershell
make migrate-up
```

3. Git Bash available (recommended on Windows) or WSL/bash.

## Run seed

```powershell
cd D:\Projects\freight-platform
make seed-dev-admin
```

Direct script:

```powershell
& "C:\Program Files\Git\bin\bash.exe" -lc "cd /d/Projects/freight-platform && make seed-dev-admin"
```

## What it creates

| Field | Value |
| ----- | ----- |
| Tenant ID | `74519f22-ff9b-4a8b-8fff-a958c689682f` |
| Tenant code | `dev-7rights` |
| Company | ООО 7Rights Dev (`PLATFORM_OPERATOR`) |
| Email | `admin@7rights.local` |
| Password | `Admin123456!` (dev fixture) |
| Role | `PLATFORM_ADMIN` |
| Login URL | http://localhost:3000/login |

Override via environment variables (optional):

```bash
TENANT_ID=... ADMIN_EMAIL=... ADMIN_PASSWORD=... make seed-dev-admin
```

## Idempotent behavior

Safe to run repeatedly (10+ times):

- Tenant upsert in PostgreSQL (`ON CONFLICT (id) DO UPDATE`)
- Company/user lookup before create
- Membership and role assignment skip duplicates (`409` tolerated)
- List API calls use `limit=100` (service max)
- POST bodies sent via `curl --data-binary @-` (UTF-8 safe on Windows)

## If backend is not up

Error message:

```text
API Gateway unavailable. Start platform first: make platform-up-no-build or make platform-up-safe
```

Then:

```powershell
make platform-up-no-build
make health-check
make seed-dev-admin
```

## Windows notes

- Uses `docker.exe` when Git Bash `docker` wrapper fails
- Run through Git Bash or `make seed-dev-admin` from Makefile (bash target)
- Cyrillic company name encoded via `jq` JSON builder

## Verify login

After seed:

1. Start web-admin: `cd apps/web-admin && npm run dev`
2. Open http://localhost:3000/login
3. Enter tenant ID, email, password from table above
4. Backend status banner should show **online**

Seed also verifies `POST /api/v1/auth/login` through the gateway when possible.

## Related docs

- [AUTH_RBAC.md](./AUTH_RBAC.md)
- [TROUBLESHOOTING.md](./TROUBLESHOOTING.md)
- [WINDOWS_ENVIRONMENT.md](./WINDOWS_ENVIRONMENT.md)
