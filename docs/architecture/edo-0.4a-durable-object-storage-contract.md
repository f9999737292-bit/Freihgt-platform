# EDO 0.4A durable object storage contract

## Status

```text
STAGE=EDO-DURABLE-STORAGE-CONTRACT-0.4A
BASE_SHA=cbbba8e413c8ef1c716b242533d7f79aaaafe47a
IMPLEMENTATION_AUTHORIZED=NO
STAGING_PROVISIONING_AUTHORIZED=NO
WRITE_GATE_OPEN_AUTHORIZED=NO
DURABLE_BLOB_PROVIDER=s3
LEGAL_ARCHIVE_READY=NO
SELECTEL_OBJECT_LOCK_VERIFIED=NO
QES_PRODUCTION_READY=NO
STAGING_ATTACHMENT_WRITE_GATE=BLOCKED
STAGING_SIGNATURE_BLOB_WRITE_GATE=BLOCKED
```

Decision record: [ADR-EDO-012](../adr/ADR-EDO-012-durable-object-storage-runtime-contract.md).

This note freezes the runtime contract for attachment bytes and detached signature blobs. It does not provision storage, change staging, open writes, implement a legal archive, or make a qualified signature production-ready.

`docs/architecture/edo-0.3-current-state-inventory.md` stays a historical inventory against its own base. This note supersedes only the current durable-storage runtime guidance. It does not rewrite that historical evidence.

## Source truth at this baseline

`ObjectStore` lives in `services/document-service/internal/platform/storage`. Implemented providers are `local` and `s3`.

```text
EMPTY_PROVIDER_SELECTS_LOCAL=YES
EXPLICIT_S3_INVALID_CONFIG_FAILS_CLOSED=YES
S3_TO_LOCAL_FALLBACK_PRESENT=NO
S3_TO_LOCAL_FALLBACK=FORBIDDEN
```

An empty `EDO_OBJECT_STORAGE_PROVIDER` selects the local adapter. The local root is `DOCUMENT_STORAGE_ROOT`, default `./data/document-storage`. Explicit `s3` requires endpoint, region, bucket, access key, and secret key. A missing or invalid value fails closed. A remote error fails the operation. There is no design for a temporary local fallback.

S3 settings:

```text
EDO_OBJECT_STORAGE_PROVIDER
EDO_OBJECT_STORAGE_ENDPOINT
EDO_OBJECT_STORAGE_REGION
EDO_OBJECT_STORAGE_BUCKET
EDO_OBJECT_STORAGE_ACCESS_KEY
EDO_OBJECT_STORAGE_SECRET_KEY
EDO_OBJECT_STORAGE_PATH_STYLE
EDO_OBJECT_STORAGE_TLS
EDO_OBJECT_STORAGE_PREFIX
```

Accepted staging state at the start of this stage, unchanged by this document:

```text
DOCUMENT_STORAGE_PROVIDER=local
DOCUMENT_STORAGE_DURABLE=NO
DOCUMENT_STORAGE_ROOT_PERSISTENT=NO
DOCUMENT_ATTACHMENT_WRITE_GATE=BLOCKED
DOCUMENT_SIGNATURE_BLOB_WRITE_GATE=BLOCKED
```

## Provider decision

```text
DURABLE_BLOB_PROVIDER=s3
STAGING_BLOB_WRITE_PROVIDER=s3
PRODUCTION_BLOB_WRITE_PROVIDER=s3
LOCAL_PROVIDER_ALLOWED_FOR=local development / tests / explicitly non-durable fixtures
LOCAL_PROVIDER_ALLOWED_FOR_STAGING_WRITES=NO
LOCAL_PROVIDER_ALLOWED_FOR_PRODUCTION_WRITES=NO
```

A service may keep metadata and read routes available while those write gates stay blocked.

## Data ownership

```text
DATABASE_METADATA_CANONICAL=YES
OBJECT_STORAGE_METADATA_NON_AUTHORITATIVE=YES
```

Postgres owns `tenant_id`, `document_id`, `attachment_id`, `signature_id`, the canonical `storage_key` or `object_key`, the digest, the size, the media type, and status or evidence, including policy and version fields.

| Bytes | Canonical key | Authoritative table |
| --- | --- | --- |
| Attachment | `tenants/{tenant}/documents/{document}/attachments/{attachment}` | `documents.document_attachments` (`000095`) |
| Detached signature | `tenants/{tenant}/documents/{document}/attachment-signatures/{attachment}/{signature}` | `documents.attachment_signature_blobs` (`000096`) |
| Verification evidence | none; evidence is a database row | `documents.signature_verification_evidence` (`000096`) |

S3 `x-amz-meta` fields may help diagnosis. They must not override Postgres identity or authorization.

```text
OBJECT_KEY_TENANT_SCOPED=YES
OBJECT_KEY_SERVER_DERIVED=YES
CLIENT_SUPPLIED_ARBITRARY_OBJECT_KEY=NO
```

The resolved S3 key is `[prefix/] + canonical object key`. The prefix is an environment or service namespace. It does not change the key stored in Postgres.

The tenant segment is defense in depth. Authorization stays at the document-service and API Gateway boundary. The S3 caller must not choose an arbitrary tenant prefix. This stage has no presigned arbitrary-write contract. A later presigned upload needs its own threat model.

## Environment and bucket

```text
CROSS_ENVIRONMENT_WRITE_NAMESPACE_SHARED=NO
PRIVATE_READ=YES
PRIVATE_WRITE=YES
ANONYMOUS_LIST=NO
PUBLIC_ACL=NO
PUBLIC_BUCKET_POLICY=NO
PUBLIC_READ_ALLOWED=NO
PUBLIC_WRITE_ALLOWED=NO
LEAST_PRIVILEGE=YES
TLS_REQUIRED_FOR_REMOTE_S3=YES
```

The preferred layout is one dedicated bucket per environment. Production and staging must not share a writable namespace. If a provider forces one physical bucket, that layout is a fallback: an environment-specific prefix is required, and the credential is limited to that prefix. This note does not invent production bucket names.

The runtime credential may `PUT`, `GET`, `HEAD`, and `DELETE` inside its bucket or prefix. It does not receive bucket administration, policy administration, credential administration, or cross-environment object access unless a later provider-evidence stage proves that a narrower grant is impossible.

Non-loopback S3 requires `https`. The current adapter rejects metadata hosts, credentials in the URL, and redirects (`CheckRedirect` returns `ErrUseLastResponse`). Secret values are not logged. This stage does not weaken those checks.

## Write-once and delete

```text
OBJECT_KEY_OVERWRITE_ALLOWED=NO
DRAFT_ATTACHMENT_DELETE_ALLOWED=YES
FINALIZED_ATTACHMENT_NORMAL_DELETE_ALLOWED=NO
SIGNATURE_BLOB_MUTATION_ALLOWED=NO
SIGNATURE_BLOB_NORMAL_DELETE_ALLOWED=NO
COMPENSATING_DELETE_ALLOWED_BEFORE_AUTHORITATIVE_COMMIT=YES
```

S3 `Put` sends `If-None-Match: *`. The local adapter also rejects an existing target. A repeated write of the same canonical key fails as object-exists unless a higher-level idempotency check proves an exact replay and does not rewrite the bytes.

Hot durable storage is not the legal archive. A draft attachment object may be deleted. A finalized attachment object may not be deleted on the normal path. The current service returns a conflict for that delete.

A detached signature blob is write-once after the authoritative signature or evidence row commits. Delete is allowed only as compensation when the object write happened and no authoritative successful reference remains. If the signature commit outcome is unknown, the object is not deleted. That rule avoids a committed row pointing at missing bytes. This is application immutability. It is not provider-level WORM.

## Integrity, health, and metrics

```text
SHA256_REQUIRED_FOR_ATTACHMENT=YES
SHA256_REQUIRED_FOR_SIGNATURE_BLOB=YES
DOCUMENT_SERVICE_HEALTH_PROVES_S3_READY=NO
REMOTE_STORAGE_PREFLIGHT_REQUIRED_BEFORE_WRITE_ENABLE=YES
STORAGE_OBSERVABILITY_MINIMUM_READY=CONTRACT_ONLY
```

Reads that the current service already verifies must keep checking bytes against the Postgres digest and size. A digest copied into S3 metadata is not authoritative.

`LoadConfig` and `Open` validate configuration and construct the S3 client. They do not prove that the bucket exists or that the credential works. HTTP 200 from document-service health is not `S3_READY`.

Current counters, with no tenant, key, or secret labels, are `object_storage_put_total`, `object_storage_get_total`, `object_storage_errors_total`, `object_storage_latency_seconds`, and `integrity_check_failures_total`. The operations minimum for a later stage also names head and delete success and failure, latency, integrity mismatches, and storage-unavailable errors. That minimum is a contract only. This stage does not add metrics.

## Future Agent A checks

These checks are defined and not executed. `NEXT_STAGE_AUTHORIZED=NO`.

Next infrastructure stage, owner Agent A:

```text
NEXT_STAGE=STAGING-EDO-DURABLE-STORAGE-PREFLIGHT-0.4B
```

That preflight is read-only toward the provider until a controller authorizes mutation. A later provision stage, only after that authorization, is `STAGING-EDO-DURABLE-STORAGE-PROVISION-0.4C`.

A later controlled probe, after explicit controller authorization, must show:

1. The endpoint is reachable.
2. The credential is accepted.
3. The private bucket or namespace is accessible.
4. `PUT` of a small random object under a dedicated probe prefix succeeds.
5. `HEAD` returns the expected size.
6. `GET` returns the same bytes.
7. `DELETE` succeeds.
8. The object is absent after delete.
9. Anonymous `GET` is denied.
10. Anonymous `LIST` is denied.
11. Anonymous `PUT` is denied.

The probe must not use a real tenant or document key. The logical namespace is `_bintrans-probes/<stage>/<uuid>`. No permanent object is hardcoded.

Before the staging write freeze can be removed, a later proof must upload a non-business probe, read it back from S3, recreate only the document-service container, read the same object, leave the database unchanged, and still report provider `s3`.

```text
CONTAINER_RECREATE_DURABILITY_PROOF_REQUIRED=YES
```

Before a provider switch, Agent A inventories local objects. A zero count needs no blob copy. A non-zero count needs a controlled copy of each canonical key with size, SHA-256, and presence checks, and zero missing objects, before the switch. Database keys stay as they are.

```text
PROVIDER_SWITCH_DB_MIGRATION_EXPECTED=NO
LOCAL_OBJECT_COPY_REQUIRED_IF_NONZERO=YES
```

This note does not claim the current staging object count is zero. That count needs a fresh later check.

## Rollback, backup, archive, and QES

Before the first successful S3 business write, configuration rollback is possible. After that write:

```text
ROLLBACK_PROVIDER_TO_LOCAL=FORBIDDEN
DUAL_WRITE=FORBIDDEN
DUAL_READ_FALLBACK=FORBIDDEN
POST_CUTOVER_LOCAL_ROLLBACK=FORBIDDEN
```

The current runtime has no dual-read path. An image rollback is allowed only when the rolled-back image understands the same S3 contract and the same object keys.

```text
POSTGRES_BACKUP_INCLUDES_OBJECT_BLOBS=NO
```

Object recovery is a separate future concern: provider durability or versioning, replication if required, restore steps, and RPO/RTO. None of that is implemented here.

ADR-EDO-007 stays the legal-archive authority.

```text
DURABLE_S3_BLOB_STORAGE != LEGAL_WORM_ARCHIVE
LEGAL_ARCHIVE_READY=NO
SELECTEL_OBJECT_LOCK_VERIFIED=NO
```

This contract does not prove object lock, WORM, retention, legal hold, or a regulatory archive certification. No external verification was performed in this stage.

ADR-EDO-010 and ADR-EDO-011 stay the qualified-signature authority. Storing a signature blob durably does not make a qualified signature production-ready. Verifier selection stays blocked. `VALID` and `INVALID` are not reopened. No migration is created.

```text
QES_FOUNDATION_STORAGE_DEPENDENCY_READY_AFTER_S3=PARTIAL_ONLY
QES_PRODUCTION_READY=NO
```

## Secrets and write gate

Future values of `EDO_OBJECT_STORAGE_ACCESS_KEY` and `EDO_OBJECT_STORAGE_SECRET_KEY` stay in protected environment only. They are not committed, printed, or copied into generated compose evidence. They are least privilege and independently rotatable. Agent A installs them. Agent B owns this application contract.

```text
SECRETS_REPOSITORY_ALLOWED=NO
```

The write gate stays closed until a separate controller stage records:

```text
S3_CONFIG_VALID=YES
BUCKET_PRIVATE=YES
LEAST_PRIVILEGE=YES
PROBE_PASS=YES
CONTAINER_RECREATE_DURABILITY=PASS
LOCAL_OBJECT_CUTOVER_SAFE=YES
```

```text
WRITE_GATE_OPENED_IN_0_4A=NO
STAGING_ATTACHMENT_WRITE_GATE=BLOCKED
STAGING_SIGNATURE_BLOB_WRITE_GATE=BLOCKED
```

## Gaps

```text
GAP_STORAGE_001=staging provider is local/non-persistent
GAP_STORAGE_002=service health does not prove remote S3 readiness
GAP_STORAGE_003=no provider-level WORM/object-lock proof
GAP_STORAGE_004=object bytes are outside PostgreSQL backup
GAP_STORAGE_005=provider rollback to local unsafe after first S3 write
GAP_STORAGE_006=production QES verifier independently unavailable
```

These gaps are recorded. This stage does not implement a fix for them.
