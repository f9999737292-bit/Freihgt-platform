# NLO-0.9A chain security, persistence, and API

This document records current privacy, storage, API, explanation, and handoff evidence. It does not add a migration, an OpenAPI path, or a visibility rule in code.

```text
NLO_0_9A_MODE=DISCOVERY_DOCS_ONLY
CHAIN_RUNTIME_IMPLEMENTED=NO
IMPLEMENTATION_AUTHORIZED=NO
OPENAPI_CHANGE_IN_0_9A=NO
PUBLIC_CHAIN_ENDPOINT_AUTHORIZED=NO
MIGRATION_AUTHORIZED=NO
```

Search model: [NLO_0_9A_CHAIN_SEARCH_MODEL.md](NLO_0_9A_CHAIN_SEARCH_MODEL.md). Blockers: [NLO_0_9A_CHAIN_OPTIMIZER_DISCOVERY.md](NLO_0_9A_CHAIN_OPTIMIZER_DISCOVERY.md).

NLO-0.9B supersedes the unfrozen privacy surface, persistence choice, API path, and execution boundary in this document. Current visibility and storage facts remain. See [NLO_0_9B_CHAIN_ARCHITECTURE_FREEZE.md](NLO_0_9B_CHAIN_ARCHITECTURE_FREEZE.md).

```text
NLO_0_9A_POLICY_VALUES_SUPERSEDED_BY=NLO-0.9B
RUNTIME_IMPLEMENTATION_AUTHORIZED=NO
```

## Privacy and tenancy

```text
CHAIN_CROSS_SHIPPER_PRIVACY_GAP_PRESENT=YES
BLOCKER_CHAIN_005=CROSS_SHIPPER_PRIVACY
FAIL_CLOSED=YES
```

A load that is visible by itself does not authorize publication of a combined chain that includes another shipper’s operational data.

Current next-load visibility is the marketplace rule: published, not owned by the viewer, and either marketplace, anonymized marketplace, or invited for that carrier company. Current route-plan visibility is stricter for a foreign load. `routeLoadVisible` requires `CrossShipperConsolidationAllowed` when `OwnerTenantID` is not the actor tenant, and then applies marketplace, anonymized, network-only, or invited scope. Same-tenant loads are visible on this path when consolidation is allowed.

`prepareRoutePlan` collects load owner tenants. When more than the actor tenant is present, and the catalog cannot supply a same-owner evaluation context, `ReferenceUnavailable` is set. The planner then adds `MULTI_PARTY_REFERENCE_CONTEXT_UNAVAILABLE`. The result is indeterminate. Accept and activate reject a plan that is not feasible, and activate also rejects an indeterminate compatibility snapshot.

```text
DISCOVERY_RESULT_VISIBILITY
!=
ACCEPTED_EXECUTION_VISIBILITY
```

A pre-accept chain result must not expose:

- another shipper’s commercial terms
- another shipper’s contracts
- another shipper’s customer identity
- internal shipment ids
- exact stop details that are not required for the carrier decision

Use the existing publication scopes and the existing cross-shipper opt-in. Do not invent a new visibility model in this discovery. If the required multi-party reference context is unavailable, the result fails closed. It is not stored as feasible and it is not described as feasible.

The joint-sequence gap remains: two individually visible loads can still reveal, once planned together, that they share an order, with both canonical stops and windows on the plan. Anonymized one-load search hiding an exact deadhead does not anonymize that joint plan. `CHAIN_PRIVACY_SURFACE` is a required NLO-0.9B decision.

## Persistence

```text
ONE_LOAD_SEARCH_PERSISTENCE_PRESENT=YES
MULTI_LOAD_ROUTEPLAN_PERSISTENCE_PRESENT=YES
CHAIN_SEARCH_RUN_PERSISTENCE_PRESENT=NO
CHAIN_CANDIDATE_PERSISTENCE_PRESENT=NO
CAN_EXISTING_ROUTE_PLAN_PERSIST_TWO_LOADS=YES
CAN_EXISTING_MATCH_CANDIDATE_ROW_REPRESENT_CHAIN=NO
CHAIN_DISCOVERY_NEW_MIGRATION_REQUIRED=CONDITIONAL
CHAIN_DISCOVERY_AUDIT_REQUIRES_NEW_SCHEMA=CONDITIONAL
DEDICATED_CHAIN_IDENTITY_PRESENT=NO
ROUTE_PLAN_ID_IS_PRE_EVALUATION_CHAIN_IDENTITY=NO
```

`next_load_search_runs` and `match_candidates` persist one-load search. Each match candidate row points at one `load_opportunity_id`. That row cannot represent L1 plus L2.

`route_plans`, `route_plan_stops`, `route_stop_actions`, `route_plan_legs`, and `route_capacity_snapshots` persist an evaluated plan. Actions store subject type, subject id, and subject version, so two load subjects can be stored after evaluation. `route_plan_id` is that evaluated identity. It is not a discovery-candidate identity.

Whether a later chain search run and its rejected pairs need a new audit schema is conditional. This discovery does not design SQL, does not assign a migration number, and does not add tables. `BLOCKER_CHAIN_003=CHAIN_IDENTITY_AND_DISCOVERY_PERSISTENCE` stays open until NLO-0.9B freezes `CHAIN_IDENTITY` and `CHAIN_DISCOVERY_PERSISTENCE`.

Version facts already pinned on an evaluated plan, and already able to cover two load subjects:

```text
LOAD_VERSION_PINNING_PRESENT=YES
CAPACITY_VERSION_PINNING_PRESENT=YES
CURRENT_TRIP_CONTEXT_FINGERPRINT_PRESENT=YES
SERVICE_DURATION_POLICY_VERSION_PRESENT=YES
ROUTING_PROOF_PRESENT=YES
COMPATIBILITY_FINGERPRINT_PRESENT=YES
RULE_FINGERPRINT_PRESENT=YES
CATALOG_FINGERPRINT_PRESENT=YES
ALGORITHM_POLICY_VERSION_PRESENT=YES
MULTI_LOAD_VERSION_PINNING_ALREADY_SUPPORTED=YES
```

`DEPOT_START` pins capacity id and version and forbids a shipment id. `CURRENT_TRIP` pins shipment id and version and forbids a capacity id. The evaluation fingerprint includes context, catalog, rule, service-duration policy, each action subject version, and each leg request and response fingerprint. Those pins belong to the evaluated plan. They do not create a chain-search audit.

## API

Current route-plan evaluate accepts `candidate_load_ids`. The server enforces length 1 or 2. The caller chooses the ids. That operation is evaluation.

```text
ROUTE_PLAN_API_ACCEPTS_CANDIDATE_LOAD_IDS_ARRAY=YES
MAX_REQUESTED_LOAD_IDS_ENFORCED=2
CALLER_CAN_PASS_TWO_LOAD_IDS=YES
CHAIN_LOAD_IDS_SERVER_DISCOVERED=NO
CHAIN_LOAD_IDS_CURRENTLY_CALLER_SELECTED=YES
```

Discovery recommendation:

```text
RECOMMENDED_API_SHAPE=SEPARATE_CHAIN_SEARCH_RESOURCE
CHAIN_API_PATH=TBD
CHAIN_NEW_PUBLIC_API_REQUIRED=CONDITIONAL
OPENAPI_CHANGE_IN_0_9A=NO
PUBLIC_CHAIN_ENDPOINT_AUTHORIZED=NO
```

A separate chain-search resource keeps one-load search as a one-load response. Extending the next-load route to return pairs would change that contract. Extending route-plan evaluate would keep the caller as the selector. The exact path is not chosen here. No OpenAPI file changes in NLO-0.9A. No public chain endpoint is authorized.

`API_RESOURCE_SHAPE` remains a required NLO-0.9B decision, including whether the first runtime is public or internal.

## Explanation

```text
CHAIN_EXPLANATION_PRESENT_NOW=NO
```

A later chain result needs an explanation distinct from the one-load score explanation. This discovery requires the categories only. It does not design the DTO.

Required categories:

- why L1 entered the shortlist
- why L2 was chained after L1
- visibility outcome
- compatibility outcome
- route feasibility outcome
- capacity outcome
- time-window outcome
- ranking inputs
- reject reasons
- dependency versions and fingerprints

The explanation served before accept follows `DISCOVERY_RESULT_VISIBILITY`. It must not include another shipper’s commercial terms, contracts, customer identity, or internal shipment ids.

## Execution handoff

```text
MULTI_LOAD_ROUTE_PLAN_ACCEPT_SUPPORTED=YES
MULTI_LOAD_ROUTE_PLAN_ACTIVATE_SUPPORTED=CONDITIONAL
MULTI_LOAD_EXECUTION_HANDOFF_SUPPORTED=CONDITIONAL
TMS_CANONICAL_EXECUTION_OWNER=YES
CHAIN_DISCOVERY_SUCCESS_DOES_NOT_PROVE_EXECUTION_HANDOFF=YES
EXECUTION_SUPPORTED_ON_CURRENT_PLANNER_RESULT=false
```

`AcceptRoutePlan` can accept a feasible plan. The accept path does not limit the plan to one load. `execution_supported` on the stored plan remains false.

`ActivateRoutePlan` can move an accepted current-trip plan to `PENDING_EXECUTION` when the result is feasible, service duration is known, and compatibility snapshots are not indeterminate. `finishActivationHandoff` runs after that transaction and projects to shipment-service. The projection requires `shipment_id`. A depot-start plan does not have one, so this handoff does not cover depot-start. Execution subjects are built from shipment-cargo actions. Load-opportunity actions are included in the action list and are not execution subjects.

The current planner result `ExecutionSupported=false` stays in this record. A later runtime must not treat a discovered chain as executable until the full accept, activate, and acknowledgement path is proven for that chain shape. TMS remains the canonical execution owner. NLO-0.9A does not change TMS.

```text
EXECUTION_SUPPORT_BOUNDARY=TBD
```

That boundary is a required NLO-0.9B decision.

## Analytics

```text
GAP_D_001=OPEN
GAP_D_003=OPEN
GAP_D_004=OPEN
GAP_D_005=OPEN
CHAIN_0_9A_MUST_CLOSE_ANALYTICS_GAPS=NO
```

NLO-0.9A does not close these gaps and does not edit analytics documents.
