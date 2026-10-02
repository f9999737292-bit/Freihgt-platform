# ADR-NET-024: Bounded backhaul search policy

## Status

Proposed. NLO-0.5B0. Implementation of backhaul is not authorized.

```text
ADR_NET_024=PROPOSED
BACKHAUL_RUNTIME_IMPLEMENTED=NO
ROUNDTRIP_RUNTIME_IMPLEMENTED=NO
CONTROLLER_REVIEW_REQUIRED=YES
```

## Context

NLO-0.5A freezes one load toward `target_location_id`, using the existing corridor and four-leg route ellipse. The current next-load search reads up to 100,000 marketplace rows and then routes every prefilter survivor. The synchronous matrix page is 25. The 2GIS client timeout is 5 seconds. Calls are sequential. Staging approval, a vendor SLA, and commercial limits are not in the repo.

A local prefilter of 1,000 synthetic points has P99 of 2 milliseconds. That does not price a provider call. An ellipse over 1,000 survivors is 162 provider calls and an 810 second timeout ceiling.

## Decision

First-release backhaul and roundtrip stay one load. The search bounds are:

```text
CANDIDATE_DISCOVERY_CAP=1000
CANDIDATE_ROUTING_CAP=25
CANDIDATE_FINAL_EVALUATION_CAP=25
MAX_MATRIX_POINTS_PER_CALL=25
MAX_MATRIX_CALLS_PER_SEARCH=4
MAX_ROUTE_CALLS_PER_SEARCH=2
MAX_PROVIDER_CALLS_TOTAL=6
SEARCH_SOFT_BUDGET_MS=5000
SEARCH_HARD_BUDGET_MS=30000
REQUEST_TIMEOUT_MS=5000
TOTAL_PROVIDER_BUDGET_MS=30000
PARALLELISM=1
RETRY_COUNT=0
```

Discovery 1,000 is the largest pool whose local prefilter was measured. That P99 is about 2 milliseconds of local projection time on one workstation. It is not search latency and not provider latency. Routing 25 is one synchronous matrix page. Final evaluation is the same 25 because scoring does not make another provider call. The four matrix calls and two route calls are the ellipse shape at that page: deadhead, two direction batches, one loaded-leg page, plus the two release-to-target routes.

```text
CAPS_RUNTIME_ENFORCED_NOW=NO
LOCAL_PREFILTER_LATENCY_ONLY=YES
PROVIDER_SLA_PROVEN=NO
PROVIDER_LATENCY_BENCHMARKED=NO
SEARCH_HARD_BUDGET_IS_SAFETY_WATCHDOG=YES
SEARCH_SOFT_BUDGET_IS_PROVIDER_SLO=NO
```

`REQUEST_TIMEOUT_MS` is the existing HTTP client timeout. `SEARCH_SOFT_BUDGET_MS` is that same timeout: one timed-out call ends the search. `SEARCH_HARD_BUDGET_MS` and `TOTAL_PROVIDER_BUDGET_MS` are 6 sequential calls times that timeout. They are a safety watchdog. They are not a 2GIS SLO.

The discovery cap keeps the existing marketplace order `created_at DESC, id`, then sorts that page by load id. It is not an unordered limit and it is not the 1,000 lowest load ids. Ranked output keeps the existing comparator: score descending, evidence descending, road kilometres ascending, then load id.

A provider timeout, 429, 5xx, invalid body, missing route, or open circuit does not retry and does not fall back to Haversine. The current adapter has no circuit breaker; if one is added, an open circuit uses this same fail-closed rule. The candidate is `ROAD_DISTANCE_UNKNOWN` unless every required road term is already present. A partial matrix keeps only cells without an error. When the hard budget is reached, no further call starts. Already complete candidates remain, in that same comparator.

Logs and metrics may count candidates, pruned rows, matrix calls, route calls, provider latency, total search time, timeouts, and degraded results. They do not carry coordinates, location ids, addresses, or provider secrets.

```text
PROVIDER_COMMERCIAL_LIMITS=UNKNOWN
PROVIDER_STAGING_APPROVAL=NOT_CONFIRMED
EXTERNAL_PROVIDER_BENCHMARK=BLOCKED
```

## Consequences

NLO-0.5B1 through NLO-0.5B4 may enforce these bounds on the existing next-load path. They may not start a chain search, a second compatibility engine, or a live reoptimization. Control Tower exposure stays a later wave.
