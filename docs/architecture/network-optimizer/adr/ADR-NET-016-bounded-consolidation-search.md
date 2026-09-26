# ADR-NET-016: Bounded deterministic consolidation search

Status: Proposed. NLO-0.3A freeze. Implementation is not authorized.

## Decision

NLO-0.3 does not run a solver. MILP, CP-SAT, VRP, large-neighbourhood search, genetic algorithms, and ML are out.

The first search shapes are:

- same-origin and same-destination pairs, ordered by load id;
- current-trip fill of exactly one additional published load, ordered by load id.

Unrestricted subset enumeration is forbidden. A later wave may add bounded expansion only after pair and one-load measurements exist. Production pool limits are not invented here. They stay policy and are unset until measured.

3D bin packing is out. Pallet, linear-metre, and height checks are scalar constraints. `placement_check` stays `NOT_EVALUATED` unless a sequence conflict is proven from stop order alone.

BNO-0.1C2 MatchScore is not reused for a set. The first waves are commercially neutral feasibility. `MAX_CONTRIBUTION` stays reserved. There is no FX and no freight-cost write.

## Consequences

- One hard incompatibility rejects the set. One unresolved required condition blocks `FEASIBLE`.
- `REQUIRE_SEPARATION` and `REQUIRE_CONDITION` stay conditions. They are not treated as satisfied.
- Compatibility is `EvaluateGroupage` from BNO-0.1B2. A second engine is not allowed.
