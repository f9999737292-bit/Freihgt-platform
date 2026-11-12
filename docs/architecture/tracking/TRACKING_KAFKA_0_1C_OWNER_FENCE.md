# Tracking Kafka 0.1C — shipment owner fence

Status: `OWNER_EPOCH_FENCE` is the selected future contract. It is specified and not implemented.
`SHIPMENT_OWNER_RACE=OPEN`.
`SHIPMENT_OWNER_RACE_RESOLVED_BY_DESIGN=NO`.

This document does not change shipment-service, tracking-service, SQL, or migrations. PR #357 at `0cf21055dd2a91c5ef071d6982c1fcc70cd11670` stays unchanged. A second read of shipment, and an advisory lock held only in the tracking database, are not evidence of atomicity with shipment-service.

## 1. Where the owner is stored

`transport.transport_executions.operating_tenant_id` is the operating tenant of one execution. Migration `000088_tms_transport_execution_foundation_v0_1a.up.sql` states that shipment ownership is not moved and that participants keep `shipment_tenant_id`. The execution row is inserted with the command tenant in `insertExecution` (`services/shipment-service/internal/repository/transport_execution_repository.go`).

The same migration defines `UNIQUE (id, operating_tenant_id)` and a foreign key from revisions to that pair. `000103_tms_relationship_pins_security_foundation_v0_1.up.sql` rejects a change of `transport.transport_execution_revisions.operating_tenant_id` with `committed revision identity is immutable`. The foreign key has no `ON UPDATE CASCADE`.

Production updates found in shipment-service change `current_revision_id`, `updated_at`, or `event_seq`. They do not assign a new `operating_tenant_id`. `event_seq` (`000089_tms_transport_execution_commands_v0_1b.up.sql`) counts events. It is not an owner epoch. No `owner_epoch` column exists on the execution row or on `tracking.execution_tracking_state`.

`shipment_tenant_id` on `transport.transport_execution_participants` is a participant tenant. It is a different field from the operating tenant.

## 2. Who can write tracking state

Inside tracking-service, `ExecutionConsumer.Apply` (`services/tracking-service/internal/service/execution_consumer.go`) upserts, clears, or touches `tracking.execution_tracking_state`. `UpsertExecutionTrackingState`, `ClearExecutionTarget`, `TouchExecutionSequence`, and `MarkApproachEmitted` (`services/tracking-service/internal/repository/execution_tracking_repository.go`) write that table. ETA ingest (`services/tracking-service/internal/service/eta_ingest_service.go`) locks the same row and refuses the write when `locked.OperatingTenantID` differs from its target. Those writers share the tracking database. Their row lock does not lock `transport.transport_executions`.

No other service contains an `INSERT` or `Upsert` of `tracking.execution_tracking_state`. Integration tests insert rows directly. That test path is not a production owner.

## 3. How a consumer learns the tenant today

`ShipmentContextClient.Get` calls `GET /internal/v1/transport-executions/{executionId}/stops/{stopId}/tracking-context` and sets `X-Tenant-ID` from the Kafka payload tenant (`services/tracking-service/internal/service/shipment_context_client.go`). The route is registered with the internal-token middleware (`services/shipment-service/internal/http/router.go`). `TrackingContextHandler.Get` parses `X-Tenant-ID` through `resolveVerifiedTenant` (`services/shipment-service/internal/http/handlers/tenant.go` and `tracking_context_handler.go`). `GetTrackingContext` filters `e.operating_tenant_id = $2` (`transport_execution_repository.go`). The JSON body does not return an owner epoch.

`Apply` performs that HTTP read before `Begin`, and only for `shipment.route_stop.current`. The payload tenant is the lookup key. A 404 currently becomes a nil view. Other stop types do not call shipment at all. They compare the payload tenant with tracking state after the inbox insert.

## 4. Option 1 — serialized handoff with receiver acknowledgement

Name: `SERIALIZED_HANDOFF_WITH_RECEIVER_ACK`.
Decision for 0.1C: not selected.

The option would require a shipment command that proposes a new operating tenant, a durable handoff record, and a tracking acknowledgement before the new tenant may write. No such command or acknowledgement table exists.

Guarantee boundary, if it were built later:

- A handoff is complete only after the acknowledgement and the owner change commit together, or after an ordered log makes the acknowledgement the sole authority.
- Until that commit, the previous tenant remains the only writer.
- The new tenant's events stay `NON_TERMINAL` while the acknowledgement is absent.

Counterexamples that keep this option from closing the race:

- Shipment commits the new tenant, and the tracking acknowledgement is lost. Each service can observe a different owner.
- Tracking stores the acknowledgement, and the shipment commit then rolls back. Tracking would accept a tenant that shipment does not store.
- Two databases cannot make those two commits one transaction. An advisory lock in tracking does not include the shipment row.
- A repeated lookup after the acknowledgement still observes one service at a time.

A later controller decision can reopen this option only with a new shipment command, a new acknowledgement record, and an explicit cross-service protocol. 0.1C does not design that command.

## 5. Option 2 — server-minted owner epoch

Name: `OWNER_EPOCH_FENCE`.
Decision for 0.1C: selected as the future contract. Not implemented.

The epoch source is `transport.transport_executions` inside shipment-service. Shipment mints a positive monotonic integer when it inserts the execution. Any later authorized owner change, if a future migration makes one possible, increments it in that same shipment transaction. The value is never taken from the HTTP caller, the Kafka producer outside that transaction, or the tracking payload as an authority.

The outbox insert that already allocates `event_seq` must copy the current epoch into the event payload in that same transaction. Tracking then treats the payload epoch as a claim that still has to match a shipment read of the same execution. The shipment read used for acceptance must return the row epoch. The tracking client must not supply the epoch in `X-Tenant-ID`.

Before any tracking state mutation, in the tracking transaction, after the existing execution row lock and before a new inbox insert is treated as durable:

- the event epoch is present and is an integer greater than zero;
- the authoritative shipment epoch was read as the same tenant and the same epoch;
- a stored epoch is null or equal to that event epoch;
- a stored epoch greater than the event epoch rejects the write.

A null stored epoch allows a first state row only when the event epoch itself was present and matched the authoritative epoch. A missing event epoch never becomes `COMMITTED_STATE_EFFECT`. The inbox row and the state row commit together after that match. A failed check rolls both back.

A mismatch, a missing epoch, an unavailable shipment read, or a changed epoch rolls the transaction back. No inbox row remains, no state change remains, and the offset stays uncommitted. Reasons are `OWNER_EPOCH_MISMATCH`, `OWNER_EPOCH_MISSING`, and `OWNER_EPOCH_UNAVAILABLE`. Those reasons do not include tenant identifiers.

`NON_TERMINAL` is the classification for every one of those outcomes. Replay stays `NON_TERMINAL` until the same checks pass. A higher `last_execution_event_sequence` does not prove this event.

Guarantee boundary:

- The epoch in the outbox row matches the execution row at the moment that shipment transaction committed.
- Tracking rejects an event whose epoch is missing, lower, or different from the shipment read used for that attempt.
- Two tracking transactions in one database can still serialize on the tracking row lock.

This boundary does not provide:

- A shared transaction between the shipment session and the tracking session. Section 9 records that the tables share one PostgreSQL database in this repository and still use two sessions.
- Proof from a second HTTP read. Two attempts can both observe epoch 4. Shipment can then move to epoch 5. Both tracking transactions can still write epoch 4 when the write does not hold the shipment epoch row until commit.
- Proof from `pg_advisory_xact_lock` or `SELECT FOR UPDATE` on `tracking.execution_tracking_state` alone. Those locks do not include `transport.transport_executions`.
- A current implementation. The column, the payload field, and the shipment response field are absent.

Counterexample that remains when the sessions are separate: tracking reads epoch 4, the shipment transaction commits epoch 5, and tracking then commits a state row stamped 4. A repeated read does not close that window. An already committed effect for epoch 4 stays committed if a later owner change moves the epoch to 5. This contract does not require that earlier effect to be undone.

Because that window remains, the race is open. The selected contract is the rule for a later implementation. It is not a claim that the race is closed.

## 6. Current schema constraint, recorded separately

A revision's `operating_tenant_id` cannot be updated under migration `000103`, and the execution foreign key has no cascade. This is an observed constraint at base `3f306fc407aeda4bdc652d825954292f4a54fad6`. It explains why no current command changes the operating tenant. It is not the fence. A later migration can replace it. Design and tests must not treat "no assignment was found" as a permanent guarantee.

The live gap that exists without any owner change is the payload tenant. `Apply` can accept a tenant that shipment did not confirm inside the tracking transaction. For every event other than `shipment.route_stop.current`, shipment is not consulted at all.

## 7. Acceptance for a later implementation

- `OWNER_EPOCH_SOURCE=transport.transport_executions` in the shipment transaction that writes the outbox row.
- `CALLER_SUPPLIED_EPOCH_ACCEPTED=NO`.
- `SECOND_LOOKUP_IS_ATOMIC=NO`.
- `TRACKING_ADVISORY_LOCK_COVERS_SHIPMENT=NO`.
- Missing, stale, and unavailable epochs are `NON_TERMINAL`, with no inbox row and no offset commit.
- Historical events without an epoch stay `NON_TERMINAL`. They are not backfilled.
- This stage creates no column and no migration.

## 8. Missing conditions

Implementation stays blocked until all of the following exist under a later controller task:

- a nullable `owner_epoch` on the shipment execution and on tracking state, added above the then-current migration head;
- the epoch copied into the outbox payload in the same shipment transaction as `event_seq`;
- a shipment read that returns the server epoch without taking it from the caller header;
- a tracking compare-and-reject rule;
- a controller decision that closes the read-versus-commit window. Section 9 shows that a same-database lock does that only by crossing the current service boundary. This stage does not authorize that change. PR #357 stays at `0cf21055dd2a91c5ef071d6982c1fcc70cd11670`. A later implementation also waits for a revision of that contract so every terminal result requires `owner_epoch`. This stage does not edit PR #357.

## 9. Same-database fence feasibility

`SAME_DATABASE_VERIFIED=YES` for the repository topology below.
`SAME_DATABASE_FENCE_FEASIBLE=NO` inside the current service boundaries.
`LOCK_ORDER_DEFINED=YES` as the order a later boundary change would have to use. That order is not authorized.
`OWNER_EPOCH_LINEARIZATION_POINT=NOT_IMPLEMENTED`.
A deployment outside this repository that points the two services at different databases is `NOT_PROVEN`. These files do not describe that deployment.

### 9.1 Where the rows live

`transport.transport_executions` is created in schema `transport` by `000088_tms_transport_execution_foundation_v0_1a.up.sql`. Schema `transport` itself comes from `000001_create_schemas.up.sql`. `tracking.execution_tracking_state` is created in schema `tracking` by `000091_tms_execution_stop_tracking_v0_1d.up.sql`. Schema `tracking` comes from `000026_add_shipment_telemetry_foundation_v0.7.0.up.sql`. Both migrations belong to one chain whose head is `000105_platform_postgres_role_foundation_v0_2_i1`.

`infrastructure/docker-compose/docker-compose.yml` sets shipment-service `DATABASE_URL` to database `freight_platform` on the `postgres` service. `services/tracking-service/internal/config/config.go` and `services/shipment-service/internal/config/config.go` default `DATABASE_URL` to that same database name. `infrastructure/docker-compose/docker-compose.bintrans-ct-staging.yml` sets tracking-service `DATABASE_URL` from the same anchor as shipment-service. The schemas are two namespaces in one PostgreSQL database. They are logically separate. The services are separate processes.

### 9.2 Why one database is not one transaction

`ExecutionConsumer.Apply` calls `c.repo.Begin` on the tracking pool (`execution_consumer.go`). Shipment commands use a shipment pool transaction, including the `event_seq` update in `emitExecutionEvent` (`transport_execution_command_repository.go`). Two sessions can block each other with row locks. They do not commit together. A lock taken on the tracking session does not cover a row that this session never locks.

`000103_tms_relationship_pins_security_foundation_v0_1.up.sql` creates `shipment_runtime` as `NOLOGIN`. Migration `000105` grants that role tables in `transport`, `documents`, `rfx`, and selected `core` columns. A search of `infrastructure/migrations` finds no `GRANT` on schema `tracking` and no `tracking_runtime` role. `SELECT FOR UPDATE` on `transport.transport_executions` from tracking-service would be a new privilege and a new SQL dependency from tracking-service onto a shipment-owned table. That is a service-boundary change. This stage does not add it.

Shipment can commit an owner-epoch change in its own transaction without writing `tracking.execution_tracking_state`. ETA ingest and `MarkApproachEmitted` already write the tracking row from tracking-service. Those paths do not lock `transport.transport_executions`.

### 9.3 Lock order that would be required later

The scenario under test is:

```text
Tracking transaction begins
Tracking observes owner_epoch=4
Shipment attempts owner_epoch=5
Tracking attempts state write
Shipment attempts commit
Expected:
No successful state write for epoch=4
after authoritative epoch=5 has committed.
```

The order that makes that expectation hold, if a later controller authorizes one session to touch both tables:

1. `SELECT ... FROM transport.transport_executions WHERE id = $execution FOR UPDATE`.
2. `SELECT ... FROM tracking.execution_tracking_state WHERE execution_id = $execution FOR UPDATE`.
3. Read `owner_epoch` while the execution lock is held.
4. Write tracking state only when the locked epoch equals the event epoch.
5. Commit. Releasing the execution lock is the end of the critical section.

Shipment's epoch increment uses the same order: the execution row first, and the tracking state row second whenever that transaction also reads or writes it. Every path that locks both rows, including a future ETA path, uses that order. The opposite order, state row then execution row, deadlocks with the order above.

The linearization point is the commit of the transaction that holds `transport.transport_executions` `FOR UPDATE` and stores the new `owner_epoch`. After that commit, a later transaction that takes the same execution lock observes the new epoch and refuses a state write for the old epoch. A state write of epoch 4 that already committed while it held the execution lock at epoch 4 stays committed. A later move to epoch 5 does not roll that effect back.

That order is not available to the current two sessions. Tracking can observe epoch 4 through HTTP, release that observation, and write state after shipment has committed epoch 5. `SAME_DATABASE_FENCE_FEASIBLE=NO` without the boundary change in section 9.2.

### 9.4 Alternative left for the controller

`SHIPMENT_SINGLE_WRITER` was analyzed and not selected.

Under that alternative, the shipment transaction is the only writer of `tracking.execution_tracking_state`. It locks the execution row, increments the epoch, writes the tracking effect, and commits once. The tracking consumer would not write that state. A stale tracking write of epoch 4 would be absent because tracking would have no write. An effect that committed at epoch 4 would remain after a later epoch 5. The Kafka offset would still wait for a persistence proof, and PR #357 would need a contract revision before any consumer used that proof.

The alternative needs grants that `shipment_runtime` does not have: `INSERT` and `UPDATE` on `tracking.execution_tracking_state`. It also needs a decision for the other current writers, ETA ingest and `MarkApproachEmitted`, which update the same tracking row from tracking-service. Leaving those writers in place reopens a second writer. Moving them into shipment-service moves tracking state ownership across the service boundary.

This stage does not choose `SHIPMENT_SINGLE_WRITER`. `SHIPMENT_OWNER_RACE=OPEN`.

### 9.5 Related gates that stay in force

- The broker plan includes a separate cross-tenant case. `BROKER_SAFETY_PROVEN=NO`.
- `owner_epoch` is mandatory for every new `COMMITTED_STATE_EFFECT`.
- Historical backfill without the original bytes, the writing `event_id`, or a server epoch is forbidden.
- `MIXED_CONSUMER_DEPLOYMENT_GATE=CLOSED`.
- The non-stop allowlist is empty, so no non-stop type is an automatic skip.
- PR #346, PR #357, and PR #372 are unchanged. Issue #344 stays open.
