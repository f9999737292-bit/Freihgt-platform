# EDO-0.2 Document State Machines

## Status

Architecture freeze — conceptual design only (EDO-0.2)

## Principle

**Orthogonal state dimensions** — never collapse business, delivery, signature, and operator transaction progress into one status column.

## Dimension overview

| Dimension | Scope | Owner | Storage concept |
|-----------|-------|-------|-----------------|
| **BUSINESS_STATE** | Document lifecycle in tenant workflow | document-service | `documents.business_state` |
| **DELIVERY_STATE** | Exchange/delivery to counterparty or operator | document-service | `documents.delivery_state` |
| **SIGNATURE_STATE** | Signing session progress | document-service | `signing_sessions.status` + aggregate on Document |
| **OPERATOR_TRANSACTION_STATE** | External operator processing | transport-edo-service | `epd_transactions.operator_state` |

Each dimension evolves independently; composite UI status is a **derived view**, not persisted SSOT.

---

## Generic Document

### BUSINESS_STATE

```text
DRAFT → IN_REVIEW → APPROVED → SUPERSEDED | VOIDED
```

| State | Meaning |
|-------|---------|
| DRAFT | Editable content |
| IN_REVIEW | Awaiting internal approval |
| APPROVED | Approved for signing/sending |
| SUPERSEDED | Replaced by newer revision/relationship |
| VOIDED | Cancelled before legal effect |

### DELIVERY_STATE

```text
NOT_SENT → OUTBOUND_PENDING → DELIVERED → DELIVERY_FAILED → ACKNOWLEDGED
```

### SIGNATURE_STATE

```text
NOT_REQUIRED → PENDING → PARTIALLY_SIGNED → SIGNED → SIGNATURE_REJECTED | SIGNATURE_EXPIRED
```

### OPERATOR_TRANSACTION_STATE

```text
NOT_APPLICABLE → NOT_SUBMITTED → SUBMITTED → PROCESSING → ACCEPTED | REJECTED | CANCELLED
```

(`NOT_APPLICABLE` for non-operator documents)

---

## UPD (Universal Transfer Document)

UPD adds regulated business transitions while sharing dimensions above.

### BUSINESS_STATE (UPD-specific extension)

```text
DRAFT → COMMERCIAL_APPROVED → LEGALLY_PREPARED → ISSUED → CORRECTION_PENDING → CORRECTED | ANNULLED
```

| State | Billing mirror | EDO SSOT |
|-------|----------------|----------|
| COMMERCIAL_APPROVED | Register item locked | Document still DRAFT legally |
| LEGALLY_PREPARED | Amounts frozen in billing | XML generated, pre-sign |
| ISSUED | `upd_documents.status` mirror | Signed + delivery started |
| CORRECTION_PENDING | UKD initiated in billing | Parent-child relationship |
| ANNULLED | Billing void | EDO void + archive seal |

**Rule:** Billing status mirror is **read-only projection** from `edo.document.*` events after `document_id` binding (ADR-EDO-002).

### DELIVERY_STATE (UPD)

Same as generic; operator path uses EDO operator when counterparty is operator-mediated.

### SIGNATURE_STATE (UPD)

KEP required for ISSUED transition. MChD evidence required when signing by representative — **EXTERNAL_LEGAL_VERIFICATION_REQUIRED**.

---

## Transport EPD

EPD documents (`document_type=EPD`) bind to `shipment_id` and optional `transport_leg_id`.

### BUSINESS_STATE

```text
DRAFT → EXECUTION_LINKED → READY_FOR_OPERATOR → CLOSED | VOIDED
```

| State | LOG coupling |
|-------|--------------|
| EXECUTION_LINKED | Shipment exists; leg optional |
| READY_FOR_OPERATOR | Signatures complete |
| CLOSED | Execution + operator acceptance complete |

### OPERATOR_TRANSACTION_STATE (primary for EPD)

```text
NOT_SUBMITTED → SUBMITTED → AWAITING_COUNTERPARTY → REGISTERED → AMENDED | REVOKED | REJECTED
```

Owned by **transport-edo-service**; document-service stores `OperatorReceipt` and `DeliveryEvidence` copies.

**Do not map operator-specific status codes into BUSINESS_STATE.**

---

## EETD package

EETD is a **DocumentPackage** lifecycle, not a single PDF.

### BUSINESS_STATE (package level)

```text
DRAFT → PARTIES_ALIGNED → EXCHANGE_OPEN → EXCHANGE_COMPLETE → ARCHIVED | DISPUTED
```

### Package member documents

Each member Document maintains its own four dimensions. Package BUSINESS_STATE aggregates via rules:

| Package transition | Requires |
|--------------------|----------|
| EXCHANGE_OPEN | All mandatory members SIGNATURE_STATE ≥ SIGNED |
| EXCHANGE_COMPLETE | All members DELIVERY_STATE ≥ ACKNOWLEDGED; operator states ACCEPTED where applicable |

### Links

| Anchor | Reference |
|--------|-----------|
| Shipment | `shipment_id` on package |
| TransportJourney / Legs | `transport_journey_id`, leg IDs on members |
| CargoHandover | Handover events trigger member document obligations |
| Signatures | Per-document SIGNATURE_STATE |
| Operator | Per-member OPERATOR_TRANSACTION_STATE via TEDO |

**EETD != single PDF** — artifacts live as Document revisions inside DocumentPackage.

---

## Example anti-pattern

```text
# FORBIDDEN
status = SIGNED_AND_SENT_AND_ACCEPTED

# CORRECT (derived view)
business_state = ISSUED
signature_state = SIGNED
delivery_state = ACKNOWLEDGED
operator_transaction_state = ACCEPTED
```

## References

- ADR-EDO-001, ADR-EDO-002, ADR-EDO-004
- `docs/architecture/edo-0.2-eetd-boundary.md`
