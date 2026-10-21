# ADR-PLATFORM-003: TMS activation pins an ACTIVE relationship version

## Status

Proposed. Architecture only. Runtime, a migration file, a gateway route, a feature-flag enablement, an NLO change, and deployment are not authorized. R1 binds replay to `contract_sha256`. R2 binds each pin to the locked subject version, keeps shipment DML on executions and revisions, sets every definer function to `search_path = pg_catalog`, and leaves a lost-ack body conflict pending in NLO. R3 judges the revision row stored at `COMMIT`, installs that check only with the `Project` that writes evidence, grants the actor reads Create, Accept, Cancel, and Revoke already use, and binds each pin to the full subject identity. R4 limits the `UPDATE` event to `source_activation_id`, `supersedes_revision_id`, and `operating_tenant_id`, so a legacy successor can mark the previous revision `SUPERSEDED` without a backfilled authorization.

```text
ADR_NUMBER=ADR-PLATFORM-003
ADR_PLATFORM_001_RESERVED_BY=PR_291_PUBLIC_ACCEPT
ADR_PLATFORM_002_RESERVED_BY=PR_292_RELATIONSHIP_REVOKE
ADR_NUMBER_COLLISION=NO
DOCS_ONLY=YES
RUNTIME_AUTHORIZED=NO
MIGRATION_NUMBER_ASSIGNED=NO
R1_REMEDIATION=PIN-01,PIN-02,PIN-03,PIN-04
R2_REMEDIATION=R2-01,R2-02,R2-03,R2-04,R2-05
R3_REMEDIATION=R3-01,R3-02,R3-03,R3-04
R4_REMEDIATION=R4-01
SECURITY_DEFINER_SEARCH_PATH=pg_catalog
ENFORCEMENT_TRIGGER_READS=CURRENT_ROW_AT_COMMIT
ENFORCEMENT_TRIGGER_UPDATE_OF=source_activation_id,supersedes_revision_id,operating_tenant_id
FLAG_BYPASSES_DB_ENFORCEMENT=NO
```

`ADR-PLATFORM-001` is public accept. `ADR-PLATFORM-002` is relationship revoke. On `e810056284ebb39014572903bf73a5ba29fc52e9` the platform ADR directory contains only those two files. This decision uses `ADR-PLATFORM-003`. It does not reuse either number.

## Context

Shipment-service already commits a transport execution from an NLO route-plan activation. The internal route is `POST /internal/v1/transport-executions/from-route-plan-activation`. The only authorized caller is `network-optimizer-service`. `TransportExecutionRepository.Project` inserts `transport.transport_executions`, one `ACTIVE` revision, participants, stops, actions, driver stop tasks, and the `shipment.execution_plan.created` outbox row in one PostgreSQL transaction. The revision is unique on `(operating_tenant_id, source_activation_id)`. `CanonicalDigest` stores that command as `transport.transport_execution_revisions.contract_sha256`. `RejectReplayOrCreate` returns the stored revision when the digest matches and returns `409 ACTIVATION_BODY_CONFLICT` when it does not. `replayAfterConflict` reads the committed row after the losing transaction rolls back.

That transaction does not read `company.carrier_relationship_heads`. The revision has no `relationship_id`. The pin table named by the 0.1B runtime design, `transport.transport_execution_carrier_relationship_pins`, does not exist. `company.authorize_carrier_relationship_activation` does not exist. `driver_id` and `vehicle_id` are copied from the projection command onto the execution row. A client-supplied relationship version is not a column, and it must not become one. `CreateSuccessorRevision` inserts another revision and points `current_revision_id` at it. It does not read a relationship head and today it copies no pins.

Company-service and shipment-service both use the database `freight_platform` in `infrastructure/docker-compose/docker-compose.yml`. They currently share the `freight` login. Migrations create no separate roles and no grants. Schemas `company` and `transport` are in that database. NLO then links `network_optimizer.route_plan_activations` to `EXECUTION_LINKED` in a later transaction, after the HTTP acknowledgement. Control Tower consumes execution events after commit. Those two steps are not inside the projection transaction.

0.1A froze `CARRIER_RELATIONSHIP_PIN_OWNER=TRANSPORT_EXECUTION_REVISION`, `RELATIONSHIP_REVOKED_BEFORE_ACTIVATE_POLICY=REFUSE_NO_EXECUTION`, and `RELATIONSHIP_REVOKED_AFTER_EXECUTION_POLICY=HISTORICAL_EXECUTION_UNCHANGED`. 0.1B froze the same-database authorization function and forbade an HTTP read of `ACTIVE` followed by a later commit. ADR-PLATFORM-002 requires the future activation function to take the relationship advisory lock before the head lock. This ADR binds those decisions to the code on `e8100562`. It does not implement them. R1 keeps `contract_sha256` as the replay predicate. `pin_set_fingerprint` stays the participant-set evidence from 0.1B section 15. It does not replace the digest.

## Decision

TMS activation pins a relationship inside one local shipment-service transaction on `freight_platform`. `shipment_runtime` inserts and updates `transport.transport_executions` and `transport.transport_execution_revisions`, including `event_seq` and successor revisions. It does not receive DML on pin or authorization tables. `company.authorize_carrier_relationship_activation` inserts the authorization header and subjects. `transport.attach_activation_pins` inserts pins only after the revision row exists. `transport.propagate_execution_relationship_pins` copies pins onto a successor. `company.record_activation_audit` writes the relationship audit and outbox after the pins. Each of those functions is `SECURITY DEFINER`, owned by `freight_migrations`, with `search_path = pg_catalog`, schema-qualified names, and no dynamic SQL. `EXECUTE` is granted to `shipment_runtime` only.

`company_runtime` keeps `SELECT`, `INSERT`, and `UPDATE` on the relationship request, root, version, head, idempotency, audit, and outbox tables that Create, Accept, Cancel, and Revoke already write. It also keeps `SELECT` on `core.users`, `core.company_memberships`, `core.companies`, `core.user_roles`, `core.roles`, and `core.tenants`. The five commands read the first five of those catalogs. They do not read `core.tenants` and they do not call `nextval`. It does not receive `EXECUTE` on the activation functions and does not receive `INSERT`, `UPDATE`, or `DELETE` on the authorization tables or the pin table.

The insert path locks every subject business key first, in ascending lowercase `shipper_tenant_id`, then lowercase `shipper_carrier_company_id`, and re-reads the revision. A committed row with the same `contract_sha256` returns HTTP 200 and does not lock heads or insert evidence. A different digest returns `ACTIVATION_BODY_CONFLICT` and inserts nothing. Only a still-absent activation locks heads `FOR UPDATE` in that same key order. `company.enforce_activation_subject_head` runs `BEFORE INSERT` on each subject, takes that same advisory lock, locks the head, and raises unless `status` is `ACTIVE` and `current_version_id` equals `locked_relationship_version_id`. A historical version row that remains `ACTIVE` is not eligible once the head has moved.

`shipment_runtime` then inserts the execution and the revision. Pins come next. Their foreign key to `transport.transport_execution_revisions` is immediate, so a pin before its revision fails. `fk_pin_subject_locked_version` keeps the version pair. `fk_pin_subject_identity` also requires `relationship_id`, `operating_tenant_id`, `operating_carrier_company_id`, `shipment_tenant_id`, `shipment_id`, and `source_company_id` to equal the cited subject. The authorization header does not reference the revision, so the evidence check and the pin foreign keys do not form a cycle. At `COMMIT`, `transport.enforce_new_activation_authorization` runs for every queued `INSERT`, `DELETE`, and `UPDATE` that names `source_activation_id`, `supersedes_revision_id`, or `operating_tenant_id`. `SET status` and `SET updated_at`, which is what `createSuccessorRevisionInTx` writes on the previous revision, do not queue it. Each queued event selects the revision still stored under that event's id and reads those three columns from that row. It does not trust the `NEW` snapshot. Naming a column in `SET` queues the event even when the value does not change, and the stored row is still the decision. A stored row with `source_activation_id` set and `supersedes_revision_id` null needs a matching authorization header. A stored successor, with `supersedes_revision_id` still set, does not. An insert that begins as a successor and an update that clears `supersedes_revision_id` before `COMMIT` is a new activation and needs a header. Changing `source_activation_id` is judged from the value still stored. Historical revisions committed before the trigger are not rewritten, and marking one `SUPERSEDED` does not create an authorization afterwards. Authorization headers and subjects are append-only, so a delete or retarget of the header cannot remove the evidence the check reads.

While every service still uses the shared `freight` login, that login owns the tables, so grants do not constrain it. Enabling `TMS_RELATIONSHIP_ACTIVATION_ENABLED` before the runtime logins exist and the shared login is gone is a deployment blocker. The flag does not disable the enforcement trigger. The trigger is installed in the same release as the `Project` that writes the authorization header. Until that pair is released, the trigger is absent and today's `Project` still commits. After the trigger exists, a headerless new activation fails at `COMMIT` with the flag on or off. A partial pair rolls back to the old `Project` and no trigger.

```text
TMS_EXECUTION_OWNER=SHIPMENT_SERVICE
ACTIVATION_ENTRY_POINT=company.authorize_carrier_relationship_activation
ACTIVATION_PIN_WRITER=transport.attach_activation_pins
ACTIVATION_AUTHORITY=company.authorize_carrier_relationship_activation
ACTIVATION_AUTHORITY_PLACEMENT=SAME_FREIGHT_PLATFORM_TRANSACTION_AS_EXECUTION_INSERT
ACTIVATION_FUNCTION_OWNER=freight_migrations
ACTIVATION_FUNCTION_SEARCH_PATH=pg_catalog
ACTIVATION_FUNCTION_DYNAMIC_SQL=FORBIDDEN
SHIPMENT_RUNTIME_EXECUTION_DML=SELECT_INSERT_UPDATE
SHIPMENT_RUNTIME_PIN_DML=FORBIDDEN
SHIPMENT_RUNTIME_AUTHORIZATION_DML=FORBIDDEN
SHIPMENT_RUNTIME_EXECUTE=company.authorize_carrier_relationship_activation,company.record_activation_audit,transport.attach_activation_pins,transport.propagate_execution_relationship_pins
COMPANY_RUNTIME_ACCEPT_REVOKE_DML=YES
COMPANY_RUNTIME_ACTIVATION_EXECUTE=NO
HTTP_READ_THEN_COMMIT_ALLOWED=NO
CLIENT_SUPPLIED_RELATIONSHIP_VERSION_AUTHORITY=NO
CLIENT_SUPPLIED_RELATIONSHIP_FIELDS_POLICY=REFUSE_BEFORE_TRANSACTION
SQL_TENANT_GUC_AUTHORITY=NO
OPERATING_CARRIER_IDENTITY=company.carrier_relationships.operating_carrier_company_id
SHIPPER_LOCAL_CARRIER_IS_SAME_UUID=NO
DRIVER_VEHICLE_IDENTITY=OPERATING_TENANT_ROWS
PIN_OWNER=TRANSPORT_EXECUTION_REVISION
PIN_MUTATION_AFTER_COMMIT=FORBIDDEN
REPLAY_PREDICATE=transport.transport_execution_revisions.contract_sha256
PIN_SET_FINGERPRINT_ROLE=PARTICIPANT_SET_EVIDENCE
PIN_SET_FINGERPRINT_REPLACES_CONTRACT_SHA256=NO
FINGERPRINT_MATCH_OVERRIDES_DIGEST_CONFLICT=NO
LEGACY_NULL_FINGERPRINT_SAME_DIGEST=RETURN_EXISTING_NO_NEW_AUTHORIZATION
LEGACY_NULL_FINGERPRINT_DIFFERENT_DIGEST=ACTIVATION_BODY_CONFLICT
SHARED_POSTGRESQL_TRANSACTION=YES_FOR_COMPANY_AND_TRANSPORT_SCHEMAS_ONLY
NLO_LINK_IN_THAT_TRANSACTION=NO
CONTROL_TOWER_IN_THAT_TRANSACTION=NO
NLO_HTTP_201=LINK_NEW_EXECUTION
NLO_HTTP_200=LINK_EXISTING_EXECUTION
NLO_HTTP_409_ACTIVATION_BODY_CONFLICT=DO_NOT_LINK_STAY_PENDING_EXECUTION
NLO_CONFIRMED_PRECOMMIT_REFUSAL=REJECT_ONLY_WHEN_NO_REVISION_STORED
IDENTICAL_CONCURRENT_PROJECT=HTTP_200_AFTER_LOSER_ROLLBACK
SPLIT_DATABASE_PLACEMENT=UNSUPPORTED_FAIL_CLOSED
SHARED_FREIGHT_LOGIN_PRESENT=YES
SHARED_FREIGHT_LOGIN=DEPLOYMENT_BLOCKER
ACTIVATION_FLAG_ENABLE_BEFORE_ROLE_SPLIT=FORBIDDEN
FLAG_BYPASSES_DB_ENFORCEMENT=NO
ENFORCEMENT_BEFORE_PROJECT_WRITES_EVIDENCE=FORBIDDEN
ENFORCEMENT_TRIGGER_READS=CURRENT_ROW_AT_COMMIT
ENFORCEMENT_TRIGGER_UPDATE_OF=source_activation_id,supersedes_revision_id,operating_tenant_id
PIN_IDENTITY_FK=fk_pin_subject_identity
SUCCESSOR_PIN_SOURCE=COPY_COMMITTED_PIN_SET
SUCCESSOR_AUTHORIZER=NOT_CALLED
SUCCESSOR_HEAD_READ=NO
```

The shared transaction is one local PostgreSQL transaction. It is not a two-phase commit and it is not an HTTP saga. It covers the company evidence written by the authorizer and the execution and revision rows written by `shipment_runtime`, because both schemas are in `freight_platform` and the calls run on the shipment-service connection. It ends at `COMMIT` of `Project`. The NLO update to `EXECUTION_LINKED`, the HTTP response, and the Control Tower inbox are later and separate. Losing the acknowledgement does not roll back a committed pin. A crash before `COMMIT` leaves no pin.

A replay that finds a committed revision for `(operating_tenant_id, source_activation_id)` compares `contract_sha256` with `CanonicalDigest` of the new command. The same digest returns HTTP 200 and does not lock the relationship head, does not call the authorizer, and does not insert evidence. Driver, vehicle, stops, actions, and the version fields of the canonical projection are part of that digest. The same subjects with a different driver or vehicle are a different digest and return `409 ACTIVATION_BODY_CONFLICT`. A matching `pin_set_fingerprint` does not turn that conflict into a replay.

A legacy revision is a committed row whose `pin_set_fingerprint` is null and whose revision has no pin rows. The same digest returns that execution and does not add authorization. Writing authorization onto it would name a relationship version the original commit never locked. A different digest returns `409 ACTIVATION_BODY_CONFLICT` and also adds nothing.

An identical concurrent `Project` may pass the first lookup and then wait on the advisory lock. After the lock it re-reads. When the winner has committed the same digest, the loser rolls back and the following read returns HTTP 200. The loser does not commit its own inserts. NLO treats 201 as a new execution it may link, and 200 as the existing execution it may link. `409 ACTIVATION_BODY_CONFLICT` is a real body mismatch. It is not a link, and it does not mark the NLO activation `REJECTED`. The NLO row stays `PENDING_EXECUTION`. Resending the original body returns HTTP 200, and only that response is linked. A confirmed pre-commit refusal may become `REJECTED` only when no revision is stored. An identical concurrent request must not be returned as 409, and `MarkRoutePlanActivationRejected` must not run for the 200 or 201 path or for `ACTIVATION_BODY_CONFLICT`. Current `finishActivationHandoff` still treats every non-temporary 409 as a reject. This ADR does not change that Go code.

`CreateSuccessorRevision` keeps today's execution row lock and revision insert. In the same transaction `transport.propagate_execution_relationship_pins` copies the current revision's pin rows onto the new revision. The copy keeps `relationship_version_id`, tenant ownership, operating company, and the original authorization-subject ids. It does not call the authorizer and does not read the head. A later revoke does not update either pin set. A copy failure rolls the successor transaction back. The successor command cannot replace the operating carrier. That change requires a new activation, not a successor.

The lock order for every relationship business key is the order Accept and Revoke already use, extended for a subject set:

1. `pg_catalog.pg_advisory_xact_lock(pg_catalog.hashtextextended('relationship:' || shipper_tenant_id || ':' || shipper_carrier_company_id || ':' || operating_tenant_id, 0))` for every subject, sorted by lowercase `shipper_tenant_id`, then lowercase `shipper_carrier_company_id`.
2. Re-read the revision, authorization header, `contract_sha256`, and `pin_set_fingerprint`. Return 200 or 409 before any head lock when a committed row exists.
3. `SELECT` each head `FOR UPDATE` in that same key order.
4. Read `status`, `current_version`, and `current_version_id` from those locked rows. The subject trigger repeats this check.
5. Insert the authorization header and subject rows.
6. Insert the execution.
7. Insert the execution revision.
8. Insert the immutable pins. The pin-to-revision foreign key is immediate.
9. Insert audit and outbox.
10. `COMMIT`, which reads each queued revision from the table, runs the deferred new-activation evidence check, and then releases the locks.

Taking a head lock before the advisory lock is forbidden. It deadlocks with Revoke. A committed pin stays after a later revoke.

## Consequences

A later runtime stage has to add the roles, the authorization tables, the pin table, the functions, the subject trigger, the deferred revision check, both composite pin foreign keys, the append-only pin and evidence triggers, the activation outbox event type, and the shipment-service calls. The deferred check and the `Project` that writes evidence are one release. No migration number is assigned here. Historical executions stay unpinned. There is no backfill. NLO continues to send the projection command and to link `EXECUTION_LINKED` after HTTP 200 or HTTP 201. A lost 201 followed by `ACTIVATION_BODY_CONFLICT` stays `PENDING_EXECUTION` until the original digest returns 200. This ADR does not set `execution_supported` and does not change NLO code. Control Tower remains a post-commit consumer. Shipper-scoped responses still omit operating `driver_id` and `vehicle_id`.

## Alternatives rejected

An HTTP `GET` of the head, followed by the existing projection commit, loses to a revoke that lands between the two calls. A durable lease has the same window after the response. Putting the execution insert in company-service moves TMS ownership. Letting the client send `relationship_version_id` makes a stale or foreign version authoritative. Updating the pin when the head becomes `REVOKED` rewrites history. Treating `pin_set_fingerprint` as the replay key returns an old execution when the driver or vehicle changed and drops the current `ACTIVATION_BODY_CONFLICT` check. One distributed transaction that also commits the NLO link and the Control Tower projection is not provided by the current process boundaries, so this ADR does not claim it. Sharing the `freight` login is not a substitute for the functions or for a later role split. A written ban on `INSERT`, granted to the same login that owns the tables, does not stop that login. A successor that reads the current head would let a later revoke block or rewrite a route change for an execution that already pinned an older version. Judging the deferred check from `NEW.supersedes_revision_id` lets an update clear that column after the insert snapshot was queued. Installing the enforcement trigger before `Project` writes a header makes today's activation fail while the flag is still off. Using the flag to skip the trigger leaves a database bypass in application configuration.
