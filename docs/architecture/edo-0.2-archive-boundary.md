# EDO-0.2 Archive Boundary

## Status

Architecture freeze (EDO-0.2)

## Summary

Legal archive requirements for future implementation. **No S3 provisioning in this phase.** **Selectel WORM not verified.**

See ADR-EDO-007 for decision record.

## Requirements matrix

| Requirement | Owner | Implementation phase |
|-------------|-------|----------------------|
| Immutable original bytes | document-service + INFRA storage | EDO-0.4+ / INFRA-0.1 |
| Hash manifest (SHA-256+) | document-service | EDO-0.4+ |
| Version/revision history | document-service (existing DocumentRevision) | EDO-0.3+ |
| Signature evidence | document-service | EDO-0.3+ |
| Certificate evidence | document-service | EDO-0.3+ |
| Operator delivery evidence | document-service (copy from TEDO) | TEDO-0.5+ |
| Retention policy metadata | document-service | EDO-0.4+ |
| Tenant isolation | All access paths + storage prefix policy | INFRA-0.1 |
| Access audit | document-service audit + `core.audit_logs` correlation | EDO-0.4+ |
| Backup/restore | INFRA | INFRA-0.1 |
| Legal hold | `ArchiveManifest.legal_hold` flag | EDO-0.4+ |

## Storage tiers (conceptual)

```text
Tier 1 — Hot    document_files (current local/S3 metadata)
Tier 2 — Archive immutable manifest + WORM references
Tier 3 — Crypto  HSM/KMS (private keys never in archive tier)
```

## Verification gates

| Claim | Status |
|-------|--------|
| Selectel WORM / object lock | EXTERNAL_INFRA_VERIFICATION_REQUIRED |
| RF data residency | Staging docs suggest RF; production attestation separate |
| Encryption at rest | EXTERNAL_INFRA_VERIFICATION_REQUIRED |

## References

- ADR-EDO-007
- Discovery F-004
