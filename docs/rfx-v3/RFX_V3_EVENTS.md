# RFx v3.0A — Event Architecture

**Status:** Architecture draft  
**Platform pattern:** Transactional outbox (`docs/SHIPMENT_STATUS_OUTBOX.md`)

---

## 1. Principles

1. **Transactional outbox** — domain write + outbox insert in same PostgreSQL transaction.
2. **At-least-once delivery** — consumers idempotent on `(event_type, aggregate_id, aggregate_version)`.
3. **Tenant partition key** — `tenant_id` on all events and outbox rows.
4. **No PII in topic keys** — PII stays in payload with access controls.
5. **Schema versioning** — `schema_version` on envelope; breaking changes → new event version suffix.

Target table: `rfx.rfx_event_outbox` (see [RFX_V3_DATA_MODEL.md](./RFX_V3_DATA_MODEL.md) §3.7).

---

## 2. Envelope (reuse platform convention)

```json
{
  "eventId": "uuid",
  "eventType": "rfx.response.submitted.v1",
  "schemaVersion": 1,
  "tenantId": "uuid",
  "aggregateType": "RFX_RESPONSE",
  "aggregateId": "uuid",
  "aggregateVersion": 3,
  "occurredAt": "2026-09-03T20:00:00Z",
  "payload": { }
}
```

---

## 3. Event catalogue

### `rfx.created.v1`

| Attribute | Value |
|---|---|
| Producer | `rfx-service` (event create) |
| Consumers | Audit indexer, analytics (future), notification planner |
| Partition key | `tenant_id` |
| Payload owner | `rfx-service` — event id, type, owner_company_id |
| PII | Minimal — no free-text description in event |
| Outbox | Yes — same TX as `rfx_events` insert |
| Idempotency | `aggregate_id` + version 1 |

### `rfx.version.created.v1`

| Producer | `rfx-service` (publish / new version) |
| Consumers | Change impact worker, participant notification |
| Partition key | `tenant_id` |
| Payload | `rfx_event_id`, `version_number`, `published_at` |
| Outbox | Yes |
| Idempotency | `(rfx_event_id, version_number)` |

### `rfx.questionnaire.updated.v1`

| Producer | `rfx-service` (draft save) |
| Consumers | Preview session invalidation, collaboration sync |
| Partition key | `tenant_id` |
| Payload | `rfx_version_id`, changed section/question ids |
| Outbox | Optional for draft; required on publish |

### `rfx.published.v1`

| Producer | `rfx-service` (lifecycle transition) |
| Consumers | Notification service, Control Tower, participant invite worker |
| Partition key | `tenant_id` |
| Payload | `rfx_event_id`, `rfx_version_id`, deadline |
| Outbox | Yes — same TX as status + version publish |
| Idempotency | `rfx_version_id` |

### `rfx.participant.invited.v1`

| Producer | `rfx-service` |
| Consumers | Notification service (email/in-app) |
| Partition key | `tenant_id` |
| Payload | `participant_id`, `company_id`, `rfx_event_id` |
| PII | Company name only; no user email in payload (notification service resolves) |

### `rfx.response.started.v1`

| Producer | `rfx-service` (first valid save or explicit start) |
| Consumers | Audit, analytics, buyer dashboard |
| Partition key | `tenant_id` |
| Payload | `response_id`, `participant_company_id`, `rfx_version_id` |

### `rfx.answer.updated.v1`

| Producer | `rfx-service` (autosave commit) |
| Consumers | Collaboration sync, progress tracker |
| Partition key | `tenant_id` |
| Payload | `response_id`, `question_ids[]`, `save_version` |
| PII | Answer values **not** in event — consumers fetch via API if authorized |
| Idempotency | `(response_id, save_version)` |

### `rfx.response.submitted.v1`

| Producer | `rfx-service` |
| Consumers | Scoring engine, notification, audit, evaluation workspace |
| Partition key | `tenant_id` |
| Payload | `response_id`, `submitted_at`, `submitted_by` |
| Outbox | Yes — same TX as status SUBMITTED |
| Idempotency | `response_id` + submit timestamp |

### `rfx.knockout.triggered.v1`

| Producer | Scoring engine (within rfx-service) |
| Consumers | Buyer alerts, qualification status, audit |
| Partition key | `tenant_id` |
| Payload | `response_id`, `question_id`, `knockout_code`, `rule_version` |

### `rfx.score.calculated.v1`

| Producer | Scoring engine |
| Consumers | Evaluation UI refresh, qualification pool updater, analytics |
| Partition key | `tenant_id` |
| Payload | `response_id`, `total_score`, `score_model_version`, `qualification_status` |
| Idempotency | `(response_id, score_model_version, calculation_seq)` |

### `rfx.carrier.qualified.v1`

| Producer | Qualification service |
| Consumers | Qualification pool, notification, Control Tower |
| Partition key | `tenant_id` |
| Payload | `response_id`, `carrier_company_id`, `pool_id` (optional) |

### `rfx.carrier.conditionally_qualified.v1`

| Producer | Qualification service |
| Consumers | Buyer review queue, notification |
| Payload | `response_id`, `conditions_json` |

### `rfx.carrier.rejected.v1`

| Producer | Qualification service |
| Consumers | Carrier notification (policy-controlled), audit |
| Payload | `response_id`, `knockout_reason_json`, `score_model_version` |

### `rfx.closed.v1`

| Producer | `rfx-service` (lifecycle) |
| Consumers | Analytics, archive jobs, notification |
| Partition key | `tenant_id` |
| Payload | `rfx_event_id`, `closed_reason`, `final_status` |

---

## 4. Kafka / broker conventions

Reuse BINTRANS patterns from shipment outbox:

| Concern | Convention |
|---|---|
| Topic naming | `bintrans.rfx.{event-name}.v1` (environment prefix in deployment) |
| Message key | `{tenant_id}:{aggregate_id}` |
| Consumer groups | `{service}-rfx-{event}-v1` |
| Failed publish | `FAILED` status + `last_error_code`; ops runbook pattern |
| Replay | From outbox `PENDING`/`FAILED` rows — not from domain tables |

Current state: RFx outbox table **not yet migrated**; audit writes to `rfx.audit_events` synchronously (`000037`).

---

## 5. PII & security

| Rule | Value |
|---|---|
| Answer values in events | **Avoid** — use reference ids |
| Preview session events | **None** — preview is ephemeral |
| Cross-tenant consumption | **Denied** |
| Event payload tenant check | Consumer validates `tenantId` matches subscription scope |

---

## 6. References

- [RFX_V3_DATA_MODEL.md](./RFX_V3_DATA_MODEL.md) — `rfx_event_outbox`
- [docs/SHIPMENT_STATUS_OUTBOX.md](../SHIPMENT_STATUS_OUTBOX.md)
- [RFX_V3_SECURITY.md](./RFX_V3_SECURITY.md)
- Current audit: `infrastructure/migrations/000037_add_rfx_audit_events_v1.0.up.sql`
