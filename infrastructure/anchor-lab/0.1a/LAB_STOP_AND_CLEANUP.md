# Stop and cleanup — anchor lab 0.1A

Stage 0.1A starts nothing, so there is nothing to stop. The future cleanup command is the wrapper below. It is not run in this stage. A direct `docker compose down` does not run the guard and is not a protected cleanup.

Run the wrapper from the lab directory that contains this `compose.yaml`. On the future host that directory is `/opt/bintrans/anchor-lab`.

```text
bash lab-down.sh --env-file .env
```

`lab-down.sh` accepts no project override and no other Compose file. It runs `env-file-guard.sh` and `project-guard.sh`, then inspects the network, the volume, and container labels. A resource is eligible only when its Compose project label is `bintrans-anchor-lab` and its config-file label is this `compose.yaml`. A missing label, a foreign label, or a contradictory label stops the script before deletion.

The wrapper's own deletion command is `docker compose -p bintrans-anchor-lab -f compose.yaml down -v` for those owned resources. `-v` deletes the lab test volume `bintrans-anchor-lab-postgres` and the test data in it. The wrapper does not pass `--remove-orphans`.

These commands are forbidden:

```text
docker system prune
docker volume prune
docker network prune
docker compose down
```

`docker compose down` without the wrapper is forbidden. Do not point a cleanup command at `infrastructure/docker-compose/docker-compose.bintrans-e2e-stand-v0.1a.yml`. A substituted `COMPOSE_PROJECT_NAME` fails in the project guard before Docker.
