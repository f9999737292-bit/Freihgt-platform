# EDO 0.3 Discovery

## Status

```text
EDO_0_2_STATUS=IMPLEMENTED_ACCEPTED
EDO_0_3_DISCOVERY_STATUS=DISCOVERY_ACCEPTED
EDO_0_3_IMPLEMENTATION_AUTHORIZED=NO
DOCUMENT_READ_TENANT_ISOLATION_STATUS=PRODUCT_REMEDIATION_REQUIRED
LEGAL_VERIFICATION_STATUS=OPEN
PRODUCT_CODE_MODIFIED=NO
```

Discovery date: 2026-09-23.

Base: `origin/main` `c38f2d13f08fffe2018d631881ff93b3ad2479f2`.

Branch: `discovery/edo-0.3-v0.1`.

Worktree: `D:\Projects\freight-platform-wt\edo-0.3-discovery-v0.1`.

The previous worktree `D:\Projects\freight-platform-wt\edo-ecosystem-architecture-v0.1` is an archive on a detached commit. It was not modified.

## What EDO 0.3 is

Accepted name, taken from the EDO 0.2 final report and the program roadmap, not from memory:

```text
EDO-0.3 — Document domain extensions (document-service only)
```

Named deliverables: `DocumentPackage`, `DocumentRelationship`, immutable revision rules, signing evidence extensions, additive `documents` schema only, and a separate Task Contract before any code.

Recommended reading of the conflicting "EDO-0.3+" archive sentence: variant A. `ArchiveManifest`, MChD verification, operator exchange, and the four-way state split stay out until a controller says otherwise.

Docs remediation of controller findings F-01…F-04:

- Get-by-id tenant isolation is not described as complete. `DOCUMENT_READ_TENANT_ISOLATION_REMEDIATION_REQUIRED` is a separate product wave.
- Package membership has one source of truth on `DocumentPackage`. Relationships do not duplicate it.
- Future signing evidence binds to the signed revision. Today `documents.signatures.document_id` is implemented and revision binding is absent.
- Waves I1–I4 name Task Contract gates and stay `NOT_AUTHORIZED`.

```text
CURRENT_PRODUCT_SECURITY_GAP:
GET /v1/documents/{id} does not enforce tenant predicate at repository read path.

ACTION:
Separate security remediation required.

NOT_IN_THIS_PR:
No product fix, migration, API change, or test implementation.
```

```text
RECOMMENDED_VARIANT=A
CONTROLLER_VERDICT=ACCEPT_EDO_0_3_SCOPE
ACCEPTED_DISCOVERY_HEAD=5b0d55b7c44eef4d9da14a2add3f7e965e7b02ec
ACCEPTED_CI=35912686104
PR=https://github.com/f9999737292-bit/Freihgt-platform/pull/162
VARIANT_A_SCOPE=ACCEPTED
S1_STATUS=NOT_AUTHORIZED
S1_IS_IMPLEMENTATION_PREREQUISITE=YES
I1_I4_STATUS=NOT_AUTHORIZED
```

Variant A is the accepted discovery scope. ArchiveManifest, MChD verification, an EDI operator integration, and e-ТрН stay out of scope. S1 remains the required product-security prerequisite and is not started. I1–I4 stay `NOT_AUTHORIZED`. Legal verification stays `OPEN`. The get-by-id tenant predicate is still absent.

## Document index

| Document | Role |
|----------|------|
| [edo-0.3-current-state-inventory.md](edo-0.3-current-state-inventory.md) | Twenty-point audit of code versus ADR |
| [edo-0.3-gap-analysis.md](edo-0.3-gap-analysis.md) | Gaps and the ArchiveManifest ambiguity |
| [edo-0.3-scope.md](edo-0.3-scope.md) | Proposed scope and out-of-scope |
| [edo-0.3-architecture-options.md](edo-0.3-architecture-options.md) | Variants A–D and the recommendation |
| [edo-0.3-security-tenant-boundary.md](edo-0.3-security-tenant-boundary.md) | Tenant, company, actors, keys |
| [edo-0.3-regulatory-source-register.md](edo-0.3-regulatory-source-register.md) | Official sources and `LEGAL_VERIFICATION_REQUIRED` |
| [edo-0.3-implementation-waves.md](edo-0.3-implementation-waves.md) | Review waves. Implementation not authorized |
| [edo-0.3-test-acceptance-strategy.md](edo-0.3-test-acceptance-strategy.md) | Future tests. None added here |
| [edo-0.3-risk-register.md](edo-0.3-risk-register.md) | Open risks |
| [workstream-status-v0.1.md](../program/workstream-status-v0.1.md) | EDO row updated to discovery complete |

## Accepted baseline this discovery does not reopen

- [edo-0.2-final-report.md](edo-0.2-final-report.md)
- [ADR-EDO-001](../adr/ADR-EDO-001-canonical-edo-document-ownership.md) through [ADR-EDO-009](../adr/ADR-EDO-009-cross-workstream-mutation-policy.md)
- [ADR-PLAT-001](../adr/ADR-PLAT-001-membership-user-roles-canonical-writer.md)
- [edo-0.2-event-contracts.md](../events/edo-0.2-event-contracts.md)
- [edo-0.2-event-versioning-policy.md](../events/edo-0.2-event-versioning-policy.md)
- [event-catalog-v0.1.md](../events/event-catalog-v0.1.md)

EDO 0.2 was merged in pull request #73 (`18a85074` on `main`).

## Controller review

Repeat review accepted this discovery scope. Evidence: `CONTROLLER_VERDICT=ACCEPT_EDO_0_3_SCOPE`, discovery head `5b0d55b7c44eef4d9da14a2add3f7e965e7b02ec`, CI run `35912686104`, pull request #162. That verdict does not authorize S1 or I1–I4. The next product step, after this discovery is merged, is a separate authorization of S1 for the document read tenant predicate. The gap is not fixed here.

## References

- [docs/adr/README.md](../adr/README.md)
- [edo-0.2-domain-model-freeze.md](edo-0.2-domain-model-freeze.md)
- [edo-0.2-archive-boundary.md](edo-0.2-archive-boundary.md)
