# ADR-NET-024: Bounded backhaul search policy

## Status

Accepted for the bounded next-load search. Backhaul chain runtime and roundtrip runtime are not implemented.

```text
ADR_NET_024=ACCEPTED_FOR_BOUNDED_SEARCH
NLO_0_5B1_DISCOVERY_CAP_ENFORCED=YES
NLO_0_5B2_ROUTING_BUDGET_ENFORCED=YES
FINAL_EVALUATION_CAP_RUNTIME_ENFORCED=NO
BACKHAUL_RUNTIME_IMPLEMENTED=NO
ROUNDTRIP_RUNTIME_IMPLEMENTED=NO
EXTERNAL_PROVIDER_BENCHMARK=BLOCKED
PROVIDER_SLA_PROVEN=NO
PROVIDER_COMMERCIAL_LIMITS=UNKNOWN
STAGING_PROVIDER_APPROVAL_NOT_IMPLIED=YES
```

NLO-0.5B1 enforces discovery on main at merge `349eedc89e0c47a71e3b23a107cc97fad5aebc91`. NLO-0.5B2 enforces the routing stage and provider budget. `CANDIDATE_FINAL_EVALUATION_CAP` stays frozen policy for a later scoring wave.

## Context

NLO-0.5A freezes one load toward `target_location_id`, using the existing corridor and four-leg route ellipse. Before B1, next-load search read up to 100,000 marketplace rows. Before B2, every cheap-prefilter survivor could be sent to the routing provider. At 1,000 ellipse survivors that shape is 160 matrix calls and 2 route calls.

The synchronous matrix page is 25 on each dimension. A 25 by 25 loaded-leg matrix is one call, not a sum of origins and destinations. The 2GIS client timeout is 5 seconds. Calls are sequential. There is no retry. Staging approval, a vendor SLA, and commercial limits are not established. A local prefilter sample is not provider latency.

## Decision

```text
CANDIDATE_DISCOVERY_CAP=1000
CANDIDATE_ROUTING_CAP=25
CANDIDATE_FINAL_EVALUATION_CAP=25
MATRIX_DIMENSION_LIMIT=25
MAX_MATRIX_CALLS_PER_SEARCH=4
MAX_ROUTE_CALLS_PER_SEARCH=2
MAX_PROVIDER_CALLS_TOTAL=6
PROVIDER_REQUEST_TIMEOUT=5s
SEARCH_HARD_BUDGET=30s
PARALLELISM=1
RETRY_COUNT=0
```

Discovery keeps `created_at DESC, id`, then the search sorts that page by load id. The routing cap keeps the first 25 cheap-prefilter survivors in that same load-id order. A client `candidate_limit` may shorten the HTTP response. It cannot raise the discovery cap or the routing cap.

The ellipse shape at 25 survivors is one deadhead matrix, two direction matrices, one loaded-leg matrix, and two release-to-target route calls. That is 4 matrix calls, 2 route calls, and 6 provider calls. Corridor and radius use fewer calls. The hard budget starts when the search starts. A provider call is not started when the context is canceled, the deadline has passed, or a call counter would be exceeded. The per-call timeout is the smaller of 5 seconds and the time left on the 30 second watchdog. Expiry does not cancel local persistence of facts already in hand.

A request-level timeout, unavailable provider, invalid response, or missing route does not retry and does not fall back to Haversine. Missing road terms stay `ROAD_DISTANCE_UNKNOWN`. A matrix response may contain both successful cells and cell errors. Successful cells remain. Failed cells stay unknown. Zero kilometres and straight-line estimates are not canonical road evidence.

`SEARCH_HARD_BUDGET` is a safety watchdog. It is not a 2GIS SLO. `PROVIDER_REQUEST_TIMEOUT` is the existing HTTP client timeout.

Logs and metrics may count candidates, pruned rows, matrix calls, route calls, budget exhaustion, provider errors, and total search time. They do not carry coordinates, location ids, addresses, customer labels, or provider secrets.

## Consequences

NLO-0.5B3 and later waves may add one-load feasibility, scoring persistence, or public exposure. They may not treat this policy as a completed backhaul product. Control Tower exposure, a new marketplace endpoint, a new score model, and a freight-cost ledger stay out of B2.
