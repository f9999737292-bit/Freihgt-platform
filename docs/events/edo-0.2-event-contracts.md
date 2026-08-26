# EDO-0.2 Event Contracts

## Status

Architecture freeze — proposed specifications (EDO-0.2)

## Naming rules

See [ADR-EDO-006](../adr/ADR-EDO-006-event-naming-versioning.md) and [edo-0.2-event-versioning-policy.md](edo-0.2-event-versioning-policy.md).

## Legend

| Mark | Meaning |
|------|---------|
| EXISTING | Implemented in repository today |
| NEW | Proposed; not implemented |
| ALIAS | Alternate name for same semantic (deprecate over time) |
| DEPRECATED | Scheduled for removal after migration |
| DO_NOT_CREATE | Explicitly rejected duplicate |

---

## EXISTING — Kafka (`shipment.status.v1`)

**Producer:** shipment-service | **Schema ownership:** LOG

| event_name | version | consumer_candidates | aggregate_id | idempotency_key | Mark |
|------------|---------|---------------------|--------------|-----------------|------|
| `shipment.created` | 1 | control-tower-read-model-service | `shipment.id` | `{event_id}` | EXISTING |
| `shipment.status.changed` | 1 | control-tower-read-model-service | `shipment.id` | `{event_id}` | EXISTING |
| `shipment.cancelled` | 1 | control-tower-read-model-service | `shipment.id` | `{event_id}` | EXISTING |
| `shipment.ready_for_billing` | 1 | control-tower-read-model-service, FC (future) | `shipment.id` | `{event_id}` | EXISTING |
| `shipment.documents_completed` | 1 | control-tower-read-model-service | `shipment.id` | `{event_id}` | EXISTING |
| `shipment.financially_closed` | 1 | control-tower-read-model-service | `shipment.id` | `{event_id}` | EXISTING |

Envelope includes: `tenant_id`, `occurred_at`, `correlation_id` via outbox payload; `company_id` optional.

**DO_NOT_CREATE:** `ShipmentCreated`, `shipment.created.v2` duplicate topics, per-status duplicate events (`carrier.assigned` as separate bus event) — use `shipment.status.changed` with payload `toStatus`.

---

## EXISTING — Kafka (`driver.events.v1`)

**Producer:** shipment-service | **Schema ownership:** LOG

| event_name | version | consumer_candidates | aggregate_id | Mark |
|------------|---------|---------------------|--------------|------|
| `driver.location.updated` | 1 | control-tower-read-model-service, tracking-service | `shipment.id` | EXISTING |
| `driver.arrived_at_pickup` | 1 | CT, tracking | `shipment.id` | EXISTING |
| `driver.departed_pickup` | 1 | CT, tracking | `shipment.id` | EXISTING |
| `driver.arrived_at_delivery` | 1 | CT, tracking | `shipment.id` | EXISTING |
| `driver.delivery.completed` | 1 | CT, tracking | `shipment.id` | EXISTING |
| `driver.delay.reported` | 1 | CT | `shipment.id` | EXISTING |
| `driver.problem.reported` | 1 | CT | `shipment.id` | EXISTING |
| `driver.documents.uploaded` | 1 | CT | `shipment.id` | EXISTING |
| `driver.tracking.lost` | 1 | CT, tracking | `shipment.id` | EXISTING |
| `driver.tracking.restored` | 1 | CT, tracking | `shipment.id` | EXISTING |
| `driver.exception_reported` | 1 | CT | `shipment.id` | EXISTING (legacy) |
| `driver.shipment_event_recorded` | 1 | CT | `shipment.id` | EXISTING |

**DO_NOT_CREATE:** `PODReceived` as separate event — filter `driver.documents.uploaded` by document type or consume future `edo.document.created` with type POD.

---

## EXISTING — Outbox partial (`driver.task_*`)

**Producer:** shipment-service (DB outbox; partial Kafka) | **Schema ownership:** LOG

| event_name | Mark |
|------------|------|
| `driver.task_created` | EXISTING |
| `driver.task_completed` | EXISTING |
| `driver.task_expired` | EXISTING |
| `driver.task_cancelled` | EXISTING |

---

## EXISTING — HTTP outbox (billing-register-service)

**Producer:** billing-register-service → freight-cost-service HTTP | **Schema ownership:** FC

| event_name | version | consumer_candidates | aggregate_id | Mark |
|------------|---------|---------------------|--------------|------|
| `freight_settlement.accrual_snapshot.v1` | 1 | freight-cost-service | settlement.id | EXISTING |
| `freight_settlement.current_actual_snapshot.v1` | 1 | freight-cost-service | settlement.id | EXISTING |
| `freight_settlement.final_actual_snapshot.v1` | 1 | freight-cost-service | settlement.id | EXISTING |
| `billing_register.settlement_billing_link_snapshot.v1` | 1 | freight-cost-service | billing_register.id | EXISTING |
| `billing_register.payable_snapshot.v1` | 1 | freight-cost-service | billing_register.id | EXISTING |

---

## EXISTING — HTTP outbox (payment-service)

**Producer:** payment-service → freight-cost-service, billing-register-service | **Schema ownership:** FF

| event_name | version | consumer_candidates | aggregate_id | Mark |
|------------|---------|---------------------|--------------|------|
| `payment_obligation.paid` | 1 | freight-cost-service, billing-register-service | payment_obligation.id | EXISTING |
| `payment_obligation.paid_snapshot.v1` | 1 | freight-cost-service | payment_obligation.id | EXISTING |

**DO_NOT_CREATE:** `ReceivableCreated` as alias of `payment_obligation.paid` — distinct aggregate (ADR-EDO-005).

---

## ALIAS — Gateway timeline (api-gateway derived, not bus SSOT)

| event_name | Maps to | Mark |
|------------|---------|------|
| `document.created` | future `edo.document.created` | ALIAS |
| `document.signed` | future `edo.document.signed` | ALIAS |
| `document.rejected` | future `edo.document.rejected` | ALIAS |

---

## NEW — Proposed `edo.document.*`

**Producer:** document-service (future Kafka outbox) | **Schema ownership:** EDO | **Topic (future):** `edo.document.v1`

| event_name | version | consumer_candidates | aggregate_id | tenant_id | company_id | idempotency_key |
|------------|---------|---------------------|--------------|-----------|------------|-----------------|
| `edo.document.created` | 1 | CT, billing-register-service (mirror), api-gateway | document.id | required | owner_company_id | `{event_id}` |
| `edo.document.revision_added` | 1 | CT, archive | document.id | required | — | `{event_id}` |
| `edo.document.signature_state_changed` | 1 | CT, billing-register-service | document.id | required | signer_company_id | `{event_id}` |
| `edo.document.signed` | 1 | CT, billing, TEDO | document.id | required | signer_company_id | `{event_id}` |
| `edo.document.delivery_state_changed` | 1 | CT | document.id | required | — | `{event_id}` |
| `edo.document.business_state_changed` | 1 | billing-register-service, CT | document.id | required | — | `{event_id}` |
| `edo.document.operator_receipt_recorded` | 1 | CT, billing | document.id | required | operator_company_id | `{event_id}` |
| `edo.document.archived` | 1 | CT, compliance read models | document.id | required | — | `{event_id}` |
| `edo.document.voided` | 1 | billing-register-service, CT | document.id | required | — | `{event_id}` |
| `edo.document.validation_failed` | 1 | CT | document.id | required | — | `{event_id}` |

All include: `occurred_at`, `correlation_id`, optional `causation_id`, `shipment_id` when applicable.

---

## NEW — Proposed `edo.package.*`

| event_name | version | producer | consumer_candidates | aggregate_id | Mark |
|------------|---------|----------|---------------------|--------------|------|
| `edo.package.created` | 1 | document-service | CT | document_package.id | NEW |
| `edo.package.member_added` | 1 | document-service | CT | document_package.id | NEW |
| `edo.package.sealed` | 1 | document-service | CT, TEDO | document_package.id | NEW |
| `edo.package.exchange_completed` | 1 | document-service | CT, LOG | document_package.id | NEW |

---

## NEW — Proposed `tedo.epd.*`

**Producer:** transport-edo-service | **Schema ownership:** TEDO

| event_name | version | consumer_candidates | aggregate_id | Mark |
|------------|---------|---------------------|--------------|------|
| `tedo.epd.submitted` | 1 | document-service, CT | epd_transaction.id | NEW |
| `tedo.epd.operator_status_changed` | 1 | document-service, CT | epd_transaction.id | NEW |
| `tedo.epd.delivery_evidence_received` | 1 | document-service | epd_transaction.id | NEW |
| `tedo.epd.inbound_document_received` | 1 | document-service | epd_transaction.id | NEW |
| `tedo.epd.rejected` | 1 | document-service, CT | epd_transaction.id | NEW |

Include: `document_id`, `shipment_id`, `tenant_id`, `correlation_id`.

---

## NEW — Proposed `mm.transport_leg.*`

**Producer:** shipment-service | **Schema ownership:** MM/LOG

| event_name | version | consumer_candidates | aggregate_id | Mark |
|------------|---------|---------------------|--------------|------|
| `mm.transport_journey.created` | 1 | CT, document-service | transport_journey.id | NEW |
| `mm.transport_leg.added` | 1 | CT, TEDO, document-service | transport_leg.id | NEW |
| `mm.transport_leg.status_changed` | 1 | CT | transport_leg.id | NEW |
| `mm.cargo_handover.recorded` | 1 | document-service (EETD), CT | cargo_handover.id | NEW |

All include `shipment_id` in payload. **DO_NOT_CREATE** duplicate shipment.status events for leg-level changes unless shipment FSM intentionally transitions.

---

## NEW — Proposed `ff.receivable.*` / `ff.factoring.*`

**Producer:** payment-service or future FF module | **Schema ownership:** FF

| event_name | version | consumer_candidates | aggregate_id | Mark |
|------------|---------|---------------------|--------------|------|
| `ff.receivable.created` | 1 | freight-cost-service, CT | receivable.id | NEW |
| `ff.receivable.amount_adjusted` | 1 | CT | receivable.id | NEW |
| `ff.factoring.application_submitted` | 1 | CT | factoring_application.id | NEW |
| `ff.factoring.offer_accepted` | 1 | CT, EDO | factor_offer.id | NEW |
| `ff.factoring.assignment.noticed` | 1 | EDO, billing-register-service | assignment.id | NEW |
| `ff.factoring.financing.disbursed` | 1 | payment-service, freight-cost-service | financing.id | NEW |

---

## DEPRECATED (future, not active)

| event_name | Replacement | Mark |
|------------|-------------|------|
| Gateway-only `document.*` timeline strings | `edo.document.*` Kafka | DEPRECATED (when EDO bus live) |

---

## DO_NOT_CREATE summary

| Rejected event | Reason |
|----------------|--------|
| `MultimodalShipment.created` | ONE_SHIPMENT_ID — use `mm.transport_journey.created` |
| `ShipmentCreated` PascalCase | Use `shipment.created` |
| `ReceivablePaid` mirroring obligation | Separate aggregates |
| Duplicate `shipment.documents_completed` per doc | Use `edo.document.*` for document granularity |

## References

- ADR-EDO-006
- `services/shipment-service/internal/domain/outbox.go`
- `services/shipment-service/internal/domain/driver_events.go`
- Discovery CURRENT_EVENTS audit
