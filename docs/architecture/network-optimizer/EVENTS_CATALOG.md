# Event catalog

Naming follows ADR-EDO-006: `{namespace}.{aggregate}.{past_tense_verb}`, lowercase, dot-separated. New work uses the `network.*` namespace. Existing `shipment.*` and `driver.*` names stay. This freeze does not publish events and does not rename live ones.

The suggested names `shipment.eta.updated`, `capacity.available`, and `marketplace.load.published` are **not** adopted. ETA is not a Kafka event today. Availability and publication use past-tense `network.*` names below.

## Shared envelope

Aligned to ADR-EDO-006. Runtime JSON for existing shipment events is camelCase; new schemas must define both the canonical fields and the JSON names in one contract so consumers do not guess.

| Field | Rule |
|-------|------|
| `event_name` | Canonical string |
| `version` | Integer, start at 1 |
| `producer` | Service name |
| `aggregate_id` | UUID |
| `tenant_id` | Owner tenant of the aggregate |
| `occurred_at` | UTC |
| `correlation_id` | Required |
| `causation_id` | Parent event when reoptimizing |
| `idempotency_key` | `{event_name}:{aggregate_id}:{version_or_seq}` |
| schema | Owned by network-optimizer or the producer named in the row |

Ordering: total order per aggregate. Partition key is the aggregate id unless the row says otherwise. Retry: at-least-once from a transactional outbox in the producer schema. Consumers dedupe on `(tenant_id, event_name, idempotency_key)`. Poison messages go to a dead-letter table in the consumer schema, following the Control Tower inbox/DLQ pattern (`control_tower.shipment_status_event_dead_letter`). No new topic is created in this freeze.

PII classes: `NONE`, `OPERATIONAL`, `COMMERCIAL`, `IDENTITY`, `TRACKING`.

## Existing events the optimizer may consume later

These exist in code. The optimizer does not redefine them.

| event name | owner | producer | consumer | partition key | PII | Notes |
|------------|-------|----------|----------|---------------|-----|-------|
| `shipment.created` | shipment-service | shipment outbox | control tower; future network | shipment id | OPERATIONAL | Topic `shipment.status.v1` |
| `shipment.status.changed` | shipment-service | shipment outbox | control tower; future network | shipment id | OPERATIONAL | |
| `shipment.cancelled` | shipment-service | shipment outbox | control tower; future network | shipment id | OPERATIONAL | Invalidates plans that cite the shipment |
| `driver.location.updated` | shipment-service | driver outbox | control tower on `driver.events.v1` | shipment id | TRACKING | Not a public marketplace event |
| `driver.arrived_at_delivery` | shipment-service | driver outbox | control tower | shipment id | OPERATIONAL | |
| `driver.delivery.completed` | shipment-service | driver outbox | control tower | shipment id | OPERATIONAL | |
| `driver.delay.reported` | shipment-service | driver outbox | control tower | shipment id | OPERATIONAL | Re-evaluate trigger |
| `driver.tracking.lost` | shipment-service | driver outbox | control tower | shipment id | TRACKING | Lowers confidence |

`tracking.eta.updated` is **PLANNED**. It is not in the tracking-service Kafka publisher (tracking ingest is HTTP). Owner must be tracking-service. The optimizer must not publish ETA under `shipment.*`.

## Proposed `network.*` events

Producer is the future network-optimizer context. Consumers are the owning carrier or shipper projection, Control Tower (facts only), and the optimizer's own reoptimization worker. Schema version 1. Idempotency, retry, and DLQ use the shared envelope.

| event name | aggregate | partition key | PII | When |
|------------|-----------|---------------|-----|------|
| `network.capacity.predicted` | capacity | capacity id | TRACKING | Rule-based or later provider wrote a prediction |
| `network.capacity.updated` | capacity | capacity id | OPERATIONAL | Mutable capacity fields changed |
| `network.capacity.published` | capacity | capacity id | OPERATIONAL | Became visible under a scope |
| `network.capacity.withdrawn` | capacity | capacity id | NONE | Publisher pulled it |
| `network.capacity.expired` | capacity | capacity id | NONE | Window ended |
| `network.load_opportunity.published` | load opportunity | load opportunity id | COMMERCIAL | Explicit publication |
| `network.load_opportunity.updated` | load opportunity | load opportunity id | COMMERCIAL | Published fields changed |
| `network.load_opportunity.withdrawn` | load opportunity | load opportunity id | NONE | |
| `network.load_opportunity.expired` | load opportunity | load opportunity id | NONE | |
| `network.match.generated` | match | match id | COMMERCIAL | Feasible candidate persisted |
| `network.match.invalidated` | match | match id | NONE | Input changed |
| `network.consolidation_candidate.generated` | consolidation | candidate id | OPERATIONAL | Hard constraints passed; commercial fields stay off this event |
| `network.route_plan.generated` | route plan | route plan id | OPERATIONAL | |
| `network.chain.generated` | chain | chain id | OPERATIONAL | |
| `network.chain.marked_at_risk` | chain | chain id | OPERATIONAL | Slack or window broken |
| `network.chain.reoptimized` | chain | chain id | OPERATIONAL | New plan linked |
| `network.carrier_offer.created` | offer | offer id | COMMERCIAL | |
| `network.carrier_offer.sent` | offer | offer id | COMMERCIAL | |
| `network.carrier_offer.viewed` | offer | offer id | NONE | |
| `network.carrier_offer.accepted` | offer | offer id | COMMERCIAL | |
| `network.carrier_offer.countered` | offer | offer id | COMMERCIAL | |
| `network.carrier_offer.rejected` | offer | offer id | NONE | |
| `network.carrier_offer.expired` | offer | offer id | NONE | |
| `network.carrier_offer.cancelled` | offer | offer id | NONE | |
| `network.assignment.created` | assignment | assignment id | OPERATIONAL | Winner recorded; does not itself create a shipment |

Tenant context on every row is the aggregate owner's `tenant_id`. Cross-shipper consolidation events omit the other shipper's rate and identity from the payload. Internal ids of the other party are not on the externalized schema.

Billing, RFx, and EDO events stay in their namespaces (`freight_settlement.*`, `billing_register.*`, future `edo.document.*`). The optimizer does not emit them.
