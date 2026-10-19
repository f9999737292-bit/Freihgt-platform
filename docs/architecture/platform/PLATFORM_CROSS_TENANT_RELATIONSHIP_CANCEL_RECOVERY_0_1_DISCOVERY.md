# BINTRANS PLATFORM / IDENTITY

## CROSS-TENANT RELATIONSHIP CANCEL / RECOVERY 0.1 DISCOVERY

```text
AGENT=C
STAGE=TMS-CROSS-TENANT-RELATIONSHIP-CANCEL-RECOVERY-0.1-DISCOVERY
BASE_SHA=bb7aab4747b211da898db3a7c8f781ac40913be1
BRANCH=docs/bintrans-relationship-cancel-recovery-v0.1
DOCS_ONLY=YES
IMPLEMENTATION_AUTHORIZED=NO
MIGRATION_AUTHORIZED=NO
MIGRATION_NUMBER_ASSIGNED=NO
GATEWAY_CHANGE_AUTHORIZED=NO
REVOKE_AUTHORIZED=NO
TMS_ACTIVATION_AUTHORIZED=NO
NLO_RUNTIME_AUTHORIZED=NO
DEPLOYMENT_AUTHORIZED=NO
TOKEN_REISSUE_POLICY=NO_REISSUE_CANCEL_AND_CREATE_NEW_REQUEST
```

This document defines the cancel and lost-ACK recovery contract for a carrier relationship request. It does not authorize runtime code, a migration, a gateway change, revocation, TMS activation, or deployment.

Authoritative inputs read at `bb7aab4747b211da898db3a7c8f781ac40913be1`:

- `docs/architecture/platform/PLATFORM_CROSS_TENANT_EXECUTION_IDENTITY_0_1B_RUNTIME_DESIGN.md`
- `docs/architecture/platform/PLATFORM_CROSS_TENANT_EXECUTION_IDENTITY_0_1A_FREEZE.md`
- `infrastructure/migrations/000100_company_carrier_relationship_foundation_v0_1c.up.sql`
- `services/company-service` relationship create, accept, and read path
- shipment-service relationship create initiator 0.1E2
- rfx-service relationship create initiator 0.1F2

0.1B already froze `TOKEN_REISSUE_POLICY=NO_REISSUE_CANCEL_AND_CREATE_NEW_REQUEST`, `PENDING_REQUEST_CANCEL_AUTHORITY`, and `EXISTING_CANCELLED_REQUEST_POLICY`. This discovery does not rewrite those decisions. It binds them to the schema and commands that are on `main`.

## 1. Verified lifecycle

Request status is stored on `company.carrier_relationship_request_heads.request_status`. The database check allows only `PENDING`, `ACCEPTED`, `EXPIRED`, and `CANCELLED`. The same four values are the event status check. Event types are `CREATED`, `ACCEPTED`, `EXPIRED`, and `CANCELLED`.

`REVOKED` is not a request status. It is a status of `company.carrier_relationship_heads` and `company.carrier_relationship_versions`, together with `ACTIVE`. A relationship version row is insert-only. Revocation, which is not implemented, would insert a new version. It would not edit the accepted request.

| Status | Where it lives | Writer on main | Meaning |
| --- | --- | --- | --- |
| `PENDING` | Request head after event `CREATED` version 1 | `RelationshipRepository.CreateRelationshipRequest` | One open request for that source type, source id, and shipper carrier company |
| `ACCEPTED` | Request head after event `ACCEPTED` | `RelationshipRepository.AcceptRelationshipRequest` | Token consumed. Relationship root and an `ACTIVE` version exist, or an existing `REVOKED` root received a new `ACTIVE` version |
| `EXPIRED` | Request head after event `EXPIRED` | Accept transaction only, when the head is still `PENDING` and `token_expires_at <= clock_timestamp()` | Terminal for that request. Pending slot released |
| `CANCELLED` | Allowed by checks, event columns, and the accept switch | No command writes it | Terminal status the accept path already refuses with `RELATIONSHIP_REQUEST_CANCELLED` (409) |
| `ACTIVE` | Relationship head and version | Accept | Current operating relationship. Not a request status |
| `REVOKED` | Relationship head and version | No command writes it | Historical relationship state. Not a request status |

`RelationshipService` exposes `Create`, `Accept`, and `Get`. It has no cancel method. The HTTP router exposes:

- `POST /v1/carrier-relationship-requests` with the dedicated caller credential
- `GET /v1/carrier-relationship-requests/{relationship_request_id}` with a user JWT
- `POST /v1/carrier-operating-relationship-acceptances` with a user JWT

There is no cancel route and no list route. `services/api-gateway` has no carrier-relationship route.

The request root is insert-only. Request events are insert-only. Relationship roots and versions are insert-only. Triggers call `company.reject_append_only_mutation`. The foundation integration test rejects an in-place update of an event to `CANCELLED` and an in-place update of a version to `REVOKED`. The head row is mutable. Its identity columns (`relationship_request_id`, `source_type`, `source_id`, `shipper_carrier_company_id`) cannot change. Its status can change to any of the four allowed values. No trigger limits the transition.

Token lifetime is a database constraint: `token_expires_at = token_created_at + interval '72 hours'`. Both timestamps are taken from `clock_timestamp()` inside the create insert. The raw token is 32 random bytes, returned only when `PersistCreateResult.Created` is true, and stored only as HMAC-SHA-256 with the company-service pepper. Create replay sets `Created` false and omits `acceptance_token`. The hash column is unique. The hash is not reversible to the token.

There is no expiry sweeper. A `PENDING` head remains `PENDING` after `token_expires_at` until an accept transaction observes the clock and appends `EXPIRED`. A lost token means that path does not run. The partial unique index `uq_crr_one_pending_source_carrier` still matches the head, so a later create with a different idempotency key returns `RELATIONSHIP_REQUEST_PENDING_CONFLICT` (409). That conflict is independent of the clock.

Create idempotency is `(caller_service, source_tenant_id, requesting_company_id, idempotency_key)`. The stored hash covers source type, source id, fingerprint, and shipper carrier company. The same key and the same hash return the current head status and do not mint a token. The same key and a different hash return `RELATIONSHIP_IDEMPOTENCY_CONFLICT` (409). Shipment and RFx integration tests record the lost-ACK case: the company row commits, the caller sees a transport failure, the same key replays `200` without `acceptance_token`, and a different key is `409`. Both clients state that they do not invent a replacement key and that recovery waits for a later cancel. Neither client performs that cancel.

`COMPANY_RELATIONSHIP_COMMANDS_ENABLED` is off unless the value is `true` or `1`. Off returns `RELATIONSHIP_COMMANDS_DISABLED` (403) before any write.

## 2. State transition matrix

Legal request-head transitions after this contract:

| From | Command | Guard under the head lock | To | New event |
| --- | --- | --- | --- | --- |
| `PENDING` | Cancel, database clock before `token_expires_at` | Actor authorized, caller credential matches source type, source id matches the root | `CANCELLED` | `CANCELLED` at `current_version + 1` |
| `PENDING` | Cancel, database clock at or after `token_expires_at` | Same authorization | `EXPIRED` | `EXPIRED` at `current_version + 1` |
| `PENDING` | Accept with valid token, clock before expiry, source still matches | Existing accept contract | `ACCEPTED` | `ACCEPTED` |
| `PENDING` | Accept with valid token, clock at or after expiry | Existing accept contract | `EXPIRED` | `EXPIRED` |
| `CANCELLED` | Cancel, create replay, or accept | Terminal | `CANCELLED` | None |
| `EXPIRED` | Cancel or create replay | Terminal | `EXPIRED` | None |
| `ACCEPTED` | Cancel | Terminal | `ACCEPTED` | None |
| `ACCEPTED`, `EXPIRED`, `CANCELLED` | Any command | No return to `PENDING` | Unchanged | None |

`CANCELLED` does not delete the root, the `CREATED` event, the token hash, or the create idempotency row. Moving the head off `PENDING` drops it from `uq_crr_one_pending_source_carrier`. That is the pending-slot release. A new `PENDING` head for the same source and carrier can then be inserted.

Cancel does not insert or update `company.carrier_relationships`, versions, or relationship heads. An `ACTIVE` or `REVOKED` relationship is unchanged. Cancel of a still-`PENDING` request is allowed whether or not some other relationship root for another operating tenant is `REVOKED`.

A request that is already `ACCEPTED` stays `ACCEPTED` if the relationship is later revoked. Revoke is outside this stage. Cancel of that request is the `ACCEPTED` row above.

## 3. Lost ACK recovery

Observed sequence on main:

1. `CreateRelationshipRequest` commits the root, `CREATED` event, `PENDING` head, create idempotency row, audit `relationship_request_created`, and outbox `carrier_relationship.requested`.
2. The HTTP response that carried the raw token is lost.
3. The same idempotency key returns `200` with `relationship_request_id` and `request_status`, and no `acceptance_token`.
4. A different idempotency key returns `409` `RELATIONSHIP_REQUEST_PENDING_CONFLICT` while the head stays `PENDING`.
5. Nothing on main can turn that head into `CANCELLED`. If nobody presents the lost token, nothing writes `EXPIRED` either.

Recovery contract:

```text
LOST_ACK_STATUS_SOURCE=ORIGINAL_CREATE_KEY_REPLAY_OR_SOURCE_SCOPED_STATUS
LOST_ACK_STATUS_BODY=relationship_request_id+request_status+shipper_carrier_company_id+token_expires_at
LOST_ACK_STATUS_TOKEN=ABSENT
LOST_ACK_BEFORE_EXPIRY=AUTHORIZED_CANCEL_THEN_NEW_CREATE
LOST_ACK_AFTER_EXPIRY=AUTHORIZED_CANCEL_COMMITS_EXPIRED_THEN_NEW_CREATE
NEW_TOKEN_SOURCE=NEW_CREATE_WITH_A_NEW_CREATE_IDEMPOTENCY_KEY_ONLY
AUTOMATIC_TOKEN_REISSUE=NO
AUTOMATIC_EXPIRE_INSIDE_CREATE=NO
SAME_CREATE_KEY_AFTER_CANCEL=REPLAYS_STORED_REQUEST_WITHOUT_A_TOKEN
```

The safe status is not a token lookup. It is either the existing create replay of the original key, or a source-scoped status read defined below. Both return the request id and the head status. Neither returns the raw token or the token hash.

The original create key remains bound to the stored request. After a confirmed `CANCELLED` or `EXPIRED` head, that same key still replays that request and still omits the token. A new token exists only on a create whose company-service result has `Created=true`. That create uses a new key in the create idempotency namespace. Reusing the lost key does not satisfy this step.

A changed source fingerprint does not block cancel. Create replay of the original key becomes `RELATIONSHIP_IDEMPOTENCY_CONFLICT` once the fingerprint in the hash no longer matches, which the shipment integration test already shows. The actor then uses the source-scoped status read, which does not require the old fingerprint, and cancels by `relationship_request_id`.

Cancel and the following create are separate commands. Create does not cancel or expire a blocking head as a side effect. A new create is legal only after the blocking head for that source and carrier is no longer `PENDING`.

If the source row has been deleted, the user route under that source id cannot be resolved. This contract does not add a token-free company-wide search and does not reissue the token. A `PENDING` head whose source row is gone stays a residual case for a later sweeper. The sweeper is not part of the next runtime stage.

```text
DELETED_SOURCE_PENDING_RELEASE=FUTURE_SWEEPER
SWEEPER_IN_NEXT_RUNTIME=NO
```

## 4. API contract

User commands stay on the source owner, as create already does. Company-service remains the writer. The gateway is not changed by the next runtime stage. The source-owner routes are mounted on shipment-service and rfx-service in the same way as create.

Status, for an authorized creator:

- `GET /v1/shipments/{shipment_id}/carrier-relationship-requests`
- `GET /v1/rfx-awards/{award_id}/carrier-relationship-requests`
- `GET /v1/rfx-award-transport-orders/{link_id}/carrier-relationship-requests`

The path id is a selector. Tenant comes from the verified JWT. A missing source, or a source in another tenant, is `404` with the source's existing not-found behavior. The body is a list of request heads whose root `source_type`, `source_id`, and `source_tenant_id` match that source. Each item contains `relationship_request_id`, `request_status`, `shipper_carrier_company_id`, and `token_expires_at`. The list is empty when no row matches. It does not contain `acceptance_token`, `acceptance_token_hash`, actor ids, or another tenant's rows.

Cancel, for an authorized creator:

- `POST /v1/shipments/{shipment_id}/carrier-relationship-requests/{relationship_request_id}/cancel`
- `POST /v1/rfx-awards/{award_id}/carrier-relationship-requests/{relationship_request_id}/cancel`
- `POST /v1/rfx-award-transport-orders/{link_id}/carrier-relationship-requests/{relationship_request_id}/cancel`

`Idempotency-Key` is required and is at most 200 characters. The JSON body may contain only `cancel_reason`. Any other field, including `acceptance_token`, `tenant_id`, `company_id`, `requesting_user_id`, and fingerprint fields, is `400` `VALIDATION_ERROR`. `cancel_reason` is optional, trimmed, and at most 500 characters. A longer reason is `400`. The reason is not authority.

Success body, first commit and replay:

```text
HTTP=200
BODY=relationship_request_id+request_status
ACCEPTANCE_TOKEN_FIELD=ABSENT
CACHE_CONTROL=no-store
```

`request_status` is `CANCELLED` when this command, or an earlier one, cancelled the request. It is `EXPIRED` when this command, or an earlier one, closed a clock-expired `PENDING` head. Both are successful recovery results: the pending slot is free and no token is issued.

Company-service internal command, called only by the source owner:

```text
POST /v1/carrier-relationship-requests/{relationship_request_id}/cancellations
CREDENTIAL=X-Internal-Service-Token dedicated to the caller
CALLER_NAME_HEADER=IGNORED
USER_JWT=NOT_AUTHORITY
```

The trusted subject is `source_type`, `source_id`, `source_tenant_id`, `requesting_user_id`, `idempotency_key`, and optional `cancel_reason`. Company-service loads the request root and the actor from the database. A body tenant or company that disagrees with the root is a refusal. `X-User-ID`, `X-Tenant-ID`, `X-User-Roles`, `X-Platform-Admin`, and `X-Company-ID` are not authority.

Existing reads stay token-free. `GET /v1/carrier-relationship-requests/{id}` continues to return only `relationship_request_id` and `request_status` to a reader who is the requesting company or the accepting operating company. Cancel must not add the token to that body. The internal companies batch route is not a relationship read and must not grow one.

| Result | HTTP | Code | Token |
| --- | --- | --- | --- |
| Cancel commits `CANCELLED`, or replays it | 200 | none | No |
| Cancel commits `EXPIRED` because the clock has passed, or replays `EXPIRED` | 200 | none | No |
| Request unknown, source mismatch, or caller tenant is not the source tenant | 404 | `RELATIONSHIP_REQUEST_NOT_FOUND` | No |
| Actor fails the creator predicate | 403 | `RELATIONSHIP_REQUEST_FORBIDDEN` | No |
| Commands flag off | 403 | `RELATIONSHIP_COMMANDS_DISABLED` | No |
| Missing or unknown service credential on the internal route | 401 | `AUTHENTICATION_REQUIRED` | No |
| Credential caller does not own the source type | 403 | `RELATIONSHIP_REQUEST_FORBIDDEN` | No |
| Head is `ACCEPTED` | 409 | `RELATIONSHIP_REQUEST_ALREADY_ACCEPTED` | No |
| Same cancel key, different request id | 409 | `RELATIONSHIP_IDEMPOTENCY_CONFLICT` | No |
| Store failure, including a lost commit acknowledgement before the caller knows the outcome | 503 | `RELATIONSHIP_STORE_UNAVAILABLE` | No |
| Accept presents the old token after cancel | 409 | `RELATIONSHIP_REQUEST_CANCELLED` | No |
| Create with a new key while the head is still `PENDING` | 409 | `RELATIONSHIP_REQUEST_PENDING_CONFLICT` | No |
| Create with the original key after cancel | 200 | none, status of the stored request | No |
| Create with a new key after the head left `PENDING` | 201 | none | Yes, once, `Cache-Control: no-store` |

RFx create already sets `Cache-Control: no-store` on the `201` that contains the token. Shipment create on this SHA does not set that header. The next runtime stage does not need to change shipment create to make cancel correct. Any response that contains a raw token, including the new create after recovery, must send `Cache-Control: no-store`. Cancel and status responses also send `no-store` so they are not reused as if they were a token response.

## 5. Authorization matrix

The actor predicate is the existing create predicate. `CanCreateRelationshipRequest` requires an active user in the verified tenant, an active membership on the company being proved, and either a tenant-global `PLATFORM_ADMIN` (`core.user_roles.company_id IS NULL`) or a company-scoped creator role. `PLATFORM_ADMIN` replaces only the role alternative. A company-scoped `PLATFORM_ADMIN` row is not the override.

The company being proved is `carrier_relationship_requests.requesting_company_id`. It is not taken from the client. For `SHIPMENT` the creator roles are `SHIPPER_ADMIN`, `SHIPPER_LOGIST`, and `FORWARDER_MANAGER`. For `RFX_AWARD` and `AWARD_TRANSPORT_ORDER_LINK` those three roles plus `PROCUREMENT_MANAGER` qualify. `CARRIER_ADMIN` and `CARRIER_DISPATCHER` do not cancel.

| Actor | Status read by source | Cancel | Accept |
| --- | --- | --- | --- |
| Active creator role on the requesting company | Yes | Yes, while the source row exists | No, unless the actor is also the operating carrier admin on an operating company |
| Tenant-global `PLATFORM_ADMIN` with active membership on the requesting company | Yes | Yes | Only when the same predicate is evaluated on the operating company |
| `PLATFORM_ADMIN` without that membership | No | No | No |
| `CARRIER_ADMIN` of the requesting company | No on the source list | No | No |
| `CARRIER_ADMIN` of a different operating company | No | No | Existing accept contract, with the token |
| Member with no creator role | No on the source list | No | No |
| Inactive, deleted, or other-tenant user | No | No | No |
| Holder of the other service's dedicated credential | No | No | No |

The original `requesting_user_id` has no private cancel right after losing the creator role. Any current creator on the requesting company may cancel. That matches 0.1B `PENDING_REQUEST_CANCEL_AUTHORITY=SOURCE_OWNER_CREATOR_AUTHORIZATION|SOURCE_TENANT_GLOBAL_PLATFORM_ADMIN`.

A foreign company or foreign tenant receives `404`, including when the request id is valid, so the refusal does not confirm the row. A member of the requesting company who fails the creator role receives `403` and no status body. The existing by-id GET remains available to its current readers; this discovery does not widen it.

The shipment credential may cancel only `SHIPMENT`. The RFx credential may cancel only `RFX_AWARD` and `AWARD_TRANSPORT_ORDER_LINK`. An empty credential, two equal configured credentials, or a credential that matches neither configured token fails closed, as `identifyRelationshipCaller` already does for create. The shared `INTERNAL_SERVICE_TOKEN` and `X-Internal-Service-Name` are not cancel credentials.

Cancel does not call source revalidation and does not require the stored fingerprint to equal the current source version. A stale source must remain cancellable. Accept keeps its current stale-source refusal.

## 6. Idempotency

Create and accept keep their current namespaces. Cancel uses a third namespace.

```text
CANCEL_IDEMPOTENCY_SCOPE=caller_service+source_tenant_id+requesting_company_id+idempotency_key
CANCEL_BODY_HASH=relationship_request_id
CANCEL_REASON_IN_HASH=NO
CANCEL_REPLAY=SAME_SCOPE_SAME_REQUEST_RETURNS_STORED_TERMINAL_STATUS_NO_NEW_EVENT
CANCEL_CONFLICT=SAME_SCOPE_DIFFERENT_REQUEST_ID
CREATE_KEY_AND_CANCEL_KEY_NAMESPACES=DISTINCT
```

The first committed cancel stores the idempotency row in the same transaction as the event, head update, audit, and outbox. A replay of that key does not append an event, does not move the head, and does not insert a second outbox row. A different reason on replay does not conflict and does not change the stored reason.

A second cancel key for a request that is already `CANCELLED` or `EXPIRED` returns `200` and the current status. It inserts no event. It stores its own idempotency row when that key is new, so a later use of that key for another request is `RELATIONSHIP_IDEMPOTENCY_CONFLICT`. A key that already names another request is that conflict even when the target request is already terminal.

A cancel that loses the race to accept does not store a success row. The response is `409` `RELATIONSHIP_REQUEST_ALREADY_ACCEPTED`. Retrying the same key observes the accepted head and returns the same `409`.

Create after cancel uses the existing create rule. The original create key replays the cancelled or expired request without a token. A new create key inserts a new root and returns a new token once. The cancel key string may be reused as a create key because the tables differ. That reuse is not the recovery step. The recovery step is a create key that has no create idempotency row.

## 7. Concurrency

Every cancel takes `SELECT ... FOR UPDATE` on the request head and holds it until commit. The lock order is the cancel idempotency advisory lock, then the head row. Accept already locks the head after its own idempotency lock. Create locks the pending scope and then reads the pending head `FOR UPDATE`.

A terminal event insert uses `current_version + 1`. `uq_crr_event_version` rejects two events with the same version. The head lock is what stops two transactions from choosing that version. If the unique index still fires, the transaction rolls back. The caller receives `503` `RELATIONSHIP_STORE_UNAVAILABLE`, or retries the command once inside the service and then returns the winner's status. That unique violation must not be mapped to `RELATIONSHIP_IDEMPOTENCY_CONFLICT`. The current repository maps every `23505` except `uq_crr_one_pending_source_carrier` to the idempotency conflict. The runtime stage must keep the terminal-event violation out of that branch.

| Case | Final head | HTTP | New event | New outbox | Pending slot |
| --- | --- | --- | --- | --- | --- |
| Cancel and accept, cancel commits first | `CANCELLED` | Cancel `200`. Accept `409` `RELATIONSHIP_REQUEST_CANCELLED` | One `CANCELLED` | One `carrier_relationship.cancelled` | Released |
| Cancel and accept, accept commits first | `ACCEPTED` | Accept `201` or replay `200`. Cancel `409` `RELATIONSHIP_REQUEST_ALREADY_ACCEPTED` | One `ACCEPTED` | One `carrier_relationship.accepted` | Released by accept, not by cancel |
| Cancel and clock expiry, cancel sees an unexpired clock | `CANCELLED` | Cancel `200` | One `CANCELLED` | One cancelled | Released |
| Cancel and clock expiry, clock is due while the head is `PENDING` | `EXPIRED` | Cancel `200` with status `EXPIRED`. A concurrent accept also observes `EXPIRED` or waits and then sees it | One `EXPIRED` | One `carrier_relationship.expired` | Released |
| Two cancels, same key | `CANCELLED` or `EXPIRED`, once | Both `200` with the same status | One | One | Released once |
| Two cancels, different keys | Same terminal status | Both `200` | One, written by the lock winner | One | Released once |
| Cancel after accept | `ACCEPTED` | `409` `RELATIONSHIP_REQUEST_ALREADY_ACCEPTED` | None | None | Already released |
| Cancel after the relationship version is `REVOKED` | The request head decides. `ACCEPTED` stays `ACCEPTED`. A different still-`PENDING` request can still be cancelled | `409` for the accepted request. `200` for the pending request | None for the accepted request | None for the accepted request | Unchanged for the accepted request |
| Create in the same moment as cancel, cancel not committed | `PENDING` until cancel commits | Create with a new key `409` `RELATIONSHIP_REQUEST_PENDING_CONFLICT` | None from create | None from create | Held |
| Create after cancel commits | New request `PENDING`. Old request `CANCELLED` or `EXPIRED` | New key `201` with a new token. Old key `200` without a token | `CREATED` on the new request only | `carrier_relationship.requested` on the new request only | Held by the new head only |
| Crash after inserting the cancel event and before commit | `PENDING` | Caller `503`. Retry of the same key performs the cancel | None remain | None remain | Held |
| Cancel commits and the HTTP response is lost | Terminal status already stored | Retry of the same key `200`, no token | None beyond the committed event | None beyond the committed outbox row | Released |

The crash row is one database transaction: event, head update, cancel idempotency, audit, and outbox commit together or roll back together. A partial cancel event is not visible.

## 8. Audit and outbox

The cancel transaction appends one event. For `CANCELLED`, the event sets `cancelled_at` from `clock_timestamp()`, `cancelled_by` to the actor user, and `cancel_reason` to the stored reason or null. It leaves the acceptance columns null. For the clock-expired path, the event is `EXPIRED` and the cancel columns stay null. That is the same expiry shape accept already writes.

Audit action `relationship_request_cancelled` is written only for a new `CANCELLED` event. The existing expiry helper already writes `relationship_request_expired` and outbox `carrier_relationship.expired`. Audit may store the reason. Audit does not store the raw token, the token hash, or the pepper.

A new `CANCELLED` event writes one outbox row, `carrier_relationship.cancelled`, in the same transaction. The payload is `relationship_request_id`, `source_type`, `source_id`, `source_tenant_id`, and `request_status`. It omits the token, the hash, the reason, and the actor. Replay writes nothing. The outbox check constraint already allows this event type. No new outbox type is required.

Request events and relationship versions remain append-only. Cancel does not update them in place.

## 9. PostgreSQL constraints and migration plan

Already enforced, and sufficient for slot release:

- `chk_crr_head_status` and `chk_crr_event_status` include `CANCELLED`.
- `chk_crr_event_type` includes `CANCELLED`.
- `uq_crr_one_pending_source_carrier` is partial on `request_status = 'PENDING'`.
- `uq_crr_event_version` is unique on `(relationship_request_id, request_version)`.
- `uq_crr_event_accepted` allows only one `ACCEPTED` event per request.
- `uq_crr_acceptance_token_hash` keeps the old hash reserved, so the lost token cannot be stored again.
- Append-only triggers protect the root, events, relationship root, and versions.
- The outbox check already lists `carrier_relationship.cancelled`.
- Event columns `cancelled_at`, `cancelled_by`, and `cancel_reason` already exist.

Not enforced, and required before runtime cancel:

- There is no cancel idempotency table.
- There is no unique rule that a request has only one of `ACCEPTED`, `EXPIRED`, or `CANCELLED`.
- The head can move from a terminal status to another status, or back to `PENDING`.

```text
MIGRATION_REQUIRED=YES
MIGRATION_NUMBER_ASSIGNED=NO
MIGRATION_CREATED_IN_THIS_STAGE=NO
```

The later runtime stage assigns the next unused migration number when it is authorized. This discovery does not reserve a number. The migration class is three objects and no new token column:

1. A cancel idempotency table keyed by `caller_service`, `source_tenant_id`, `requesting_company_id`, and `idempotency_key`, with the request id, a 32-byte hash of that request id, and the terminal status stored for replay.
2. A unique index on request events for one terminal event per request, limited to `ACCEPTED`, `EXPIRED`, and `CANCELLED`.
3. A head transition guard: `PENDING` may become `ACCEPTED`, `EXPIRED`, or `CANCELLED`, and a terminal status may not change.

The down migration drops those three objects. It does not delete request rows, events, or outbox history. Existing foundation tables are not rewritten.

## 10. Security

- Tenant isolation is the request root's `source_tenant_id`, compared with the JWT tenant on the source-owner route and rechecked by company-service.
- The actor is loaded with `LoadRelationshipActor`: active user, active membership, company-scoped roles, and tenant-global `PLATFORM_ADMIN` only.
- A request whose requesting company is not the actor's proved company is `404`.
- Client identity headers and body company or user ids are not authority.
- The dedicated service credential is bound to one caller and one source-type set. A direct call without that credential fails closed.
- Status and cancel bodies have no raw token and no token hash. A presented `acceptance_token` on cancel is a validation refusal, not a lookup key.
- The old token presented to accept after cancel returns `409` `RELATIONSHIP_REQUEST_CANCELLED` and does not accept.
- The internal cancellation route is not added under `/internal/v1/companies` and is not authenticated by `INTERNAL_SERVICE_TOKEN`.
- Logs, audit, and outbox follow the existing token rule: never store or emit the raw token or the pepper.
- Gateway routes are out of scope. A later gateway registration must proxy the source-owner command without logging the create body and without a route that returns a stored token.

## 11. Integration tests required by the runtime stage

PostgreSQL tests, not store fakes alone:

1. Same create key after a lost acknowledgement returns `200` without a token, one request row, and one `requested` outbox row.
2. A different create key while that head is `PENDING` returns `409` `RELATIONSHIP_REQUEST_PENDING_CONFLICT` and does not insert a second request.
3. An authorized creator cancels that pending request. The head is `CANCELLED`, the `CREATED` event remains, one `CANCELLED` event and one cancel outbox row exist, and the response has no token.
4. The original create key then returns `200` with `CANCELLED` and no token. A new create key returns `201`, a different request id, and a different token, once.
5. Cancel replay with the same key does not add an event or an outbox row. A second key also adds no event.
6. The same cancel key used for another request id returns `409` `RELATIONSHIP_IDEMPOTENCY_CONFLICT`.
7. A carrier admin, a user without membership, an inactive user, a foreign tenant, and the other service credential do not cancel and do not write an event.
8. Spoofed `X-User-ID`, `X-Tenant-ID`, `X-User-Roles`, and `X-Platform-Admin` do not change the actor.
9. Concurrent cancel and accept produce one terminal event. The loser receives the code in the concurrency table.
10. Cancel at or after `token_expires_at` commits `EXPIRED`, releases the pending index, and still returns no token.
11. Cancel after accept returns `409` `RELATIONSHIP_REQUEST_ALREADY_ACCEPTED` and does not change the relationship version.
12. A rolled-back cancel leaves the head `PENDING` and leaves the event, idempotency, audit, and outbox counts unchanged.
13. Status and cancel responses, and a create `201`, do not contain `acceptance_token` except the single `201`. Database rows contain the hash and not the raw token.
14. A changed source fingerprint still allows cancel by request id.
15. Shipment, RFx award, and award-link each refuse the other caller's credential.
16. Commands disabled refuses cancel with no write.
17. An in-place update or delete of the new cancel event fails. A second terminal event for one request fails the new unique index. A head update from `CANCELLED` back to `PENDING` fails the transition guard.

## 12. Next runtime stage

The next stage may implement only this contract:

- source-owner status and cancel commands for shipment, RFx award, and award-link
- the company-service cancellation command behind the existing dedicated credentials
- the three migration objects, numbered at that stage
- the PostgreSQL tests above
- the same commands flag, still defaulting off

It must not implement revoke, a relationship version transition, TMS activation, the authorization function's runtime use, NLO, gateway changes, OpenAPI publication beyond the source-owner routes it adds, token reissue, or an expiry sweeper. Deployment and merge of that runtime stage need their own authorization.

```text
NEXT_STAGE=TMS-CROSS-TENANT-RELATIONSHIP-CANCEL-RECOVERY-0.1-IMPLEMENTATION
NEXT_STAGE_AUTHORIZED=NO
READY_FOR_CONTROLLER_REVIEW=YES
READY_FOR_MERGE=NO
```
