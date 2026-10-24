# BINTRANS E2E Isolated Stand 0.1A — Resource Estimate

Stage: `BINTRANS-E2E-ISOLATED-STAND-0.1A-DISCOVERY`

Baseline: `fb9dc6c409f048c4b9f913bb40f315c2a11b4d6a`. No container was started, so the figures below are planning bounds from the images and settings already in the repo. They are not measurements.

## Local stand

The stand is a local Docker project. This design has no cloud account, no staging VM, and no extra hosted database.

| Component | Source of the bound | Planning figure | Uncertainty |
| --- | --- | --- | --- |
| PostgreSQL 16 | CI and dev Compose use `mirror.gcr.io/library/postgres:16`. One small database, 105 migrations, one shipment. | 0.5–1 GB RAM. Under 2 GB disk including the data directory. | Not measured. WAL and the migrated schema may land lower. |
| Redpanda | Dev Compose command: `--smp 1`, `--memory 512M`, `--reserve-memory 0M`, `--overprovisioned`. | Reserve at least 1 GB RAM. The 512M flag is not the whole container RSS. | RSS on Docker Desktop was not measured. Single-node disk for this scenario should stay under 1 GB. |
| Seven Go services | Harness builds `./cmd/server` for contract-rate, transport-order, rfx, shipment, document, billing, and control-tower. | Budget 1.5 GB RAM for the seven processes together. | Per-process RSS was not measured. A container image build is larger than the running process and was not sized. |
| Migrate | `migrate/migrate:v4.17.1`, one-shot. | Negligible RAM. CPU for the 105 files is part of startup, not a resident service. | Wall time on a cold disk was not measured. |
| Runner | `go test` with `-timeout 25m`, matching the CI job. | Budget 2 GB RAM during compilation. | Windows Docker Desktop file sharing can make the first build much slower. Not measured. |
| Document volume | One POD body of 16 bytes in the current test. | Negligible. | A later binary fixture would change this. The current test does not use one. |

Planning total for a later local run: 6 GB RAM available to Docker, 4 GB as a minimum that may be tight during compilation, 4 CPU cores, and 15 GB free disk before the first image pull and Go build. Those totals are sums of the bounds above plus image layers. They are not a benchmark.

## Time

| Step | Bound used for planning | Uncertainty |
| --- | --- | --- |
| Image pull of Postgres 16, Redpanda v24.2.8, and migrate v4.17.1 | Treat the first pull as several minutes. | Depends on the mirror and the link. |
| First Go build of seven services | Inside the existing CI job budget. The job timeout is 35 minutes and the test timeout is 25 minutes. | A local Windows build was not timed. |
| Migrations `000001`–`000105` | Must finish before `/ready`. No separate timeout is defined in the harness beyond the test timeout. | Not timed in this stage. |
| One scenario | The test waits up to 30 seconds for the outbox and 45 seconds for control-tower. Broker read timeout is 20 seconds. | A cold Redpanda consumer can use that whole window. Not observed here. |

## CI cost

This docs PR runs the existing `.github/workflows/ci.yml` because pull requests to `main` have no path filter. That includes job `bintrans-e2e-transport-flow`, which uses `kfake` and a GitHub `postgres:16` service. It does not start Redpanda.

A later implementation that adds Redpanda to that job would add one service container on `ubuntu-latest` for up to the 35-minute job timeout. Dollar cost is not estimated. The runner class is the existing `ubuntu-latest` class, not a larger runner.

## What would change the estimate

- Building all platform images instead of the seven services. The base Compose file also builds identity, company, gateway, payment, and low-code. The stand does not.
- Leaving Redpanda `--memory 512M` on a host that cannot satisfy the container's real RSS.
- Running the stand and the dev stack or `bintrans-ct-staging` on one Docker Desktop VM at the same time. The stand contract forbids sharing volumes and networks, and the RAM bound assumes the stand is the only extra project.
- Switching services to `shipment_login`. That path does not complete `ExecuteAwardOrder` on this baseline, so it is not a sizing case.

## Discovery cost

This stage used local git inspection and GitHub API reads. It did not pull images, start Docker, or reserve a cloud resource.
