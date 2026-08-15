# Driver Mobile Platform v0.1

Production-grade driver operations boundary for authenticated DRIVER role clients.

## Authentication

- JWT Bearer via API Gateway (`Authorization: Bearer …`)
- Gateway strips untrusted identity headers and injects `X-Tenant-ID` / `X-User-ID`
- Driver identity resolves through `transport.drivers.user_id` binding (unique per tenant)

## Authorization

Central guard: `CanDriverAccessShipment(tenantID, driverID, shipment)` using authoritative `shipments.driver_id`.

Drivers cannot supply trusted `tenant_id`, `driver_id`, or cross-shipment identifiers.

## API Surface (Gateway)

| Method | Route | Purpose |
|--------|-------|---------|
| GET | `/api/v1/driver/me` | Driver profile |
| GET | `/api/v1/driver/me/shipments` | Assigned shipments |
| GET | `/api/v1/driver/me/shipments/{id}` | Shipment detail |
| POST | `/api/v1/driver/me/shipments/{id}/events` | Operational FSM events |
| POST | `/api/v1/driver/me/shipments/{id}/exceptions` | Exception reporting |
| POST | `/api/v1/driver/me/shipments/{id}/locations` | GPS ingest |
| POST | `/api/v1/driver/me/shipments/{id}/pod/uploads` | POD upload intent |
| PUT | `/api/v1/driver/me/shipments/{id}/pod/uploads/{uploadId}/content` | POD bytes (token) |
| POST | `/api/v1/driver/me/shipments/{id}/pod/uploads/{uploadId}/complete` | POD finalize |

Shipment-service mirrors routes under `/v1/driver/me/*` for trusted internal calls.

## Status Events

Allow-listed driver events map to existing shipment FSM transitions:

- `ARRIVED_AT_PICKUP` → `IN_PICKUP`
- `LOADING_STARTED` → informational (no status change)
- `PICKUP_COMPLETED` → `LOADED`
- `DEPARTED_PICKUP` → `IN_TRANSIT`
- `ARRIVED_AT_DELIVERY` → `ARRIVED_AT_CONSIGNEE`
- `UNLOADING_STARTED` → `UNLOADING`
- `DELIVERY_COMPLETED` → `DELIVERED`

Idempotency: `Idempotency-Key` header or `idempotencyKey` field scoped to tenant+driver+operation.

## Location Ingest

Gateway validates driver/shipment assignment, resolves driver ID from `/driver/me`, ensures `driver_mobile` tracking binding, and feeds existing tracking ingest pipeline. No per-GPS automation triggers.

## Exceptions → Control Tower

1. Shipment-service persists `transport.driver_reported_exception`
2. Transactional outbox event `driver.exception_reported`
3. Gateway integration adapter ensures Control Tower exception workflow
4. Automation ingress receives `exception_created` trigger (tenant-scoped)

## Known Limitations (v0.1)

- No mobile UI, push, driver inbox, or CT→driver reverse channel
- POD driver workflow deferred (document upload architecture requires separate upload token flow)
- Batch location offline sync not implemented (single-point ingest with idempotency only)

## v0.1.1 Verification Evidence

- Migration `000030` verified on embedded PostgreSQL (schema, uniqueness, concurrent idempotency)
- Migration `000031` relaxes outbox FK so `driver.exception_reported` can reference `driver_reported_exception.id`
- Migration `000032` allows `critical_event_workflow.source=driver`
- Full automation E2E: persisted rule → playbook → recommendation → execution → steps (`internal/integration/automation`)
- Shipment driver DB tests: concurrent exception idempotency, wrong-driver denial, status idempotency
- POD v0.1.1: document-service upload intent + local object storage + gateway driver wrapper (migrations `000033`)
- Exception event IDs normalized to 32-char hex before Control Tower ingress

## Security

- DRIVER RBAC on gateway driver routes only (no dispatcher/admin leak)
- Cross-tenant and same-tenant wrong-driver return safe not-found
- Denied mutations emit no outbox/automation events
- Driver comments sanitized; severity/category server-controlled for exceptions
