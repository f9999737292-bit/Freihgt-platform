# Docker / WSL Stability (Windows)

Guidance for Docker Desktop + WSL when parallel `docker compose build` crashes the engine.

## Problem

On Windows + Docker Desktop + WSL, parallel Docker Compose build can fail with:

- `error reading from server: EOF`
- `failed to receive status`
- `Docker Desktop is unable to start`
- `exit status 0xc00000fd`

This is usually an environment/Docker/WSL stability problem, not a Go code problem.

## Recommended workflow

1. Restart Docker Desktop.
2. Run:

```powershell
wsl --shutdown
```

3. Start Docker Desktop again and wait until it is Running.
4. Build services sequentially:

```bash
make platform-build-serial
```

5. Start containers without rebuilding:

```bash
make platform-up-no-build
```

Or use:

```bash
make platform-up-safe
```

## Build one service

```bash
make platform-build-service SERVICE=document-service
make platform-build-service SERVICE=billing-register-service
make platform-build-service SERVICE=api-gateway
```

## Continue after partial build

If some services were already built, build only failed services:

```bash
make platform-build-service SERVICE=document-service
make platform-build-service SERVICE=billing-register-service
make platform-build-service SERVICE=api-gateway
```

Then:

```bash
make platform-up-no-build
make platform-ps
make migrate-up
make health-check
```

## Do not run destructive cleanup

Do not run:

```bash
docker volume prune
```

unless local database data can be deleted.

Safe alternative when Docker is healthy: `make docker-clean-safe` (no volume prune).

## Runtime chain after successful startup

```bash
make platform-ps
make migrate-up
make health-check
make seed-dev-admin
make generate-db-metrics-traffic
make db-metrics-check
make db-pool-metrics-check
```

## Makefile targets

| Target | Purpose |
|--------|---------|
| `make platform-up` | Fast mode: parallel build + start |
| `make platform-up-safe` | Serial build, then start without rebuild |
| `make platform-build-serial` | Build all backend images one at a time |
| `make platform-build-service SERVICE=name` | Build a single service |
| `make platform-up-no-build` | Start stack without rebuilding |
| `make platform-up-backend-only` | Start postgres + backend services only (no rebuild) |

On Windows, run `make` from PowerShell or Git Bash. `platform-build-serial` calls `platform-build-service` once per service (no parallel compose bake).

See also: [DOCKER_DISK_TROUBLESHOOTING.md](./DOCKER_DISK_TROUBLESHOOTING.md), [WINDOWS_ENVIRONMENT.md](./WINDOWS_ENVIRONMENT.md).
