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

The execution checkout is the SHA the controller approves after this readiness change. That SHA must contain `scripts/ops/bintrans_e2e_stand/run-session.sh`. The branch base `0005a659429404fe5a30425e4bbdbba899b6daa0` does not contain that script, so it is not the execution checkout.

Clone, entering the repository, and checkout prepare the tree. The session runner then verifies that SHA. One `run-session.sh` process owns preflight, the Docker endpoint pin, the ephemeral env file, the EXIT trap, Compose config, and, only in an approved execute, `up` and scoped `down`.

## 1. Clone

```text
git clone https://github.com/f9999737292-bit/Freihgt-platform.git
```

## 2. Enter the repository

```text
cd Freihgt-platform
```

## 3. Check out the approved SHA

`<CONTROLLER_APPROVED_SHA>` is the full 40-character SHA from a separate controller approval. It must contain `scripts/ops/bintrans_e2e_stand/run-session.sh`.

```text
git checkout <CONTROLLER_APPROVED_SHA>
```

The worktree must be clean. Do not copy a dirty checkout onto the host.

## 4. Check-only session

Each E2E run needs its own controller approval. Until that approval exists, run only the default check:

```text
bash scripts/ops/bintrans_e2e_stand/run-session.sh --sha <CONTROLLER_APPROVED_SHA>
```

That single process:

1. Confirms `HEAD` is the approved SHA and the worktree is clean.
2. Runs `host-preflight.sh` and reads `DOCKER_ENDPOINT=unix://...`.
3. Exports that value as `DOCKER_HOST` and unsets `DOCKER_CONTEXT`.
4. Installs the EXIT trap, and the same cleanup for SIGINT and SIGTERM, before creating the env file.
5. Creates a new ephemeral env outside the repository. The directory is mode `0700` and owned by the current user. The file is created with `O_CREAT | O_EXCL | O_NOFOLLOW` and mode `0600`, and the password is written through that file descriptor. An existing file, a symbolic link, a path inside the checkout, or a parent that is a link or is not mode `0700` stops the session. A leftover env file is not replaced or deleted.
6. Runs the project guard and `docker compose config` without printing the password.
7. Exits without `docker compose up`.

The trap then deletes the env file. It does not call `down.sh`, because no stand resources were created. `SESSION_RESULT=CHECK_ONLY_PASS` and `SESSION_CLEANUP_PID` equal `SESSION_PID`. A `FAIL` or `BLOCKED` preflight stops the process before the env file exists.

## 5. Execute only after a separate approval

A later controller approval, naming this host and this SHA, is required for every E2E run. The check-only result is not that approval. The approved command is:

```text
bash scripts/ops/bintrans_e2e_stand/run-session.sh --sha <CONTROLLER_APPROVED_SHA> --execute
```

The process repeats the check-only sequence. After config passes, it sets `resources_started=yes` and immediately runs:

```text
docker compose up --build --abort-on-container-exit --exit-code-from runner
```

with the pinned `DOCKER_HOST` and the same env file. `up` runs in its own process group. The session records that group id. The group is not the session's group, and every member is the `up` process or its descendant. Success and failure both enter the trap. The trap calls `down.sh --env-file` for project `bintrans-e2e-stand-v01a` only, using the same `DOCKER_HOST` and the same env file. After `down.sh` exits 0, the trap deletes the env file only when its device and inode still match the file this session created, then returns the `up` status, so a failed run is not reported as success.

If `down.sh` fails, the trap prints `CLEANUP_FAILED`, leaves the env file in place, and exits 1. Do not report the project as removed. Retry with the same file and the same `DOCKER_HOST`. Do not run `docker system prune`, `docker volume prune`, or `docker network prune`.

SIGINT and SIGTERM are handled by this same Bash process. It signals the verified child group, waits until those processes exit, and only then runs the EXIT trap. The wait is bounded. If the child is still running, the session escalates to SIGKILL for that same verified group and no other process. A successful cleanup preserves exit code 130 for SIGINT and 143 for SIGTERM. No second shell is required to pin the endpoint, create or delete the env file, or run `up` or `down`. `down` does not start while `up` is still running.

## 6. Evidence

Keep the runner output, including `DOCKER_HOST`, `SESSION_PID`, and `SESSION_CLEANUP_PID`. Drop any collected container log line that contains `POSTGRES_PASSWORD`, `DATABASE_URL`, or `postgres://`. Do not attach the env file.

After a successful `down.sh`, the same pinned daemon shows an empty project:

```text
docker compose -p bintrans-e2e-stand-v01a ps -a
docker volume ls
docker network ls
```

No volume or network name starts with `bintrans-e2e-stand-v01a`. The later execution report records the host profile, the approved SHA, the runner exit code, and either removal or `CLEANUP_FAILED`. Stage 0.1F does not produce that execution report.

## 7. Recovery after SIGKILL or a host crash

Bash cannot trap SIGKILL. A killed session, or a host that lost power, does not run the EXIT trap. Do not read a missing process as proof that the stand was removed, and do not claim the env file was cleaned up.

1. Export the local socket recorded for that attempt as `DOCKER_HOST=unix://...` and unset `DOCKER_CONTEXT`. Do not select another daemon, context, or host.
2. Confirm the only Compose project under inspection is `bintrans-e2e-stand-v01a`.
3. With that `DOCKER_HOST`, run `docker compose ls -a -q`, `docker volume ls`, and `docker network ls`. If a list command fails, the result is BLOCKED. Keep the output. Do not guess which objects exist.
4. If that project is present, or a volume or network name starts with `bintrans-e2e-stand-v01a`, resources may exist. Run `bash scripts/ops/bintrans_e2e_stand/down.sh --env-file` with the env file the session left behind and the same `DOCKER_HOST`. Do not substitute another env file or another project.
5. Delete that env file only after `down.sh` exits 0 and a second list shows the project is gone. If `down.sh` fails, keep the file and record `CLEANUP_FAILED`.
6. If the lists are ambiguous, the preserved env file is missing, or the endpoint is not the recorded local socket, stop. The result is BLOCKED. Keep the command output.

Do not remove any other Compose project. Do not run `docker system prune`, `docker volume prune`, or `docker network prune`.
