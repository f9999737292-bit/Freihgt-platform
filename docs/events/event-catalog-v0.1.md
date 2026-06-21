# Event Catalog v0.1

> Placeholder — domain events for freight-platform.

## Status

Skeleton only. No event bus implementation yet.

## Planned event namespaces

| Namespace | Examples |
|-----------|----------|
| `core.*` | `UserCreated`, `CompanyRegistered` |
| `transport.*` | `TransportOrderCreated`, `ShipmentDispatched` |
| `rfx.*` | `RfxPublished`, `BidSubmitted` |
| `documents.*` | `DocumentUploaded`, `DocumentSigned` |
| `billing.*` | `InvoiceIssued`, `BillingRegisterClosed` |

## Event envelope (draft)

```json
{
  "id": "uuid",
  "type": "transport.TransportOrderCreated",
  "occurredAt": "2026-06-17T00:00:00Z",
  "tenantId": "uuid",
  "payload": {}
}
```

## Next steps

- Define versioned payloads in `packages/proto` or JSON Schema
- Document producers and consumers per service
