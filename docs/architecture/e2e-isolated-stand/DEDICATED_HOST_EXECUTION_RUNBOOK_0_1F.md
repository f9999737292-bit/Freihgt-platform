# BINTRANS E2E Dedicated Host — Execution Runbook 0.1F

Stage: `BINTRANS-E2E-STAND-0.1F-DEDICATED-HOST-READINESS`

Baseline: `0005a659429404fe5a30425e4bbdbba899b6daa0`. This runbook is the procedure for a later approved run. Stage 0.1F does not execute it. `E2E_EXECUTION_AUTHORIZED` remains no until the controller names the host and approves the run.

Target profile: `x86_64`, at least 4 vCPU, at least 16 GB total RAM, at least 50 GB free on the Docker data filesystem, recommended disk size 100 GB, a supported Linux LTS, Docker Engine, and the Docker Compose plugin. The Compose project id is `bintrans-e2e-stand-v01a`.

## 1. Clean checkout

On the dedicated host, clone the public repository and check out the SHA named by the controller. At authoring time that SHA is `0005a659429404fe5a30425e4bbdbba899b6daa0`.

```text
git clone https://github.com/f9999737292-bit/Freihgt-platform.git
git checkout 0005a659429404fe5a30425e4bbdbba899b6daa0
git status --short
```

The worktree must be empty. Do not copy the Windows checkout or a dirty worktree onto the host.

## 2. Resource and isolation diagnosis

```text
bash scripts/ops/bintrans_e2e_stand/host-preflight.sh
```

Exit 0 prints `PREFLIGHT_RESULT=PASS`. Exit 1 is `FAIL`. Exit 2 is `BLOCKED` when the Docker daemon is not available. The script does not start Docker, pull images, or create containers. A `BLOCKED` or `FAIL` result stops the procedure.

## 3. Ephemeral environment

```text
umask 077
mkdir -p /var/tmp/bintrans-e2e-stand
scripts/ops/bintrans_e2e_stand/ephemeral-env.sh > /var/tmp/bintrans-e2e-stand/stand.env
chmod 0600 /var/tmp/bintrans-e2e-stand/stand.env
```

Keep this file outside the repository. Do not print it.

## 4. Project guard

```text
bash scripts/ops/bintrans_e2e_stand/project-guard.sh --project bintrans-e2e-stand-v01a
```

Any other project name, including `bintrans-ct-staging` and the dotted id `bintrans-e2e-stand-v0.1a`, must exit before Docker.

## 5. Compose config without disclosing secrets

`docker compose config` interpolates the ephemeral password. Do not tee the full render into a log.

```text
docker compose \
  --env-file /var/tmp/bintrans-e2e-stand/stand.env \
  -p bintrans-e2e-stand-v01a \
  -f infrastructure/docker-compose/docker-compose.bintrans-e2e-stand-v0.1a.yml \
  config --quiet
```

Then print only the project name and fail if a port binding appears:

```text
docker compose \
  --env-file /var/tmp/bintrans-e2e-stand/stand.env \
  -p bintrans-e2e-stand-v01a \
  -f infrastructure/docker-compose/docker-compose.bintrans-e2e-stand-v0.1a.yml \
  config | awk '
    /^name:/ {print; seen=1}
    /^[[:space:]]*ports:/ {print "PORTS_PRESENT"; bad=1}
    END {if (!seen || bad) exit 1}
  '
```

The printed name must be `name: bintrans-e2e-stand-v01a`. Discard the rendered config. It contains the ephemeral password.

## 6. E2E execution

Run this step only after a separate controller approval. Until that approval, stop after step 5.

```text
docker compose \
  --env-file /var/tmp/bintrans-e2e-stand/stand.env \
  -p bintrans-e2e-stand-v01a \
  -f infrastructure/docker-compose/docker-compose.bintrans-e2e-stand-v0.1a.yml \
  up --build --abort-on-container-exit --exit-code-from runner
```

## 7. Evidence

Collect the preflight output, the guard exit code, the `name:` line from step 5, and the runner exit code. When collecting container logs, drop lines that contain `POSTGRES_PASSWORD`, `DATABASE_URL`, or `postgres://`. Do not attach `stand.env`.

## 8. Scoped cleanup

```text
bash scripts/ops/bintrans_e2e_stand/down.sh --env-file /var/tmp/bintrans-e2e-stand/stand.env
rm -f /var/tmp/bintrans-e2e-stand/stand.env
```

`down.sh` refuses a project override and removes only `bintrans-e2e-stand-v01a`, including its volumes. Do not prune other Docker resources.

## 9. Confirm removal

```text
docker compose -p bintrans-e2e-stand-v01a ps -a
docker volume ls
docker network ls
```

The project list is empty. No volume or network name starts with `bintrans-e2e-stand-v01a`. `freight-platform-network` and `freight_postgres_data` were not present before the run and are not created by it.

## 10. E2E report

The later execution report records the host profile, preflight result, SHA, runner exit code, cleanup result, and these statements: containers removed, migrations applied only inside the discarded stand volume, staging unchanged. Stage 0.1F does not produce that execution report.
