# Security boundary — anchor lab 0.1A

This boundary describes the Compose file in this directory. Stage 0.1A does not apply it on a server.

## Network

- Project name: `bintrans-anchor-lab`.
- The only network is `bintrans-anchor-lab-internal` with `internal: true` and driver `bridge`.
- The file has no `ports` key and no `network_mode: host`.
- No service mounts `/var/run/docker.sock` or another Docker socket.
- A process outside this network cannot reach the database through a published port. A process on this network cannot open a connection to production, staging, or the E2E stand.

## Database

- Image reference: `mirror.gcr.io/library/postgres:16`. This stage does not pull the image.
- Database name, user, and password come from `ANCHOR_LAB_POSTGRES_DB`, `ANCHOR_LAB_POSTGRES_USER`, and `ANCHOR_LAB_POSTGRES_PASSWORD`.
- `.env.example` contains the placeholder password `change-me`. It is not a production, staging, or E2E credential. Stage 0.1A does not create a `.env` file and does not copy the example to a host.
- The file contains no production DSN and no connection string to another Compose project.
- The volume `bintrans-anchor-lab-postgres` is the only data location. It is not marked `external`.

## Process limits

- `privileged` is absent.
- `security_opt` is `no-new-privileges:true`.
- Capabilities drop `ALL`, then add `CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `SETGID`, and `SETUID` for the official PostgreSQL image entrypoint. This set was not confirmed by starting the container.
- CPU limit `0.50`, memory limit `512M`, process limit `128`, shared memory `64mb`.
- JSON logs are limited to three files of `10m`.
- Restart policy is `no`.
- No `container_name` is set, so Compose names the container from the project and the service key `postgres`.

## Components that are not in this file

Witness, high-water, and anchor are not declared. Their absence is intentional. This PostgreSQL service is a future test database, not an implemented recovery anchor.

`MANAGED_ANCHOR=NOT_SELECTED`. `P3B_ALL_STORES_ROLLBACK=NOT_FIXED`. `ANCHOR_FAILURE_DOMAIN=NOT_PROVEN`. `IMPLEMENTATION_GATE=CLOSED`. `PRODUCTION_CONSUMER_READINESS=BLOCKED`.
