# Stop and cleanup — anchor lab 0.1A

Stage 0.1A starts nothing, so there is nothing to stop. The future cleanup command is the wrapper below. It is not run in this stage. A direct `docker compose down` does not run the guard and is not a protected cleanup.

Run the wrapper from the lab directory that contains this `compose.yaml`. On the future host that directory is `/opt/bintrans/anchor-lab`.

```text
bash lab-down.sh --env-file .env
```

`lab-down.sh` accepts no project override and no other Compose file. It runs `env-file-guard.sh` and `project-guard.sh`, then inspects resources separately. A network is eligible only when its name is `bintrans-anchor-lab-internal`, `com.docker.compose.project` is `bintrans-anchor-lab`, and `com.docker.compose.network` is `anchor-lab`. A volume is eligible only when its name is `bintrans-anchor-lab-postgres`, the project label is `bintrans-anchor-lab`, and `com.docker.compose.volume` is `anchor-lab-postgres`. Compose v5.1.4 and v5.7.0 do not put `com.docker.compose.project.config_files` on those two resources, so that label is not required for them. If it is present and names another file, the resource is rejected. A container is eligible only when its project label is `bintrans-anchor-lab`, its service label is `postgres`, its config-files label is this `compose.yaml`, and its name is the lab postgres container name. A missing label, a foreign label, or a contradictory label stops the script before any deletion. A partial set is not cleaned up piece by piece: one unknown resource rejects the whole cleanup.

The wrapper's own deletion command is `docker compose -p bintrans-anchor-lab -f compose.yaml down -v` after those checks. `-v` deletes the lab test volume `bintrans-anchor-lab-postgres` and the test data in it. The wrapper does not pass `--remove-orphans`. The first-start preflight still rejects a network or volume that already has the lab name, including one whose labels match. Cleanup and first start are different checks.

The inspect and the later `down -v` are separate calls. A mock test does not prove that another process cannot change the resources between them. Before a future deployment, the operator must choose a lock or another daemon-side precondition. This stage does not implement that mechanism. `CLEANUP_TOCTOU=NOT_PROVEN_RUNTIME`.

These commands are forbidden:

```text
docker system prune
docker volume prune
docker network prune
docker compose down
```

`docker compose down` without the wrapper is forbidden. Do not point a cleanup command at `infrastructure/docker-compose/docker-compose.bintrans-e2e-stand-v0.1a.yml`. A substituted `COMPOSE_PROJECT_NAME` fails in the project guard before Docker.
