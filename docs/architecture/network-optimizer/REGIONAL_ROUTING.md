# Regional routing

Regional optimization is a policy profile on the same core, not a separate engine.

## Typical jobs

```text
depot → customers → depot
suppliers → hub
hub → customers
milk run
multi-pick
multi-drop
```

Open route (does not return) and closed route (returns to depot) are both first-class. The caller sets `route_closure = OPEN | CLOSED`.

## Constraints the profile must pass to the core

- service time at each stop
- time windows
- maximum route duration
- vehicle capacity (all known dimensions)
- driver shift, when a driver is bound
- maximum number of stops
- depot return, when closure is `CLOSED`

Private-fleet regional plans bind vehicle, driver, and depot. Marketplace regional plans may start from capacity plus equipment and bind driver and vehicle only at assignment.

## Horizon

The chain horizon is a parameter: `24h`, `48h`, `72h`, or `N days`. It is not a constant in solver code.

## What v0.1 does not do

No regional solver is implemented. SCN-07 specifies the milk-run shape the later local-search or VRP adapter must accept. See [SCENARIOS.md](SCENARIOS.md) and [ROADMAP.md](ROADMAP.md) NLO-0.6.
