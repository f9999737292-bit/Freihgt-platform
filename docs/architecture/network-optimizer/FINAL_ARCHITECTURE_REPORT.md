# BINTRANS TRANSPORTATION & NETWORK OPTIMIZER v0.1

## ARCHITECTURE FREEZE — FINAL REPORT

### GIT

```text
BRANCH=discovery/bintrans-transport-network-optimizer-v0.1
BASE_SHA=2c0ad2bd573ca78788d09d13cfe629b4023b88d9
FINAL_HEAD=2c0ad2bd573ca78788d09d13cfe629b4023b88d9
WORKTREE=D:\Projects\freight-platform-wt\transport-network-optimizer-v0.1
WORKTREE_CLEAN=NO
```

`WORKTREE_CLEAN=NO` because this freeze adds uncommitted documentation. No commit was requested. `git fetch origin` on 2026-09-24 left `origin/main` equal to `BASE_SHA`. Fast-forward was not required.

### SAFETY

```text
PRODUCT_CODE_CHANGED=NO
RUNTIME_CHANGED=NO
MIGRATIONS_CHANGED=NO
DEPLOYMENT_CHANGED=NO
```

`packages/openapi` was not edited. The API skeleton is under `contracts/`.

### DISCOVERY

```text
CURRENT_STATE_INVENTORY=PASS
REUSABLE_COMPONENTS=transport order, shipment O-D, cargo weight/volume/temperature/ADR flag, locations with optional lat/lon, vehicle weight/volume, driver, carrier company, tracking ETA and positions, slot windows, contract rate, rate snapshot, freight-cost ledger, RFx and freight request, billing settlement, document registry, control tower projections, shipment/driver outbox, gateway JWT tenant
MAJOR_GAPS=no legs or stops, no trailer, no facility master, no pallet or linear-metre model, no marketplace load projection, no capacity aggregate, no routing network, no city-rule data, no solver, ETA not on Kafka, EDO operator service not in code, driver app does not publish GPS
```

### DOMAIN

```text
CAPACITY_MODEL=DEFINED
CARGO_UNIT_MODEL=DEFINED
LOAD_OPPORTUNITY_MODEL=DEFINED
ROUTE_MODEL=DEFINED
CONSOLIDATION_MODEL=DEFINED
```

### OPTIMIZATION

```text
NEXT_LOAD=DEFINED
BACKHAUL=DEFINED
CURRENT_TRIP_FILL=DEFINED
MULTI_STOP=DEFINED
CHAIN=DEFINED
REGIONAL=DEFINED
URBAN=DEFINED
PRIVATE_FLEET=DEFINED
```

### CITY RULES

```text
MOSCOW_PROFILE=DEFINED
SAINT_PETERSBURG_PROFILE=DEFINED
RULES_VERSIONED=YES
RULES_HARDCODED=NO
```

Normative Moscow and Saint Petersburg thresholds are intentionally absent. Profiles are empty rule-set selectors until sourced data exists.

### SECURITY

```text
TENANT_ISOLATION=PASS
MARKETPLACE_PROJECTION=DEFINED
CROSS_SHIPPER_PRIVACY=PASS
THREAT_MODEL=DEFINED
```

### INTEGRATION

```text
SHIPMENT=READ execution facts; do not scan cross-tenant; do not invent stops in the current table
RFX=OWNER preserved; mini-tender RFQ auction handoff
FREIGHT_COST=OWNER preserved; planning contribution is not a ledger entry
DRIVER_VEHICLE=PRIVATE fleet binds them; marketplace may match equipment first; trailer NOT_FOUND
SLOT_BOOKING=OWNER tracking-service; optimizer queries and does not book
CONTROL_TOWER=future projections of network facts; CT remains the operator read model
EDO=document registry only; operator exchange DOCS_ONLY; optimizer does not send EDO
SETTLEMENT=billing-register-service remains owner
```

### CONTRACTS

```text
OPENAPI=SKELETON at contracts/network-optimizer-v0.1.openapi.yaml
EVENT_CATALOG=DEFINED network.* plus existing shipment.* and driver.*
STATE_MACHINES=DEFINED
ERD=DEFINED
ADRS=ADR-NET-001 through ADR-NET-012 PROPOSED
```

### SCENARIOS

```text
SCN_01=COVERED
SCN_02=COVERED
SCN_03=COVERED
SCN_04=COVERED
SCN_05=COVERED
SCN_06=COVERED
SCN_07=COVERED
SCN_08=COVERED
SCN_09=COVERED
SCN_10=COVERED
SCN_11=COVERED
SCN_12=COVERED
```

### ROADMAP

```text
NLO_0_1=Capacity and load foundation
NLO_0_2=Predictive next load
NLO_0_3=Fill and consolidation
NLO_0_4=Multi-stop plans, execution gated
NLO_0_5=Backhaul and roundtrip
NLO_0_6=Regional routing
NLO_0_7=Urban Moscow data profile
NLO_0_8=Urban Saint Petersburg data profile
NLO_0_9=Chain optimizer
NLO_1_0=Network optimizer
```

Private fleet is FLEET-1.x on the same core.

### RECOMMENDED_FIRST_IMPLEMENTATION

```text
SCOPE=NLO-0.1 + NLO-0.2
WHY=One shipment, one rule-based prediction, hard feasibility, explained top N. No ML and no global solver. Current cargo and vehicle fields can support weight and volume only.
DEPENDENCIES=JWT tenant headers; reads of shipment, tracking ETA, location, cargo, vehicle; explicit publication; optional rate snapshot
RISKS=Haversine treated as road km; cross-tenant shipment scan; missing pallets treated as zero; multi-stop promised before execution can store it
```

### VERDICT

```text
ARCHITECTURE_FREEZE=PASS
IMPLEMENTATION_AUTHORIZED=NO
STOP_AFTER_NETWORK_OPTIMIZER_V0_1_ARCHITECTURE_FREEZE=YES
```

Acceptance gates from the task are satisfied as documentation. They are not runtime evidence. Pre-commit review is in [CONSISTENCY_REVIEW.md](CONSISTENCY_REVIEW.md). ADRs stay Proposed. Implementation is not authorized.
