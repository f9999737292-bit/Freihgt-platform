# BINTRANS Product Roadmap 2026–2027 — NEXT

```text
STAGE=PRODUCT-ROADMAP-NEXT-2026-2027-v0.1
KIND=STRATEGIC_ARCHITECTURE_FREEZE
DOCS_ONLY=YES
PRODUCT_CODE_CHANGED=NO
RUNTIME_IMPLEMENTATION_AUTHORIZED=NO
MIGRATION_AUTHORIZED=NO
STAGING_AUTHORIZED=NO
MERGE_AUTHORIZED=NO
```

This document freezes the product **NEXT** architecture themes for 2026–2027. It does **not** authorize agents to start implementation, create migrations, change OpenAPI, mutate staging, or change production.

A later Task Contract with explicit authorization is required for every implementation slice.

## Purpose

Move BINTRANS from a working operational TMS baseline toward a coherent next product layer:

| Theme | Strategic ADR | Intent |
| --- | --- | --- |
| Agent-ready Control Tower | [ADR-PLAT-002](../adr/ADR-PLAT-002-agent-ready-action-model.md) | Recommend actions; humans or authorized workflows execute |
| Canonical analytics | [ADR-AN-006](../adr/ADR-AN-006-canonical-analytics-semantic-layer.md) | One KPI meaning, versioned semantic layer |
| Progressive network optimization | [ADR-NET-026](../adr/ADR-NET-026-progressive-network-optimization.md) | Grow NLO by bounded stages; never claim a global solver |
| Execution → document → settlement | [ADR-FIN-001](../adr/ADR-FIN-001-execution-document-settlement-chain.md) | Preserve ownership across the money chain |

## Non-authorization

```text
ROADMAP_IS_IMPLEMENTATION_PERMIT=NO
ADR_ACCEPTANCE_IS_IMPLEMENTATION_PERMIT=NO
AGENT_MAY_START_NEXT_IMPLEMENTATION_FROM_THIS_DOC=NO
CONTROL_TOWER_READ_MODEL_MODE_PRIMARY_AUTHORIZED=NO
AUTONOMOUS_DOMAIN_MUTATION_AUTHORIZED=NO
```

Existing stage freezes remain in force until a newer authorized contract supersedes them. Examples:

- Analytics-0.1B architecture and `docs/architecture/analytics/**`
- NLO stage freezes under `docs/architecture/network-optimizer/**`
- Finance-0.1B ownership and money foundations under `docs/architecture/finance-accounting/**`
- Finance-0.1D accounting-document rule freeze: [FINANCE_0_1D_ACCOUNTING_DOCUMENT_RULES.md](finance-accounting/FINANCE_0_1D_ACCOUNTING_DOCUMENT_RULES.md) (open legal questions remain open; runtime not authorized)
- EDO ADRs under `docs/adr/ADR-EDO-*`
- Customer portal and Control Tower rollout docs already on main

## Current baseline (context only)

BINTRANS already has:

- API Gateway trust boundary and tenant-scoped services
- TMS core: orders, RFx, shipments, documents, billing/settlement foundations
- Control Tower operational read model (shadow-first; primary mode separately gated)
- Network Optimizer progressive stages through chain foundation workstreams
- Analytics source inventory and architecture freeze without a warehouse claim
- Finance money, commercial-relationship, and accounting-document rule freezes without full AR/AP runtime

This roadmap does not restate every readiness score. It freezes **direction**.

## NEXT product pillars (2026–2027)

### 1. Agent-ready operations

Control Tower and related surfaces may **propose** actions with evidence, confidence, and required permissions. Domain services remain the only writers of their aggregates. Autonomous mutation of shipment, document, finance, or NLO state is out of scope until a dedicated authorized wave.

### 2. Canonical analytics semantic layer

Business KPIs have one canonical definition and version. Frontends and Control Tower do not invent alternate business formulas. Analytics derives; it does not invent missing source facts. Historical warehouse claims stay deferred until source contracts and history classes are ready.

### 3. Progressive network optimization

NLO grows by measured stages: capacity/load → prediction → consolidation/current-trip → route plan → backhaul → urban profiles → bounded chain → network-scale jobs. Each stage needs its own authorization. A global optimum claim is forbidden.

### 4. Execution–document–settlement chain

Physical execution facts stay with shipment/TMS. Legal artifacts stay with EDO/document-service. Monetary truth stays with billing/finance. Settlement identity includes commercial relationship. PaymentObligation is not Receivable. Customer-side and supply-side forwarder chains stay separate.

## Recommended sequencing (planning only)

This sequence is a controller planning hint. It is not a work queue and does not start agents.

1. Close remaining operational portal and Control Tower observation gates already in flight under separate contracts.
2. Keep EDO durable storage and least-privilege staging on their own authorization path.
3. Advance analytics only through explicit KPI promotion tasks after source contracts exist.
4. Advance NLO only through the published stage ladder and per-stage Task Contracts.
5. Advance finance settlement/document close only after money, tax, and relationship freezes have an authorized implementation slice.

## Explicitly deferred

| Item | Why deferred |
| --- | --- |
| Autonomous agent writes to domain APIs | Trust and audit model not authorized |
| Analytics DWH as v1 requirement | Source gaps and history classes incomplete |
| Global NLO solver | Contradicts progressive bounded design |
| Merging PaymentObligation into Receivable | Forbidden by ADR-EDO-005 / Finance-0.1B |
| `CONTROL_TOWER_READ_MODEL_MODE=primary` | Separate live gate |
| Production pilot GO | Separate readiness program |

## Relationship to earlier product roadmap

[PRODUCT_NEXT_ITERATION_ROADMAP_V0.1.md](../PRODUCT_NEXT_ITERATION_ROADMAP_V0.1.md) remains a historical internal-iteration planning note. This 2026–2027 NEXT document is the strategic architecture freeze for the themes above. It does not cancel prior stage freezes.

## Safety

```text
PRODUCT_CODE_CHANGED=NO
MIGRATION_CREATED=NO
OPENAPI_CHANGED=NO
STAGING_CHANGED=NO
RUNTIME_IMPLEMENTATION_AUTHORIZED=NO
```
