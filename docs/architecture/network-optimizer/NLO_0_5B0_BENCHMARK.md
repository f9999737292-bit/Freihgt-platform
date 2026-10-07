# NLO-0.5B0 benchmark

Local only. No 2GIS call. Synthetic coordinates around latitudes 10, 20, 30, and 40. No customer positions.

```text
BENCHMARK_MODE=LOCAL_PREFILTER_AND_CALL_ACCOUNTING
EXTERNAL_PROVIDER_BENCHMARK=BLOCKED
PROVIDER_TIME=NOT_MEASURED
PROVIDER_SLA_PROVEN=NO
PROVIDER_COMMERCIAL_LIMITS=UNKNOWN
STAGING_PROVIDER_APPROVAL_NOT_IMPLIED=YES
DB_TIME=NOT_MEASURED
```

## Algorithm time

`TestNLO05B0LocalPrefilterTiming` projects each synthetic point onto a two-point line and computes straight-line distance. Thirty-one rounds. One published workstation sample for the dense-city class at 1,000 points was about 2 milliseconds at P99. The test does not assert that number. It is not NLO search latency and not provider latency.

## Uncapped call shape

`TestNLO05B0ProviderCallAccounting` records the shape before the routing cap. At 1,000 ellipse survivors that shape is 160 matrix calls and 2 route calls. At 25 ellipse survivors it is 4 matrix calls and 2 route calls. B2 does not let a search route the 1,000-survivor row.

## Frozen ceilings

```text
CANDIDATE_DISCOVERY_CAP=1000
CANDIDATE_ROUTING_CAP=25
MATRIX_DIMENSION_LIMIT=25
MAX_MATRIX_CALLS_PER_SEARCH=4
MAX_ROUTE_CALLS_PER_SEARCH=2
MAX_PROVIDER_CALLS_TOTAL=6
PROVIDER_REQUEST_TIMEOUT=5s
SEARCH_HARD_BUDGET=30s
RETRY_COUNT=0
PARALLELISM=1
```

The hard budget is a watchdog for six sequential calls at the 5 second client timeout. It is not a 2GIS SLO. On the stop, the search does not start another provider call, does not retry, and does not replace a missing road fact with Haversine.
