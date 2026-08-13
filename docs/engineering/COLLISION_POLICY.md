# Collision Prevention Policy

Rules for high-collision files and shared artifacts in freight-platform.

## High-collision files (verified in repo)

Changes to any path below MUST be declared in the Task Contract **Allowed paths** and flagged for orchestrator review.

### Root / workspace

| File | Risk |
|------|------|
| `Makefile` | CI targets, migrate commands, platform orchestration |
| `go.work` | Go workspace module set |
| `package.json` | Root npm scripts |
| `pnpm-workspace.yaml` | Frontend workspace membership |
| `README.md` | Platform entry documentation |

### Infrastructure

| File | Risk |
|------|------|
| `infrastructure/docker-compose/docker-compose.yml` | Core stack composition |
| `infrastructure/docker-compose/docker-compose.staging-shadow.yml` | Staging shadow stack |
| `infrastructure/docker-compose/docker-compose.rebuild-acceptance.yml` | Rebuild acceptance stack |
| `infrastructure/migrations/*.sql` | Global migration order |

### Contracts

| File | Risk |
|------|------|
| `packages/openapi/openapi.yaml` | Aggregated public API |
| `packages/openapi/*-service.yaml` | Service-level OpenAPI |
| `packages/openapi/schemas/**` | Shared schemas |

### Gateway (shared routing / RBAC)

| File | Risk |
|------|------|
| `services/api-gateway/internal/http/router.go` | Route registration |
| `services/api-gateway/internal/http/proxy.go` | Upstream proxy wiring |
| `services/api-gateway/internal/shipmentrbac/**` | Shipment RBAC policies |

### CI

| File | Risk |
|------|------|
| `.github/workflows/ci.yml` | Primary CI pipeline |
| `.github/workflows/ci-manual.yml` | Manual recovery CI |

## Rule

> Changing a high-collision file MUST be explicitly declared in the Task Contract and approved by the orchestrator.

## OpenAPI collision rule

**Policy: contract owner + sequential integration**

When multiple tasks touch `packages/openapi/**`:

1. Orchestrator assigns **one contract-owner task** (often architect-led) that freezes the API shape.
2. Backend and frontend tasks reference the frozen contract SHA.
3. Parallel OpenAPI edits on separate branches are **not allowed** without that freeze.
4. Integration task merges contract + implementations in order: contract → backend → frontend → validation.

Workflow:

```
API requirement → contract definition → OpenAPI → backend → frontend → contract validation
```

Backend and frontend may run in parallel **only after** contract stabilization.

## Database migration collision rule

**Policy: central numbered migrations + single coordinator**

Migrations live in `infrastructure/migrations/` as paired files:

```text
NNNNNN_<description>.up.sql
NNNNNN_<description>.down.sql
```

Ordering is by numeric prefix (`000001`, `000002`, …). **Do not hard-code the current maximum** — it changes as migrations merge to `main`.

Determine the next number dynamically before assigning work:

```powershell
# PowerShell — highest existing migration prefix
(Get-ChildItem infrastructure/migrations/*.up.sql |
  ForEach-Object { if ($_.BaseName -match '^(\d{6})_') { [int]$Matches[1] } } |
  Measure-Object -Maximum).Maximum
# Next migration = maximum + 1, zero-padded to 6 digits (e.g. 000020)
```

Rules:

1. **One migration coordinator task** per integration batch when multiple features need schema changes.
2. Coordinator runs the discovery step above on the task **base branch/SHA**, finds highest `NNNNNN`, assigns next number to each pending migration **before** parallel implementation starts.
3. Parallel agents do **not** independently pick migration numbers.
4. Never rewrite applied migration history or renumber merged migrations.
5. Service-local migration tests (e.g. `migration_000014_test.go`) must reference the assigned central migration ID.
6. Apply via Makefile: `make migrate-up`, `make migrate-version` — not ad hoc SQL in production paths.

If two tasks need migrations simultaneously:

```
Orchestrator → migration coordinator assigns N+1, N+2 → parallel service code → integration merge in numeric order
```

## Task registry

Active tasks register allowed paths in `parallel/tasks/` to detect overlap. See `parallel/tasks/README.md`.

## Dependency classes

| Class | Meaning |
|-------|---------|
| parallel-safe | Disjoint allowed paths, no shared contract edits |
| dependent | Requires output or SHA from another task |
| high-collision | Touches shared files above; serialize or use coordinator |
| integration | Merge/review only; no new feature scope |
