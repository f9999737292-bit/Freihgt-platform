# BINTRANS multi-agent Docker policy

Local Docker Desktop is shared by every Git worktree. Compose project name alone does not isolate stacks while `container_name: freight_*` is set. Several worktrees plus `restart: unless-stopped` previously produced one mixed persistent stack.

This policy separates one canonical local stack from temporary agent environments. It does not change Compose runtime behavior.

## Owners

| Agent | Scope | Worktree | Temporary project prefix |
|---|---|---|---|
| A | Platform, staging, release | its own worktree | `bintrans-a-<task>` |
| B | EDO | its own worktree | `bintrans-b-<task>` |
| C | TMS | its own worktree | `bintrans-c-<task>` |
| D | NLO/BNO | its own worktree | `bintrans-d-<task>` |

No agent owns a permanent shared local stack. A feature worktree must not create, rebuild, or replace containers named `freight_*`.

## Canonical stack

```text
OWNER=CANONICAL
SOURCE=origin/main
PURPOSE=normal local development
TARGET_PROJECT_NAME=bintrans-dev
```

The canonical stack is started only from a checkout of `origin/main`, or from a release/staging worktree that an operator explicitly approved. Feature branches do not update it.

Live resources that already exist keep their current identity until a separate cleanup stage:

```text
COMPOSE_PROJECT=docker-compose
CONTAINER_NAMES=freight_*
VOLUME=docker-compose_freight_postgres_data
```

Do not mount that volume from an agent project. Do not adopt `bintrans-dev` by running Compose again while the existing `freight_*` containers are present: the fixed names would collide. Renaming the live project is a later stage.

Ordinary TMS development after the lightweight targets landed uses `make tms-lite-up`, `make tms-api-up`, and `make tms-e2e-up` from the canonical checkout only. Those targets still address `freight_*`. A feature image test uses `bintrans-c-<task>` instead.

## Agent environments

Project names:

```text
bintrans-a-<task>
bintrans-b-<task>
bintrans-c-<task>
bintrans-d-<task>
```

Examples: `bintrans-c-tms-mstop-01f`, `bintrans-d-nlo-05b`.

Do not use the Compose project name `docker-compose` for a temporary environment.

| Agent | Name prefix | Rule |
|---|---|---|
| A | `bintrans-a-staging-*` for staging/release exercises | Do not mix staging containers into the local `freight_*` stack. |
| B | `bintrans-b-edo-*` | Do not reuse `freight_document_service` for a feature image. |
| C | `bintrans-c-tms-*` | Feature image tests are a separate project. Canonical `tms-lite` / `tms-api` / `tms-e2e` stay on `origin/main`. |
| D | `bintrans-d-nlo-*` | Do not replace canonical shipment or NLO containers with a feature image under the same name. |

An agent environment must set all of the following. Changing only the project name is not enough.

```text
container_name=bintrans-<agent>-<task>-<service>
volume=bintrans-<agent>-<task>-postgres-data
network=bintrans-<agent>-<task>-network
restart=no
```

Example volume: `bintrans-c-tms-postgres-data`.

The existing overlay `infrastructure/docker-compose/docker-compose.a21-validate.yml` is the pattern: `container_name`, `ports: !override`, and `volumes: !override`, plus `COMPOSE_PROJECT_NAME`. Copy that shape into an untracked override for the task. Do not commit a one-off override, and do not edit the canonical `docker-compose.yml` for a single test.

### Ports

Do not publish the canonical host ports (`5432`, `8080`–`8090`, `9090`, `3001`, `19092`) from an agent environment while the canonical stack may use them.

Prefer, in order:

1. No host port, when the test talks only on the Compose network.
2. A host port outside the canonical set and outside Windows excluded ranges (`netsh interface ipv4 show excludedportrange protocol=tcp`).
3. A dynamic host port (`"0:5432"`) when a fixed port is unnecessary.

### Networks

One network per task: `bintrans-<agent>-<task>-network`. Do not attach a temporary stack to `freight-platform-network` or to another agent's network.

### Restart

```text
AGENT_TEMP_RESTART_POLICY=no
```

Temporary containers must not come back when Docker Desktop starts. The canonical file may keep `restart: unless-stopped`. That policy is why a manually started canonical container returns after a Docker restart until it is stopped. Agent overrides must set `restart: "no"` and must not rely on a later global policy edit.

### Labels

Every temporary container, volume, and network:

```text
com.bintrans.owner=agent-a|agent-b|agent-c|agent-d
com.bintrans.task=<task-id>
com.bintrans.lifecycle=temporary
com.bintrans.git-sha=<full HEAD of the worktree that created it>
```

Also record in the task handoff:

```text
GIT_WORKTREE=
GIT_BRANCH=
GIT_HEAD=
```

`com.bintrans.owner` must match the name prefix (`bintrans-c-*` with `agent-c`). A mismatch is a policy violation. The canonical stack does not need these labels; absence of an owner label on a `freight_*` container means canonical-or-legacy, not an invitation for a feature worktree to reuse it.

## Fixed container names

`infrastructure/docker-compose/docker-compose.yml` sets `container_name` on postgres, identity, company, transport-order, rfx, shipment, document, billing-register, payment, low-code, control-tower, api-gateway, prometheus, grafana, and redpanda. `migrate` has no `container_name`.

Each fixed name blocks a second Compose project: Docker rejects a second container with that name. Strategy for now is an override for agent tests, not removal of the names. Removal is unsafe until `Makefile` (`db-shell`, `db-check`), `scripts/dev/*`, `tests/integration/smoke-test.sh`, and operator docs stop calling `docker exec freight_postgres` and the other `freight_*` names. `.github/workflows/ci.yml` matches `freight_` mostly as schema and job text, not as these container names.

## Closeout

Before the worktree is closed:

```text
TEMP_STACK_STOPPED=
TEMP_CONTAINERS_REMOVED=
TEMP_NETWORK_REMOVED=
TEMP_VOLUME_DECISION=
CANONICAL_STACK_UNCHANGED=
WORKTREE_CLEAN=
TEMP_DOCKER_PROJECT=
DOCKER_CLOSEOUT=
```

Default commands, scoped to that project:

```text
docker compose -p bintrans-c-<task> stop
docker compose -p bintrans-c-<task> rm -f
```

`docker compose down` without `-v` may remove that project's containers and network. It must not be pointed at the canonical project.

```text
DISPOSABLE_TEST_DATA=YES  -> that project's volume may be removed by name
DISPOSABLE_TEST_DATA unset or NO -> keep the volume and name it in the handoff
```

Never delete `docker-compose_freight_postgres_data` from an agent task.

## Forbidden cleanup

These commands are forbidden for every agent unless an operator explicitly approves that cleanup stage:

```text
docker system prune
docker volume prune
docker image prune -a
```

Cleanup is by exact project, container, network, or volume name.

```text
UNKNOWN RESOURCE = KEEP
```

A container, volume, or network the inventory cannot attribute to the canonical stack or to agent A/B/C/D stays in place. The inventory never marks a resource safe to delete.

## Read-only inventory

```text
bash scripts/ops/docker_multi_agent_inventory.sh
```

The script prints container, Compose project, workdir, service, owner, task, lifecycle, git SHA, status, created time, and restart policy. It also lists volumes and networks. It does not start, stop, remove, or prune.

## Scenario check

These four uses cannot land on another owner's containers if the override rules above are followed.

| Scenario | Allowed identity | Forbidden identity |
|---|---|---|
| A staging/release | `bintrans-a-staging-<task>` | `freight_*`, `bintrans-b/c/d-*` |
| B EDO integration | `bintrans-b-edo-<task>` | `freight_document_service`, other agents' prefixes |
| C TMS development | canonical `tms-*` targets from `origin/main`; feature images only as `bintrans-c-tms-<task>` | rebuilding `freight_shipment_service` from a feature worktree |
| D NLO integration | `bintrans-d-nlo-<task>` | replacing canonical shipment or NLO containers in place |

The inventory script reports one class per container and does not delete anything:

```text
CANONICAL
AGENT_A
AGENT_B
AGENT_C
AGENT_D
UNLABELED
OWNER_MISMATCH
UNKNOWN
```

`freight_*` without an agent owner label is `CANONICAL`. A prefixed name with the matching `com.bintrans.owner` is `AGENT_A` through `AGENT_D`. A prefixed name with a different owner is `OWNER_MISMATCH`. A `bintrans*` container with no owner label is `UNLABELED`. Everything else is `UNKNOWN`, which means keep.

## Historical exception

`bintrans_tms_test_*` was created in Stage 1I, before this policy. Those containers may have no owner label and `restart: unless-stopped`, inherited from the canonical file. That is not a template. New environments use a unique prefix, `restart: "no"`, and the labels above. Do not relabel or remove the Stage 1I containers from this document.
