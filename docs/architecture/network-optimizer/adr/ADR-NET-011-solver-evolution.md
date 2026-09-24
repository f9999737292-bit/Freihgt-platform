# ADR-NET-011: Solver evolution

## Status

Proposed — architecture freeze v0.1. Implementation not authorized.

## Context

Problem classes range from one next load to urban VRP and a private fleet of tens of vehicles. No solver exists. Scale numbers are not known.

## Decision

Do not pick a single mathematical method. Stage the core: rules and greedy ranking, then local improvement, then a VRP or CP-SAT adapter where measurements justify it. Hybrid is the long-term shape. MILP and CP-SAT are for small and medium fleet instances inside a time budget. Global network optimum is NLO-1.0, not the start. Every stage keeps hard rejects and `score_explanation`.

## Consequences

NLO-0.1 and NLO-0.2 can ship without a VRP dependency. Later methods must preserve the explanation contract.
