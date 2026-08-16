# Control Tower Telemetry Foundation v0.7.0

Provider-agnostic telemetry foundation for shipment tracking in 7RIGHT Control Tower.

## Scope

v0.7.0 establishes canonical location ingestion, tracking state, freshness/quality semantics, Control Tower exposure, and `tracking_loss_risk` migration. **ETA (v0.7.1)** and **slot intelligence (v0.7.2)** are explicitly out of scope.

## Diagram A — ingestion pipeline

```text
Provider
   ↓
Adapter (generic / future Wialon, Geotab, …)
   ↓
Normalize
   ↓
Canonical Location Event (tracking.location_event)
   ↓
Shipment Tracking State (tracking.shipment_tracking_state)
   ↓
Control Tower / Risk
```

## Diagram B — tracking lifecycle

```text
not_configured
      ↓ binding created
awaiting_data
      ↓ first telemetry
active
  ↓           ↑
stale ────────┘
  ↓           ↑
lost ─────────┘

shipment completed / binding ended
      ↓
ended
```

## Diagram C — tracking_loss_risk migration

```text
Shipment.updated_at proxy   [legacy — no longer used for GPS loss after v0.7.0]
               ↓
       v0.7 transition
               ↓
Canonical Telemetry Freshness (when binding exists)
               ↓
tracking_loss_risk
```

## Service boundary

Dedicated **`tracking-service`** owns high-frequency telemetry ingest, persistence, and read APIs. The API gateway exposes public read routes and batch lookup for Control Tower. Shipment-service remains responsible for shipment domain mutations only.

## Canonical model

`LocationEvent` mandatory minimum:

- `tenantId` (from binding, never from untrusted provider payload)
- resolvable shipment via `ShipmentTrackingBinding`
- `latitude`, `longitude` (validated server-side)
- `recordedAt`, `receivedAt` (UTC)
- provider identity (`provider_code`, `provider_device_id`)

Optional: speed, heading, accuracy, altitude, source type, provider event id.

## Provider adapter boundary

```go
type TelemetryProviderAdapter interface {
    ProviderCode() string
    Normalize(ctx context.Context, payload ProviderPayload) ([]NormalizedLocationInput, error)
}
```

v0.7.0 ships **`generic`** adapter only (contract/testing). `driver_mobile` adapter is a hook for future mobile ingestion — no background collection in this release.

## Binding model

`tracking.shipment_tracking_binding` maps `provider_code + provider_device_id → tenant + shipment (+ optional vehicle/driver)`.

States: `active`, `inactive`, `revoked`. History is non-destructive (`active_from` / `active_to`).

Tenant resolution path for ingest:

```text
provider/device → stored active binding → tenant/shipment
```

Provider payloads must **not** override tenant or shipment via untrusted fields.

## Ingestion security

- `POST /internal/v1/tracking/providers/{provider}/locations`
- Auth: `X-Provider-Secret` matched against `TRACKING_PROVIDER_SECRETS`
- Secrets never logged or persisted in raw payload (raw payload not stored in v0.7.0)

## Idempotency

- Primary key: `(tenant_id, provider_code, provider_event_id)` when event id present
- Fallback dedup: SHA-256 of provider + device + recordedAt + coordinates (`dedup_key`)

## Out-of-order events

Late events are stored in history. Current state updates only when `recordedAt` is newer (tie-break: `receivedAt`).

## Freshness policy (centralized)

Environment-configurable defaults:

| Status | Threshold |
|--------|-----------|
| fresh  | ≤ 10 minutes |
| stale  | > 10 and ≤ 30 minutes |
| lost   | > 30 minutes |

Computed at read/evaluation time using server UTC (`telemetryAgeSeconds = now - lastRecordedAt`).

## Quality (deterministic v1)

Inputs: freshness, accuracy, receipt delay, impossible movement (Haversine implied speed > 300 km/h).

Values: `unknown`, `good`, `degraded`, `poor`. Events are never deleted; poor quality is annotated.

## APIs

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/v1/shipments/{id}/tracking` | Current tracking summary |
| GET | `/api/v1/shipments/{id}/tracking/locations` | Bounded history (max 500) |
| POST | `/internal/v1/tracking/providers/{provider}/locations` | Provider ingest |
| POST | `/internal/v1/tracking/states/lookup` | Batch state lookup |

## Control Tower integration

Gateway batch-enriches shipments with:

- `trackingStatus`, `trackingFreshness`, `trackingQuality`
- `lastPositionRecordedAt`, `telemetryAgeSeconds`
- optional last coordinates when genuine telemetry exists

Full history is never embedded in summary/list responses.

## Risk integration

`tracking_loss_risk` uses canonical telemetry when `HasBinding=true`:

| Tracking status | Risk behaviour |
|-----------------|----------------|
| `not_configured` | No GPS/telemetry loss signal |
| `awaiting_data` | Optional low-weight `telemetry_awaiting_data` |
| `stale` | `telemetry_stale` |
| `lost` | `telemetry_lost` |
| degraded/poor quality | `telemetry_quality_degraded` |

`shipment.updated_at` is **not** treated as GPS heartbeat after v0.7.0.

## Tenant security

All tables and queries are tenant-scoped. Cross-tenant device lookup is rejected.

## Data volume & retention

- History paginated; Control Tower uses batch lookup (no N+1 per shipment)
- No retention/destructive archive job in v0.7.0 (documented future requirement)

## Failure behaviour

Tracking dependency failures degrade gracefully: shipment APIs remain available; UI shows “Tracking temporarily unavailable”.

## Future extensions

- v0.7.1: ETA engine (interfaces only prepared)
- v0.7.2: slot-window intelligence
- Real provider adapters (Wialon, Omnicomm, Geotab, …)
- Route deviation risk (requires corridor geometry — not two-point inference)
- Driver mobile background collection

## Database

Migration: `000026_add_shipment_telemetry_foundation_v0.7.0` (create only — not applied in this pilot).

Schema: `tracking.*` — no PostGIS required (NUMERIC coordinates).
