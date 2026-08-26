# EDO-0.2 EETD Boundary

## Status

Architecture freeze (EDO-0.2)

## Principle

**EETD != single PDF**

ЕЭТД (electronic unified transport document) is modeled as a **legally significant document/exchange lifecycle**, not a file blob or alternate shipment.

## Scope

| Concern | Design |
|---------|--------|
| Aggregate | `DocumentPackage` with type `EETD` (document-service) |
| Execution anchor | `shipment_id` + optional `transport_journey_id` |
| Leg scope | Member documents may bind `transport_leg_id` |
| Custody transitions | `CargoHandover` events trigger exchange milestones (MM → EDO via events) |
| Artifacts | Multiple `Document` members with XML/PDF revisions — no monolithic PDF SSOT |
| Signatures | Per-document SIGNATURE_STATE; package seal when all mandatory members signed |
| Operator | Per-member OPERATOR_TRANSACTION_STATE via transport-edo-service |
| Shipment duplication | **Forbidden** — reference IDs only |

## Lifecycle (summary)

See [edo-0.2-document-state-machines.md](edo-0.2-document-state-machines.md#eetd-package).

## Integration map

```text
shipment-service          document-service           transport-edo-service
     │                           │                            │
     │ shipment_id               │ DocumentPackage (EETD)     │
     │ transport_leg_id          │ Document[] members         │
     │ cargo_handover.* ──event──▶│ milestone obligations      │
     │                           │ signed doc ──ref──▶        │ epd/edo operator
     │                           │◀── evidence ───────────────│
```

## Explicit non-scope (EDO-0.2)

- XML schema implementation
- Regulatory format validation
- GIS EPD status mapping (**EXTERNAL_LEGAL_VERIFICATION_REQUIRED**)

## References

- ADR-EDO-003, ADR-EDO-004
- Discovery EETD_TARGET
