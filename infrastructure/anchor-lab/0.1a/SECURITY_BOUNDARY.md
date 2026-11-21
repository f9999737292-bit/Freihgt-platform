# Security boundary — anchor lab 0.1A

This boundary describes the Compose file in this directory. Stage 0.1A does not apply it on a server.

## Network

- Project name: `bintrans-anchor-lab`.
- The only network is `bintrans-anchor-lab-internal` with `internal: true` and driver `bridge`.
- The file has no `ports` key and no `network_mode: host`.
- No service mounts `/var/run/docker.sock` or another Docker socket.
- The file requests an internal network and publishes no host port. This stage did not start a container, so it did not prove that a running process is unable to reach another network.
- `project-guard.sh` and `resource-ownership-preflight.sh` reject a remote `DOCKER_HOST` or a remote Docker context before using the daemon. They do not contact staging or production. A direct `docker compose` command does not pass through those checks.

## Database

- Image reference: `mirror.gcr.io/library/postgres:16`. This stage does not pull the image.
- Database name, user, and password come from `ANCHOR_LAB_POSTGRES_DB`, `ANCHOR_LAB_POSTGRES_USER`, and `ANCHOR_LAB_POSTGRES_PASSWORD`.
- `.env.example` contains the placeholder password `change-me`. It is not a production, staging, or E2E credential, and it is not a usable lab password.
- A future `.env` must be the regular file beside this file, not a symlink, use mode `0600`, be owned by the account that runs the wrapper, and contain a lab password other than `change-me` or an empty value. Inside a git work tree it must also be untracked. Without `.git`, as in a standalone `/opt/bintrans/anchor-lab`, the tracked check is skipped and the other checks still apply. A git work tree is not treated as proof that the directory is trusted. `env-file-guard.sh` checks those conditions and does not print the password.
- A standalone copy needs its own integrity check of the received files against the reviewed commit. The `.env` check does not provide that. It does not read a staging or production environment file. This stage does not create that `.env` and does not store a real password.
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
