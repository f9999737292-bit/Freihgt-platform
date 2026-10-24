# BINTRANS E2E Isolated Stand 0.1B — Implementation Runbook

Stage: `BINTRANS-E2E-ISOLATED-STAND-0.1B-IMPLEMENTATION`

Baseline: `968388ece4a6eaaf0ef835799b86ae41f2dc4ed3` (`origin/main`, merge of PR #306). PR #305 is the parent contract. Migration head remains `000105_platform_postgres_role_foundation_v0_2_i1`.

This stage adds the stand files and the fail-closed broker switch. It does not start Docker, create volumes, apply SQL, or call staging.

## Files

| Piece | Path |
| --- | --- |
| Compose project `bintrans-e2e-stand-v0.1a` | `infrastructure/docker-compose/docker-compose.bintrans-e2e-stand-v0.1a.yml` |
| Runner image | `infrastructure/docker/bintrans-e2e-stand/Dockerfile` |
| Runner entrypoint | `infrastructure/docker/bintrans-e2e-stand/runner-entrypoint.sh` |
| Topic creation | `infrastructure/docker/bintrans-e2e-stand/topic-init.sh` |
| Project cleanup | `scripts/ops/bintrans_e2e_stand/down.sh` |
| Ephemeral database variables | `scripts/ops/bintrans_e2e_stand/ephemeral-env.sh` |
| Broker and schema guards | `tests/bintrans-e2e-transport-flow/broker_policy.go`, `stand_guard.go` |
| Opt-in workflow | `.github/workflows/bintrans-e2e-stand-opt-in.yml` |

The Compose project has five services: `postgres`, `redpanda`, `topic-init`, `migrate`, and `runner`. contract-rate, transport-order, rfx, shipment, document, billing, and control-tower stay child processes of the existing harness inside the runner. They are not Compose services.

## Boundaries

- Network key `bintrans-e2e-stand` with `internal: true`. No `freight-platform-network`.
- Named volumes `bintrans-e2e-stand-postgres`, `bintrans-e2e-stand-documents`, and `bintrans-e2e-stand-redpanda`. No `freight_postgres_data`.
- No `container_name` and no host bindings.
- No `staging.env`, no `/protected` path, and no committed database password.
- Redpanda advertises only `internal://redpanda:9092`. The in-container `rpk` healthcheck uses `localhost:9092` and is not the client bootstrap.
- Migrate image `migrate/migrate:v4.17.1` runs `up` on the mounted `infrastructure/migrations` tree. The enforcement-gate SQL file is not mounted.
- Runner image download of Go modules happens at build time. The running container sets `GOPROXY=off`.

`BINTRANS_E2E_STAND_MODE=stand` and `BINTRANS_E2E_KAFKA_BROKERS=redpanda:9092` are set on the runner. No stand script and no stand Compose service sets `BINTRANS_E2E_KFAKE_ALLOW`. The runner entrypoint exits if that variable is present.

`TEST_DATABASE_URL` is assembled at runtime from the ephemeral user, password, host `postgres`, and database name. The harness rejects any other host before seed and does not print the URL. In stand mode it does not call `CREATE DATABASE` or `applyMigrations`. It reads `schema_migrations` and refuses a version other than 105, a dirty migration, `trg_new_activation_authorization`, an extra `LOGIN` role, or a runtime role that can log in.

`TMS_RELATIONSHIP_ACTIVATION_ENABLED` stays `false`.

CI job `bintrans-e2e-transport-flow` keeps the current scenario by setting `BINTRANS_E2E_STAND_MODE=ci` and `BINTRANS_E2E_KFAKE_ALLOW=1`. That job is not stand evidence. The same test package runs the fail-closed cases without Docker.

Workflow `bintrans-e2e-stand-opt-in` is `workflow_dispatch` only. It is not added to `ci.yml` and is not a required check. This stage did not dispatch it.

## Later authorized run

Do not run these commands in stage 0.1B.

1. Generate variables with `scripts/ops/bintrans_e2e_stand/ephemeral-env.sh` and keep the output outside the repository.
2. Confirm the project before any Compose command:

```text
bash scripts/ops/bintrans_e2e_stand/project-guard.sh --project bintrans-e2e-stand-v0.1a
```

The guard reads `name:` from the stand Compose file, `COMPOSE_PROJECT_NAME` when it is set, and the `--project` argument. The resolved project must be `bintrans-e2e-stand-v0.1a`. Any other name, including a staging project name, exits before Docker.

3. Build and run only this project:

```text
docker compose --env-file <ephemeral-file> -p bintrans-e2e-stand-v0.1a -f infrastructure/docker-compose/docker-compose.bintrans-e2e-stand-v0.1a.yml up --build --abort-on-container-exit --exit-code-from runner
```

4. Remove only this project with the same ephemeral file:

```text
bash scripts/ops/bintrans_e2e_stand/down.sh --env-file <ephemeral-file>
```

`down.sh` checks that the file exists and is readable, requires the three ephemeral database keys without printing them, runs the project guard, and then executes `docker compose --env-file <ephemeral-file> -p bintrans-e2e-stand-v0.1a -f infrastructure/docker-compose/docker-compose.bintrans-e2e-stand-v0.1a.yml down -v --remove-orphans`. A missing file, an unreadable file, or a different project exits before that command. The script does not accept a caller project override and does not prune volumes, networks, or another project.

In stand mode the harness sets `SHIPMENT_HTTP_LISTEN_HOST=127.0.0.1` and `CONTROL_TOWER_HTTP_LISTEN_HOST=127.0.0.1`. An empty host keeps the existing `:<port>` bind used by normal deployment. Any other host is rejected.

## Broker results covered without Docker

| Case | Result |
| --- | --- |
| `stand`, broker unset | FAIL before seed |
| `stand`, broker contains `localhost`, `127.0.0.1`, or `kfake` | FAIL before seed |
| `stand`, broker is `redpanda:9092`, dial fails | FAIL before seed, `kfake` is not authorized |
| `stand`, broker is `redpanda:9092`, dial succeeds | Gate opens. The HTTP scenario was not executed |
| `ci` and `BINTRANS_E2E_KFAKE_ALLOW=1` | `kfake` path stays available |
| missing mode, or `stand` with the allow flag | FAIL before seed |

## What this stage executed

Repository identity, `origin/main`, the PR #305 and PR #306 merge commits, the migration inventory, the 0.1A contract, Go contract tests, and draft CI. No `docker compose` command. No volume create or delete. No SQL session. No credential creation. No staging change. No merge and no deployment.
