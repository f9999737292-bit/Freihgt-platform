# EDO 0.3 W1 — I1 Schema Foundation Task Contract

## Status

```text
DOCUMENT_STATUS=TASK_CONTRACT
CONTROLLER_DECISION=ACCEPT_EDO_0_3_W1_I1_TASK_CONTRACT
W1_STATUS=IMPLEMENTATION_CONTRACT_ACCEPTED
IMPLEMENTATION_AUTHORIZED=I1_ONLY
AUTHORIZED_WAVE=I1
I1_STATUS=AUTHORIZED_NOT_STARTED
I1_IMPLEMENTATION_STARTED=NO
I2_STATUS=NOT_AUTHORIZED
I3_STATUS=NOT_AUTHORIZED
I4_STATUS=NOT_AUTHORIZED
PRODUCT_CODE_CHANGED=NO
MIGRATION_CREATED=NO
OPENAPI_CHANGED=NO
LEGAL_VERIFICATION_STATUS=OPEN
LEGAL_VERIFICATION_REQUIRED
```

This document freezes the implementation contract for wave I1. Controller review accepted that contract for I1 only. I1 has not started. I2, I3, and I4 stay unauthorized.

## Purpose

W1 converts accepted Variant A discovery into a reviewable Task Contract for:

```text
I1 — EDO-0.3 Schema Foundation
```

I1 is an additive `documents` schema foundation for `DocumentPackage`, package membership, append-only `DocumentRelationship`, revision-bound signature evidence, certificate evidence metadata, and database constraints that protect signed artifacts from delete and in-place payload change. I1 does not add public routes, service commands, events, or legal-validity claims.

## Baseline

```text
REPOSITORY_ROOT=D:/Projects/freight-platform-wt/edo-agent-b-v0.1
SOURCE_BRANCH=discovery/edo-agent-b-v0.1
WORK_BRANCH=docs/edo-0.3-w1-i1-task-contract-v0.1
BASE_SHA=1566c0d30b93bca1016fed6e81620edf277be52f
ORIGIN_MAIN_SHA=1566c0d30b93bca1016fed6e81620edf277be52f
HEAD_EQUALS_ORIGIN_MAIN_AT_BRANCH_START=YES
WORKTREE_CLEAN_AT_BRANCH_START=YES
```

`discovery/edo-agent-b-v0.1` stays in place. This contract does not rebase unrelated branches.

## Prerequisites

I1 may start only after all of the following are true:

1. Controller review of this document records `IMPLEMENTATION_AUTHORIZED=I1_ONLY`.
2. Accepted state remains: EDO 0.2 `IMPLEMENTED_ACCEPTED`, EDO 0.3 discovery `DISCOVERY_ACCEPTED`, variant A, S1 `IMPLEMENTED_ACCEPTED`.
3. S1 evidence stays the accepted read-isolation floor: pull request #164, product head `dff9eacf04e0fd8c0e381d8716cf5a4039c90ff1`, CI run `35991273506`, `TestDocumentReadTenantIsolation` PASS. I1 does not repeat or reopen S1.
4. The I1 implementer rediscovers the migration head immediately before writing a migration. The inventory below is informational and does not reserve a number.
5. A cross-tenant precheck on existing `documents.signatures` and `documents.signing_sessions` returns zero mismatched rows. A non-zero result stops I1. Rows are not repaired by inference.

S1 is already satisfied on current `main`. The discovery sentence that get-by-id is not fail-closed describes the pre-S1 defect. Current `DocumentRepository.GetDetail` calls `GetByIDAndTenant`. Current `SigningService.GetSession` calls `GetSessionByIDAndTenant` and returns unauthorized when the trusted tenant is missing.

## Accepted variant

```text
VARIANT=A
```

Variant A, as accepted in [edo-0.3-architecture-options.md](edo-0.3-architecture-options.md) and [edo-0.3-scope.md](edo-0.3-scope.md):

```text
DocumentPackage
DocumentRelationship
immutable revision rules
revision-bound signing/certificate evidence
documents schema only
```

No contradiction in current `main` makes variant A impossible. Variants B, C, and D stay closed.

```text
ARCHIVE_MANIFEST=OUT
MCHD=OUT
STATE_SPLIT=OUT
OPERATOR_RUNTIME=OUT
ETRN_RUNTIME=OUT
BILLING_BRIDGE=OUT
FACTORING=OUT
```

ADR-EDO-001 remains in force: `document-service` owns `DocumentPackage`, `DocumentRelationship`, `DocumentSignature`, and `CertificateEvidence`. The ADR aggregate-root cell that names `DocumentRelationship` as "DocumentPackage or Document" is not an authorization to store package membership twice. Accepted variant A is more specific: membership has one store. This contract does not replace ADR-EDO-001.

## Authoritative current schema

Inspected on `BASE_SHA`. Not modified by W1.

```text
DOCUMENT_SCHEMA=documents
SCHEMA_CREATED_IN=infrastructure/migrations/000001_create_schemas.up.sql
DOCUMENTS_TABLE=documents.documents
DOCUMENT_VERSIONS_TABLE=documents.document_versions
DOCUMENT_FILES_TABLE=documents.document_files
SIGNATURES_TABLE=documents.signatures
SIGNING_SESSIONS_TABLE=documents.signing_sessions
UPLOAD_INTENT_TABLE=documents.document_upload_intent
```

### `documents.documents` (migration `000005`)

| Column | Notes |
|--------|--------|
| `id` | UUID primary key |
| `tenant_id` | UUID NOT NULL. No composite unique with `id` yet |
| `document_number` | Unique with `tenant_id` |
| `document_type` | Check constraint including `ETRN`, `UPD`, `POD`, and the other values in `000005` |
| `document_status` | Single column. Check constraint: `DRAFT`, `READY_FOR_SIGNING`, `SIGNING_IN_PROGRESS`, `SIGNED`, `SENT_TO_OPERATOR`, `ACCEPTED`, `REJECTED`, `ARCHIVED`, `CANCELLED` |
| `owner_company_id` | UUID NOT NULL. No foreign key |
| `related_entity_type`, `related_entity_id` | Optional correlation. Not a package membership store |
| `created_at`, `created_by`, `updated_at`, `updated_by` | `created_by` nullable |
| `deleted_at` | Soft-delete column. S1 treats a soft-deleted row as the same 404 as a missing row |
| `version` | Optimistic integer on the document row. This is not `document_versions.version_number` |

### `documents.document_versions` (migration `000005`)

| Column | Notes |
|--------|--------|
| `id` | UUID primary key. This is the revision identity |
| `document_id` | FK to `documents.documents(id)` `ON DELETE CASCADE` |
| `version_number` | Unique with `document_id` |
| `payload_json`, `payload_xml_path`, `pdf_file_path` | Revision payload pointers. No digest column |
| `created_at`, `created_by` | No `tenant_id` column. Tenant is reached through `document_id` |

There is no `UPDATE` statement for an existing version payload in `DocumentRepository`. There is also no trigger that rejects such an update.

### `documents.document_files` (migration `000005`)

| Column | Notes |
|--------|--------|
| `document_id` | FK `ON DELETE CASCADE` |
| `document_version_id` | Nullable FK to `documents.document_versions(id)` `ON DELETE SET NULL` |
| `checksum_sha256` | Optional file checksum. Not a signature binding |
| storage columns | `storage_provider`, `bucket_name`, `object_key`, file metadata |

`DocumentService.AddFile` checks tenant and `VersionBelongsToDocument`. It does not check `document_status`. A file can be attached after `SIGNED`. That gap stays an I2 service rule. I1 does not add an `INSERT` trigger on `document_files`.

### `documents.signing_sessions` (migration `000005`)

`tenant_id`, `document_id` FK `ON DELETE CASCADE`, status check, signer counts. No revision id.

### `documents.signatures` (migration `000005`)

| Column | Current binding |
|--------|-----------------|
| `tenant_id` | Present |
| `signing_session_id` | FK `ON DELETE CASCADE` |
| `document_id` | FK `ON DELETE CASCADE` |
| `signer_user_id`, `signer_company_id` | Present. Company id has no foreign key |
| `signature_type` | Application values: `SIMPLE_ELECTRONIC`, `ENHANCED_UNQUALIFIED`, `ENHANCED_QUALIFIED` |
| `signature_payload_path` | Path only. Not a private key |
| `certificate_fingerprint` | Optional identifier. Not a revision binding |
| `verification_status` | Mutable column. Check: `PENDING`, `VALID`, `INVALID`, `EXPIRED`, `REVOKED`, `FAILED`. `AddSignature` inserts `VALID` |
| revision | Absent |
| digest | Absent |
| private key | Absent |

```text
SIGNATURE_REVISION_BINDING_CURRENT=ABSENT
REVISION_ID_MODEL=documents.document_versions.id
```

`AddSignature` names its insert columns and does not reference a version. New nullable columns with no default rewrite do not require that insert to change. I1 still must not infer a version for rows this statement creates before I2.

### Delete and cascade behavior today

| Parent delete or update | Child effect |
|-------------------------|--------------|
| `documents.documents` hard delete | Cascades to versions, files, signing sessions, and signatures |
| `documents.document_versions` delete | `document_files.document_version_id` becomes NULL |
| `documents.signing_sessions` delete | Cascades to signatures |
| Signed payload update | Not blocked in the database |
| Soft delete | `deleted_at` update. S1 integration test sets it on an unsigned document |

```text
CURRENT_DELETE_CASCADE_BEHAVIOR=ON_DELETE_CASCADE_FROM_DOCUMENT_AND_SESSION
FILE_VERSION_FK=ON_DELETE_SET_NULL
```

### Tenant and company fields in current code

```text
TENANT_FIELD_SOURCE=documents.documents.tenant_id and documents.signing_sessions.tenant_id and documents.signatures.tenant_id
COMPANY_FIELD_SOURCE=documents.documents.owner_company_id and documents.signatures.signer_company_id
```

`SigningService.AddSignature` checks `UserExists` and `CompanyExists` inside the supplied tenant. That is existence, not authority to assemble a package or to sign. List and get do not filter on `owner_company_id`. Mutation handlers still accept `tenant_id` in JSON. S1 document and signing-session reads take `X-Tenant-ID` only after the gateway has set it. I1 adds no route, so it does not change either pattern.

### Idempotency already present

`documents.document_upload_intent` (migration `000033`) has `UNIQUE (tenant_id, driver_id, idempotency_key)`. Document create, version create, file attach, and signature insert have no idempotency key. `UNIQUE (tenant_id, document_number)` is not a retry contract.

## Migration inventory

Informational snapshot at `BASE_SHA`. I1 must count `infrastructure/migrations/*.up.sql` again immediately before adding a file.

```text
CURRENT_MAIN_MIGRATION_HEAD=000085_nlo_route_plan_bounded_planner_v0_4b
DOCUMENT_SCHEMA_LAST_MIGRATION=000033_add_document_upload_intent
NEXT_MIGRATION_NUMBER=TO_BE_DISCOVERED_AT_I1_START
MIGRATION_CREATED=NO
```

`000085` is not an EDO reservation. A later migration on `main` wins. If the head has moved, I1 uses the new next number and does not rename or edit `000085`.

## Decision table

| ID | Decision |
|----|----------|
| D001 | `VARIANT=A` |
| D002 | `ARCHIVE_MANIFEST=OUT` |
| D003 | `MCHD=OUT` |
| D004 | `STATE_SPLIT=OUT` |
| D005 | `OPERATOR_RUNTIME=OUT` |
| D006 | `ETRN_RUNTIME=OUT` |
| D007 | `BILLING_BRIDGE=OUT` |
| D008 | `FACTORING=OUT` |
| D009 | `DOCUMENTPACKAGE_MEMBERSHIP_SINGLE_SOURCE=YES` |
| D010 | `PACKAGE_CONTAINS_DOCUMENT_RELATIONSHIP=FORBIDDEN` |
| D011 | `SIGNATURE_BOUND_TO_REVISION=REQUIRED` for evidence written after I2. I1 adds the nullable binding and does not backfill |
| D012 | `LEGACY_AMBIGUOUS_BINDING=NO_INFERENCE` |
| D013 | `PRIVATE_KEYS_STORED=NO` |
| D014 | `I1_PUBLIC_API_CHANGE=NO` |
| D015 | `I1_PRODUCT_SERVICE_RULE_CHANGE=NO` |

D015 means I1 does not change Go handlers or service commands. Database constraints in this contract are schema, not an I2 command implementation. The I2 rules that block signed file attach and that seal or append through the service stay out of I1.

## Exact schema scope for I1

Target schema: `documents` only. Additive objects:

| Object | Role |
|--------|------|
| `documents.document_packages` | Package identity, tenant, assembling company, seal state |
| `documents.document_package_members` | The only membership store |
| `documents.document_relationships` | Append-only semantic edges |
| nullable columns on `documents.signatures` | Revision binding and content digest |
| `documents.certificate_evidence` | Fingerprint metadata for one bound signature and one revision |
| `documents.signature_verification_records` | Append-only verification history |
| unique `(id, tenant_id)` on existing tenant-owned document rows | Enables same-tenant composite foreign keys |
| triggers | Seal freeze, append-only evidence, signed-artifact delete and payload protection |

No other schema. No `PACKAGE_CONTAINS_DOCUMENT` type. No MChD table. No archive table. No billing, payment, shipment, transport-order, or network-optimizer table.

### Enforcement split

| Rule | I1 database | I2 service |
|------|-------------|------------|
| One membership row per document per package | Unique constraint | Command returns the existing member on idempotent replay |
| Same tenant for package and member | Composite foreign key | Trusted tenant predicate on read and write |
| `PACKAGE_CONTAINS_DOCUMENT` cannot be stored | Check constraint excludes it | Domain rejection before insert |
| Membership frozen after seal | Trigger rejects member insert, update, and delete when status is `SEALED` | Seal command is the only writer of seal columns |
| Relationship append-only | No `updated_at`. Trigger rejects update and delete | Append command only |
| Signed document hard delete | Trigger rejects delete when status is in the signed class, so existing `ON DELETE CASCADE` does not run | No delete command is added |
| Signed version payload update | Trigger rejects updates of payload columns | No payload-update command is added |
| Bound signature or certificate row delete | Trigger, plus `ON DELETE RESTRICT` on the new revision foreign key | No delete command is added |
| Signed file attach | Not in I1 | `AddFile` rejects the signed class |
| New signature names the revision and digest | Columns exist and partial rows are rejected by check constraint | `AddSignature` writes the binding. I1 does not change `AddSignature` |
| Re-verification history | Append-only verification table. Binding columns cannot be updated once set | Service appends a record and does not rewrite digest or fingerprint |
| Idempotent replay | Partial unique keys | Compare `request_fingerprint`, return the original row, or conflict |

Signed class for these triggers:

```text
SIGNED
SENT_TO_OPERATOR
ACCEPTED
ARCHIVED
```

`DRAFT`, `READY_FOR_SIGNING`, `SIGNING_IN_PROGRESS`, `REJECTED`, and `CANCELLED` keep today's delete and version-create behavior. `ValidateCreateVersionStatus` already blocks new versions for `SIGNED` and `ARCHIVED`. I1 does not retune that Go rule.

Do not replace `ON DELETE CASCADE` with a global `RESTRICT`. Draft and rejected fixtures must remain deletable. Do not change `document_files.document_version_id` from `ON DELETE SET NULL` in I1. Signed version delete is rejected by trigger before that set-null can detach a signed file. Unsigned version delete may still null the file pointer. That residual stays documented and is not an archive design.

Do not block `deleted_at` updates on unsigned documents. `TestDocumentReadTenantIsolation` sets `deleted_at` on an unsigned foreign document and requires the same 404 as a missing document. I1 must leave that statement valid. Setting `deleted_at` on a signed-class document is rejected by the same signed-class trigger, because hiding a signed registry row is not a retention implementation and is not a license to drop the bytes.

## DocumentPackage model

Table: `documents.document_packages`.

| Column | Rule |
|--------|------|
| `id` | UUID primary key |
| `tenant_id` | UUID NOT NULL |
| `assembling_company_id` | UUID NOT NULL. Attribute of the assembling company. Not an authorization proof |
| `status` | `OPEN` or `SEALED`. Default `OPEN` |
| `created_at` | NOT NULL, default `now()` |
| `created_by` | UUID NULL, same nullability as `documents.documents.created_by` |
| `sealed_at` | NULL while `OPEN`. NOT NULL when `SEALED` |
| `sealed_by` | UUID NULL |
| `idempotency_key` | VARCHAR(128) NULL |
| `request_fingerprint` | CHAR(64) NULL. Hex SHA-256 of the canonical create command. Not a content digest and not a secret |
| `seal_idempotency_key` | VARCHAR(128) NULL |
| `seal_request_fingerprint` | CHAR(64) NULL |
| `updated_at` | Maintained only for the seal transition |

Checks:

- `status IN ('OPEN','SEALED')`.
- `OPEN` implies `sealed_at IS NULL`.
- `SEALED` implies `sealed_at IS NOT NULL`.
- `request_fingerprint` is null or 64 lowercase hex characters. Same for `seal_request_fingerprint`.

Also declare `UNIQUE (id, tenant_id)`.

```text
TENANT_SCOPED=YES
ASSEMBLING_COMPANY_OWNER=STORED_NOT_AUTHORIZED
```

Company authorization that would decide who may seal is not knowable from the frozen architecture. See company ownership below.

## Package membership invariant

Table: `documents.document_package_members`.

| Column | Rule |
|--------|------|
| `id` | UUID primary key |
| `package_id` | NOT NULL |
| `document_id` | NOT NULL |
| `tenant_id` | NOT NULL |
| `created_at` | NOT NULL |
| `created_by` | UUID NULL |

```text
ONE_FACT_ONE_STORE=YES
MEMBERSHIP_SINGLE_SOURCE=document_package_members
DUPLICATE_MEMBER_PREVENTED=UNIQUE (package_id, document_id)
CROSS_TENANT_MEMBERSHIP_FORBIDDEN=YES
SEAL_FREEZES_MEMBERSHIP=YES
```

Foreign keys:

- `(package_id, tenant_id)` → `document_packages (id, tenant_id)` `ON DELETE RESTRICT`
- `(document_id, tenant_id)` → `documents.documents (id, tenant_id)` `ON DELETE RESTRICT`

`RESTRICT` stops a package delete from deleting documents, and stops a document delete from silently discarding membership. Unsigned document hard-delete that still uses `ON DELETE CASCADE` from `documents.documents` to versions is a different path. A member row pointing at a document blocks deleting that document until membership is removed. Removal is rejected after seal. An `OPEN` package may lose a member only through a future I2 command. I1 provides the table and the seal trigger, not the unseal or remove command.

There is no relationship type on this table. There is no second membership edge.

A trigger `BEFORE INSERT OR UPDATE OR DELETE` on `document_package_members` raises when the parent package is `SEALED`.

## DocumentRelationship model

Table: `documents.document_relationships`.

| Column | Rule |
|--------|------|
| `id` | UUID primary key |
| `tenant_id` | UUID NOT NULL |
| `source_document_id` | UUID NOT NULL |
| `target_document_id` | UUID NOT NULL |
| `relationship_type` | NOT NULL |
| `created_at` | NOT NULL |
| `created_by` | UUID NULL |
| `idempotency_key` | VARCHAR(128) NULL |
| `request_fingerprint` | CHAR(64) NULL |

```text
APPEND_ONLY=YES
TENANT_SCOPED=YES
CROSS_TENANT_FORBIDDEN=YES
PACKAGE_MEMBERSHIP_DUPLICATION_FORBIDDEN=YES
```

Allowed types, and no others:

```text
CORRECTS
REPLACES
RELATED_TO
```

`PACKAGE_CONTAINS_DOCUMENT` is absent from the check list, so the database rejects it. I2 also rejects it in the service. UKD XML, cancellation documents, and operator titles are not created here. ADR-EDO-002 still requires a new document plus a semantic relationship for those future acts. I1 only stores the edge.

Checks:

- `source_document_id <> target_document_id`
- type in the three values above
- fingerprint null or 64 lowercase hex characters

Foreign keys, both `ON DELETE RESTRICT`:

- `(source_document_id, tenant_id)` → `documents.documents (id, tenant_id)`
- `(target_document_id, tenant_id)` → `documents.documents (id, tenant_id)`

No `updated_at`. A trigger rejects `UPDATE` and `DELETE`. A second command with a new idempotency key and the same endpoints is allowed by the schema. I1 does not add uniqueness on `(source, target, type)`, because that would invent a cardinality rule the accepted taxonomy does not state. Replay identity is the idempotency key.

Actor evidence is `created_by`, matching `documents.documents.created_by`. No new `documents` audit or outbox table. Platform `core.audit_logs` stays an EDO-0.4+ item.

## Revision immutability model

```text
SIGNED_REVISION_MUTATION=FORBIDDEN_BY_I1_TRIGGER_FOR_PAYLOAD_COLUMNS
SIGNED_FILE_ATTACH=STILL_ALLOWED_UNTIL_I2
SIGNED_REVISION_DELETE=FORBIDDEN_BY_I1_TRIGGER
CASCADE_DELETE_POLICY=SIGNED_CLASS_DELETE_REJECTED_BEFORE_CASCADE
NEW_REVISION_SIGNATURE_INHERITANCE=NONE
```

I1 trigger function rejects:

- `DELETE` on `documents.documents` when `OLD.document_status` is in the signed class
- `UPDATE` on `documents.documents` that sets `deleted_at` when `OLD.document_status` is in the signed class
- `DELETE` on `document_versions`, `document_files`, or `signing_sessions` when the parent document is in the signed class
- `DELETE` on `document_versions` when any `signatures.document_version_id` equals that version
- `UPDATE` of `payload_json`, `payload_xml_path`, or `pdf_file_path` when the parent document is in the signed class, or when a signature row references that version
- `DELETE` on `signatures` when `document_version_id` is not null, or when the parent document is in the signed class
- `UPDATE` of `document_version_id`, `content_digest_algorithm`, `content_digest_value`, `certificate_fingerprint`, or `verification_status` when `OLD.document_version_id` is not null

Current `AddSignature` inserts `verification_status` and leaves `document_version_id` null. That insert stays valid. Legacy unbound rows may still have `verification_status` updated. Bound rows may not. Re-verification of a bound signature appends `signature_verification_records` and does not change the bound columns.

No trigger copies signature rows onto a newly inserted `document_versions` row.

`documents.documents.version` may still increment when status changes, including the existing transition into `SIGNED`. That integer is not the signed payload.

## Signature and revision binding

`document_version_id` is the concrete foreign key. It is not a new identifier.

Reason:

- `documents.document_versions.id` has been the revision primary key since migration `000005`.
- `documents.document_files.document_version_id` already references it.
- `CreateDocumentFileInput.DocumentVersionID` and `VersionBelongsToDocument` already use it.
- Discovery required `document_version_id` or an equivalent proof that names the revision and the content digest. The existing primary key is that revision name.
- The version row has no digest. `checksum_sha256` on a file is optional, per file, and can be detached by `ON DELETE SET NULL`. It is not the signed-bytes proof.

I1 adds to `documents.signatures`, all nullable, no backfill:

| Column | Rule |
|--------|------|
| `document_version_id` | FK to `documents.document_versions(id)` `ON DELETE RESTRICT` |
| `content_digest_algorithm` | NULL or `SHA-256` |
| `content_digest_value` | NULL or 64 lowercase hex characters |

Check: all three are null, or all three are not null. A partial binding is rejected. `SHA-256` is the only algorithm because it is the algorithm already named by `checksum_sha256` and by the hash-manifest wording in ADR-EDO-007. I1 does not add a free-text algorithm.

```text
REVISION_BINDING_MODEL=signatures.document_version_id -> document_versions.id plus digest columns on the signature
DIGEST_ALGORITHM_FIELD=content_digest_algorithm
DIGEST_VALUE_FIELD=content_digest_value
```

The digest is the digest of the signed revision bytes as supplied by the future I2 command. I1 does not hash stored objects and does not read `payload_json` to invent a digest.

Do not add `tenant_id` to `document_versions` in I1. Existing version rows have no tenant column, and copying it from the parent would be a backfill. Tenant equality for a signature is the composite foreign key `(document_id, tenant_id)` added below. Separately, a non-null revision must be a version of that same document. PostgreSQL needs a unique constraint on the referenced pair, and the primary key on `document_versions.id` does not satisfy a foreign key that cites `(document_id, id)`. Add:

```text
UNIQUE (document_id, id) ON documents.document_versions
FOREIGN KEY (document_id, document_version_id)
  REFERENCES documents.document_versions (document_id, id)
```

`MATCH SIMPLE` is the default: when `document_version_id` is null, this foreign key is not checked, so legacy unbound rows stay valid. When it is not null, the version row must carry the same `document_id`. Cross-document binding is rejected without inferring a version for old rows. I2 still calls `VersionBelongsToDocument` before insert. That service check is not an I1 Go change.

Also add, after the precheck, `UNIQUE (id, tenant_id)` on `documents.documents`, `documents.signing_sessions`, and `documents.signatures`, and composite foreign keys:

- `signatures (document_id, tenant_id)` → `documents (id, tenant_id)`
- `signatures (signing_session_id, tenant_id)` → `signing_sessions (id, tenant_id)`
- `signing_sessions (document_id, tenant_id)` → `documents (id, tenant_id)`

If the precheck finds `tenant_id` mismatches, the migration raises and stops. It does not `UPDATE` tenant ids.

## Certificate evidence

Table: `documents.certificate_evidence`.

Metadata is limited to fields the current signature row can already name. Current code stores `certificate_fingerprint` only. I1 does not add issuer, subject, serial, validity window, certificate body, or chain columns. Those fields are not in the document-service model. ADR-EDO-007's chain-validation metadata belongs to a later archive wave and is not collected here. Calling a certificate authority is out of variant A.

| Column | Rule |
|--------|------|
| `id` | UUID primary key |
| `tenant_id` | UUID NOT NULL |
| `signature_id` | UUID NOT NULL. Unique. One evidence row per signature |
| `document_version_id` | UUID NOT NULL |
| `certificate_fingerprint` | VARCHAR(255) NOT NULL |
| `created_at` | NOT NULL |
| `created_by` | UUID NULL |

Foreign keys `ON DELETE RESTRICT`:

- `(signature_id, tenant_id)` → `signatures (id, tenant_id)`
- `(signature_id, document_version_id)` → `signatures (id, document_version_id)`

The second key needs `UNIQUE (id, document_version_id)` on `signatures`. A certificate row can exist only when the signature already has a non-null revision binding. Legacy unbound signatures get no invented certificate row.

```text
CERTIFICATE_EVIDENCE_MODEL=fingerprint plus signature_id plus document_version_id
PRIVATE_KEY_COLUMN=ABSENT
REVERIFICATION_REWRITES_HISTORY=NO
LEGAL_VALIDITY_CLAIM=NO
LEGAL_VERIFICATION_REQUIRED
```

Forbidden column names on every new table and on the new signature columns: `private_key`, `secret`, `credential`, `token`, `pem`, `password`. The I1 test reads `information_schema.columns` for those names.

A new revision does not inherit the previous signature or the previous certificate row. There is no copy default and no copy trigger.

### Verification history

Table: `documents.signature_verification_records`.

| Column | Rule |
|--------|------|
| `id` | UUID primary key |
| `tenant_id` | UUID NOT NULL |
| `signature_id` | UUID NOT NULL |
| `document_version_id` | UUID NULL only when the signature binding is null |
| `verification_status` | Same check list as `signatures.verification_status` |
| `recorded_at` | NOT NULL |
| `recorded_by` | UUID NULL |

Append-only: trigger rejects `UPDATE` and `DELETE`. Insert does not update `certificate_evidence` or signature digest columns.

`verification_status=VALID` in either table means the application stored a status string. It does not mean the signature is legally valid.

## Tenant rules

```text
S1_REGRESSION_REQUIRED=YES
TRUSTED_TENANT_SOURCE=gateway JWT claim written to X-Tenant-ID
BODY_TENANT_AUTHORITATIVE=NO
CROSS_TENANT_PACKAGE=FORBIDDEN
CROSS_TENANT_RELATIONSHIP=FORBIDDEN
```

New aggregates store `tenant_id`. Composite foreign keys reject a member or a relationship whose document tenant differs. I1 creates no public command, so it does not read a body or query tenant. I2 and I3, when separately authorized, take tenant only from the trusted gateway context. A missing trusted tenant fails closed. A body or query `tenant_id` is not the authority. Foreign and missing package or relationship reads return one not-found result.

S1 concealment stays in force:

```text
TestDocumentReadTenantIsolation=PASS
foreign document GET=same 404 as missing
foreign signing session=same 404 as missing
missing trusted tenant=401
```

Unscoped `GetByID` and `GetSessionByID` remain unused by those HTTP reads. I1 does not switch reads back to them.

## Company ownership rule

```text
COMPANY_AUTHORIZATION_DETAIL=OPEN_FOR_I2
COMPANY_AUTHORIZATION_STATUS=OPEN_FOR_I2
```

| Concept | I1 treatment |
|---------|----------------|
| Tenant ownership | `tenant_id` on the new rows, enforced by composite foreign keys |
| Company ownership | `assembling_company_id` stored on the package. `owner_company_id` remains the document attribute. `signer_company_id` remains the signature attribute |

I1 does not add a foreign key to company tables. `owner_company_id` has none today. I1 does not require the member document's `owner_company_id` to equal `assembling_company_id`. The accepted rule is same tenant, not same company. I1 does not add a role matrix and does not change ADR-PLAT-001 membership writes.

I2 must not pretend the stored company id is permission to seal, append, or sign. Until a reviewed company rule exists, commands may persist the identifier and must not describe it as authorization.

## Idempotency design

```text
IDEMPOTENCY_DEFINED=YES
IDEMPOTENCY_IMPLEMENTED=NO
```

I1 stores the minimum keys. I2 implements replay. No shared idempotency framework and no change to `document_upload_intent`.

| Operation | Key scope | Database uniqueness | Replay | Payload conflict |
|-----------|-----------|---------------------|--------|------------------|
| Package create | tenant + `idempotency_key` on `document_packages` | partial unique `(tenant_id, idempotency_key)` where the key is not null | Same fingerprint returns the existing package | Different fingerprint raises a conflict and does not insert a second package |
| Package seal | tenant + `seal_idempotency_key` on the same package row | partial unique `(tenant_id, seal_idempotency_key)` where the key is not null | Same key returns the sealed package and does not change `sealed_at` | A different key against an already `SEALED` package does not unseal, does not rewrite `sealed_at`, and does not replace `seal_idempotency_key` |
| Relationship append | tenant + `idempotency_key` on `document_relationships` | partial unique `(tenant_id, idempotency_key)` where the key is not null | Same fingerprint returns the existing edge | Different fingerprint conflicts |

Operation scope is the table, not a global operation enum. Keys are not unique across package create and relationship append. A null key does not deduplicate. I2 may require a key on those three commands. I1 does not reject null keys, because no command exists yet and existing document routes are untouched.

`request_fingerprint` covers the canonical command fields that define identity of the attempt: tenant, assembling company, and the member document ids for create; package id for seal; source, target, and type for append. It is not a document content digest.

## Migration strategy

```text
TARGET_SCHEMA=documents
ADDITIVE_ONLY=YES
UP_MIGRATION_REQUIRED=YES
DOWN_OR_IRREVERSIBLE_DECISION_REQUIRED=YES
UP_DOWN_UP_REQUIRED=YES
ZERO_DESTRUCTIVE_REWRITE=YES
```

Up:

1. Run the cross-tenant precheck. Raise on any mismatch. Do not update rows.
2. Add `UNIQUE (id, tenant_id)` on `documents.documents`, `signing_sessions`, and `signatures`.
3. Add `UNIQUE (document_id, id)` on `document_versions`.
4. Add composite foreign keys listed above.
5. Add nullable signature binding columns and the all-or-nothing check.
6. Create the five new tables, checks, partial unique indexes, and foreign keys.
7. Create the triggers in this contract.

Down:

- If any `signatures.document_version_id` is not null, or any new table is non-empty, down raises and stops.
- If those objects are empty, down drops triggers, new tables, new signature columns, new foreign keys, and the unique constraints added in this migration.
- Down does not drop `documents.documents`, versions, files, sessions, signatures, or upload intents.
- Down does not rewrite legacy signature rows.

Once I2 has written a binding, down is refused. That is the irreversible decision. Empty up/down/up remains required before I1 acceptance, using a database that has no bound signatures and no package rows.

Rollback of a deployed I1 that already holds package or evidence rows is restore-from-backup, not this down script. I1 does not take a backup and does not touch staging.

Existing statements that list columns explicitly, including `AddSignature` and `CreateVersion`, stay valid because new signature columns are nullable and new tables are unread by current Go.

## Legacy-data strategy

```text
AMBIGUOUS_BACKFILL=FAIL_CLOSED
NO_INFERRED_SIGNATURE_REVISION_BINDING=YES
LEGACY_SIGNATURE_BINDING_INFERENCE=NO
```

- Existing document rows stay readable.
- Existing unsigned and signed documents are not rewritten.
- Existing signature rows stay unbound: `document_version_id`, `content_digest_algorithm`, and `content_digest_value` remain null.
- The migration must not contain `UPDATE documents.signatures SET document_version_id`, including any assignment to the latest `document_versions` row.
- No certificate evidence row is inserted for a legacy signature.
- Unbound historical rows remain legacy until a later wave has authoritative evidence that names the exact signed revision and digest. This contract does not define that later wave.
- Absence of a binding is not recorded as `INVALID` and is not recorded as legally valid.

## Rollback strategy

See migration down. Application rollback without the down script leaves the new empty tables unused. Current document commands do not read them. If the up migration fails mid-way, the migration tool's transaction or dirty-state handling applies. I1 must not mark the schema version applied when the precheck raises.

## Security gates

- Tenant composite foreign keys on every new edge.
- No client-selected tenant authority on future commands.
- S1 regression floor listed below.
- No private key, certificate body, operator credential, or token column.
- Logs and fixtures in I1 tests must not contain document bytes, signature bytes, certificate bodies, or secrets.
- `verification_status` and `certificate_fingerprint` are evidence fields, not a legal-validity claim.
- `LEGAL_VERIFICATION_REQUIRED` stays on statutory format, qualified signature effect, MChD, and retention.

## Testing requirements

I1 acceptance requires executed tests, not schema review alone. W1 does not run them.

Harness: real PostgreSQL, following the `//go:build integration` embedded-Postgres pattern in `services/document-service/internal/integration/podupload/test_database.go`. New tests live in a new package and do not rewrite POD tests.

| Required I1 check | Proof |
|-------------------|--------|
| Migration up | Next discovered migration applies on current head |
| Migration down | Empty new objects drop. Non-empty binding or package data makes down raise |
| Up-down-up | Second up succeeds after an empty down |
| Package uniqueness | Second member row for the same package and document fails |
| Same-tenant membership | Matching tenant insert succeeds |
| Cross-tenant membership | Composite foreign key rejects it |
| Relationship invariants | Both ends in-tenant succeed. Cross-tenant fails. Source equal to target fails |
| `PACKAGE_CONTAINS_DOCUMENT` | Check constraint rejects the insert |
| Signature revision binding | Null triple remains valid. Partial triple fails. Version from another document fails. Digest algorithm other than `SHA-256` fails |
| Ambiguous legacy backfill | After up, pre-existing signature rows still have null `document_version_id`. Migration source contains no inference update |
| Signed artifact delete and cascade | Delete of a signed-class document, its version, its file, its session, and its signature fails. Draft document delete still cascades. Unsigned `deleted_at` update still succeeds |
| Payload immutability | Update of signed version payload columns fails |
| New revision inheritance | Inserting version 2 does not create a signature or certificate row for version 2 |
| Certificate evidence | No private-key-like column. Evidence insert without a bound signature fails. Update and delete of evidence fail |
| Re-verification | Insert of a second verification record succeeds. Digest and fingerprint on the signature stay unchanged |
| S1 regression | `TestDocumentReadTenantIsolation` PASS with the behaviors below |
| Existing document regression | The six tests in the regression floor PASS |

S1 floor:

```text
TestDocumentReadTenantIsolation=PASS
foreign document GET=same 404 as missing
foreign signing session=same 404 as missing
missing trusted tenant=401
```

Existing regression floor, from [edo-0.3-test-acceptance-strategy.md](edo-0.3-test-acceptance-strategy.md):

```text
TestDocumentServiceCreateVersionOnlyDraftOrRejected
TestDocumentServiceCancelSignedDocument
TestDocumentServiceArchiveOnlySignedOrAccepted
TestSigningServiceAddSignatureCompletesDocument
TestPODUploadIntentIdempotency
TestPODCrossTenantShipmentRejected
```

I1 does not claim these passed during W1. Result for W1 is `NOT_RUN`.

I2, not I1, adds the service tests for seal, relationship command replay, and `AddFile` after `SIGNED`.

## Out of scope

```text
ArchiveManifest
legal hold
retention enforcement
Selectel bucket creation
S3 migration
WORM
Object Lock
LEGAL_ARCHIVE_READY=YES
MChD registry
MChD validity verification
power-of-attorney government integration
state-dimension split
e-TrN lifecycle
GIS EPD
transport-edo-service
operator adapter
operator credentials
transport titles
billing-register-service runtime
closing documents runtime
UPD operator state
invoice flow
billing.upd_documents.document_id changes
receivable
assignment
factoring request
financing offer
payment obligation changes
shipment-service schema
transport-order-service schema
TransportJourney
TransportLeg
CargoHandover
TMS execution
driver
Control Tower runtime
NLO
BNO
Kafka topics
new producers
outbox runtime
OpenAPI
API gateway
public routes
frontend
```

```text
OPENAPI_CHANGE_IN_I1=NO
PUBLIC_ROUTE_CHANGE_IN_I1=NO
GATEWAY_CHANGE_IN_I1=NO
```

I3 remains responsible for gateway exposure and OpenAPI parity if a later controller authorizes public commands. I4 remains the verification wave. This document does not satisfy I4.

## Events

I1 publishes nothing. Existing proposed names in [edo-0.2-event-contracts.md](../events/edo-0.2-event-contracts.md) stay governed by ADR-EDO-006. `edo.package.created`, `edo.package.member_added`, `edo.package.sealed`, and `edo.package.exchange_completed` are proposed catalog rows. EDO 0.3 discovery did not accept them as an implementation authorization. `edo.package.exchange_completed` is outside I1 and I2.

```text
FOLLOW_UP_REQUIRED
```

Relationship append has no accepted event name. A name must be reviewed before any I2 or I3 producer exists. I1 does not add that name to the catalog.

## Future I2 boundary

Not in this authorization. When a separate contract allows it:

```text
seal package command
append relationship command
block signed mutation
block signed file attachment
trusted tenant predicate on every new read and write
idempotent replay behavior specified above
```

## Future I3 boundary

```text
gateway exposure if required
OpenAPI source-of-truth
generated artifact parity
trusted tenant boundary
```

## Future I4 boundary

Dedicated verification and acceptance. W1 documentation is not I4 evidence.

## Future I1 allowed paths

Inspected locations. I1 may change only:

```text
infrastructure/migrations/<rediscovered_number>_edo_0_3_i1_schema_foundation.up.sql
infrastructure/migrations/<rediscovered_number>_edo_0_3_i1_schema_foundation.down.sql
services/document-service/internal/integration/edo03schema/**
docs/architecture/edo-0.3-w1-i1-task-contract.md
docs/architecture/edo-0.3-implementation-waves.md
docs/program/workstream-status-v0.1.md
```

Go service, domain, repository, and handler files are not required. Current inserts name their columns, and the new signature columns are nullable. If an existing insert or the S1 soft-delete statement fails because a trigger or constraint is mis-specified, the fix is to correct the migration so current unsigned behavior survives. That is not permission to add package commands in Go.

Domain and repository paths stay closed in I1 unless the controller expands this list. Package and relationship commands belong to I2.

## Future I1 forbidden paths

```text
services/document-service/internal/http/**
services/document-service/internal/service/**
services/document-service/internal/domain/**
services/document-service/internal/repository/**
services/api-gateway/**
services/shipment-service/**
services/transport-order-service/**
services/billing-register-service/**
services/payment-service/**
apps/**
packages/openapi/**
packages/**
operator integrations
infrastructure provisioning outside the single pair of documents-schema migration files
```

`services/billing-*/**` is covered by `billing-register-service` and any sibling billing runtime. None of it is an I1 path.

## Cross-workstream stop conditions

If I1 implementation requires any of the following, stop. Do not improvise a local substitute.

```text
STOP
CROSS_WORKSTREAM_REQUEST_REQUIRED
```

Triggers:

```text
operator credential
GIS EPD call
shipment-service mutation
transport-order-service mutation
billing status mutation
Selectel bucket
```

Use [cross-workstream-request-template.md](../program/cross-workstream-request-template.md). ADR-EDO-009 stays in force.

## Risk review

| Risk | Impact | Mitigation | I1 gate | Follow-up |
|------|--------|------------|---------|-----------|
| `RISK_SIGNATURE_LEGACY_BINDING` | A backfill onto the latest version would claim a revision was signed when the schema never recorded that fact | `NO_INFERRED_SIGNATURE_REVISION_BINDING`. Nullable triple. Down refuses once any binding exists | Migration test proves old rows stay null and the SQL has no inference update | Later evidence wave only with an authoritative digest source. Owner: EDO. Not I1 |
| `RISK_CASCADE_DELETE` | Today's `ON DELETE CASCADE` and session cascade can destroy signed signatures. File FK `ON DELETE SET NULL` can detach a version | Signed-class delete trigger fires before cascade. Bound signature FK is `ON DELETE RESTRICT`. Unsigned cascade stays | Integration test: signed delete fails, draft delete still cascades | I2 still blocks signed `AddFile`. Owner: EDO I1 for delete, EDO I2 for attach |
| `RISK_CROSS_TENANT_MEMBERSHIP` | A package or relationship could point at another tenant's document | Composite foreign keys. Precheck raises instead of repairing old rows | Cross-tenant insert fails. Precheck failure aborts up | I2 trusted-tenant predicate. Owner: EDO |
| `RISK_COMPANY_AUTHORIZATION_UNDERSPECIFIED` | Storing `assembling_company_id` can be mistaken for permission | `COMPANY_AUTHORIZATION_DETAIL=OPEN_FOR_I2`. No new RBAC. Company id is an attribute | Schema has no role table and no claim text that the company is authorized | I2 contract must state the rule or keep the gap open. Owner: EDO with PLAT if membership writes are required |
| `RISK_IDEMPOTENCY_SCHEMA_OVERDESIGN` | A global idempotency service would widen I1 past variant A | Three partial unique keys on the new rows only. No change to POD intents | Migration contains no new schema outside `documents` package, relationship, and evidence objects | I2 implements replay. Owner: EDO |
| `RISK_MIGRATION_COLLISION_AT_IMPLEMENTATION_TIME` | `000085` may no longer be the head when I1 starts | Number is not reserved. I1 recounts `infrastructure/migrations` immediately before the file is added | Review checks the chosen number is the next free number on that branch's `main` base | Implementer. Owner: EDO I1 |
| `RISK_FALSE_LEGAL_VALIDITY_SIGNAL` | `verification_status=VALID` or a fingerprint can be read as legal effect | No private key, no certificate body, no MChD table, no archive-ready claim. Contract text keeps `LEGAL_VERIFICATION_REQUIRED` | Column-name test. Docs and fixtures do not say BINTRANS is an operator or that evidence is legally valid | Legal review stays open. Owner: Controller / legal, not this wave |

## Legal language

```text
LEGAL_VERIFICATION_REQUIRED
LEGAL_VERIFICATION_STATUS=OPEN
```

This contract does not state that BINTRANS is an EDI operator, that BINTRANS is a GIS EPD operator, that stored signature evidence proves legal validity, that the archive is legally compliant, or that any retention term is legally sufficient. Those sentences remain unverified. W1 does not perform that verification.

## W1 allowed paths

This W1 change is docs only:

```text
docs/architecture/edo-0.3-w1-i1-task-contract.md
docs/architecture/edo-0.3-implementation-waves.md
docs/program/workstream-status-v0.1.md
```

## W1 forbidden paths

```text
services/**
apps/**
packages/**
infrastructure/**
scripts/migrations/**
OpenAPI artifacts
CI workflows
```

```text
PRODUCT_CODE_CHANGED=NO
SCHEMA_CHANGED=NO
OPENAPI_CHANGED=NO
CI_CHANGED=NO
```

## Acceptance criteria

W1 was ready for Controller review when:

1. This document contains the sections listed in the W1 assignment, including the decision table and the risk review.
2. I1 had not started.
3. The diff is docs only.
4. `git diff --check` is clean.
5. Links in this document resolve to repository files.
6. The docs contain no secret values.

Controller decision `ACCEPT_EDO_0_3_W1_I1_TASK_CONTRACT` records `IMPLEMENTATION_AUTHORIZED=I1_ONLY`. I1 remains `AUTHORIZED_NOT_STARTED`. This closeout does not claim the I1 migration or tests exist.

## Controller authorization

Recorded from the Controller authorization for EDO-0.3 I1. This closeout does not implement I1.

```text
CONTROLLER_DECISION=ACCEPT_EDO_0_3_W1_I1_TASK_CONTRACT
CONTROLLER_REVIEW_REF=PR187
IMPLEMENTATION_AUTHORIZED=I1_ONLY
AUTHORIZED_WAVE=I1
W1_STATUS=IMPLEMENTATION_CONTRACT_ACCEPTED
I1_STATUS=AUTHORIZED_NOT_STARTED
I2_STATUS=NOT_AUTHORIZED
I3_STATUS=NOT_AUTHORIZED
I4_STATUS=NOT_AUTHORIZED
LEGAL_VERIFICATION_STATUS=OPEN
LEGAL_VERIFICATION_REQUIRED
```

## References

- [edo-0.3-scope.md](edo-0.3-scope.md)
- [edo-0.3-architecture-options.md](edo-0.3-architecture-options.md)
- [edo-0.3-implementation-waves.md](edo-0.3-implementation-waves.md)
- [edo-0.3-test-acceptance-strategy.md](edo-0.3-test-acceptance-strategy.md)
- [edo-0.3-security-tenant-boundary.md](edo-0.3-security-tenant-boundary.md)
- [edo-0.3-gap-analysis.md](edo-0.3-gap-analysis.md)
- [edo-0.3-current-state-inventory.md](edo-0.3-current-state-inventory.md)
- [edo-0.3-s1-document-read-tenant-isolation.md](edo-0.3-s1-document-read-tenant-isolation.md)
- [ADR-EDO-001](../adr/ADR-EDO-001-canonical-edo-document-ownership.md)
- [ADR-EDO-002](../adr/ADR-EDO-002-billing-edo-boundary.md)
- [ADR-EDO-006](../adr/ADR-EDO-006-event-naming-versioning.md)
- [ADR-EDO-007](../adr/ADR-EDO-007-legal-archive-boundary.md)
- [ADR-EDO-008](../adr/ADR-EDO-008-external-operator-first-own-operator-ready.md)
- [ADR-EDO-009](../adr/ADR-EDO-009-cross-workstream-mutation-policy.md)
- [edo-0.2-event-contracts.md](../events/edo-0.2-event-contracts.md)
- [cross-workstream-request-template.md](../program/cross-workstream-request-template.md)
