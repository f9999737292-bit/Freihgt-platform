# DB identity inventory — trusted authority boundary 0.1P

Status: discovery. No role, grant, migration, or product code is changed here.

Base: `origin/main` `3f306fc407aeda4bdc652d825954292f4a54fad6`. Reference draft PR #385 remains at `bc820e9239b9977215a3bbf80d220d9151faa427` and is not modified. Credential values are omitted. A present credential is classified `PRESENT`.

## Question

Which check can stop the writer from inserting an authority ticket for another tenant, or from consuming a ticket that belongs to another presenter? The check has to be visible to PostgreSQL. A tenant argument, a custom session variable, and the shared `freight` login are not that check.

## What the database session is

| Fact | Evidence |
| --- | --- |
| Compose creates the database user from `POSTGRES_USER`, default `freight`. The password setting is `PRESENT`. | `infrastructure/docker-compose/docker-compose.yml:18-19` |
| `000103` says `freight` stays the shared login until a later role split. It does not `ALTER ROLE freight`. | `infrastructure/migrations/000103_tms_relationship_pins_security_foundation_v0_1.up.sql:6-8` |
| `000104` calls that login a deployment blocker. | `infrastructure/migrations/000104_tms_activation_runtime_grants_v0_1.up.sql:6-8` |
| `000105` does not alter the shared login and does not create a `LOGIN` role. | `infrastructure/migrations/000105_platform_postgres_role_foundation_v0_2_i1.up.sql:3-5` |
| `000105` leaves the award-link lock function uncreated until a server-owned caller identity exists. | same file `:8-9` |
| Shipment-service opens one pool from `DATABASE_URL`. An empty variable falls back to a DSN whose user is `freight`. The credential in that fallback is `PRESENT`. | `services/shipment-service/internal/config/config.go:73-75`, `cmd/server/main.go:36` |
| The shared pool helper parses that URL and applies pool limits. It has no `AfterConnect` and no `SET ROLE`. | `packages/shared-go/database/postgres.go:12-21`, `:38-54` |
| Default pool size is 25 connections. | `packages/shared-go/database/pool_config.go:20` |
| Tracking-service opens its own pool the same way. Its fallback user is also `freight`. The credential is `PRESENT`. Tracking Go does not name `transport.transport_executions`. | `services/tracking-service/internal/config/config.go:87`, `internal/platform/database/postgres.go:14-19` |

These compose services set a database URL whose user is `freight`. Each credential is `PRESENT` and is not copied here.

| Service | Line |
| --- | --- |
| identity-service | `docker-compose.yml:40` |
| company-service | `:66` |
| transport-order-service | `:90` |
| rfx-service | `:116` |
| shipment-service | `:143` |
| document-service | `:170` |
| billing-register-service | `:194` |
| payment-service | `:219` |
| low-code-service | `:244` |
| control-tower-read-model-service (`CONTROL_TOWER_DATABASE_URL`) | `:274` |

On this path `current_user` is the login in the URL. That is one principal for every service in the table.

## Roles that exist

| Role | Attributes in the migration | Membership |
| --- | --- | --- |
| `freight_migrations` | `NOLOGIN NOSUPERUSER NOINHERIT NOBYPASSRLS` | none in a numbered migration |
| `company_runtime` | same | none in a numbered migration |
| `shipment_runtime` | same | none in a numbered migration |

Creation and the confirming `ALTER ROLE` are `000103:17-48`. `NOLOGIN` means `shipment_runtime` cannot open a pool. `NOINHERIT` means a member does not receive its privileges until `SET ROLE`. The production pool never does that.

An integration test grants `shipment_runtime` to `freight` and then runs `SET ROLE shipment_runtime` on a test pool. That membership is not a migration. `services/shipment-service/internal/integration/transportexecution/activation_runtime_integration_test.go:607` and `:637`.

`shipment_runtime` holds table-level `SELECT, INSERT, UPDATE` on `transport.transport_executions` (`000103:1225`) and `SELECT, INSERT, UPDATE` on `transport.transport_execution_active_shipments` (`000104:18`). Those grants are unused by a session that stays `freight` and does not `SET ROLE`, except where `freight` is owner or superuser.

## Owner and superuser

`000088` creates `transport.transport_execution_active_shipments` at `:76` and does not change its owner. No numbered migration found here assigns that owner. `ACTIVE_LINK_TABLE_OWNER=NOT_PROVEN`.

The official image makes `POSTGRES_USER` a superuser. This compose file does not revoke that. `000103` and `000105` do not `ALTER ROLE freight` either. Whether a live server still has that attribute is `NOT_PROVEN`. A superuser, or the table owner, is not fenced by `GRANT` and `REVOKE` aimed at `shipment_runtime`.

`transport.assert_activation_enforcement_role_split` (`000103:1088-1135`) rejects a superuser, bypassrls, or pin-table owner named `freight`. It does not name the active-link table. The activation probe in shipment-service reads `current_user` and refuses the activation store when that user is `freight` (`transport_execution_activation.go:123-156`). That probe is not the ticket boundary, and the enforcement trigger it looks for is not installed by `000103` or `000105`.

## SECURITY DEFINER that exists

Pin and activation functions are `SECURITY DEFINER` with `search_path = pg_catalog`. `000103:1184-1198` revokes `EXECUTE` from `PUBLIC`. `000103:1207-1213` grants the runtime functions to `shipment_runtime`. `000103:1215` grants the assert function to `freight_migrations`. None of these functions insert an authority ticket. `freight_migrations` owns the pin function (`000103:1150`) and is not reused as a ticket owner.

## Identity the database does not see

Identity-service puts `tenant_id` in the access token and the user id in `sub`. `services/identity-service/internal/platform/security/jwt.go:11-16` and `:37-42`.

The gateway strips client identity headers, parses that token, and stores the claims in `AuthContext`. `services/api-gateway/internal/http/middleware/auth.go:29-66`. `AuthContext` is `UserID`, `TenantID`, `Email`, `AuthToken`, `RequestedCompanyID` (`auth_context.go:14-20`). A second middleware does the same after rejecting integration tokens (`integration_auth.go:220-249`).

When auth is enabled, route policy reads that context, not the raw header (`routeauth/context.go:20-30`), then asks identity for roles (`routeauth/guard.go:44-46`). The identity call forwards the verified tenant and user (`routeauth/client.go:70-78`).

`IntegrationVerifyHandler.VerifyBearer` checks an API-key bearer and returns `principal_id` plus `tenant_id` (`integration_verify_handler.go:41-76`). That is a different credential from the human access token. It is not attached to the shipment database session.

Shipment-service detail reads take `X-Tenant-ID` as the tenant (`handlers/tenant.go:13-16`). `RequireJWT` verifies a bearer and does not read those headers (`userauth/jwt.go:17-19`). Neither result is sent to PostgreSQL. A SQL function cannot see `AuthContext`.

## Designed roles that do not exist

`binding_authority_issuer`, `binding_authority_caller`, and `binding_authority_owner` are not in `000103`, `000104`, or `000105`. Describing them here does not make them a boundary.

## What breaks if `freight` stops being the login

- Every service in the compose table fails its pool connect until it has another login.
- `shipment_runtime` and `company_runtime` cannot replace it. They are `NOLOGIN`.
- Objects owned by `freight` block `DROP ROLE` until ownership moves. That owner is `NOT_PROVEN` for the active-link table.
- Tests and local defaults that use this login stop connecting. Those defaults are not a production identity.
- Revoking grants from `shipment_runtime` does not change a session whose `current_user` is `freight`.
