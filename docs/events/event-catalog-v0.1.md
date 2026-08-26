# Event Catalog v0.1

> Index — superseded for detail by EDO-0.2 event contracts.

## Status

**Audited** against repository implementation (discovery v0.1 + EDO-0.2 freeze). Kafka and outbox events documented in linked catalog.

## Canonical catalogs

| Document | Scope |
|----------|-------|
| [edo-0.2-event-contracts.md](edo-0.2-event-contracts.md) | Full event inventory: EXISTING, NEW, ALIAS, DEPRECATED |
| [edo-0.2-event-versioning-policy.md](edo-0.2-event-versioning-policy.md) | Compatibility, idempotency, outbox, DLQ |
| [ADR-EDO-006](../adr/ADR-EDO-006-event-naming-versioning.md) | Naming rules and namespaces |

## Implemented namespaces (summary)

| Namespace | Transport | Producer |
|-----------|-----------|----------|
| `shipment.*` | Kafka `shipment.status.v1` | shipment-service |
| `driver.*` | Kafka `driver.events.v1` | shipment-service |
| `driver.task_*` | DB outbox (partial) | shipment-service |
| `payment_obligation.*` | HTTP outbox | payment-service |
| `freight_settlement.*`, `billing_register.*` | HTTP outbox | billing-register-service |

## Proposed namespaces (EDO-0.2 freeze)

| Namespace | Producer (future) |
|-----------|---------------------|
| `edo.document.*`, `edo.package.*` | document-service |
| `tedo.epd.*` | transport-edo-service |
| `mm.transport_leg.*`, `mm.cargo_handover.*` | shipment-service |
| `ff.receivable.*`, `ff.factoring.*` | payment-service / FF module |

## Event envelope (required fields)

See ADR-EDO-006: `event_name`, `version`, `producer`, `aggregate_id`, `tenant_id`, `occurred_at`, `correlation_id`, `idempotency_key`, payload.
