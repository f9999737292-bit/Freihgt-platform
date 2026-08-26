# EDO-0.2 EPD Operator Port Specification

## Status

Architecture freeze — conceptual only (EDO-0.2)

## Ownership

**transport-edo-service** owns port and adapters (ADR-EDO-004). Not in shared-go unless cross-service invocation is demonstrated.

## Layer structure

```
transport-edo-service/
  domain/
    EPDTransaction          # operator-agnostic correlation
    OperatorSubmission      # domain command result
  application/
    SubmitDocumentUseCase
    PollTransactionUseCase
    InboundDocumentUseCase
  ports/
    EPDOperator             # interface below
  adapters/
    ExternalOperatorA
    ExternalOperatorB
    FutureBintransIS_EPD    # reserved; not connected
```

## EPDOperator port (conceptual operations)

### SubmitDocument

**Input (domain-level):**

| Field | Type | Notes |
|-------|------|-------|
| `tenant_id` | UUID | |
| `document_id` | UUID | Signed document in document-service |
| `document_revision_id` | UUID | Specific revision submitted |
| `shipment_id` | UUID | Optional but typical for EPD |
| `transport_leg_id` | UUID | Optional |
| `idempotency_key` | string | Client-supplied |

**Output:**

| Field | Type |
|-------|------|
| `epd_transaction_id` | UUID |
| `operator_reference` | string (opaque) |
| `initial_operator_state` | enum |

### GetTransactionStatus

**Input:** `epd_transaction_id`, `tenant_id`

**Output:** `operator_state`, `operator_reference`, `last_updated_at`, optional `rejection_reason_code` (domain enum, not provider code)

### ReceiveInboundDocument

**Input:** opaque operator payload (adapter parses)

**Output:** `document_id` (created or matched), `inbound_type`, `correlation_ids`

### Acknowledge

**Input:** `epd_transaction_id`, `ack_type`

**Output:** success / retryable failure

### Reject

**Input:** `epd_transaction_id`, `rejection_reason_code`, `detail`

**Output:** updated `operator_state`

### GetDeliveryEvidence

**Input:** `epd_transaction_id`

**Output:** `DeliveryEvidence` DTO (timestamps, receipt reference, hash) — persisted to document-service by application layer

## Domain rejection reason codes (examples)

| Code | Meaning |
|------|---------|
| `INVALID_FORMAT` | Schema/format rejected |
| `SIGNATURE_INVALID` | Signature verification failed |
| `COUNTERPARTY_UNKNOWN` | Recipient not registered |
| `DUPLICATE_SUBMISSION` | Idempotent duplicate |
| `OPERATOR_UNAVAILABLE` | Transient |

Adapters map provider-specific codes to these — **never expose provider codes in public API**.

## Configuration flags

```text
EXTERNAL_OPERATOR_MODE=YES
FUTURE_OWN_OPERATOR_READY=YES
OWN_IS_EPD_OPERATOR_MODE=NO
GIS_EPD_CONNECTED=NO
```

## Gaps (document only)

- Real operator WSDL/OpenAPI mappings
- Credential rotation runbook
- Multi-operator routing policy implementation
- Retry/backoff per operator SLA
- Regulatory certification

## References

- ADR-EDO-004, ADR-EDO-008
- Discovery TRANSPORT_EDO_TARGET
