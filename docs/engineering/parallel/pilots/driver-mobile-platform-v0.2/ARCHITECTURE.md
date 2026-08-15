# Driver Mobile Platform v0.2 — Tasks / Inbox / Push

## Overview

v0.2 introduces a persistent **Driver Task** domain in `shipment-service`. Push is transport only; the task record is the source of truth. Control Tower v0.8.1 will call the trusted internal task API later — automation actions are **not** enabled in v0.2.

## Domain

| Table | Purpose |
|-------|---------|
| `transport.driver_task` | Task lifecycle, idempotency, correlation |
| `transport.driver_task_response` | Structured driver responses |
| `transport.driver_device` | Push token registry (hash + ciphertext) |
| `transport.driver_notification_delivery` | Durable push delivery work |

### Task types (allow-list)

- `REQUEST_DELAY_REASON` (primary acceptance flow)
- `REQUEST_STATUS_CONFIRMATION`
- `REQUEST_ARRIVAL_CONFIRMATION`
- `REQUEST_DOCUMENT_ACTION`
- `GENERAL_OPERATIONAL_NOTICE`

### States

`PENDING` → `DELIVERED` / `READ` → `ACKNOWLEDGED` → `COMPLETED`, or terminal `EXPIRED` / `CANCELLED`.

Push delivery does **not** imply READ/COMPLETED.

## Public API (gateway → shipment-service)

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/v1/driver/me/tasks` | Inbox list |
| GET | `/api/v1/driver/me/tasks/{taskId}` | Task detail |
| POST | `/api/v1/driver/me/tasks/{taskId}/read` | Mark read |
| POST | `/api/v1/driver/me/tasks/{taskId}/acknowledge` | Acknowledge |
| POST | `/api/v1/driver/me/tasks/{taskId}/responses` | Structured response |
| POST | `/api/v1/driver/me/devices` | Register device |
| DELETE | `/api/v1/driver/me/devices/{deviceId}` | Revoke device |

## Internal API (trusted)

| Method | Path | Auth |
|--------|------|------|
| POST | `/internal/v1/driver/tasks` | `X-Internal-Service-Token` |
| POST | `/internal/v1/driver/tasks/{taskId}/cancel` | `X-Internal-Service-Token` + `X-Tenant-ID` |

## Push

- `PushProvider` abstraction with `FakeProvider` (tests) and `FCMProvider` (config-only credentials).
- Notification worker: poll → claim → send → retry with bounded attempts (default 3, exponential backoff).
- No device: task + inbox still work; delivery status `no_device`.
- FCM unavailable: inbox unaffected; retries until max attempts.

## Events (outbox)

- `driver.task_created`
- `driver.task_completed`
- `driver.task_expired`
- `driver.task_cancelled`

Payload preserves `sourceEventId` and `correlationId` for Control Tower consumption.

## Control Tower v0.8.1 contract (future)

```
Guarded Action REQUEST_DRIVER_DELAY_REASON
  → POST /internal/v1/driver/tasks
      type=REQUEST_DELAY_REASON
      source=CONTROL_TOWER
      sourceEventId, correlationId, idempotencyKey
  → driver responds
  → driver.task_completed outbox event
  → CT case/timeline adapter (deferred to v0.8.1)
```

## Security

- Tenant + driver derived from JWT/context; client cannot set owner fields.
- Same-tenant wrong-driver → 404 anti-enumeration.
- Task type allow-list rejects arbitrary commands.
- Push tokens stored as hash + ciphertext; never logged in full.

## Known limitations

- No mobile UI, APNS direct adapter, or CT timeline write in v0.2.
- `REQUEST_DELAY_REASON` is the only task type with response schema in v0.2.
- Exactly-once push delivery not guaranteed; task idempotency is server-side.
