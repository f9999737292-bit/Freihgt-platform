# NLO-0.5A test strategy

These cases are specified for a later implementation wave. This docs wave does not add them.

| Id | Case |
| --- | --- |
| NLO05-01 | Exact delivery at the target is an eligible backhaul when the other gates pass |
| NLO05-02 | A pickup near the release and inside the corridor is accepted |
| NLO05-03 | A pickup behind the release is `BACKTRACK_DIRECTION_REJECTED` |
| NLO05-04 | Lateral distance over `corridor_deviation_km` is rejected |
| NLO05-05 | Road deadhead over `max_deadhead_km` is rejected |
| NLO05-06 | Haversine is never copied into road kilometres |
| NLO05-07 | Provider unavailable yields `ROAD_DISTANCE_UNKNOWN` and no executable plan |
| NLO05-08 | Route not found yields the same fail-closed result |
| NLO05-09 | Arrival after the pickup window is `PICKUP_WINDOW_MISSED` |
| NLO05-10 | Arrival before the window stores `waiting_minutes` |
| NLO05-11 | Equipment incompatibility is a hard reject |
| NLO05-12 | Temperature incompatibility is a hard reject |
| NLO05-13 | ADR incompatibility is a hard reject |
| NLO05-14 | Pallet or linear-metre overflow is a hard reject |
| NLO05-15 | A stale predicted capacity is rejected |
| NLO05-16 | An anonymized result has coarse display geography and bucketed deadhead |
| NLO05-17 | A foreign private load is not in the candidate pool |
| NLO05-18 | Route increase uses the four road legs, not Haversine |
| NLO05-19 | Increase over `max_route_increase_km` is `ROUTE_INCREASE_EXCEEDED` |
| NLO05-20 | Ranking uses the active score profile only |
| NLO05-21 | The same inputs produce the same top N |
| NLO05-22 | A fresh cache hit is reused and an expired entry is not |
| NLO05-23 | An executable plan records the service-duration policy version |
| NLO05-24 | An accepted plan activates through the NLO-0.4 handshake |
| NLO05-25 | Current-trip planning does not change onboard evidence, completed stops, or TMS return state |

```text
NLO05_01_25_DEFINED=YES
```
