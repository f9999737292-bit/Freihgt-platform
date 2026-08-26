# ADR-EDO-004: EPD Operator Port Ownership

## Status

Accepted — architecture freeze (EDO-0.2)

## Context

Discovery v0.1 noted document types `ETRN`, `EPD`, `WAYBILL` in document-service with statuses including `SENT_TO_OPERATOR`, but **no operator integration code** exists. Discovery initially suggested `packages/shared-go/edo/epdport` for the port contract.

EDO-0.2 requires: do not place EPD operator business interfaces in generic shared packages unless a demonstrated cross-service requirement exists.

## Decision

### Service ownership

**`transport-edo-service` (TEDO workstream) owns the EPD operator port and all external operator adapters.**

Proposed structure (conceptual — no implementation in EDO-0.2):

```
transport-edo-service/
  domain/           # EPD transaction, operator-agnostic lifecycle
  application/      # Use cases: submit, poll status, ingest inbound
  ports/
    EPDOperator     # Interface — operator-agnostic operations
  adapters/
    ExternalOperatorA
    ExternalOperatorB
    FutureBintransIS_EPD   # Own-operator adapter slot (not connected)
```

### Port operations (conceptual)

| Operation | Purpose |
|-----------|---------|
| `SubmitDocument` | Send legally prepared document package to operator |
| `GetTransactionStatus` | Poll operator processing state |
| `ReceiveInboundDocument` | Accept operator-initiated document delivery |
| `Acknowledge` | Confirm receipt to operator |
| `Reject` | Reject with reason code (domain-level, not provider-specific) |
| `GetDeliveryEvidence` | Retrieve operator delivery proof metadata |

**Do not define provider-specific API fields** in the port contract. Adapters map provider payloads internally.

### Boundaries

| Component | Owner |
|-----------|-------|
| EPD operator port interface + adapters | transport-edo-service |
| Legally significant document bytes, signatures | document-service (EDO) |
| Shipment / leg correlation IDs | shipment-service (LOG) — referenced by UUID |
| Operator credentials vault | INFRA + TEDO (CRITICAL segment) |
| Shared-go extraction | **Deferred** until a second service must invoke the port directly; if needed, extract **thin DTO types only**, not business logic |

### Integration flow

```
document-service (signed EPD Document)
    ──reference──▶ transport-edo-service (EPDTransaction)
                        ──EPDOperator port──▶ External operator
    ◀──DeliveryEvidence / OperatorReceipt ── document-service (persist evidence)
```

GIS EPD / Mintrans connectivity: **NOT CONNECTED** in any phase until licensed. Adapter slot preserved per ADR-EDO-008.

## Consequences

### Positive

- Operator coupling isolated from document core and shared libraries
- Enables adapter substitution without changing EDO signing/archive workflows
- Resolves discovery F-003 at design level

### Negative

- New service to deploy and secure on staging (future phase)
- document-service ↔ transport-edo-service synchronous/API contract required

## References

- ADR-EDO-001, ADR-EDO-008
- Discovery finding F-003
- `services/document-service/internal/domain/document.go` (document types)
