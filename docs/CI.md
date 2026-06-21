# CI v0.1

GitHub Actions workflow for [freight-platform](https://github.com/f9999737292-bit/Freihgt-platform).

Workflow file: [`.github/workflows/ci.yml`](../.github/workflows/ci.yml)

## Triggers

- `push` to `main`
- `pull_request` targeting `main`

## What CI v0.1 checks

| Job | Purpose |
| --- | ------- |
| **repository-safety** | Ensures no `.env` or `.env.*` files are tracked by git (`.env.example` is allowed) |
| **backend-go-check** | Runs `go test ./...` in `packages/shared-go` and each `services/*` module (Go 1.22 matrix) |
| **frontend-web-admin-build** | `npm ci` + `npm run build` for `apps/web-admin` (Node.js 20 LTS) |
| **scripts-check** | `bash -n` syntax validation for integration/dev shell scripts |

## What CI v0.1 does not check

- Docker Compose / `make platform-up`
- Database migrations at runtime
- `make integration-smoke-test` / full business flow
- `make seed-dev-admin` against live Postgres
- UI browser tests / Playwright
- Observability stack (Prometheus/Grafana)
- Performance / k6 load tests

These require a Docker runtime, secrets, or long-running services.

## Why Docker smoke test is CI v0.2

The integration smoke test (`tests/integration/smoke-test.sh`) needs:

- Docker Desktop / Linux Docker engine
- PostgreSQL + 8 backend containers healthy
- Stable WSL/Windows networking for curl + psql

CI v0.1 is intentionally **static checks only** — fast feedback on every push without cloud Docker cost or flaky infra.

**Planned for CI v0.2:**

- Docker-based job (or self-hosted runner) with `make platform-up-no-build`
- `make health-check`
- `make integration-smoke-test` via Git Bash-compatible runner

## How to view results

1. Open the repository on GitHub
2. Go to **Actions** tab
3. Select the **CI** workflow run for your commit or PR
4. Expand jobs: `repository-safety`, `backend-go-check`, `frontend-web-admin-build`, `scripts-check`

Green check on all jobs = CI v0.1 passed.

## Local equivalents

```powershell
cd D:\Projects\freight-platform

# repository-safety (manual)
git ls-files | findstr /R "\.env"

# backend (example)
cd packages\shared-go
go test ./...

# frontend
cd apps\web-admin
npm ci
npm run build

# scripts (Git Bash)
bash -n scripts/dev/seed_dev_admin.sh
```

Full runtime verification remains local: `make health-check`, `make integration-smoke-test`.

## Related docs

- [DEV_SEED.md](./DEV_SEED.md)
- [RUNTIME_VERIFICATION_REPORT_V0.1.md](./RUNTIME_VERIFICATION_REPORT_V0.1.md)
- [DOCKER_WSL_STABILITY.md](./DOCKER_WSL_STABILITY.md)
