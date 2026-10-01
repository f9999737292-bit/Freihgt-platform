# NLO-0.5 implementation roadmap

NLO-0.5A is this docs freeze. Implementation is not authorized.

```text
NLO_0_5A=ARCHITECTURE_FROZEN_PENDING_CONTROLLER_REVIEW
NLO_0_5_IMPLEMENTATION_AUTHORIZED=NO
NLO_0_6_STARTED=NO
NLO_0_9_STARTED=NO
NLO_1_0_STARTED=NO
```

## Waves

| Wave | Content | Depends on |
| --- | --- | --- |
| NLO-0.5A | This freeze | NLO-0.4 on main `00b53de9` |
| NLO-0.5B | Backhaul search over the existing next-load pipeline, one load, corridor and ellipse, no new mode | Controller acceptance, benchmark numbers for candidate cap and time budget |
| NLO-0.5C | Promote an accepted backhaul candidate through the existing route plan, service-duration policy, activation, and TMS handshake | NLO-0.5B |
| Later | Chain search and live reoptimization | NLO-0.9 |

NLO-0.5B does not change the 2GIS adapter, shipment status transitions, billing, EDO, or Control Tower. It does not add a migration unless a later contract proves a new stored field that the current search run cannot hold.

## Benchmark classes

No production scale is claimed.

| Class | Purpose |
| --- | --- |
| SMALL | One capacity, a handful of visible loads, one matrix batch |
| MEDIUM | Enough loads to cross the 25-target batch boundary |
| LARGE | A visible marketplace page that must still stop at the benchmark cap |

Measure candidate count, matrix calls, routing latency, total search latency, determinism, cache hit ratio, hard-reject counts, and top-N stability. Provider latency is the configured adapter or an explicit fake. A fake must not be reported as 2GIS time.

## Future metrics

Names only, not implemented here: `backhaul_search_total`, `backhaul_candidate_prefilter_count`, `backhaul_routed_candidate_count`, `backhaul_feasible_count`, `routing_matrix_calls_total`, `routing_failures_total`, `routing_latency`, `deadhead_km_saved`, `loaded_km_ratio`, `return_direction_score`, `search_duration`.

## Carried blockers

Q4 stays open. Matrix-call cap, search time budget, routing SLA, and provider commercial terms are not invented here. Exact road distance on anonymized loads stays bucketed.
