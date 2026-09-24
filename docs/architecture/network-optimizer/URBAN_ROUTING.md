# Urban routing

Urban logistics uses a different objective profile on the same core. It is not a Moscow-specific solver.

## Primary KPIs

```text
deliveries_per_hour
revenue_per_vehicle_hour
km_per_delivery
on_time_rate
stop_time
waiting_time
vehicle_utilization
```

`revenue_per_km` may be reported. It is not the primary urban objective. Dense stop time and waiting dominate city economics.

## City profiles

The first profiles are data bundles:

- `CITY_PROFILE_MOSCOW`
- `CITY_PROFILE_SAINT_PETERSBURG`

Adding a city means adding a profile and rule versions. The solver calls `CityRulesProvider`. It does not branch on city name. See [CITY_RULES.md](CITY_RULES.md).

## Inputs

- stops with time windows
- vehicle mass, dimensions, and equipment
- the active city-rule version
- service duration
- optional slot requirement at a facility

SCN-08 and SCN-09 are the acceptance sketches. They do not embed legal restriction numbers.

## Zone clustering

Large urban jobs are split before route construction:

```text
Orders → geo/operational clustering → smaller route problems → local optimization
```

Cluster boundaries are operational (density, windows, depot, vehicle limits). They are not required to follow administrative borders. A cluster id is an input to a child `OptimizationJob`, not a permanent geography master.

## Failure

If city rules are missing or stale, an urban plan that would be executable is `HARD_REJECT`. The system must not fall back to an unconstrained city route.
