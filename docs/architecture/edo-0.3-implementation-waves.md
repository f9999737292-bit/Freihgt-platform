# EDO 0.3 Implementation Waves

## Status

```text
DOCUMENT_STATUS=DISCOVERY
EDO_0_3_S1_STATUS=IMPLEMENTED_AWAITING_CONTROLLER_REVIEW
DOCUMENT_READ_TENANT_ISOLATION_STATUS=REMEDIATED_AWAITING_CONTROLLER_REVIEW
EDO_0_3_I1_I4_STATUS=NOT_AUTHORIZED
LEGAL_VERIFICATION_STATUS=OPEN
EACH_WAVE_REQUIRES=INDEPENDENT_CONTROLLER_REVIEW
```

Waves are a review sequence. Completing discovery does not start wave I1. S1 is the only product change on this branch. I1–I4 do not change product code, migrations, OpenAPI, or CI.

Accepted discovery scope is variant A from [edo-0.3-architecture-options.md](edo-0.3-architecture-options.md). S1 is implemented and awaits controller review. I1–I4 stay `NOT_AUTHORIZED`.

## Wave register

| Wave | Kind | Content | Status |
|------|------|---------|--------|
| W0 | Discovery | This document set. Variant A accepted as discovery scope. The product read gap stays open | `DISCOVERY_ACCEPTED` |
| S1 | Product security remediation | Tenant predicate on `GET /v1/documents/{id}` and on `GetSession`. Separate from EDO 0.3 schema | `IMPLEMENTED_AWAITING_CONTROLLER_REVIEW` |
| W1 | Decision | Variant A is already the accepted discovery scope. A later implementation Task Contract still needs its own controller review. S1 stays a prerequisite | `NOT_STARTED` |
| I1 | Schema | Additive `documents` migration for package, semantic relationship, certificate-evidence metadata bound to a revision, and immutability constraints. No other schema. No `PACKAGE_CONTAINS_DOCUMENT` | `NOT_AUTHORIZED` |
| I2 | Rules | document-service commands: seal package, append semantic relationship, block signed mutation and signed file attach, trusted tenant predicate on every read including package and relationship | `NOT_AUTHORIZED` |
| I3 | Edge | Gateway exposure only if I2 needs a public route. Trusted tenant only. OpenAPI source-of-truth update in the same reviewed change | `NOT_AUTHORIZED` |
| I4 | Verification | Unit, tenant-isolation, security regression, and immutability tests named in [edo-0.3-test-acceptance-strategy.md](edo-0.3-test-acceptance-strategy.md) | `NOT_AUTHORIZED` |

W0 remains accepted discovery. This branch implements S1 only and awaits controller review.

```text
DOCUMENT_READ_TENANT_ISOLATION_REMEDIATION_REQUIRED
```

S1 is that remediation. Its code is on this branch and awaits an independent controller review. I1, I2, I3, and I4 are not started. Legal verification stays open.

## Task Contract requirements for I1–I4

Each future implementation Task Contract must include all of the following. None of them is authorized now:

- up migration
- down migration, or a documented irreversible decision
- rollback procedure
- OpenAPI source-of-truth update
- generator and artifact parity with that OpenAPI source
- targeted tests
- tenant-isolation regression, including the get-by-id path once S1 exists
- security regression
- CI on the exact head of that wave
- controlled merge
- post-merge CI
- a separate controller verdict for that wave

Membership stays on `DocumentPackage` only. Relationships stay semantic. Signature and certificate evidence in I1 bind to the signed revision, not to `document_id` alone.

## Review gate for every wave after W0

S1 needs its own controller verdict and does not wait for the EDO 0.3 variant decision. I1 and later waves wait for both that S1 verdict and the variant decision.

A wave may start only when a separate controller review records:

- the wave id
- the accepted variant
- allowed paths
- confirmation that TEDO, billing bridge, TMS schema, factoring, and WORM provisioning stay out
- confirmation that legal questions still marked `LEGAL_VERIFICATION_REQUIRED` are not being asserted as done
- `IMPLEMENTATION_AUTHORIZED` for that wave only

A pass on W0 is not `IMPLEMENTATION_AUTHORIZED` for I1.

## Dependencies that can proceed in parallel outside EDO 0.3

These are other workstreams. They are not children of I1 and this discovery does not assign them:

| Item | Workstream |
|------|------------|
| Membership write-path remediation | PLAT-0.1 |
| Object storage design, not bucket creation by EDO | INFRA-0.1 |
| ETRN lifecycle design | TEDO-0.3 |
| Billing document bridge | EDO-0.5 |

## Stop rule

If a wave needs an operator credential, a GIS EPD call, a change to `shipment-service` or `transport-order-service`, a billing status change, or a Selectel bucket, it is outside EDO 0.3 and stops for a cross-workstream request ([template](../program/cross-workstream-request-template.md)).

## References

- [edo-0.3-scope.md](edo-0.3-scope.md)
- [edo-0.3-test-acceptance-strategy.md](edo-0.3-test-acceptance-strategy.md)
- [ADR-EDO-009](../adr/ADR-EDO-009-cross-workstream-mutation-policy.md)
