# ADR-NET-026: Chain search persistence and API contract

## Status

Accepted as a docs-only architecture freeze. Persistence implementation, migration, OpenAPI, and public route activation are not authorized by this ADR.

```text
ADR_NET_026=ACCEPTED_ARCHITECTURE_ONLY
NLO_0_9C_I1_STATUS=PERSISTENCE_API_CONTRACT_FROZEN
PRODUCT_IMPLEMENTATION_AUTHORIZED=NO
MIGRATION_CREATED=NO
MIGRATION_NUMBER=UNASSIGNED
OPENAPI_CHANGED=NO
PUBLIC_ROUTE_ACTIVATION_AUTHORIZED=NO
```

The decision body is [NLO_0_9C_I1_CHAIN_PERSISTENCE_DESIGN.md](../NLO_0_9C_I1_CHAIN_PERSISTENCE_DESIGN.md). This ADR does not change ADR-NET-015 privacy, ADR-NET-024 one-load budgets, or ADR-NET-025 search policy.

## Context

NLO-0.9B froze the two-load search policy and required a future search-run plus shortlist persistence model without assigning a migration. NLO-0.9C-I0 implemented the in-memory engine and an in-memory repository port. Durable idempotency, route-plan linkage, fingerprint provenance, cancellation rows, and public GET/POST resources remain undefined in schema and HTTP. CHAIN-17 needed an architectural close for catalog, policy, and algorithm fingerprints.

## Decision

```text
CHAIN_DISCOVERY_PERSISTENCE=SEARCH_RUN_AND_SHORTLIST
CHAIN_SEARCH_STATUSES=PENDING,RUNNING,COMPLETED,CANCELLED,FAILED
DURABLE_IDEMPOTENCY_CONTRACT_FROZEN=YES
REQUEST_FINGERPRINT_FROZEN=YES
CATALOG_POLICY_ALGORITHM_FINGERPRINT_FROZEN=YES
ROUTING_PROVIDER_VERSION_UNKNOWN_LITERAL=UNKNOWN
CANCELLATION_PERSISTENCE_FROZEN=YES
PARTIAL_RESULTS_PUBLICLY_RETURNED_AFTER_CANCEL=NO
ROUTE_PLAN_ID_ONLY_AFTER_DURABLE_EVALUATION=YES
PUBLIC_API_PATHS=POST_/api/v1/network/chain-searches,GET_/api/v1/network/chain-searches/{searchId},GET_/api/v1/network/chain-searches/{searchId}/candidates
CHAIN_API_EXECUTION_MODEL=SYNCHRONOUS_BOUNDED
TENANT_SERVER_DERIVED=YES
CROSS_TENANT_SEARCH_READ=DENY
TMS_EXECUTION_OWNER=YES
NLO_EXECUTION_OWNER=NO
MIGRATION_NUMBER=UNASSIGNED
```

Budget exhaustion persists as `FAILED` with `failure_code=CHAIN_BUDGET_EXHAUSTED`. Cancellation is a separate terminal status with no retry. Missing revenue and unknown road distance stay omitted or excluding facts, not numeric zero. `000097` is not reserved.

## Consequences

- A later persistence implementation may open a migration under a newly assigned number and must match this contract.
- Public routes stay inactive until separately authorized.
- Exact HTTP mapping for mid-flight disconnect does not invent repository-wide `499`/`408` requirements; durable `CANCELLED` remains mandatory.
- NLO-0.9C remains incomplete until persistence and public activation stages finish under their own authorizations.
- NLO-0.9D execution handoff remains unauthorized.
