# ADR-PLATFORM-001: Public accept of a carrier relationship request

## Status

Proposed for architecture review. Discovery only. Implementation is not authorized.

```text
AGENT=A
STAGE=PLATFORM-CROSS-TENANT-RELATIONSHIP-PUBLIC-ACCEPT-0.1-DISCOVERY
BASE_SHA=3551b960585f58d7a86c29d3b12fb4677caa6f88
PR290_PRESENT=YES
R1_STAGE=PUBLIC-ACCEPT-0.1-R1-REMEDIATION
R1_FINDINGS=PAR-01,PAR-02,PAR-03,PAR-04
PR292_IN_MAIN=YES
PR292_MERGE=92ad5e01d97748f7895aaa83ff2463894ad4aa87
DOCS_ONLY=YES
IMPLEMENTATION_AUTHORIZED=NO
MIGRATION_AUTHORIZED=NO
GATEWAY_CHANGE_AUTHORIZED=NO
OPENAPI_FILE_CHANGE_AUTHORIZED=NO
DEPLOYMENT_AUTHORIZED=NO
```

## Context

`company-service` already accepts a pending carrier relationship request. `RelationshipService.Accept` is the command. `RelationshipRepository.AcceptRelationshipRequest` is the only writer. The HTTP route is `POST /v1/carrier-operating-relationship-acceptances`, behind `relationshipauth.RequireJWT`.

0.1B froze the public shape in `PLATFORM_CROSS_TENANT_EXECUTION_IDENTITY_0_1B_RUNTIME_DESIGN.md` section 8 and then said that stage does not add the gateway route or the OpenAPI operation. PR #290 added cancel and lost-token recovery. It did not publish accept.

`origin/main` at the base above is that code. The gateway has no accept route. `packages/openapi` has no accept operation. The longest existing prefix `/api/v1/carrier` is owned by `rfx-service`, and that prefix matches the path 0.1B named. Gateway authentication strips `X-Company-ID` and does not put it back. Company-service still requires that header as the selected operating company.

The raw acceptance token is 32 random bytes, returned once on create, and stored only as HMAC-SHA-256 with the company-service pepper. Accept never returns it. Audit and outbox rows do not contain it.

## Decision

Public accept is the existing company-service command, published through the API gateway to an authenticated caller. It is not an anonymous token link.

```text
PUBLIC_ACCEPT_MEANS=GATEWAY_PUBLISHED_AUTHENTICATED_ROUTE
ANONYMOUS_ACCEPT_LINK=NO
ACCEPT_COMMAND_OWNER=company-service.RelationshipService.Accept
ACCEPT_PERSISTENCE_OWNER=company-service.RelationshipRepository.AcceptRelationshipRequest
SECOND_ACCEPT_COMMAND=NO
PUBLIC_ROUTE=POST /api/v1/carrier-operating-relationship-acceptances
UPSTREAM_ROUTE=POST /v1/carrier-operating-relationship-acceptances
IDENTITY_SOURCE=VERIFIED_JWT
TENANT_SOURCE=VERIFIED_JWT
SELECTED_COMPANY_SOURCE=CLIENT_SELECTOR_RESTORED_AFTER_GATEWAY_STRIP
SELECTED_COMPANY_AUTHORITY=SERVER_MEMBERSHIP_AND_ROLE_IN_COMPANY_SERVICE
OPERATING_ACCEPTANCE_PREDICATE=VERIFIED_ACTIVE_USER AND ACTIVE_MEMBERSHIP_IN_SELECTED_OPERATING_COMPANY AND (COMPANY_SCOPED_CARRIER_ADMIN OR OPERATING_TENANT_GLOBAL_PLATFORM_ADMIN)
TOKEN_AUTHORITY=ONE_TIME_HMAC_SHA256
TOKEN_TRANSPORT=JSON_BODY_ONLY
MIGRATION_REQUIRED=NO
```

The operating company is the company the caller selects inside the operating tenant. It is not `shipper_carrier_company_id`. That id is the shipper tenant's carrier record. The relationship root binds the two endpoints and is unique on `(shipper_tenant_id, shipper_carrier_company_id, operating_tenant_id)`.

When that root already exists, the selected company must equal the immutable `operating_carrier_company_id`. A sibling company in the same operating tenant must not consume the token, must not append `ACCEPTED`, and must not insert a version on a `REVOKED` head. The current repository links or reactivates without that comparison. The implementation stage adds the comparison inside the existing transaction, after the relationship advisory lock and before `Revalidate`. A mismatch rolls back without calling the source callback. It does not add a table or a migration.

Two sibling companies racing the first insert both take that lock. Under `READ COMMITTED` the loser sees the committed root and receives the same generic 403. Its request stays `PENDING`, and one root is stored. `RELATIONSHIP_IDEMPOTENCY_CONFLICT` (409) remains the unique-index result only when that lock is not held.

An expired token is observed under the request-head lock before the comparison. The command commits one `EXPIRED` event and returns `RELATIONSHIP_REQUEST_EXPIRED` (409). The request does not stay `PENDING`. That exception applies to a sibling and to the endpoint company.

The company equal to `operating_carrier_company_id` that accepts a new pending request while the head is `REVOKED` inserts the next `ACTIVE` version and leaves the revoked version in place. That insert is test 14 of the revoke contract merged in PR #292. A sibling receives 403, the head stays `REVOKED`, and no version row is rewritten.

Gateway publication must use a route prefix longer than `/api/v1/carrier`, restore the captured `X-Company-ID` only as a selector, forward the bearer token, and stay off the anonymous route list. Company-service keeps ignoring `X-User-ID`, `X-Tenant-ID`, `X-User-Roles`, and `X-Platform-Admin`.

The binding HTTP contract, access matrix, transaction order, tests, and implementation sequence are in `docs/architecture/platform/PLATFORM_CROSS_TENANT_RELATIONSHIP_PUBLIC_ACCEPT_0_1_DISCOVERY.md`.

## Consequences

- Callers need a user JWT and an operating-company selector. Possession of the acceptance token is not enough.
- Create, cancel, status, and get stay off this public operation.
- `COMPANY_RELATIONSHIP_COMMANDS_ENABLED` stays default off. Publishing the route does not turn the command on.
- Replay of the same tenant idempotency key and the same selected company returns the stored acceptance and does not return the token.
- A sibling operating company with a still-valid token receives a generic 403, and that request stays `PENDING`. An expired token is the exception: one `EXPIRED` event and 409 `RELATIONSHIP_REQUEST_EXPIRED`.
- The matching operating company, and only that company, inserts the next `ACTIVE` version after `REVOKED`. The locked first-insert race uses the same 403. The unique-index 409 applies only when the relationship advisory lock is not held.
- No schema change is required for this decision. The acceptance idempotency primary key stays `(actor_tenant_id, idempotency_key)`.

## Non-goals

This ADR does not authorize runtime code, a migration, a gateway edit, an OpenAPI file edit, a revoke command, TMS activation, or deployment. The merged revoke contract remains the owner of the revoke command. This decision only binds a later accept of a new pending request to that contract's reactivation rule.
