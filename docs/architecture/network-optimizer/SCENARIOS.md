# Scenarios

Architecture acceptance sketches. No solver is implemented. Distances and clock times are illustrative.

## SCN-01 — Predictive next load

A shipment Moscow → Kazan is in execution. Tracking ETA and an unload-duration assumption produce `PredictedCapacity` at Kazan, available 16:35, window 16:10–17:15, method `RULE_BASED`. The matcher searches published opportunities whose pickup is reachable after that window and whose delivery is compatible. A Kazan → Moscow load that passes hard constraints is ranked with `score_explanation`. If ETA is missing, no offer is sent.

## SCN-02 — Backhaul near destination

After Moscow → Kazan, geo search uses a corridor around Kazan, not an exact city match. A pickup 35 km from Kazan with delivery toward Moscow can enter the candidate set. Road distance replaces the 35 km proximity figure before the plan is executable. If the routing provider is down, the pair may stay a candidate and must not be offered as executable kilometres.

## SCN-03 — Current trip fill

Vehicle 20 t and 82 m³. Cargo already aboard 11 t and 44 m³. Residual 9 t and 38 m³. Additional cargo must fit residual weight and volume. Pallet and linear-metre checks run only when those attributes exist; otherwise the explanation says they were not evaluated. Incompatibility and temperature are still hard constraints.

## SCN-04 — Cross-shipper consolidation

Shipper A, Shipper B, and Shipper C each published a load with a scope that allows co-load. One compatible vehicle is proposed. A's API view excludes B's and C's rate, contract, customer, internal ids, and tender. The same exclusion applies to each other shipper. The carrier view has the operational plan and the rate offered to that carrier. The BNO service holds all three owner keys only inside the trust zone. See the visibility matrix.

## SCN-05 — Multi-pick

Pickups Moscow, Khimki, Podolsk, then delivery Saint Petersburg. Stops are ordered. Each pickup has a window and a cargo action. The plan stays `PROPOSED` until execution can store multiple stops. City rules for the Moscow pickups are evaluated through `CITY_PROFILE_MOSCOW` without embedding legal numbers in the scenario.

## SCN-06 — Multi-drop

Pickup Moscow, then Tver, Veliky Novgorod, Saint Petersburg. Unload sequence is recorded. Placement feasibility is `NOT_EVALUATED` in v0.1 unless a later phase sets `SEQUENCE_OK` or `SEQUENCE_CONFLICT`. A conflict with rehandling forbidden is `HARD_REJECT`.

## SCN-07 — Regional milk run

Depot → A → B → C → Depot. `route_closure = CLOSED`. Constraints: service time, windows, route duration, capacity, driver shift, stop count, return to depot. This is private-fleet or a single-tenant regional job, not a cross-shipper advertisement.

## SCN-08 — Moscow urban route

Several customer stops, time windows, vehicle limits, `CITY_PROFILE_MOSCOW`. Primary ranking uses urban KPIs (deliveries per hour, time, on-time), not revenue per kilometre alone. Stale or empty rule versions reject an executable plan.

## SCN-09 — Saint Petersburg urban route

Same as SCN-08 with `CITY_PROFILE_SAINT_PETERSBURG`. The profile id differs. The solver code path does not.

## SCN-10 — Chain

Moscow → Kazan, Kazan → Samara, Samara → Moscow over a configured horizon. Each leg is a feasible match. Slack between predicted availability and the next pickup is stored on `ChainPlan`.

## SCN-11 — Chain ETA failure

The first shipment is late. Predicted availability at Kazan moves past the second pickup window. The chain enters `AT_RISK`. Reoptimization searches a replacement. If none exists, it stays at risk and does not cancel the first shipment.

## SCN-12 — Private fleet

100 orders, 30 vehicles, drivers, and depots in one tenant. No marketplace publication. The same core assigns resources under `MIN_TOTAL_COST` or `SERVICE_LEVEL_FIRST`. Scale class for the benchmark is at least `MEDIUM`. Exact runtime numbers are to be measured, not assumed. A global optimum is not required for the first fleet slice; heuristic assignment plus local improvement is acceptable if explanations and hard constraints hold.
