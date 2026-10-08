# NLO-0.5B implementation roadmap

NLO-0.5B0 froze the search bounds. NLO-0.5B1 and NLO-0.5B2 enforce the discovery cap and the routing budget. NLO-0.5B3 evaluates one-load corridor and four-leg ellipse feasibility. Backhaul chain runtime and roundtrip runtime are not implemented.

```text
NLO_0_5B0=POLICY_ACCEPTED_FOR_BOUNDED_SEARCH
NLO_0_5B1=MERGED
NLO_0_5B2=ROUTING_BUDGET_IMPLEMENTED
NLO_0_5B3=IMPLEMENTED
NLO_0_5B4=DISCOVERY_ONLY
FINAL_EVALUATION_CAP_RUNTIME_ENFORCED=NO
BACKHAUL_RUNTIME_IMPLEMENTED=NO
ROUNDTRIP_RUNTIME_IMPLEMENTED=NO
EXTERNAL_PROVIDER_BENCHMARK=BLOCKED
PROVIDER_SLA_PROVEN=NO
```

| Wave | Content | State |
| --- | --- | --- |
| NLO-0.5B1 | Stop marketplace discovery at 1,000 visible loads in `created_at DESC, id`. | Enforced. Cap remains 1000. |
| NLO-0.5B2 | After the cheap prefilter, route at most 25 loads. Stop at 4 matrix calls, 2 route calls, and 6 provider calls. Honor the 5 second call timeout and the 30 second search watchdog. No retry. | Enforced. |
| NLO-0.5B3 | One-load corridor and four-leg ellipse feasibility on the existing search. Radius is not a backhaul mode. A missing road leg stays unknown. | This change. |
| NLO-0.5B4 | Runtime enforcement of the final-evaluation cap, if it is still distinct from the routing cap, plus any further score persistence. | Discovery only. The routing slice plus the unknown-road reject already keep scoring at 25. No separate runtime guard and no migration. `FINAL_EVALUATION_CAP_RUNTIME_ENFORCED=NO`. See [NLO_0_5B4_FINAL_EVALUATION_DISCOVERY.md](NLO_0_5B4_FINAL_EVALUATION_DISCOVERY.md). |
| NLO-0.5B5 | Public exposure beyond the current next-load response, including Control Tower. | Not started. |

B2 does not add a search mode, a migration, a marketplace endpoint, or a freight-cost ledger. An accepted plan still uses the NLO-0.4 activation handshake.

B3 decides one published load. Corridor feasibility is the existing geometry plus road distance from delivery to the target strictly smaller than road distance from pickup to the target. Ellipse feasibility is `road(release, pickup) + road(pickup, delivery) + road(delivery, target) - road(release, target)`. A nil leg is `ROAD_DISTANCE_UNKNOWN`. It is not stored as zero and it is not replaced by the three-leg pickup detour. `RADIUS` does not become a backhaul result. B3 does not add a search mode, a second load, a chain, a roundtrip runtime, a migration, or a public field.
