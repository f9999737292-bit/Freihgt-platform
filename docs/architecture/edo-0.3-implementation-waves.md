# EDO 0.3 Implementation Waves

## Status

```text
DOCUMENT_STATUS=DISCOVERY
IMPLEMENTATION_AUTHORIZED=NO
EACH_WAVE_REQUIRES=INDEPENDENT_CONTROLLER_REVIEW
```

Waves are a review sequence. Completing discovery does not start wave I1. No wave in this list is authorized to change product code, migrations, OpenAPI, or CI.

Recommended content is variant A from [edo-0.3-architecture-options.md](edo-0.3-architecture-options.md). If the controller selects another variant, the wave contents change before any build wave is written.

## Wave register

| Wave | Kind | Content | Status |
|------|------|---------|--------|
| W0 | Discovery | This document set. Scope, gaps, options, security, sources, tests, risks | `DISCOVERY_COMPLETE_AWAITING_CONTROLLER_REVIEW` |
| W1 | Decision | Controller accepts variant A or explicitly selects B, C, or D. Confirms out-of-scope list | `NOT_STARTED` |
| I1 | Schema | Additive `documents` migration for package, relationship, certificate-evidence metadata, and immutability constraints. No other schema | `NOT_AUTHORIZED` |
| I2 | Rules | document-service commands: seal package, append relationship, block signed mutation and signed file attach, tenant predicate on every read | `NOT_AUTHORIZED` |
| I3 | Edge | Gateway exposure only if I2 needs a public route. Trusted tenant only. OpenAPI update in the same reviewed change | `NOT_AUTHORIZED` |
| I4 | Verification | Unit, tenant-isolation, and immutability tests named in [edo-0.3-test-acceptance-strategy.md](edo-0.3-test-acceptance-strategy.md) | `NOT_AUTHORIZED` |

W0 is the only wave this branch performs. It is documentation.

## Review gate for every wave after W0

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
