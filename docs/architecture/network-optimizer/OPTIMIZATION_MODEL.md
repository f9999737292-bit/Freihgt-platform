# Optimization model

## Separation

Candidate generation and candidate scoring are different stages (ADR-NET-006).

```text
Capacity + Load Opportunities
        ↓
Geo candidate search          # proximity / corridor; not road distance
        ↓
Time feasibility
        ↓
Equipment feasibility
        ↓
Cargo compatibility
        ↓
Commercial / policy filters
        ↓
Candidate set
        ↓
Deterministic score
        ↓
Optional local route optimization
```

Do not start with a global optimum. NLO-0.3A additionally freezes that an unrestricted subset enumeration of N loads is not the default search. The first consolidation waves are bounded: a same-origin/same-destination pair, or one confirmed onboard cargo projection plus exactly one additional published load. MILP, CP-SAT, VRP, LNS, genetic search, and ML stay deferred. See [ADR-NET-016](adr/ADR-NET-016-bounded-consolidation-search.md).

The supported phase order is:

```text
candidate generation → feasibility filters → heuristic ranking → local route optimization
```

Global network optimization is NLO-1.0, after the earlier stages are measured.

## Match score

No ML in the first stage. The contract is deterministic and explained.

```text
ExpectedContribution
+ CapacityUtilization
+ DestinationNetworkValue
+ HomeDirectionValue
+ CarrierPreference
+ ShipperPreference
- DeadheadPenalty
- WaitingPenalty
- ETAUncertainty
- DelayRisk
- CounterpartyRisk
```

The API returns `score_explanation`, never only `score=93`. Each component has a code, a numeric contribution, and a human-readable reason. Hard rejects are a separate list and are not folded into a low score.

### Explanations

Carrier example:

```text
Recommended because:
- 24 km empty reposition
- compatible curtain-side 20t
- pickup 1h 40m after predicted unloading
- 88% expected capacity utilization
- destination has high future load availability
```

Rejection example:

```text
Not offered because:
pickup window ends before predicted vehicle availability.
```

## Objective profiles

Carrier-selectable:

`MAX_REVENUE`, `MAX_CONTRIBUTION`, `MIN_DEADHEAD`, `MAX_CAPACITY_UTILIZATION`, `RETURN_HOME`, `MIN_RISK`, `BALANCED`.

Additional private-fleet profiles:

`MIN_TOTAL_COST`, `MAX_FLEET_UTILIZATION`, `MIN_NUMBER_OF_VEHICLES`, `SERVICE_LEVEL_FIRST`.

A profile is data: weights and which hard constraints stay mandatory. It is not a city-specific code branch.

## Network value

```text
NetworkValue(location, time, equipment) ->
  value, confidence, method
```

Method is `DETERMINISTIC_FALLBACK`, `STATISTICAL`, `ML`, or `EXTERNAL`.

The future optimizer may prefer destination B with a slightly lower immediate rate when future load probability is higher. v0.1 defines the interface, the data requirements (location, time bucket, equipment class, historical accepted loads — not yet collected), and a deterministic fallback: value `0`, confidence `0`, explanation `network value not yet observed`. No ML model is built.

## Economics

See [RATE_OWNERSHIP_MATRIX.md](RATE_OWNERSHIP_MATRIX.md). Ranking uses `expected_contribution` as a planning figure.

## Solver comparison

No method is chosen as the permanent engine.

| Method | Use | Advantages | Limitations | Scale | Latency | Explainability | Build cost |
|--------|-----|------------|-------------|-------|---------|----------------|------------|
| Rules / heuristics | P1, P2 filters | Transparent, fast | Misses combinations | small–large filter | low | high | low |
| Greedy | Top-N next load, fill | Simple, deterministic | Local optima | medium | low | high | low |
| Local search | Improve a route | Good quality/effort | Needs a seed | medium | medium | medium | medium |
| Large neighborhood search | Regional, chain | Strong on VRP-like problems | Tuning | medium–large | medium–high | medium | high |
| VRP solver | P6, P7 | Standard routing | Weak on marketplace privacy and offers | medium | medium | medium | high |
| MILP | Small fleet assignment | Optimal for small sets | Does not scale to the network | small | high | medium | high |
| CP-SAT | Windows + assignment | Good constraints | Large models time out | small–medium | high | medium | high |
| Graph search | Chains, corridors | Natural for sequences | Cost model is extra | medium | medium | high | medium |
| Min-cost flow | Bulk assignment | Fast matching | Poor multi-stop geometry | large | low | medium | medium |
| Hybrid | Target end state | Fit method to problem class | Operational complexity | staged | mixed | must keep explanation | high |

### Staged recommendation

1. NLO-0.1–0.2: rules plus greedy ranking for one capacity and a candidate pool.
2. NLO-0.3–0.5: same core, richer feasibility (fill, consolidation, backhaul, short chains).
3. NLO-0.6–0.8: local search or a VRP adapter behind `RoutingOptimizer`, still explained.
4. NLO-0.9–1.0: chain search, then hybrid network jobs.
5. FLEET-1.x: MILP or CP-SAT only for measured small and medium fleet instances; heuristic fallback when the time budget expires.

## Scale classes

No production volumes are asserted. Benchmarks must measure:

| Class | What to measure |
|-------|-----------------|
| SMALL | One vehicle, tens of loads, interactive latency |
| MEDIUM | One region, hundreds of loads, seconds |
| LARGE | Multi-region day, candidate-pair growth |
| VERY_LARGE | Network-wide batch, concurrent jobs |

Separate series: loads/day, available capacities, active vehicles, candidate pairs, urban stops, concurrent optimization jobs. Record p50/p95 duration, rejection reasons, and determinism across repeats.

## Geospatial

| Need | v0.1 stance |
|------|-------------|
| Coordinates | Exist on locations and tracking events; may be null |
| Geocoding | Provider port; not in repo |
| Distance matrix | Provider port; not Haversine |
| Route distance / travel time | Provider port |
| Road network | External |
| Zones / corridors | City rules and geo index |

Straight-line distance, including Haversine, is a candidate pre-filter only. It is not canonical road distance, not canonical travel time, and not canonical freight cost distance. Executable kilometres, deadhead, and ETA slack require a road result or an audited `MANUAL_DISTANCE`. A plan that used Haversine as if it were road distance is invalid. Freight cost remains the owner of any billed distance.

## Provider ports

No hard dependency on one vendor.

```text
RoutingProvider
DistanceMatrixProvider
GeocodingProvider
TrafficProvider
CityRulesProvider
CapacityPredictionProvider   # RULE_BASED | STATISTICAL | ML | EXTERNAL
NetworkValueProvider
```

Adapters are replaceable. The core depends on the ports.

## Failure modes

| Failure | Behavior |
|---------|----------|
| Routing provider down | Do not emit an executable distance. Candidate search may still use proximity. Road-based objectives return `INFEASIBLE_ROUTING`. |
| Traffic down | Freeflow time plus raised `ETAUncertainty`. Explanation says traffic was not applied. |
| City rules stale | Urban executable plans `HARD_REJECT` when the rule version is older than policy `max_age`. Do not ignore rules. |
| GPS stale | Do not treat the point as current for in-trip fill once freshness is exceeded. Confidence drops. |
| ETA unavailable | No `PredictedCapacity` above the confidence floor. Next-load offers that need an arrival time are not sent. |
| Rate unavailable | `UNPRICED`. Excluded from revenue and contribution ranking. |
| Low capacity confidence | No automatic offer. Owning carrier may see a low-confidence candidate. |
| Slot unavailable | Stops that require a slot are `HARD_REJECT`, or `SLOT_UNCONFIRMED` if policy allows a non-executable proposal. |

Fail safe means: do not assign, do not publish an executable plan, and record the reason.

## Audit

`OptimizationDecision` keeps enough to reconstruct:

- why a candidate was eligible
- why a candidate was rejected
- score components
- rule versions
- rate snapshot id or offered-rate revision
- route assumptions and provider ids
- capacity snapshot
- user or automation action

Decisions are immutable. A reoptimization writes a new decision and links `causation_id`.

## Reoptimization loop

```text
PLAN → MATCH → OFFER → ACCEPT → EXECUTE → TRACK → ETA CHANGE → RE-EVALUATE → REOPTIMIZE
```

`network.chain.marked_at_risk` when, under current predictions, the next leg's pickup window ends before predicted availability, or slack falls below the policy minimum. The previous plan becomes `AT_RISK`, then `REOPTIMIZING`. Active execution is not cancelled by the optimizer; shipment cancellation stays with shipment-service.

## Reservation races

See [STATE_MACHINES.md](STATE_MACHINES.md). Two carriers cannot both win one load. One carrier cannot hold two loads that fail hard constraints against the same capacity.
