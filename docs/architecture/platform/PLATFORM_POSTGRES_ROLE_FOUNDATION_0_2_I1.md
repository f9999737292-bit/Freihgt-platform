# Platform PostgreSQL Role Foundation 0.2 I1

Grant migration for the company and shipment runtime gaps in the accepted role-coverage contract. The roles stay `NOLOGIN`. The shared application login, `pg_hba`, the enforcement gate, and `TMS_RELATIONSHIP_ACTIVATION_ENABLED` are unchanged.

```text
STAGE=PLATFORM-POSTGRES-ROLE-FOUNDATION-0.2-I1
MIGRATION=000105_platform_postgres_role_foundation_v0_2_i1
BASE=deb1255199fa9fad614523bcd920998439b0b984
PREVIOUS_HEAD=000104_tms_activation_runtime_grants_v0_1
LOGIN_ROLES_IN_MIGRATION=NO
FREIGHT_LOGIN_CHANGED=NO
ENFORCEMENT_GATE_INSTALLED=NO
FEATURE_FLAG_ENABLED=NO
RFX_LOCK_FUNCTION=NOT_CREATED
```

## Grants

`company_runtime` receives `INSERT` and column `UPDATE` on `core.companies` and `core.company_memberships`, and `INSERT` and `DELETE` on `core.user_roles`. The company columns are `legal_name`, `short_name`, `legal_name_en`, `legal_name_zh`, `company_type`, `tax_id`, `registration_number`, `country_code`, `preferred_locale`, `status`, `updated_at`, `version`, and `deleted_at`. The membership columns are `position`, `status`, `deleted_at`, `updated_at`, and `version`. `tenant_id`, `company_id`, and `user_id` are not updateable. Those are the statements in `company_repository.go` and `membership_repository.go`. The repository does not update `core.user_roles`, so that privilege is not granted. Relationship Create, Accept, Cancel, and Revoke keep the grants from `000103`. Evidence and pin DML stay revoked.

`shipment_runtime` receives the section 4.2 grants from the role-coverage discovery: shipment, driver, and vehicle inserts; source-authority reads; document and bid reads; column `UPDATE (status, version, updated_at)` plus `SELECT` on `transport.transport_orders`; delivery-disposition writes; and the driver-platform writes the shipment binary runs. `core.users` is `SELECT (id, tenant_id, status, deleted_at)` only. `rfx.rfx_award_transport_orders` receives `SELECT` only. New `UPDATE` grants are column lists. `tenant_id`, `operating_tenant_id`, `shipment_tenant_id`, `driver_id`, and `shipment_id` are not in those lists. `transport.driver_task.id` is included because `driver_task_repository.go` assigns that column in `ON CONFLICT DO UPDATE SET id = transport.driver_task.id`. It is the task primary key, not a tenant, company, or user identity column.

## What a login test proves

The integration tests create `company_login` and `shipment_login` inside the disposable database only. Each login is `INHERIT`, is a member of exactly one runtime role, and is not a superuser. The tests call the repository and relationship service methods on that session. A bootstrap superuser running `SET ROLE` is not the proof. The test roles are dropped with the database session and are not part of the migration.

## Rollout blockers

| Id | State after this change |
| --- | --- |
| `ROLL-02` | The three company grants are in `000105`. The blocker stays open until this migration is reviewed and merged. Production `DATABASE_URL` is unchanged. |
| `ROLL-03` | Open. The shipment DML grants are in `000105`. The award-link definer is not created. `ExecuteAwardOrder` under `shipment_login` fails closed with `42501` and writes nothing. This path is not ready for production cutover. |
| `ROLL-01` | Open. Other service roles do not exist, and the shared login is unchanged. |
| `ROLL-04` | Open. Revision ownership is unchanged. |
| `ROLL-05` | Open. The gate is not installed and the flag is not enabled. |
| `ROLL-06` | Open. Live attributes were not read. |
| `ROLL-07` | Open. No staging scenario was run. |

Tenant isolation remains the application predicate already used by the repositories. This migration does not add row-level security. A hand-written statement from the service login can still name another tenant, which is the same table-level shape the accepted contract describes. The login tests show the repository and relationship methods do not read or update a row under the wrong tenant, and a foreign disposition operating tenant is denied.

The award-link lock is specified in `PLATFORM_POSTGRES_ROLE_FOUNDATION_0_2_I1_RFX_LOCK_SECURITY_CONTRACT.md`.

## Remediation R1

`shipment_login` cannot read `core.users.password_hash`, `email`, `phone`, `mfa_enabled`, or `last_login_at`. `ReadShipmentCreator` for the caller's own tenant still passes on `id`, `tenant_id`, `status`, and `deleted_at`.

Direct `UPDATE` of `tenant_id` under `company_login` and `shipment_login` returns `42501`. Repository updates of the caller's own tenant still pass. Repository reads and updates that name another tenant stay denied.

`ExecuteAwardOrder` is covered by the shipment login integration test: an existing award link is seeded by the bootstrap role, the login call returns `42501`, `transport.shipments` gains no row for that order, the award link and order status stay unchanged, and the login session can run the next statement. `ROLL-03` stays open.

The login tests open a new connection as `company_login` or `shipment_login`. `current_user` and `session_user` are that login. `rolsuper` and `rolbypassrls` are false. `pg_has_role` against `freight` is false when that role exists, and the only role membership is the matching runtime role. `freight_migrations` and any superuser or `BYPASSRLS` role are not in the membership. The migration does not create or alter those roles.

Job `platform-postgres-role-foundation-i1` runs PostgreSQL 16 and the two dedicated-login tests. A skipped test or a missing `PASS` line fails the job, and a failing `go test` fails the job. The checks above live inside those two tests. Branch protection is not changed by this remediation. Until an administrator adds that job to the required checks, finding I-05 stays open even when the job succeeds.
