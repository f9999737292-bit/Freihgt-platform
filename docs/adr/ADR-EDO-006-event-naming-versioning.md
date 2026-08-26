# ADR-EDO-006: Event Naming and Versioning

## Status

Accepted — architecture freeze (EDO-0.2)

## Context

Implemented events use dot-separated lowercase names (`shipment.created`, `driver.location.updated`, `payment_obligation.paid`). Gateway derives timeline strings that duplicate document lifecycle naming. `docs/events/event-catalog-v0.1.md` is a placeholder understating real events (discovery F-001, F-006).

## Decision

### Canonical naming rules

1. **Format:** `{namespace}.{aggregate_or_resource}.{past_tense_verb}` or `{namespace}.{past_tense_verb}` for well-known singleton transitions.
2. **Case:** lowercase ASCII, segments separated by `.` — no PascalCase bus names.
3. **Namespaces (preferred for new events):**

| Namespace | Workstream | Examples |
|-----------|------------|----------|
| `edo.document.*` | EDO | `edo.document.created`, `edo.document.signed` |
| `edo.package.*` | EDO | `edo.package.completed` |
| `tedo.epd.*` | TEDO | `tedo.epd.submitted`, `tedo.epd.operator_status_changed` |
| `mm.transport_leg.*` | MM / LOG | `mm.transport_leg.added`, `mm.cargo_handover.recorded` |
| `ff.receivable.*` | FF | `ff.receivable.created` |
| `ff.factoring.*` | FF | `ff.factoring.assignment.noticed` |

4. **Existing namespaces — do not rename without deprecation window:**

| Namespace | Status |
|-----------|--------|
| `shipment.*` | EXISTING — canonical LOG execution events |
| `driver.*` | EXISTING — canonical driver execution events |
| `payment_obligation.*` | EXISTING — payment outbox |
| `billing_register.*`, `freight_settlement.*` | EXISTING — billing HTTP outbox |

5. **Gateway timeline names** (`document.created`, `document.signed`) — **ALIAS** of future `edo.document.*` Kafka events for HTTP read model only; not bus SSOT.

6. **Do not duplicate shipment events** at document level when shipment-level signal suffices (e.g. keep `shipment.documents_completed`; add `edo.document.completed` only for per-document lifecycle).

### Event envelope (required fields)

Every published event specification must define:

| Field | Required | Notes |
|-------|----------|-------|
| `event_name` | yes | Canonical string |
| `version` | yes | Integer schema version |
| `producer` | yes | Service name |
| `consumer_candidates` | yes | List |
| `aggregate_id` | yes | UUID of owning aggregate |
| `tenant_id` | yes | Tenant scope |
| `company_id` | where applicable | Acting or owning company |
| `occurred_at` | yes | UTC timestamp |
| `correlation_id` | yes | Trace across services |
| `causation_id` | optional | Parent event id |
| `idempotency_key` | yes | `{event_name}:{aggregate_id}:{version_or_seq}` |
| `schema_ownership` | yes | Workstream owning JSON schema |

### Versioning policy summary

See [docs/events/edo-0.2-event-versioning-policy.md](../events/edo-0.2-event-versioning-policy.md) for full rules.

- **Additive changes** — new optional fields, new event version; consumers must ignore unknown fields.
- **Breaking changes** — new `version` with parallel publish window; old version marked DEPRECATED.
- **Idempotency** — consumers dedupe on `(tenant_id, event_name, idempotency_key)`.
- **Outbox** — all Kafka producers use transactional outbox in owning service schema.
- **Ordering** — per-aggregate total order assumed; cross-aggregate causal order via `causation_id` only.

## Consequences

- Event catalog must be updated from code audit (see `edo-0.2-event-contracts.md`)
- CT and gateway gradually subscribe to `edo.document.*` instead of inferring from HTTP

## References

- Discovery findings F-001, F-006
- `services/shipment-service/internal/domain/outbox.go`
