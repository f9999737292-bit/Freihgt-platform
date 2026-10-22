# ADR-PLATFORM-004: TMS activation rollout separates logins before the enforcement gate

## Status

Proposed. Discovery and rollout procedure only. No migration file, no credential, no `pg_hba` change, no enforcement-gate install, no feature-flag enablement, and no deployment is authorized. This decision does not replace ADR-PLATFORM-003.

```text
ADR_NUMBER=ADR-PLATFORM-004
ADR_PLATFORM_001_RESERVED_BY=PR_291_PUBLIC_ACCEPT
ADR_PLATFORM_002_RESERVED_BY=PR_292_RELATIONSHIP_REVOKE
ADR_PLATFORM_003_RESERVED_BY=PR_295_ACTIVATION_PINS
ADR_NUMBER_COLLISION=NO
DOCS_ONLY=YES
RUNTIME_AUTHORIZED=NO
MIGRATION_NUMBER_ASSIGNED=NO
MIGRATION_CREATED=NO
FEATURE_FLAG_ENABLE_AUTHORIZED=NO
ENFORCEMENT_GATE_INSTALL_AUTHORIZED=NO
DEPLOYMENT_AUTHORIZED=NO
STAGING_CHANGES_AUTHORIZED=NO
SHARED_LOGIN_DEMOTE_AUTHORIZED=NO
FLAG_BYPASSES_DB_ENFORCEMENT=NO
APP_ROLLBACK_INCOMPATIBLE_AFTER=trg_new_activation_authorization
ROLLBACK_OF_000103=FORBIDDEN_AS_ROLLOUT_ROLLBACK
```

`ADR-PLATFORM-003` remains the pin and authorization decision. On `00bc72d6a6b3bbd70cb813ef9c0fe975f2efcebb` the platform ADR directory contains `ADR-PLATFORM-001`, `ADR-PLATFORM-002`, and `ADR-PLATFORM-003`. This decision uses `ADR-PLATFORM-004`.

## Context

PR #295, PR #296, and PR #297 are in `main`. Migration head on that commit is `000104_tms_activation_runtime_grants_v0_1`, not `000103`. `000103` creates `freight_migrations`, `company_runtime`, and `shipment_runtime` as `NOLOGIN` roles and does not create `trg_new_activation_authorization`. `000104` grants the tables `Project` and `CreateSuccessorRevision` write. It does not create a password, a `LOGIN` attribute, or the trigger, and it does not enable `TMS_RELATIONSHIP_ACTIVATION_ENABLED`.

`infrastructure/postgres/tms_activation_enforcement_gate_v0.1.sql` is outside the numbered migration chain. Its first statement, `transport.assert_activation_enforcement_role_split()`, raises `42501` while the role named `freight` is a superuser, has `BYPASSRLS`, owns any of the four protected relations, or holds `INSERT`, `UPDATE`, or `DELETE` on the pin or evidence tables. The local compose file creates that shared login as the PostgreSQL bootstrap user and points identity, company, transport-order, RFx, shipment, document, billing-register, payment, low-code, Control Tower, and the migrate tool at it. The repository contains no `pg_hba.conf`. Live server state was not queried.

The shipment-service flag defaults to false in code. While false, `Project` still inserts a headerless revision. While true, `Project` writes the authorization header, pins, audit, and outbox in the same transaction, and it refuses unless the deferred constraint trigger on `transport.transport_execution_revisions` matches the catalog identity fixed by PR #297. The flag does not disable the trigger. A headerless insert fails at `COMMIT` once the trigger exists.

`company_runtime` can run relationship Create, Accept, Cancel, and Revoke. It cannot write `core.companies`, memberships, or user roles. `shipment_runtime` can run `Project` and `CreateSuccessorRevision`. It cannot insert shipments or vehicles, cannot read the source-authority catalogs, and cannot write delivery-disposition tables. Both services today use one pool.

## Decision

The shared `freight` login is a deployment blocker for the enforcement gate. Removing it is a platform change, not a company-service or shipment-service setting. Those two services move to their own credentials only after a later reviewed grant or a separate pool covers every statement the binary executes. Until that review, production `DATABASE_URL` stays on the current login.

`company_runtime` and `shipment_runtime` stay `NOLOGIN` until an authorized cutover. Passwords are created in a secret store and are never committed. Neither runtime role is granted to `freight`. `freight_migrations` stays `NOLOGIN` and remains the owner of the security-definer functions.

The gate script stays unapplied until all of the following are true in the target database:

- migrations through `000104` are the schema head;
- the running shipment-service binary is the PR #297 `Project`, which can write authorization evidence;
- `freight` fails none of the three checks in `transport.assert_activation_enforcement_role_split()`;
- `transport.transport_execution_revisions` is not owned by `freight`;
- the installer is not the application login and is not a role named `freight`;
- activation traffic is stopped, or the flag is enabled in the same maintenance window as the trigger.

Enabling `TMS_RELATIONSHIP_ACTIVATION_ENABLED` is a separate authorization after that window. It is not implied by installing the trigger, and installing the trigger is not implied by enabling the flag. Either half alone stops new activations.

After `trg_new_activation_authorization` exists, a shipment-service binary that commits a headerless new activation is incompatible with the database. That includes the PR #297 binary with the flag left false and any older `Project`. Restoring compatibility requires a confirmed `DROP TRIGGER`. It does not require migration `000103` down. `000103` down drops evidence, pins, and the runtime roles. It is not the rollout rollback. Rows already committed with pins stay in place. Their data rollback is a backup restore.

`CreateSuccessorRevision` of a legacy headerless revision stays valid after the trigger exists, because the successor row keeps `supersedes_revision_id` and the previous revision is marked `SUPERSEDED` without updating the trigger columns. A successor of a pinned revision copies pins through `transport.propagate_execution_relationship_pins` in the same transaction.

NLO still links `EXECUTION_LINKED` only after the shipment-service acknowledgement. A `COMMIT` failure must leave the route plan unlinked. This decision does not change NLO.

## Consequences

Staging can rehearse the catalog checks, the role probes, and the negative gate install. Staging cannot receive login passwords, `pg_hba` edits, the trigger, or the flag under this decision.

The first production cutover that installs the trigger is a maintenance window. Preparing secrets and running read-only catalog checks are not. Demoting `freight` while other services still depend on its superuser or owner rights is an outage and is outside this decision.

A later implementation stage may add the missing grants or a second pool. That stage assigns its own migration number. This ADR does not reserve one.

## Confirmation

```text
GATE_SCRIPT=infrastructure/postgres/tms_activation_enforcement_gate_v0.1.sql
GATE_IS_NUMBERED_MIGRATION=NO
TRIGGER=trg_new_activation_authorization
TRIGGER_RELATION=transport.transport_execution_revisions
ASSERT=transport.assert_activation_enforcement_role_split
ASSERT_ERRCODE=42501
HEADERLESS_COMMIT_ERRCODE=23514
RUNTIME_ROLES=freight_migrations,company_runtime,shipment_runtime
RUNTIME_ROLES_LOGIN=NO
RUNTIME_ROLES_GRANTED_TO_FREIGHT=FORBIDDEN
FLAG=TMS_RELATIONSHIP_ACTIVATION_ENABLED
FLAG_DEFAULT_IN_CODE=false
FLAG_DISABLES_TRIGGER=NO
MIGRATION_HEAD_ON_BASE=000104
BRIEF_MIGRATION_HEAD=000103
MIGRATION_HEAD_COMPATIBLE=YES
LIVE_GATE_STATE=NOT_VERIFIED
LIVE_FLAG_STATE=NOT_VERIFIED
LIVE_PG_HBA=NOT_VERIFIED
```
