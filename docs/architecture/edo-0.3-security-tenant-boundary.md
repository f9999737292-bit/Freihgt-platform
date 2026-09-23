# EDO 0.3 Security and Tenant Boundary

## Status

```text
DOCUMENT_STATUS=DISCOVERY
IMPLEMENTATION_AUTHORIZED=NO
SECURITY_REVIEW_OF_CODE=NOT_PERFORMED
```

This document records constraints for a later implementation. It does not change auth, RBAC, or tenant code.

## Trust boundary

External clients authenticate at `api-gateway`. Trusted downstream identity is the gateway-established tenant and user, not a body field supplied by the client.

Observed today:

| Path | Tenant source | Risk for a later EDO 0.3 route |
|------|---------------|--------------------------------|
| `POST/GET /v1/documents` and status transitions | JSON or query `tenant_id` in `document_handler.go` | A new package or relationship route must not copy this pattern |
| `POST /internal/v1/pod-uploads` | `X-Tenant-ID` | Header is trusted only when the gateway set it |
| `SigningService.GetSession` | Session id only | A read of package or relationship by id must include tenant |
| Signing add | Tenant plus user and company existence in that tenant | Existence is not authority to sign |

Gateway mounts document proxies behind human JWT support and integration JWT support (`services/api-gateway/internal/http/router.go`, `proxy.go`). New public routes, if ever added, stay on that chain. No unauthenticated operator callback is in scope.

## Tenant isolation

Every `DocumentPackage` and `DocumentRelationship` row needs `tenant_id` or a parent that is loaded with `tenant_id`. Queries use the trusted tenant predicate. A missing row and a foreign-tenant row return the same not-found result.

Package membership cannot include a document from another tenant.

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
- Certificate evidence may store fingerprint and a verification snapshot. It must not store a private key.
- Signature payload paths point at stored artifacts. Logs must not include payload bytes, certificate bodies, or tokens.
- Qualified-signature legal effect and MChD checks are `LEGAL_VERIFICATION_REQUIRED`. Variant A does not implement them.

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
