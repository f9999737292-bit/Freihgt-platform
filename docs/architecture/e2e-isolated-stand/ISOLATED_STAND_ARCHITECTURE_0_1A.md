# BINTRANS E2E Isolated Stand 0.1A — Architecture

Stage: `BINTRANS-E2E-ISOLATED-STAND-0.1A-DISCOVERY`

Baseline: `origin/main` `fb9dc6c409f048c4b9f913bb40f315c2a11b4d6a` (merge of PR #304). Migration head on that commit is `000105_platform_postgres_role_foundation_v0_2_i1`. Remediation R1 keeps this a documentation contract: `stand` mode is fail-closed on Redpanda, and the tracking port note matches the staging overlay. This revision does not add a Compose file, start containers, apply SQL, or change `startKafka`.

Related documents:

- [Compose dependency matrix](COMPOSE_DEPENDENCY_MATRIX_0_1A.md)
- [Security and secrets policy](SECURITY_AND_SECRETS_POLICY_0_1A.md)
- [Execution runbook](E2E_EXECUTION_RUNBOOK_0_1A.md)
- [Go/No-Go checklist](GO_NO_GO_CHECKLIST_0_1A.md)
- [Rollback and cleanup](ROLLBACK_AND_CLEANUP_PLAN_0_1A.md)
- [Resource estimate](RESOURCE_ESTIMATE_0_1A.md)

## Purpose

The stand exists so `TestBintransE2ETransportFlow` can run one shipment from RFx award through `CLOSING_DOCUMENTS_CREATED` on real service processes and a real Redpanda broker.

The business writes stay HTTP calls. SQL is limited to the identity and reference rows the test already inserts before the first RFx request, plus read-only assertions. The stand does not insert a shipment, a POD, a settlement, a register, or a closing package.

This stage does not deploy the stand.

## What the current test already does

`tests/bintrans-e2e-transport-flow` builds and starts these processes, then calls them on `127.0.0.1`:

| Process | Role in the flow |
| --- | --- |
| contract-rate-service | Required by transport-order configuration. The harness starts it before the order service. |
| transport-order-service | Award conversion target and the public create that rejects a tenant mismatch. |
| rfx-service | Event, lot, lane, publish, response, award, and conversion HTTP. |
| shipment-service | Execute, assign driver and vehicle, driver events, outbox producer. |
| document-service | POD intent, content upload, and complete. |
| billing-register-service | Settlement, register, and closing-document package. |
| control-tower-read-model-service | Kafka consumer and status projection. |

The harness does not start identity-service, company-service, api-gateway, payment-service, or low-code-service. Tenants, companies, users, memberships, role links, locations, drivers, and vehicles are inserted with SQL. System roles `PROCUREMENT_MANAGER` and `CARRIER_DISPATCHER` are read from the migrated catalog; the test does not insert those role rows.

PostgreSQL is either `TEST_DATABASE_URL` or an embedded PostgreSQL 16. The harness creates a database named `bintrans_e2e_flow_<id>`, applies every `*.up.sql` in lexical order, and drops that database at the end. It refuses to run unless the last file starts with `000105_`.

The broker is not Redpanda. On this baseline `startKafka` always calls `kfake.NewCluster` and seeds `shipment.status.v1` and `driver.events.v1`. There is no environment switch yet. CI job `bintrans-e2e-transport-flow` uses the same harness against a GitHub Actions `postgres:16` service and still uses `kfake`.

## Target topology

One Compose project, used only by this stand:

| Contract item | Value |
| --- | --- |
| Project name | `bintrans-e2e-stand-v01a` |
| Compose file | Future file. Not `infrastructure/docker-compose/docker-compose.yml`. |
| Network | Project network `bintrans-e2e-stand`. `internal: true`. No `name: freight-platform-network`. |
| Postgres volume | Project volume `bintrans-e2e-stand-postgres`. Anonymous or project-scoped. Never `freight_postgres_data`. |
| Document storage | Project volume `bintrans-e2e-stand-documents`. |
| Redpanda data | Ephemeral container filesystem, or a project volume removed with the project. |
| Host ports | None. No `0.0.0.0` publish and no `127.0.0.1` publish. |
| Container names | None of the global `freight_*` names. Let Compose prefix names with the project. |

The runner is a one-shot container on `bintrans-e2e-stand` and on no other network. It reaches Postgres only at host `postgres` and Redpanda only at `redpanda:9092`. Go child processes may listen on `127.0.0.1` inside that container. Those listeners are not published to the host, and `127.0.0.1` is not an allowed Postgres or broker address. The host Docker daemon can still use `docker exec` for an operator; that is not a published port.

Do not attach this project to the staging Compose project `bintrans-ct-staging`, and do not pass `/protected/bintrans/control-tower-observation/staging.env`.

## Required and optional processes

Required for the existing scenario once the harness can address Redpanda:

1. PostgreSQL 16 (`mirror.gcr.io/library/postgres:16`, the image already used by CI and the dev Compose file).
2. Redpanda `docker.redpanda.com/redpandadata/redpanda:v24.2.8` (the image already pinned in the dev Compose file).
3. One-shot migrate `migrate/migrate:v4.17.1` with command `up` on the empty stand database. This is the only stand applier. In `stand` mode the harness does not call `applyMigrations` and does not create a second database.
4. contract-rate-service, transport-order-service, rfx-service, shipment-service, document-service, billing-register-service, control-tower-read-model-service.

The first implementation should keep those seven processes inside the existing harness, on the stand network, and should limit Compose to Postgres, Redpanda, topic creation, and migrate. Starting the same service both in Compose and in the harness would create two writers.

Optional, and not part of `TestBintransE2ETransportFlow`:

| Service | Why it stays out |
| --- | --- |
| identity-service | The test seeds `core.users` and does not call identity HTTP. |
| company-service | The test seeds `core.companies` and memberships. |
| api-gateway | The test calls service routes directly. Gateway JWT proof is a separate later probe. |
| payment-service | The scenario asserts zero `billing.payment_obligations` and does not call payment HTTP. |
| low-code-service, prometheus, grafana, tracking-service, freight-cost-service, analytics-service, network-optimizer-service | No call in this scenario. tracking-service listens on container port `8089` in the staging overlay and that overlay publishes no host port for it. Host port `8089` there belongs to control-tower. The stand does not import the staging overlay. |

Dev Compose `AUTH_ENABLED=false` must not be copied into a later gateway probe. That probe is outside this flow.

## Startup order

1. Create the project network.
2. Start Postgres. Wait for `pg_isready` against the stand database.
3. Start Redpanda. Wait until `rpk cluster info --brokers localhost:9092` succeeds inside the Redpanda container. That healthcheck address is not the client bootstrap. Clients use `redpanda:9092`.
4. Create topics `shipment.status.v1` and `driver.events.v1` with one partition and replication factor 1. Do this before any consumer starts. Do not rely on auto-create.
5. Apply migrations `000001` through `000105` with the migrate container only. Stop if the recorded version is not 105, if `trg_new_activation_authorization` exists, or if any statement creates a `LOGIN` role. The harness does not apply those files again.
6. Start contract-rate, then transport-order, then rfx, shipment, document, and billing. Each must answer `GET /ready` with HTTP 200.
7. Start control-tower with `CONTROL_TOWER_CONSUMER_ENABLED=true` and a new group id `control-tower-e2e-<run-id>`. Wait for `/ready` and for the log line `starting shipment status consumer`.
8. Run `go test -tags=integration -count=1 -timeout 25m -run TestBintransE2ETransportFlow` in the runner container with `BINTRANS_E2E_STAND_MODE=stand` and `BINTRANS_E2E_KAFKA_BROKERS=redpanda:9092`. Do not set `BINTRANS_E2E_KFAKE_ALLOW`. The test logs the mode and the broker address, then fails before seed unless the broker is exactly `redpanda:9092` and the dial succeeds.

`GET /health` is liveness. The stand gate is `/ready`, which the shared server mount ties to the database. Dev Compose healthchecks currently call `/health` only.

## Kafka path

```text
shipment HTTP status change
  -> transport.shipment_event_outbox
  -> shipment outbox worker
  -> Redpanda topic shipment.status.v1
  -> control-tower consumer group control-tower-e2e-<run-id>
  -> control_tower.shipment_status_projection
  -> GET /internal/v1/control-tower/shipments/{id}/status
```

Producer settings already in `services/shipment-service/internal/outbox/kafka_publisher.go`: seed brokers from `SHIPMENT_KAFKA_BROKERS`, client id `shipment-service`, `RequiredAcks(AllISRAcks)`, three record retries. SASL and TLS stay off inside this network.

The dev Compose file sets `CONTROL_TOWER_CONSUMER_ENABLED=false` and puts both Redpanda and control-tower behind profiles. The stand turns the consumer on and does not use those profiles as they are written today.

`kfake` is not an acceptable broker for `BINTRANS_E2E_STAND_MODE=stand`. That mode forbids `kfake.NewCluster`, rejects an empty broker, a `kfake` address, and any `localhost` or `127.0.0.1` broker, and does not fall back to `kfake` when the Redpanda dial fails. `kfake` remains available only in `BINTRANS_E2E_STAND_MODE=ci` when `BINTRANS_E2E_KFAKE_ALLOW=1`. The stand runner does not set that allow. The override does not exist on this baseline, so the stand cannot execute the scenario yet. See the runbook blocker and the matrix contract.

## Scenario cardinality

The happy path asserts one operating tenant, the shipper company, and the awarded carrier company, then one RFx, one award link, one transport order, one shipment, one POD, one approved settlement, one register, and one closing package. The register status is `CLOSING_DOCUMENTS_CREATED`. It is not `PAID` or `CLOSED`.

The fixture also inserts a foreign tenant, another shipper, another carrier, and their drivers and vehicles. Those rows exist so the negative HTTP cases can run. They are reference data, not a second business shipment.

## Explicit non-goals

- No staging or production change.
- No `docker compose up` or `down` in this stage.
- No edit to `pg_hba.conf`.
- No `LOGIN` role and no change to an existing login.
- No application of `infrastructure/postgres/tms_activation_enforcement_gate_v0.1.sql`.
- `TMS_RELATIONSHIP_ACTIVATION_ENABLED` remains `false`. The shipment config default is already `false`.
- Services keep the database owner used by the current test. `shipment_login` cannot run `ExecuteAwardOrder`; that session receives `42501`. Migration `000105` does not add the award-lock function. See `docs/architecture/platform/PLATFORM_POSTGRES_ROLE_FOUNDATION_0_2_I1_RFX_LOCK_SECURITY_CONTRACT.md`.
