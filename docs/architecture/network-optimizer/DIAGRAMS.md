# Diagrams

## Context

```mermaid
flowchart TB
  subgraph demand [Demand]
    Shippers[Shippers]
    TO[Transport Order]
  end
  subgraph capacity [Capacity]
    Carriers[Carriers]
    Fleet[Private fleet]
  end
  subgraph core [Network optimizer]
    Feas[Feasibility]
    Cons[Consolidation]
    Opt[Routing and optimization]
  end
  TO --> Feas
  Shippers --> TO
  Carriers --> Feas
  Fleet --> Feas
  Feas --> Cons --> Opt
  Opt --> Offer[Carrier offer]
  Offer --> RFx[RFx when competitive]
  Offer --> Ship[Shipment execution]
  Ship --> CT[Control Tower]
  Opt --> Rules[City rules]
  Opt --> RouteProv[Routing providers]
```

## Component

```mermaid
flowchart LR
  Pub[Publication]
  Cap[Capacity and prediction]
  Match[Matching]
  Score[Scoring]
  Consol[Consolidation]
  Routes[Route and chain plans]
  Offers[Offers]
  Audit[Decision log]
  Rules[CityRulesProvider]
  Econ[Planning economics]
  Pub --> Match
  Cap --> Match
  Match --> Consol --> Routes
  Match --> Score --> Econ
  Score --> Offers
  Routes --> Offers
  Rules --> Match
  Match --> Audit
  Score --> Audit
```

## Data flow

```mermaid
flowchart TD
  Order[Tenant transport order]
  Cargo[Tenant cargo]
  Ship[Tenant shipment]
  Eta[Tracking ETA]
  Proj[Explicit projection]
  Eng[Optimizer]
  Plan[Route plan]
  Order --> Proj
  Cargo --> Proj
  Ship --> Eta --> Pred[Predicted capacity]
  Proj --> Eng
  Pred --> Eng
  Eng --> Plan
  Plan --> Offer[Offer or RFx handoff]
```

## Marketplace privacy flow

```mermaid
flowchart TD
  S1[Shipper A order]
  S2[Shipper B order]
  P1[Projection A]
  P2[Projection B]
  Eng[Engine in trust zone]
  VA[View for A]
  VB[View for B]
  VC[View for carrier]
  S1 --> P1 --> Eng
  S2 --> P2 --> Eng
  Eng --> VA
  Eng --> VB
  Eng --> VC
```

A's view does not include B's rate. The engine may hold both owner keys.

## Predictive capacity sequence

```mermaid
sequenceDiagram
  participant Ship as Shipment
  participant Tr as Tracking
  participant Pred as Prediction provider
  participant Cap as Capacity
  Ship->>Pred: status and remaining leg
  Tr->>Pred: ETA and freshness
  Pred->>Cap: PredictedCapacity RULE_BASED
  Note over Cap: No ML in this freeze
```

## Next load sequence

```mermaid
sequenceDiagram
  participant Cap as Predicted capacity
  participant Pool as Published loads
  participant Match as Matcher
  participant Score as Scorer
  Cap->>Match: location and window
  Pool->>Match: opportunities in scope
  Match->>Match: geo then time equipment cargo policy
  Match->>Score: feasible set
  Score->>Score: explanation and top N
```

## Cross-shipper consolidation sequence

```mermaid
sequenceDiagram
  participant A as Shipper A
  participant B as Shipper B
  participant Eng as Consolidation
  participant C as Carrier
  A->>Eng: opportunity scope allows co-load
  B->>Eng: opportunity scope allows co-load
  Eng->>Eng: hard constraints
  Eng->>C: one plan equipment and windows
  Eng->>A: own cargo only
  Eng->>B: own cargo only
```

## Backhaul sequence

```mermaid
sequenceDiagram
  participant Leg as Completed or active A to B
  participant Geo as Corridor search
  participant Pool as Loads near B
  Leg->>Geo: destination and max deadhead
  Geo->>Pool: proximity not exact OD
  Pool->>Leg: candidate toward A or near A
```

## Chain reoptimization sequence

```mermaid
sequenceDiagram
  participant Eta as ETA change
  participant Chain as Chain plan
  participant Job as Reoptimizer
  Eta->>Chain: slack below minimum
  Chain->>Chain: AT_RISK
  Chain->>Job: network.chain.marked_at_risk
  Job->>Chain: new PROPOSED plan or remain AT_RISK
  Note over Chain: Live shipment is not cancelled here
```

## Private fleet sequence

```mermaid
sequenceDiagram
  participant Orders as Tenant orders
  participant Fleet as Vehicles drivers depots
  participant Core as Same core
  participant Plan as Fleet plan
  Orders->>Core: no marketplace publication
  Fleet->>Core: explicit resources
  Core->>Plan: assignments and routes
```

## Urban route sequence

```mermaid
sequenceDiagram
  participant Stops as Stops
  participant City as CityRulesProvider
  participant Core as Optimizer
  Stops->>Core: windows and vehicle
  Core->>City: profile id and rule version
  City->>Core: restrictions or stale
  Core->>Core: reject if stale else local route
```

## Product shape

```mermaid
flowchart TB
  Net[BINTRANS network]
  Net --> Demand[Demand]
  Net --> Cap[Capacity]
  Demand --> Feas[Feasibility engine]
  Cap --> Feas
  Feas --> Consol[Consolidation]
  Consol --> Route[Routing and optimization]
  Route --> Fill[Fill]
  Route --> Next[Next load]
  Route --> Back[Backhaul]
  Route --> Chain[Chain]
  Route --> Geo[Regional and urban]
  Geo --> Offer[Carrier offer]
  Offer --> Ship[Shipment]
  Ship --> Tower[Control Tower]
```
