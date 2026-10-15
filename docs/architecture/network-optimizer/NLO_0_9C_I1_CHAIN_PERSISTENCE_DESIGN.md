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

Base: `755f2a946f5fa746363935de25d0925e32fd9e90`.

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
| `started_at` | YES | Search clock when expensive work begins. |
| `completed_at` | nullable | Set when status becomes `COMPLETED` or `FAILED`. |
| `cancelled_at` | nullable | Set when status becomes `CANCELLED`. |
| `request_fingerprint` | YES | Hash of the frozen request inputs. |
| `policy_version` | YES | Chain policy version string. |
| `algorithm_version` | YES | Bounded two-order comparison algorithm version. |
| `catalog_fingerprint` | YES | Compatibility/catalog provenance used for the run. |
| `routing_provider_version` | YES | Proven provider version, or `UNKNOWN`. |
| `deadline_at` | YES | `started_at + 20s` wall-clock deadline. |
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
| `FAILED` | Search stopped without a complete result. Includes budget exhaustion and hard system/provider failure. `search_complete=false`. |

Mapping from I0 outcome codes:

| I0 / 0.9B outcome | Durable status | `failure_code` |
| --- | --- | --- |
| `SEARCH_COMPLETE` | `COMPLETED` | null |
| `CHAIN_BUDGET_EXHAUSTED` | `FAILED` | `CHAIN_BUDGET_EXHAUSTED` |
| `CANCELLED` | `CANCELLED` | null |
| Predicted capacity / invalid seed after claim | `FAILED` | policy code such as `PREDICTED_CAPACITY_EXCLUDED` |
| Idempotency fingerprint conflict | no new search row | HTTP `409` |

`PENDING` and `RUNNING` exist so a crash or restart can distinguish an unfinished claim from a finished audit row. The first public POST remains synchronous: the caller still waits for a terminal status in the same request whenever the connection stays open.

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
| Same tenant + same key + same fingerprint | Replay the same search resource and body. |
| Same tenant + same key + different fingerprint | Conflict. No overwrite. |
| Same key in another tenant | Independent scope. New search allowed. |
| New key | New `chain_search_id` even when seed and policy match. |

Unique constraint: `(tenant_id, idempotency_key)`. The stored fingerprint detects conflict. Replay returns the terminal search already linked by `search_id`.

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
- `algorithm_version`
- publication visibility context fingerprint already used to authorize the load pool
- deterministic filter set that the server applied (not client-invented free-form filters outside the frozen seed)

Excluded:

- response-only metrics
- provider call counts
- wall-clock timestamps that would make replays diverge for the same logical request
- client-supplied chain id (rejected before fingerprinting)
- predicted-capacity seed fields (rejected)

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

Partial shortlist rows already materialized may remain for tenant-scoped audit. They are not returned as a successful public shortlist after cancel. A cancelled search must not be rewritten to `COMPLETED`. Unfinished candidates stay `UNFINISHED` / infeasible for selection.

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
| `200` | Idempotent replay of an existing search. |
| `201` | New search created and finished in-request with a terminal status. |
| `400` | Malformed JSON or missing required header shape. |
| `401` | Unauthenticated. |
| `403` | Authenticated but wrong role, or cross-tenant read denied. |
| `404` | `GET` for unknown `searchId` inside the caller tenant, or not found after tenant filter. Existence of another tenant’s id is not disclosed. |
| `409` | Idempotency key reused with a different request fingerprint. Existing NLO handlers already use conflict for idempotency and stale plan cases. |
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
- Cross-tenant `GET` is deny / not-found under the caller tenant filter.
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
- Unique: none beyond PK
- Indexes: `(tenant_id, created_at DESC)`, `(tenant_id, status)`, `(tenant_id, id)`
- Check: status in the five frozen values; `global_optimum = false`; `execution_supported = false`

### `chain_candidates`

- PK: `candidate_id`
- FK: `search_id` → `chain_search_runs.id` ON DELETE CASCADE with tenant match
- Unique: `(search_id, lower_load_id, higher_load_id, first_load_version, second_load_version)`
- Unique: `(search_id, ordinal)`
- Indexes: `(tenant_id, search_id)`, `(search_id, ordinal)`
- Check: `route_plan_id` null unless feasibility finished and a stored plan exists; metric nullability as above

### `chain_search_idempotency`

- PK: `(tenant_id, idempotency_key)`
- FK: `search_id` → `chain_search_runs.id`
- Unique: PK above
- Indexes: `(search_id)`
- Check: fingerprint non-empty

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
| CHAIN-PERSIST-12 | `GET` for another tenant’s search is denied / not disclosed. | FUTURE_REQUIRED |
| CHAIN-PERSIST-13 | Partial failure or budget exhaustion does not claim `COMPLETED`. | FUTURE_REQUIRED |
| CHAIN-PERSIST-14 | Deadline or cancellation stops new expensive work. | FUTURE_REQUIRED |
| CHAIN-PERSIST-15 | After process restart, a completed search remains readable by id. | FUTURE_REQUIRED |

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
