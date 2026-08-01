# Shipment Status History v0.1

## Purpose

Shipment Status History persists every successful shipment status transition in `shipment-service` as a canonical audit row. Each row is written in the **same PostgreSQL transaction** as the corresponding update to `transport.shipments.status`.

The API Gateway Shipment Event History BFF reads this history through an internal endpoint and exposes timeline events with:

```json
{
  "derived": false,
  "source": "SHIPMENT_STATUS_HISTORY",
  "sourceEventId": "<history-row-id>"
}
```

## Table

Schema: `transport.shipment_status_history` (migration `000012_create_shipment_status_history_v0.1`).

| Column | Description |
|--------|-------------|
| `id` | Stable UUID primary key |
| `tenant_id` | Verified tenant context |
| `shipment_id` | FK to `transport.shipments(id)` ON DELETE CASCADE |
| `shipment_version` | Shipment `version` after successful transition |
| `from_status` | Previous status; `NULL` for initial creation transition |
| `to_status` | New status after transition |
| `reason_code` | Optional structured reason (e.g. cancel reason code) |
| `source` | History source (`SHIPMENT_SERVICE`) |
| `actor_type` | `USER` or `SYSTEM` |
| `actor_id` | Verified user UUID when `actor_type = USER` |
| `correlation_id` | Request / correlation ID when available |
| `occurred_at` | Server-side transition time |
| `recorded_at` | Row insert time (default `NOW()`) |

### Indexes and constraints

- `UNIQUE (shipment_id, shipment_version)` — idempotency guard against duplicate canonical events on retry
- `idx_shipment_status_history_tenant_shipment_time (tenant_id, shipment_id, occurred_at DESC)`

## Transactional invariant

**Изменение shipment status и запись shipment_status_history выполняются атомарно в одной PostgreSQL-транзакции. Успешный переход без history row и history row без соответствующего перехода недопустимы.**

Repository methods (`CreateShipment`, `AssignDriver`, `AssignVehicle`, `UpdateStatus`, `Accept`, `Cancel`) begin a transaction, update the shipment with optimistic locking, insert the history row when the status actually changes, then commit. Any failure rolls back both operations.

## Optimistic locking

Updates use `WHERE id = … AND tenant_id = … AND version = expectedVersion`. On success, `version` increments and the history row stores the new version. A retry with a stale version returns conflict semantics and does **not** insert a duplicate history row because the update affects zero rows.

## Idempotency

The unique constraint on `(shipment_id, shipment_version)` prevents two canonical events for the same shipment version. Optimistic lock conflicts stop the insert path before commit.

## Tenant isolation

All reads and writes include `tenant_id` from verified gateway context (`X-Tenant-ID`). History list queries use:

```sql
WHERE tenant_id = $1 AND shipment_id = $2
```

Foreign or unknown shipments return `404`; missing tenant returns `401`.

## Shipment mutation trust boundary

**Пользовательские операции Create, Accept, UpdateStatus и Cancel не принимают tenant_id как источник области данных. Tenant определяется только из verified gateway context и используется одинаково для shipment mutation и shipment_status_history.**

- Tenant is not selected via request body or query parameters on user-facing mutations (`POST /v1/shipments/from-transport-order`, `POST /v1/shipments/from-bid`, `POST /v1/shipments/{id}/accept`, `PATCH /v1/shipments/{id}/status`, `POST /v1/shipments/{id}/cancel`).
- Actor is not selected via body or query; missing verified user is rejected before any transaction (`401` / `400`).
- Missing verified tenant is rejected before JSON decode, service call, or repository transaction.
- Foreign and unknown shipments or source entities (transport order, bid) are indistinguishable (`404`).
- Status mutation and history row always share the same verified tenant within one transaction.
- Direct shipment-service access from untrusted networks is prohibited by production network policy; clients must use API Gateway with JWT authentication.

Strict JSON decoding (`DisallowUnknownFields`) rejects spoofed `tenant_id` / `tenantId` in mutation bodies with `400 Bad Request`.

Gateway route-level authorization for Create, Accept, UpdateStatus, and Cancel is documented in [SHIPMENT_MUTATION_AUTHORIZATION.md](./SHIPMENT_MUTATION_AUTHORIZATION.md). Shipment-service does not perform RBAC; it trusts verified gateway identity headers after authorization succeeds.

## Actor provenance

Отсутствие verified user context в пользовательском HTTP-запросе **не интерпретируется как SYSTEM**. Такой запрос отклоняется до изменения shipment (`401 Unauthorized` when `X-User-ID` is missing; `400 Bad Request` when malformed or zero UUID). SYSTEM actor используется только для явно инициированных server-side переходов через `NewSystemTransitionContext`.

User-facing HTTP mutations require verified gateway-authenticated context:

- `X-User-ID` (set by API Gateway from JWT `sub` after `StripUntrustedIdentityHeaders`) → `actor_type = USER`, `actor_id = verified user UUID`
- Incoming spoofed `X-User-ID` is stripped at the gateway; shipment-service reads only the trusted header value

The service does **not** read actor or tenant from request body or query parameters. Correlation ID (`X-Request-ID`) is not an actor ID. JWT, email, phone, and full name are never stored in history rows.

Server-side background or internal callers may use `NewSystemTransitionContext` with `actor_type = SYSTEM` and `actor_id = NULL`. No real background status mutation callers exist in v0.1; the constructor is reserved for future trusted processes.

## Correlation ID

`correlation_id` is taken from the request ID (`X-Request-ID` / middleware context) when present.

## Internal endpoint

```http
GET /internal/v1/shipments/{shipmentId}/status-history
```

Tenant: required `X-Tenant-ID` header only.

Query: `page` (default 1), `limit` (default 50, max 200), `order` (`asc` / `desc`).

This route is **not** registered as a public browser/API Gateway catch-all endpoint. The gateway BFF calls shipment-service directly on the internal network path.

## History completeness

History is **complete** when a row exists with `from_status IS NULL` (initial creation transition).

Response field:

```json
{ "complete": true }
```

For shipments created before status history existed:

```json
{
  "complete": false,
  "warnings": ["SHIPMENT_STATUS_HISTORY_PARTIAL"]
}
```

## No backfill

**История является полной только для перевозок, у которых сохранена начальная запись создания. Для ранее созданных перевозок автоматический backfill не выполняется.**

Migration `000012` does not insert synthetic rows from `updated_at` or current status. Legacy shipments may have partial history starting from the first post-deployment transition.

## Legacy shipment behavior

When `complete = false`:

- Canonical rows from persisted history are shown
- Derived fallback events may supplement missing early transitions
- Timeline includes `SHIPMENT_STATUS_HISTORY_PARTIAL`
- Derived duplicates for transitions already covered by canonical rows are removed

When the internal endpoint is unavailable, the gateway returns a partial timeline with `SHIPMENT_STATUS_HISTORY_UNAVAILABLE` and derived fallback where applicable.

## Canonical vs derived events

| Source | `derived` | When |
|--------|-----------|------|
| `SHIPMENT_STATUS_HISTORY` | `false` | Persisted status history row |
| `SHIPMENT_STATE` | `true` | Fallback from current shipment entity state |

Canonical events win over derived duplicates in deduplication.

## Public timeline mapping

| History transition | Timeline event type |
|--------------------|---------------------|
| `from_status = null` | `SHIPMENT_CREATED` |
| `to_status = CANCELLED` | `SHIPMENT_CANCELLED` |
| `to_status = READY_FOR_BILLING` | `READY_FOR_BILLING` |
| `to_status = DOCUMENTS_COMPLETED` | `DOCUMENTS_COMPLETED` |
| `to_status = FINANCIALLY_CLOSED` | `FINANCIALLY_CLOSED` |
| other status change | `SHIPMENT_STATUS_CHANGED` |

Metadata allowlist: `fromStatus`, `toStatus`, `shipmentVersion`, `reasonCode`.

## v0.1 limitations

- No separate event-store microservice
- No Kafka / outbox / event streaming
- No retroactive synthesis of unknown transition times
- Assign driver/vehicle without status change does not write history
- State machine rules are unchanged; history layer does not validate transitions independently

## Future work

- Outbox pattern and event streaming for cross-service consumers
- Optional dedicated audit/event store when legal retention requirements expand
