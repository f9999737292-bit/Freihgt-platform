# NLO-0.5B test strategy

Specified for a later implementation wave. This wave runs only the local accounting and prefilter tests.

| Gate | Expected |
| --- | --- |
| Deterministic cap | Discovery stops at 1,000 and routing stops at 25 |
| Deadline | The 30 second watchdog starts no further provider call |
| Provider ceiling | Ellipse issues at most 4 matrix calls and 2 route calls |
| Partial matrix | Cells with errors stay unknown; successful cells remain |
| 429 | `ROUTING_INVALID_RESPONSE`, no retry, no Haversine |
| Timeout | `ROUTING_TIMEOUT`, search stops |
| No route | `ROUTE_NOT_FOUND` becomes `ROAD_DISTANCE_UNKNOWN` |
| Large pool | A 1,000 row fixture does not produce 160 matrix calls |
| Tenant isolation | A foreign private load is absent |
| Duplicates | One load id appears once |
| Ordering | Same inputs keep the existing score order, then load id |
| Pruning | Corridor rejects are not routed |
| Regression | Call accounting stays on the table in `NLO_0_5B0_BENCHMARK.md` |

`TestNLO05B0ProviderCallAccounting` and `TestNLO05B0LocalPrefilterTiming` cover the accounting table and the local prefilter. They do not call 2GIS.
