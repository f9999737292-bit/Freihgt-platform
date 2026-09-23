# BINTRANS Program Workstream Status Model

## Status

Canonical status model — EDO-0.2 freeze

## Purpose

Prevent project confusion across parallel workstreams. **Do not alter active workstream runtime status without evidence.**

## Row schema

Each workstream row carries:

| Column | Description |
|--------|-------------|
| CURRENT_PHASE | Active program phase ID |
| STATUS | Phase status enum |
| OWNER | Responsible workstream lead / role |
| BASE_SHA | Git SHA evidence for status claims |
| BRANCH/PR | Active branch or PR reference |
| DEPENDENCIES | Upstream workstreams or external blockers |
| BLOCKERS | Active blockers |
| NEXT_PHASE | Recommended successor phase |

## Status enum

| STATUS | Meaning |
|--------|---------|
| DISCOVERY_COMPLETE | Read-only architecture discovery done |
| ARCHITECTURE_FREEZE | Contracts frozen; no product code |
| IMPLEMENTATION | Authorized product code in progress |
| VERIFICATION | Test/staging verification |
| BLOCKED | Cannot proceed |
| OPERATIONAL | Running in staging/production subset |

---

## Workstream registry (evidence-based snapshot)

| WS | CURRENT_PHASE | STATUS | OWNER | BASE_SHA | BRANCH/PR | DEPENDENCIES | BLOCKERS | NEXT_PHASE |
|----|---------------|--------|-------|----------|-----------|--------------|----------|------------|
| **PLAT** | PLAT-active | IMPLEMENTATION | Platform team | `a5163c3` (primary dev WT) | `test/control-tower-projection-rebuild-live-acceptance-v0.4` @ `D:\Projects\freight-platform` | — | F-002 dual-write not yet remediated | PLAT-0.1 membership cleanup |
| **LOG** | LOG-active | IMPLEMENTATION | Logistics team | `a5163c3` | same primary dev WT | PLAT | ROAD-only mode | MM-0.2 leg schema (via LOG) |
| **CT** | CT-shadow | OPERATIONAL | Control Tower team | staging pack `4d0cdfb` | `ops/bintrans-ct-staging-pack` | LOG Kafka events | PRIMARY mode disabled; shadow only | CT consume `edo.document.*` (future) |
| **FC** | FC-v2.2 | IMPLEMENTATION | Finance cost team | `d0005bd` | main-aligned | LOG, billing | Mock EDO billing path (F-003) | EDO-0.5 billing bridge |
| **EDO** | EDO-0.3 | DISCOVERY_COMPLETE | EDO architecture | `5b0d55b7` | PR #162, CI `35912686104`, verdict `ACCEPT_EDO_0_3_SCOPE` | S1 product read-isolation remediation | Legal verification open; `DOCUMENT_READ_TENANT_ISOLATION_REMEDIATION_REQUIRED`; S1 and I1–I4 not authorized; no archive storage | Separate authorization of S1. Variant A is the accepted discovery scope. Implementation is not started |
| **TEDO** | TEDO-0.2 | ARCHITECTURE_FREEZE | Transport EDO | `d0005bd` | EDO-0.2 archive `discovery/edo-ecosystem-architecture-v0.1` | EDO, LOG | No operator licensing | TEDO-0.3 ETRN lifecycle design |
| **MM** | MM-0.2 | ARCHITECTURE_FREEZE | Multimodal | `d0005bd` | EDO-0.2 archive `discovery/edo-ecosystem-architecture-v0.1` | LOG (shipment-service) | ROAD-only enforcement in code | MM-0.2 leg implementation request → LOG |
| **FF** | FF-v1.9 | IMPLEMENTATION | Payments/finance | `d0005bd` | payment reconciliation branches | FC | No receivable aggregate yet | FF-0.2 receivable design impl |
| **INFRA** | INFRA-staging | OPERATIONAL | DevOps | `4d0cdfb` staging pack | `ops/bintrans-ct-staging-pack` | — | S3/WORM not configured (F-004) | INFRA-0.1 object storage |

### Notes

- Primary BINTRANS development continues in `D:\Projects\freight-platform` — **untouched by EDO-0.2**.
- EDO-0.2 discovery/freeze worktree `D:\Projects\freight-platform-wt\edo-ecosystem-architecture-v0.1` remains an archive. Do not treat it as the active EDO-0.3 checkout.
- EDO-0.3 discovery scope is accepted (`CONTROLLER_VERDICT=ACCEPT_EDO_0_3_SCOPE`) at `5b0d55b7c44eef4d9da14a2add3f7e965e7b02ec`, CI `35912686104`, pull request #162. Variant A is that scope. S1 is a required product-security prerequisite and is not authorized. I1–I4 are not authorized. Legal verification stays open. The get-by-id tenant gap is not fixed.
- BASE_SHA for TEDO/MM rows remains discovery baseline `d0005bd8b055b0d2250e5092a0c1c0484decf540`. The EDO row base is the EDO-0.3 discovery base above.

## Update procedure

1. Change row only with evidence: merged PR SHA, release manifest, or operator sign-off.
2. Reference ADR or Task Contract ID in commit message when updating this file.
3. EDO agents update EDO/TEDO/MM rows only; LOG/CT/PLAT rows require evidence from those streams.

## References

- ADR-EDO-009
- Discovery PROPOSED_WORKSTREAMS
- `docs/program/cross-workstream-request-template.md`
