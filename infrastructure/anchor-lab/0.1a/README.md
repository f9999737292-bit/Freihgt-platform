# BINTRANS anchor lab 0.1A

This directory is a Compose configuration for a future isolated recovery-anchor laboratory. Stage 0.1A prepares the files only. It does not copy them to a host, create Docker resources, or start containers.

The intended future host directory is `/opt/bintrans/anchor-lab`. The Compose project name is `bintrans-anchor-lab`.

## What this Compose file contains

One service, `postgres`, using `mirror.gcr.io/library/postgres:16`. It is a disposable test database. The image is not pulled in this stage.

The database is attached to the network `bintrans-anchor-lab-internal`. The Compose file sets `internal: true` and does not publish a host port. Runtime isolation was not started in this stage, so a running daemon has not confirmed it. The data location in the file is the named volume `bintrans-anchor-lab-postgres`.

## What this Compose file does not contain

Witness, high-water, and anchor are not services here. No placeholder container stands in for them. A later stage may add a test component only by editing this lab file, joining `bintrans-anchor-lab-internal`, and addressing the database as `postgres:5432` on that network. Until that stage exists, those components are not implemented and this database is not a recovery anchor.

The file does not mount platform migrations, the E2E stand tree, or a Docker socket.

## Project name

`name:` is `bintrans-anchor-lab`. Compose can derive a different project name from a directory such as `0.1a`, because a directory name containing `.` is not a valid Compose project name. `project-guard.sh` accepts only `bintrans-anchor-lab` from `name:`, from `COMPOSE_PROJECT_NAME`, from the env file when that key is present, and from a project selector. It then checks the rendered name. A mismatch exits before Docker.

A direct `docker compose` command does not run this guard. Future start and stop go through `lab-up.sh` and `lab-down.sh`. Those wrappers are not run in this stage.

`resource-ownership-preflight.sh` is a read-only check for a future first start. If the lab network or volume already exists, it stops without adopting or deleting that resource. A model for a later start is not implemented. `lab-up.sh` therefore cannot be used for a second start until that model exists.

## Files

| File | Purpose |
| --- | --- |
| `compose.yaml` | Lab project definition |
| `.env.example` | Placeholder variable names and non-production values |
| `project-guard.sh` | Rejects every project source other than `bintrans-anchor-lab` |
| `env-file-guard.sh` | Rejects a missing, tracked, world-readable, empty, or placeholder lab password |
| `resource-ownership-preflight.sh` | Read-only first-start check; does not adopt or delete resources |
| `lab-up.sh` | Future first start through the guards; not run in this stage |
| `lab-down.sh` | Future cleanup of labeled lab resources only; not run in this stage |
| `SECURITY_BOUNDARY.md` | Network, capability, and credential boundary |
| `E2E_NON_INTERFERENCE_CHECKLIST.md` | Separation from the E2E stand |
| `LAB_PREFLIGHT.md` | Checks recorded for this stage |
| `LAB_STOP_AND_CLEANUP.md` | Future cleanup limited to this project |

## Status

`MANAGED_ANCHOR=NOT_SELECTED`. `IMPLEMENTATION_GATE=CLOSED`. `PRODUCTION_CONSUMER_READINESS=BLOCKED`.
