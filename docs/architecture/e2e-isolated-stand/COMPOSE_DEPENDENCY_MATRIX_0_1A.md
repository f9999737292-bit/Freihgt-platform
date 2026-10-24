# BINTRANS E2E Isolated Stand 0.1A — Compose Dependency Matrix

Stage: `BINTRANS-E2E-ISOLATED-STAND-0.1A-DISCOVERY`

Baseline: `fb9dc6c409f048c4b9f913bb40f315c2a11b4d6a`. This matrix describes the future project `bintrans-e2e-stand-v0.1a`. It does not change `infrastructure/docker-compose/docker-compose.yml`.

## Why the current Compose file is not the stand

`infrastructure/docker-compose/docker-compose.yml` is the local platform file. It cannot be reused as the stand:

| Current file | Stand requirement |
| --- | --- |
| Network `name: freight-platform-network` | A project-only internal network. Staging overlay uses the same named network. |
| Volume `freight_postgres_data` | A project volume that does not exist outside this project. |
| `container_name: freight_postgres`, `freight_redpanda`, and the other `freight_*` names | No global container names. Staging does not override these names. |
| Host ports on `5432`, `8080`–`8090`, and Redpanda `19092` | No published host ports. |
| `contract-rate-service` is absent, while transport-order points at `http://contract-rate-service:8091` | contract-rate is required by the harness. The staging overlay defines it; the base file does not. |
| control-tower profile `read-model`, consumer disabled, brokers `redpanda:9092` | Consumer enabled on the stand network, no profile gate. |
| Redpanda profile `messaging`, external advertise `localhost:19092` | Always started. Clients use `redpanda:9092` only. |
| migrate profile `tools`. Command is `-path` and `-database` without `up` | One-shot `migrate -path=/migrations -database=... up`, then exit. |
| shipment-service has no outbox or Kafka environment | Outbox enabled, transport `kafka`, topics below. |
| rfx-service has no `TRANSPORT_ORDER_SERVICE_URL` | Set to the in-network order URL, matching the harness. |
| Hard-coded dev database password and `JWT_SECRET: dev_secret_change_me` | Ephemeral stand credentials. No committed secret values. See the secrets policy. |

`docker-compose.bintrans-ct-staging.yml` sets Compose project `bintrans-ct-staging` and clears most host ports. It is a staging override of the same base services. The stand must be a separate project and a separate file so a later `up` cannot select the staging project by default. Makefile `COMPOSE` points at the base file and has no project-name flag.

## Required services

| Service | Image or build | Waits for | Ready signal | Stand environment that differs from the base file |
| --- | --- | --- | --- | --- |
| postgres | `mirror.gcr.io/library/postgres:16` | none | `pg_isready` for the stand database and user | Ephemeral user, password, and database name supplied at run time. No host port. |
| redpanda | `docker.redpanda.com/redpandadata/redpanda:v24.2.8` | none | `rpk cluster info --brokers localhost:9092` | `--kafka-addr internal://0.0.0.0:9092` and `--advertise-kafka-addr internal://redpanda:9092`. No external listener and no host port. `--smp 1`, `--memory 512M`, `--overprovisioned`, `--reserve-memory 0M`, as the base file already sets for a single node. |
| topic-init | same Redpanda image, one-shot `rpk` | redpanda healthy | `rpk topic describe` for both topics | Creates `shipment.status.v1` and `driver.events.v1`, partition count 1, replication factor 1. |
| migrate | `migrate/migrate:v4.17.1` | postgres healthy | process exit 0 and `schema_migrations` version `105` | Mount `infrastructure/migrations` read-only. Command includes `up`. Do not mount the enforcement-gate SQL file. |
| contract-rate-service | `services/contract-rate-service/Dockerfile` | migrate succeeded | `GET /ready` = 200 | `DATABASE_URL` stand DSN. `CONTRACT_RATE_SERVICE_PORT=8091`. `RFX_SERVICE_URL=http://rfx-service:8084`. `INTERNAL_SERVICE_TOKEN` from the runner. `ENVIRONMENT=test`. |
| transport-order-service | `services/transport-order-service/Dockerfile` | contract-rate `/ready` | `GET /ready` = 200 | `CONTRACT_RATE_SERVICE_URL=http://contract-rate-service:8091`. Same token and DSN. Port `8083`. |
| rfx-service | `services/rfx-service/Dockerfile` | transport-order `/ready` | `GET /ready` = 200 | `TRANSPORT_ORDER_SERVICE_URL=http://transport-order-service:8083`. `RFX_DEADLINE_WORKER_ENABLED=false`. Port `8084`. |
| shipment-service | `services/shipment-service/Dockerfile` | redpanda topics exist, migrate succeeded | `GET /ready` = 200 | `SHIPMENT_OUTBOX_ENABLED=true`, `SHIPMENT_OUTBOX_TRANSPORT=kafka`, `SHIPMENT_KAFKA_BROKERS=redpanda:9092`, `SHIPMENT_KAFKA_TOPIC=shipment.status.v1`, `SHIPMENT_KAFKA_DRIVER_TOPIC=driver.events.v1`, `SHIPMENT_KAFKA_CLIENT_ID=shipment-service`, poll `500ms`, publish timeout `5s`, lease `30s`, `DRIVER_NOTIFICATION_ENABLED=false`, `TMS_RELATIONSHIP_ACTIVATION_ENABLED=false`. Port `8085`. |
| document-service | `services/document-service/Dockerfile` | migrate succeeded | `GET /ready` = 200 | `DOCUMENT_STORAGE_ROOT` on the stand document volume. Port `8086`. |
| billing-register-service | `services/billing-register-service/Dockerfile` | migrate succeeded | `GET /ready` = 200 | `BILLING_FREIGHT_COST_OUTBOX_ENABLED=false`. No `PAYMENT_SERVICE_URL`. Port `8087`. |
| control-tower-read-model-service | `services/control-tower-read-model-service/Dockerfile` | topics exist, shipment `/ready` | `GET /ready` = 200 and log `starting shipment status consumer` | `CONTROL_TOWER_CONSUMER_ENABLED=true`, `CONTROL_TOWER_DATABASE_URL` same DSN, `CONTROL_TOWER_KAFKA_BROKERS=redpanda:9092`, `CONTROL_TOWER_KAFKA_TOPIC=shipment.status.v1`, `CONTROL_TOWER_KAFKA_GROUP_ID=control-tower-e2e-<run-id>`, `CONTROL_TOWER_KAFKA_CLIENT_ID=control-tower-e2e`, `CONTROL_TOWER_DRIVER_EVENTS_ENABLED=false`, `SHIPMENT_SERVICE_URL=http://shipment-service:8085`. Port `8089`. |
| runner | future test image, or `docker compose run` with the repo mounted | all `/ready` above | `go test` exit 0 | `TEST_DATABASE_URL` set. `REQUIRE_TEST_DATABASE=1`. Broker override is not implemented on this baseline. |

If the runner keeps building the seven Go processes itself, the Compose project starts only `postgres`, `redpanda`, `topic-init`, and `migrate`. The application rows above are then process dependencies of the harness, with the same environment and the same ready checks. Do not start both copies.

## Optional services

| Service | Depends on | Stand decision |
| --- | --- | --- |
| identity-service | postgres | Omit. Not called by the test. |
| company-service | postgres | Omit. Not called by the test. |
| api-gateway | identity, company, and downstream URLs | Omit for this scenario. A later public-boundary probe would set `AUTH_ENABLED=true` and would not replace the direct service calls. |
| payment-service | postgres, billing URL | Omit. The scenario requires zero payment obligations. |
| low-code-service | postgres, identity | Omit. |
| prometheus, grafana | gateway | Omit. Profiles `observability` stay off. |
| tracking-service, freight-cost-service | staging overlay only | Omit. Staging gives tracking host port `8089`, which is the control-tower port in the base file. The stand does not import that overlay. |

## Health and stop

Ready order is the startup list above. A failed `/ready` stops the run before seed SQL and before the first RFx POST.

Stop and remove only this project:

```text
docker compose -p bintrans-e2e-stand-v0.1a -f <future-stand-file> down -v --remove-orphans
```

That command is specified here and is not run in this stage. It must not be pointed at `docker-compose.yml`, `docker-compose.bintrans-ct-staging.yml`, or project `bintrans-ct-staging`. Do not run `docker volume prune`, `docker system prune`, or `docker network rm freight-platform-network`.

## Dev file gaps that a later implementation has to close

1. Add the stand file. Do not edit the base file in order to hide staging ports.
2. Teach `startKafka` to use `redpanda:9092` when the stand broker variable is set, and to fail if that variable is set and `kfake` would still start.
3. Decide one migration owner: the migrate container or `applyMigrations`. The harness currently applies SQL itself and does not write `schema_migrations`. golang-migrate does. Running both against one database will fail or double-apply.
4. Give the runner a unique consumer group per invocation. The base file uses the fixed group `control-tower-shipment-status-v1`.
5. Set service `LOG_LEVEL=info` on shipment and control-tower for the stand. The harness currently sets `error`, which drops the proof lines described in the runbook.
