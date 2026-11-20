# Lab preflight — anchor lab 0.1A

Stage 0.1A runs the static checks below on the workstation copy. It does not copy the directory to `/opt/bintrans/anchor-lab` and does not contact `bintrans-e2e-stand-v01a`.

## Render the project

A direct `docker compose` command does not run the lab guard. The guarded render is:

```text
bash project-guard.sh --env-file .env -p bintrans-anchor-lab -f compose.yaml
```

The guard reads `name:`, `COMPOSE_PROJECT_NAME`, and the env file, rejects any other project selector, and only then runs `docker compose config`. It does not print the rendered document. `docker compose config` does not create a network, volume, or container, and it does not pull `mirror.gcr.io/library/postgres:16`.

Before a future first start, run the read-only preflight:

```text
bash resource-ownership-preflight.sh
```

The preflight and `lab-down.sh` need a working `jq` on the host. They read each resource as one JSON object from `docker inspect --format '{{json .}}'` and read each label separately, so an empty label cannot take the value of the next field. Malformed JSON, more than one object, duplicate keys, or a label of the wrong type stops the check. Without `jq` they stop before inspecting Docker. Host availability of `jq` is not checked in this stage.

If `bintrans-anchor-lab-internal` or `bintrans-anchor-lab-postgres` already exists, the preflight stops. It does not adopt the resource and does not delete it. This is not a model for a later start. `bash lab-up.sh --env-file .env` is the future first-start wrapper and is not run in this stage.

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

Do not run `docker compose up`, `docker compose create`, `docker compose pull`, `docker compose down`, or `lab-up.sh`. Do not SSH to the lab host. A future stage uses the wrappers from `/opt/bintrans/anchor-lab` with a lab `.env` that is a regular file, not a symlink, mode `0600`, owned by the account that runs the wrapper, untracked when the directory is a git work tree, and does not contain `change-me`. The placeholder in `.env.example` is rejected for that file. That directory may have no `.git`. Before any wrapper runs there, the received files need an integrity check against the reviewed commit; this stage does not define one.
