# ADR-NET-020: Bounded multi-stop sequencing

## Status

Accepted. NLO-0.4B applies this heuristic as runtime policy.

```text
ADR_NET_020=ACCEPTED
```

## Context

Unrestricted permutation of stops is a VRP. NLO-0.4 must stay a bounded deterministic planner. NLO-0.3D inserts one load with four route calls and does not search insertion gaps. NLO-0.3E uses no routing.

## Decision

The algorithm is a bounded incremental heuristic: cost-min insertion, one parent kept. Future stops only. Completed stops are not gaps. Loads are taken in lexicographic load-id order. Load-order permutations are not explored. Each load enumerates pickup and delivery gaps with pickup at or before delivery. The winner is the feasible sequence with the lowest road duration, then the lowest pickup ordinal, then the lowest delivery ordinal. Discarded parents are not expanded. This is not an optimal search and it does not prove global infeasibility.

```text
SEARCH_ALGORITHM=BOUNDED_INCREMENTAL_HEURISTIC
UNRESTRICTED_PERMUTATION_SEARCH=NO
GLOBAL_SEQUENCE_SPACE_EXHAUSTIVE=NO
GLOBAL_INFEASIBILITY_PROVEN=NO
LOAD_ORDER=LEXICOGRAPHIC_LOAD_ID
LOAD_ORDER_PERMUTATIONS_EXPLORED=NO
MAX_ADDITIONAL_LOADS=2
MAX_ROUTE_LOAD_SUBJECTS=4
MAX_STOPS=8
MAX_SEQUENCE_CANDIDATES=64
MAX_ROUTE_LEG_EVALUATIONS=300
MAX_ROUTING_PROVIDER_CALLS=300
MAX_GROUPAGE_EVALUATIONS=600
TIME_BUDGET=5s
TIME_BUDGET_IS_SLO=NO
```

On four future stops the heuristic evaluates 43 sequences. Retaining every first-load parent would evaluate 435. `43 != exhaustive 435`. A finished heuristic with no plan is `NO_PLAN_FOUND_WITHIN_POLICY`. A budget interrupt is `SEARCH_BUDGET_EXHAUSTED`. `NO_FEASIBLE_SEQUENCE` is not a public result.

`MAX_ROUTE_LOAD_SUBJECTS=4` is the total of base subjects plus requested additional loads. It is not justified as two onboard plus two additional. NLO-0.3C does not cap onboard units at 2. Exceeding 4 is `PLAN_LOAD_LIMIT_EXCEEDED` and does not truncate onboard cargo. Requested additional loads stay at most 2.

Road routing is required. Haversine is not a substitute. Provider calls cannot exceed leg evaluations, and cache cannot enlarge the structural search. `UNIQUE_LOCATION_PAIRS` is not the provider-call count because departure buckets can split one origin-destination pair. Groupage work is structurally bounded at 600 evaluations.

Time is forward propagation. Waiting until a window opens is allowed. Late arrival is `STOP_WINDOW_VIOLATION`. Unknown service duration is indeterminate, not zero.

## Consequences

No solver is introduced. These caps are accepted. NLO-0.4B applies them as runtime policy. `NLO_0_4B_STATUS=IMPLEMENTED_CLOSED`.
