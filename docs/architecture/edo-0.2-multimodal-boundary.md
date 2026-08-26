# EDO-0.2 Multimodal Boundary

## Status

Architecture freeze (EDO-0.2)

## Frozen model

See ADR-EDO-003. Summary:

```text
ONE_SHIPMENT_ID_ACROSS_BINTRANS=YES
NO MultimodalShipment entity
```

```
Shipment (shipment-service, aggregate root)
 └── TransportJourney
      └── TransportLeg[]
           └── CargoHandover[]
```

## Entity ownership

| Entity | Owner | Identifier | Backward compatibility |
|--------|-------|------------|------------------------|
| Shipment | shipment-service | `shipment.id` | Unchanged road FSM |
| TransportJourney | shipment-service | `transport_journey.id` | Auto-created 1:1 for existing shipments |
| TransportLeg | shipment-service | `transport_leg.id` | Single ROAD leg default |
| CargoHandover | shipment-service | `cargo_handover.id` | Empty for unimodal |
| Terminal | transport-order-service | `location.id` | Existing location types |
| TransportMode | LOG platform enum | `ROAD`, `RAIL`, `SEA`, `INLAND_WATER`, `AIR` | ROAD-only validation until MM phase |

## EDO / TEDO references

- Transport documents (EPD, ETRN, EETD members) carry `shipment_id` (required) and `transport_leg_id` (optional).
- EETD packages scope to journey when multimodal; still one `shipment_id`.
- Billing register items continue to reference flat `shipment_id` only.

## Event surface (proposed)

See `mm.transport_leg.*` in event contracts — additive; does not replace `shipment.status.changed`.

## References

- ADR-EDO-003
- Discovery MULTIMODAL_TARGET (superseded on MultimodalShipment rejection)
