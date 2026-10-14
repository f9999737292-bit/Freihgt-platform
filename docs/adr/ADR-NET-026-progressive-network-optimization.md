# ADR-NET-026: Progressive network optimization

## Status

Accepted — strategic architecture freeze; implementation separately authorized

```text
ADR_NET_026=ACCEPTED
RUNTIME_IMPLEMENTATION_AUTHORIZED=NO
GLOBAL_CHAIN_OPTIMIZATION_CLAIM_ALLOWED=NO
GLOBAL_NETWORK_OPTIMUM_CLAIM_ALLOWED=NO
```

## Context

Network Optimizer work on main already follows a staged ladder documented under `docs/architecture/network-optimizer/`: capacity and load foundation, prediction, consolidation and current-trip fill, route-plan execution boundaries, backhaul budgets, urban profiles, and bounded chain architecture. Stage ADRs in that tree use the `ADR-NET-*` namespace through ADR-NET-025.

NEXT needs a registry-level rule: optimization grows by progressive, measured stages. Agents must not jump to an unbounded solver or claim global optimality. This record continues the same `ADR-NET` namespace as ADR-NET-026. A parallel `ADR-NLO-*` registry prefix is not created.

## Decision

### 1. Progressive stage ladder

Later stages add problem classes; they do not silently replace earlier contracts.

Illustrative ladder (existing stage docs own the exact status of each row):

| Band | Problem class |
| --- | --- |
| NLO-0.1 | Capacity / load foundation, hard feasibility, candidates |
| NLO-0.2 | Predictive next load from shipment + tracking ETA |
| NLO-0.3 | Residual / pairwise / current-trip planning |
| NLO-0.4 | Route plan and execution boundary for multi-stop |
| NLO-0.5 | Backhaul / roundtrip with discovery and routing budgets |
| NLO-0.6–0.8 | Regional and urban profile mechanisms |
| NLO-0.9 | Bounded chain search and reoptimization seeds |
| NLO-1.x | Measured larger network jobs after benchmarks |
| FLEET-1.x | Private fleet assignment on the same core |

Each band requires its own authorization before runtime work.

### 2. Bounds before breadth

Every public search or plan path must declare:

- subject cardinality caps
- discovery shortlist caps
- provider call and wall-clock budgets
- `execution_supported` honesty
- fail-closed behavior on missing evidence or indeterminate ownership

Unbounded N-load enumeration and “best network solution” marketing claims are forbidden.

### 3. Planning versus execution

A planning search may return candidates with `execution_supported=false`. Activating or mutating shipments, stops, or legs requires the execution-owning domain contracts (shipment / route-plan execution stages), not an NLO search response alone.

### 4. Tenancy and privacy

Search and ranking remain tenant-scoped with trusted identity. Cross-tenant visibility stays explicit publication and opt-in rules already frozen in stage docs. Public results stay privacy-safe. GPS is not silently promoted into a required input for stages that froze without it.

### 5. Registry relationship

`ADR-NET-*` is the single Network Optimizer ADR namespace. ADR-NET-001 through ADR-NET-025 remain unchanged stage freezes under `docs/architecture/network-optimizer/adr/`. This ADR is the strategic progressive-optimization product freeze as ADR-NET-026 and does not reopen closed stage acceptances or authorize the next runtime slice.

## Consequences

- Controllers sequence NLO work by stage, not by a single “optimizer epic.”
- Benchmarks and budgets gate claims of scale.
- Product UI must not promise multi-stop execution or chain optimality ahead of the owning stage.

## Non-authorization

This ADR does not authorize chain runtime expansion, migrations, OpenAPI changes, solver introduction, staging NLO enablement, or weakening of existing stage fail-closed rules.

## References

- [BINTRANS_PRODUCT_ROADMAP_2026_2027_NEXT.md](../architecture/BINTRANS_PRODUCT_ROADMAP_2026_2027_NEXT.md)
- [network-optimizer/ROADMAP.md](../architecture/network-optimizer/ROADMAP.md)
- [NLO_0_9B_CHAIN_ARCHITECTURE_FREEZE.md](../architecture/network-optimizer/NLO_0_9B_CHAIN_ARCHITECTURE_FREEZE.md)
