# Control Tower ETA Intelligence v0.7.1

Provider-neutral ETA intelligence layer for shipment delivery (and optional pickup) predictions in 7RIGHT Control Tower.

## Scope

v0.7.1 adds canonical ETA observations, current ETA state, freshness/quality/deviation semantics, Control Tower batch enrichment, `delivery_delay_risk` migration to real ETA when available, and shipment detail / work-item ETA context.

**Explicitly out of scope:** commercial routing providers, GPS-to-ETA calculation, slot intelligence (v0.7.2), ML prediction, migration apply, tests, deployment.

## Discovery summary

| Capability | Available |
|---|---|
| Real ETA source in repo | NO — generic contract adapter only |
| Routing provider | NO |
| Route geometry | NO |
| Road distance | NO |
| Provider-reported ETA | NO (ingestion contract ready) |
| Planned delivery (`planned_delivery_at`) | YES |
| Delivery window (`delivery_window_start/end`) | NO |

Coordinates alone are **not** used to fabricate road ETA. When no valid observation exists: `etaStatus = unavailable`.

## Diagram A — ETA pipeline

```text
Provider ETA
Carrier ETA
Manual ETA
Calculated ETA [future]
      │
      └──────────────┐
                     ↓
             ETA Normalization
                     ↓
             ETA Observation (tracking.eta_observation)
                     ↓
             Current ETA State (tracking.shipment_eta_state)
                     ↓
      Planned vs Predicted Arrival
                     ↓
           Control Tower / Risk
```

## Diagram B — ETA lifecycle

```text
unavailable
    ↓ first valid ETA
available
   ↓       ↑
stale ─────┘
   ↓       ↑
expired ───┘

actual milestone (delivery)
      ↓
completed
```

## Diagram C — delivery_delay_risk precedence

```text
Fresh real ETA available
        ↓
ETA-driven delivery risk
        ↓
delivery_delay_risk

No usable ETA
        ↓
Existing lifecycle/deadline fallback
        ↓
delivery_delay_risk
(without claiming ETA)
```

## Service boundary

Extends **`tracking-service`** (same `tracking` schema as v0.7.0 telemetry). API gateway proxies public ETA reads and performs batch lookup for Control Tower summary.

## Canonical model — ETAObservation

Persisted in `tracking.eta_observation`:

- `tenant_id`, `shipment_id`
- `target_type` (`pickup`, `delivery` — delivery primary in v0.7.1)
- `estimated_arrival_at`
- `source_type` (`provider_eta`, `carrier_eta`, `driver_eta`, `manual_operator`; `calculated` only when a real calculator exists)
- `source_observed_at`, `received_at` (UTC)
- `quality_status`, `quality_reasons`
- optional `provider_code`, `provider_event_id`, `provider_confidence`

## Current ETA state

`tracking.shipment_eta_state` holds fast lookup per `(tenant, shipment, target_type)` with versioned upsert. Selection considers source precedence, freshness, quality, and latest `source_observed_at` — not merely `received_at`.

### Source precedence (centralized)

1. `provider_eta` (fresh)
2. `carrier_eta`
3. `driver_eta`
4. `manual_operator`
5. `calculated` (future — disabled until calculator exists)

Tie-break: newer `source_observed_at`.

## Freshness policy (centralized)

Defaults (minutes since `source_observed_at`):

| Status | Threshold |
|---|---|
| fresh | ≤ 15 |
| stale | > 15 and ≤ 60 |
| expired | > 60 |

Stale ETA may remain informational; expired/poor ETA does not drive canonical delay prediction.

## Quality model

| Status | Meaning |
|---|---|
| unknown | insufficient evidence |
| good | fresh trusted source |
| degraded | stale source, high delivery lag, or provider degradation |
| poor | not suitable for risk weighting |

## Deviation & arrival projection

Compared against existing shipment `planned_delivery_at` (no fabricated delivery windows).

```
projectedDeviationSeconds = estimatedArrivalAt - plannedArrivalAt
```

| Projection | Rule (single planned timestamp) |
|---|---|
| early | ETA > 15 min before plan |
| on_time | within ±15 min |
| at_risk | 15–30 min after plan |
| late | > 30 min after plan |
| unknown | no usable ETA |

Policy centralized in `domain/eta_deviation.go`.

## Ingestion

```
POST /internal/v1/tracking/providers/{provider}/eta
```

- Reuses v0.7.0 provider secret authentication
- Tenant/shipment resolved via device binding (same as telemetry) or trusted internal shipment id
- Idempotent dedup via `(tenant, provider, provider_event_id, target)` or deterministic hash
- Out-of-order observations accepted; current state never regresses to older `source_observed_at`

## Public APIs (via gateway)

```
GET /api/v1/shipments/{shipmentId}/eta
GET /api/v1/shipments/{shipmentId}/eta/history   (max limit 200)
```

Planned/actual milestone times passed as query params from authenticated shipment context for deviation computation at read time.

## Control Tower integration

Batch lookup:

```
POST /internal/v1/tracking/eta/lookup
```

Gateway enriches `ControlTowerShipment` additively:

- `etaStatus`, `estimatedDeliveryAt`, `etaFreshness`, `etaQuality`
- `projectedDelaySeconds`, `arrivalProjection`

No ETA history embedded in summary projections.

## Risk integration

`delivery_delay_risk` precedence:

1. If fresh/usable canonical ETA → ETA-vs-plan signals (`eta_delivery_at_risk`, `eta_after_planned_delivery`, `eta_stale`, `eta_quality_degraded`)
2. Else → existing lifecycle/deadline proximity fallback **without labeling fallback as ETA**

`pickup_delay_risk` unchanged unless genuine pickup ETA exists. `slot_miss_risk` unchanged (v0.7.2).

## Database

Migration `000027_add_shipment_eta_intelligence_v0.7.1` (create only — **not applied** in this pilot):

- `tracking.eta_observation`
- `tracking.shipment_eta_state`
- `tracking.eta_state_transition` (meaningful transitions only)

## Observability

Prometheus counters/histograms:

- `eta_observations_received_total`
- `eta_observations_rejected_total`
- `eta_observations_deduplicated_total`
- `eta_ingestion_lag_seconds`

## Security

- Tenant-scoped reads and writes
- Provider ingestion auth separate from user JWT
- No cross-tenant ingestion or IDOR
- Safe logging — no raw provider secrets/payloads

## Known limitations

- No commercial routing provider connected
- No calculated ETA from GPS coordinates
- No delivery window model (single planned timestamp only)
- Manual operator ETA endpoint deferred if RBAC expansion required
- Event bus ETA topics deferred to avoid noise without durable consumers

## Future

- v0.7.2 Slot Intelligence (ETA vs slot windows)
- Real routing provider adapter behind `ETAProviderAdapter`
- Provider accuracy analytics from preserved history
