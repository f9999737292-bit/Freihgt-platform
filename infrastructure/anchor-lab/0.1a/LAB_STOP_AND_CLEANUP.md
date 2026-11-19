# Stop and cleanup — anchor lab 0.1A

Stage 0.1A starts nothing, so there is nothing to stop. The command below is the only cleanup command reserved for a future run of this lab. It is not run in this stage.

Run it from the lab directory that contains this `compose.yaml`. On the future host that directory is `/opt/bintrans/anchor-lab`.

```text
docker compose --env-file .env -p bintrans-anchor-lab -f compose.yaml down -v --remove-orphans
```

Before running it, render the project and confirm that `name` is `bintrans-anchor-lab`. The command removes containers, the network `bintrans-anchor-lab-internal`, and the volume `bintrans-anchor-lab-postgres` belonging to that project. It does not select `bintrans-e2e-stand-v01a`, `bintrans-ct-staging`, or the platform Compose project.

These commands are forbidden:

```text
docker system prune
docker volume prune
docker network prune
docker compose down
```

`docker compose down` without `-p bintrans-anchor-lab` and without `-f compose.yaml` is forbidden. Do not add a project-wide prune, and do not point `-f` at `infrastructure/docker-compose/docker-compose.bintrans-e2e-stand-v0.1a.yml`.
