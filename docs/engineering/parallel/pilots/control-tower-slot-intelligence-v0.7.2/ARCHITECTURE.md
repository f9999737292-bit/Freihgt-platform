# Control Tower Slot Intelligence v0.7.2

Provider-neutral canonical slot window intelligence for pickup/delivery operations in BINTRANS Control Tower.

## Scope

v0.7.2 replaces status-only slot semantics (`PICKUP_SLOT_BOOKED` / `DELIVERY_SLOT_BOOKED`) with canonical slot windows, revision history, ETA-vs-slot arrival projection, Control Tower batch enrichment, and `slot_miss_risk` migration.

**Out of scope:** automatic rescheduling, commercial slot provider integration, slot booking UI, ML prediction, migration apply, tests, deployment.

## Discovery summary

| Capability | Available |
|---|---|
| Real slot window in repo | NO — lifecycle status flags only |
| Slot booking service | NO |
| Warehouse timezone (location) | YES (`locations.timezone` on transport orders) |
| Facility timezone | via location model |
| Pickup/delivery slot status | YES (shipment lifecycle) |
| Slot provider integration | NO |
| Check-in timestamp | NO |
| Actual arrival | YES (`actual_pickup_at`, `actual_delivery_at`) |

**Critical rule:** `DELIVERY_SLOT_BOOKED` without start/end → `windowStatus = unavailable`. No fabricated 30/60-minute windows.

## Diagram A — Slot pipeline

```text
Slot Provider / Internal Booking
             ↓
         Normalize
             ↓
     Canonical Slot Revision
             ↓
      Current Slot State
             ↓
             ETA
             ↓
   Arrival Projection
             ↓
  Control Tower / Risk / Case
```

## Diagram B — Slot lifecycle

```text
PROPOSED
   ↓
BOOKED
   ↓
CONFIRMED
   ↓
   ├────────→ CANCELLED
   │
   ├────────→ COMPLETED
   │
   └────────→ MISSED
```

## Diagram C — ETA vs slot window

```text
Slot: 14:00 ───────────── 14:30

ETA 13:40  → EARLY
ETA 14:15  → ON_TIME
ETA 14:27  → AT_RISK      (≤15 min margin)
ETA 14:48  → PROJECTED_MISS (+18m)
```

## Diagram D — slot_miss_risk precedence

```text
Real Slot + usable ETA
        ↓
ETA-vs-Slot evaluation
        ↓
slot_miss_risk

No real Slot or no usable ETA
        ↓
legacy lifecycle/status fallback
        ↓
slot_miss_risk
(without precise slot-window claim)
```

## Service boundary

Extends **`tracking-service`** (`tracking` schema). Reuses v0.7.1 ETA batch lookup in gateway enrichment — no ETA recalculation inside slot service.

## Canonical model

### Slot revision (`tracking.shipment_slot_revision`)

History/reschedule audit trail with `window_start`, `window_end` (UTC), optional `timezone`, lifecycle `slot_status`, source metadata, dedup keys.

### Current state (`tracking.shipment_slot_state`)

Fast lookup per `(tenant, shipment, slot_type)` with `window_status` (`unavailable` | `available`).

## Source precedence

1. `warehouse_api`
2. `internal_booking`
3. `shipper_api`
4. `carrier_api`
5. `manual_operator`
6. `system_import`

Tie-break: newer `source_observed_at`; active non-cancelled status preferred.

## Arrival projection (centralized)

Policy: `domain/slot_arrival.go` — `WarningBufferBeforeEnd = 15m`

| Projection | Rule |
|---|---|
| early | ETA before windowStart |
| on_time | windowStart ≤ ETA ≤ windowEnd − buffer |
| at_risk | windowEnd − 15m < ETA ≤ windowEnd |
| projected_miss | ETA > windowEnd (predictive) |
| missed | actual milestone > windowEnd |
| completed | actual milestone within window |
| unknown | no window or no usable/non-expired ETA |

Actual milestone source: `actual_pickup_at` / `actual_delivery_at` (not check-in).

## Ingestion

```
POST /internal/v1/tracking/providers/{provider}/slots
```

Generic adapter contract only. Provider secret auth + device binding tenant/shipment resolution (mirrors telemetry/ETA).

## Public APIs (via gateway)

```
GET /api/v1/shipments/{shipmentId}/slots
GET /api/v1/shipments/{shipmentId}/slots/history   (max 200)
POST /internal/v1/tracking/slots/lookup            (batch + ETA context)
```

## Control Tower integration

After ETA enrichment, batch slot lookup with ETA snapshots per shipment. Additive fields: `deliverySlotWindow*`, `pickupSlotWindow*`, `*ArrivalProjection`, `*ProjectedLateSeconds`, `*MarginSeconds`.

## Risk integration

`slot_miss_risk` precedence:

1. Real slot window + usable ETA → `slot_eta_at_risk`, `slot_projected_miss`, `slot_actual_missed`, `slot_eta_stale`
2. Else → legacy `pickup_slot_not_booked` / `delivery_slot_not_booked` fallback

Stale ETA: reduced weight (0.75×). Expired ETA: no slot-window prediction.

## Database

Migration `000028_add_shipment_slot_intelligence_v0.7.2` — **not applied** in this pilot.

## Known limitations

- No real slot provider connected
- No manual operator public mutation endpoint (FOLLOW_UP)
- No automatic reschedule/booking automation
- Pickup ETA on Control Tower batch uses delivery ETA fields only until pickup ETA enrichment extended
- Event bus slot topics deferred (noise control)

## Future

- Manual slot CRUD with `MANAGE_SLOT` RBAC
- Facility timezone resolution on ingest
- Slot booking automation / warehouse API adapters
- Actual exception materialization to `slot_issue` category
