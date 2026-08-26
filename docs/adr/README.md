# Architecture Decision Records — EDO Program

## Status

Active registry for BINTRANS EDO / TEDO / MM / FF architecture decisions.

## Numbering convention

| Prefix | Scope | Examples |
|--------|-------|----------|
| `ADR-EDO-*` | Electronic document exchange core | Document ownership, archive, events |
| `ADR-TEDO-*` | Transport EDO / EPD | Operator port, ETRN lifecycle |
| `ADR-PLAT-*` | Platform / identity / cross-cutting | Membership, RBAC |
| `ADR-MM-*` | Multimodal logistics | Reserved for MM-specific decisions |
| `ADR-FF-*` | Freight finance / factoring | Reserved for FF-specific decisions |

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

## Related documents

- Discovery baseline: BINTRANS ECOSYSTEM + EDO ARCHITECTURE DISCOVERY v0.1 (`DISCOVERY_PASS_WITH_FINDINGS`)
- Freeze report: [docs/architecture/edo-0.2-final-report.md](../architecture/edo-0.2-final-report.md)
- Event contracts: [docs/events/edo-0.2-event-contracts.md](../events/edo-0.2-event-contracts.md)
