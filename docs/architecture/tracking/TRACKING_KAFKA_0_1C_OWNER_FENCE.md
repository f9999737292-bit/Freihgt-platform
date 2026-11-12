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

Before any tracking state mutation, in the tracking transaction, after the existing execution row lock and before a new inbox insert is treated as success:

- the event epoch is present and is an integer greater than zero;
- the trusted shipment read returned the same tenant and the same epoch;
- stored `owner_epoch` is null or equal to the event epoch;
- a stored epoch greater than the event epoch rejects the write.

A mismatch, a missing epoch, an unavailable shipment read, or a changed epoch rolls back. There is no inbox insert, no state change, and no offset commit. Reasons are `OWNER_EPOCH_MISMATCH`, `OWNER_EPOCH_MISSING`, and `OWNER_EPOCH_UNAVAILABLE`. Those reasons do not include tenant identifiers.

`NON_TERMINAL` is the classification for every one of those outcomes. Replay stays `NON_TERMINAL` until the same checks pass. A higher `last_execution_event_sequence` does not prove this event.

Guarantee boundary:

- The epoch in the outbox row matches the execution row at the moment that shipment transaction committed.
- Tracking rejects an event whose epoch is missing, lower, or different from the shipment read used for that attempt.
- Two tracking transactions in one database can still serialize on the tracking row lock.

This boundary does not provide:

- A shared transaction between shipment-service and tracking-service.
- Proof from a second HTTP read. Two attempts can both observe epoch 4. Shipment can then move to epoch 5. Both tracking transactions can still write epoch 4 if the write does not also carry a fence that shipment enforces at write time.
- Proof from `pg_advisory_xact_lock` or `SELECT FOR UPDATE` in tracking. Those locks do not include the shipment row.
- A current implementation. The column, the payload field, and the shipment response field are absent.

Counterexample that remains even after the column exists: tracking reads epoch 4, the shipment owner transaction commits epoch 5, and tracking then commits a state row stamped 4. Closing that window requires the state write to be rejected by a check that cannot succeed after epoch 5 is stored, in the same database as the epoch or through a shipment write that is the only writer of tracking state. Tracking is a separate database, and moving the state write into shipment-service is outside this task. Until a controller authorizes one of those mechanisms, the epoch contract detects a stale claim on a later read and still loses the race between the read and the commit.

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
- a controller decision for the residual read-versus-commit window in section 5, which this option does not close by itself.
