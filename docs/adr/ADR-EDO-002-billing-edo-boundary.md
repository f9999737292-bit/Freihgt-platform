# ADR-EDO-002: Billing ↔ EDO Boundary

## Status

Accepted — architecture freeze (EDO-0.2)

## Context

Discovery finding **F-009 (UPD SSOT split)**: Universal Transfer Documents (UPD) and related closing documents exist in two places:

1. **Billing SSOT** — `billing.upd_documents`, `billing.invoices`, `billing.acts`, `billing.vat_invoices`, `billing.closing_document_packages` with monetary fields, seller/buyer companies, function codes, and billing register lifecycle.
2. **Document registry** — `documents.documents` with type `UPD`, versions, files, signing — linked optionally via `billing.upd_documents.document_id UUID`.

Today billing can advance UPD status (including mock `mark-sent-to-edo`) without a populated `document_id`. This creates duplicate truth risk for legally significant representation vs commercial projection.

## Decision

### Separation of concerns (frozen)

| Domain | Billing owns | EDO owns |
|--------|--------------|----------|
| Intent | Commercial/accounting intent; register item linkage | Legally significant electronic document |
| Money | Amounts, VAT rates, function codes, settlement basis | None (reference amounts only in read models if needed for display) |
| Lifecycle | Register status, closing package assembly, payment handoff | Document business/delivery/signature/operator dimensions |
| Payload | Summary fields for billing UI and freight-cost snapshots | XML, PDF artifact bytes, format version, revisions |
| Evidence | Billing audit events | Signatures, MChD, validation, operator receipts, archive |

### Relationship model

```
BillingRegister (billing-register-service)
    └── BillingRegisterItem[] ──references──▶ Shipment.id
    └── UPDDocument (commercial projection)
            document_id ──FK reference──▶ Document.id (document-service)
    └── ClosingDocumentPackage
            package_document_ids[] ──references──▶ DocumentPackage.id
```

**Rules:**

1. **Monetary truth lives in billing only.** EDO must not persist authoritative `amount_with_vat` or tax calculation outputs.
2. **Legal/XML truth lives in EDO only.** Billing must not store duplicate XML or signature blobs once `document_id` is bound.
3. **`document_id` is mandatory** before any UPD transitions to operator-facing states (`SENT_TO_OPERATOR`, `ACCEPTED`, archived). Until bound, billing status reflects **commercial draft only**.
4. **Synchronization is reference-based**, not payload copy. Billing may cache denormalized display fields (number, date, status mirror) with explicit `last_synced_at` — never a second editable XML source.
5. **Correction/cancellation UPD (UKD)** — new EDO Document with DocumentRelationship to parent; billing receives new commercial row or version pointer, not in-place XML mutation.

### Anti-patterns (blocked)

- Storing full UPD XML in `billing.upd_documents`
- Treating `documents.documents.payload_json` as VAT calculation SSOT
- Creating `edo_billing_registers` parallel table
- Duplicating seller/buyer identity beyond `core.companies` UUID references

## Consequences

### Positive

- Resolves F-009 without merging bounded contexts
- Enables EDO archive and operator workflows independent of billing register edits
- Preserves existing billing → payment obligation → freight-cost pipeline

### Negative / migration

- EDO-0.5+ implementation must backfill `document_id` for existing UPD rows before operator integration
- Billing mock `mark-sent-to-edo` must be replaced by EDO-driven status mirror updates
- Cross-workstream contract required for billing to subscribe to `edo.document.*` status events (read-only mirror)

## References

- ADR-EDO-001
- `infrastructure/migrations/000006_create_billing_tables.up.sql`
- Discovery finding F-003 (mock EDO path)
