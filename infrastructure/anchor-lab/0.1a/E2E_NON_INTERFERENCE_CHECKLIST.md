# E2E non-interference checklist — anchor lab 0.1A

The existing stand Compose project is `bintrans-e2e-stand-v01a`, defined by `infrastructure/docker-compose/docker-compose.bintrans-e2e-stand-v0.1a.yml`. This lab does not edit that file, its Dockerfile, its scripts, or `infrastructure/migrations`.

| Check | Result in this stage |
| --- | --- |
| Repository path is `infrastructure/anchor-lab/0.1a`, not the E2E Compose path | PASS |
| Future host directory is `/opt/bintrans/anchor-lab` | DOCUMENTED_ONLY |
| `/opt/bintrans/e2e-stand` is not a volume source and is not a command working directory | PASS |
| Compose project is `bintrans-anchor-lab` | PASS |
| Project is not `bintrans-e2e-stand-v01a` and not `bintrans-ct-staging` | PASS |
| No bind mount of the E2E tree, `/opt/bintrans/e2e-stand`, `/opt/bintrans/freight-platform`, or `/opt/bintrans/control-tower-staging` | PASS |
| Network name is `bintrans-anchor-lab-internal` | PASS |
| Network is not `bintrans-e2e-stand` and not `freight-platform-network` | PASS |
| Volume name is `bintrans-anchor-lab-postgres` | PASS |
| Volume is not `bintrans-e2e-stand-postgres`, `bintrans-e2e-stand-documents`, `bintrans-e2e-stand-redpanda`, or `freight_postgres_data` | PASS |
| No `container_name`, including any `freight_*` or `bintrans_e2e_*` name | PASS |
| Platform migrations are not mounted | PASS |
| Cleanup text names this project and this `compose.yaml` | PASS |
| `lab-down.sh` refuses a foreign project, network, volume, or label before deletion | PASS |
| Empty, shifted, malformed, or duplicate label metadata is refused before deletion | PASS (mock Docker only) |
| Standalone host copy is checked for integrity against the reviewed commit | NOT_DEFINED |
| A direct `docker compose` command is outside the lab guard | DOCUMENTED |
| `docker system prune` and `docker volume prune` are absent from the lab scripts | PASS |
| Existing E2E Compose file and E2E scripts were not modified | PASS |

Server-side confirmation that the live Docker daemon has no shared resource is `NOT_RUN`. This stage has no SSH access and creates no Docker resource.
