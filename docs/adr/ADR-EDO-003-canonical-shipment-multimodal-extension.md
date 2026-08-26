# ADR-EDO-003: Canonical Shipment and Multimodal Extension

## Status

Accepted — architecture freeze (EDO-0.2)

## Context

Discovery v0.1 proposed a separate `MultimodalShipment` aggregate. EDO-0.2 explicitly **rejects** a second canonical shipment entity. The platform already keys execution, Control Tower projection, billing register items, and driver workflows on flat `transport.shipments.id`.

Road-only shipments must remain backward compatible. Location types (`TERMINAL`, `PORT`, `RAIL_STATION`, etc.) exist in transport-order-service but `transport_mode` is enforced as `ROAD` today.

## Decision

### Canonical invariant

```text
ONE_SHIPMENT_ID_ACROSS_BINTRANS=YES
```

**`shipment-service` is the sole canonical owner of `Shipment`.** No `MultimodalShipment` entity, table, or public API identifier.

### Extension model (child entities, same aggregate root)

```
Shipment (aggregate root, shipment-service)
 └── TransportJourney (0..1 per shipment; implicit single-leg for road-only)
      └── TransportLeg[] (ordered sequence)
           ├── TransportMode (enum reference)
           ├── origin_location_id / destination_location_id
           ├── carrier_company_id (optional per leg)
           ├── planned/actual schedule
           └── CargoHandover[] (at leg boundaries)
```

| Entity | Owner | Aggregate root | Identifier | Notes |
|--------|-------|----------------|------------|-------|
| Shipment | shipment-service | Shipment | `shipment.id` | Existing FSM unchanged for road-only |
| TransportJourney | shipment-service | Shipment | `transport_journey.id`, FK `shipment_id` UNIQUE | Auto-created as single-leg for legacy rows |
| TransportLeg | shipment-service | Shipment | `transport_leg.id`, FK `journey_id` | `sequence_number`; mode per leg |
| CargoHandover | shipment-service | Shipment | `cargo_handover.id` | Links `from_leg_id`, `to_leg_id`, custody parties |
| Terminal | transport-order-service | Location | `location.id` where `location_type IN (TERMINAL, PORT, RAIL_STATION, AIRPORT, INLAND)` | Reference data, not shipment-owned |
| TransportMode | LOG platform convention | Enum | `ROAD`, `RAIL`, `SEA`, `INLAND_WATER`, `AIR`, `MULTIMODAL` | Shared enum; validation evolves via LOG contract |

### Backward compatibility

1. **Road-only shipments** — one implicit `TransportJourney` with one `TransportLeg` (`mode=ROAD`) mirroring existing shipment origin/destination/carrier fields. No FSM change required.
2. **Existing APIs** — `/shipments`, Kafka `shipment.status.v1`, billing `shipment_id` on register items remain unchanged.
3. **Multimodal activation** — additive migrations under `transport` schema; new fields nullable; CT consumes optional leg/handover events without breaking flat projection.
4. **EETD / EPD** — reference `shipment.id` + optional `transport_leg_id`; never a parallel shipment identifier.

### Ownership boundary

- **MM workstream** designs and implements leg/handover schema and APIs **within shipment-service** (or via LOG cross-workstream request — never a forked shipment table elsewhere).
- **TEDO / EDO** reference `shipment_id` and `transport_leg_id` only.

## Consequences

### Positive

- Preserves CT, billing, driver mobile integrations
- Resolves discovery F-007 without breaking flat shipment model
- Supports EETD multi-leg exchanges under one shipment identity

### Negative

- shipment-service aggregate grows — requires careful transaction boundaries
- Terminal master data remains in transport-order-service — cross-service FK by UUID only

## References

- Discovery finding F-007
- `services/shipment-service/internal/domain/outbox.go` (shipment events)
- `transport-order-service` location types in migrations
