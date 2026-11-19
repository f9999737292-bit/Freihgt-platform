# Lab preflight — anchor lab 0.1A

Stage 0.1A runs the static checks below on the workstation copy. It does not copy the directory to `/opt/bintrans/anchor-lab` and does not contact `bintrans-e2e-stand-v01a`.

## Render the project

From this directory:

```text
docker compose --env-file .env.example -p bintrans-anchor-lab -f compose.yaml config
```

`docker compose config` renders the file. It does not create a network, volume, or container, and it does not pull `mirror.gcr.io/library/postgres:16`.

Accept the render only when all of the following are true:

1. The rendered document `name` is `bintrans-anchor-lab`.
2. The only service key is `postgres`.
3. No `ports` key is present, and no `network_mode` is `host`.
4. The network `bintrans-anchor-lab-internal` is `internal: true`.
5. The only volume name is `bintrans-anchor-lab-postgres`.
6. No bind mount, Docker socket, or `container_name` is present.
7. No `privileged: true` is present.
8. The text does not contain `bintrans-e2e-stand-v01a`, `bintrans-ct-staging`, `freight-platform-network`, or `freight_postgres_data`.

The directory name `0.1a` is not a valid Compose project name. A render whose `name` is anything other than `bintrans-anchor-lab` fails this preflight.

## Workstation result recorded for stage 0.1A

`docker compose version` reported Docker Compose v5.1.4. The render command above was run from this directory with `.env.example`. The rendered `name` was `bintrans-anchor-lab`. The render contained one service, `postgres`, network `bintrans-anchor-lab-internal` with `internal: true`, and volume `bintrans-anchor-lab-postgres`. It contained no `ports`, `network_mode`, `container_name`, `privileged`, or Docker socket. No container was started and no Docker resource was created.

## Explicitly out of this stage

Do not run `docker compose up`, `docker compose create`, `docker compose pull`, or `docker compose down`. Do not SSH to the lab host. A future stage repeats this render on the host before any start, with the working directory `/opt/bintrans/anchor-lab` and with a lab `.env` that replaces `change-me`.
