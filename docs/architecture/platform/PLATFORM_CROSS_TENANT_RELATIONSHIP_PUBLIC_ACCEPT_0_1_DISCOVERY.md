# BINTRANS PLATFORM / IDENTITY

## CROSS-TENANT RELATIONSHIP PUBLIC ACCEPT 0.1 DISCOVERY

```text
AGENT=A
STAGE=PLATFORM-CROSS-TENANT-RELATIONSHIP-PUBLIC-ACCEPT-0.1-DISCOVERY
BASE_SHA=3551b960585f58d7a86c29d3b12fb4677caa6f88
PR290_MERGE=3551b960
BRANCH=docs/bintrans-relationship-public-accept-v0.1
R1_STAGE=PUBLIC-ACCEPT-0.1-R1-REMEDIATION
R1_FINDINGS=PAR-01,PAR-02,PAR-03,PAR-04
PR292_IN_MAIN=YES
PR292_MERGE=92ad5e01d97748f7895aaa83ff2463894ad4aa87
ORIGIN_MAIN_AT_R1=92ad5e01d97748f7895aaa83ff2463894ad4aa87
DOCS_ONLY=YES
IMPLEMENTATION_AUTHORIZED=NO
MIGRATION_AUTHORIZED=NO
MIGRATION_REQUIRED=NO
GATEWAY_CHANGE_AUTHORIZED=NO
OPENAPI_FILE_CHANGE_AUTHORIZED=NO
DEPLOYMENT_AUTHORIZED=NO
ADR=docs/architecture/platform/adr/ADR-PLATFORM-001-public-relationship-accept.md
```

This document binds the public accept contract to the command that is already on `main`. It does not add a route, an OpenAPI file change, a migration, or runtime code.

Authoritative inputs read at `3551b960585f58d7a86c29d3b12fb4677caa6f88`:

- `docs/architecture/platform/PLATFORM_CROSS_TENANT_EXECUTION_IDENTITY_0_1B_RUNTIME_DESIGN.md` sections 8, 10, and 12
- `docs/architecture/platform/PLATFORM_CROSS_TENANT_EXECUTION_IDENTITY_0_1A_FREEZE.md`
- `docs/architecture/platform/PLATFORM_CROSS_TENANT_RELATIONSHIP_CANCEL_RECOVERY_0_1_DISCOVERY.md`
- `infrastructure/migrations/000100_company_carrier_relationship_foundation_v0_1c.up.sql`
- `services/company-service` accept command, JWT gate, and persistence
- `services/api-gateway` route table and identity-header strip
- `packages/openapi/openapi.yaml` company paths

R1 also reads the revoke contract merged by PR #292 at `92ad5e01d97748f7895aaa83ff2463894ad4aa87`:

- `docs/architecture/platform/PLATFORM_CROSS_TENANT_RELATIONSHIP_REVOKE_0_1_ARCHITECTURE.md`
- `docs/architecture/platform/adr/ADR-PLATFORM-002-relationship-revoke.md`

That merge is on `origin/main`. It is not the base of this pull request. The files in this pull request do not overlap that merge.

PR #290 is the merge commit at the base SHA. Cancel and lost-ACK recovery are in the base. Accept was already present before that merge. PR #292 adds the revoke command contract and leaves `ensureActiveRelationship` as the reactivation path.

## 1. Command owner

`RelationshipService.Accept` owns the decision. `RelationshipRepository.AcceptRelationshipRequest` is the only method that writes an `ACCEPTED` request event, a relationship root, a relationship version, an acceptance idempotency row, the accept audit row, and `carrier_relationship.accepted`.

The company-service route is already:

```text
POST /v1/carrier-operating-relationship-acceptances
MIDDLEWARE=relationshipauth.RequireJWT
HANDLER=RelationshipHandler.Accept
```

Create, status, and cancel are registered outside that JWT group and use the dedicated service credential. They are not this public operation. Get is JWT-gated and is not this public operation. Shipment-service and rfx-service clients call create, status, and cancel. They do not call accept.

```text
PUBLIC_ACCEPT_COMMAND=EXISTING_COMPANY_SERVICE_ACCEPT
NEW_ACCEPT_SERVICE=NO
NEW_ACCEPT_TABLE=NO
```

`COMPANY_RELATIONSHIP_COMMANDS_ENABLED` is true only for the values `true` and `1`. Any other value, including empty, returns `RELATIONSHIP_COMMANDS_DISABLED` (403) before a write. An empty `RELATIONSHIP_TOKEN_PEPPER` returns `RELATIONSHIP_STORE_UNAVAILABLE` (503) before the token is looked up.

## 2. One-time token

`NewAcceptanceToken` reads 32 bytes from `crypto/rand`. `EncodeAcceptanceToken` uses base64 raw URL encoding. `DecodeAcceptanceToken` accepts only that encoding and only 32 bytes. `FingerprintAcceptanceToken` is HMAC-SHA-256 with the company-service pepper as the key. The column `acceptance_token_hash` is 32 bytes, unique, and not reversible.

Create returns `acceptance_token` only when `PersistCreateResult.Created` is true. Replay of create omits it. The accept response fields are `relationship_id`, `relationship_version`, and `request_status`. The accept response has no token field. The foundation integration test replays accept without presenting the token and still receives the stored relationship.

Lookup compares the stored hash with `hmac.Equal`. A missing row also runs `hmac.Equal` against a zero digest so the not-found path is not an obvious early return. A missing token, a token that does not decode, an unknown hash, and an optional `relationship_request_id` that does not equal the row found by the hash all return `RELATIONSHIP_REQUEST_NOT_FOUND` (404). Those responses do not say which of the four cases failed.

Token lifetime is the database constraint `token_expires_at = token_created_at + interval '72 hours'`. There is no expiry sweeper. Expiry is written only when a command observes `token_expires_at <= clock_timestamp()` under the request-head lock.

```text
TOKEN_BYTES=32
TOKEN_ENCODING=BASE64_RAW_URL
TOKEN_STORAGE=HMAC_SHA256_PEPPER
TOKEN_COLUMN=acceptance_token_hash
TOKEN_RETURNED_ON_ACCEPT=NO
TOKEN_RETURNED_ON_ACCEPT_REPLAY=NO
TOKEN_QUERY_STRING=NOT_ACCEPTED
IDENTICAL_404=MISSING_BAD_UNKNOWN_AND_LOCATOR_MISMATCH
```

## 3. Operating tenant and operating company

The operating tenant is the `tenant_id` claim of the verified user JWT. The operating company is `X-Company-ID` after it has been checked against that tenant.

`LoadRelationshipActor` reads `core.users` for the JWT user id and JWT tenant id, `deleted_at IS NULL`, and `status = 'ACTIVE'`. No row leaves `ActorAuthority.Active` false. Membership is an active `core.company_memberships` row for that user, that tenant, and the selected company, joined to an active company in the same tenant. Company roles are `core.user_roles` rows with that `company_id`. Tenant-global `PLATFORM_ADMIN` is a role row with `company_id IS NULL` and `roles.code = 'PLATFORM_ADMIN'`.

`CanAcceptRelationship` requires `Active`, `ActiveMembership`, and either tenant-global `PLATFORM_ADMIN` or company-scoped `CARRIER_ADMIN` on the selected company. `PLATFORM_ADMIN` does not replace the active user or the active membership. A company-scoped `PLATFORM_ADMIN` row is not the tenant-global override. `SHIPPER_ADMIN`, `SHIPPER_LOGIST`, `FORWARDER_MANAGER`, `PROCUREMENT_MANAGER`, and an ordinary member do not accept.

The first accept inserts `company.carrier_relationships` with:

- `shipper_tenant_id` and `shipper_carrier_company_id` from the request
- `operating_tenant_id` from the JWT
- `operating_carrier_company_id` from the selected company

`company.enforce_relationship_endpoint_tenants` rejects an operating company that is missing, deleted, inactive, or outside the operating tenant. The root is insert-only. Unique key: `(shipper_tenant_id, shipper_carrier_company_id, operating_tenant_id)`. The operating company is not part of that key, and it cannot be updated later.

```text
OPERATING_TENANT_SOURCE=JWT
OPERATING_COMPANY_SOURCE=SELECTED_COMPANY_AFTER_MEMBERSHIP_CHECK
SHIPPER_CARRIER_COMPANY_IS_OPERATING_COMPANY=NO
ROOT_BUSINESS_KEY=shipper_tenant_id+shipper_carrier_company_id+operating_tenant_id
ROOT_OPERATING_COMPANY_IMMUTABLE=YES
```

### Endpoint match

0.1B reuses one root for later sources in the same business key. The current `ensureActiveRelationship` does that without comparing the selected company to `operating_carrier_company_id`. A `CARRIER_ADMIN` of a sibling company in the same operating tenant can therefore consume a new pending token and attach an `ACCEPTED` event to a root whose endpoint is the other company. The same path reactivates a `REVOKED` head.

This contract keeps the business key and adds one predicate. The predicate runs after the relationship advisory lock in section 6 and before `Revalidate`. A mismatch ends the transaction there. The source callback is not called.

```text
OPERATING_ENDPOINT_MATCH=REQUIRED_WHEN_ROOT_EXISTS
STORED_COLUMN=company.carrier_relationships.operating_carrier_company_id
MISMATCH_HTTP=403
MISMATCH_CODE=RELATIONSHIP_ACCEPTANCE_FORBIDDEN
MISMATCH_BODY=GENERIC
MISMATCH_FOREIGN_COMPANY_ID=ABSENT
MISMATCH_TOKEN_CONSUMED=NO
MISMATCH_EVENT=NONE
MISMATCH_APPLIES_TO=ACTIVE_AND_REVOKED
MISMATCH_BEFORE_REVALIDATE=YES
MISMATCH_CALLS_REVALIDATE=NO
MISMATCH_REQUEST_STATUS_WHEN_TOKEN_VALID=PENDING
EXPIRED_TOKEN_BEFORE_MATCH=ONE_EXPIRED_EVENT
FIRST_INSERT_RACE_WITH_RELATIONSHIP_LOCK=403_PENDING_ONE_ROOT
UNIQUE_VIOLATION_WITHOUT_RELATIONSHIP_LOCK=409_RELATIONSHIP_IDEMPOTENCY_CONFLICT
REVOKED_MATCHING_COMPANY=INSERT_NEXT_ACTIVE_VERSION
REVOKED_SIBLING=403_HEAD_STAYS_REVOKED
```

No root means the selected company becomes the endpoint, subject to the membership predicate and the tenant trigger. A matching company links to the current `ACTIVE` version. When the head is `REVOKED`, the company equal to `operating_carrier_company_id` inserts the next `ACTIVE` version on the same root and leaves the `REVOKED` version in place. That insert is the reactivation already required by the merged revoke contract, section 1 and test 14. A sibling does not insert that version. The head stays `REVOKED`, and no version row is rewritten.

A mismatch of a still-valid token rolls the transaction back. The request stays `PENDING`. The endpoint company can still accept. This 403 is the only sibling result for a valid token, including the first-insert race below.

Expiry is the exception to that `PENDING` result. Section 6 step 6 observes an expired token under the request-head lock, before the endpoint comparison. It appends one `EXPIRED` event, commits, and returns `RELATIONSHIP_REQUEST_EXPIRED` (409). The request does not stay `PENDING`. The same outcome applies to the endpoint company and to a sibling. A head that is already `EXPIRED` returns the same code and does not append another event.

Two sibling companies, one business key, and no root yet both take the section 6 lock `relationship:{source_tenant_id}:{shipper_carrier_company_id}:{actor_tenant_id}` before insert. Under `READ COMMITTED` the loser runs after the winner commits, sees the root, and takes the mismatch path: HTTP 403 `RELATIONSHIP_ACCEPTANCE_FORBIDDEN`, its own request stays `PENDING`, and the database contains one root. `RELATIONSHIP_IDEMPOTENCY_CONFLICT` (409) remains the mapping when `uq_cr_business_key` fires without that lock. It is not the result of the locked race. The token is not consumed and the body has no foreign company id in either outcome.

The same-request case is already implemented. A second accept of a request that is already `ACCEPTED`, including from `OperatingB2` in the foundation test, returns `RELATIONSHIP_REQUEST_ALREADY_ACCEPTED` (409) and the error text does not contain the winning company id.

## 4. Access matrix

| Caller | Predicate result | HTTP | Code |
| --- | --- | --- | --- |
| No bearer, bad bearer, or expired JWT | Not authenticated | 401 | `AUTHENTICATION_REQUIRED` |
| JWT user missing, deleted, or not `ACTIVE` | `Active=false` | 403 | `RELATIONSHIP_ACCEPTANCE_FORBIDDEN` |
| Active user, no active membership on the selected company | Membership fails | 403 | `RELATIONSHIP_ACCEPTANCE_FORBIDDEN` |
| Active membership, no `CARRIER_ADMIN` and no tenant-global `PLATFORM_ADMIN` | Role fails | 403 | `RELATIONSHIP_ACCEPTANCE_FORBIDDEN` |
| `CARRIER_ADMIN` on a different company than `X-Company-ID` | Role is not on the selection | 403 | `RELATIONSHIP_ACCEPTANCE_FORBIDDEN` |
| Tenant-global `PLATFORM_ADMIN` without active membership on the selection | Membership still required | 403 | `RELATIONSHIP_ACCEPTANCE_FORBIDDEN` |
| Active user, active membership, `CARRIER_ADMIN` on that company | Accept predicate passes | continues | |
| Active user, active membership, tenant-global `PLATFORM_ADMIN` | Accept predicate passes | continues | |
| Commands flag off | Before the actor load | 403 | `RELATIONSHIP_COMMANDS_DISABLED` |
| Body `tenant_id` present and different from the JWT tenant | Spoof | 409 | `TENANT_CONTEXT_CONFLICT` |
| Body `company_id` present and different from `X-Company-ID` | Spoof | 403 | `RELATIONSHIP_ACCEPTANCE_FORBIDDEN` |
| Missing or unparsable `X-Company-ID` | No selection | 403 | `RELATIONSHIP_ACCEPTANCE_FORBIDDEN` |
| Valid token, request `PENDING`, source matches, endpoint matches or no root | First commit | 201 | body below |
| Valid token, root exists or was just committed by the other sibling, selected company differs | Mismatch before `Revalidate` | 403 | `RELATIONSHIP_ACCEPTANCE_FORBIDDEN` |
| Expired token before the endpoint comparison, sibling or endpoint company | One `EXPIRED` event | 409 | `RELATIONSHIP_REQUEST_EXPIRED` |
| Matching operating company, head `REVOKED`, new pending token, source matches | Next `ACTIVE` version | 201 | body below |
| Same tenant idempotency key, same company, same locator | Replay | 200 | stored ids, no token |

`X-User-ID`, `X-Tenant-ID`, `X-User-Roles`, and `X-Platform-Admin` are not read by `RelationshipHandler.Accept`. The handler tests that set those headers still authenticate from the bearer token. A body `requesting_user_id` is not an accept input. Create uses that field on the internal credential path. Accept does not.

## 5. Header and body spoofing

Company-service accept authority is the JWT plus the server actor query. The body may repeat `tenant_id` and `company_id`. If either is present, it must equal the JWT tenant or the selected company. If it differs, the command refuses before `AcceptRelationshipRequest`. An absent body company is allowed. The body cannot choose a different operating company.

`relationship_request_id` is an optional locator. It is not authority. The token hash selects the request. A locator that does not parse, or that parses to a different id, is the generic 404. The idempotency body hash is SHA-256 of `operating_company_id` and the locator string, so a replay with a different locator or company is `RELATIONSHIP_IDEMPOTENCY_CONFLICT` (409) and does not return the stored relationship.

Gateway authentication, both `Auth` and `AuthWithIntegrationSupport`, copies `X-Company-ID` into `AuthContext.RequestedCompanyID` and then `StripUntrustedIdentityHeaders` deletes `X-Company-ID`, `X-User-ID`, `X-Tenant-ID`, `X-User-Roles`, and `X-Platform-Admin`. It writes `X-User-ID` and `X-Tenant-ID` back from the JWT. It does not write `X-Company-ID` back. Other domains restore the selector inside their own RBAC guards. A new proxy-only route would therefore arrive at company-service without a company selector and fail closed with 403. That is safe and also unusable.

```text
GATEWAY_STRIP_UNTRUSTED_IDENTITY=YES
GATEWAY_RESTORES_X_COMPANY_ID_TODAY=NO
PUBLIC_ACCEPT_GATEWAY_SELECTOR=RESTORE_CAPTURED_X_COMPANY_ID_AFTER_STRIP
GATEWAY_SELECTOR_IS_AUTHORITY=NO
COMPANY_SERVICE_RECHECKS_MEMBERSHIP_AND_ROLE=YES
GATEWAY_DECIDES_CARRIER_ADMIN=NO
AUTHORIZATION_HEADER_FORWARDED=YES
ANONYMOUS_ROUTE_LIST=MUST_NOT_INCLUDE_ACCEPT
```

`isPublicRoute` allows login, health, and OpenAPI downloads. Accept must not be added there. Human gateway auth stays required. The forwarded bearer token is what company-service verifies again.

## 6. Transaction sequence

One company-schema transaction. Lock order stays the 0.1B order. The endpoint comparison runs after the relationship key is locked. When the companies differ, that comparison finishes the transaction before the source callback.

```text
ACCEPTANCE_TRANSACTION_BOUNDARY=ONE_COMPANY_SCHEMA_DATABASE_TRANSACTION
ACCEPTANCE_LOCK_ORDER=IDEMPOTENCY_ADVISORY_THEN_TOKEN_HASH_THEN_REQUEST_HEAD_THEN_RELATIONSHIP_ADVISORY_THEN_ENDPOINT_MATCH_THEN_SOURCE_CALLBACK_ONLY_ON_MATCH
FIRST_INSERT_RACE_WITH_LOCK=403_RELATIONSHIP_ACCEPTANCE_FORBIDDEN
UNIQUE_INDEX_WITHOUT_LOCK=409_RELATIONSHIP_IDEMPOTENCY_CONFLICT
```

1. `pg_advisory_xact_lock` on `accept:{actor_tenant_id}:{idempotency_key}`.
2. Read `company.carrier_relationship_acceptance_idempotency` by `(actor_tenant_id, idempotency_key)`. A hit with the same company, the same locator, and the same body hash returns the stored relationship and `Created=false`. A hit that differs is `RELATIONSHIP_IDEMPOTENCY_CONFLICT`. This step happens before the token is required.
3. Require a decoded token. Otherwise generic 404.
4. Load the request by `acceptance_token_hash`. Compare with `hmac.Equal`. Compare the optional locator.
5. `SELECT ... FROM company.carrier_relationship_request_heads WHERE relationship_request_id = $1 FOR UPDATE`.
6. If the token is expired and the head is still `PENDING`, append `EXPIRED`, commit that terminal event, and return `RELATIONSHIP_REQUEST_EXPIRED` (409). This step runs before the endpoint comparison. It is the exception to the sibling `PENDING` rule in section 3: an expired token becomes one `EXPIRED` event whether or not the selected company matches the stored endpoint. If the head is already `EXPIRED`, return the same code and do not append another event.
7. `ACCEPTED` returns `RELATIONSHIP_REQUEST_ALREADY_ACCEPTED` (409). `CANCELLED` returns `RELATIONSHIP_REQUEST_CANCELLED` (409). Neither writes a relationship.
8. Lock `relationship:{source_tenant_id}:{shipper_carrier_company_id}:{actor_tenant_id}`.
9. If a root exists and `operating_carrier_company_id` is not the selected company, roll back before `Revalidate` and return `RELATIONSHIP_ACCEPTANCE_FORBIDDEN` (403). Do not call the source callback. No `ACCEPTED` event. A still-valid token leaves the request `PENDING`. After this lock, the loser of a first-insert race is this step, not the unique-index 409.
10. Revalidate only when no root exists or the selected company equals `operating_carrier_company_id`. `UNAVAILABLE` or an error is `RELATIONSHIP_SOURCE_UNAVAILABLE` (503) and no relationship row. `CHANGED`, `NOT_FOUND`, or any value other than `MATCH` is `RELATIONSHIP_REQUEST_STALE_SOURCE` (409). A sibling mismatch never reaches this callback, so an unavailable source does not turn that call into 503.
11. Insert the root and version 1, or link the current `ACTIVE` version, or, when the selected company equals `operating_carrier_company_id` and the head is `REVOKED`, insert the next `ACTIVE` version. That last insert closes revoke test 14. A sibling does not take it. The previous `REVOKED` version stays. No version row is rewritten.
12. Append request event `ACCEPTED` at `current_version + 1`, including `accepted_operating_tenant_id` and `accepted_operating_carrier_company_id`.
13. Insert the idempotency row, audit action `relationship_request_accepted`, and outbox `carrier_relationship.accepted`.
14. Commit. `Created=true` becomes HTTP 201. Replay becomes HTTP 200.

Audit columns are actor, tenant, selected company, request id, relationship id, version, source type, source id, idempotency key, correlation id, and action. Outbox payload fields are request id, relationship id, version, source type, source id, source tenant id, and `request_status`. Neither store contains the raw token, the token hash, or the pepper.

The shared access log records method, path, status, duration, request id, remote address, and user agent. It does not record the body, the query, or the Authorization header. The implementation must not add body logging on this route. The token must not be placed in the query string.

Accept responses today do not set `Cache-Control`. Status and cancel do set `no-store`. The public route sets `Cache-Control: no-store` on 201, 200, and every error.

## 7. Accept, cancel, and expire races

Accept and cancel do not share an advisory lock. They both take `FOR UPDATE` on the same request head. One transaction commits a terminal event. The other sees that status.

| Winner | Loser observes | Loser result | Extra event |
| --- | --- | --- | --- |
| Accept | Cancel | `RELATIONSHIP_REQUEST_ALREADY_ACCEPTED` (409) | No |
| Cancel to `CANCELLED` | Accept | `RELATIONSHIP_REQUEST_CANCELLED` (409) | No |
| Cancel of an already expired token, writing `EXPIRED` | Accept | `RELATIONSHIP_REQUEST_EXPIRED` (409) | No |
| Accept observes expiry first | Cancel | Head is `EXPIRED` | Accept wrote the only `EXPIRED` event |

`assertCancelAcceptRace` in the cancel-recovery integration test covers the same-request accept/cancel race. A pending request does not return to `PENDING` after either command. Revoke of an `ACTIVE` relationship does not change an `ACCEPTED` request back to `PENDING`. The sibling first-insert race is not an accept/cancel race: both callers hold the relationship advisory lock from step 8, one commit creates the root, and the other receives 403 while its own request stays `PENDING`. One root is stored. The unique-index 409 is not that outcome. A later accept of a new pending request on a `REVOKED` head follows section 3.

There is no deadlock between the two commands: cancel waits on the head row; accept does not wait on the cancel advisory lock.

## 8. Replay, lost ACK, and idempotency

The primary key is `(actor_tenant_id, idempotency_key)`. It is not per user. Any later caller who passes the accept predicate for the stored operating company, and who repeats that key and locator, receives the 200 replay. The replay does not require the token and does not return the token. The foundation test covers the same carrier admin without a token.

A different key after the request is `ACCEPTED` is `RELATIONSHIP_REQUEST_ALREADY_ACCEPTED` (409). The same key with a different company, locator, or body hash is `RELATIONSHIP_IDEMPOTENCY_CONFLICT` (409) with empty details.

Lost ACK is the same replay. The commit already stored the relationship id and version. The caller retries the same idempotency key. HTTP 200. `Created=false`. No second `ACCEPTED` event and no second relationship version.

```text
ACCEPT_IDEMPOTENCY_SCOPE=actor_tenant_id+idempotency_key
REPLAY_REQUIRES_TOKEN=NO
REPLAY_RETURNS_TOKEN=NO
REPLAY_HTTP=200
FIRST_COMMIT_HTTP=201
LOST_ACK_RECOVERY=SAME_KEY_REPLAY
NEW_KEY_AFTER_ACCEPT=409_RELATIONSHIP_REQUEST_ALREADY_ACCEPTED
SAME_KEY_DIFFERENT_BODY=409_RELATIONSHIP_IDEMPOTENCY_CONFLICT
IDEMPOTENCY_MIGRATION=NO
```

Replay returns the request status stored with the acceptance (`ACCEPTED`) and the relationship version written at accept time. It does not re-read a later relationship head. A later revoke of that relationship is the merged PR #292 contract. A new accept after that revoke follows section 3: the matching operating company inserts the next `ACTIVE` version, and a sibling receives 403 with the head left `REVOKED`.

## 9. Gateway route and public OpenAPI

`packages/openapi/openapi.yaml` publishes company operations under `/api/v1/companies`. It has no carrier-relationship operation. `services/api-gateway/internal/http/proxy.go` rewrites `/api` + path to the upstream path. `POST /api/v1/carrier-operating-relationship-acceptances` would become `POST /v1/carrier-operating-relationship-acceptances`, which is the existing company-service route.

`MatchRoute` selects the longest prefix. `/api/v1/carrier` already routes to `rfx-service`. The accept path starts with that prefix. A route registered only as `/api/v1/carrier` would send accept to rfx-service. The implementation must register the full accept path, or a prefix that is longer than `/api/v1/carrier` and does not capture other rfx carrier routes, with service `company-service`.

```text
GATEWAY_COLLISION_PREFIX=/api/v1/carrier
GATEWAY_COLLISION_SERVICE=rfx-service
REQUIRED_GATEWAY_PREFIX=/api/v1/carrier-operating-relationship-acceptances
REQUIRED_GATEWAY_SERVICE=company-service
REWRITE=STRIP_/api
UPSTREAM_MATCH=EXISTING_COMPANY_SERVICE_ROUTE
OPENAPI_FILE_THIS_STAGE=UNCHANGED
```

Normative operation, to be added to `packages/openapi` only in the implementation stage:

```yaml
/api/v1/carrier-operating-relationship-acceptances:
  post:
    operationId: acceptCarrierOperatingRelationship
    security:
      - bearerAuth: []
    parameters:
      - in: header
        name: X-Company-ID
        required: true
        schema: { type: string, format: uuid }
    requestBody:
      required: true
      content:
        application/json:
          schema:
            type: object
            required: [acceptance_token, idempotency_key]
            properties:
              acceptance_token: { type: string }
              idempotency_key: { type: string, maxLength: 200 }
              relationship_request_id: { type: string, format: uuid }
              tenant_id: { type: string, format: uuid }
              company_id: { type: string, format: uuid }
            additionalProperties: false
    responses:
      "201":
        description: First accept commit. No acceptance_token.
      "200":
        description: Idempotent replay. No acceptance_token.
      "401":
        description: Authentication required.
      "403":
        description: Commands disabled, actor refused, or sibling operating company.
      "404":
        description: Generic token or locator failure.
      "409":
        description: Expired, cancelled, already accepted, stale source, tenant conflict, or a unique-index idempotency conflict when the relationship advisory lock is not held. The locked sibling race is 403.
      "503":
        description: Store or source unavailable.
```

Success body, both 201 and 200:

```json
{
  "relationship_id": "uuid",
  "relationship_version": 1,
  "request_status": "ACCEPTED"
}
```

Error body, unchanged company-service shape:

```json
{
  "error": {
    "code": "RELATIONSHIP_REQUEST_NOT_FOUND",
    "message": "relationship request not found",
    "details": {}
  }
}
```

Every response includes `Cache-Control: no-store`. The schema does not define `acceptance_token` on the response. `additionalProperties: false` on the request stops a client from sending a company id under another name and expecting it to be read. Unknown JSON fields are ignored by the current decoder. The implementation may keep that decoder. It must not start reading a new field as authority.

Internal create, cancel, and status stay out of this operation. They remain service-credential routes.

## 10. What already matches, and what the next stage must add

Already on `main`, and must stay:

- JWT on the company-service accept route
- the accept predicate in section 4
- HMAC storage, one-time create disclosure, and identical 404s
- idempotency lookup before token reuse
- head-lock serialization with cancel and expiry
- no token in the accept body, audit, or outbox
- commands default off
- source callback fail-closed
- no foreign company id on `RELATIONSHIP_REQUEST_ALREADY_ACCEPTED`

Not on `main`, and required before the route is published:

| Gap | Required change | Migration |
| --- | --- | --- |
| No gateway route, and `/api/v1/carrier` would capture the path | Longer prefix to company-service | No |
| Gateway deletes `X-Company-ID` | Restore the captured selector after the strip | No |
| Accept responses omit `Cache-Control: no-store` | Set it on success and error | No |
| Reuse and reactivation ignore the stored operating company | Endpoint match in section 3, before `Revalidate`. The matching company reactivates `REVOKED` by inserting the next `ACTIVE` version. A sibling does not | No |
| OpenAPI has no operation | Add the operation in section 9 after the route tests pass | No |

```text
MIGRATION_REQUIRED=NO
MIGRATION_NUMBER_ASSIGNED=NO
```

## 11. Tests the implementation stage must add

These tests are not added in this discovery.

| Id | Case | Expected |
| --- | --- | --- |
| PA-01 | Gateway path is not matched by `/api/v1/carrier` | Upstream service is company-service |
| PA-02 | Spoofed `X-User-ID` and `X-Tenant-ID` | Gateway strip; company-service uses the JWT |
| PA-03 | Captured `X-Company-ID` restored, membership absent | 403, no write |
| PA-04 | Body tenant or body company differs | 409 or 403 as in section 4, no write |
| PA-05 | Ordinary member, inactive user, role on another company | 403, store not called |
| PA-06 | `CARRIER_ADMIN` and tenant-global `PLATFORM_ADMIN` with membership | 201 |
| PA-07 | Tenant-global `PLATFORM_ADMIN` without membership | 403 |
| PA-08 | Missing, bad, unknown, and mismatched locator | Four identical 404 bodies |
| PA-09 | Replay and lost ACK without the token | 200, same ids, body has no token |
| PA-10 | Same key, different company or locator | 409 `RELATIONSHIP_IDEMPOTENCY_CONFLICT`, no stored ids |
| PA-11 | Accept versus cancel, both orders | One terminal event, the loser gets 409 |
| PA-12 | Accept at expiry | `EXPIRED` event, 409, no relationship |
| PA-13 | Sibling company, root already present | 403, request stays `PENDING`, token still usable by the endpoint company |
| PA-14 | Sibling company reactivating `REVOKED` | 403, head stays `REVOKED` |
| PA-15 | Matching company, second source, `ACTIVE` root | 201 linking the current version |
| PA-16 | Audit, outbox, and access log | No raw token, hash, or pepper |
| PA-17 | Commands disabled and empty pepper | 403 and 503, no relationship |
| PA-18 | Source unavailable and stale source | 503 and 409, no relationship |
| PA-19 | Response header | `Cache-Control: no-store` |
| PA-20 | Route is not in `isPublicRoute` | Missing bearer is 401 at the gateway |
| PA-21 | Two sibling companies, no root, one business key, two valid tokens | One 201. The other is 403 `RELATIONSHIP_ACCEPTANCE_FORBIDDEN`. The loser's request stays `PENDING`. One root is stored |
| PA-22 | Root belongs to another company in the same tenant, and the source would return `UNAVAILABLE` | 403, zero `Revalidate` calls, request stays `PENDING`. The matching company with an unavailable source still receives 503 |
| PA-23 | Sibling, root of another company, token expired | 409 `RELATIONSHIP_REQUEST_EXPIRED` and one `EXPIRED` event. The same sibling with a valid token receives 403 and the request stays `PENDING` |
| PA-24 | After revoke, two accepts of new tokens | Matching `operating_carrier_company_id`: 201 and the next `ACTIVE` version; the previous `REVOKED` version remains. Sibling: 403, head stays `REVOKED`, no second `ACTIVE` version |

## 12. Implementation sequence

The next stage, after architecture review, is one implementation pull request. It is not authorized by this document.

1. Add the endpoint comparison before `Revalidate`, and `Cache-Control: no-store`, in company-service. A mismatch returns 403 without calling the callback. Cover PA-05 through PA-24. Do not change the idempotency primary key. Do not map the locked first-insert race to 409.
2. Add the gateway route with the longer prefix, restore only the captured company selector, and cover PA-01 through PA-04 and PA-20. Do not register an anonymous route. Do not move `/api/v1/carrier`.
3. Add the OpenAPI operation from section 9 only after those tests pass. Do not generate a second accept path.
4. Leave `COMPANY_RELATIONSHIP_COMMANDS_ENABLED` default off. Do not revoke, do not activate TMS, and do not deploy.

## 13. Non-goals

Anonymous accept, a second token, token reissue, cancel, the revoke command, relationship pins, and activation authorization are outside this contract. Cancel recovery remains the PR #290 contract. The merged PR #292 revoke contract is an input for a `REVOKED` head: a later accept by the stored operating company inserts the next `ACTIVE` version, and a sibling does not. This document does not add a revoke command. Create still returns the raw token once to the source-owning service. This public route never returns it.
