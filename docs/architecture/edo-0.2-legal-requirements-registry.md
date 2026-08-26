# EDO-0.2 Legal Requirements Registry

## Status

Architecture freeze — structure only; no verified regulatory content (EDO-0.2)

## Purpose

Requirements registry for future **verified** legal/compliance rules. Architecture must allow rule/version updates without recompiling unrelated domains.

## Registry record structure

| Field | Description |
|-------|-------------|
| `LEGAL_REQUIREMENT_ID` | Stable identifier, e.g. `LR-RU-UPD-FORMAT-001` |
| `SOURCE` | Authoritative source citation (law, FNS order, Mintrans directive) — must be external |
| `EFFECTIVE_FROM` | Date rule applies |
| `EFFECTIVE_TO` | Optional expiry |
| `AFFECTED_DOCUMENT_TYPE` | `UPD`, `EPD`, `ETRN`, `EETD`, etc. |
| `FORMAT_VERSION` | Binding to FormatVersion catalog |
| `SIGNATURE_RULE` | KEP, MChD, counterparty rules summary |
| `RETENTION_RULE` | Minimum retention period or reference |
| `SYSTEM_CONTROL` | Which service enforces (document-service, transport-edo-service) |
| `VERIFICATION_STATUS` | See status enum below |

## Verification status enum

| Status | Meaning |
|--------|--------|
| `EXTERNAL_LEGAL_VERIFICATION_REQUIRED` | Default for all entries until legal review |
| `VERIFIED` | Confirmed against authoritative external source |
| `SUPERSEDED` | Replaced by newer requirement ID |
| `NOT_APPLICABLE` | Rule does not apply to BINTRANS deployment |

## Initial placeholder entries (unverified)

| LEGAL_REQUIREMENT_ID | AFFECTED_DOCUMENT_TYPE | VERIFICATION_STATUS |
|----------------------|------------------------|---------------------|
| LR-RU-UPD-FORMAT-001 | UPD | EXTERNAL_LEGAL_VERIFICATION_REQUIRED |
| LR-RU-UPD-SIGN-001 | UPD | EXTERNAL_LEGAL_VERIFICATION_REQUIRED |
| LR-RU-UKD-FORMAT-001 | UPD (correction) | EXTERNAL_LEGAL_VERIFICATION_REQUIRED |
| LR-RU-EPD-FORMAT-001 | EPD | EXTERNAL_LEGAL_VERIFICATION_REQUIRED |
| LR-RU-ETRN-FORMAT-001 | ETRN | EXTERNAL_LEGAL_VERIFICATION_REQUIRED |
| LR-RU-EETD-FORMAT-001 | EETD | EXTERNAL_LEGAL_VERIFICATION_REQUIRED |
| LR-RU-MCHD-001 | All KEP by representative | EXTERNAL_LEGAL_VERIFICATION_REQUIRED |
| LR-RU-ARCHIVE-RET-001 | All archived documents | EXTERNAL_LEGAL_VERIFICATION_REQUIRED |

**No statutory text is reproduced in this registry.** Populate via legal review workstream.

## Extensibility mechanism (design)

1. **FormatDefinition / FormatVersion** — runtime binding of XSD/rules to document types.
2. **ValidationResult** — outcome recorded per validation run against active FormatVersion.
3. **Feature flags** — tenant-level enablement of format versions without redeploying LOG/FC services.
4. **Rule engine placement** — validation executes in document-service; operator rules in transport-edo-service adapters.

## Compliance boundary

Architecture **does not claim compliance**. All regulatory details require external verification before production assertions.

## References

- ADR-EDO-001, ADR-EDO-007
- Discovery finding F-005
