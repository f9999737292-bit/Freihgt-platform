# BINTRANS E2E Isolated Stand 0.1A — Execution Runbook

Stage: `BINTRANS-E2E-ISOLATED-STAND-0.1A-DISCOVERY`

Baseline: `fb9dc6c409f048c4b9f913bb40f315c2a11b4d6a`. Do not run this runbook in the discovery stage. `docker compose up`, `docker compose down`, and migration against any existing database are out of scope until a later authorized stage.

## Current blocker

`tests/bintrans-e2e-transport-flow/harness.go` function `startKafka` always starts `kfake`. It does not read a broker address. `TEST_DATABASE_URL` is already honored, and CI sets it. A database container alone does not make the scenario a Redpanda test.

Until a later change adds an explicit stand-broker switch that refuses to start `kfake`, the stand is specified and not executable. CI job `bintrans-e2e-transport-flow` remains the `kfake` job. A green run of that job is not stand evidence.

## Preconditions for a later authorized run

1. Worktree is this scenario's implementation branch, not the dirty Control Tower checkout and not `bintrans-ct-staging`.
2. Docker is local. The target is not the staging VM.
3. Project name is `bintrans-e2e-stand-v0.1a`.
4. No published host ports.
5. Ephemeral Postgres credentials exist only in the process environment.
6. Staging env file is not loaded.
7. The harness broker switch is present and the test process is on network `bintrans-e2e-stand`.

## Migration plan

Apply on an empty database inside the stand container only.

1. Confirm the data directory is empty and `schema_migrations` is absent.
2. Apply `infrastructure/migrations/000001_create_schemas.up.sql` through `000105_platform_postgres_role_foundation_v0_2_i1.up.sql`. There are 105 up files. Lexical order matches version order on this baseline.
3. Prefer one owner:
   - either golang-migrate `v4.17.1` with `up`, which records version `105`;
   - or the harness `applyMigrations`, which executes each up file and does not record `schema_migrations`.
4. Do not run both against the same database.
5. The dev Compose migrate service is not a valid command yet: its `command` has `-path` and `-database` and omits `up`.
6. After apply, read-only checks:
   - latest file prefix is `000105_`;
   - `transport.assert_activation_enforcement_role_split()` exists;
   - trigger `trg_new_activation_authorization` does not exist;
   - `freight_migrations`, `company_runtime`, and `shipment_runtime` are `NOLOGIN`;
   - no new `LOGIN` role was created by the migrations.
7. Do not run `infrastructure/postgres/tms_activation_enforcement_gate_v0.1.sql`.
8. Do not run `make migrate-up` from a shell whose `COMPOSE` target is the shared platform file.

This discovery stage did not apply these files. The application path is the one already used by the harness and by CI against a throwaway Postgres 16. That is not a new execution performed here.

## Seed

Allowed before the first business POST, and already implemented in `seedFlowFixture`:

- two `core.tenants` rows (operating tenant and foreign tenant);
- shipper, other shipper, awarded carrier, other carrier, and foreign carrier;
- users and company memberships for those actors;
- links to existing roles `PROCUREMENT_MANAGER` and `CARRIER_DISPATCHER`;
- two locations, three drivers, and three vehicles.

Not allowed as seed:

- `transport.shipments`
- `transport.transport_orders`
- `documents.documents` POD rows
- `billing.freight_settlements`
- `billing.billing_registers`
- `billing.closing_document_packages`
- `control_tower.shipment_status_projection`

The test already fails if award conversion inserts an order early, or if conversion inserts a shipment.

## HTTP scenario

The test already performs this sequence. The stand run keeps it.

1. Buyer creates one RFx, adds the awarded carrier, adds one lot and one lane, and publishes.
2. Carrier submits one commercial response. Buyer awards it.
3. Buyer converts the award. Expect one order, source `rfx_award`, and no shipment yet.
4. Awarded carrier executes the order. Expect one shipment.
5. Negative execute and assignment calls, then the awarded vehicle and driver.
6. Repeat conversion and repeat execute. Expect the same ids and no extra rows.
7. Foreign conversion and a public create with a mismatched body tenant. Expect no extra order.
8. Driver events through `DELIVERY_COMPLETED`. Replay the same idempotency key. Expect one `DELIVERED` history row.
9. Wait until one outbox row for that shipment is `PUBLISHED` with `toStatus=DELIVERED`, and no unpublished row remains.
10. Read `shipment.status.v1` from Redpanda and find one record that contains the shipment id and `DELIVERED`.
11. `GET /internal/v1/control-tower/shipments/{id}/status` for the operating tenant returns `DELIVERED`. The foreign tenant receives 404. Projection rows: one for the operating tenant, zero for the foreign tenant.
12. Document-service POD intent, `PUT` content, and complete. Replay complete. Expect one POD and one file.
13. Billing settlement create, submit, buyer approve, include in register, calculate, approve register, and closing-document package. Replay the package. Register status is `CLOSING_DOCUMENTS_CREATED`.
14. Counts are one settlement, one register, one item, one package, one act, one VAT invoice, and zero payment obligations. Status is not `PAID` or `CLOSED`.

## Redpanda proof

| Check | Evidence | Enough by itself |
| --- | --- | --- |
| Producer | Shipment log `outbox event published` with `aggregate_id`, `event_type`, and `result=published`. Requires `LOG_LEVEL=info`. The harness sets `error` today. | No. Pair it with the outbox row. |
| Broker | Independent reader of `shipment.status.v1` from `redpanda:9092`, same predicate as `assertBrokerDelivered`. | Proves the record was on Redpanda. Does not prove the control-tower group consumed it. |
| Consumer start | Log `starting shipment status consumer` with topic `shipment.status.v1` and group `control-tower-e2e-<run-id>`. | Proves the group was configured, not that the record was applied. |
| Consumer apply | Log `shipment status event processed` with `shipment_id`, `event_id`, `event_type`, and `outcome`. This line has no `tenant_id`. | Not sufficient for tenant isolation. |
| Projection | HTTP 200 `currentStatus=DELIVERED` for the operating tenant, HTTP 404 for the foreign tenant, and the two SQL counts in the test. | This is the business proof. |
| Replay | Second `DELIVERY_COMPLETED` with the same idempotency key does not add a second `PUBLISHED` `DELIVERED` outbox row. Projection primary key is `(tenant_id, shipment_id)` with `ON CONFLICT DO UPDATE`. | Required so a redelivery is not a second shipment. |

Topic names and group:

| Setting | Value |
| --- | --- |
| Bootstrap | `redpanda:9092` inside the stand network |
| Status topic | `shipment.status.v1` |
| Driver topic configured on shipment | `driver.events.v1` |
| Control-tower driver consumer | off |
| Consumer group | `control-tower-e2e-<run-id>` |
| Acks | all ISR |
| Retries | 3 produce retries in the publisher |

Do not point the stand at `localhost:19092`. Do not reuse group `control-tower-shipment-status-v1` from the dev file for the proof run.

## Cleanup after a later run

Follow [ROLLBACK_AND_CLEANUP_PLAN_0_1A.md](ROLLBACK_AND_CLEANUP_PLAN_0_1A.md). The harness also drops its private database with `DROP DATABASE ... WITH (FORCE)` when it created one. That drop is allowed only for a database whose name starts with `bintrans_e2e_flow_` on the stand instance.

## What this stage executed

Repository identity, `origin/main`, PR #304 merge commit, migration file inventory, Compose and CI inspection, and the test source. No Compose command. No SQL session. No credential creation.
