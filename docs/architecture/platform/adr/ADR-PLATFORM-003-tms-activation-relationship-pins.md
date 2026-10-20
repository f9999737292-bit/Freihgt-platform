# ADR-PLATFORM-003: TMS activation pins an ACTIVE relationship version

## Status

Proposed. Architecture only. Runtime, a migration file, a gateway route, a feature-flag enablement, an NLO change, and deployment are not authorized.

```text
ADR_NUMBER=ADR-PLATFORM-003
ADR_PLATFORM_001_RESERVED_BY=PR_291_PUBLIC_ACCEPT
ADR_PLATFORM_002_RESERVED_BY=PR_292_RELATIONSHIP_REVOKE
ADR_NUMBER_COLLISION=NO
DOCS_ONLY=YES
RUNTIME_AUTHORIZED=NO
MIGRATION_NUMBER_ASSIGNED=NO
```

`ADR-PLATFORM-001` is public accept. `ADR-PLATFORM-002` is relationship revoke. On `e810056284ebb39014572903bf73a5ba29fc52e9` the platform ADR directory contains only those two files. This decision uses `ADR-PLATFORM-003`. It does not reuse either number.

## Context

Shipment-service already commits a transport execution from an NLO route-plan activation. The internal route is `POST /internal/v1/transport-executions/from-route-plan-activation`. The only authorized caller is `network-optimizer-service`. `TransportExecutionRepository.Project` inserts `transport.transport_executions`, one `ACTIVE` revision, participants, stops, actions, driver stop tasks, and the `shipment.execution_plan.created` outbox row in one PostgreSQL transaction. The revision is unique on `(operating_tenant_id, source_activation_id)`.

That transaction does not read `company.carrier_relationship_heads`. The revision has no `relationship_id`. The pin table named by the 0.1B runtime design, `transport.transport_execution_carrier_relationship_pins`, does not exist. `company.authorize_carrier_relationship_activation` does not exist. `driver_id` and `vehicle_id` are copied from the projection command onto the execution row. A client-supplied relationship version is not a column, and it must not become one.

Company-service and shipment-service both use the database `freight_platform` in `infrastructure/docker-compose/docker-compose.yml`. They currently share the `freight` login. Schemas `company` and `transport` are in that database. NLO then links `network_optimizer.route_plan_activations` to `EXECUTION_LINKED` in a later transaction, after the HTTP acknowledgement. Control Tower consumes execution events after commit. Those two steps are not inside the projection transaction.

0.1A froze `CARRIER_RELATIONSHIP_PIN_OWNER=TRANSPORT_EXECUTION_REVISION`, `RELATIONSHIP_REVOKED_BEFORE_ACTIVATE_POLICY=REFUSE_NO_EXECUTION`, and `RELATIONSHIP_REVOKED_AFTER_EXECUTION_POLICY=HISTORICAL_EXECUTION_UNCHANGED`. 0.1B froze the same-database authorization function and forbade an HTTP read of `ACTIVE` followed by a later commit. ADR-PLATFORM-002 requires the future activation function to take the relationship advisory lock before the head lock. This ADR binds those decisions to the code on `e8100562`. It does not implement them.

## Decision

TMS activation may pin a relationship only inside the existing shipment-service projection transaction, by calling a company-owned SQL function on the same `freight_platform` connection. The function locks the relationship, requires the current head to be `ACTIVE`, and returns the server-read version. Shipment-service inserts immutable pin rows that name that version, then inserts the execution revision, then commits once. A `REVOKED` head rolls the transaction back. No execution, pin, authorization, audit, outbox, or driver task remains.

```text
TMS_EXECUTION_OWNER=SHIPMENT_SERVICE
ACTIVATION_AUTHORITY=company.authorize_carrier_relationship_activation
ACTIVATION_AUTHORITY_PLACEMENT=SAME_FREIGHT_PLATFORM_TRANSACTION_AS_EXECUTION_INSERT
HTTP_READ_THEN_COMMIT_ALLOWED=NO
CLIENT_SUPPLIED_RELATIONSHIP_VERSION_AUTHORITY=NO
CLIENT_SUPPLIED_RELATIONSHIP_FIELDS_POLICY=REFUSE_BEFORE_TRANSACTION
OPERATING_CARRIER_IDENTITY=company.carrier_relationships.operating_carrier_company_id
SHIPPER_LOCAL_CARRIER_IS_SAME_UUID=NO
DRIVER_VEHICLE_IDENTITY=OPERATING_TENANT_ROWS
PIN_OWNER=TRANSPORT_EXECUTION_REVISION
PIN_MUTATION_AFTER_COMMIT=FORBIDDEN
SHARED_POSTGRESQL_TRANSACTION=YES_FOR_COMPANY_AND_TRANSPORT_SCHEMAS_ONLY
NLO_LINK_IN_THAT_TRANSACTION=NO
CONTROL_TOWER_IN_THAT_TRANSACTION=NO
SPLIT_DATABASE_PLACEMENT=UNSUPPORTED_FAIL_CLOSED
SHARED_FREIGHT_LOGIN_PRESENT=YES
ACTIVATION_FLAG_ENABLE_BEFORE_ROLE_SPLIT=FORBIDDEN
```

The shared transaction is one local PostgreSQL transaction. It is not a two-phase commit and it is not an HTTP saga. It covers `company.*` writes performed by the function and `transport.*` writes performed by shipment-service, because both schemas are in `freight_platform` and the function runs on the shipment-service connection. It ends at `COMMIT` of `Project`. The NLO update to `EXECUTION_LINKED`, the HTTP response, and the Control Tower inbox are later and separate. Losing the acknowledgement does not roll back a committed pin. A crash before `COMMIT` leaves no pin.

The lock order for every relationship business key is the order Accept and Revoke already use:

1. `pg_advisory_xact_lock(hashtextextended('relationship:' || shipper_tenant_id || ':' || shipper_carrier_company_id || ':' || operating_tenant_id, 0))`.
2. `SELECT` the head `FOR UPDATE`.
3. Read `status`, `current_version`, and `current_version_id` from that locked row.
4. Insert the authorization, the pins, the execution, the revision, audit, and outbox.
5. `COMMIT`, which releases both locks.

All advisory locks for the subject set are taken first, in ascending lowercase UUID order of `(shipper_tenant_id, shipper_carrier_company_id)`. Head locks follow in that same order. Taking a head lock before the advisory lock is forbidden. It deadlocks with Revoke.

A replay that finds a committed revision for `(operating_tenant_id, source_activation_id)` returns that revision when the stored `pin_set_fingerprint` matches. It does not lock the relationship and it does not read the current head. A later `REVOKED` head does not change that revision. The same activation id with a different fingerprint returns `RELATIONSHIP_ACTIVATION_FINGERPRINT_CONFLICT` and does not insert a second execution.

`pin_set_fingerprint` is the 0.1B SHA-256 document. It names the operating company and the shipment subjects. It does not name `relationship_id` or `relationship_version_id`. Those values are written on the pin from the locked head.

The `freight` login is shared today. Table privileges therefore do not separate the services. The function is still the consistency mechanism. Enabling activation before a dedicated shipment login has `EXECUTE` on the function and no `INSERT` or `UPDATE` on `company.carrier_relationship_*` is forbidden. This ADR does not create that login.

## Consequences

A later runtime stage has to add the authorization table, the pin table, the function, the append-only pin trigger, the activation outbox event type, and the shipment-service call. No migration number is assigned here. Historical executions stay unpinned. There is no backfill. NLO continues to send the projection command and to link `EXECUTION_LINKED` after the acknowledgement. This ADR does not set `execution_supported` and does not change NLO code. Control Tower remains a post-commit consumer. Shipper-scoped responses still omit operating `driver_id` and `vehicle_id`.

## Alternatives rejected

An HTTP `GET` of the head, followed by the existing projection commit, loses to a revoke that lands between the two calls. A durable lease has the same window after the response. Putting the execution insert in company-service moves TMS ownership. Letting the client send `relationship_version_id` makes a stale or foreign version authoritative. Updating the pin when the head becomes `REVOKED` rewrites history. One distributed transaction that also commits the NLO link and the Control Tower projection is not provided by the current process boundaries, so this ADR does not claim it. Sharing the `freight` login is not a substitute for the function or for a later role split.
