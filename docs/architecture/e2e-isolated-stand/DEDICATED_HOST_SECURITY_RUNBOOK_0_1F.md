# BINTRANS E2E Dedicated Host — Security Runbook 0.1F

Stage: `BINTRANS-E2E-STAND-0.1F-DEDICATED-HOST-READINESS`

Baseline: `0005a659429404fe5a30425e4bbdbba899b6daa0` (`origin/main`, merge of PR #308). This document prepares the host procedure. It does not provision a server, register a runner, or start the stand.

## Boundary

The machine is an isolated E2E host. It has no route to staging or production networks, no staging DNS, and no VPN into those environments. It does not receive `staging.env`, files from `/protected`, SSH keys for staging, or registry credentials used by staging.

The stand Compose project remains `bintrans-e2e-stand-v01a`. Its network is `bintrans-e2e-stand` with `internal: true`. It does not join `freight-platform-network` and does not mount `freight_postgres_data`.

## Access

- SSH accepts public-key authentication only. Password login and root login stay disabled.
- The allowlist is the operator addresses named by the controller. No other source address reaches port 22.
- One Unix user runs the stand. That user is not a shared account.
- Membership of the `docker` group is root-equivalent, because the Docker socket can start privileged containers. Only that operator is added to the group, and only for the approved run window.
- The host is not registered as a GitHub Actions self-hosted runner. The repository is public, so a permanent runner would expose the Docker socket to repository workflows.

## Docker API

The engine listens on the local Unix socket only. `tcp://` endpoints, `0.0.0.0:2375`, and published Docker TLS ports are rejected by `scripts/ops/bintrans_e2e_stand/host-preflight.sh`. Do not set `DOCKER_HOST` to a remote daemon.

## Ports and cleanup

The stand file publishes no host ports. Preflight fails if a `ports:` key appears in that file or if an existing container of project `bintrans-e2e-stand-v01a` has a published port.

Cleanup is `scripts/ops/bintrans_e2e_stand/down.sh --env-file <ephemeral-file>`. That command removes only project `bintrans-e2e-stand-v01a`. `docker system prune`, `docker volume prune`, and `docker network prune` are forbidden.

## Ephemeral secrets

`scripts/ops/bintrans_e2e_stand/ephemeral-env.sh` prints a new database user and password. Write that output outside the repository with mode `0600`, for example `/var/tmp/bintrans-e2e-stand/stand.env`. Do not commit it, do not copy it into the transcript, and do not reuse a staging or production password.

After `down.sh` finishes, delete the env file. A failed run deletes it as well, once any allowed log collection is done.

## Egress

Allow outbound HTTPS only for the checkout and the stand images:

- `github.com` and `codeload.github.com`
- `mirror.gcr.io` for `postgres:16`
- `docker.redpanda.com` for Redpanda `v24.2.8`
- Docker Hub endpoints required by `golang:1.25-bookworm` and `migrate/migrate:v4.17.1`

Do not open a path to staging or production APIs, databases, or brokers.

## Updates

Apply OS security updates before the run window, then freeze Docker Engine and the Compose plugin. Record `docker version` and `docker compose version` in the execution evidence. Do not upgrade them during the run.

## Prohibited on this host

- Staging or production credentials and kubeconfigs.
- A published Docker API.
- A GitHub self-hosted runner registration.
- Host port bindings for the stand.
- Shared Compose projects, networks, or volumes.
- `docker compose up` before the controller's execution approval.
