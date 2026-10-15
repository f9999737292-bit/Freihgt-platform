# BINTRANS Architecture Decision Records

## Status

Active cross-program BINTRANS ADR registry. It covers EDO / TEDO / platform, analytics, network optimizer, finance / settlement, tracking, and related product decisions. Program-local ADR trees (for example under `docs/architecture/analytics/adr/` and `docs/architecture/network-optimizer/adr/`) remain binding for their scopes and are indexed here when promoted to strategic NEXT records.

## Numbering convention

| Prefix | Scope | Examples |
|--------|-------|----------|
| `ADR-EDO-*` | Electronic document exchange core | Document ownership, archive, events |
| `ADR-TEDO-*` | Transport EDO / EPD | Operator port, ETRN lifecycle |
| `ADR-PLAT-*` | Platform / identity / cross-cutting | Membership, RBAC, agent-ready actions |
| `ADR-AN-*` | Analytics / KPI semantic layer | Canonical KPI definitions, semantic layer |
| `ADR-NET-*` | Network optimizer | Progressive optimization and stage freezes |
| `ADR-FIN-*` | Finance / settlement | Execution–document–settlement chain |
| `ADR-MM-*` | Multimodal logistics | Reserved for MM-specific decisions |
| `ADR-FF-*` | Freight finance / factoring | Reserved for FF-specific decisions |
| `ADR-TRACK-*` | Tracking provider boundary | External provider ingest contract |

EDO-0.2 introduces `ADR-EDO-001` … `ADR-EDO-009`. Membership ownership is recorded as `ADR-PLAT-001` because it predates EDO implementation and affects PLAT workstream.

## ADR pack (EDO-0.2)

| ID | Title | Status |
|----|-------|--------|
| [ADR-EDO-001](ADR-EDO-001-canonical-edo-document-ownership.md) | Canonical EDO document ownership | Accepted (freeze) |
| [ADR-EDO-002](ADR-EDO-002-billing-edo-boundary.md) | Billing ↔ EDO boundary (F-009) | Accepted (freeze) |
| [ADR-EDO-003](ADR-EDO-003-canonical-shipment-multimodal-extension.md) | Canonical Shipment and multimodal extension | Accepted (freeze) |
| [ADR-EDO-004](ADR-EDO-004-epd-operator-port-ownership.md) | EPD operator port ownership | Accepted (freeze) |
| [ADR-EDO-005](ADR-EDO-005-receivable-vs-payment-obligation.md) | Receivable vs PaymentObligation | Accepted (freeze) |
| [ADR-EDO-006](ADR-EDO-006-event-naming-versioning.md) | Event naming and versioning | Accepted (freeze) |
| [ADR-EDO-007](ADR-EDO-007-legal-archive-boundary.md) | Legal archive boundary | Accepted (freeze) |
| [ADR-EDO-008](ADR-EDO-008-external-operator-first-own-operator-ready.md) | External operator first / own operator ready | Accepted (freeze) |
| [ADR-EDO-009](ADR-EDO-009-cross-workstream-mutation-policy.md) | Cross-workstream mutation policy | Accepted (freeze) |
| [ADR-PLAT-001](ADR-PLAT-001-membership-user-roles-canonical-writer.md) | Membership and user_roles canonical writer (F-002) | Accepted (freeze) |

## Later EDO records

The EDO-0.2 table above is unchanged. ADR-EDO-010 and ADR-EDO-011 stay proposed. ADR-EDO-012 is the durable blob runtime contract and does not replace ADR-EDO-007.

| ID | Title | Status |
|----|-------|--------|
| [ADR-EDO-010](ADR-EDO-010-attachment-signature-verification.md) | Attachment signature verification | Proposed — architecture only |
| [ADR-EDO-011](ADR-EDO-011-production-gost-verifier.md) | Production GOST verifier | Proposed — discovery only |
| [ADR-EDO-012](ADR-EDO-012-durable-object-storage-runtime-contract.md) | Durable object storage runtime contract | Accepted (runtime freeze, no provisioning) |

## Tracking provider contract

| ID | Title | Status |
|----|-------|--------|
| [ADR-TRACK-001](ADR-TRACK-001-external-provider-ingest-contract.md) | External provider ingest contract | Accepted — provider-neutral contract freeze |

## Strategic NEXT records

These records freeze 2026–2027 product direction. They do **not** authorize agents to start NEXT implementation, migrations, OpenAPI changes, or staging mutation. Implementation requires a separate Task Contract.

| ID | Title | Status |
|----|-------|--------|
| [ADR-PLAT-002](ADR-PLAT-002-agent-ready-action-model.md) | Agent-ready action model | Accepted — strategic architecture freeze; implementation separately authorized |
| [ADR-AN-006](ADR-AN-006-canonical-analytics-semantic-layer.md) | Canonical analytics semantic layer | Accepted — strategic architecture freeze; implementation separately authorized |
| [ADR-NET-026](ADR-NET-026-progressive-network-optimization.md) | Progressive network optimization | Accepted — strategic architecture freeze; implementation separately authorized |
| [ADR-FIN-001](ADR-FIN-001-execution-document-settlement-chain.md) | Execution–document–settlement chain | Accepted — strategic architecture freeze; implementation separately authorized |

Roadmap index: [BINTRANS_PRODUCT_ROADMAP_2026_2027_NEXT.md](../architecture/BINTRANS_PRODUCT_ROADMAP_2026_2027_NEXT.md).

Analytics-tree ADR-AN-001 through ADR-AN-005 under `docs/architecture/analytics/adr/` remain unchanged. Network Optimizer stage ADR-NET-001 through ADR-NET-025 under `docs/architecture/network-optimizer/adr/` remain unchanged. A parallel `ADR-NLO-*` registry prefix is not used.

## Related documents

- Discovery baseline: BINTRANS ECOSYSTEM + EDO ARCHITECTURE DISCOVERY v0.1 (`DISCOVERY_PASS_WITH_FINDINGS`)
- Freeze report: [docs/architecture/edo-0.2-final-report.md](../architecture/edo-0.2-final-report.md)
- EDO 0.3 discovery (not an accepted ADR, implementation not authorized): [docs/architecture/edo-0.3-discovery.md](../architecture/edo-0.3-discovery.md)
- Event contracts: [docs/events/edo-0.2-event-contracts.md](../events/edo-0.2-event-contracts.md)
- Document completeness source contract (not an accepted runtime): [docs/architecture/edo-document-completeness-source-v0.4b.md](../architecture/edo-document-completeness-source-v0.4b.md)
- Document requirement policy framework (discovery freeze, not runtime): [docs/architecture/edo-document-requirement-policy-v0.1a.md](../architecture/edo-document-requirement-policy-v0.1a.md)
