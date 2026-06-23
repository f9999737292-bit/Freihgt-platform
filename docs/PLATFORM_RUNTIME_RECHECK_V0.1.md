# Platform Runtime Recheck v0.1

Date: 2026-06-23

## Summary

Docker Desktop runtime was recovered after disk-space and WSL startup issues. Full platform runtime recheck completed successfully: all 9 backend services healthy, seed scripts OK, integration smoke test passed, low-code API spot checks returned HTTP 200, and `web-admin` production build completed.

## Environment

| Item | Value |
|------|-------|
| Project | `D:\Projects\freight-platform` |
| Git HEAD | `d1693f4` — docs: design low-code migrate to active flow |
| Working tree | clean |
| OS | Windows 10 (build 26200) |
| Docker Client | 29.5.3 |
| Docker Desktop | 4.78.0 (229452) |
| Docker Engine | 29.5.3 |
| C: free space | 12.72 GB (after recovery and platform startup) |
| D: free space | 325.72 GB |

## Docker Recovery

Initial state after break:

- Docker Desktop processes could start, but `docker version` hung or timed out.
- WSL distro `docker-desktop` was **Stopped**.
- Root cause: **C: drive nearly full** (~0.10 GB free). Docker/WSL2 requires space on C: for the WSL virtual disk and runtime.

Recovery steps applied (non-destructive):

1. Quit Docker Desktop / stop Docker processes.
2. `wsl --shutdown` (also attempted Windows reboot during earlier session).
3. Freed disk space on C: without `docker volume prune` or Postgres volume deletion:
   - User temp folders (~5.7 GB)
   - Docker logs
   - go-build and browser caches (~0.4 GB)
   - Adobe ARM old Acrobat 25 update cache (~11.0 GB)
4. Restarted Docker Desktop as regular user.
5. Confirmed Docker readiness: CLI, daemon, and compose all OK.

## Docker Status

```
Docker CLI: OK
Docker daemon: OK
Docker compose: OK
Docker readiness: OK
```

Docker system df (post-recovery):

| TYPE | TOTAL | ACTIVE | SIZE | RECLAIMABLE |
|------|-------|--------|------|-------------|
| Images | 13 | 12 | 9.846 GB | 82.97 MB |
| Containers | 12 | 12 | 1.167 MB | 0 B |
| Local Volumes | 2 | 2 | 150.3 MB | 0 B |
| Build Cache | 135 | 0 | 8.146 GB | 7.977 GB |

## Platform Startup

```bash
make platform-up-no-build
```

Result: **OK** — all containers already running; postgres healthy; all services healthy including `low-code-service`. No rebuild required.

## Health Check

```bash
make health-check
```

Result: **OK** — 9/9 services:

- api-gateway
- identity-service
- company-service
- transport-order-service
- rfx-service
- shipment-service
- document-service
- billing-register-service
- low-code-service

## Seed Checks

| Command | Result |
|---------|--------|
| `make seed-dev-admin` | OK — login verified via API Gateway |
| `make seed-demo-data` | OK — demo entities present (mostly SKIP/idempotent) |
| `make seed-lowcode-demo` | OK — 6 published templates, custom field values seeded |

## Low-code API Spot Checks

Tenant: `74519f22-ff9b-4a8b-8fff-a958c689682f`

| Endpoint | HTTP | Result |
|----------|------|--------|
| `GET /api/v1/low-code/form-templates/active?entity_type=TRANSPORT_ORDER` | 200 | Active template `transport_order_default` (PUBLISHED, v1, `is_active: true`) |
| `GET /api/v1/low-code/custom-field-values?entity_type=TRANSPORT_ORDER&entity_id=2db04b49-665c-469f-bcb1-ffeb1274fedb` | 200 | 3 custom field values (`cargo_class`, `internal_cost_center`, `loading_window_note`) |
| `GET /api/v1/low-code/audit-events?entity_type=TRANSPORT_ORDER&limit=5` | 200 | 5 audit events returned |

## Frontend Build

```bash
cd apps/web-admin
npm run build
```

Result: **OK** — Nuxt/Nitro build complete (total server output ~5.39 MB).

Low-code routes available for manual visual check via `npm run dev`:

- http://localhost:3000/low-code
- http://localhost:3000/low-code/form-templates
- http://localhost:3000/low-code/custom-field-values
- http://localhost:3000/low-code/audit

## Issues Found

1. Docker Engine unavailable due to C: disk exhaustion (~0.10 GB free).
2. WSL `docker-desktop` distro stopped; `docker version` hung until disk space recovered.
3. PowerShell `curl` alias breaks API spot checks — use `curl.exe` on Windows.

## Fixes Applied

1. Safe C: cleanup (~17 GB freed): temp, Docker logs, go-build/browser caches, old Adobe ARM update cache.
2. Docker Desktop restart after `wsl --shutdown`.
3. No backend code, API contract, or migration changes.

## Next Action

**Low-code Migrate-to-Active Preview API Pack v0.1**
