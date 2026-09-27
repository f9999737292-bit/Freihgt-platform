# NLO-0.3E measurements

These numbers are one local in-memory run. They are not a production capacity claim, not Postgres latency, and not 2GIS latency.

## Environment

```text
go=go1.26.4
os=windows
arch=amd64
cpus=12
store=repository.Memory
routing=BENCHMARK_PROVIDER
NOT_PRODUCTION_LATENCY=YES
command=go test -tags nlo03ediscovery -count=1 -timeout 20m ./internal/service/ -run TestNLO03EDiscoveryBenchmark -v
```

The harness is `services/network-optimizer-service/internal/service/nlo_03e_discovery_benchmark_test.go` with build tag `nlo03ediscovery`. Default `go test` does not compile it. It calls the existing `SearchConsolidation`. It does not change production code.

Fixtures are seeded UUIDs. Pairwise loads share one pickup location and one delivery location. Windows overlap. Weight and volume are known. The carrier owns capacity. The shipper owns every load and opts in. Fill loads set `CrossShipperConsolidationAllowed` because the searcher is the carrier.

Repeats: 5, except pairwise 500 uses 3 and pairwise 1000 uses 1. Duration samples include the first call, so p95 on a small sample is the slowest observed call, including cold start. p50 is the middle sorted duration. Fingerprints are SHA-256 of the sorted candidate fingerprints and matched across repeats.

`db_reads_logical` and `db_writes_logical` count repository operations and persisted rows in the memory store. They are not SQL timings. `alloc_bytes` is `TotalAlloc` delta for the first repeat. `heap_alloc` is process heap at that moment. Fill `heap_alloc` after the pairwise 1000 case still reflects that earlier allocation. Use `alloc_bytes` for the fill case.

## Pairwise, one origin-destination group, all feasible

Catalog loads equal groupage calls. Both are `2 * pairs` because the capacity tenant and the load tenant each receive one evaluation. Routing calls are 0.

| Pool | Pairs = N*(N-1)/2 | Groupage | Logical writes | Response bytes | Alloc bytes | p50 ms | p95 ms | Repeats |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 10 | 45 | 90 | 136 | 100491 | 2117240 | 2 | 39 | 5 |
| 50 | 1225 | 2450 | 3676 | 2723637 | 48799832 | 90 | 109 | 5 |
| 100 | 4950 | 9900 | 14851 | 11004313 | 210755888 | 363 | 385 | 5 |
| 500 | 124750 | 249500 | 374251 | 277319719 | 5878136976 | 11194 | 11474 | 3 |

Fingerprints:

```text
10  f15fac9b2ef6724337a0736eef7491b2c541870e9d1704ebca2098e8d99334ec
50  b9edaee5d9b448bd860c5fa5e2fce54427251dff5cd4b0c273f54047926eecbe
100 cf7f228f2fe7ab7cae480b2e0d68131700b199194bf327f71a2329a76b8d8f66
500 90d01066b0994d0a59cc4b559b88aa3e549bcf28c7b1f54658cd814534d2f8a0
```

Logical writes are `1 + pairs + 2*pairs`.

Exploratory only, not a supported limit. One repeat:

```text
pool=1000 pairs=499500 groupage=999000 writes=1498501
response_bytes=1110388970 alloc_bytes=23372570168 heap_alloc=6831600040
p50_ms=49270 fingerprint=a82204f20fc92c85b6bdbaeecc0f49694baa57c4fe17dac3d642d8a38c4f72df
```

## Current-trip fill

Eligible equals pool in every scenario below. Groupage and catalog loads are `2 * N`. Logical writes are `1 + N + N` (run, candidate, one member). Logical reads are recorded as 7: one pool read plus six context-source reads for one onboard cargo. That count is harness accounting, not a Postgres trace.

Provider label on every fill row: `BENCHMARK_PROVIDER|NOT_PRODUCTION_LATENCY`. Durations below include that double. They are not road-provider latency.

### FEASIBLE

Weight 100 kg, volume 1 m³. Routing calls = `4 * N`.

| Pool | Routing | Groupage | Writes | Alloc bytes | p50 ms | p95 ms | Fingerprint |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| 10 | 40 | 20 | 21 | 533264 | 2 | 14 | c7d7f32a077b61f7a231b43b5c4953144685ee6770fa83c798d82ff306e607b5 |
| 50 | 200 | 100 | 101 | 2729784 | 10 | 16 | 737cb8e41e505df545a6724aafc8e8842bd1c6b0d61b2e6384d2ab35f1d01cef |
| 100 | 400 | 200 | 201 | 5480296 | 30 | 75 | 9d9f8745ca376b91fdd4f2fba2ea045bd932ceb56a9c30643717ae89d3198097 |
| 500 | 2000 | 1000 | 1001 | 26609224 | 319 | 989 | 857c045e706d5a14ac6c6066856aeb22d25e996d146e05839509eba1455d0292 |

### HARD_REJECT_STILL_ROUTES

Weight 50000 kg. Routing calls stay `4 * N`. Hard reject does not skip the provider.

| Pool | Routing | Groupage | p50 ms | p95 ms | Fingerprint |
| --- | ---: | ---: | ---: | ---: | --- |
| 10 | 40 | 20 | 2 | 9 | 0760781cc0c3c5e0eddac8c82b059d77961fcb8b236dc48d1229c97e47d68013 |
| 50 | 200 | 100 | 16 | 22 | 552d33e673a709c54a6bdb3997a8bfa64c8f5f0309c5ed9ac1a0011cde0253ad |
| 100 | 400 | 200 | 61 | 92 | 196bfd8d4b35746090fa0f4d7aba8f878ff189e2eef51bb5e3d288c115ad30d4 |
| 500 | 2000 | 1000 | 220 | 485 | 495f4d02d9ad4ff76dc0390afcfaac4ac447d043c57ef989dfbad3c6833129af |

### STALE_POSITION_SKIPS_ROUTING

Routing calls = 0. Groupage still runs.

| Pool | Routing | Groupage | p50 ms | p95 ms | Fingerprint |
| --- | ---: | ---: | ---: | ---: | --- |
| 10 | 0 | 20 | 2 | 4 | 1b9019cfa9f3bc5758f2ea2be17abee1356f71e9b839e9d57ac68dc8a2fced39 |
| 50 | 0 | 100 | 9 | 12 | f14eeca0ef3ee0b0ab1de86a0dee6774e6540f85a811b32e9dbded6913bb9897 |
| 100 | 0 | 200 | 24 | 61 | caff4e23aa112b712d899027b614eb11c46b98b6f2cd756290fcb68cd7d23caf |
| 500 | 0 | 1000 | 92 | 122 | fd285bc040ad3e66ada71ae8d78760413565069e4941ce80f51563512d47ae77 |

### INDETERMINATE

Weight absent. Routing still `4 * N` because the position is fresh.

| Pool | Routing | Groupage | p50 ms | p95 ms | Fingerprint |
| --- | ---: | ---: | ---: | ---: | --- |
| 10 | 40 | 20 | 2 | 4 | 32f8bffbef9f308af9501acbcc2fa1fccf2bd49edcf2950eefa008f8a2171da5 |
| 50 | 200 | 100 | 12 | 13 | aac9b6967c9ab952098c92799109877f97b0fe9fde4cd32a05eeb2a8bccb8b42 |
| 100 | 400 | 200 | 28 | 44 | 9e4549aa09b36cf6511bb0d605741b1149320ee3a56c177712c2b4e17f9cf761 |
| 500 | 2000 | 1000 | 112 | 183 | 86b0078be9e9d5a5635680d7b8fe29cc12d483d0cf827278bdfc2981ba5a634d |

### MIXED

About one quarter overweight, one quarter unknown weight, the rest feasible. Routing stays `4 * N`.

| Pool | Routing | Groupage | p50 ms | p95 ms | Fingerprint |
| --- | ---: | ---: | ---: | ---: | --- |
| 10 | 40 | 20 | 1 | 2 | ac51a80bbcb899dc905d64187d2e65d7a594b47168d65a6e7e34d3f2ff4eb0ab |
| 50 | 200 | 100 | 9 | 11 | b5202b2c3c85fcb1fc43c5074e7d78e89a6bac9f3f565d9b0fa6a4d1351c0527 |
| 100 | 400 | 200 | 27 | 36 | b5d7be4d90194072a9df017fc48bbc24d0acfd9f0e97c1611934477aa880116f |
| 500 | 2000 | 1000 | 126 | 169 | 587e9c8834e94541c08aadfb55dc65f1f05d86c4b782a5c5b8803c73bf194f18 |

## Cost drivers

Primary: combinatorial pair enumeration inside one origin-destination group, together with persisting and marshaling every pair. Class: COMBINATORIAL, CPU, MEMORY. At 500 loads the process held about 1.7 GB heap and spent about 11 s. At 1000 loads it held about 6.8 GB and spent about 49 s. That shape is not a production pool.

Secondary: two catalog contexts and two `EvaluateGroupageItems` calls per same-owner pair, because the capacity tenant and the load tenant are both projected. Class: CPU. The call count is linear in the pair count, so it scales with the square of the group.

DATABASE: logical rows scale as `1 + 3*pairs` for pairwise and `1 + 2*N` for fill. Postgres round-trip time was not measured. Do not treat the memory-store duration as database latency.

NETWORK/ROUTING: pairwise makes no routing call. Fill makes four calls per fresh-position candidate, including hard rejects and indeterminate weight. The measured milliseconds are `BENCHMARK_PROVIDER` and must not be quoted as 2GIS time. The call count itself is production-shaped: 2000 calls at 500 candidates.

Sorting and privacy projection are present and were not separable from the search duration. They are not the dominant term next to N² persistence.

## What the in-memory run does not justify

The in-memory numbers above are not Postgres latency and not 2GIS latency. The Postgres section below is the budget evidence for NLO-0.3E. Routing-provider load is not required for this wave: `NLO03E_ROUTING_CALLS_PER_SET=0`.

## Postgres pairwise, one origin-destination group

Disposable embedded Postgres 16 on localhost, database created for this test and stopped with the test. Migrations applied through `000083`. Not staging and not production. Same machine as the in-memory run: go1.26.4, windows/amd64, 12 CPUs.

Fixtures are seeded. One canonical origin and destination. Weight and volume are known. Catalog contexts are empty, so each catalog load is followed by one `EvaluateGroupageItems`. Groupage calls equal catalog loads for that reason. Routing calls are 0. p50 is the middle sorted duration. Observed max is the slowest sample. Duration is `SearchConsolidation` only.

| Pool | Pairs | Catalog and groupage | Candidate rows | Member rows | Inserted rows | Response bytes | Alloc bytes | Heap bytes | DB growth bytes | p50 ms | Observed max ms | Samples |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 10 | 45 | 90 | 45 | 90 | 136 | 104091 | 2730512 | 1404624 | 212992 | 150 | 196 | 5 |
| 50 | 1225 | 2450 | 1225 | 2450 | 3676 | 2821637 | 63783208 | 14541888 | 2547712 | 2493 | 2846 | 5 |
| 100 | 4950 | 9900 | 4950 | 9900 | 14851 | 11400313 | 270912896 | 55150848 | 9953280 | 10183 | 10431 | 5 |
| 500 | 124750 | 249500 | 124750 | 249500 | 374251 | 287299719 | 7391905000 | 1335144032 | 247742464 | 445323 | 445323 | 1 |

Fingerprints:

```text
10  4cc6cd6f4c74be57c7bc0463a15e4e4d8019945abb0a0c9f42c894ab30f5de50
50  48c848cee5cfdf5c32fddeeb72ecaa8318c1c3ba17b11d0a18bc43e08d7973f9
100 52d9f248bb5cf39c01815221accbac9e4d94047470bdb87eb65f2bb6dbe50eb7
500 6bacdda8c59cdc4581846345eebf257dd4aae9802f10e274a67f5eaf63604a4a
```

Inserted rows are 1 run + 1 candidate per pair + 2 members per pair. DB growth is `pg_database_size` after the first search minus the size after the fixture insert. Heap is Go heap after that search, not Postgres process RSS. Pool 500 has one sample, so its p50 and observed maximum are that sample.

Pool 500 took 445323 ms, inserted 374251 rows, returned about 287 MB of JSON, grew the database by about 236 MB, and left about 1.3 GB on the Go heap. Routing calls were 0. That size is not a supported pool.

Adopted NLO-0.3E ceilings are not these pairwise sizes. A global eligible pool of 10 with set size at most 3 evaluates at most 165 sets and 330 groupage calls. Pool 50 size-2 observed max was 2846 ms. Pool 100 size-2 observed max was 10431 ms. Those two figures are the source of the 5 second watchdog. The watchdog is not an SLO. Pool 500 is not a supported pool.
