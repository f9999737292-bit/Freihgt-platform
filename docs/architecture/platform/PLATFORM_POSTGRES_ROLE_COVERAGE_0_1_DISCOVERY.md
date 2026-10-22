# Platform PostgreSQL Role Coverage 0.1

Discovery and connection contract. No migration, no password, no `pg_hba` file, no enforcement-gate install, and no feature-flag change is included.

```text
STAGE=PLATFORM-POSTGRES-ROLE-COVERAGE-0.1-DISCOVERY
BASE=00bc72d6a6b3bbd70cb813ef9c0fe975f2efcebb
ORIGIN_MAIN_MATCHES_EXPECTED_BASE=YES
MIGRATION_HEAD=000104_tms_activation_runtime_grants_v0_1
PR_295_IN_MAIN=YES
PR_296_IN_MAIN=YES
PR_297_IN_MAIN=YES
PR_298_REFERENCE=553a35e43dcbe34f586b70edcb5e6e49df42f613
PR_298_MODIFIED=NO
ADR_PLATFORM_004_MODIFIED=NO
DOCS_ONLY=YES
PRODUCT_CODE_CHANGED=NO
MIGRATION_CREATED=NO
CREDENTIAL_CREATED=NO
PG_HBA_CHANGED=NO
ENFORCEMENT_GATE_INSTALL_AUTHORIZED=NO
FEATURE_FLAG_ENABLE_AUTHORIZED=NO
DEPLOYMENT_AUTHORIZED=NO
LIVE_CATALOG_READ=NO
```

PR #298 and ADR-PLATFORM-004 stay the rollout reference. They are open and unmodified. This document plans how the shared login stops being required. It does not perform that change.

The observed statement list is `PLATFORM_POSTGRES_ROLE_COVERAGE_0_1_OBSERVED_SQL.md`.

## 1. Inventory method

Runtime SQL is the set of string literals under `services/`, excluding `*_test.go`, `integration`, `testdata`, and `testkit`. `SELECT` includes `FROM` and `JOIN`. Function calls are recorded separately from tables.

These matches were discarded:

- the `EXCLUDED` row alias;
- `SET role = $4`, which assigns the participant column `role`;
- the words `copy option` and `copy rule`, which are not `COPY`;
- `pg_catalog` helpers such as `unnest` and `array_agg`.

`company.authorize_carrier_relationship_activation(...)` is a set-returning function used in `FROM`. It needs `EXECUTE`, not a table grant.

No live server was queried. A statement built only by concatenation would be absent. The company and shipment repositories that this contract depends on use const SQL.

`FOR UPDATE` and `FOR UPDATE SKIP LOCKED` require `UPDATE` on the locked relation, including when the statement does not change a column. `FOR UPDATE` on `transport.shipments`, `transport.cargoes`, `transport.drivers`, and `transport.vehicles` is already covered by the `UPDATE` grant in `000104`.

## 2. PostgreSQL consumers

`infrastructure/docker-compose/docker-compose.yml` on this base sets the bootstrap user name to `freight`. The official PostgreSQL image makes that user a superuser. The same login is the `DATABASE_URL` user for identity-service, company-service, transport-order-service, rfx-service, shipment-service, document-service, billing-register-service, payment-service, and low-code-service. control-tower-read-model-service reads `CONTROL_TOWER_DATABASE_URL`, and the compose file points that variable at the same login. The migrate profile uses `POSTGRES_USER`, which defaults to the same name. Development passwords already in that file are not repeated here.

These binaries also open `DATABASE_URL` and are not services in that compose file:

| Binary | Runtime schema | Compose on this base |
| --- | --- | --- |
| tracking-service | `tracking` | absent; the config default user name is `freight` |
| network-optimizer-service | `network_optimizer` | absent |
| freight-cost-service | `freight_cost` | absent |
| contract-rate-service | `contract_rate` | absent |

analytics-service and localization-service do not open PostgreSQL. api-gateway opens a pool only in integration tests.

Each inventoried server loads one URL and opens one `pgxpool`. company-service does this in `database.Connect`. shipment-service does the same. `cmd/shipment-outbox-replay`, `cmd/shipment-status-snapshot-export`, and `cmd/control-tower-status-snapshot-import` use that same URL for their service. There is no second pool that could keep activation on `shipment_runtime` while the rest of the process stays on `freight`.

The compose file has no PgBouncer. Session state that the binaries do use is transaction-local: `pg_advisory_xact_lock` in company-service, transport-order-service, freight-cost-service, and control-tower, and `set_config('lock_timeout', ..., true)` in the control-tower rebuild. Those do not require a superuser. `pg_advisory_xact_lock` and `hashtextextended` are executable by a normal login.

No runtime statement requires superuser, `BYPASSRLS`, `CREATEDB`, `CREATEROLE`, or ownership of the protected relations. The current login has those powers because it is the image bootstrap user. Grants do not constrain it. That is the bypass `transport.assert_activation_enforcement_role_split()` rejects.

## 3. Company-service coverage

`000103` grants `company_runtime` `SELECT`, `INSERT`, and `UPDATE` on the relationship, version, head, request, request-event, request-head, four idempotency, audit, and outbox tables. It grants `SELECT` on `core.companies`, `core.tenants`, `core.users`, `core.company_memberships`, `core.user_roles`, and `core.roles`. It grants `USAGE` on schemas `company`, `transport`, and `core`. It revokes `INSERT`, `UPDATE`, and `DELETE` on the activation evidence tables and on the pin table. The role is `NOLOGIN`.

Relationship Create, Accept, Cancel, and Revoke, their audit and outbox inserts, and the authorization reads of users, roles, memberships, and companies match those grants. `FOR UPDATE` on `company.carrier_relationship_request_heads` and the relationship head rows is covered by the existing `UPDATE` grant. The advisory transaction locks need no extra grant. No runtime `nextval` appears. No runtime `DELETE` appears on the relationship tables, and `DELETE` is not granted.

Company administration does not fit the same role:

| Relation | Runtime operations | Granted today | Gap |
| --- | --- | --- | --- |
| `core.companies` | `SELECT`, `INSERT`, `UPDATE` | `SELECT` | `INSERT`, `UPDATE` |
| `core.company_memberships` | `SELECT`, `INSERT`, `UPDATE` | `SELECT` | `INSERT`, `UPDATE` |
| `core.user_roles` | `SELECT`, `INSERT`, `DELETE` | `SELECT` | `INSERT`, `DELETE` |
| `core.users` | `SELECT` | `SELECT` | none |
| `core.roles` | `SELECT` | `SELECT` | none |
| `core.tenants` | none in this binary | `SELECT` | surplus `SELECT`; leave it |

`core.user_roles` is also written by identity-service. Both roles need `SELECT`, `INSERT`, and `DELETE`. Neither runtime statement updates that table, so the later grant does not include `UPDATE`.

No inventoried runtime binary inserts `core.tenants`, `core.roles`, `core.permissions`, or `core.role_permissions`. Those rows are created by migrations and by tests. Tenant and role-catalog writes stay with the migration login.

Company administration is direct DML. A security-definer function is the wrong shape: the binary is supposed to write these rows, and a definer would hide that write behind the function owner. The activation evidence tables stay revoked from `company_runtime`.

Several relationship grants include `UPDATE` where the runtime strings only insert, and `SELECT` on `core.tenants` has no runtime reader in this binary. The first grant migration does not revoke those. A revoke needs its own proof.

## 4. Shipment-service coverage

`000103` and `000104` let `shipment_runtime` run `Project` and `CreateSuccessorRevision` and the execution child rows those commands write. They do not let the same pool create a shipment, assign a driver or vehicle, record delivery disposition, or read source authority. The process has one pool, so the activation path cannot move until the rest of the binary can run as the same role.

### 4.1 Already sufficient

| Relation or function | Why it is sufficient |
| --- | --- |
| `transport.transport_executions` | `SELECT`, `INSERT`, `UPDATE` granted and used |
| `transport.transport_execution_revisions` | `SELECT`, `INSERT`, and column `UPDATE` for the columns `Project` and `CreateSuccessorRevision` set. `DELETE` is revoked. `pin_set_fingerprint` is absent from the column list and from runtime `UPDATE` statements |
| execution participants, active shipments, stops, revision stops, actions, revision actions, commands, command audit | granted `SELECT`, `INSERT`, `UPDATE` |
| `transport.driver_stop_tasks`, `transport.shipment_event_outbox` | granted `SELECT`, `INSERT`, `UPDATE` |
| `transport.shipments`, `transport.cargoes`, `transport.drivers`, `transport.vehicles` | `SELECT` and `UPDATE` cover the readers and the `FOR UPDATE` locks in activation |
| `transport.locations`, `core.companies` | `SELECT` granted and used |
| `company.authorize_carrier_relationship_activation` | `EXECUTE` granted. The function is `SECURITY DEFINER`, owned by `freight_migrations`, and writes evidence the runtime role cannot write |
| `transport.attach_activation_pins`, `transport.propagate_execution_relationship_pins`, `company.record_activation_audit` | `EXECUTE` granted. Propagate sets `pin_set_fingerprint` as the function owner |
| `company.activation_subject_input` | `USAGE` granted. The authorizer call casts rows to that type |

`EXECUTE` on the trigger helper functions is already granted to `shipment_runtime` so a later trigger can fire in that session. The application does not call those helpers as statements.

### 4.2 Gaps that block a shipment login

Schema `USAGE` is granted on `company`, `transport`, and `core` only. The cross-schema reads also need `USAGE` on `documents` and `rfx`.

| Relation | Runtime need | Later grant |
| --- | --- | --- |
| `transport.shipments` | `INSERT`, plus the `SELECT` and `UPDATE` already granted | `INSERT` |
| `transport.drivers` | `INSERT`, plus the existing `SELECT` and `UPDATE` that cover `FOR UPDATE` | `INSERT` |
| `transport.vehicles` | `INSERT`, plus the existing `SELECT` and `UPDATE` | `INSERT` |
| `core.users`, `core.company_memberships`, `core.user_roles`, `core.roles` | `SELECT` for source authority | `SELECT` |
| `documents.documents` | `SELECT` | `USAGE` on `documents`, `SELECT` |
| `rfx.bids` | `SELECT` | `USAGE` on `rfx`, `SELECT` |
| `rfx.rfx_award_transport_orders` | `SELECT` and `FOR UPDATE` in `lockAwardLink`. The statement changes no column | no table `UPDATE`. Add a definer lock in section 4.3 |
| `transport.transport_orders` | `SELECT`, `UPDATE` of `status`, `version`, `updated_at`, and `FOR UPDATE` | `SELECT` and column `UPDATE (status, version, updated_at)` |
| `transport.transport_execution_pod_requirement_decisions` | `SELECT` | `SELECT` |
| `transport.shipment_status_history` | `INSERT`, `SELECT` | `INSERT`, `SELECT` |
| `transport.shipment_cargo_execution_evidence` | `INSERT`, `SELECT` | `INSERT`, `SELECT` |
| `transport.delivery_attempt_facts` | `INSERT`, `SELECT` | `INSERT`, `SELECT` |
| `transport.delivery_disposition_cases` | `INSERT`, `SELECT`, `UPDATE` | those three |
| `transport.delivery_disposition_commands` | `INSERT`, `SELECT`, `UPDATE` | those three |
| `transport.delivery_disposition_audit` | `INSERT` | `INSERT` |
| `transport.delivery_disposition_evidence` | `INSERT` | `INSERT` |
| `transport.driver_device` | `INSERT`, `SELECT`, `UPDATE` | those three |
| `transport.driver_notification_delivery` | `INSERT`, `SELECT`, `UPDATE` | those three |
| `transport.driver_operation_idempotency` | `INSERT`, `SELECT` | those two |
| `transport.driver_reported_delay` | `INSERT`, `SELECT` | those two |
| `transport.driver_reported_exception` | `INSERT`, `SELECT` | those two |
| `transport.driver_task` | `INSERT`, `SELECT`, `UPDATE` | those three |
| `transport.driver_task_response` | `INSERT`, `SELECT` | those two |

A staging probe still rolls an insert back. If a serial or identity default then returns `42501` on `nextval`, that sequence gets `USAGE` and `SELECT`. The shipment statements above use UUID keys. The only explicit runtime `nextval` in the inventory is `freight_cost.charge_code_mapping_version_seq`, which belongs to freight-cost-service.

These privileges stay revoked from `shipment_runtime`:

- `INSERT`, `UPDATE`, and `DELETE` on `transport.transport_execution_carrier_relationship_pins`;
- `INSERT`, `UPDATE`, and `DELETE` on `company.carrier_relationship_activation_authorizations`;
- `INSERT`, `UPDATE`, and `DELETE` on `company.carrier_relationship_activation_subjects`;
- `DELETE` on `transport.transport_execution_revisions`;
- `UPDATE` of `pin_set_fingerprint`.

### 4.3 Security definer and direct DML

Direct DML is the grant for every shipment gap in section 4.2 except the award-link lock. Those statements are the service's own writes. Wrapping shipment create, driver assignment, or delivery disposition in a definer would let the function owner bypass the column limit on revisions.

The award-link statement only locks `rfx.rfx_award_transport_orders`. A table `UPDATE` grant would also let the shipment login change award rows. The later migration adds one function, owned by `freight_migrations`, with a fixed `search_path`, named `rfx.lock_award_transport_order_for_execution(uuid, uuid)`. It locks the row and returns the columns the repository already reads. `shipment_runtime` receives `EXECUTE`. It does not receive `UPDATE` on that table. This discovery does not create the function.

`transport.transport_orders` is a real column update. The later grant is column-level `UPDATE (status, version, updated_at)` plus `SELECT`. The same probe confirms that `FOR UPDATE` succeeds under that column grant. A `42501` on the lock widens the grant only far enough for the lock to succeed, and the widening is reviewed again before it is applied.

The four activation functions that already exist remain the only definers on the activation path. They write evidence and `pin_set_fingerprint`, which the runtime role must not write directly.

`requireActivationEnforcement` reads `pg_catalog` for `current_user`. It refuses a superuser, `BYPASSRLS`, the role name `freight`, ownership of the four protected relations, or a direct `INSERT`, `UPDATE`, or `DELETE` ACL on the pin and evidence tables. The check looks at `current_user` and direct ACL entries. It does not walk role membership. The login model in section 6 is shaped around that.

## 5. Other services

No privilege role exists yet for these binaries. The minimum later grant is `USAGE` on the service schema, the observed DML in the appendix, and the cross-schema exceptions below. Own-schema tables are not repeated here.

| Service | Later NOLOGIN role | Cross-schema need |
| --- | --- | --- |
| identity-service | `identity_runtime` | `INSERT`, `SELECT`, `UPDATE` on `core.users`. `SELECT`, `INSERT`, `DELETE` on `core.user_roles`. `SELECT` on `core.companies`, `core.company_memberships`, `core.roles`, `core.permissions`, `core.role_permissions`, `core.tenants`. Statements are separate `pool.Exec` calls, with no explicit transaction |
| transport-order-service | `transport_order_runtime` | `SELECT` on `core.companies`. Own writes are `transport.transport_orders`, `transport.cargoes`, `transport.cargo_items`, `transport.locations`, rate snapshots, and create idempotency |
| rfx-service | `rfx_runtime` | `SELECT` on `core.companies`, `core.company_memberships`, `core.users`, `core.user_roles`, `core.roles`. `SELECT` on `transport.locations`. `INSERT` on `transport.cargoes` from award conversion. No `UPDATE` or `DELETE` on cargoes |
| document-service | `document_runtime` | `SELECT` on `core.companies` and `core.users`. `SELECT` on `transport.shipments`. Writes stay in `documents`, including POD evidence and signatures |
| billing-register-service | `billing_runtime` | `SELECT` on `core.companies`, `core.company_memberships`, `core.roles`, `core.user_roles`, `documents.documents`, `rfx.rfx_award_transport_orders`, `transport.shipments`, `transport.transport_orders`, `transport.transport_order_rate_snapshots`. Writes stay in `billing` |
| payment-service | `payment_runtime` | `SELECT` on `billing.billing_registers`, `billing.billing_register_items`, `billing.freight_settlements`, and the same core authorization tables as billing. Writes are `billing.payments`, `billing.payment_allocations`, `billing.payment_obligations`, `billing.payment_audit_events`, `billing.payment_outbox` |
| low-code-service | `low_code_runtime` | writes and reads stay in `lowcode` |
| control-tower-read-model-service | `control_tower_runtime` | `SELECT` on `core.users` and `transport.shipments`. Writes stay in `control_tower`. Inserts into `control_tower.operational_case_event` and `control_tower.critical_event_action` omit `id`, so the role needs `USAGE` and `SELECT` on `control_tower.operational_case_event_id_seq` and `control_tower.critical_event_action_id_seq` |
| tracking-service | `tracking_runtime` | `SELECT` on `transport.shipments`. `INSERT` on `transport.shipment_event_outbox`. No `UPDATE` or `DELETE` on that outbox |
| network-optimizer-service | `network_optimizer_runtime` | writes and reads stay in `network_optimizer`. Route-plan activation calls shipment-service. It does not write shipment tables |
| freight-cost-service | `freight_cost_runtime` | writes and reads stay in `freight_cost`. `USAGE` and `SELECT` on `freight_cost.charge_code_mapping_version_seq` |
| contract-rate-service | `contract_rate_runtime` | `SELECT` on `core.companies`, `core.company_memberships`, `core.user_roles`, `core.roles`, `core.permissions`, `core.role_permissions`, and `transport.locations`. Writes stay in `contract_rate` |

Shared writes that must remain shared:

| Relation | Roles that write it | Boundary |
| --- | --- | --- |
| `core.user_roles` | `identity_runtime`, `company_runtime` | `SELECT`, `INSERT`, `DELETE` for both |
| `transport.cargoes` | `transport_order_runtime` inserts, `rfx_runtime` inserts, `shipment_runtime` selects and updates | no cargo `UPDATE` for RFx |
| `transport.transport_orders` | `transport_order_runtime` inserts and updates; `shipment_runtime` updates status | column-level for shipment |
| `transport.shipment_event_outbox` | `shipment_runtime` already has full DML; `tracking_runtime` inserts | tracking gets `INSERT` only |
| `billing` payment tables | `payment_runtime` writes them; `billing_runtime` reads settlements and registers | payment does not gain invoice or act writes |

Demoting `freight` while any of these binaries still connect as `freight` is an outage. EDO continues to need document-service and the shipment reads of `documents.documents`. Finance continues to need billing-register-service, payment-service, and freight-cost-service. Control Tower continues to need its projection writes and the two sequence grants. Those three stay on the current login until their own roles exist and their pools have been moved.

## 6. Login, NOLOGIN, and ownership

Three kinds of role stay distinct.

| Kind | Roles | Password | Who connects |
| --- | --- | --- | --- |
| NOLOGIN privilege | `company_runtime`, `shipment_runtime`, `freight_migrations`, and the roles named in section 5 | none | nobody |
| LOGIN service | one login per process, created at cutover outside git. Suggested names are `company_login` and `shipment_login`, and the same `_login` suffix for the other services. The name `freight` is reserved and is not reused | secret store only | that service's pool |
| LOGIN migration | one login whose name is not `freight` | secret store only, break-glass network | migrate and the gate installer |

`000103` already creates `freight_migrations`, `company_runtime`, and `shipment_runtime` as `NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS`. The new privilege roles use that same attribute set. They are not created by this change.

`freight_migrations` remains the owner of the activation functions, the activation composite type, the two evidence tables, and the pin table. It is not the migrate login and it is not granted to a service login.

The service login is created `LOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS`. It is a member of exactly one privilege role: `GRANT company_runtime TO company_login`. `NOINHERIT` on the privilege role does not block that direction. The login inherits the privilege role's grants, so the pool does not call `SET ROLE`. `current_user` is the login name. For shipment that passes `requireActivationEnforcement` when the login is not named `freight`, is not a superuser, does not bypass row security, does not own the four protected relations, and is not a direct grantee of evidence or pin DML. The evidence grants stay on `shipment_runtime`, which does not have them, so the member does not have them either.

Forbidden membership, in either direction:

- a service login and `freight`;
- a service login and `freight_migrations`;
- a service login and any other service's privilege role;
- `GRANT company_runtime TO freight` or `GRANT shipment_runtime TO freight`.

CI may keep granting a runtime role to the test bootstrap user and calling `SET ROLE`. That proves the privilege role. It is not the production pool shape, and it is not copied into a deployment secret.

Ownership of `transport.transport_execution_revisions` stays with the migration login until shipment-service no longer connects as that owner. `000103` does not change that owner. In the compose bootstrap the owner name is `freight`. The later owner is `freight_migrations`, in the same window as the shipment login cutover. Doing it earlier, while shipment-service still connects as the owner, removes the writer's access.

`000103` and `000104` set no `ALTER DEFAULT PRIVILEGES`. A table created afterwards is reachable by its owner and not by the runtime roles. The later migration login sets default privileges per schema to that schema's privilege role only. It does not set a default grant to `PUBLIC` or to `freight`. Until those defaults exist, every new table still needs an explicit grant migration.

## 7. Authentication and pg_hba

The repository has no `pg_hba.conf`. This change does not add one. The policy a later operator applies is:

- host connections use `scram-sha-256`;
- `trust` is not a host method;
- each application network has a line for its own service login and for no other login;
- the migration login is reachable from the migration network only;
- the name `freight` has no line from an application network;
- a change is a reviewed reload of the previous file's replacement, after a read of the live file.

Secrets live in the secret store. A document, a compose default, or a CI variable is not a production password. Connection strings are not copied into this discovery.

The pool keeps using one URL. Moving a service is a change of that URL to the service login, a rolling restart, and a check that `current_user` is the new login. There is no fallback URL pointing at `freight`.

## 8. Credential rotation and rollback

PostgreSQL stores one password on a role. Overlap uses two login roles that are both members of the same privilege role, for example `shipment_login` and `shipment_login_next`.

1. Create the next login outside git, with the attributes in section 6, and grant the existing privilege role to it.
2. Store its password in the secret store.
3. Point the service secret at the next login and roll the process.
4. Confirm `current_user`, a representative read, and a representative write.
5. After that confirmation, set the previous login to `NOLOGIN` or revoke its `CONNECT`. Keep the privilege role.

Rollback of a bad rotation is the previous secret and a restart. The privilege role and its grants stay. The previous login is still present because step 5 has not run. A `42501` from a missing grant is a failed cutover: restore the previous secret. It is not a reason to grant the login a superuser attribute or to grant the privilege role to `freight`.

Rollback of a service login does not drop `000103` or `000104`, does not drop pins, and does not install or remove `trg_new_activation_authorization`. Gate rollback remains the `DROP TRIGGER` described in ADR-PLATFORM-004, and only after that trigger exists. It does not exist on this base.

`pg_hba` rollback is a reload of the last reviewed file by the operator who changed it.

## 9. Enforcement-gate prerequisites

`infrastructure/postgres/tms_activation_enforcement_gate_v0.1.sql` is not applied by migrate. Its first statement raises `42501` while a role named `freight` is a superuser, has `BYPASSRLS`, owns any of the four protected relations, or has direct write on the pin or evidence tables. Installing it in front of a writer that still connects as `freight` stops new activations at `COMMIT`, including a `Project` with the flag left false.

| Prerequisite | State on this base |
| --- | --- |
| Shipment runtime coverage for the whole binary | Open. Section 4.2 is the missing grant list. `000104` covers `Project` and `CreateSuccessorRevision` only |
| Company runtime coverage for the whole binary | Open for administration. Relationship Create, Accept, Cancel, and Revoke match `company_runtime` |
| Authorizer is protected | Met in code. The authorizer is `SECURITY DEFINER`, owned by `freight_migrations`, and the runtime role cannot write evidence directly |
| Direct DML on pins and evidence is denied | Met in `000103` and repeated in `000104`. Triggers from `000103` already reject mutation of evidence, subjects, and pins |
| `Project` writes the authorization in the same transaction | Met on `main` by PR #297. `TMS_RELATIONSHIP_ACTIVATION_ENABLED` defaults to false |
| Successor revision still propagates pins | Met by `transport.propagate_execution_relationship_pins` and the `000104` grants. CI runs `TestSuccessorRevision` as the test bootstrap user |
| Shared `freight` bypass is gone | Open. The compose login is still that role. Live attributes were not read |
| Written operator approval for the gate and the flag | Open. This document is not that approval |

The gate stays unapplied until every row above is met, shipment-service is connected as `shipment_login`, and the flag decision is in the same maintenance window. ADR-PLATFORM-004 is the procedure for that window.

## 10. ROLL-01 through ROLL-07

The identifiers are the blockers in the PR #298 discovery. This plan closes none of them in a database.

| Id | What removes the blocker | When |
| --- | --- | --- |
| `ROLL-02` | A reviewed migration grants the three company gaps in section 3. `company_runtime` stays `NOLOGIN`. Production `DATABASE_URL` stays unchanged | First grant migration |
| `ROLL-03` | The same series grants the shipment gaps in section 4.2 and adds the award-link definer. One pool remains. The activation path is not split in code | Same series, still `NOLOGIN` |
| `ROLL-01` | Every consumer in section 2 has a privilege role and a login, and the pools have moved. Only then may `freight` lose superuser, bypass, and application `CONNECT` | After the other services, including EDO, finance, Control Tower, tracking, NLO, and migrate |
| `ROLL-04` | `ALTER TABLE transport.transport_execution_revisions OWNER TO freight_migrations` after shipment-service no longer connects as the owner | Same window as the shipment login move |
| `ROLL-05` | Gate install and the flag decision happen together, after the shipment login passes `requireActivationEnforcement` | The maintenance window in ADR-PLATFORM-004 |
| `ROLL-06` | An operator reads live role attributes, the live flag, trigger presence, and `pg_hba`, and records the result outside the secret values | Before the window. Still `NOT_VERIFIED` |
| `ROLL-07` | Staging shows an unlinked route plan when shipment `COMMIT` fails. NLO SQL grants are not part of that proof | Staging scenario. No NLO code change |

Order that must be preserved:

1. Add grants while the roles are `NOLOGIN` and `freight` is untouched.
2. Rehearse on a disposable database with logins created from the secret store.
3. Move service pools, shipment last among the activation pair.
4. Move migrate to a login that is not named `freight`.
5. Change revision ownership.
6. Remove the `freight` bypass.
7. Install the gate and apply the flag decision in one window.

A grant migration does not revoke `freight`. EDO, finance, and Control Tower keep working on the current login until their own cutover.

## 11. Integration test plan

Existing CI connects as the workflow bootstrap user. The official image makes that user a superuser. Security tests then `SET ROLE`. That is not a login-role rehearsal. The later tests keep the `SET ROLE` proof and add a login that has no superuser attribute.

| Scenario | Already in CI | Still required |
| --- | --- | --- |
| Relationship Create, Accept, Cancel, Revoke | `company-relationship-foundation-postgres` runs `TestCompanyRelationshipFoundationPostgres`. `tms-postgres-security-foundation` repeats Cancel, Accept, and Revoke as `company_runtime` | Run company administration and those four commands as `company_login`, with the administrative grants present and evidence DML still denied |
| Public accept | Inside the foundation test | Same login |
| Shipment create and carrier, driver, and vehicle assignment | `tms-activation-runtime-postgres` runs foundation and projection tests as the bootstrap user | Run create and assignment as `shipment_login` |
| `Project` and successor | `TestActivationRuntimePins` and `TestSuccessorRevision` | Repeat both as `shipment_login`, including one pinned successor |
| Source authority | `shipment-source-authority-postgres` and `rfx-source-authority-postgres` | Confirm the `SELECT` grants on membership and role tables under the service login |
| Direct SQL bypass and immutable pins | `TestPostgresSecurityFoundation` | Repeat pin and evidence DML as the service login and expect `42501` or the immutability error |
| Revoke during activation | `TestActivationRuntimePins` | Repeat under `company_login` and `shipment_login` |
| EDO | `execution-tracking-pod-integration` runs document POD upload, tracking, and control-tower execution tests as the bootstrap user | Repeat document POD, shipment document read, and tracking outbox insert as `document_login`, `shipment_login`, and `tracking_login` |
| Finance | `freight-settlement-integration`, `freight-billing-closing-integration`, `freight-payments-core-integration`, `freight-cost-ledger-integration` | Repeat as `billing_login`, `payment_login`, and `freight_cost_login` |
| Control Tower | The execution-tracking job covers one slice. The read-model binary's case, workflow, and projection statements have no role-coverage job | Run projection rebuild, case event insert, and critical-event insert as `control_tower_login`, including the two sequences |
| RFx award conversion | `rfx-award-to-transport-order-integration` | Confirm `INSERT` on `transport.cargoes` as `rfx_login` and the absence of cargo `UPDATE` |
| NLO handoff | NLO persistence jobs store `EXECUTION_LINKED`. They do not assert that a failed shipment `COMMIT` leaves the route plan unlinked | `ROLL-07` staging scenario |
| Low-code | No PostgreSQL integration job, and low-code-service is outside the `backend-go-check` matrix | Add a role-coverage test before `low_code_login` is used |
| Identity | No dedicated PostgreSQL job | User insert and role assign or delete as `identity_login` |
| Connection-pool recovery | Absent | Drop the connection and confirm the new backend's `current_user` is the service login |
| Credential rotation | Absent | Rehearse the two-login overlap on a disposable database |
| Fail closed | The security foundation expects denial under `SET ROLE` | A login missing one grant receives `42501` and the pool does not reconnect as `freight` |

## 12. Closed authorizations

```text
DOCS_ONLY=YES
PRODUCT_CODE_CHANGED=NO
MIGRATION_CREATED=NO
NLO_RUNTIME_CHANGED=NO
CONTROL_TOWER_RUNTIME_CHANGED=NO
CREDENTIAL_CREATED=NO
LOGIN_ATTRIBUTE_CHANGED=NO
PG_HBA_CHANGED=NO
ENFORCEMENT_GATE_INSTALLED=NO
FEATURE_FLAG_ENABLED=NO
FREIGHT_DEMOTED=NO
DEPLOYMENT_EXECUTED=NO
DOCKER_VOLUME_CHANGED=NO
BACKUP_RESTORED=NO
READY_FOR_MERGE=NO
```
