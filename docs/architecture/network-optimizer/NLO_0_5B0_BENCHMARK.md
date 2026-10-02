# NLO-0.5B0 benchmark

Local only. No 2GIS call. Synthetic coordinates around latitudes 10, 20, 30, and 40. No customer positions.

```text
BENCHMARK_MODE=LOCAL_PREFILTER_AND_CALL_ACCOUNTING
EXTERNAL_PROVIDER_BENCHMARK=BLOCKED
PROVIDER_TIME=NOT_MEASURED
DB_TIME=NOT_MEASURED
SERIALIZATION_TIME=NOT_SEPARATED
```

## Algorithm time

`TestNLO05B0LocalPrefilterTiming` projects each synthetic point onto a two-point line and computes straight-line distance. Thirty-one rounds. The slowest class is dense city at 1,000 points:

```text
P50_MS=1
P95_MS=2
P99_MS=2
```

Smaller pools are often below the clock resolution, so a zero microsecond sample means less than one millisecond, not zero work. Metro, regional highway, and sparse long-haul at 1,000 points are the same order. The published milliseconds are that local prefilter rounded from one workstation sample of 31 rounds, using index `floor((n-1)*q)`. The test does not assert those milliseconds. They are not NLO search latency.

## Call accounting

`TestNLO05B0ProviderCallAccounting` matches the current search shape. At 1,000 ellipse survivors the search would issue 160 matrix calls and 2 route calls. At 25 ellipse survivors it issues 4 matrix calls and 2 route calls.

## Profiles

These are benchmark profiles, not production presets. Recall against a real optimum is not measured.

| Profile | Routing cap | Ellipse provider calls | Timeout ceiling |
| --- | --- | --- | --- |
| LOW_COST | 10 | 5 | 25s |
| BALANCED | 25 | 6 | 30s |
| HIGH_QUALITY | 50 | 10 | 50s |

BALANCED is the proposed first-release ceiling because 25 is the synchronous matrix page already implemented. HIGH_QUALITY is not frozen: provider latency for the extra page was not measured.

## Budgets

```text
SEARCH_SOFT_BUDGET_MS=5000
SEARCH_HARD_BUDGET_MS=30000
REQUEST_TIMEOUT_MS=5000
TOTAL_PROVIDER_BUDGET_MS=30000
PARALLELISM=1
```

The soft budget is the existing per-call client timeout. The local prefilter is not the bottleneck, and no vendor percentile exists, so the soft stop is "a timed-out call ends the search". The hard budget is 6 calls times that timeout, which is the BALANCED ellipse shape. It is a watchdog, not a 2GIS SLO.

On the hard stop the search does not start another provider call, does not retry, and does not replace a missing road fact with Haversine. Candidates that already have complete road facts stay. The rest are `ROAD_DISTANCE_UNKNOWN` and are not executable. Order stays the existing score order, then load id.
