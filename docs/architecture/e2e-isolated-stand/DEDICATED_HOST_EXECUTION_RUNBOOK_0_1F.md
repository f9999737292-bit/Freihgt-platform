# BINTRANS E2E Dedicated Host — Execution Runbook 0.1F

Stage: `BINTRANS-E2E-STAND-0.1F-DEDICATED-HOST-READINESS`

This runbook is the procedure for a later approved run. Stage 0.1F does not execute it. `E2E_EXECUTION_AUTHORIZED` remains no until the controller names the host and the SHA.

Resource gates, with the units the controller fixed:

- Total RAM at least `16000000000` bytes (16 decimal GB, `16 × 10^9`).
- Available RAM at least `6442450944` bytes (6 GiB, `6 × 1024^3`).
- At least 4 vCPU.
- At least `53687091200` bytes free (50 GiB, `50 × 1024^3`) on the filesystem that holds Docker's data root.
- Recommended disk capacity: 100 decimal GB.
- `x86_64`, a supported Linux LTS, Docker Engine, and the Docker Compose plugin.
- Compose project id `bintrans-e2e-stand-v01a`.

The execution checkout is the SHA the controller approves after this readiness change. That SHA must contain `scripts/ops/bintrans_e2e_stand/host-preflight.sh`. The branch base `0005a659429404fe5a30425e4bbdbba899b6daa0` does not contain that script, so it is not the execution checkout.

## 1. Clone

```text
git clone https://github.com/f9999737292-bit/Freihgt-platform.git
```

## 2. Enter the repository

```text
cd Freihgt-platform
```

## 3. Check out the approved SHA

`<CONTROLLER_APPROVED_SHA>` is the full SHA named in a separate controller approval. It must contain `scripts/ops/bintrans_e2e_stand/host-preflight.sh`.

```text
git checkout <CONTROLLER_APPROVED_SHA>
```

## 4. Verify HEAD and a clean worktree

```text
bash scripts/ops/bintrans_e2e_stand/checkout-guard.sh --sha <CONTROLLER_APPROVED_SHA>
```

`CHECKOUT_GUARD=PASS` means `HEAD` equals that SHA, `git status --porcelain` is empty, and `host-preflight.sh` is present. Any other result stops the procedure. Do not copy a dirty worktree onto the host.

## 5. Resource and isolation diagnosis

```text
bash scripts/ops/bintrans_e2e_stand/host-preflight.sh
```

Exit 0 prints `PREFLIGHT_RESULT=PASS` and `DOCKER_ENDPOINT=unix://...`. Exit 1 is `FAIL`. Exit 2 is `BLOCKED` when the local socket or daemon is unavailable. The script does not start Docker. A `BLOCKED` or `FAIL` result stops the procedure.

Pin every later Docker command, including `down.sh`, to that same socket:

```text
export DOCKER_HOST="<DOCKER_ENDPOINT value printed above>"
unset DOCKER_CONTEXT
```

`DOCKER_HOST` stays that `unix://` value. Do not point it at `tcp://` or `ssh://`.

## 6. Ephemeral environment and EXIT trap

Install the trap before creating the env file. `resources_started` stays `no` until a later, separately approved `up`.

```bash
set -euo pipefail
resources_started=no
env_file=/var/tmp/bintrans-e2e-stand/stand.env
# BEGIN SESSION_TRAP
resources_started=${RESOURCES_STARTED:-$resources_started}
env_file=${ENV_FILE:-$env_file}
cleanup() {
  status=$?
  trap - EXIT
  if ! bash scripts/ops/bintrans_e2e_stand/session-cleanup.sh --env-file "$env_file" --resources-started "$resources_started"; then
    exit 1
  fi
  exit "$status"
}
trap cleanup EXIT
# END SESSION_TRAP
umask 077
mkdir -p /var/tmp/bintrans-e2e-stand
scripts/ops/bintrans_e2e_stand/ephemeral-env.sh > "$env_file"
chmod 0600 "$env_file"
```

Keep this file outside the repository. Do not print it. On a normal exit or an error before any stand resource exists, the trap deletes it.

## 7. Project guard and Compose config

```text
bash scripts/ops/bintrans_e2e_stand/project-guard.sh --project bintrans-e2e-stand-v01a
```

Any other project name, including `bintrans-ct-staging` and the dotted id `bintrans-e2e-stand-v0.1a`, must exit before Docker.

`docker compose config` interpolates the ephemeral password. Do not tee the full render into a log. `DOCKER_HOST` is still the socket from step 5.

```text
docker compose \
  --env-file "$env_file" \
  -p bintrans-e2e-stand-v01a \
  -f infrastructure/docker-compose/docker-compose.bintrans-e2e-stand-v0.1a.yml \
  config --quiet
```

Then print only the project name and fail if a port binding appears:

```text
docker compose \
  --env-file "$env_file" \
  -p bintrans-e2e-stand-v01a \
  -f infrastructure/docker-compose/docker-compose.bintrans-e2e-stand-v0.1a.yml \
  config | awk '
    /^name:/ {print; seen=1}
    /^[[:space:]]*ports:/ {print "PORTS_PRESENT"; bad=1}
    END {if (!seen || bad) exit 1}
  '
```

The printed name must be `name: bintrans-e2e-stand-v01a`. Discard the rendered config. It contains the ephemeral password.

## 8. Stop unless execution is approved

Without a separate controller approval, exit here. The trap removes the env file. Do not run `docker compose up`.

A later approval is the only point that sets `resources_started=yes`, and it does so immediately before `up`:

```text
resources_started=yes
docker compose \
  --env-file "$env_file" \
  -p bintrans-e2e-stand-v01a \
  -f infrastructure/docker-compose/docker-compose.bintrans-e2e-stand-v0.1a.yml \
  up --build --abort-on-container-exit --exit-code-from runner
```

## 9. Evidence and guaranteed env removal

Collect the preflight output, `DOCKER_ENDPOINT`, the guard exit code, the `name:` line from step 7, and, only after an approved run, the runner exit code. When collecting container logs, drop lines that contain `POSTGRES_PASSWORD`, `DATABASE_URL`, or `postgres://`. Do not attach `stand.env`.

Leaving the script by success or by error runs the EXIT trap:

- `resources_started=no`: the trap deletes the env file and does not call `down.sh`.
- `resources_started=yes`: the trap calls `down.sh --env-file` with the same file first. Only after that command exits 0 does it delete the env file.

## 10. Scoped cleanup failure

`scripts/ops/bintrans_e2e_stand/session-cleanup.sh` performs that order. If `down.sh` fails, it prints `CLEANUP_FAILED`, leaves the env file on disk, and exits 1. The operator does not report the project as removed. The retry uses the same env file and the same `DOCKER_HOST`. Do not run `docker system prune`, `docker volume prune`, or `docker network prune`.

After a successful `down.sh`, confirm removal with the pinned daemon:

```text
docker compose -p bintrans-e2e-stand-v01a ps -a
docker volume ls
docker network ls
```

The project list is empty. No volume or network name starts with `bintrans-e2e-stand-v01a`. `freight-platform-network` and `freight_postgres_data` were not present before the run and are not created by it.

The later execution report records the host profile, preflight result, approved SHA, runner exit code, and either successful removal or `CLEANUP_FAILED`. Stage 0.1F does not produce that execution report.
