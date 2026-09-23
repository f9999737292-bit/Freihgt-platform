# EDO 0.3 Security and Tenant Boundary

## Status

```text
DOCUMENT_STATUS=DISCOVERY
IMPLEMENTATION_AUTHORIZED=NO
SECURITY_REVIEW_OF_CODE=NOT_PERFORMED
```

This document records constraints for a later implementation. It does not change auth, RBAC, or tenant code.

## Current product security gap

Severity of this gap stays high. This section does not close it and does not hide it inside the later risk table.

```text
CURRENT_PRODUCT_SECURITY_GAP:
GET /v1/documents/{id} does not enforce tenant predicate at repository read path.

ACTION:
Separate security remediation required.

NOT_IN_THIS_PR:
No product fix, migration, API change, or test implementation.
```

```text
List and mutations:
GetByIDAndTenant / tenant predicate = IMPLEMENTED

GET /v1/documents/{id}:
DocumentHandler.GetByID → service.GetDetail → repository.GetByID
repository predicate = id + deleted_at only
tenant predicate = ABSENT

GetSession:
tenant predicate = ABSENT
```

The service method on that path is `DocumentService.GetByID`. It calls repository `GetDetail`, which calls repository `GetByID`.

```text
DOCUMENT_READ_TENANT_ISOLATION_REMEDIATION_REQUIRED
```

That remediation is its own product wave and its own controller review. EDO 0.3 implementation waves do not start until that review accepts the product fix. This pull request does not perform the fix.

Current read isolation is not complete and is not fail-closed.

## Trust boundary

External clients authenticate at `api-gateway`. Trusted downstream identity is the gateway-established tenant and user, not a body field supplied by the client.

Observed today:

| Path | Tenant source | What is true today |
|------|---------------|-------------------|
| List and mutations that call `GetByIDAndTenant` | JSON or query `tenant_id` in `document_handler.go` | Tenant predicate is implemented. The value is still caller-supplied, not the gateway trust boundary. A new route must not copy that pattern. |
| `GET /v1/documents/{id}` | No tenant argument | `DocumentHandler.GetByID` → `GetDetail` → `GetByID`. Predicate is `id` and `deleted_at` only. Tenant predicate is absent. |
| `POST /internal/v1/pod-uploads` | `X-Tenant-ID` | Header is trusted only when the gateway set it |
| `SigningService.GetSession` | Session id only | Tenant predicate is absent. A later read of a package or relationship by id must include tenant. |
| Signing add | Tenant plus user and company existence in that tenant | Existence is not authority to sign |

Gateway mounts document proxies behind human JWT support and integration JWT support (`services/api-gateway/internal/http/router.go`, `proxy.go`). New public routes, if ever added, stay on that chain. No unauthenticated operator callback is in scope.

## Tenant isolation

Every future `DocumentPackage` and `DocumentRelationship` row needs `tenant_id`, and every read of those rows must use the trusted tenant predicate. A missing row and a foreign-tenant row return the same not-found result. That rule is not true of today's document get-by-id path.

Package membership is stored only on `DocumentPackage`. A document and its package must share one tenant. Cross-tenant membership is forbidden. Membership uniqueness is one document once per package inside that tenant. `DocumentRelationship` must not repeat that fact.

## Company isolation

`owner_company_id` and `signer_company_id` exist. List and get paths observed in `document_repository.go` do not filter by company.

EDO 0.3 discovery does not choose a new role matrix. A later wave must state which company memberships may seal a package, append a relationship, or attach signature evidence. Until that statement is reviewed, company id on the row is an attribute, not proof of authorization.

ADR-PLAT-001 still names `company-service` as the writer of `core.company_memberships` and `identity-service` as the writer of `core.user_roles`. EDO 0.3 does not repair the dual-write.

## Actor classes

| Actor | Allowed use in a future EDO 0.3 API | Forbidden |
|-------|--------------------------------------|-----------|
| Human user with gateway JWT | Operate on documents in the trusted tenant, subject to a reviewed company rule | Present a different tenant in the body |
| Integration principal | Same tenant rule, machine credential at the gateway | Shared human passwords, long-lived tokens in the repository |
| document-service | Enforce tenant predicates and immutability | Mint identity or accept raw client tenant as authority |
| transport-edo-service | Does not exist. Not a signer of EDO 0.3 scope | Embedding operator credentials in document-service |

## Signature and key material

- Private keys and HSM material stay out of `document-service` and out of the archive tier (ADR-EDO-007).
- Today `documents.signatures.document_id` is implemented and revision binding is absent. No `document_version_id` column is claimed.
- A future signature and its certificate evidence bind to the immutable revision of the signed bytes (`document_version_id` or an equivalent proof that includes the revision and the content digest). Digest algorithm and digest value refer to that revision.
- Re-verification does not rewrite historical evidence. A new revision does not inherit the previous signature.
- Certificate evidence must not store a private key, a token, or extra personal data beyond the reference needed to identify the certificate.
- Logs must not include document bytes, signature bytes, certificate bodies, JWT, or other secrets.
- Evidence does not mean the signature is legally valid. Qualified-signature effect and MChD checks stay `LEGAL_VERIFICATION_REQUIRED`. Variant A does not implement them.

## Immutability as a security control

Signed revision bytes, signature rows, and certificate evidence rows are append-only. `ON DELETE CASCADE` from `documents.documents` to versions, files, and signatures contradicts that rule for signed artifacts. A later wave replaces cascade-on-signed-children with a retain rule. Soft delete of a signed document is not a substitute for retention.

Idempotency keys for package seal and relationship append must be tenant-scoped so a replay in one tenant cannot apply in another.

## Personal data and secrets

Payload JSON and files may contain personal data. Processing is constrained by Federal Law No. 152-FZ. The consolidated duty (legal basis, retention, cross-border transfer) is `LEGAL_VERIFICATION_REQUIRED`. See [edo-0.3-regulatory-source-register.md](edo-0.3-regulatory-source-register.md).

This discovery does not add `.env` files, credentials, or connection strings. Logging of document bodies is out of policy for later waves.

## Selectel

Staging placement on Selectel does not verify encryption at rest, object lock, or residency. Those remain `EXTERNAL_INFRA_VERIFICATION_REQUIRED`. EDO 0.3 does not open storage firewall rules or create buckets.

## Acceptance constraints for any later wave

- Tests must show a foreign tenant cannot read or append to a package.
- Tests must show a client-supplied tenant header or body that disagrees with the gateway tenant is rejected.
- Tests must show a signed revision cannot gain a new file or a changed payload.
- Tests must show signature logs and error bodies omit payload bytes and secrets.

No such tests are added in this discovery.

## References

- [ADR-EDO-001](../adr/ADR-EDO-001-canonical-edo-document-ownership.md)
- [ADR-EDO-007](../adr/ADR-EDO-007-legal-archive-boundary.md)
- [ADR-EDO-009](../adr/ADR-EDO-009-cross-workstream-mutation-policy.md)
- [ADR-PLAT-001](../adr/ADR-PLAT-001-membership-user-roles-canonical-writer.md)
- [edo-0.3-scope.md](edo-0.3-scope.md)
