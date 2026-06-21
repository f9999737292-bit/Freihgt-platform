# Docker disk troubleshooting — Windows / WSL

Guide for diagnosing and safely freeing Docker Desktop disk space when local dev is blocked by a full WSL virtual disk or Docker data store.

## Problem

Docker Desktop / WSL disk image can grow over time and block rebuilds and runtime checks.

Typical symptoms:

- `make platform-up` fails during image build or pull
- Docker cannot pull or build images (`read-only file system`, `no space left on device`)
- `go run` or link step fails with **There is not enough space on the disk**
- Runtime verification is blocked, but application code is already implemented (e.g. Performance v0.2 `db_pool_*` metrics missing only because containers were not rebuilt)

This is an **environment** issue, not a backend code defect.

## Check Docker disk usage

```bash
make docker-disk-usage
```

Or directly:

```bash
docker system df
```

Review:

| Type | What it means |
|------|----------------|
| Images | Built and pulled container images |
| Containers | Running and stopped containers |
| Local Volumes | Persistent data (includes PostgreSQL) |
| Build Cache | Layers from `docker compose build` |

On Windows, Docker data usually lives inside the WSL2 virtual disk (`docker-desktop-data`). If `docker system df` shows high usage, safe cleanup below often helps before expanding the WSL disk.

## Safe cleanup

Run:

```bash
make docker-clean-safe
```

This executes (in order):

```bash
docker builder prune -af
docker container prune -f
docker image prune -af
docker network prune -f
```

What each command does:

| Command | Effect |
|---------|--------|
| `docker builder prune -af` | Removes build cache (frees the most space after heavy `platform-up --build`) |
| `docker container prune -f` | Removes **stopped** containers |
| `docker image prune -af` | Removes images not used by any container |
| `docker network prune -f` | Removes unused networks |

**These commands do not delete volumes.** PostgreSQL data in `freight_postgres_data` is preserved.

## Be careful with volumes

Do **not** run automatically:

```bash
docker volume prune -f
```

That can delete unused volumes, including local PostgreSQL data.

List volumes first:

```bash
make docker-volumes
```

Or:

```bash
docker volume ls
```

Freight Platform typically uses:

- `freight_postgres_data` — PostgreSQL data for local dev

Only remove a volume if you accept losing that data and can recreate it with:

```bash
make platform-up
make migrate-up
```

To remove a **specific** unused volume manually (only when you are sure):

```bash
docker volume rm <volume_name>
```

Never add `docker volume prune` to Makefile automation in this project.

## Restart Docker Desktop

After cleanup on Windows:

1. **Quit** Docker Desktop (tray icon → Quit)
2. **Start** Docker Desktop again and wait until the engine is ready (`docker ps` works)
3. Re-run the platform and performance checks:

```bash
make platform-up
make migrate-up
make health-check
make generate-db-metrics-traffic
make db-metrics-check
make db-pool-metrics-check
```

## Expected DB pool metrics

After a successful rebuild and API traffic, instrumented services should expose on `/metrics`:

- `db_pool_open_connections`
- `db_pool_in_use_connections`
- `db_pool_idle_connections`
- `db_pool_max_open_connections`

Quick check (Windows):

```powershell
curl http://localhost:8082/metrics | findstr db_pool
```

Linux / WSL / Git Bash:

```bash
curl http://localhost:8082/metrics | grep db_pool
```

Or:

```bash
make db-pool-metrics-check
```

## If Prometheus/Grafana cannot be pulled

Network or disk issues when pulling monitoring images are **not critical** for the backend.

Backend without observability:

```bash
make platform-up
make health-check
```

Observability later (optional profile):

```bash
make observability-up
```

See also: [TROUBLESHOOTING.md](./TROUBLESHOOTING.md), [PERFORMANCE_REPORT_V0.2.md](./PERFORMANCE_REPORT_V0.2.md).
