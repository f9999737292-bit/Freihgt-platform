# Product scope

## Commercial priority

**Marketplace network optimization is the primary mode.** Private fleet optimization is the second mode of the same core.

```text
many shippers + many loads + many carriers + current/future truck capacity
                              ↓
                     NETWORK OPTIMIZATION
```

The carrier should receive a recommended executable transport plan, not only a list of freight advertisements.

## MODE A — Marketplace Optimizer

Inputs: published load opportunities, published or predicted capacity, equipment, time windows, city/regional rules, commercial policy.

Outputs the optimizer may seek:

- current-trip fill
- next load
- backhaul
- roundtrip
- multi-load chain
- multi-stop combination
- regional route
- urban route

Human publication is not required for every capacity. Sources include manual entry, current-shipment prediction, telematics, driver app, carrier TMS, API, and fleet plan.

## MODE B — Private Fleet Optimizer

Inputs: orders, fleet, drivers, trailers, depots, calendars, restrictions, rates and costs.

Outputs: vehicle assignments, driver assignments, routes, multi-stop plans, fleet plan.

Marketplace anonymity and cross-shipper publication do not apply. Tenant isolation still applies: a fleet plan never reads another tenant's unpublished orders.

## One core

```text
ONE OPTIMIZATION CORE
  policy profile = MARKETPLACE | PRIVATE_FLEET
  objective profile = see OPTIMIZATION_MODEL.md
  constraint profile = cargo + equipment + time + geo + city rules + documents
```

There are not two solvers. City and fleet behavior are data and policy, not `if city == "Moscow"` branches.

## Problem classes

| ID | Name | Intent |
|----|------|--------|
| P1 | Next load | From a current shipment, predict when and where the vehicle is free, then find a compatible later load. |
| P2 | Backhaul / roundtrip | After A→B, find a load from B or near B toward A or near A. Corridor and proximity, not exact OD equality. |
| P3 | Load chain | A sequence of loads over a configurable horizon: 24h, 48h, 72h, or N days. |
| P4 | Current trip fill | Remaining payload, volume, pallet positions, and linear metres on an in-progress trip. Empty space, not only empty kilometres. |
| P5 | Consolidation | Same OD, multi-pick/one-drop, one-pick/multi-drop, multi-pick/multi-drop, hub, cross-dock. |
| P6 | Regional routing | Milk run, collection, distribution, depot loop, open route, closed route. |
| P7 | Urban routing | City logistics with its own KPI set. First profiles: `CITY_PROFILE_MOSCOW`, `CITY_PROFILE_SAINT_PETERSBURG`. New cities are new rule data. |

## Strategic measures

Unused capacity has three forms:

- empty truck — deadhead kilometres
- empty space — unused kg, m³, pallets, linear metres
- empty time — idle and waiting

`NETWORK_UTILIZATION` is an aggregate that must always break down into vehicle utilization, loaded-km ratio, capacity utilization, time utilization, and revenue utilization. A single score must not hide the components.

## MVP after this freeze (not started)

Default implementation scope, only after a separate approval:

```text
NLO-0.1 Capacity and load foundation
NLO-0.2 Predictive next load
```

MVP behavior:

```text
one active shipment
  → one predicted capacity
  → candidate load pool
  → hard feasibility
  → deterministic economic ranking
  → top N opportunities with score_explanation
```

No ML. No global network optimum. No mutation of shipment topology.

## Non-goals for the first implementation

Unless a later discovery finds them already in code (this discovery did not):

- global optimum for the whole network
- 3D load packing
- advanced ML
- a full hours-of-service regulatory engine
- dynamic city-traffic ML
- autonomous contract pricing
- a multi-day global fleet solver
- automatic cross-dock execution

Also out of this freeze: new runtime service, migrations, handlers, production OpenAPI edits, deploy, merge.

## Acceptance scenarios

SCN-01 through SCN-12 are specified in [SCENARIOS.md](SCENARIOS.md). They are architecture acceptance scenarios, not implemented tests.
