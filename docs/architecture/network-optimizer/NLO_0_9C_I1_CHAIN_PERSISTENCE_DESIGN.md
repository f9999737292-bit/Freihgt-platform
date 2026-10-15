# NLO-0.9C-I1 chain persistence and API contract freeze

This document freezes the durable persistence and public API design for the bounded two-load chain search. It does not create a migration, OpenAPI file, public route, or Go persistence implementation.

```text
NLO_0_9C_I1_MODE=DOCS_ONLY_ARCHITECTURE
NLO_0_9C_I1_STATUS=PERSISTENCE_API_CONTRACT_FROZEN
PRODUCT_IMPLEMENTATION_AUTHORIZED=NO
MIGRATION_AUTHORIZED=NO
OPENAPI_IMPLEMENTATION_AUTHORIZED=NO
PUBLIC_ROUTE_ACTIVATION_AUTHORIZED=NO
STAGING_AUTHORIZED=NO
TMS_EXECUTION_HANDOFF_AUTHORIZED=NO
MIGRATION_NUMBER=UNASSIGNED
MIGRATION_CREATED=NO
OPENAPI_CHANGED=NO
GATEWAY_CHANGED=NO
PRODUCT_CODE_CHANGED=NO
```

Current review baseline: `52def1ba7f1098efe9a8da809b15ffa7da7d5abb`.
Authoring baseline at first draft: `755f2a946f5fa746363935de25d0925e32fd9e90`.

Authority order: NLO-0.9B freezes search policy (horizon, caps, objective, privacy, stacking, seed, execution boundary). NLO-0.9C-I0 implemented the in-memory engine without durable tables or a public route. This I1 freeze defines how a later persistence stage stores search runs, shortlists, and idempotency, and how the public resources behave. It does not supersede 0.9B policy values. It does not authorize NLO-0.9D execution.

Decision record: [ADR-NET-026](adr/ADR-NET-026-chain-search-persistence.md).

## Current baseline

```text
CHAIN_RUNTIME_FOUNDATION_PRESENT=YES
CHAIN_PERSISTENCE_PRESENT=NO
PUBLIC_CHAIN_ENDPOINT_ACTIVE=NO
DURABLE_IDEMPOTENCY_PRESENT=NO
TMS_EXECUTION_OWNER=YES
NLO_EXECUTION_OWNER=NO
```

The I0 engine already allocates `chain_search_id` and `chain_candidate_id`, compares two load orders, enforces the 48h horizon and budget caps, cancels without retry, and projects a pre-accept privacy surface. Persistence today is only an in-memory repository double for tests.

## A. Chain search resource

```text
CHAIN_SEARCH_RESOURCE=CHAIN_SEARCH
CHAIN_SEARCH_ID=SERVER_GENERATED_UUID
CHAIN_SEARCH_TENANT_SCOPED=YES
CLIENT_SUPPLIED_CHAIN_SEARCH_ID_ACCEPTED=NO
```

Conceptual durable fields:

| Field | Required | Notes |
| --- | --- | --- |
| `id` | YES | Server UUID. Same as public `chain_search_id`. |
| `tenant_id` | YES | Carrier operating tenant from gateway JWT. Never client-trusted. |
| `status` | YES | See lifecycle below. |
| `created_at` | YES | Row insert time (UTC). |
| `started_at` | nullable | Set atomically on `PENDING` → `RUNNING`, before expensive work begins. |
| `completed_at` | nullable | Set when status becomes `COMPLETED` or `FAILED`. |
| `cancelled_at` | nullable | Set when status becomes `CANCELLED`. |
| `request_fingerprint` | YES | Hash of the frozen request inputs. |
| `policy_version` | YES | Chain policy version string. |
| `algorithm_version` | YES | Bounded two-order comparison algorithm version. |
| `catalog_fingerprint` | YES | Compatibility/catalog provenance used for the run. |
| `routing_provider_version` | YES | Proven provider version, or `UNKNOWN`. |
| `deadline_at` | YES | `created_at + 20s`; claim setup consumes the same bounded compute window. Never extended. |
| `owner_token` | YES | Server-generated opaque claim token; internal only, never a public credential or response field. |
| `claim_expires_at` | YES | `deadline_at + 5s`; bounded terminal-write grace, not additional compute time. Never renewed. |
| `service_duration_policy_id` / `service_duration_policy_version` | conditional | Server-resolved policy provenance, or explicit `UNKNOWN` when no policy is available. |
| `service_duration_snapshot_json` | YES | Canonical resolved duration values and known/unknown markers actually used by chain and route-plan evaluation. |
| `horizon_start` | YES | Fixed 48h horizon start instant. |
| `horizon_end` | YES | Fixed 48h horizon end instant. |
| `seed_source` | YES | `MANUAL_AVAILABLE_CAPACITY` or `CURRENT_TRIP_RESIDUAL`. |
| `seed_capacity_id` / `seed_capacity_version` | conditional | Manual seed. |
| `seed_shipment_id` / `seed_shipment_version` | conditional | Current-trip seed. Carrier-owned only. |
| `candidate_count` | YES | Shortlist rows retained, at most `CHAIN_PAIR_CAP=4`. |
| `evaluated_pair_count` | YES | Route-plan evaluations started, at most 2. |
| `provider_call_count` | YES | Provider calls charged to the search. |
| `search_complete` | YES | `true` only for `COMPLETED`. |
| `global_optimum` | YES | Always `false`. |
| `execution_supported` | YES | Always `false` through 0.9C. |
| `failure_code` | nullable | Machine code when `FAILED`. |
| `cancel_reason_class` | nullable | Machine class when `CANCELLED`. |
| `reason_counts_json` | YES | Aggregate exclusion counts only. No foreign private detail. |

### Status lifecycle

```text
PENDING
RUNNING
COMPLETED
CANCELLED
FAILED
```

No additional primary statuses are frozen.

| Status | Meaning |
| --- | --- |
| `PENDING` | Idempotency claim accepted; durable search id allocated; expensive work not yet started. |
| `RUNNING` | Provider or route-plan work in progress under the request context. |
| `COMPLETED` | Search finished inside the budget. `search_complete=true`. Candidate set may be empty. |
| `CANCELLED` | Client or gateway cancelled the request context. `search_complete=false`. `NO_RETRY=YES`. |
| `FAILED` | Search stopped without a complete result. Includes budget exhaustion, hard system/provider failure, and abandoned-claim recovery (`CHAIN_INTERRUPTED`). `search_complete=false`. |

Mapping from I0 outcome codes:

| I0 / 0.9B outcome | Durable status | `failure_code` |
| --- | --- | --- |
| `SEARCH_COMPLETE` | `COMPLETED` | null |
| `CHAIN_BUDGET_EXHAUSTED` | `FAILED` | `CHAIN_BUDGET_EXHAUSTED` |
| `CANCELLED` | `CANCELLED` | null |
| Predicted capacity / invalid seed after claim | `FAILED` | policy code such as `PREDICTED_CAPACITY_EXCLUDED` |
| Idempotency fingerprint conflict | no new search row | HTTP `409` |

`PENDING` and `RUNNING` distinguish unfinished claims from terminal audit rows. The first public POST remains synchronous: the owner waits for a terminal status within the bounded request. Matching concurrent POSTs do not start another computation or wait indefinitely; section D defines their response and recovery.

Allowed transitions are `PENDING` → `RUNNING`, `PENDING`/`RUNNING` → `CANCELLED` or `FAILED`, and `RUNNING` → `COMPLETED`. Terminal states are immutable. A claim that expires in either nonterminal state becomes `FAILED` with `failure_code=CHAIN_INTERRUPTED`, `completed_at` set, `search_complete=false`, and no public successful shortlist. Recovery never re-runs computation and never infers client cancellation merely from a process crash.

## B. Chain candidate resource

```text
CHAIN_CANDIDATE_RESOURCE=CHAIN_CANDIDATE
CHAIN_CANDIDATE_ID=SERVER_GENERATED_UUID_BEFORE_EVALUATION
PRE_ACCEPT_PRIVACY_PRESERVED=YES
```

One candidate is one unordered pair inside one search. Both action orders are evaluations of that candidate.

Conceptual durable fields:

| Field | Required | Notes |
| --- | --- | --- |
| `candidate_id` | YES | Server UUID allocated before evaluation. |
| `search_id` | YES | FK to the owning search. |
| `tenant_id` | YES | Same tenant as the search. Denormalized for isolation indexes. |
| `ordinal` / `rank` | YES | Dense rank in the returned shortlist, 1..`candidate_count`. |
| `first_load_id` / `first_load_version` | YES | Lower load id of the unordered pair. Publication-visible only. |
| `second_load_id` / `second_load_version` | YES | Higher load id of the unordered pair. |
| `feasibility_status` | YES | `FEASIBLE`, `INFEASIBLE`, or `UNFINISHED`. |
| `selected_order` | nullable | `L1_BEFORE_L2` or `L2_BEFORE_L1` when a finished feasible order exists. |
| `empty_road_distance_m` | nullable | Present only when measured. Not numeric zero for unknown. |
| `total_duration_seconds` | nullable | Present only when measured. |
| `utilization` | nullable | Present only when every consumed dimension is known. |
| `target_distance_m` | nullable | Present only when the seed has an explicit target and the leg is measured. |
| `revenue_amount` / `revenue_currency` | nullable | Present only when both loads expose comparable authoritative revenue. Missing revenue is omitted, not zero. |
| `omitted_components` | YES | Explicit list such as utilization, target, revenue. |
| `reason_codes` | YES | Machine codes without foreign commercial text. |
| `route_plan_id` | nullable | Set only after a durable route-plan evaluation is stored. |
| `candidate_fingerprint` | YES | Provenance of the pair and selected order inputs. |
| `created_at` | YES | Materialization time. |

Private foreign shipper identity, customer fields, contract rates, private commercial terms, and foreign internal shipment ids are not columns on this table and are not allowed in reason text.

A separate `candidate_metrics` table is not selected. The metric columns above are 1:1 with the shortlist row and stay on `chain_candidates`.

## C. Route-plan boundary

```text
ROUTE_PLAN_ID_ONLY_AFTER_DURABLE_EVALUATION=YES
CHAIN_CANDIDATE_TO_ROUTE_PLAN=OPTIONAL
TMS_EXECUTION_OWNER=YES
NLO_EXECUTION_OWNER=NO
EXECUTION_SUPPORTED=false
```

`route_plan_id` remains null for unevaluated, unfinished, or rejected candidates. When an evaluated order is persisted, the existing route-plan tables own stops, actions, legs, and capacity snapshots. The chain candidate stores only the foreign key. Chain persistence does not accept, activate, or hand a plan to TMS. That boundary stays NLO-0.9D.

## D. Durable idempotency

```text
DURABLE_IDEMPOTENCY_CONTRACT_FROZEN=YES
IDEMPOTENCY_SCOPE=TENANT_PLUS_KEY
```

Conceptual entity `chain_search_idempotency`:

| Field | Required |
| --- | --- |
| `tenant_id` | YES |
| `idempotency_key` | YES |
| `request_fingerprint` | YES |
| `search_id` | YES |
| `created_at` | YES |

Behavior:

| Case | Result |
| --- | --- |
| Same tenant + same key + same fingerprint, terminal search | Replay the same stored terminal resource and body. |
| Same tenant + same key + same fingerprint, unexpired `PENDING`/`RUNNING` | HTTP `409`, code `CHAIN_SEARCH_IN_PROGRESS`, `Retry-After: 1`, and the tenant-owned `search_id`. No duplicate compute and no blocking wait. |
| Same tenant + same key + same fingerprint, expired `PENDING`/`RUNNING` | Atomically terminalize as `FAILED` / `CHAIN_INTERRUPTED`, then replay that terminal resource with HTTP `200`. No recomputation. |
| Same tenant + same key + different fingerprint | Conflict. No overwrite. |
| Same key in another tenant | Independent scope. New search allowed. |
| New key | New `chain_search_id` even when seed and policy match. |

Unique constraint: `(tenant_id, idempotency_key)`. The stored fingerprint detects conflict. Replay returns the terminal search already linked by `search_id`; it never allocates replacement ids or restarts cancelled/failed work.

### Atomic claim, ownership, and crash recovery

1. Authenticate, derive tenant, validate the seed, and resolve canonical fingerprint inputs before claiming. In one transaction insert the `PENDING` search and its idempotency row with identical tenant, search id, and request fingerprint. Allocate the owner token and database-clock timestamps in that transaction. Commit both or neither. No provider or route-plan computation may start before this commit.
2. On a unique-key collision, roll back the attempted search insert and read the winning tenant/key row. A different fingerprint returns conflict without overwriting it. A matching fingerprint follows the terminal/in-progress/expired cases above. Do not leave an orphan search for the losing request.
3. Only the owner may atomically transition `PENDING` → `RUNNING`, using tenant, search id, owner token, expected status, and database time before `deadline_at`. Every candidate write, route-plan link, and terminal write must lock/check the parent in the same transaction. Owner writes require that owner token, a nonterminal expected state, and database time before `claim_expires_at`; the recovery transaction in step 4 instead requires an expired nonterminal state. New computation stops at `deadline_at`; the final five seconds permit only bounded persistence/cleanup. A failed ownership/status/expiry check discards late output, including any uncommitted route-plan linkage.
4. A periodic recovery sweep, a startup sweep, and lookup of an expired claim use a conditional transaction/row lock to change only expired `PENDING`/`RUNNING` rows to `FAILED` / `CHAIN_INTERRUPTED`. The idempotency mapping is retained. GET for a tenant-owned expired claim performs this terminalization before returning it; the candidates endpoint returns HTTP `200` with an empty candidate list for nonterminal, cancelled, or failed searches and exposes a shortlist only for `COMPLETED`. Recovery is idempotent, starts no provider work, and never takes over computation. A database outage returns `503`; durable recovery resumes when storage is available.
5. Cancellation is persisted with a separate bounded cleanup context, within the same owner/expiry fencing rules, rather than using the already-cancelled request context. If cancellation commits first, recovery and late completion cannot overwrite `CANCELLED`. If the process dies before recording cancellation, expiry produces `FAILED` / `CHAIN_INTERRUPTED`; do not falsely claim knowledge of the disconnect cause. After expiry the old owner cannot write, even if the sweep has not yet run.

The terminal row and its public candidate projection commit atomically, so replay cannot see `COMPLETED` with a partially committed shortlist. Concurrent completion, cancellation, and recovery serialize on the parent row; at most one terminal transition wins. No lease renewal, takeover, automatic compute retry, or cancellation retry is authorized. Polling/replaying an existing resource is not a compute retry.

## E. Request fingerprint

```text
REQUEST_FINGERPRINT_FROZEN=YES
```

Included:

- carrier tenant identity
- seed source
- capacity id and version, or current-trip shipment id and version
- horizon anchor inputs used to derive the 48h window (manual `available_from` / `available_until` when present; otherwise the search clock policy marker)
- chain `policy_version`
- resolved service-duration policy identity/version and the canonical effective duration values (including explicit unknown markers) used by the run
- `algorithm_version`
- publication visibility context fingerprint already used to authorize the load pool
- deterministic filter set that the server applied (not client-invented free-form filters outside the frozen seed)

Excluded:

- response-only metrics
- provider call counts
- wall-clock timestamps that would make replays diverge for the same logical request
- client-supplied chain id (rejected before fingerprinting)
- predicted-capacity seed fields (rejected)

Resolve service-duration provenance on each POST before fingerprint comparison. A change in policy identity, version, known/unknown state, or effective duration changes the fingerprint: reusing the same tenant/key then returns `409`; a new key creates a new search. An unchanged fingerprint replays the stored result without recomputation. This preserves the I0 `TestCHAIN17ServiceDurationChangesRequestHash` distinction; there is no silent first-claim exception for changed durations.

Store the resolved snapshot with the claim. Chain evaluation and route-plan preparation must consume that same immutable snapshot, never independently re-read an active policy during this run. If the later route-plan integration cannot use the snapshot, it must reject a changed policy before evaluation rather than mix durations under one fingerprint. Unknown durations remain unknown; zero is a distinct known value.

## F. Catalog / policy / algorithm fingerprint (CHAIN-17)

```text
CATALOG_POLICY_ALGORITHM_FINGERPRINT_FROZEN=YES
CHAIN_17_ARCHITECTURE_CLOSED=YES
```

Stored on every search run:

| Field | Rule |
| --- | --- |
| `catalog_fingerprint` | Compatibility catalog / rule-set fingerprint used for groupage during the run. |
| `policy_version` | Chain policy version, for example the I0 policy marker evolved by the persistence stage. |
| `algorithm_version` | Bounded two-order comparison algorithm version. |
| Service-duration provenance | Policy identity/version plus canonical effective values/unknown markers stored in section A and included in the request fingerprint. |
| `routing_provider_version` | Proven provider build/version when available; otherwise the literal `UNKNOWN`. |

Purpose: historical reproducibility of what policy and catalog governed the shortlist. This is not a guarantee of exact binary route replay. An empty string must not mean “known empty”. If the provider version cannot be proven, store `UNKNOWN`.

## G. Cancellation persistence

```text
CANCELLATION_PERSISTENCE_FROZEN=YES
PARTIAL_RESULTS_PUBLICLY_RETURNED_AFTER_CANCEL=NO
CHAIN_REQUEST_CANCEL_RETRY=NO
```

When the client or gateway cancels the request context:

1. No new provider call or route-plan evaluation starts.
2. Status becomes `CANCELLED`.
3. `cancelled_at` is set.
4. `cancel_reason_class` is one of `CLIENT_CANCELLED`, `GATEWAY_CANCELLED`, or `CONTEXT_CANCELLED` when the exact layer is unknown.
5. `search_complete` stays false.
6. No retry is scheduled.

Partial shortlist rows already materialized may remain for tenant-scoped audit. They are not returned as a successful public shortlist after cancel. A cancelled search must not be rewritten to `COMPLETED`. Cancellation persistence and abandoned-claim recovery follow the ownership and expiry rules in section D. Unfinished candidates stay `UNFINISHED` / infeasible for selection.

Budget exhaustion remains `FAILED` with `failure_code=CHAIN_BUDGET_EXHAUSTED`, not `CANCELLED`.

## H. Public API contract

```text
PUBLIC_API_CONTRACT_FROZEN=YES
PUBLIC_CHAIN_ENDPOINT_ACTIVE=NO
CHAIN_API_EXECUTION_MODEL=SYNCHRONOUS_BOUNDED
```

Conceptual endpoints (not implemented in this stage):

| Method | Path | Role |
| --- | --- | --- |
| `POST` | `/api/v1/network/chain-searches` | Create or replay a search. Synchronous bounded compute. |
| `GET` | `/api/v1/network/chain-searches/{searchId}` | Read one tenant-owned search. |
| `GET` | `/api/v1/network/chain-searches/{searchId}/candidates` | Read the durable shortlist for that search. |

Gateway prefix stays `/api/v1/network`. Internal service prefix stays `/v1/network`. Auth remains the existing carrier JWT roles already allowed for network search, plus platform admin. Tenant is server-derived.

`POST` body carries the seed only. `candidate_load_ids`, L1, or L2 are rejected. Response must complete inside the existing 20s compute budget and before the gateway 30s write timeout.

### Status and error mapping

Aligned to current network-optimizer conventions where they exist:

| HTTP | When |
| --- | --- |
| `200` | Idempotent replay of an existing terminal search, including an expired claim terminalized as `FAILED` / `CHAIN_INTERRUPTED`. Successful GET of a tenant-owned resource. |
| `201` | New search created and finished in-request with a terminal status. |
| `400` | Malformed JSON or missing required header shape. |
| `401` | Unauthenticated. |
| `403` | Authenticated but wrong role; authorization is decided independently of resource existence. Never used to distinguish another tenant’s search id. |
| `404` | On both GET endpoints, unknown and other-tenant `searchId` have the same status, error code, and response shape after tenant-scoped lookup. Existence of another tenant’s id is not disclosed. |
| `409` | Key reused with a different fingerprint (conflict), or matching unexpired in-flight claim (`CHAIN_SEARCH_IN_PROGRESS`, `Retry-After: 1`, section D). Neither starts new computation. |
| `422` | Policy refusal or domain validation (predicted capacity seed, client-selected pair, invalid seed reference). Existing NLO handlers already use 422 for domain validation. |
| `503` | Routing provider or required dependency unavailable when the search cannot produce a durable terminal result. |

Cancellation and timeout gap:

```text
HTTP_499_ESTABLISHED_IN_REPO=NO
HTTP_408_ESTABLISHED_FOR_CHAIN=NO
```

The repository does not currently standardize HTTP `499` or chain-specific `408`. This freeze does not invent those codes as required public contract. Persistence still records `CANCELLED` when the request context ends. If the response can still be written, the handler may return the cancelled search body with `200`/`201` and status `CANCELLED`. If the connection is already gone, only the durable row remains. Exact wire mapping for mid-flight disconnect stays an implementation detail of the later activation stage and must not weaken the durable `CANCELLED` semantics.

## I. Security and privacy

```text
TENANT_SERVER_DERIVED=YES
CLIENT_TENANT_AUTHORITY=NO
CROSS_TENANT_SEARCH_READ=DENY
PRE_ACCEPT_PRIVACY_PRESERVED=YES
```

- Tenant comes from gateway JWT claims, never from a client-trusted body or header override.
- A carrier may read only searches and candidates in its own tenant.
- Both GET endpoints use tenant-scoped lookup and indistinguishable `404` errors for unknown and other-tenant ids. Role-based `403` is evaluated independently of resource existence.
- Pre-accept projection continues ADR-NET-015 and the 0.9B privacy surface: publication-visible load references plus carrier-owned route metrics only.
- Forbidden in durable columns and public JSON: foreign shipper internal ids, private customer fields, contract rates, private commercial terms, provider secrets, and foreign shipment ids not already publication-visible.

## J. Schema design (no migration)

```text
SCHEMA_DESIGN_FROZEN=YES
MIGRATION_NUMBER=UNASSIGNED
DO_NOT_RESERVE_000097=YES
```

Future tables live in the network-optimizer schema. Names are conceptual.

### `chain_search_runs`

- PK: `id`
- FK: none required beyond tenant ownership
- Unique: `(tenant_id, id)` and `(tenant_id, id, request_fingerprint)` as composite FK targets
- Indexes: `(tenant_id, created_at DESC)`, `(tenant_id, status)`, `claim_expires_at` for nonterminal recovery
- Required internal claim fields: `owner_token`, `claim_expires_at`, and the service-duration snapshot/provenance in section A
- Check: `deadline_at = created_at + 20s`; `claim_expires_at = deadline_at + 5s`; fingerprint non-empty
- Check: status in the five frozen values; `global_optimum = false`; `execution_supported = false`

### `chain_candidates`

- PK: `candidate_id`
- Composite FK: `(tenant_id, search_id)` → `chain_search_runs(tenant_id, id)` ON DELETE CASCADE
- Nullable composite FK: `(tenant_id, route_plan_id)` → `network_optimizer.route_plans(tenant_id, id)` ON DELETE RESTRICT, using MATCH SIMPLE so null `route_plan_id` is permitted
- Unique: `(search_id, lower_load_id, higher_load_id, first_load_version, second_load_version)`
- Unique: `(search_id, ordinal)`
- Indexes: `(tenant_id, search_id)`, `(search_id, ordinal)`
- Check: non-null `route_plan_id` requires finished feasible evaluation; the composite FK proves plan existence and tenant match. Metric nullability as above

### `chain_search_idempotency`

- PK: `(tenant_id, idempotency_key)`
- Composite FK: `(tenant_id, search_id, request_fingerprint)` → `chain_search_runs(tenant_id, id, request_fingerprint)` ON DELETE RESTRICT; all three columns NOT NULL
- Unique: PK above
- Indexes: `(search_id)`
- Check: fingerprint non-empty

Supporting route-plan key: the later authorized migration must add `UNIQUE (tenant_id, id)` on the existing `network_optimizer.route_plans` table before creating the candidate FK. An ordinary index is not a substitute. This document changes no existing migration or table now.

The idempotency composite FK enforces tenant and fingerprint equality with its search at database level, in addition to the atomic claim transaction in section D. Candidate and route-plan links cannot cross tenants even if application validation is bypassed. The nullable route-plan FK uses the search's non-null tenant, not an independently supplied plan tenant.

Optional `candidate_metrics` table: rejected. Metrics stay on `chain_candidates`.

`match_candidates` is not redefined as a chain pair row.

## K. Retention

```text
CHAIN_SEARCH_RECORD_CLASS=AUDITABLE_OPERATIONAL_RECORD
CHAIN_SEARCH_EPHEMERAL_CACHE=NO
AUTOMATIC_TTL_DELETE=NO
RETENTION_DURATION=TBD
```

Search runs and shortlists are auditable operational records for the owning tenant. They are not an ephemeral cache. Automatic deletion without an explicit retention policy is forbidden. Retention duration remains policy TBD. Tenant deletion removes the tenant’s rows with the owning tenant.

## L. Future tests

All rows below are `FUTURE_REQUIRED` until a persistence implementation stage is authorized and executed.

| ID | Expectation | Status |
| --- | --- | --- |
| CHAIN-PERSIST-01 | Creating a search persists a `chain_search_runs` row with server id and fingerprints. | FUTURE_REQUIRED |
| CHAIN-PERSIST-02 | Same tenant, key, and fingerprint replay the same search id and body. | FUTURE_REQUIRED |
| CHAIN-PERSIST-03 | Same tenant and key with a different fingerprint conflict and do not overwrite. | FUTURE_REQUIRED |
| CHAIN-PERSIST-04 | Cross-tenant reads and writes are denied; idempotency keys do not collide across tenants. | FUTURE_REQUIRED |
| CHAIN-PERSIST-05 | Durable candidate ordinals match the lexicographic shortlist order. | FUTURE_REQUIRED |
| CHAIN-PERSIST-06 | `route_plan_id` is null until a durable evaluation exists. | FUTURE_REQUIRED |
| CHAIN-PERSIST-07 | Cancellation persists status `CANCELLED`, timestamp, and reason class. | FUTURE_REQUIRED |
| CHAIN-PERSIST-08 | A cancelled search is never rewritten to `COMPLETED`. | FUTURE_REQUIRED |
| CHAIN-PERSIST-09 | Catalog, policy, and algorithm fingerprints are stored on the run. | FUTURE_REQUIRED |
| CHAIN-PERSIST-10 | Missing provider version is stored as `UNKNOWN`, not empty-as-known. | FUTURE_REQUIRED |
| CHAIN-PERSIST-11 | Private foreign fields are not persisted or exposed improperly. | FUTURE_REQUIRED |
| CHAIN-PERSIST-12 | Both GET endpoints return indistinguishable `404` for unknown and other-tenant ids; role `403` is independent of id existence. | FUTURE_REQUIRED |
| CHAIN-PERSIST-13 | Partial failure or budget exhaustion does not claim `COMPLETED`. | FUTURE_REQUIRED |
| CHAIN-PERSIST-14 | Deadline or cancellation stops new expensive work. | FUTURE_REQUIRED |
| CHAIN-PERSIST-15 | After process restart, a completed search remains readable by id. | FUTURE_REQUIRED |
| CHAIN-PERSIST-16 | Crash before claim commit leaves neither row; crash after commit recovers expired `PENDING`/`RUNNING` to immutable `FAILED` / `CHAIN_INTERRUPTED`, without recomputation. | FUTURE_REQUIRED |
| CHAIN-PERSIST-17 | Concurrent matching POSTs create one search/claim, return in-flight `409` promptly, and replay the same terminal body after completion. | FUTURE_REQUIRED |
| CHAIN-PERSIST-18 | Late owner writes fail after expiry or terminalization; concurrent cancellation/completion/recovery yield one immutable terminal state and atomic shortlist visibility. | FUTURE_REQUIRED |
| CHAIN-PERSIST-19 | Direct SQL cross-tenant candidate/search, idempotency/search, and candidate/route-plan links fail; a same-tenant idempotency fingerprint mismatch also fails. | FUTURE_REQUIRED |
| CHAIN-PERSIST-20 | Changed duration policy identity/version/values or unknown state changes the fingerprint and conflicts for the same key; chain and route-plan evaluation use one stored snapshot. | FUTURE_REQUIRED |
| CHAIN-PERSIST-21 | Cancelled request context still permits bounded cancellation persistence; a crash before that write recovers as interrupted without inventing a cancellation cause. | FUTURE_REQUIRED |

Related earlier tests that stay blocked on activation or durable storage: `CHAIN-HTTP-01`, public authenticated read parts of `CHAIN-05`, durable fingerprint proofs in `CHAIN-17`, durable route-plan linkage in `CHAIN-18`, and durable idempotency in `CHAIN-22`.

## M. Non-goals

This stage does not:

- create a migration or choose `000097`
- write Go SQL repositories beyond the existing in-memory test double
- activate public routes or change OpenAPI under `packages/openapi`
- change the API gateway timeout or proxy
- deploy staging
- accept or activate a chain
- mutate shipment or TMS execution

## N. Readiness

```text
READY_FOR_PERSISTENCE_IMPLEMENTATION_REVIEW=YES
PERSISTENCE_IMPLEMENTATION_AUTHORIZED=NO
NLO_0_9C_COMPLETE=NO
```

A later implementation stage may create the schema under a newly assigned migration number, wire the repository port to PostgreSQL, and activate the public routes only after separate authorization.
