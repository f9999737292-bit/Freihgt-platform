# Platform award-lock caller identity 0.1

Docs-only contract for the missing database caller identity behind `ROLL-03`. This document does not create a function, a migration, a grant, a login, or an enforcement gate.

```text
AGENT=A
STAGE=PLATFORM-AWARD-LOCK-CALLER-IDENTITY-0.1
BASE_MAIN_SHA=2a99b615a237b6d1f6e6038ca86a7433c567dc78
WORKTREE=D:/Projects/freight-platform-wt/platform-award-lock-identity-v0.1
BRANCH=docs/platform-award-lock-caller-identity-v0.1
DOCS_ONLY=YES
PRODUCT_CODE_CHANGED=NO
MIGRATION_CHANGED=NO
MIGRATION_HEAD_ON_BASE=000105_platform_postgres_role_foundation_v0_2_i1
RFX_LOCK_FUNCTION=NOT_CREATED
ROLL_02=NOT_VERIFIED
ROLL_03=BLOCKED
ROLL_06=NOT_VERIFIED
ROLL_07=NOT_VERIFIED
EXECUTE_AWARD_ORDER_AS_SHIPMENT_LOGIN=BLOCKED
SECURITY_DEFINER_FROM_CALLER_UUIDS=REJECTED
SHARED_LOGIN_CHANGED=NO
SHARED_LOGIN_AUTHORITY=NOT_VERIFIED
ENFORCEMENT_GATE_INSTALLED=NO
ENFORCEMENT_GATE_AUTHORIZED=NO
TMS_RELATIONSHIP_ACTIVATION_ENABLED=NO
PRODUCTION_CUTOVER_AUTHORIZED=NO
DATABASE_URL_CHANGED=NO
IMPLEMENTATION_AUTHORIZED=NO
NEXT_STAGE_AUTHORIZED=NO
```

## Sources

Read on `2a99b615a237b6d1f6e6038ca86a7433c567dc78`:

| Source | What it establishes |
| --- | --- |
| `infrastructure/migrations/000105_platform_postgres_role_foundation_v0_2_i1.up.sql` | `shipment_runtime` receives `SELECT` on `rfx.rfx_award_transport_orders`. `INSERT`, `UPDATE`, and `DELETE` on that table are revoked. The file comment says the award-link lock function is not created. The same file does not create a login role, does not alter the shared application login, does not install `trg_new_activation_authorization`, and does not enable `TMS_RELATIONSHIP_ACTIVATION_ENABLED`. |
| `docs/architecture/platform/PLATFORM_POSTGRES_ROLE_FOUNDATION_0_2_I1.md` | `RFX_LOCK_FUNCTION=NOT_CREATED`. `ROLL-03` stays open because `ExecuteAwardOrder` under `shipment_login` fails closed with `42501` and writes nothing. |
| `docs/architecture/platform/PLATFORM_POSTGRES_ROLE_FOUNDATION_0_2_I1_RFX_LOCK_SECURITY_CONTRACT.md` | A `SECURITY DEFINER` owned by `freight_migrations` whose inputs are the two caller UUIDs is not a server-owned caller identity. The later contract must name the server-owned check, the function owner, `search_path`, the execute grant, and the foreign-tenant regression. That file says it is not the implementation. |
| `services/shipment-service/internal/repository/order_execution_repository.go` | `lockAwardLink` runs `SELECT ... FROM rfx.rfx_award_transport_orders WHERE tenant_id = $1 AND transport_order_id = $2 FOR UPDATE`. Both values come from `ExecuteAwardOrder` parameters. |
| `services/shipment-service/internal/integration/transportexecution/role_foundation_login_integration_test.go` | `TestShipmentRuntimeLoginFoundation` connects as `shipment_login`, member of `shipment_runtime` only. `assertExecuteAwardOrderFailClosed` requires SQLSTATE `42501`, order status `DRAFT`, and no shipment row. |
| `services/api-gateway/internal/http/middleware/auth.go` | When authentication is enabled and the JWT `tenant_id` claim is non-empty, the middleware writes that claim to `X-Tenant-ID`. |
| `services/api-gateway/internal/http/router.go` | `AuthWithIntegrationSupport` is router middleware. `POST /api/v1/order-execution/transport-orders/{id}/execute` uses `executionrbac.PolicyExecute`. |
| `services/api-gateway/internal/executionrbac/policies.go` | `CanExecuteOrder` allows `PLATFORM_ADMIN`, `CARRIER_ADMIN`, and `CARRIER_DISPATCHER`. |
| `services/api-gateway/internal/routeauth/guard.go` | The execution guard reads the auth context when auth is enabled, checks the role policy, and proxies the request. This guard does not set a PostgreSQL session attribute. |
| `services/shipment-service/internal/http/handlers/tenant.go` | `resolveVerifiedTenant` reads `X-Tenant-ID`. |
| `services/shipment-service/internal/http/handlers/order_execution_handler.go` | `Execute` passes that tenant and the URL order id into `ExecuteAwardOrder`. |
| `infrastructure/migrations/000103_tms_relationship_pins_security_foundation_v0_1.up.sql` | `company.enforce_activation_subject_head()` is `SECURITY DEFINER` with `SET search_path = pg_catalog`. It checks a stored relationship head. It is not an award-link lock. |

`000105` is the latest numbered product migration on this SHA. This stage does not add `000106`.

## Caller identity that exists

```text
HTTP_TENANT_SOURCE=IMPLEMENTED
HTTP_TENANT_CARRIER=X-Tenant-ID
HTTP_TENANT_WRITER=api-gateway auth middleware from JWT tenant_id when auth is enabled and the claim is non-empty
HTTP_ROLE_CHECK=IMPLEMENTED
HTTP_EXECUTE_ROLES=PLATFORM_ADMIN,CARRIER_ADMIN,CARRIER_DISPATCHER
SHIPMENT_HANDLER_TENANT_READER=resolveVerifiedTenant
SQL_PARAMETERS=tenant_id,transport_order_id from ExecuteAwardOrder arguments
DATABASE_CURRENT_USER=service login
DATABASE_TENANT_GUC=ABSENT
```

`current_setting` uses found in shipment-service are `server_version`, `current_database()`, `session_replication_role`, and one test-only `lock_timeout`. No application tenant setting is read or written on the award-lock path.

An allowed HTTP role is not a database role. `shipment_login` in the integration test has `current_user` and `session_user` equal to `shipment_login`, is not a superuser, does not bypass row-level security, and is a member only of `shipment_runtime`.

## Database privilege

```text
AWARD_TABLE_SELECT=IMPLEMENTED
AWARD_TABLE_INSERT=REVOKED
AWARD_TABLE_UPDATE=REVOKED
AWARD_TABLE_DELETE=REVOKED
SELECT_FOR_UPDATE_AS_SHIPMENT_LOGIN=BLOCKED
DIRECT_UPDATE_AS_SHIPMENT_LOGIN=BLOCKED
SQLSTATE=42501
```

`TestShipmentRuntimeLoginFoundation` also expects `SELECT ... FOR UPDATE` and a direct `UPDATE` on `rfx.rfx_award_transport_orders` to return `42501` for `shipment_login`. The bootstrap connection seeds the award link before the login call. The login call of `ExecuteAwardOrder` returns `42501`. The following `SELECT 1` on that login shows the failed transaction was rolled back. `awardSideEffects` then requires shipment count `0`, order status `DRAFT`, and an unchanged award link.

Other tests in `order_execution_integration_test.go` call the service `ExecuteAwardOrder` on the test environment service. They are not the `shipment_login` proof. The database role of that test pool is `NOT_VERIFIED` here.

## Why the two UUIDs are not the check

`lockAwardLink` binds `tenant_id` and `transport_order_id` as `$1` and `$2`. Those values are chosen by the caller of `ExecuteAwardOrder`. PostgreSQL sees them as arguments. It does not see the JWT, the gateway role, or `X-Tenant-ID`.

A match between `rfx.rfx_award_transport_orders.tenant_id` and `transport.transport_orders.tenant_id` shows that the two rows name the same tenant. It does not show that the session is allowed to act for that tenant. The existing security contract rejects a `SECURITY DEFINER` function owned by `freight_migrations` that locks the award row from those two UUIDs alone, because any session granted `EXECUTE` could name any tenant and any transport order.

Activation definers in `000103` are a different shape. `company.enforce_activation_subject_head()` runs only after the stored relationship head, version, and subject identity match, and the runtime role cannot insert those rows itself. No equivalent stored caller identity was found for the award lock.

```text
SERVER_OWNED_AWARD_CALLER_CHECK=ABSENT
SECURITY_DEFINER_FROM_CALLER_UUIDS=REJECTED
FUNCTION_OWNER_CHOSEN=NO
FUNCTION_CREATED=NO
```

## Required properties of a later function

These properties are obligations for a later authorized stage. They are not SQL in this stage.

| Property | Required state | This stage |
| --- | --- | --- |
| Server-owned check | The check must prove the session may act for the tenant without trusting the two UUID arguments as that proof. | `BLOCKED`. No such stored fact was found on the award path. |
| Function owner | Owner is not chosen. `freight_migrations` ownership of a UUID-only definer stays rejected. | `DEFERRED` |
| `search_path` | A later function must pin `search_path`. The pin used by `company.enforce_activation_subject_head()` is `pg_catalog`. | `DEFERRED` |
| `EXECUTE` grant | A later grant may name `shipment_runtime`. That grant would also be available to members of `shipment_runtime`, to the function owner, and to superusers. An unchanged `DATABASE_URL` does not prove that the shared application login lacks `EXECUTE` on a future `SECURITY DEFINER` function. Isolation of that login requires a separate check that it is not a member of `shipment_runtime`, is not a superuser, and does not own the function. Those attributes are `ROLL-06` and are `NOT_VERIFIED`. Until that check, this contract does not claim the shared application login is isolated from the future function. | `DEFERRED` |
| Foreign regression | A foreign tenant or a foreign transport order locks nothing and writes nothing. | `NOT_IMPLEMENTED` |
| Direct statement | `lockAwardLink` stays `SELECT ... FOR UPDATE` until that later contract exists. | `IMPLEMENTED` |

```text
ALC_001=CLOSED
DATABASE_URL_UNCHANGED_PROVES_NO_EXECUTE=NO
EXECUTE_ON_SHIPMENT_RUNTIME_REACHES=members of shipment_runtime, function owner, superusers
SHARED_LOGIN_MEMBERSHIP_IN_SHIPMENT_RUNTIME=NOT_VERIFIED
SHARED_LOGIN_SUPERUSER=NOT_VERIFIED
SHARED_LOGIN_OWNS_FUTURE_FUNCTION=NOT_VERIFIED
SHARED_LOGIN_AUTHORITY=NOT_VERIFIED
ROLL_06=NOT_VERIFIED
```

`000105` does not change `DATABASE_URL`, does not install the enforcement gate, and does not enable `TMS_RELATIONSHIP_ACTIVATION_ENABLED`. Those file facts are not a privilege proof. A direct `GRANT EXECUTE` to the shared application login remains outside this contract. This stage does not authorize the enforcement gate, does not enable the activation flag, and does not authorize a production cutover.

## Rollout blockers left open

| Id | State on this base | Classification |
| --- | --- | --- |
| `ROLL-03` | Award-link definer is absent. `ExecuteAwardOrder` as `shipment_login` returns `42501` and writes nothing. | `BLOCKED` |
| `ROLL-01` | Other service roles are absent. The shared login is unchanged. | `BLOCKED` |
| `ROLL-04` | Revision ownership is unchanged. | `BLOCKED` |
| `ROLL-05` | Enforcement gate is not installed. The activation flag is not enabled. | `BLOCKED` |
| `ROLL-06` | Live role attributes were not read. | `NOT_VERIFIED` |
| `ROLL-07` | No staging scenario was run. | `NOT_VERIFIED` |
| `ROLL-02` | The source document says the company grants stay open until `000105` is reviewed and merged. The migration file is present as the product head on this SHA. A separate post-merge closure of `ROLL-02` was not re-verified in this stage. | `NOT_VERIFIED` |

`I-05` in the role-foundation document stays open until an administrator adds job `platform-postgres-role-foundation-i1` to required checks. This stage does not change branch protection. Whether that job is currently required is `NOT_VERIFIED`.

## Dependencies

```text
DEPENDS_ON=000105_platform_postgres_role_foundation_v0_2_i1
DEPENDS_ON_CONTRACT=PLATFORM_POSTGRES_ROLE_FOUNDATION_0_2_I1_RFX_LOCK_SECURITY_CONTRACT.md
BLOCKS=production cutover of ExecuteAwardOrder under shipment_login
DOES_NOT_AUTHORIZE=TMS enforcement gate
DOES_NOT_AUTHORIZE=TMS_RELATIONSHIP_ACTIVATION_ENABLED
DOES_NOT_RESUME=PR 298
DOES_NOT_TOUCH=tools/bintrans_autopilot
DOES_NOT_TOUCH=docs/engineering/autopilot
DOES_NOT_TOUCH=infrastructure/migrations
```

Draft PR #298 is the older rollout-security branch. It is not the base of this contract and is not continued here.

Agent C owns the shipment execution behavior. This contract does not change `services/shipment-service`. A later implementation that edits `lockAwardLink` belongs to a new authorized stage after the server-owned check exists.

## Open questions

1. Which already-stored server fact, if any, binds a PostgreSQL session to the gateway tenant for this command? None was found in the award-lock path.
2. Who owns a future lock function if the rejected `freight_migrations` UUID-only definer remains rejected?
3. Does the order-execution integration pool run as the shared application login or as a bootstrap role? `NOT_VERIFIED` in this document.
4. Are live database attributes and a staging scenario still unread (`ROLL-06`, `ROLL-07`)? The role-foundation document says yes. This stage did not open a staging session.

## Recommended next stage

```text
NEXT_RECOMMENDED_STAGE=PLATFORM-AWARD-LOCK-CALLER-IDENTITY-0.2
NEXT_STAGE_KIND=controller decision on the server-owned check
NEXT_STAGE_AUTHORIZED=NO
MIGRATION_AUTHORIZED=NO
FUNCTION_CREATE_AUTHORIZED=NO
SHIPMENT_CODE_CHANGE_AUTHORIZED=NO
```

The next stage may choose the server-owned check and only then name an owner, a pinned `search_path`, an execute grant, and the foreign-tenant regression. A grant to `shipment_runtime` still requires the unread `ROLL-06` attributes before anyone can claim the shared application login cannot call that function. Until that choice exists, `ExecuteAwardOrder` under `shipment_login` stays fail-closed and `lockAwardLink` stays the direct `SELECT ... FOR UPDATE`. This document does not start that next stage.
