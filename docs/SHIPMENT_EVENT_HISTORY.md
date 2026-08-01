# Shipment Event History v0.1

## Purpose

Shipment Event History provides a unified read-only timeline for a single shipment by aggregating data from multiple domain services through the API Gateway BFF endpoint:

```http
GET /api/v1/shipments/{shipmentId}/events
```

**Shipment Event History v0.1 is not a complete legally significant audit journal and may contain derived events.**

## Architecture

Aggregation is implemented in `services/api-gateway/internal/shipmentevents/`:

- **Handler** — HTTP validation, auth context, RBAC
- **Service** — orchestrates downstream calls and timeline assembly
- **Builders** — maps shipment/document state and SLA calculator output to derived timeline events
- **Timeline utilities** — deduplication, sorting, filtering, pagination, metadata allowlist

The gateway does not access downstream databases directly and does not introduce a parallel event store.

## Event provenance

An event is **canonical** (`derived: false`) when downstream returns a dedicated persisted record with stable ID, explicit type, confirmed `occurredAt`, and shipment linkage.

**Shipment status transitions** use persisted history from `shipment-service`:

```text
SHIPMENT_STATUS_HISTORY
```

See [SHIPMENT_STATUS_HISTORY.md](./SHIPMENT_STATUS_HISTORY.md) for transactional storage, completeness rules, and legacy behavior.

| Event type | Source | Timestamp field | Event record? | derived |
|---|---|---|---|---|
| `SHIPMENT_CREATED` | status history (or shipment entity fallback) | history `occurred_at` / `created_at` | Yes when history complete | false / true |
| `SHIPMENT_STATUS_CHANGED` | status history (or derived fallback) | history `occurred_at` | Yes when available | false / true |
| `SHIPMENT_CANCELLED` | status history (or derived fallback) | history `occurred_at` | Yes when available | false / true |
| `READY_FOR_BILLING`, `DOCUMENTS_COMPLETED`, `FINANCIALLY_CLOSED` | status history when present | history `occurred_at` | Yes when available | false |
| `PICKUP_*` / `DELIVERY_*` | shipment entity | planned/actual fields | No | true |
| `DOCUMENT_CREATED` | document entity | `created_at` | No | true |
| `DOCUMENT_SIGNED` | document entity | `signed_at` only | No | true |
| `DOCUMENT_REJECTED` | document entity | `rejected_at` only | No | true |
| SLA events | SLA calculator | deadline / stale field | No | true |
| Billing milestones | — | — | Not emitted in v0.1 | — |

Sources for BFF-generated (derived) events:

```text
SHIPMENT_STATE
DOCUMENT_STATE
SLA_CALCULATOR
```

Do **not** use names implying a separate event store (`DOCUMENT_SERVICE`, `BILLING_REGISTER`).

`sourceEventId` is set for canonical status history events. Derived events omit `sourceEventId`.

## Shipment mutation trust boundary

Status history rows are written only from verified tenant context on successful user-facing mutations (Create, Accept, UpdateStatus, Cancel). Clients cannot select tenant or actor via mutation body or query; API Gateway sets trusted identity headers from JWT after stripping spoofed values. Gateway route-level RBAC runs before proxying mutations — see [SHIPMENT_MUTATION_AUTHORIZATION.md](./SHIPMENT_MUTATION_AUTHORIZATION.md). See [SHIPMENT_STATUS_HISTORY.md](./SHIPMENT_STATUS_HISTORY.md) for invariants and handler ordering.

## Timestamp guarantees

`occurredAt` is never approximate. The BFF does not use:

- current time / `generatedAt`
- generic `updated_at` for business milestones
- HTTP response receive time

Allowed timestamp fields (when present in downstream payload):

```text
created_at, signed_at, rejected_at
planned_pickup_at, planned_delivery_at, actual_pickup_at, actual_delivery_at
SLA deadline fields, last_updated_at (only for SLA stale signal)
```

**General `updatedAt` is not used** as the time of a specific business transition (sign, reject, payment, cancellation, billing milestone) without a dedicated field.

Events removed when no dedicated timestamp exists:

- `SHIPMENT_CANCELLED` (no `cancelled_at` in shipment API)
- `READY_FOR_BILLING`, `FINANCIALLY_CLOSED`, `DOCUMENTS_COMPLETED`, `DOCUMENTS_MISSING`
- All billing milestone types in v0.1
- `DOCUMENT_SIGNED` / `DOCUMENT_REJECTED` when only `status` + `updated_at` are available

## Billing lookup limitations

`billing-register-service` has no reverse lookup by `shipment_id` and no shipment-scoped items endpoint in v0.1.

The gateway **does not scan** billing registers or perform N+1 item fetches.

Response semantics:

```json
{
  "dataFreshness": {
    "billingLoaded": false,
    "partial": true,
    "warnings": ["BILLING_EVENTS_UNAVAILABLE"]
  }
}
```

This is a **capability limitation**, not a runtime outage. Billing timeline events will be added when a bounded server-side lookup exists.

## Capability limitations vs runtime failures

| Situation | `partial` | Warning |
|---|---|---|
| Derived shipment history (legacy / history unavailable) | true | `SHIPMENT_HISTORY_DERIVED_FROM_CURRENT_STATE` |
| Persisted status history incomplete (pre-migration shipment) | true | `SHIPMENT_STATUS_HISTORY_PARTIAL` |
| Status history internal endpoint unavailable | true | `SHIPMENT_STATUS_HISTORY_UNAVAILABLE` |
| Billing not supported in v0.1 | true | `BILLING_EVENTS_UNAVAILABLE` |
| Document service configured but fails | true | `DOCUMENT_EVENTS_UNAVAILABLE` |
| Document list truncated at fetch limit | true | `TIMELINE_CALCULATED_FROM_LIMITED_DATASET` |
| GPS / geolocation not integrated | — | *(no warning — not a runtime failure)* |
| Technical events source not configured | — | *(no warning — `technicalEventsLoaded: false`)* |

`shipmentEventsLoaded: true` when persisted status history was loaded successfully (including empty or partial legacy history). `shipmentEventsLoaded: false` only when the internal history source is unavailable; derived shipment events may still appear in `timeline.items`.

## Data sources

| Source | Required | Notes |
|---|---|---|
| `shipment-service` `GET /v1/shipments/{id}` | Yes | Verified tenant via trusted internal `X-Tenant-ID` header only (no `tenant_id` query). Tenant-scoped repository lookup; defense-in-depth tenant check on response remains. |
| `shipment-service` `GET /internal/v1/shipments/{id}/status-history` | Yes (BFF) | Internal network only. Canonical status transition source. Not exposed as public browser route. |
| `document-service` filtered list | No | Runtime failure → `DOCUMENT_EVENTS_UNAVAILABLE` |
| Billing reverse lookup | No | Not called in v0.1 |
| Shared SLA calculator | Derived | Reused from Control Tower |

## Tenant isolation

- Tenant from verified JWT `AuthContext` only
- Spoofed headers / `tenant_id` query ignored for auth context at gateway
- Outbound shipment detail request sends verified tenant as `X-Tenant-ID` header only (no `tenant_id` in URL)
- **`GET /v1/shipments/{id}` does not accept `tenant_id` from query.** Tenant arrives only through trusted gateway context.
- **Shipment isolation invariant:** shipment-service loads shipment in one repository query by `shipment_id` and verified `tenant_id`. Post-fetch tenant check is defense in depth only.
- Foreign tenant shipment is indistinguishable from missing shipment (`404`)
- Second lookup by shipment ID only is forbidden in shipment-service
- Missing verified tenant → gateway `401`; shipment-service `401` without repository call
- Foreign tenant → **404** from shipment-service (gateway defense-in-depth comparison retained)
- In production, shipment-service must not be reachable from the external network directly; external ingress goes to API Gateway only. Local port `8085` in docker-compose is development-only.

## RBAC

Backend allowlist (independent of frontend route tables):

```text
PLATFORM_ADMIN, SHIPPER_ADMIN, SHIPPER_LOGIST,
CARRIER_ADMIN, CARRIER_DISPATCHER, FORWARDER_MANAGER,
CONSIGNEE_OPERATOR, CONSIGNEE_VIEWER
```

Denied: `FINANCE_MANAGER`, `PROCUREMENT_MANAGER`, unknown roles → **403**

## Metadata allowlist

Only safe keys are emitted:

```text
documentId, documentType, documentStatus,
billingRegisterId, billingRegisterNumber, billingStatus,
plannedAt, actualAt, delayMinutes, slaReason, slaStatus
```

## Partial response

Warnings are machine codes only — no hostnames, URLs, stack traces, or downstream bodies.

## v0.1 limitations

- No canonical event store in any domain service
- No billing timeline without reverse lookup API
- No GPS/geolocation events
- Not suitable as legal audit evidence

## Production event store (future)

Requires dedicated event/audit tables or outbox in shipment, document, and billing services, plus geolocation ingestion and billing reverse lookup.

## Control Tower relationship

- Shares `internal/platform/sla` with Control Tower
- Links to `/shipments/{id}/events`
