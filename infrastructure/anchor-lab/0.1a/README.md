# BINTRANS anchor lab 0.1A

This directory is a Compose configuration for a future isolated recovery-anchor laboratory. Stage 0.1A prepares the files only. It does not copy them to a host, create Docker resources, or start containers.

The intended future host directory is `/opt/bintrans/anchor-lab`. The Compose project name is `bintrans-anchor-lab`.

## What this Compose file contains

One service, `postgres`, using `mirror.gcr.io/library/postgres:16`. It is a disposable test database. The image is not pulled in this stage.

The database is attached only to the network `bintrans-anchor-lab-internal`. That network has `internal: true`. The service publishes no host ports, so PostgreSQL port 5432 stays inside the network. The data directory is the named volume `bintrans-anchor-lab-postgres`.

## What this Compose file does not contain

Witness, high-water, and anchor are not services here. No placeholder container stands in for them. A later stage may add a test component only by editing this lab file, joining `bintrans-anchor-lab-internal`, and addressing the database as `postgres:5432` on that network. Until that stage exists, those components are not implemented and this database is not a recovery anchor.

The file does not mount platform migrations, the E2E stand tree, or a Docker socket.

## Project name

`name:` is `bintrans-anchor-lab`. Compose can derive a different project name from a directory such as `0.1a`, because a directory name containing `.` is not a valid Compose project name. The rendered name is the `name:` value, or `-p bintrans-anchor-lab` when that flag is passed. Both must stay `bintrans-anchor-lab`. Confirm the rendered name with `docker compose config` before any future start. This stage does not start the project.

## Files

| File | Purpose |
| --- | --- |
| `compose.yaml` | Lab project definition |
| `.env.example` | Placeholder variable names and non-production values |
| `SECURITY_BOUNDARY.md` | Network, capability, and credential boundary |
| `E2E_NON_INTERFERENCE_CHECKLIST.md` | Separation from the E2E stand |
| `LAB_PREFLIGHT.md` | Checks recorded for this stage |
| `LAB_STOP_AND_CLEANUP.md` | Future cleanup limited to this project |

## Status

`MANAGED_ANCHOR=NOT_SELECTED`. `IMPLEMENTATION_GATE=CLOSED`. `PRODUCTION_CONSUMER_READINESS=BLOCKED`.
