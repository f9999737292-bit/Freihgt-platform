# BINTRANS E2E Dedicated Host — Security Runbook 0.1F

Stage: `BINTRANS-E2E-STAND-0.1F-DEDICATED-HOST-READINESS`

Baseline of this branch: `0005a659429404fe5a30425e4bbdbba899b6daa0` (`origin/main`, merge of PR #308). This document prepares the host procedure. It does not provision a server, register a runner, or start the stand.

## Host profile

The preflight thresholds are fixed by the controller:

| Check | Threshold | Unit |
| --- | --- | --- |
| Total RAM | `16000000000` bytes | 16 decimal GB (`16 × 10^9`) |
| Available RAM | `6442450944` bytes | 6 GiB (`6 × 1024^3`) |
| vCPU | `4` | processors reported by `nproc` |
| Free space on the Docker data filesystem | `53687091200` bytes | 50 GiB (`50 × 1024^3`) |

Recommended disk capacity is 100 decimal GB. That figure is planning guidance. The gate is the 50 GiB free-space check on the filesystem that holds Docker's data root.

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

The engine listens on one local Unix socket. `scripts/ops/bintrans_e2e_stand/host-preflight.sh` resolves that socket before `docker info`:

- A set `DOCKER_CONTEXT` selects the context whose endpoint is checked. This selection wins over `DOCKER_HOST`.
- `DOCKER_HOST` must be empty or the same `unix://` value as that context. `tcp://` and `ssh://` fail closed.
- The socket path must exist. A missing socket is `BLOCKED`. The script does not start the Docker service.
- Preflight then calls Docker only with `--host` set to that socket. The execution runbook exports the printed `DOCKER_ENDPOINT` as `DOCKER_HOST` and unsets `DOCKER_CONTEXT` for every later command, including `down.sh`.

## Ports and cleanup

The stand file publishes no host ports. Preflight fails if a `ports:` key appears in that file or if an existing container of project `bintrans-e2e-stand-v01a` has a published port.

Cleanup is `scripts/ops/bintrans_e2e_stand/down.sh --env-file <ephemeral-file>`, invoked from the EXIT trap in the execution runbook when stand resources may exist. That command removes only project `bintrans-e2e-stand-v01a`. `docker system prune`, `docker volume prune`, and `docker network prune` are forbidden. A failed cleanup prints `CLEANUP_FAILED` and keeps the env file for a retry.

## Ephemeral secrets

`scripts/ops/bintrans_e2e_stand/ephemeral-env.sh` prints a new database user and password. Write that output outside the repository with mode `0600`, for example `/var/tmp/bintrans-e2e-stand/stand.env`. Do not commit it, do not copy it into the transcript, and do not reuse a staging or production password.

The EXIT trap removes the file on success and on error when no stand resources were created, and after a successful `down.sh` when they were. `CLEANUP_FAILED` leaves the file in place. Do not delete it before `down.sh` succeeds, and do not report the resources as removed.

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
- A published or remote Docker API (`tcp://`, `ssh://`).
- A GitHub self-hosted runner registration.
- Host port bindings for the stand.
- Shared Compose projects, networks, or volumes.
- `docker compose up` before the controller's execution approval.
