# Driver Mobile Pilot v0.9.0

First driver-facing mobile client for delay and problem reporting through the existing Driver HTTP API.

## Architecture

```text
apps/driver-mobile (Vue 3 + Ionic + Capacitor)
        ↓ HTTPS + JWT
api-gateway /api/v1/driver/me/*
        ↓
shipment-service
        ↓
transport.shipment_event_outbox
        ↓
Kafka driver.events.v1
        ↓
control-tower-read-model-service (driverconsumer)
        ↓
Control Tower API + web-admin
```

The mobile app does **not** call Control Tower or Kafka directly.

## Stack

| Layer | Choice |
|---|---|
| UI | Vue 3, Ionic Vue 8 |
| Native shell | Capacitor 6 (Android sync supported) |
| State | Pinia |
| i18n | vue-i18n (RU primary, EN secondary) |
| Tests | Vitest |

## Configuration

Create `apps/driver-mobile/.env.local` (never commit secrets):

```env
VITE_API_BASE_URL=http://localhost:8080
VITE_PILOT_TENANT_ID=<pilot-tenant-uuid>
VITE_API_TIMEOUT_MS=30000
```

| Variable | Purpose |
|---|---|
| `VITE_API_BASE_URL` | API Gateway base URL |
| `VITE_PILOT_TENANT_ID` | Pilot tenant UUID (ops-configured; drivers do not enter tenant manually) |
| `VITE_API_TIMEOUT_MS` | HTTP timeout |

## Local startup

1. Start platform backend (api-gateway + shipment-service + identity at minimum).
2. Seed a driver user with driver role and assigned shipment.
3. Configure `.env.local` with gateway URL and pilot tenant UUID.
4. From repository root:

```powershell
pnpm install
pnpm dev:driver-mobile
```

Open http://localhost:5174 and sign in with driver credentials.

## Authentication

- Login: `POST /api/v1/auth/login` with `{ tenant_id, email, password }`
- `tenant_id` is injected from `VITE_PILOT_TENANT_ID` at build/deploy time — not a driver-entered field
- Token storage: Capacitor Preferences (native) with sessionStorage fallback (web)
- Driver API calls send `Authorization: Bearer <token>` only — no `X-Tenant-ID` header

## Pilot flows

### Report delay

1. My Shipments → select shipment → **Сообщить о задержке**
2. Choose reason (canonical `reasonCode` enum)
3. Optional comment / new ETA
4. Submit → `POST /api/v1/driver/me/shipments/{id}/delays`
5. Success screen: **Сообщение принято** (driver API acceptance only)

Idempotency: stable `Idempotency-Key` header + `idempotencyKey` body field per user action.

### Report problem

1. Shipment detail → **Сообщить о проблеме**
2. Choose category from OpenAPI enum
3. Optional comment
4. Submit → `POST /api/v1/driver/me/shipments/{id}/exceptions`

## Network resilience

| State | UX |
|---|---|
| Offline before send | Form retained in sessionStorage; retry when online |
| Timeout after send | "Result unknown" — same idempotency key on retry |
| 4xx/5xx | Error message; retry allowed |
| Success (200/201) | Confirmation screen |

## Build & test

```powershell
cd apps/driver-mobile
pnpm typecheck
pnpm test
pnpm build
pnpm cap:sync:android
```

iOS sync requires macOS (`NOT_RUN` on Windows is expected).

## Android development setup

1. Install Android Studio + SDK
2. `pnpm build` then `pnpm cap:sync:android`
3. Open `apps/driver-mobile/android` in Android Studio
4. Run on emulator/device (unsigned debug build)

## Staging configuration

1. Set staging gateway URL in `VITE_API_BASE_URL`
2. Set staging pilot tenant in `VITE_PILOT_TENANT_ID`
3. Enable Control Tower driver consumer on staging: `CONTROL_TOWER_DRIVER_EVENTS_ENABLED=true`
4. Run staging E2E from `docs/control-tower/driver-event-staging-e2e-v0.8.2-runbook.md` (on verification branch)

## Known limitations (v0.9.0)

- No GPS tracking, POD, chat, push notifications
- No offline durable queue (session draft only)
- Pilot tenant UUID configured at deploy time (platform login still requires tenant_id server-side)
- Staging Selectel E2E remains pending until runtime access is granted
- ACK cross-tenant integrity finding tracked separately (see report)

## Security note — ACK tenant integrity (v0.8.2 finding)

Cross-tenant ACK with the same `event_id` creates tenant-scoped acknowledgement rows without mutating tenant A workflow. Classified as **EXPECTED_IDEMPOTENT_TENANT_SCOPING** for read-model ACK API; hardening tracked as `ACK_TENANT_INTEGRITY_FOLLOW_UP=REQUIRED` if product requires deny-instead-of-isolated-row semantics.
