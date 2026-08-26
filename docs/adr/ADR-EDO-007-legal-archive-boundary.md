# ADR-EDO-007: Legal Archive Boundary

## Status

Accepted — architecture freeze (EDO-0.2)

## Context

Document files today use local filesystem (`document-service` `LocalObjectStore`). Discovery F-004 identified missing S3/WORM archive. EDO requires immutable legal archive for originals, signatures, and operator evidence. Selectel WORM capability is **not verified** in this phase.

## Decision

### Archive ownership

**document-service (EDO workstream) owns the legal archive domain model and `ArchiveManifest` aggregate.** Object storage infrastructure is provisioned by INFRA workstream; EDO defines requirements only.

### Requirements (design freeze)

| Requirement | Description |
|-------------|-------------|
| Immutable original bytes | Write-once storage of signed document artifacts; no in-place overwrite |
| Hash manifest | SHA-256 (or stronger) per artifact + manifest row linking revision chain |
| Version/revision history | All revisions indexed; post-sign revisions create new revision, never mutate signed bytes |
| Signature evidence | KEP signature files, timestamps, signer certificate references |
| Certificate evidence | Chain validation metadata (not private keys) |
| Operator delivery evidence | Receipt IDs, timestamps, operator correlation |
| Retention policy | Per document type + tenant policy metadata; enforcement deferred to INFRA |
| Tenant isolation | Prefix or bucket policy per tenant; access always scoped by JWT tenant |
| Access audit | Every archive read logged to audit trail |
| Backup/restore | INFRA responsibility; RPO/RTO TBD |
| Legal hold capability | Flag on Document/ArchiveManifest preventing deletion despite retention expiry |

### Explicit non-claims

- **Selectel WORM capability:** `EXTERNAL_INFRA_VERIFICATION_REQUIRED` — do not claim until INFRA verifies object lock / immutability on target bucket.
- **Regulatory certification:** `EXTERNAL_LEGAL_VERIFICATION_REQUIRED`.
- **S3 provisioning:** not in EDO-0.2 scope.

### Storage layering

```
Hot storage    — recent document files (existing document_files metadata)
Archive tier   — immutable manifest + WORM object references (future)
Crypto segment — private keys / HSM (future INFRA-0.2; not in document-service)
```

## Consequences

- EDO-0.3+ adds `ArchiveManifest` schema in `documents` schema only
- Billing and TEDO store references to archive manifest IDs, not duplicate bytes
- Production EDO blocked on INFRA-0.1 object storage design (discovery blocker)

## References

- ADR-EDO-001
- Discovery findings F-004, F-005
- `services/document-service/internal/platform/storage/local.go`
