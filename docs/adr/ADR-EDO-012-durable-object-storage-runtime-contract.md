# ADR-EDO-012: Durable object storage runtime contract

## Status

Accepted — architecture/runtime contract freeze (EDO-DURABLE-STORAGE-CONTRACT-0.4A).

This record freezes the runtime and operations contract for durable object bytes. It does not authorize implementation, staging provisioning, a provider mutation, or opening the document write gate.

```text
IMPLEMENTATION_AUTHORIZED=NO
STAGING_PROVISIONING_AUTHORIZED=NO
WRITE_GATE_OPEN_AUTHORIZED=NO
BASE_SHA=cbbba8e413c8ef1c716b242533d7f79aaaafe47a
```

Baseline `cbbba8e413c8ef1c716b242533d7f79aaaafe47a` is the source of this freeze. `docs/architecture/edo-0.3-current-state-inventory.md` remains a historical inventory against its own base and is not rewritten here.

## Context

`services/document-service/internal/platform/storage.ObjectStore` is the blob port. The implemented providers are `local` and `s3`. An empty `EDO_OBJECT_STORAGE_PROVIDER` selects local storage under `DOCUMENT_STORAGE_ROOT` (default `./data/document-storage`). An explicit `s3` configuration fails closed when endpoint, region, bucket, access key, or secret key is missing or unsafe. `Open` does not fall back from `s3` to local.

Attachment bytes use the server key `tenants/{tenant}/documents/{document}/attachments/{attachment}`. Detached signature bytes use `tenants/{tenant}/documents/{document}/attachment-signatures/{attachment}/{signature}`. Postgres tables `documents.document_attachments` (migration `000095`) and `documents.attachment_signature_blobs` plus `documents.signature_verification_evidence` (migration `000096`) own the authoritative identity, digest, size, and status.

The accepted staging runtime still uses a local, non-persistent document store. Attachment writes and signature-blob writes stay blocked. ADR-EDO-007 remains the legal-archive boundary. ADR-EDO-010 and ADR-EDO-011 remain the qualified-signature boundary. This stage does not provision Selectel, create a bucket, or claim object lock.

## Decision

1. `DURABLE_BLOB_PROVIDER=s3`. Staging and production blob writes use `s3`. The local provider is allowed for local development, tests, and explicitly non-durable fixtures. `LOCAL_PROVIDER_ALLOWED_FOR_STAGING_WRITES=NO`. `LOCAL_PROVIDER_ALLOWED_FOR_PRODUCTION_WRITES=NO`. Metadata and read paths may run while the write gate is blocked.
2. `S3_TO_LOCAL_FALLBACK=FORBIDDEN`. Missing or invalid S3 settings, and a remote storage error, fail the operation. A temporary local fallback is not part of this contract.
3. `DATABASE_METADATA_CANONICAL=YES`. Postgres owns tenant, document, attachment, and signature identity, the canonical object key, hash, size, media type, and status or evidence. S3 user metadata is non-authoritative and must not override Postgres authorization or identity.
4. Object keys stay server-derived and tenant-scoped. A caller must not supply an arbitrary object key. `EDO_OBJECT_STORAGE_PREFIX` may prepend an environment namespace. The resolved S3 key is `[prefix/] + canonical object key`. The database stores the canonical key, not the prefixed key. `OBJECT_KEY_OVERWRITE_ALLOWED=NO`.
5. Environments do not share a writable namespace. `CROSS_ENVIRONMENT_WRITE_NAMESPACE_SHARED=NO`. The preferred layout is one private bucket per environment. A shared physical bucket is a fallback only when the provider requires it, and then only with an environment prefix and a credential limited to that prefix.
6. Remote buckets are private for read and write. Anonymous list, public ACLs, and public bucket policies are forbidden. The runtime credential is least privilege: `PUT`, `GET`, `HEAD`, and `DELETE` inside the dedicated bucket or prefix. TLS is required for non-loopback S3. Metadata endpoints, credentials in the URL, secret logging, and redirects stay denied by the existing adapter. This stage does not weaken those checks.
7. Draft attachment objects may be deleted. Finalized attachment objects may not be deleted in the normal path. Signature blobs are write-once after a successful authoritative signature or evidence row. Delete is compensating cleanup only before that authoritative reference exists. An unknown signature-commit outcome must not delete the object. This is application immutability. `LEGAL_ARCHIVE_READY=NO`. `SELECTEL_OBJECT_LOCK_VERIFIED=NO`.
8. SHA-256 and size are required for attachment and signature blobs. The digest in Postgres is authoritative. `POSTGRES_BACKUP_INCLUDES_OBJECT_BLOBS=NO`.
9. `/health` does not prove remote S3 readiness. `LoadConfig` and `Open` construct a client. They do not prove the bucket or the credential. `REMOTE_STORAGE_PREFLIGHT_REQUIRED_BEFORE_WRITE_ENABLE=YES`. `CONTAINER_RECREATE_DURABILITY_PROOF_REQUIRED=YES`.
10. A later provider switch does not expect a database migration of object keys. If the local object count is greater than zero, each object must be copied and verified before the switch. `DUAL_WRITE=FORBIDDEN`. `DUAL_READ_FALLBACK=FORBIDDEN`. `POST_CUTOVER_LOCAL_ROLLBACK=FORBIDDEN` after the first authoritative S3 business write.
11. Durable S3 storage is not a qualified-signature production claim. `QES_PRODUCTION_READY=NO`. Verifier selection stays blocked under ADR-EDO-011. No migration is created.
12. The storage provider does not authorize tenants. The key prefix is defense in depth. There is no presigned arbitrary-write contract in this stage.
13. The staging attachment write gate and the staging signature-blob write gate stay `BLOCKED`. This stage does not open them. Agent A owns the later preflight `STAGING-EDO-DURABLE-STORAGE-PREFLIGHT-0.4B`, which is not authorized by this record.

The architecture note is [edo-0.4a-durable-object-storage-contract.md](../architecture/edo-0.4a-durable-object-storage-contract.md).

## Consequences

- Agent A may prepare a Selectel-compatible S3 staging design only in a later, separately authorized stage. This ADR is not permission to create a bucket, an access key, or a staging env change.
- Opening document writes requires a later controller stage with a valid S3 config, a private bucket, least privilege, a probe pass, recreate durability, and a safe local-object cutover.
- ADR-EDO-007 is unchanged. Hot durable blobs are not a WORM archive.
- ADR-EDO-011 is unchanged. `VALID` and `INVALID` stay closed.

## References

- ADR-EDO-007
- ADR-EDO-010
- ADR-EDO-011
- Migration `000095_edo_0_3_i2_signed_file_attachment`
- Migration `000096_edo_0_3_i4b_signature_verification_foundation`
