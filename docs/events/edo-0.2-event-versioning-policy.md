# EDO-0.2 Event Versioning Policy

## Status

Architecture freeze (EDO-0.2)

## Scope

Applies to all new EDO/TEDO/MM/FF events and evolution of existing LOG/FC/FF bus contracts. **No Kafka topic implementation in EDO-0.2.**

## Compatibility rules

### Backward compatible (minor / same version)

- Add optional JSON fields
- Add new enum values at end of consumer switch (consumers must ignore unknown)
- Add new event types in new namespace

### Breaking (requires new version)

- Remove or rename fields
- Change field type or semantic meaning
- Change `aggregate_id` meaning
- Tighten required fields previously optional

**Policy:** Publish `version=N+1` in parallel; mark `version=N` as DEPRECATED; minimum 2 release overlap before consumer removal.

## Schema evolution rules

1. **Schema ownership** — workstream that owns aggregate owns JSON schema (`docs/events/schemas/{namespace}/` future layout).
2. **Envelope stability** — common envelope fields (see ADR-EDO-006) are versioned separately from payload.
3. **Registry** — event catalog markdown is SSOT until JSON Schema lands in `packages/proto` or equivalent.
4. **Unknown fields** — producers must not require consumers to understand new fields; consumers must fail-soft on unknown fields.

## Idempotency policy

| Layer | Key |
|-------|-----|
| Producer outbox | `(tenant_id, event_type, aggregate_id, aggregate_version)` or explicit `source_event_id` |
| Consumer dedup | `(tenant_id, event_name, idempotency_key)` |
| HTTP outbox (billing/payment) | Existing unique constraints preserved |

**Rule:** Replays must not double-apply side effects. Consumers store processed idempotency keys per aggregate.

## Outbox requirements

| Producer type | Requirement |
|---------------|-------------|
| Kafka producers (shipment, future edo/tedo) | Transactional outbox in service schema; at-least-once publish |
| HTTP outbox (billing, payment) | Existing pattern; migrate to Kafka only via coordinated INFRA task |
| New EDO producers | Must use outbox from first implementation — no direct Kafka publish |

Outbox row must include: `event_id`, `event_type`, `aggregate_id`, `aggregate_version`, `payload`, `occurred_at`, publish status.

## Consumer retry policy

| Error class | Action |
|-------------|--------|
| Transient (network, DB timeout) | Exponential backoff retry; max attempts configurable |
| Parse / schema mismatch (unknown version) | DLQ after N attempts; alert |
| Permanent business rejection (tenant mismatch, unknown aggregate) | DLQ immediately; no retry |
| Idempotent duplicate | ACK without side effect |

Align with existing CT consumer: permanent errors → DLQ (`control-tower-read-model-service` pattern).

## Dead-letter policy

- DLQ topic or table per consumer service
- Manual replay tooling required before production EDO events
- DLQ messages retain original envelope + failure reason + first/last attempt timestamps

## Ordering assumptions

| Scope | Assumption |
|-------|------------|
| Single aggregate | Total order guaranteed by outbox `aggregate_version` monotonic increase |
| Cross-aggregate | No global order; use `causation_id` / `correlation_id` for sagas |
| Shipment vs document | Document events may arrive after shipment events; consumers must tolerate |

## Breaking-change approval

Breaking changes require:

1. ADR or CROSS_WORKSTREAM_REQUEST with `BREAKING_CHANGE=YES`
2. Consumer inventory updated in event catalog
3. Migration window documented in release manifest

## References

- ADR-EDO-006
- `services/shipment-service/internal/domain/outbox.go`
- `docs/engineering/FREIGHT_PAYMENTS_RECONCILIATION_v1.9.2_ARCHITECTURE.md` (payment outbox idempotency)
