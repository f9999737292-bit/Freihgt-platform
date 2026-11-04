# BINTRANS PLATFORM / TMS

## ACTIVATION ROLLOUT SECURITY 0.1 DISCOVERY

```text
AGENT=A
STAGE=PLATFORM-TMS-ACTIVATION-ROLLOUT-SECURITY-0.1-DISCOVERY
REPOSITORY=f9999737292-bit/Freihgt-platform
BASE_SHA=00bc72d6a6b3bbd70cb813ef9c0fe975f2efcebb
BRANCH=docs/tms-activation-rollout-security-v0.1
WORKTREE=D:/Projects/freight-platform-wt/tms-activation-rollout-security-v0.1
ADR=docs/architecture/platform/adr/ADR-PLATFORM-004-tms-activation-rollout-security.md
ADR_NUMBER=ADR-PLATFORM-004
DOCS_ONLY=YES
PRODUCT_CODE_CHANGED=NO
MIGRATION_CREATED=NO
MERGE_AUTHORIZED=NO
DEPLOYMENT_AUTHORIZED=NO
STAGING_CHANGES_AUTHORIZED=NO
FEATURE_FLAG_ENABLE_AUTHORIZED=NO
ENFORCEMENT_GATE_INSTALL_AUTHORIZED=NO
```

This document is the rollout plan for the TMS activation runtime that PR #297 merged. It does not deploy that runtime, install the enforcement gate, or enable the feature flag. ADR-PLATFORM-004 records the decision. ADR-PLATFORM-003 remains the pin and authorization architecture.

## 1. Preflight

`origin/main` at discovery time is `00bc72d6a6b3bbd70cb813ef9c0fe975f2efcebb`, the merge of PR #297. That SHA matches the expected base. The worktree was created from it and was clean. No other worktree was changed.

| Predecessor | Result on this base |
| --- | --- |
| PR #295 activation-pin architecture | Ancestor. Merge `e53905d1`. ADR-PLATFORM-003 is present. |
| PR #296 PostgreSQL security foundation | Ancestor. Merge `58ec61bf`. Migration `000103` is present. |
| PR #297 activation runtime | This merge. Runtime head `98c2d109`. Migration `000104` is present. |

The stage brief asked to confirm migration head `000103`. The repository head on the expected base is `000104`. There are 104 up migrations, and `000104_tms_activation_runtime_grants_v0_1` is the last. That is compatible. `000104` adds the grants `Project` and `CreateSuccessorRevision` need, repeats the evidence-table revokes, and does not create the trigger or enable the flag. A plan that stopped at `000103` would describe a shipment role that cannot run the current `Project`.

`TMS_RELATIONSHIP_ACTIVATION_ENABLED` defaults to false in `services/shipment-service/internal/config/config.go`. The local compose file does not set the variable. The live process environment was not read. Live flag state is `NOT_VERIFIED`.

The enforcement gate is the file `infrastructure/postgres/tms_activation_enforcement_gate_v0.1.sql`. It is not a numbered migration. The comments in `000103` and `000104` say the trigger stays unapplied. The local `freight_postgres` container was not running, and no staging or production catalog was queried. Live trigger state is `NOT_VERIFIED`.

The repository has no `pg_hba.conf`. Live authentication policy is `NOT_VERIFIED`.

## 2. PostgreSQL roles as they exist in source

`000103` creates three cluster-wide roles. Each is `NOLOGIN`, `NOSUPERUSER`, `NOCREATEDB`, `NOCREATEROLE`, `NOINHERIT`, `NOREPLICATION`, and `NOBYPASSRLS`. The migration sets those attributes again if the role already exists. It creates no password. A role comment marks the row only when this migration creates it, so two databases on one cluster can apply the migration without dropping a role the other database still uses.

| Role | Login | Owns | May execute |
| --- | --- | --- | --- |
| `freight_migrations` | No | Security-definer functions, evidence tables, pin table, `company.activation_subject_input` | `transport.assert_activation_enforcement_role_split()` |
| `company_runtime` | No | Nothing in this migration | No activation function |
| `shipment_runtime` | No | Nothing in this migration | Authorizer, pin attach, pin propagate, activation audit, and the enforcement function |

`GRANT USAGE` on schemas `company`, `transport`, and `core` is given to all three. `CREATE` on those schemas is not. No `GRANT role TO freight` appears in `000103` or `000104`. Tests grant a runtime role to the test login so they can `SET ROLE`. Production must not copy that grant. A session that can `SET ROLE shipment_runtime` passes the application check of `current_user`.

The security-definer functions are owned by `freight_migrations`, revoked from `PUBLIC`, and use `search_path = pg_catalog`. Their names are schema-qualified. The functions that write evidence are:

- `company.authorize_carrier_relationship_activation`
- `company.enforce_activation_subject_head`
- `company.reject_activation_evidence_mutation`
- `company.record_activation_audit`
- `transport.attach_activation_pins`
- `transport.propagate_execution_relationship_pins`
- `transport.enforce_new_activation_authorization`
- `transport.enforce_activation_pin_binding`
- `transport.reject_activation_pin_mutation`
- `transport.reject_revision_fingerprint_rewrite`
- `transport.assert_activation_enforcement_role_split`

Triggers installed by `000103` already protect evidence. Subjects check the `ACTIVE` head before insert. Authorization headers, subjects, and pins reject update and delete. Pins must match the revision identity. `pin_set_fingerprint` cannot be rewritten to a different value. Those triggers are present before the enforcement gate. They do not reject a headerless revision.

Table ownership of the older transport and company tables is unchanged. In the compose definition the bootstrap login is the migration login, so those tables are owned by the role named `freight` wherever that bootstrap was used. `000103` moves ownership of only these objects to `freight_migrations`:

- `company.carrier_relationship_activation_authorizations`
- `company.carrier_relationship_activation_subjects`
- `transport.transport_execution_carrier_relationship_pins`
- `company.activation_subject_input`

`transport.transport_execution_revisions` stays with the migration owner. The gate assert treats ownership of that table by `freight` as a bypass.

### 2.1 Shared login

The compose bootstrap user defaults to the name `freight`. Official PostgreSQL images make `POSTGRES_USER` a superuser. The same login name is the `DATABASE_URL` user for identity-service, company-service, transport-order-service, rfx-service, shipment-service, document-service, billing-register-service, payment-service, low-code-service, and control-tower-read-model-service. The migrate profile uses `POSTGRES_USER` as well. Development defaults for that password are already in the compose file. They must not be copied into a new role, and this plan does not repeat them.

A superuser ignores grants. Table ownership also bypasses grants. The enforcement function still runs for a superuser unless the trigger is disabled or `session_replication_role` is `replica`. The application check does not rely on that. `requireActivationEnforcement` reads `pg_catalog` for the current user and refuses when that user is a superuser, has `BYPASSRLS`, is named `freight`, owns a protected relation, or has write privilege on the evidence tables.

Demoting `freight` before every other service has its own grants is an outage. This plan does not demote it.

### 2.2 Company-service credentials

Relationship Create, Accept, Cancel, and Revoke use tables `000103` grants to `company_runtime`: the relationship, version, head, request, request-event, request-head, and the four idempotency tables, plus audit and outbox. The same role may `SELECT` `core.companies`, `core.tenants`, `core.users`, `core.company_memberships`, `core.user_roles`, and `core.roles`. Write on the authorization and pin tables is revoked.

The company-service binary also inserts and updates `core.companies`, memberships, and user roles. `company_runtime` has no write grant on those tables. One `DATABASE_URL` is loaded in `services/company-service/internal/config/config.go`. Pointing that variable at `company_runtime` fails company administration even when relationship commands succeed.

The safe company plan is:

1. Leave production `DATABASE_URL` unchanged.
2. Keep `company_runtime` `NOLOGIN` until a later stage.
3. On an authorized staging rehearsal, enable `LOGIN` for `company_runtime` from a secret that is not committed, and run only Create, Accept, Cancel, and Revoke through that role.
4. Do not grant `company_runtime` to `freight`.
5. A later reviewed change either grants the missing company-administration statements or gives those statements a different role. This discovery does not choose the grant list and does not add a migration.

### 2.3 Shipment-service credentials

`000103` lets `shipment_runtime` insert and update executions, insert revisions, and update the revision columns `Project` and `CreateSuccessorRevision` already set. `DELETE` on revisions is revoked. `000104` adds participants, active shipments, stops, actions, commands, command audit, driver stop tasks, and `transport.shipment_event_outbox`. It also grants `SELECT` and `UPDATE` on shipments, cargoes, drivers, and vehicles, and `SELECT` on locations and `core.companies`. It does not grant `INSERT` on shipments or vehicles. It does not grant the source-authority tables `core.users`, `core.company_memberships`, `core.user_roles`, or `core.roles`. It does not grant delivery-disposition tables, shipment status history, or transport orders.

`pin_set_fingerprint` is absent from the column-level `UPDATE` grant. `transport.propagate_execution_relationship_pins` sets that column as `freight_migrations`. The runtime role must not receive a direct update grant for it.

Shipment-service has one pool, configured from `DATABASE_URL`, and `cmd/server/main.go` applies the flag to that repository. The activation transaction cannot be moved to `shipment_runtime` while the rest of the process stays on `freight` without a code change. That code change is a later stage.

The safe shipment plan is the company plan: no production cutover, no password in git, no grant of `shipment_runtime` to `freight`, and a staging rehearsal limited to `Project` and `CreateSuccessorRevision` after `000104`.

### 2.4 Sequences, authentication, and pg_hba

`000103` and `000104` contain no `GRANT` on sequences and no `ALTER DEFAULT PRIVILEGES`. Relationship and execution identifiers used by these paths are application UUIDs or `gen_random_uuid()` inside security-definer functions. A staging privilege probe must still roll back an insert for any serial or identity column the rehearsed command touches. A permission error on `nextval` is a grant gap, not a reason to make the role a superuser.

`pg_hba` is not in the repository and was not read on a server. The policy this plan requires, when a later authorization allows a change, is:

- host connections use `scram-sha-256`;
- `trust` is not used for host connections;
- application networks authenticate as `company_runtime` or `shipment_runtime`, not as `freight` and not as a superuser;
- the migration and break-glass login is a different role name from `freight`, and application networks have no line for it;
- a change is a confirmed `pg_hba` reload, not a CI step.

No statement in this repository may be applied to a live `pg_hba.conf` under this stage.

## 3. Enforcement gate

The gate file is seventeen lines. It is not applied by `migrate`. The operator runs it by hand, in one session, after the preconditions below.

```text
SELECT transport.assert_activation_enforcement_role_split();
CREATE CONSTRAINT TRIGGER trg_new_activation_authorization
    AFTER INSERT OR DELETE
        OR UPDATE OF id, source_activation_id, supersedes_revision_id, operating_tenant_id
    ON transport.transport_execution_revisions
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW
    EXECUTE FUNCTION transport.enforce_new_activation_authorization();
```

### 3.1 Privileges required to install it

`EXECUTE` on the assert is granted to `freight_migrations` and revoked from `PUBLIC`. Creating the trigger requires ownership of `transport.transport_execution_revisions` or a superuser. The role named `freight` cannot be that installer while it still fails the assert. The installer is a break-glass role with a different name. It is not the company-service or shipment-service login. The script does not itself create the trigger when the assert raises, because the assert is the first statement.

Before the script, a reviewed ownership change must make `freight` stop owning `transport.transport_execution_revisions`. `000103` does not do that. This discovery does not add the `ALTER TABLE`. Doing it while shipment-service still connects as the owner is an outage, so the ownership change belongs in the same maintenance window as the gate, after shipment writes have a role that does not need ownership.

The assert fails closed with `42501` and the message `shared freight login can bypass activation enforcement` when any of these is true:

- `freight` has `rolsuper` or `rolbypassrls`;
- `freight` owns revisions, pins, authorization headers, or authorization subjects;
- `freight` has `INSERT`, `UPDATE`, or `DELETE` on pins, authorization headers, or authorization subjects.

A disabled trigger, a trigger of the same name on another table, or a non-constraint trigger does not satisfy `requireActivationEnforcement`. That function requires one non-internal constraint trigger on `transport.transport_execution_revisions`, named `trg_new_activation_authorization`, deferrable, initially deferred, executing `transport.enforce_new_activation_authorization()` with zero arguments. For a normal session `tgenabled` must be `O` or `A`. For `session_replication_role = replica` it must be `R` or `A`. The PR #297 check replaced a name-only match.

### 3.2 Compatibility with Project and Successor

`Project` with the flag false does not call the authorizer. It inserts a revision with `source_activation_id` set and `supersedes_revision_id` null, then commits. The deferred trigger reads the stored row at `COMMIT`. That shape requires an authorization header written in the same transaction. The flag-false path does not write one. The result is `23514` and `new activation revision has no authorization header`. The flag does not turn the trigger off. Installing the gate in front of today's flag-false `Project` stops new activations.

`Project` with the flag true calls `company.authorize_carrier_relationship_activation` before the execution insert, then `transport.attach_activation_pins` and `company.record_activation_audit` before `COMMIT`. Audit and the relationship outbox row are in that same transaction. A refusal rolls the execution back. This is the only `Project` that can commit a new activation after the trigger exists, and only when `requireActivationEnforcement` also accepts the session.

`CreateSuccessorRevision` marks the previous revision `SUPERSEDED` by updating `status` and `updated_at`. Those columns are not in the trigger's `UPDATE OF` list, so a legacy successor does not queue an authorization check for the old row. The new row carries `supersedes_revision_id`. The trigger's header requirement applies only when `source_activation_id` is set and `supersedes_revision_id` is null. A successor therefore does not need a new authorization header. If the predecessor has pins, `transport.propagate_execution_relationship_pins` copies them in the same transaction. If it has none, the copy returns zero and the successor still commits. An update that clears `supersedes_revision_id` is a new activation and fails closed without a header.

`000103` must already be applied. The assert and the enforcement function are created there. `000104` must already be applied before a `shipment_runtime` rehearsal of `Project`. The gate does not depend on `000104` for its own `CREATE TRIGGER`, and `000104` must not be treated as a substitute for the gate.

### 3.3 Fail closed, rollback, and catalog proof

Wrong pairings fail closed:

- flag true and trigger absent, disabled, or attached to the wrong relation: `Project` returns `RELATIONSHIP_STORE_UNAVAILABLE` and inserts nothing;
- flag true and the session is `freight`, a superuser, the owner of a protected table, or a writer of evidence: the same refusal;
- flag false and trigger present: the insert reaches `COMMIT` and the trigger aborts it;
- direct `INSERT` into pins or evidence by `shipment_runtime` or `company_runtime`: privilege error, because those grants are revoked and the tables are owned by `freight_migrations`.

Rollback of the gate is one confirmed statement, not migration down:

```sql
DROP TRIGGER IF EXISTS trg_new_activation_authorization
    ON transport.transport_execution_revisions;
```

Run it only after activation traffic is stopped or the flag is confirmed false. Dropping the trigger restores the headerless `Project`. It does not delete headers or pins already committed. `000103` down also drops this trigger, and then drops the evidence tables and the three roles. Using it as a rollout rollback destroys activation history and is forbidden.

Read-only proof, before and after any later install:

```sql
SELECT c.relname, r.rolname AS owner
  FROM pg_catalog.pg_class AS c
  JOIN pg_catalog.pg_roles AS r ON r.oid = c.relowner
 WHERE c.oid IN (
        'transport.transport_execution_revisions'::regclass,
        'transport.transport_execution_carrier_relationship_pins'::regclass,
        'company.carrier_relationship_activation_authorizations'::regclass,
        'company.carrier_relationship_activation_subjects'::regclass
 );
SELECT trg.tgname, trg.tgenabled, trg.tgdeferrable, trg.tginitdeferred,
       ns.nspname AS function_schema, fn.proname
  FROM pg_catalog.pg_trigger AS trg
  JOIN pg_catalog.pg_proc AS fn ON fn.oid = trg.tgfoid
  JOIN pg_catalog.pg_namespace AS ns ON ns.oid = fn.pronamespace
 WHERE trg.tgname = 'trg_new_activation_authorization'
   AND NOT trg.tgisinternal;
SELECT rolname, rolcanlogin, rolsuper, rolbypassrls
  FROM pg_catalog.pg_roles
 WHERE rolname IN ('freight', 'freight_migrations', 'company_runtime', 'shipment_runtime');
```

A missing trigger row is the expected result until a later authorization installs the gate. This stage does not run the gate.

## 4. Rollout sequence

| Step | Action | Downtime |
| --- | --- | --- |
| 1 | Backup and recovery preflight. Confirm a restorable backup of `freight_platform` and a rehearsed restore on a disposable server. Record the backup identity. Do not prune volumes. | No user downtime when the existing backup method is online. The restore rehearsal is a separate server. |
| 2 | Schema check. Read `schema_migrations` through `000104`. Confirm `000103` functions exist and `trg_new_activation_authorization` does not. Do not apply migrations in this stage. | No downtime. Applying a missing `000103` later takes a short exclusive lock for `pin_set_fingerprint` and is a different authorization. |
| 3 | Prepare credentials in a secret store for a future `company_runtime` and `shipment_runtime` login. Leave the roles `NOLOGIN` until cutover is authorized. | No downtime. Not automated. |
| 4 | Privilege probe in a transaction that rolls back: Create, Accept, Cancel, Revoke as `company_runtime`; `Project` and `CreateSuccessorRevision` as `shipment_runtime`. Record every `42501`. | No downtime. Staging only. |
| 5 | Switch service connections. Blocked for production. The binaries do not have grants for their full surface, and they have one pool. | Would be a rolling restart only after the grant gap is closed. The first attempt is a maintenance window, not a silent restart. |
| 6 | Repeat Create, Accept, Cancel, and Revoke on staging through the role that will serve them. | Staging only. |
| 7 | Repeat legacy `Project`, flagged `Project`, legacy successor, and pinned successor on staging. | Staging only. |
| 8 | Install the gate in the same maintenance window as a shipment-service that can write evidence. Stop activation traffic, or enable the flag in that same window. Do not install the trigger in front of a flag-false writer. | Maintenance window. |
| 9 | Re-run the catalog proof and the negative checks: shared-login assert, direct pin insert, disabled trigger, wrong-table trigger, headerless commit. | Inside the same window. |
| 10 | Enable `TMS_RELATIONSHIP_ACTIVATION_ENABLED` only under a separate authorization, and only when step 8's pairing is already true. | Maintenance window if activations are in flight. |
| 11 | Watch the signals in section 8. | No further downtime. |
| 12 | On failure, stop activation traffic, drop only the enforcement trigger, confirm the flag is false, and restore service. Use the backup if committed pin data must disappear. | Maintenance window. |

Steps 1, 2, and 4 can be done without a maintenance window. Steps 5 and 8 cannot be combined into a no-downtime rolling restart with the grants that exist today. Step 3 creates no traffic change. Steps 6, 7, and 9 are staging or in-window checks. Step 10 is not part of step 8's authorization.

The point after which the old application cannot commit is the successful `CREATE CONSTRAINT TRIGGER`. From that instant until the trigger is dropped, flag-false `Project` and every pre-297 `Project` fail new activations at `COMMIT`. Successor revisions of existing executions remain compatible. NLO must observe the shipment-service error and must not set `EXECUTION_LINKED`.

## 5. Security dependencies

| Dependency | Where it is enforced | Rollout note |
| --- | --- | --- |
| Same-transaction authorization | `Project` calls the authorizer, writes the execution, attaches pins, and records audit before one `COMMIT`. The trigger requires the header `xmin` to belong to that transaction. | True only on the flag-true path. The gate makes the header mandatory for new activations. |
| Immutable pins | `trg_execution_relationship_pin_immutable` and the fingerprint trigger from `000103`. | Already in the migration. Independent of the gate. |
| `ACTIVE` head | `company.enforce_activation_subject_head` before subject insert, and a second read at `COMMIT` inside `transport.enforce_new_activation_authorization`. | A head that moves to non-`ACTIVE`, or whose `current_version_id` changes, raises `RELATIONSHIP_REVOKED` and rolls the activation back. |
| Revoke during activation | The authorizer locks the relationship before the execution insert. The deferred trigger reads the head again at `COMMIT`. | Covered by `TestActivationRuntimePins` and `TestPostgresSecurityFoundation`. Staging repeats it under the runtime roles. |
| Digest replay | Unique `(operating_tenant_id, source_activation_id)` and `contract_sha256`. The authorizer returns `REPLAY` when the digest matches and `ACTIVATION_BODY_CONFLICT` when it does not. | Replay does not insert a second header. A conflict inserts nothing. |
| Role isolation | Grants and revokes in `000103` and `000104`. Application refusal when `current_user` is `freight`. | Incomplete for the rest of each binary. See the blockers. |
| Direct SQL bypass | Runtime roles have no write grant on pins or evidence. Owners and superusers still can write. The assert refuses to install the gate while `freight` is in that set. | Demotion of `freight` is a platform-wide change. |
| Legacy `Project` | Flag false, no header, no pins. | Compatible only while the trigger is absent. |
| Legacy successor | Status update plus a row that supersedes. No header required. | Compatible after the trigger exists. |
| Pinned successor | `propagate_execution_relationship_pins` copies the predecessor pin set. | Requires `000103` functions. Fails if the copy does not match the historical set. |
| Audit and outbox | `company.record_activation_audit` inserts `company.carrier_relationship_audit` and the `carrier_relationship.activation_authorized` outbox row in the activation transaction. | A commit failure leaves neither the execution nor the outbox row. |
| NLO handoff | `network-optimizer-service` calls `POST /internal/v1/transport-executions/from-route-plan-activation`, then links `EXECUTION_LINKED` in a later transaction. | This plan does not change NLO. A failed `COMMIT` must not link. |
| Tenant isolation | Authorizer and pin foreign keys bind operating tenant, carrier, shipment tenant, and source company. Shipment commands return `TENANT_DENIED` for a foreign operating tenant. | Staging uses two tenants. The gate does not add a new tenant predicate. |

## 6. Deployment runbook

This runbook is the procedure a later authorization would follow. This stage stops at the document.

1. Confirm `origin/main` still contains PR #295, PR #296, and PR #297, and that the deployed shipment-service commit contains `requireActivationEnforcement`.
2. Confirm backup identity and a restore rehearsal.
3. Confirm schema version `000104` and the absence of `trg_new_activation_authorization` with the catalog queries in section 3.3.
4. Confirm `TMS_RELATIONSHIP_ACTIVATION_ENABLED` is unset or false on the running shipment-service. Do not print the environment file.
5. Confirm no application secret is named for `company_runtime` or `shipment_runtime` yet, or that any prepared secret is outside git.
6. On staging, apply nothing new. Rehearse steps 4 through 9 of section 4 against a copy.
7. Stop when a blocker in section 11 is open.
8. For the later window only: quiesce route-plan activation, move the activation writer to a session that passes `requireActivationEnforcement`, install the gate as the break-glass role, run section 9, then restart traffic. Enable the flag only if that separate authorization is written down in the change ticket.
9. Keep the break-glass login out of service environment variables.

## 7. Rollback runbook

1. Stop route-plan activation so NLO does not retry into a failing `COMMIT`.
2. Set `TMS_RELATIONSHIP_ACTIVATION_ENABLED` to false and restart shipment-service. This step alone does not restore commits while the trigger exists.
3. As the break-glass role, run the `DROP TRIGGER` in section 3.3. Confirm the catalog query returns no row.
4. Confirm a flag-false `Project` commits and writes no authorization header.
5. Leave `000103` and `000104` in place. Do not run their down scripts to undo a bad gate install.
6. If a pinned activation committed and must be removed, restore the rehearsed backup onto a new server and cut over by the backup runbook. Do not delete pin rows with ad hoc SQL. The immutability triggers reject that delete.
7. Do not restore `freight` to `SUPERUSER` as a shortcut. That re-opens grant bypass for every service using the shared login. It does not disable the trigger.
8. Do not edit `pg_hba` during rollback unless the failed change was itself a `pg_hba` reload. Reload back to the previous file only from the change ticket.

## 8. Observability

Watch these signals for at least one activation cycle after any later window. Do not log connection strings, passwords, or tokens.

| Signal | Healthy | Failure |
| --- | --- | --- |
| `POST /internal/v1/transport-executions/from-route-plan-activation` | `200` for a new plan and for a digest replay | `409 ACTIVATION_BODY_CONFLICT`, `RELATIONSHIP_STORE_UNAVAILABLE`, or `5xx` |
| PostgreSQL `23514` | Absent during flag-true traffic | `new activation revision has no authorization header`, `RELATIONSHIP_REVOKED`, pin-set mismatch, carrier mismatch |
| PostgreSQL `42501` | Absent for the runtime roles on their granted tables | `permission denied` or `shared freight login can bypass activation enforcement` |
| Trigger catalog | One row, `tgenabled` `O` or `A`, after an authorized install | Zero rows after install, or `D` |
| Outbox `carrier_relationship.activation_authorized` | One row per new flagged activation, same transaction | Execution committed without the outbox row, or outbox row without an execution |
| NLO `EXECUTION_LINKED` | Follows shipment-service `200` | Linked after a shipment error, or stuck after a `200` |
| Flag | Matches the change ticket | True while the trigger is absent, or false while the trigger is present and traffic continues |

## 9. Feature-flag matrix

| Flag | Trigger on `transport.transport_execution_revisions` | New activation | Successor of a legacy revision | Successor of a pinned revision |
| --- | --- | --- | --- | --- |
| False | Absent | Headerless commit. Current intended state. | Commits, no pins. | Not expected. Propagate copies zero pins when the predecessor has none. |
| True | Absent, disabled, or wrong identity | Refuses before insert. | Successor path does not consult the flag. | Propagate runs when the function exists. |
| False | Present and enabled | `COMMIT` fails closed. Forbidden pairing. | Commits. | Copies pins. |
| True | Present, correct identity, session is `shipment_runtime` and is not a bypass | Authorized commit with header, pins, audit, and outbox. | Commits. | Copies pins in the same transaction. |
| True | Present, session is `freight` or a superuser | Refuses before insert. | Commits if that session still has table rights. | Copies pins only when the session can execute propagate. |

The flag never disables the trigger.

## 10. Permissions matrix

| Object | `freight` today | `freight_migrations` | `company_runtime` | `shipment_runtime` |
| --- | --- | --- | --- | --- |
| Bootstrap login | `LOGIN`, superuser in the image default | `NOLOGIN` | `NOLOGIN` | `NOLOGIN` |
| Evidence and pin tables | Owner until `000103`, then no write grant | Owner | Write revoked | Write revoked |
| Revisions | Owner until a later ownership change | `SELECT`, `UPDATE` | No grant | `SELECT`, `INSERT`, listed-column `UPDATE`, no `DELETE` |
| Relationship command tables | Owner and superuser bypass | `SELECT` or `INSERT` as granted for definer functions | `SELECT`, `INSERT`, `UPDATE` | No grant |
| `core.companies` | Owner bypass | No broad grant | `SELECT` | `SELECT` |
| Membership and role tables | Owner bypass | No grant | `SELECT` only | No grant |
| Activation functions | Superuser bypass | Owner | Execute revoked | Execute on authorizer, attach, propagate, audit |
| Assert function | Superuser can execute | `EXECUTE` | Revoked | Revoked |
| Enforcement trigger | Not installed by migration | Function owner | Cannot create it | Cannot create it |

A cell that says superuser bypass is the reason the gate assert exists. It is not a privilege this plan preserves.

## 11. Go / no-go

All of these must be true before a later stage asks to install the gate. None of them is true merely because this document merged.

- Backup restore has been rehearsed, and the backup identity is recorded.
- Schema head is `000104` on the target database.
- Catalog proof shows no `trg_new_activation_authorization`.
- The shipment-service image is the PR #297 binary or a descendant that still contains the catalog identity check.
- The flag is false, and the change ticket keeps it false until a separate approval.
- `freight` is still the shared login, and nobody has demoted it as a side effect.
- Staging privilege probe for Create, Accept, Cancel, Revoke, legacy `Project`, and legacy successor has a written result.
- The grant gap for the rest of company-service and shipment-service is either closed by a reviewed migration or explicitly left on the old login.
- `pg_hba` for the target server has been read by an operator and matches section 2.4, or the cutover does not enable `LOGIN` yet.
- NLO is unchanged and its link step is still after the HTTP acknowledgement.
- This discovery PR is merged only as documentation. Merge of the document does not authorize the window.

No-go if any catalog check cannot be run, if the trigger is already present, if the flag is already true, or if the only way to make the assert pass is to `ALTER ROLE freight NOSUPERUSER` while other services still use that login.

## 12. Staging acceptance

Run these on a disposable database. Do not point them at production. Do not leave `LOGIN` passwords in the repository.

| Check | Existing CI | Staging addition |
| --- | --- | --- |
| Security foundation, including role split, direct pin denial, headerless denial, and gate refusal while `freight` is superuser | `tms-postgres-security-foundation` runs `TestPostgresSecurityFoundation` and fails on `SKIP` | Repeat the catalog proof against the staging schema version. |
| Multi-database role down | `TestPostgresSecurityFoundationMultiDatabaseDown` is in the same package. The CI `-run` filter `TestPostgresSecurityFoundation` includes it. | Not a staging data test. |
| Activation runtime, legacy project, disabled and decoy trigger, headerless commit, direct SQL, revoke race, tenant mismatch | `tms-activation-runtime-postgres` runs `TestActivationRuntimePins` | Run once as `shipment_runtime`, not only as the CI superuser that `SET ROLE`s. |
| Execution foundation and projection API | Same job runs `TestTransportExecutionFoundation` and `TestExecutionProjectionAPI` | Confirm they still pass after `000104` on staging. |
| `CreateSuccessorRevision` | Same job runs `TestSuccessorRevision` | Add one pinned successor and one legacy successor after the gate is installed on the disposable database. |
| Accept, Cancel, Revoke | `company-relationship-foundation-postgres` runs `TestCompanyRelationshipFoundationPostgres`, which accepts, cancels, recovers, and revokes. `TestPostgresSecurityFoundation` repeats Cancel, Accept, and Revoke as `company_runtime`. | Repeat those four commands as `company_runtime` on staging. |
| Public accept | Covered inside the foundation test through `runPublicAcceptScenarios`. `TestPublicAcceptPostgres` is not a separate CI job name. | Include the public-accept cases in the staging script. |
| Relationship create from shipment | `shipment-source-authority-postgres` runs `TestShipmentRelationshipCreatePostgres` | Confirm source-authority reads still use a role that can see memberships. `shipment_runtime` cannot. |
| Disabled or wrong trigger | `TestActivationRuntimePins` covers the disabled trigger and a same-name trigger on another table. | Required again after a real gate install on staging. |
| Tenant isolation | Activation runtime and source-authority jobs. | Two-tenant staging fixture. |
| NLO handoff | NLO PostgreSQL tests cover `EXECUTION_LINKED` storage. They do not fail a shipment `COMMIT` and assert that the route plan stays unlinked. | Add this as a staging scenario. No NLO code change in this stage. |
| `pg_hba` and live role attributes | Not in CI. CI uses a fresh PostgreSQL service. | Operator read of `pg_hba` and `pg_roles`. Result stays `NOT_VERIFIED` until that read exists. |

## 13. Operator actions that stay manual

These actions require a named approval. CI, a migration, or an agent must not perform them because this document exists:

- create or alter a password, or set `LOGIN` on `company_runtime`, `shipment_runtime`, or `freight_migrations`;
- `GRANT` either runtime role to `freight`;
- `ALTER ROLE freight` with `NOSUPERUSER`, `NOBYPASSRLS`, or `NOLOGIN`;
- edit or reload `pg_hba.conf`;
- `ALTER TABLE ... OWNER` on revisions or evidence;
- apply `000103`, `000104`, or any later migration to staging or production;
- run `tms_activation_enforcement_gate_v0.1.sql`;
- `DROP TRIGGER trg_new_activation_authorization`;
- set `TMS_RELATIONSHIP_ACTIVATION_ENABLED` to true;
- restore a backup over a database that has newer pin rows;
- run `000103` down or `000104` down as a rollback;
- delete or prune Docker volumes.

## 14. Deployment blockers

| Id | Blocker | Why it blocks |
| --- | --- | --- |
| `ROLL-01` | `freight` is the shared superuser login for every compose service and for migrate. | The gate assert refuses to install while that role can bypass grants. Demotion is a platform outage until the other services have grants. |
| `ROLL-02` | `company_runtime` cannot write companies, memberships, or user roles. | Switching company-service `DATABASE_URL` breaks administration. |
| `ROLL-03` | `shipment_runtime` cannot serve the rest of shipment-service, and the process has one pool. | Switching shipment-service `DATABASE_URL` breaks shipment create, source authority, and delivery disposition. The activation path cannot be split without a later code change. |
| `ROLL-04` | Revisions remain owned by the migration role, which is `freight` in the compose bootstrap. | The assert treats that ownership as a bypass. The owner change is not in the gate script. |
| `ROLL-05` | The gate and the flag are different authorizations, and each half alone stops new activations. | They need one maintenance window plus a written flag approval. |
| `ROLL-06` | Live flag, live trigger, and live `pg_hba` are `NOT_VERIFIED`. | A window cannot start from repository evidence alone. |
| `ROLL-07` | NLO link-after-failure is not an automated CI assertion. | Staging must show an unlinked route plan when shipment `COMMIT` fails. |

## 15. Authorization

```text
DOCS_ONLY=YES
PRODUCT_CODE_CHANGED=NO
MIGRATION_CREATED=NO
NLO_RUNTIME_CHANGED=NO
CONTROL_TOWER_CHECKOUT_CHANGED=NO
DEPLOYMENT_EXECUTED=NO
STAGING_CHANGED=NO
FEATURE_FLAG_ENABLED=NO
ENFORCEMENT_GATE_INSTALLED=NO
CREDENTIALS_CREATED=NO
PG_HBA_CHANGED=NO
READY_FOR_MERGE=NO
```

Merge of the documentation pull request requires a separate review. It still does not authorize deployment.
