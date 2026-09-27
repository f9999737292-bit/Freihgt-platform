# ADR-NET-020: Bounded multi-stop sequencing

## Status

Proposed. Discovery only. Not accepted. Implementation is not authorized.

## Context

Unrestricted permutation of stops is a VRP. NLO-0.4 must stay a bounded deterministic planner. NLO-0.3D inserts one load with four route calls and does not search insertion gaps. NLO-0.3E uses no routing.

## Decision

The algorithm is incremental cost-min insertion. Future stops only. Completed stops are not gaps. Loads are taken in load-id order. Each load enumerates pickup and delivery gaps with pickup at or before delivery. One winning sequence is kept. The winner is the feasible sequence with the lowest road duration, then the lowest pickup ordinal, then the lowest delivery ordinal.

```text
UNRESTRICTED_PERMUTATION_SEARCH=NO
MAX_ADDITIONAL_LOADS=2
MAX_STOPS=8
MAX_ACTIVE_LOADS_PER_PLAN=4
MAX_SEQUENCE_CANDIDATES=64
MAX_ROUTING_CALLS=300
TIME_BUDGET=5s
TIME_BUDGET_IS_SLO=NO
```

The measured reason is in `NLO_0_4A_MULTI_STOP_ALGORITHM.md`. On four future stops, two additional loads cost 271 uncached legs and 43 sequences. Three additional loads cost 676 legs and 88 sequences. The first release is two additional loads and eight stops.

Road routing is required. Haversine is not a substitute. Leg reuse is allowed inside one search on `RouteLegKey`. A truncated search is `SEARCH_BUDGET_EXHAUSTED` and must not be reported as `NO_FEASIBLE_SEQUENCE`.

Time is forward propagation. Waiting until a window opens is allowed. Late arrival is `STOP_WINDOW_VIOLATION`.

## Consequences

No solver is introduced. A later controller may reject these numbers. Until then they are the proposed caps, not a runtime policy.
